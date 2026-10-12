package engine_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type mockAuditLLM struct {
	callCount int
	response  string
	err       error
	usage     engine.TokenUsage
}

func (m *mockAuditLLM) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockAuditLLM) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, engine.TokenUsage, error) {
	m.callCount++
	if m.err != nil {
		return "", engine.TokenUsage{}, m.err
	}
	return m.response, m.usage, nil
}

func (m *mockAuditLLM) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (<-chan engine.StreamChunk, error) {
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

func testProject() *domain.Project {
	return &domain.Project{
		ID:             "proj_audit_1",
		Title:          "万界归一",
		TargetPlatform: "起点仙侠",
		WorldRules:     "弱肉强食，道法通神",
		Framework: &domain.ProjectFramework{
			ThemePremise: "凡人向道，以心代天",
			Logline:      "少年走出大荒，踏碎凌霄",
			StoryMilestones: []domain.StoryMilestone{
				{Key: "OPENING_HOOK", Name: "开局钩子", TargetChapter: 1, Note: "祖传灵戒苏醒"},
				{Key: "ACT1_TURN", Name: "第一幕转折", TargetChapter: 10, Note: "家族剧变被迫出逃"},
			},
		},
	}
}

func TestRunStructureAudit_EmptySummaries(t *testing.T) {
	orch := engine.NewOrchestrator(nil)
	llm := &mockAuditLLM{}
	orch.SetClient(llm)

	findings, usage, err := orch.RunStructureAudit(context.Background(), "test-model", testProject(), nil, nil, "")
	if err != nil {
		t.Fatalf("expected nil error on empty summaries, got: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got: %d", len(findings))
	}
	if llm.callCount != 0 {
		t.Fatalf("expected no LLM call for empty summaries, got: %d", llm.callCount)
	}
	if usage.TotalTokens != 0 {
		t.Fatalf("expected zero token usage, got: %v", usage)
	}
}

func TestRunStructureAudit_NilFailClosed(t *testing.T) {
	t.Run("nil client fails", func(t *testing.T) {
		orch := engine.NewOrchestrator(nil)
		_, _, err := orch.RunStructureAudit(context.Background(), "test-model", testProject(), []*domain.ChapterSummary{{ChapterIndex: 1, Text: "摘要"}}, nil, "")
		if err == nil {
			t.Fatal("expected error with nil client")
		}
	})

	t.Run("nil project fails", func(t *testing.T) {
		orch := engine.NewOrchestrator(nil)
		llm := &mockAuditLLM{}
		orch.SetClient(llm)
		_, _, err := orch.RunStructureAudit(context.Background(), "test-model", nil, []*domain.ChapterSummary{{ChapterIndex: 1, Text: "摘要"}}, nil, "")
		if err == nil {
			t.Fatal("expected error with nil project")
		}
	})
}

func TestRunStructureAudit_ParsingAndDiscardInvalid(t *testing.T) {
	mockJSON := `[
		{
			"type": "PLOT_HOLE",
			"severity": "HIGH",
			"chapter_start": 3,
			"chapter_end": 5,
			"summary": "第3章设定的金丹法则在第5章自相矛盾",
			"evidence": "第3章提到不可越级，第5章筑基斩金丹",
			"suggestion": "补充越级战斗的神器消耗代价设定"
		},
		{
			"type": "INVALID_TYPE",
			"severity": "HIGH",
			"chapter_start": 1,
			"chapter_end": 2,
			"summary": "非法类型项应被丢弃",
			"evidence": "无",
			"suggestion": "无"
		},
		{
			"type": "PACING",
			"severity": "HIGH",
			"chapter_start": 10,
			"chapter_end": 5,
			"summary": "起止倒置项应被丢弃",
			"evidence": "倒置",
			"suggestion": "修复"
		},
		{
			"type": "REDUNDANT",
			"severity": "MEDIUM",
			"chapter_start": 4,
			"chapter_end": 4,
			"summary": "   ",
			"evidence": "空摘要项应被丢弃",
			"suggestion": "修复"
		},
		{
			"type": "ARC_DRIFT",
			"severity": "LOW",
			"chapter_start": 8,
			"chapter_end": 9,
			"summary": "主角性格突然转为圣母",
			"evidence": "前文杀伐果断，此处强行放走死敌",
			"suggestion": "改为借刀杀人或暗布追踪标记"
		}
	]`

	orch := engine.NewOrchestrator(nil)
	llm := &mockAuditLLM{
		response: "```json\n" + mockJSON + "\n```",
		usage:    engine.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
	}
	orch.SetClient(llm)

	summaries := []*domain.ChapterSummary{
		{ChapterIndex: 1, Text: "开局大荒遭遇妖兽袭击"},
		{ChapterIndex: 2, Text: "偶遇隐世高人指点迷津"},
	}
	hooks := []*domain.PlotHook{
		{ID: "h1", Title: "青铜残片之谜", CreatedChapter: 1, TargetChapter: 10, Status: domain.HookStatusOpen, Details: "残片夜间发光"},
	}

	findings, usage, err := orch.RunStructureAudit(context.Background(), "test-model", testProject(), summaries, hooks, "Bench: 纯净")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got: %d", usage.TotalTokens)
	}

	// 5 条输入中，3 条非法（非法类型、起止倒置、空摘要），应仅保留 2 条
	if len(findings) != 2 {
		t.Fatalf("expected exactly 2 valid findings, got: %d", len(findings))
	}

	if findings[0].Type != domain.StructureFindingTypePlotHole || findings[0].Severity != domain.StructureFindingSeverityHigh {
		t.Errorf("first finding type/severity unexpected: %+v", findings[0])
	}
	if findings[0].ProjectID != "proj_audit_1" {
		t.Errorf("expected project ID proj_audit_1, got: %s", findings[0].ProjectID)
	}
	if findings[0].Status != domain.StructureFindingStatusOpen {
		t.Errorf("expected status OPEN, got: %s", findings[0].Status)
	}

	if findings[1].Type != domain.StructureFindingTypeArcDrift || findings[1].Severity != domain.StructureFindingSeverityLow {
		t.Errorf("second finding type/severity unexpected: %+v", findings[1])
	}
}

func TestRunStructureAudit_Cap20(t *testing.T) {
	var items []string
	for i := 1; i <= 25; i++ {
		items = append(items, fmt.Sprintf(`{
			"type": "DROPPED_HOOK",
			"severity": "MEDIUM",
			"chapter_start": %d,
			"chapter_end": %d,
			"summary": "伏笔测试项 %d 长期未收",
			"evidence": "第 %d 章提及后无下文",
			"suggestion": "在后续章节推进或结算"
		}`, i, i, i, i))
	}
	mockJSON := "[" + strings.Join(items, ",") + "]"

	orch := engine.NewOrchestrator(nil)
	llm := &mockAuditLLM{
		response: mockJSON,
	}
	orch.SetClient(llm)

	summaries := []*domain.ChapterSummary{
		{ChapterIndex: 1, Text: "首章测试"},
	}

	findings, _, err := orch.RunStructureAudit(context.Background(), "test-model", testProject(), summaries, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(findings) != 20 {
		t.Fatalf("expected findings capped at 20, got: %d", len(findings))
	}
}
