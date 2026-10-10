package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

type mockVolBatchStore struct {
	*mockChronicleStore
	mu          sync.Mutex
	checkpoints map[string]*domain.ChapterCheckpoint
}

func newMockVolBatchStore(project *domain.Project) *mockVolBatchStore {
	return &mockVolBatchStore{
		mockChronicleStore: &mockChronicleStore{
			project:  project,
			chapters: []*domain.Chapter{},
			hooks:    []*domain.PlotHook{},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-vol_pro",
					ProjectID: project.ID,
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "练气圆满",
					Inventory: "青玉短剑",
				},
			},
		},
		checkpoints: make(map[string]*domain.ChapterCheckpoint),
	}
}

func (m *mockVolBatchStore) SaveCheckpoint(ctx context.Context, cp *domain.ChapterCheckpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%d", cp.ProjectID, cp.ChapterIndex)
	m.checkpoints[key] = cp
	return nil
}

func (m *mockVolBatchStore) GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%d", projectID, chapterIndex)
	return m.checkpoints[key], nil
}

func (m *mockVolBatchStore) ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%d", projectID, chapterIndex)
	delete(m.checkpoints, key)
	return nil
}

type mockVolBatchLLMClient struct {
	mu             sync.Mutex
	failChapter    int
	currentChapter int
}

func (m *mockVolBatchLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	m.mu.Lock()
	for i := 1; i <= 200; i++ {
		if strings.Contains(userPrompt, fmt.Sprintf("第 %d 章", i)) || strings.Contains(systemPrompt, fmt.Sprintf("第 %d 章", i)) {
			m.currentChapter = i
			break
		}
	}
	cur := m.currentChapter
	m.mu.Unlock()

	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
	if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
		if m.failChapter > 0 && cur == m.failChapter {
			return `{
				"verdict": "REVISION_NEEDED",
				"score": 60,
				"issues": ["剧情张力不足，未达标"],
				"suggestions": "重写增加悬念"
			}`, nil
		}
		return `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "通过"
		}`, nil
	}
	if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
		return `{
			"beats": [
				{"phase": "蓄力", "tension": 6, "action": "对峙", "expectation_broken": "反派失算"},
				{"phase": "破局", "tension": 9, "action": "逆转", "expectation_broken": "全面击溃"}
			],
			"state_mutation": {"inventory_delta": "+战利品", "power_delta": "突破"}
		}`, nil
	}

	return "夜色渐深，冷风呼啸。林凡拔出青玉长剑，一剑断去黑衣人退路。", nil
}

func (m *mockVolBatchLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 80, CompletionTokens: 160, TotalTokens: 240}, err
}

func (m *mockVolBatchLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 80, CompletionTokens: 160, TotalTokens: 240}}
	close(ch)
	return ch, nil
}

func setupTestVolumeWorkshop(client LLMClient, s store.Store) *ChapterWorkshop {
	orch := NewOrchestrator(client)
	chronicle := NewCanonChronicle(s)
	qGate := NewQualityGate(client, nil)
	return NewChapterWorkshop(orch, chronicle, qGate, s)
}

// 1. 3 章全过（Stopped=false，3 章 committed，事件序列含 VOLUME_START/CHAPTER_DONE×3/VOLUME_COMPLETE）
func TestVolumeBatch_ThreeChaptersAllPass(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-batch-pass",
		Title:          "全通大纲",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷 初入仙门",
					StartChapter: 1,
					EndChapter:   3,
				},
			},
		},
	}
	mockStore := newMockVolBatchStore(proj)
	client := &mockVolBatchLLMClient{}
	workshop := setupTestVolumeWorkshop(client, mockStore)
	batch := NewVolumeBatch(workshop, mockStore)

	var events []VolumeEvent
	req := VolumeProduceRequest{
		ProjectID:       "p-batch-pass",
		VolumeIndex:     1,
		WordsTarget:     2000,
		MaxRewriteLoops: 1,
		OnProgress: func(ev VolumeEvent) {
			events = append(events, ev)
		},
	}

	res, err := batch.ProduceVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceVolume failed: %v", err)
	}

	if res.Stopped {
		t.Errorf("expected Stopped == false, got true (reason: %s)", res.StopReason)
	}
	if res.StopReason != "" {
		t.Errorf("expected empty StopReason, got %s", res.StopReason)
	}
	if len(res.Chapters) != 3 {
		t.Fatalf("expected 3 chapters in result, got %d", len(res.Chapters))
	}
	for i, ch := range res.Chapters {
		if !ch.Committed {
			t.Errorf("chapter %d should be committed", ch.ChapterIndex)
		}
		if ch.ChapterIndex != i+1 {
			t.Errorf("expected chapter index %d, got %d", i+1, ch.ChapterIndex)
		}
	}

	// 3 chapters committed in store
	if len(mockStore.chapters) != 3 {
		t.Errorf("expected 3 chapters in store, got %d", len(mockStore.chapters))
	}

	// Check event sequence contains VOLUME_START, CHAPTER_DONE x 3, VOLUME_COMPLETE
	var phases []string
	var chapterDoneCount int
	for _, ev := range events {
		phases = append(phases, ev.Phase)
		if ev.Phase == "CHAPTER_DONE" {
			chapterDoneCount++
		}
	}
	if len(phases) == 0 || phases[0] != "VOLUME_START" {
		t.Errorf("expected first event to be VOLUME_START, got %v", phases)
	}
	if chapterDoneCount != 3 {
		t.Errorf("expected 3 CHAPTER_DONE events, got %d", chapterDoneCount)
	}
	if phases[len(phases)-1] != "VOLUME_COMPLETE" {
		t.Errorf("expected last event to be VOLUME_COMPLETE, got %s", phases[len(phases)-1])
	}
}

