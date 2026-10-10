package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type mockPlannerLLM struct {
	callCount int
	response  string
	err       error
	usage     engine.TokenUsage
}

func (m *mockPlannerLLM) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockPlannerLLM) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, engine.TokenUsage, error) {
	m.callCount++
	if m.err != nil {
		return "", engine.TokenUsage{}, m.err
	}
	return m.response, m.usage, nil
}

func (m *mockPlannerLLM) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (<-chan engine.StreamChunk, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan engine.StreamChunk, 2)
	ch <- engine.StreamChunk{Delta: m.response}
	ch <- engine.StreamChunk{Done: true, Usage: &m.usage}
	close(ch)
	return ch, nil
}

func TestGenerateVolumeIntents_ValidMappingWithMarkdownFence(t *testing.T) {
	mockResponse := "```json\n" + `[
  {
    "chapter_index": 1,
    "conflict": "主角在家族大比遭遇阴谋阻挠",
    "pov": "林动",
    "emotion_curve": ["压抑", "反转", "扬眉吐气"],
    "hook_plants": ["古石符暗发异芒"],
    "hook_payoffs": ["清算恶奴旧账"],
    "tension_target": 7,
    "expected_mutation": {
      "inventory_delta": "获得玄阴丹一枚",
      "power_delta": "晋升通脉境三重"
    }
  },
  {
    "chapter_index": 2,
    "conflict": "后山古洞探索遭遇二阶妖兽",
    "pov": "林动",
    "emotion_curve": ["警惕", "惊险", "绝处逢生"],
    "hook_plants": ["洞壁残留上古宗门符文"],
    "hook_payoffs": [],
    "tension_target": 8,
    "expected_mutation": {
      "inventory_delta": "采摘千年何首乌",
      "power_delta": "气血淬炼，肉身蜕变"
    }
  }
]` + "\n```"

	mockClient := &mockPlannerLLM{
		response: mockResponse,
		usage:    engine.TokenUsage{PromptTokens: 120, CompletionTokens: 250, TotalTokens: 370},
	}
	orch := engine.NewOrchestrator(mockClient)

	project := &domain.Project{
		ID:             "proj-intent-test",
		Title:          "武动苍穹",
		TargetPlatform: "起点仙侠",
		WorldRules:     "天地元力，强者为尊",
	}
	arc := domain.VolumeArc{
		VolumeIndex:  1,
		Title:        "青阳风云",
		CoreGoal:     "在家族大比中胜出并立足",
		Climax:       "击溃雷家阴谋夺魁",
		KeyPayoffs:   []string{"解开石符之谜"},
		StartChapter: 1,
		EndChapter:   2,
	}

	intents, usage, err := orch.GenerateVolumeIntents(context.Background(), "deepseek-reasoner", project, arc, nil)
	if err != nil {
		t.Fatalf("GenerateVolumeIntents failed: %v", err)
	}

	if mockClient.callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", mockClient.callCount)
	}
	if len(intents) != 2 {
		t.Fatalf("expected 2 intents, got %d", len(intents))
	}
	if usage.TotalTokens != 370 {
		t.Errorf("expected 370 total tokens, got %d", usage.TotalTokens)
	}

	// 验证第1张卡字段映射
	c1 := intents[0]
	if c1.ProjectID != "proj-intent-test" {
		t.Errorf("expected ProjectID proj-intent-test, got %q", c1.ProjectID)
	}
	if c1.VolumeIndex != 1 {
		t.Errorf("expected VolumeIndex 1, got %d", c1.VolumeIndex)
	}
	if c1.ChapterIndex != 1 {
		t.Errorf("expected ChapterIndex 1, got %d", c1.ChapterIndex)
	}
	if c1.Conflict != "主角在家族大比遭遇阴谋阻挠" {
		t.Errorf("expected Conflict '主角在家族大比遭遇阴谋阻挠', got %q", c1.Conflict)
	}
	if c1.POV != "林动" {
		t.Errorf("expected POV '林动', got %q", c1.POV)
	}
	if len(c1.EmotionCurve) != 3 || c1.EmotionCurve[0] != "压抑" || c1.EmotionCurve[1] != "反转" || c1.EmotionCurve[2] != "扬眉吐气" {
		t.Errorf("unexpected EmotionCurve: %v", c1.EmotionCurve)
	}
	if len(c1.HookPlants) != 1 || c1.HookPlants[0] != "古石符暗发异芒" {
		t.Errorf("unexpected HookPlants: %v", c1.HookPlants)
	}
	if len(c1.HookPayoffs) != 1 || c1.HookPayoffs[0] != "清算恶奴旧账" {
		t.Errorf("unexpected HookPayoffs: %v", c1.HookPayoffs)
	}
	if c1.TensionTarget != 7 {
		t.Errorf("expected TensionTarget 7, got %d", c1.TensionTarget)
	}
	if c1.Status != domain.IntentStatusDraft {
		t.Errorf("expected Status DRAFT, got %s", c1.Status)
	}
	if c1.ExpectedMutation.InventoryDelta != "获得玄阴丹一枚" || c1.ExpectedMutation.PowerDelta != "晋升通脉境三重" {
		t.Errorf("unexpected ExpectedMutation: %+v", c1.ExpectedMutation)
	}
	if time.Since(c1.UpdatedAt) > time.Minute {
		t.Errorf("expected recent UpdatedAt, got %v", c1.UpdatedAt)
	}

	// 验证第2张卡字段映射
	c2 := intents[1]
	if c2.ChapterIndex != 2 {
		t.Errorf("expected ChapterIndex 2, got %d", c2.ChapterIndex)
	}
	if c2.TensionTarget != 8 {
		t.Errorf("expected TensionTarget 8, got %d", c2.TensionTarget)
	}
	if c2.ExpectedMutation.InventoryDelta != "采摘千年何首乌" {
		t.Errorf("unexpected c2 InventoryDelta: %s", c2.ExpectedMutation.InventoryDelta)
	}
}

