package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type compactionCacheFakeStore struct {
	*mockChronicleStore
	cache  map[string]string
	getErr error
	putErr error
	puts   int
}

func (f *compactionCacheFakeStore) GetCompactionCache(ctx context.Context, projectID, cacheKey string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	if f.cache == nil {
		return "", nil
	}
	return f.cache[projectID+":"+cacheKey], nil
}

func (f *compactionCacheFakeStore) PutCompactionCache(ctx context.Context, projectID, cacheKey, compiledText string) error {
	f.puts++
	if f.putErr != nil {
		return f.putErr
	}
	if f.cache == nil {
		f.cache = make(map[string]string)
	}
	f.cache[projectID+":"+cacheKey] = compiledText
	return nil
}

func setupCompactionCacheFixture() (*CanonChronicle, *compactionCacheFakeStore, int) {
	chronicle, budget := setupCompactionTestFixture()
	baseMock := chronicle.store.(*mockChronicleStore)
	fake := &compactionCacheFakeStore{
		mockChronicleStore: baseMock,
		cache:              make(map[string]string),
	}
	chronicle.store = fake
	return chronicle, fake, budget
}

func TestCompactionCacheKey_DeterminismAndStability(t *testing.T) {
	items1 := []CompactionItem{
		{ID: "item-b", Kind: "codex", Text: "text-b"},
		{ID: "item-a", Kind: "callback", Text: "text-a"},
	}
	items2 := []CompactionItem{
		{ID: "item-a", Kind: "callback", Text: "text-a"},
		{ID: "item-b", Kind: "codex", Text: "text-b"},
	}

	key1 := compactionCacheKey(items1, 150)
	key2 := compactionCacheKey(items2, 150)
	if key1 != key2 {
		t.Fatalf("expected stable sort to produce identical key regardless of slice order, got %q vs %q", key1, key2)
	}

	keyDiffBudget := compactionCacheKey(items1, 200)
	if key1 == keyDiffBudget {
		t.Fatalf("expected different budget to produce different key")
	}
}

// Case A. 同输入二次组装：首次 compactor 调用 1 次、二次 0 次（缓存命中）、二次 trace.CompactionCacheHit==true、注入内容一致。
func TestCompactionCache_CaseA_IdenticalInputHitsCache(t *testing.T) {
	chronicle, fake, _ := setupCompactionCacheFixture()

	excerpt := "【压缩摘录】妖皇深渊封印因果潜伏未消。"
	compactorCalls := 0
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		compactorCalls++
		return excerpt, nil
	})

	ctx := context.Background()
	projectID := "p_compaction_test"
	chapter := 5

	// 首次组装：未命中缓存，调用 compactor，回写缓存
	h1, trace1, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("first assemble failed: %v", err)
	}
	if compactorCalls != 1 {
		t.Fatalf("expected 1 compactor call on first assemble, got %d", compactorCalls)
	}
	if trace1.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==false on first assemble")
	}
	if !strings.Contains(h1.HistoricalCallbacks, excerpt) {
		t.Fatalf("expected h1 to contain excerpt")
	}
	if fake.puts != 1 {
		t.Fatalf("expected fake.puts==1, got %d", fake.puts)
	}

	// 二次组装：同输入，命中缓存，compactor 不再调用
	h2, trace2, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("second assemble failed: %v", err)
	}
	if compactorCalls != 1 {
		t.Fatalf("expected 0 additional compactor calls (total 1), got %d", compactorCalls)
	}
	if !trace2.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==true on second assemble")
	}
	if !strings.Contains(h2.HistoricalCallbacks, excerpt) {
		t.Fatalf("expected h2 to contain excerpt")
	}
	if h2.HistoricalCallbacks != h1.HistoricalCallbacks {
		t.Errorf("expected injected content to be identical across runs")
	}
}

// Case B. 剩余预算不同 → 未命中（compactor 再次调用）。
func TestCompactionCache_CaseB_DifferentRemainingBudgetMissesCache(t *testing.T) {
	chronicle, _, budget := setupCompactionCacheFixture()

	excerpt := "【压缩摘录】因果潜伏未消。"
	compactorCalls := 0
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		compactorCalls++
		return excerpt, nil
	})

	ctx := context.Background()
	projectID := "p_compaction_test"
	chapter := 5

	// 首次组装
	_, trace1, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("first assemble failed: %v", err)
	}
	if compactorCalls != 1 {
		t.Fatalf("expected 1 compactor call, got %d", compactorCalls)
	}
	if trace1.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==false")
	}

	// 改变剩余预算（增加 50 tokens）
	chronicle.SetBudgetTokens(budget + 50)
	_, trace2, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("second assemble failed: %v", err)
	}
	if compactorCalls != 2 {
		t.Fatalf("expected compactor called again (total 2), got %d", compactorCalls)
	}
	if trace2.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==false due to different remaining budget")
	}
}

