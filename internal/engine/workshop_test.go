package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockWorkshopLLMClient struct {
	beatsJSON   string
	renderDraft string
	reviewJSON  string
}

func (m *mockWorkshopLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "总设计师") {
		return m.beatsJSON, nil
	}
	if strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") {
		return m.reviewJSON, nil
	}
	return m.renderDraft, nil
}

func TestChapterWorkshop_ProduceChapter(t *testing.T) {
	beatsResp := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 6, "action": "反派带队堵截主角洞府", "expectation_broken": "反派预料主角会求饶"},
			{"phase": "绝地反转", "tension": 9, "action": "主角引动护山大阵反扣杀", "expectation_broken": "反派全部被镇压"}
		],
		"state_mutation": {
			"inventory_delta": "+战利品储物袋",
			"power_delta": "威名初显"
		}
	}`

	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "节奏明快，表现良好"
	}`

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   beatsResp,
		renderDraft: "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
		reviewJSON:  reviewResp,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-ws",
			Title:          "万域封神",
			TargetPlatform: "番茄脑洞",
			WorldRules:     "杀生夺灵，因果必报",
			Protagonist: domain.Protagonist{
				NameAndLevel: "林凡 (练气圆满)",
				Inventory:    "青玉短剑",
			},
		},
		chapters: []*domain.Chapter{},
		hooks:    []*domain.PlotHook{},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-ws",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭抢夺古经",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-v3",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      true,
		MaxRewriteLoops: 2,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if len(res.Beats) != 2 {
		t.Errorf("expected 2 beats, got %d", len(res.Beats))
	}
	if res.Audit.Verdict != domain.ReviewVerdictAccepted {
		t.Errorf("expected ACCEPTED, got %s", res.Audit.Verdict)
	}
	if !res.Committed {
		t.Errorf("expected chapter to be auto-committed")
	}
	if !strings.Contains(res.Content, "阵纹骤亮") {
		t.Errorf("unexpected content: %s", res.Content)
	}
}