// 2. 第 2 章 NeedsHuman → StoppedAt=2、HALTED、第 3 章未消费（断言 store 里只有已入库章节，无第 3 章）
func TestVolumeBatch_ChapterTwoNeedsHuman_Halts(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-batch-halt",
		Title:          "停线自卫",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷 初入仙门",
					StartChapter: 1,
					EndChapter:   3,
				},
			},
		},
	}
	mockStore := newMockVolBatchStore(proj)
	client := &mockVolBatchLLMClient{
		failChapter: 2, // Chapter 2 will fail review and trigger NeedsHuman
	}
	workshop := setupTestVolumeWorkshop(client, mockStore)
	batch := NewVolumeBatch(workshop, mockStore)

	var events []VolumeEvent
	req := VolumeProduceRequest{
		ProjectID:       "p-batch-halt",
		VolumeIndex:     1,
		WordsTarget:     2000,
		MaxRewriteLoops: 1,
		OnProgress: func(ev VolumeEvent) {
			events = append(events, ev)
		},
	}

	res, err := batch.ProduceVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceVolume should not return error on HALTED, got: %v", err)
	}

	if !res.Stopped {
		t.Fatalf("expected Stopped == true on NeedsHuman, got false")
	}
	if res.StopReason != "HALTED" {
		t.Errorf("expected StopReason == HALTED, got %s", res.StopReason)
	}
	if res.StoppedAtChapter != 2 {
		t.Errorf("expected StoppedAtChapter == 2, got %d", res.StoppedAtChapter)
	}

	// Result should only contain 2 chapters processed
	if len(res.Chapters) != 2 {
		t.Errorf("expected 2 chapters in result, got %d", len(res.Chapters))
	}
	if !res.Chapters[0].Committed {
		t.Errorf("chapter 1 should be committed")
	}
	if res.Chapters[1].Committed {
		t.Errorf("chapter 2 should NOT be committed due to NeedsHuman")
	}
	if !res.Chapters[1].NeedsHuman {
		t.Errorf("chapter 2 NeedsHuman should be true")
	}

	// Store must have only chapter 1 committed; chapter 3 was NEVER consumed
	if len(mockStore.chapters) != 1 {
		t.Errorf("expected exactly 1 chapter committed in store, got %d", len(mockStore.chapters))
	}
	for _, ch := range mockStore.chapters {
		if ch.ChapterIndex >= 3 {
			t.Errorf("chapter %d should not have been consumed or committed", ch.ChapterIndex)
		}
	}

	// Verify events include VOLUME_STOPPED
	hasStoppedEvent := false
	for _, ev := range events {
		if ev.Phase == "VOLUME_STOPPED" && ev.ChapterIndex == 2 {
			hasStoppedEvent = true
			break
		}
	}
	if !hasStoppedEvent {
		t.Errorf("expected VOLUME_STOPPED event for chapter 2")
	}
}

// 3. 卷不存在 error
func TestVolumeBatch_VolumeNotFound_Error(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-batch-missing-vol",
		Title:          "缺卷作品",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷",
					StartChapter: 1,
					EndChapter:   10,
				},
			},
		},
	}
	mockStore := newMockVolBatchStore(proj)
	client := &mockVolBatchLLMClient{}
	workshop := setupTestVolumeWorkshop(client, mockStore)
	batch := NewVolumeBatch(workshop, mockStore)

	req := VolumeProduceRequest{
		ProjectID:   "p-batch-missing-vol",
		VolumeIndex: 99, // 不存在
	}

	_, err := batch.ProduceVolume(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for non-existent volume index, got nil")
	}
	if !strings.Contains(err.Error(), "volume 99 not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 4. ChapterFrom 超界 error
func TestVolumeBatch_ChapterFromExceedsEnd_Error(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-batch-chfrom-oob",
		Title:          "起始章超界",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷",
					StartChapter: 1,
					EndChapter:   10,
				},
			},
		},
	}
	mockStore := newMockVolBatchStore(proj)
	client := &mockVolBatchLLMClient{}
	workshop := setupTestVolumeWorkshop(client, mockStore)
	batch := NewVolumeBatch(workshop, mockStore)

	req := VolumeProduceRequest{
		ProjectID:   "p-batch-chfrom-oob",
		VolumeIndex: 1,
		ChapterFrom: 11, // 超过 EndChapter 10
	}

	_, err := batch.ProduceVolume(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error when chapter_from exceeds EndChapter, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds volume end chapter") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 5. 跨度>200 error
func TestVolumeBatch_ChapterSpanOver200_Error(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-batch-span200",
		Title:          "超长大卷",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "超长第一卷",
					StartChapter: 1,
					EndChapter:   205, // 跨度 205 > 200
				},
			},
		},
	}
	mockStore := newMockVolBatchStore(proj)
	client := &mockVolBatchLLMClient{}
	workshop := setupTestVolumeWorkshop(client, mockStore)
	batch := NewVolumeBatch(workshop, mockStore)

	req := VolumeProduceRequest{
		ProjectID:   "p-batch-span200",
		VolumeIndex: 1,
	}

	_, err := batch.ProduceVolume(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error when chapter span exceeds 200, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds limit of 200") {
		t.Errorf("unexpected error message: %v", err)
	}
}