// Case C. 缓存值超出 remaining → 失效重编译。
func TestCompactionCache_CaseC_OversizedCachedValueInvalidatesAndRecompiles(t *testing.T) {
	chronicle, fake, budget := setupCompactionCacheFixture()

	normalExcerpt := "【压缩摘录】妖皇深渊封印因果潜伏未消。"
	compactorCalls := 0
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		compactorCalls++
		return normalExcerpt, nil
	})

	ctx := context.Background()
	projectID := "p_compaction_test"
	chapter := 5

	// 首次组装：写入正常产物
	_, _, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("first assemble failed: %v", err)
	}
	if compactorCalls != 1 {
		t.Fatalf("expected 1 compactor call, got %d", compactorCalls)
	}

	// 找到写入的 cacheKey 并注入超大内容（超预算）
	oversized := strings.Repeat("超长压缩记忆文本超出预算限制", 50)
	if domain.EstimateTokens(oversized) <= budget {
		t.Fatalf("expected oversized text to exceed budget")
	}
	for k := range fake.cache {
		fake.cache[k] = oversized
	}

	// 二次组装：读取缓存发现超预算，视为未命中，重新调用 compactor
	h2, trace2, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("second assemble failed: %v", err)
	}
	if compactorCalls != 2 {
		t.Fatalf("expected compactor called again (total 2), got %d", compactorCalls)
	}
	if trace2.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==false due to oversized cache entry")
	}
	if !strings.Contains(h2.HistoricalCallbacks, normalExcerpt) {
		t.Fatalf("expected recompiled excerpt in HistoricalCallbacks")
	}
}

// Case D. 存储不支持能力（用纯 mockChronicleStore）→ 每次编译、无 error。
func TestCompactionCache_CaseD_UnsupportedStoreSkipsGracefully(t *testing.T) {
	chronicle, _ := setupCompactionTestFixture() // uses pure *mockChronicleStore

	compactorCalls := 0
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		compactorCalls++
		return "【压缩摘录】无缓存存储支持。", nil
	})

	ctx := context.Background()
	projectID := "p_compaction_test"
	chapter := 5

	// 首次
	h1, trace1, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("first assemble failed: %v", err)
	}
	if compactorCalls != 1 {
		t.Fatalf("expected 1 call, got %d", compactorCalls)
	}
	if trace1.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==false")
	}
	if trace1.CompactionFailed {
		t.Errorf("expected CompactionFailed==false")
	}

	// 二次：由于存储不支持能力，仍正常编译，无 error
	h2, trace2, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("second assemble failed: %v", err)
	}
	if compactorCalls != 2 {
		t.Fatalf("expected 2 calls, got %d", compactorCalls)
	}
	if trace2.CompactionCacheHit {
		t.Errorf("expected CompactionCacheHit==false")
	}
	if trace2.CompactionFailed {
		t.Errorf("expected CompactionFailed==false")
	}
	if h1.HistoricalCallbacks != h2.HistoricalCallbacks {
		t.Errorf("expected consistent output across runs")
	}
}

// Case E. putErr 注入 → trace.CompactionNote 含"压缩缓存写入失败"、组装仍成功。
func TestCompactionCache_CaseE_PutErrorRecordsNoteAndSucceeds(t *testing.T) {
	chronicle, fake, _ := setupCompactionCacheFixture()
	fake.putErr = errors.New("simulated disk I/O error")

	excerpt := "【压缩摘录】妖皇深渊封印因果潜伏未消。"
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		return excerpt, nil
	})

	ctx := context.Background()
	projectID := "p_compaction_test"
	chapter := 5

	horizon, trace, err := chronicle.AssembleHorizonTraced(ctx, projectID, chapter)
	if err != nil {
		t.Fatalf("assemble failed: %v", err)
	}
	if trace.CompactionFailed {
		t.Errorf("expected CompactionFailed==false, assembly should not fail")
	}
	if !strings.Contains(trace.CompactionNote, "压缩缓存写入失败") {
		t.Errorf("expected CompactionNote to contain '压缩缓存写入失败', got %q", trace.CompactionNote)
	}
	if !strings.Contains(horizon.HistoricalCallbacks, excerpt) {
		t.Errorf("expected injected callbacks to contain compiled excerpt")
	}
}

func TestCompactionCache_GetErrorRecordsNoteAndRecompiles(t *testing.T) {
	chronicle, fake, _ := setupCompactionCacheFixture()
	fake.getErr = errors.New("simulated read error")

	compactorCalls := 0
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		compactorCalls++
		return "【压缩摘录】产物", nil
	})

	ctx := context.Background()
	_, trace, err := chronicle.AssembleHorizonTraced(ctx, "p_compaction_test", 5)
	if err != nil {
		t.Fatalf("assemble failed: %v", err)
	}
	if compactorCalls != 1 {
		t.Fatalf("expected compactor called, got %d", compactorCalls)
	}
	if !strings.Contains(trace.CompactionNote, "压缩缓存读取失败") {
		t.Errorf("expected note to contain '压缩缓存读取失败', got %q", trace.CompactionNote)
	}
}
