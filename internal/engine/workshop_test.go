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

func (m *mockWorkshopLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}, err
}

func (m *mockWorkshopLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}}
	close(ch)
	return ch, nil
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

	var events []WorkshopEvent
	req.OnProgress = func(ev WorkshopEvent) {
		events = append(events, ev)
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
	if res.TotalUsage.TotalTokens == 0 {
		t.Errorf("expected non-zero total tokens")
	}
	if len(events) == 0 {
		t.Errorf("expected progress events to be emitted")
	}
}

func TestChapterWorkshop_ResumeCheckpoint(t *testing.T) {
	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   `{"beats": [], "state_mutation": {}}`,
		renderDraft: "新草稿内容",
		reviewJSON:  `{"verdict": "ACCEPTED", "score": 95, "issues": []}`,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:    "p-cp",
			Title: "断点测试",
		},
	}

	// Pre-populate mock checkpoint with drafted text
	savedCheckpoint := &domain.ChapterCheckpoint{
		ProjectID:    "p-cp",
		ChapterIndex: 2,
		Phase:        domain.CheckpointPhaseDrafted,
		Beats: []domain.SceneBeat{
			{Phase: "绝地反转", Action: "反击破敌"},
		},
		DraftText: "断点恢复的既有正文内容，无需重新消耗Token渲染。",
	}

	mockStoreWithCP := &mockCheckpointWorkshopStore{
		mockChronicleStore: mockStore,
		cp:                 savedCheckpoint,
	}

	chronicle := NewCanonChronicle(mockStoreWithCP)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStoreWithCP)

	req := WorkshopProduceRequest{
		ProjectID:        "p-cp",
		ChapterIndex:     2,
		ResumeCheckpoint: true,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter with checkpoint failed: %v", err)
	}

	if res.Content != "断点恢复的既有正文内容，无需重新消耗Token渲染。" {
		t.Errorf("expected resumed content, got %s", res.Content)
	}
	if res.ResumedPhase != string(domain.CheckpointPhaseDrafted) {
		t.Errorf("expected resumed phase %s, got %s", domain.CheckpointPhaseDrafted, res.ResumedPhase)
	}

	// Test B: When ResumeCheckpoint is false, clear stale checkpoint and generate afresh
	reqFresh := WorkshopProduceRequest{
		ProjectID:        "p-cp",
		ChapterIndex:     2,
		ResumeCheckpoint: false,
	}
	resFresh, err := workshop.ProduceChapter(context.Background(), reqFresh)
	if err != nil {
		t.Fatalf("ProduceChapter without checkpoint failed: %v", err)
	}
	if resFresh.ResumedPhase != "" {
		t.Errorf("expected no resumed phase when ResumeCheckpoint is false, got %s", resFresh.ResumedPhase)
	}
	if resFresh.Content != "新草稿内容" {
		t.Errorf("expected fresh content '新草稿内容', got %s", resFresh.Content)
	}
}

type mockCheckpointWorkshopStore struct {
	*mockChronicleStore
	cp *domain.ChapterCheckpoint
}

func (m *mockCheckpointWorkshopStore) GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error) {
	return m.cp, nil
}

func (m *mockCheckpointWorkshopStore) ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error {
	m.cp = nil
	return nil
}
