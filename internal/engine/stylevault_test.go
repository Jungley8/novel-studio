package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type mockStyleVaultStore struct {
	vaults   map[string]*domain.StyleVault
	chapters map[string][]*domain.Chapter
}

func newMockStyleVaultStore() *mockStyleVaultStore {
	return &mockStyleVaultStore{
		vaults:   make(map[string]*domain.StyleVault),
		chapters: make(map[string][]*domain.Chapter),
	}
}

func (m *mockStyleVaultStore) GetStyleVault(ctx context.Context, projectID string) (*domain.StyleVault, error) {
	return m.vaults[projectID], nil
}

func (m *mockStyleVaultStore) SaveStyleVault(ctx context.Context, vault *domain.StyleVault) error {
	m.vaults[vault.ProjectID] = vault
	return nil
}

func (m *mockStyleVaultStore) SetGoldenExcerpts(ctx context.Context, projectID string, excerpts []string) error {
	m.vaults[projectID] = &domain.StyleVault{
		ProjectID:      projectID,
		GoldenExcerpts: excerpts,
		UpdatedAt:      time.Now(),
	}
	return nil
}

func (m *mockStyleVaultStore) ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error) {
	return m.chapters[projectID], nil
}

// ① ≥3 章自动播种确定性（同样输入同样 excerpts）
func TestStyleVault_SeedDeterminism_GE3Chapters(t *testing.T) {
	ctx := context.Background()
	store := newMockStyleVaultStore()
	projectID := "proj-golden"

	// Create 3 chapters with unique content (>300 runes each)
	ch1Runes := strings.Repeat("甲", 200) + strings.Repeat("乙", 300)
	ch2Runes := strings.Repeat("丙", 200) + strings.Repeat("丁", 300)
	ch3Runes := strings.Repeat("戊", 200) + strings.Repeat("己", 300)

	store.chapters[projectID] = []*domain.Chapter{
		{ChapterIndex: 1, Title: "第一章", Content: ch1Runes},
		{ChapterIndex: 2, Title: "第二章", Content: ch2Runes},
		{ChapterIndex: 3, Title: "第三章", Content: ch3Runes},
	}

	// 1. Initial seeding
	vault1, err := engine.EnsureSeededVault(ctx, store, projectID)
	if err != nil {
		t.Fatalf("EnsureSeededVault failed: %v", err)
	}
	if vault1 == nil || len(vault1.GoldenExcerpts) != 3 {
		t.Fatalf("expected 3 golden excerpts, got %+v", vault1)
	}

	expectedEx1 := strings.Repeat("乙", 300)
	expectedEx2 := strings.Repeat("丁", 300)
	expectedEx3 := strings.Repeat("己", 300)

	if vault1.GoldenExcerpts[0] != expectedEx1 {
		t.Errorf("excerpt 0 mismatch")
	}
	if vault1.GoldenExcerpts[1] != expectedEx2 {
		t.Errorf("excerpt 1 mismatch")
	}
	if vault1.GoldenExcerpts[2] != expectedEx3 {
		t.Errorf("excerpt 2 mismatch")
	}

	// 2. Second call must return identical excerpts deterministically
	vault2, err := engine.EnsureSeededVault(ctx, store, projectID)
	if err != nil {
		t.Fatalf("second EnsureSeededVault failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if vault2.GoldenExcerpts[i] != vault1.GoldenExcerpts[i] {
			t.Errorf("determinism violation at excerpt %d", i)
		}
	}

	// 3. Anchor is constant: adding chapter 4 & 5 does not change the seed
	store.chapters[projectID] = append(store.chapters[projectID],
		&domain.Chapter{ChapterIndex: 4, Title: "第四章", Content: strings.Repeat("庚", 500)},
		&domain.Chapter{ChapterIndex: 5, Title: "第五章", Content: strings.Repeat("辛", 500)},
	)
	vault3, err := engine.EnsureSeededVault(ctx, store, projectID)
	if err != nil {
		t.Fatalf("third EnsureSeededVault failed: %v", err)
	}
	if vault3.GoldenExcerpts[0] != expectedEx1 || vault3.GoldenExcerpts[1] != expectedEx2 || vault3.GoldenExcerpts[2] != expectedEx3 {
		t.Errorf("vault was mutated after adding more chapters; must remain constant")
	}
}

