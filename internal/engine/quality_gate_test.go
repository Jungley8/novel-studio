package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockQualityReviewerClient struct {
	response string
}

func (m *mockQualityReviewerClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	return m.response, nil
}

func (m *mockQualityReviewerClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	return m.response, TokenUsage{PromptTokens: 50, CompletionTokens: 50, TotalTokens: 100}, nil
}

func (m *mockQualityReviewerClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: m.response}
	ch <- StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func TestQualityGate_Audit(t *testing.T) {
	mockResp := `{
		"verdict": "ACCEPTED",
		"score": 88,
		"issues": ["配角台词略显生硬"],
		"suggestions": "缩短配角发言长度"
	}`

	client := &mockQualityReviewerClient{response: mockResp}
	gate := NewQualityGate(client, nil)

	proj := &domain.Project{
		Title:          "修真模拟器",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "不可跨大境界逆伐",
		Protagonist: domain.Protagonist{
			NameAndLevel: "林凡 (练气一层)",
			Inventory:    "粗布衣, 基础飞剑",
		},
	}

	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 5, Action: "遭遇挑衅", ExpectationBroken: "反派以为主角会退缩"},
	}

	// 1. Audit clean text
	cleanDraft := "长街尽头。杀机乍现！林凡握紧飞剑。寒芒一闪，对手的护体罡气应声碎裂！"
	report, usage, err := gate.Audit(context.Background(), "mock-reviewer", proj, 1, beats, cleanDraft)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if report.Verdict != domain.ReviewVerdictAccepted {
		t.Errorf("expected ACCEPTED, got %s", report.Verdict)
	}
	if report.Score != 88 {
		t.Errorf("expected score 88, got %d", report.Score)
	}
	if usage.TotalTokens != 100 {
		t.Errorf("expected usage 100 tokens, got %d", usage.TotalTokens)
	}

	// 2. Audit text containing cliché (must penalize and flip to REVISION_NEEDED)
	clicheDraft := "他不由得冷笑了一声，嘴角勾起一丝嘲讽的笑意。对手一时间眼中闪过一丝慌乱。"
	reportCliche, _, err := gate.Audit(context.Background(), "mock-reviewer", proj, 1, beats, clicheDraft)
	if err != nil {
		t.Fatalf("Audit with cliches failed: %v", err)
	}
	if reportCliche.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected REVISION_NEEDED when cliches hit, got %s", reportCliche.Verdict)
	}
	if len(reportCliche.HitBannedWords) == 0 {
		t.Errorf("expected hit banned words")
	}
	foundClicheIssue := false
	for _, issue := range reportCliche.Issues {
		if strings.Contains(issue, "命中AI套词") {
			foundClicheIssue = true
			break
		}
	}
	if !foundClicheIssue {
		t.Errorf("expected cliché in issues list, got %v", reportCliche.Issues)
	}

	// 3. Test LLM rejection preservation: LLM gives REVISION_NEEDED even with score >= 80 and clean text
	client.response = `{
		"verdict": "REVISION_NEEDED",
		"score": 85,
		"issues": ["主角未持有辟毒珠却免疫尸毒，剧情逻辑出现硬伤"],
		"suggestions": "请在上一段补充主角服用辟毒丹或借用阵法"
	}`
	reportRejection, _, err := gate.Audit(context.Background(), "mock-reviewer", proj, 1, beats, cleanDraft)
	if err != nil {
		t.Fatalf("Audit rejection test failed: %v", err)
	}
	if reportRejection.Verdict != domain.ReviewVerdictRevision {
		t.Fatalf("CRITICAL: LLM REVISION_NEEDED verdict was overwritten to %s despite score 85!", reportRejection.Verdict)
	}

	// 4. Verify structural metrics are populated on clean report
	if report.BurstinessScore <= 0 {
		t.Errorf("expected non-zero burstiness score")
	}
	if report.ExclamationDensity <= 0 {
		t.Errorf("expected non-zero exclamation density")
	}
}
