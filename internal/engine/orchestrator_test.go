package engine_test

import (
	"context"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestOrchestrator_DeriveBeats(t *testing.T) {
	mockJSON := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 5, "action": "反派拦路叫嚣", "expectation_broken": "主角面色平静"},
			{"phase": "绝地反转", "tension": 9, "action": "主角一剑封喉", "expectation_broken": "反派不可置信"}
		],
		"state_mutation": {
			"inventory_delta": "消耗长剑",
			"power_delta": "略有领悟"
		}
	}`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{
		Title:          "剑道独尊",
		TargetPlatform: "番茄脑洞",
		Protagonist: domain.Protagonist{
			NameAndLevel: "陆青 (练气五层)",
		},
	}

	out, err := orch.DeriveBeats(context.Background(), "deepseek-reasoner", p, 1, "同门挑衅", nil)
	if err != nil {
		t.Fatalf("DeriveBeats failed: %v", err)
	}

	if len(out.Beats) != 2 {
		t.Errorf("expected 2 beats, got %d", len(out.Beats))
	}
	if out.StateMutation.InventoryDelta != "消耗长剑" {
		t.Errorf("unexpected state mutation: %v", out.StateMutation)
	}
}

func TestOrchestrator_RenderScene(t *testing.T) {
	mock := &mockLLMClient{response: "青云峰上，寒风如刀。陆青抬起眼皮，指尖微屈。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "剑道独尊"}
	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Action: "风雪封山"},
	}

	content, err := orch.RenderScene(context.Background(), "deepseek-chat", p, 1, beats, 1500)
	if err != nil {
		t.Fatalf("RenderScene failed: %v", err)
	}

	if content != "青云峰上，寒风如刀。陆青抬起眼皮，指尖微屈。" {
		t.Errorf("unexpected content: %s", content)
	}
}