// ② 不足 3 章空锚
func TestStyleVault_InsufficientChapters_EmptyAnchor(t *testing.T) {
	ctx := context.Background()
	store := newMockStyleVaultStore()
	projectID := "proj-short"

	// 0 chapters
	v0, err := engine.EnsureSeededVault(ctx, store, projectID)
	if err != nil {
		t.Fatalf("EnsureSeededVault on 0 chapters failed: %v", err)
	}
	if len(v0.GoldenExcerpts) != 0 {
		t.Errorf("expected 0 excerpts for 0 chapters, got %d", len(v0.GoldenExcerpts))
	}
	if anchor := engine.BuildStyleAnchor(v0); anchor != "" {
		t.Errorf("expected empty anchor, got %q", anchor)
	}

	// 2 chapters
	store.chapters[projectID] = []*domain.Chapter{
		{ChapterIndex: 1, Title: "第一章", Content: "起步。"},
		{ChapterIndex: 2, Title: "第二章", Content: "探索。"},
	}
	v2, err := engine.EnsureSeededVault(ctx, store, projectID)
	if err != nil {
		t.Fatalf("EnsureSeededVault on 2 chapters failed: %v", err)
	}
	if len(v2.GoldenExcerpts) != 0 {
		t.Errorf("expected 0 excerpts for 2 chapters, got %d", len(v2.GoldenExcerpts))
	}
	if anchor := engine.BuildStyleAnchor(v2); anchor != "" {
		t.Errorf("expected empty anchor, got %q", anchor)
	}
}

// ④ echo 抓住预埋跨章短语
func TestDetectEchoPhrases_DetectsEmbeddedPhrases(t *testing.T) {
	// 10 chapters
	recent := make([]string, 10)
	for i := 0; i < 10; i++ {
		recent[i] = "寻常叙事，山川风物，修士静坐吐纳。"
	}

	// Plant "指尖微屈" in 3 chapters (chapters 0, 1, 2) -> 3/10 = 30% <= 50%
	recent[0] += "他面色漠然，指尖微屈。"
	recent[1] += "狂风呼啸过，指尖微屈间杀机暗涌。"
	recent[2] += "对峙良久后，指尖微屈蓄势待发。"

	// Plant "寒风如刀" in 4 chapters (chapters 3, 4, 5, 6) -> 4/10 = 40% <= 50%
	recent[3] += "深冬时节处寒风如刀割面生疼。"
	recent[4] += "夜幕降临际寒风如刀呼啸而过。"
	recent[5] += "漫步雪原上寒风如刀刺入肌理。"
	recent[6] += "山巅极寒境寒风如刀凛冽彻骨。"

	draft := "风雪漫天间寒风如刀。顾渊负手而立，指尖微屈，冷眼看着眼前的敌人。"

	hits := engine.DetectEchoPhrases(recent, draft)
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 hits, got %v", hits)
	}

	// Higher hit count first: "寒风如刀" (4 hits) should appear before "指尖微屈" (3 hits)
	if hits[0] != "寒风如刀" {
		t.Errorf("expected first hit to be 寒风如刀, got %s", hits[0])
	}
	if hits[1] != "指尖微屈" {
		t.Errorf("expected second hit to be 指尖微屈, got %s", hits[1])
	}
}

// ⑤ 通用高频短语（>50% 章出现）不告警
func TestDetectEchoPhrases_CommonPhrasesExcluded(t *testing.T) {
	// 10 chapters
	recent := make([]string, 10)
	for i := 0; i < 10; i++ {
		recent[i] = "寻常叙事，各方势力暗流涌动。"
	}

	// "天地灵气" planted in 6 chapters (6/10 = 60% > 50%) -> common style, must NOT alarm
	for i := 0; i < 6; i++ {
		recent[i] += "运转功法，吞吐天地灵气化为己用。"
	}

	// "指尖微屈" planted in 3 chapters (3/10 = 30% <= 50%) -> echo phrase, MUST alarm
	recent[0] += "顾渊指尖微屈。"
	recent[1] += "黑衣人指尖微屈。"
	recent[2] += "老者指尖微屈。"

	draft := "引动周遭天地灵气灌入经脉，他指尖微屈，骤然发难。"

	hits := engine.DetectEchoPhrases(recent, draft)

	// Verify "天地灵气" is NOT reported
	for _, h := range hits {
		if h == "天地灵气" {
			t.Errorf("expected common phrase '天地灵气' (>50%% chapters) to be excluded, but it was alerted: %v", hits)
		}
	}

	// Verify "指尖微屈" IS reported
	foundTarget := false
	for _, h := range hits {
		if h == "指尖微屈" {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		t.Errorf("expected echo phrase '指尖微屈' to be alerted, got: %v", hits)
	}
}

// ⑥ 空输入不 panic
func TestDetectEchoPhrases_EmptyInputsNoPanic(t *testing.T) {
	// Empty slices or empty draft
	_ = engine.DetectEchoPhrases(nil, "")
	_ = engine.DetectEchoPhrases([]string{}, "")
	_ = engine.DetectEchoPhrases([]string{"a", "b"}, "c")
	_ = engine.DetectEchoPhrases(nil, "天地玄黄")
	_ = engine.DetectEchoPhrases([]string{"天地玄黄"}, "")

	// BuildStyleAnchor nil/empty checks
	if s := engine.BuildStyleAnchor(nil); s != "" {
		t.Errorf("expected empty string for nil vault, got %q", s)
	}
	if s := engine.BuildStyleAnchor(&domain.StyleVault{}); s != "" {
		t.Errorf("expected empty string for empty vault, got %q", s)
	}
	if s := engine.BuildStyleAnchor(&domain.StyleVault{GoldenExcerpts: []string{"", "   "}}); s != "" {
		t.Errorf("expected empty string for whitespace excerpts, got %q", s)
	}
}

type promptCaptureClient struct {
	capturedUserPrompt string
}

func (c *promptCaptureClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, error) {
	c.capturedUserPrompt = userPrompt
	return "正文输出", nil
}

func (c *promptCaptureClient) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, engine.TokenUsage, error) {
	c.capturedUserPrompt = userPrompt
	return "正文输出", engine.TokenUsage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}, nil
}

