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

func (m *mockLLMClient) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, engine.TokenUsage, error) {
	if m.err != nil {
		return "", engine.TokenUsage{}, m.err
	}
	return m.response, engine.TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}, nil
}

func (m *mockLLMClient) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (<-chan engine.StreamChunk, error) {
	ch := make(chan engine.StreamChunk, 2)
	ch <- engine.StreamChunk{Delta: m.response}
	ch <- engine.StreamChunk{Done: true, Usage: &engine.TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}}
	close(ch)
	return ch, nil
}

func TestOrchestrator_DeriveBeats(t *testing.T) {
	mockJSON := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 5, "action": "反派拦路叫嚣", "expectation_broken": "主角面色平静", "reader_emotion": "紧张压抑", "info_gap": "反派不知主角已突破"},
			{"phase": "绝地反转", "tension": 9, "action": "主角一剑封喉", "expectation_broken": "反派不可置信", "reader_emotion": "大呼解气", "hook_type": "CLIFFHANGER"}
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
	if out.Beats[0].ReaderEmotion != "紧张压抑" {
		t.Errorf("expected reader emotion 紧张压抑, got %s", out.Beats[0].ReaderEmotion)
	}
	if out.Beats[1].HookType != "CLIFFHANGER" {
		t.Errorf("expected hook type CLIFFHANGER, got %s", out.Beats[1].HookType)
	}
	if out.StateMutation.InventoryDelta != "消耗长剑" {
		t.Errorf("unexpected state mutation: %v", out.StateMutation)
	}
}

func TestOrchestrator_RenderScene(t *testing.T) {
	mock := &mockLLMClient{response: "青云峰上，寒风如刀。陆青抬起眼皮，指尖微屈。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "剑道独尊", TargetPlatform: "知乎盐言"}
	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 8, Action: "风雪封山"},
	}

	content, err := orch.RenderScene(context.Background(), "deepseek-chat", p, 1, beats, 1500)
	if err != nil {
		t.Fatalf("RenderScene failed: %v", err)
	}

	if content != "青云峰上，寒风如刀。陆青抬起眼皮，指尖微屈。" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestOrchestrator_RenderSceneWithHorizon(t *testing.T) {
	mock := &mockLLMClient{response: "漫天风雪呼啸。陆青迎风而立。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "剑道独尊", TargetPlatform: "番茄脑洞"}
	horizon := &engine.CanonHorizon{
		Project:       p,
		TargetChapter: 2,
		TailAnchor:    "寒风卷起漫天飞雪，山门前悄然立着一道冷冽黑影。",
	}
	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 6, Action: "风雪封山"},
		{Phase: "章末留钩", Tension: 9, Action: "剑气破空", HookType: "CLIFFHANGER"},
	}

	content, err := orch.RenderSceneWithHorizon(context.Background(), "deepseek-chat", horizon, beats, 2000)
	if err != nil {
		t.Fatalf("RenderSceneWithHorizon failed: %v", err)
	}

	if content != "漫天风雪呼啸。陆青迎风而立。" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestOrchestrator_ReviewDraft(t *testing.T) {
	mockJSON := `{
		"verdict": "REVISION_NEEDED",
		"score": 72,
		"issues": ["主角未携带玄重尺却施展了该兵器技能"],
		"suggestions": "改为徒手拳法"
	}`
	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "斗破苍穹"}
	rev, err := orch.ReviewDraft(context.Background(), "deepseek-reasoner", p, 1, nil, "草稿正文...")
	if err != nil {
		t.Fatalf("ReviewDraft failed: %v", err)
	}

	if rev.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected REVISION_NEEDED, got %s", rev.Verdict)
	}
	if rev.Score != 72 {
		t.Errorf("expected score 72, got %d", rev.Score)
	}
	if len(rev.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(rev.Issues))
	}
}

func TestOrchestrator_RewriteDraft(t *testing.T) {
	mock := &mockLLMClient{response: "重修后的正文：陆青翻掌成印，呼啸破风。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "斗破苍穹"}
	rev := &domain.ReviewResult{
		Verdict:     domain.ReviewVerdictRevision,
		Score:       70,
		Issues:      []string{"需增加拳法动作"},
		Suggestions: "改为八极崩",
	}

	rewritten, err := orch.RewriteDraft(context.Background(), "deepseek-chat", p, 1, "旧草稿", rev)
	if err != nil {
		t.Fatalf("RewriteDraft failed: %v", err)
	}

	if rewritten != "重修后的正文：陆青翻掌成印，呼啸破风。" {
		t.Errorf("unexpected rewritten text: %s", rewritten)
	}
}