func TestGenerateVolumeIntents_TargetsEmpty_ZeroLLMCallAndZeroToken(t *testing.T) {
	mockClient := &mockPlannerLLM{
		response: "should not be called",
		usage:    engine.TokenUsage{TotalTokens: 999},
	}
	orch := engine.NewOrchestrator(mockClient)

	project := &domain.Project{
		ID:    "proj-1",
		Title: "测试项目",
	}
	arc := domain.VolumeArc{
		VolumeIndex:  1,
		StartChapter: 1,
		EndChapter:   3,
	}
	existing := map[int]bool{
		1: true,
		2: true,
		3: true,
	}

	intents, usage, err := orch.GenerateVolumeIntents(context.Background(), "deepseek-reasoner", project, arc, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(intents) != 0 {
		t.Errorf("expected 0 intents, got %d", len(intents))
	}
	if usage.TotalTokens != 0 || usage.PromptTokens != 0 || usage.CompletionTokens != 0 {
		t.Errorf("expected zero token usage, got %+v", usage)
	}
	if mockClient.callCount != 0 {
		t.Errorf("expected 0 LLM calls, got %d", mockClient.callCount)
	}
}

func TestGenerateVolumeIntents_ExceedsLimit60_FailClosed(t *testing.T) {
	mockClient := &mockPlannerLLM{}
	orch := engine.NewOrchestrator(mockClient)

	project := &domain.Project{
		ID:    "proj-1",
		Title: "测试项目",
	}
	arc := domain.VolumeArc{
		VolumeIndex:  1,
		StartChapter: 1,
		EndChapter:   65,
	}

	_, _, err := orch.GenerateVolumeIntents(context.Background(), "deepseek-reasoner", project, arc, nil)
	if err == nil {
		t.Fatalf("expected error when targets > 60, got nil")
	}
	if !strings.Contains(err.Error(), "60") || (!strings.Contains(err.Error(), "split") && !strings.Contains(err.Error(), "batches") && !strings.Contains(err.Error(), "range")) {
		t.Errorf("expected error message with limit 60 and split suggestion, got: %v", err)
	}
	if mockClient.callCount != 0 {
		t.Errorf("expected 0 LLM calls on validation error, got %d", mockClient.callCount)
	}
}

func TestGenerateVolumeIntents_BadJSON_Error(t *testing.T) {
	mockClient := &mockPlannerLLM{
		response: "抱歉，我无法以 JSON 输出，这是一段纯文本回复。",
	}
	orch := engine.NewOrchestrator(mockClient)

	project := &domain.Project{
		ID:    "proj-1",
		Title: "测试项目",
	}
	arc := domain.VolumeArc{
		VolumeIndex:  1,
		StartChapter: 1,
		EndChapter:   1,
	}

	_, _, err := orch.GenerateVolumeIntents(context.Background(), "deepseek-reasoner", project, arc, nil)
	if err == nil {
		t.Fatalf("expected error on malformed JSON, got nil")
	}
}

func TestGenerateVolumeIntents_BadCard_Tension11_Error(t *testing.T) {
	mockResponse := `[
  {
    "chapter_index": 1,
    "conflict": "测试冲突",
    "tension_target": 11
  }
]`
	mockClient := &mockPlannerLLM{
		response: mockResponse,
	}
	orch := engine.NewOrchestrator(mockClient)

	project := &domain.Project{
		ID:    "proj-1",
		Title: "测试项目",
	}
	arc := domain.VolumeArc{
		VolumeIndex:  1,
		StartChapter: 1,
		EndChapter:   1,
	}

	_, _, err := orch.GenerateVolumeIntents(context.Background(), "deepseek-reasoner", project, arc, nil)
	if err == nil {
		t.Fatalf("expected error on invalid card with tension 11, got nil")
	}
	if !strings.Contains(err.Error(), "tension_target") && !strings.Contains(err.Error(), "validate chapter intent") {
		t.Errorf("expected tension_target validation error, got: %v", err)
	}
}