func (c *promptCaptureClient) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (<-chan engine.StreamChunk, error) {
	c.capturedUserPrompt = userPrompt
	ch := make(chan engine.StreamChunk, 2)
	ch <- engine.StreamChunk{Delta: "正文输出"}
	ch <- engine.StreamChunk{Done: true, Usage: &engine.TokenUsage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}}
	close(ch)
	return ch, nil
}

// ⑦ render prompt 含主锚（fake client 捕获 userPrompt 断言，可走现有 orchestrator fake 模式）
func TestOrchestrator_RenderPrompt_ContainsStyleVaultAnchor(t *testing.T) {
	client := &promptCaptureClient{}
	orch := engine.NewOrchestrator(client)

	p := &domain.Project{Title: "长生仙途", TargetPlatform: "番茄脑洞", WorldRules: "长生有代价"}
	beats := []domain.SceneBeat{
		{Phase: "起势", Tension: 5, Action: "踏入山门"},
	}

	// 1. With non-empty StyleVaultAnchor and TailAnchor
	vaultAnchor := "【文风契约·稳定锚（全书恒定，严禁漂移）】\n• 样本 1：\n青云如墨，古剑无声。"
	tailAnchor := "上一章末尾二百字衔接锚点。"

	horizonWithVault := &engine.CanonHorizon{
		Project:          p,
		TargetChapter:    4,
		StyleVaultAnchor: vaultAnchor,
		TailAnchor:       tailAnchor,
	}

	_, _, err := orch.RenderSceneWithHorizon(context.Background(), "deepseek-chat", horizonWithVault, beats, 1500)
	if err != nil {
		t.Fatalf("RenderSceneWithHorizon failed: %v", err)
	}

	prompt := client.capturedUserPrompt
	if !strings.Contains(prompt, "【主文风锚定】") {
		t.Errorf("expected prompt to contain primary anchor tag '【主文风锚定】', got:\n%s", prompt)
	}
	if !strings.Contains(prompt, vaultAnchor) {
		t.Errorf("expected prompt to contain StyleVaultAnchor, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "【辅·上章收尾文风锚定 (最后200字，请严格承接此腔调与视角)】") {
		t.Errorf("expected prompt to contain secondary anchor tag '【辅·上章收尾文风锚定...】', got:\n%s", prompt)
	}
	if !strings.Contains(prompt, tailAnchor) {
		t.Errorf("expected prompt to contain TailAnchor, got:\n%s", prompt)
	}

	// Verify primary anchor is injected BEFORE secondary anchor
	idxVault := strings.Index(prompt, vaultAnchor)
	idxTail := strings.Index(prompt, tailAnchor)
	if idxVault < 0 || idxTail < 0 || idxVault >= idxTail {
		t.Errorf("expected StyleVaultAnchor to appear before TailAnchor, got idxVault=%d, idxTail=%d", idxVault, idxTail)
	}

	// 2. Without StyleVaultAnchor (behavior identical to main)
	client2 := &promptCaptureClient{}
	orch2 := engine.NewOrchestrator(client2)

	horizonWithoutVault := &engine.CanonHorizon{
		Project:       p,
		TargetChapter: 2,
		TailAnchor:    tailAnchor,
	}

	_, _, err = orch2.RenderSceneWithHorizon(context.Background(), "deepseek-chat", horizonWithoutVault, beats, 1500)
	if err != nil {
		t.Fatalf("RenderSceneWithHorizon failed: %v", err)
	}

	prompt2 := client2.capturedUserPrompt
	if strings.Contains(prompt2, "【主文风锚定】") {
		t.Errorf("prompt without vault anchor should NOT contain '【主文风锚定】', got:\n%s", prompt2)
	}
	expectedMainAnchor := "【上章收尾文风锚定 (最后200字，请严格承接此腔调与视角)】"
	if !strings.Contains(prompt2, expectedMainAnchor) {
		t.Errorf("prompt without vault anchor should retain main anchor header %q, got:\n%s", expectedMainAnchor, prompt2)
	}
}
