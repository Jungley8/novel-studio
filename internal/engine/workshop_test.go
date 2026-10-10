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
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:        "codex_p-ws_pro",
				ProjectID: "p-ws",
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
				Name:      "林凡",
				Realm:     "练气圆满",
				Inventory: "青玉短剑",
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

	// Test C: When user provides explicit InitialDraft, it MUST override checkpoint's stale text
	mockStoreWithCP.cp = savedCheckpoint
	reqUserDraft := WorkshopProduceRequest{
		ProjectID:        "p-cp",
		ChapterIndex:     2,
		ResumeCheckpoint: true,
		InitialDraft:     "这是用户在工坊中手动精修修改的最新稿子，必须以此为准。",
	}
	resUserDraft, err := workshop.ProduceChapter(context.Background(), reqUserDraft)
	if err != nil {
		t.Fatalf("ProduceChapter with user initial draft failed: %v", err)
	}
	if resUserDraft.Content != "这是用户在工坊中手动精修修改的最新稿子，必须以此为准。" {
		t.Errorf("expected user draft to override checkpoint, got: %s", resUserDraft.Content)
	}
}

type mockCheckpointWorkshopStore struct {
	*mockChronicleStore
	cp *domain.ChapterCheckpoint
}

func (m *mockCheckpointWorkshopStore) SaveCheckpoint(ctx context.Context, checkpoint *domain.ChapterCheckpoint) error {
	m.cp = checkpoint
	return nil
}

func (m *mockCheckpointWorkshopStore) GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error) {
	return m.cp, nil
}

func (m *mockCheckpointWorkshopStore) ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error {
	m.cp = nil
	return nil
}

type fakeQualityGateFailClosedLLMClient struct {
	beatsJSON   string
	renderDraft string
	reviewJSON  string
}

func (f *fakeQualityGateFailClosedLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	// Review routing FIRST: division prompts (分庭总审编) contain the substring
	// "剧情节拍" in their point-1 text, so beats matching must not take precedence.
	if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
		return f.reviewJSON, nil
	}
	if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
		return f.beatsJSON, nil
	}
	return f.renderDraft, nil
}

func (f *fakeQualityGateFailClosedLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 80, CompletionTokens: 160, TotalTokens: 240}, err
}

func (f *fakeQualityGateFailClosedLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 80, CompletionTokens: 160, TotalTokens: 240}}
	close(ch)
	return ch, nil
}

func TestChapterWorkshop_AutoCommit_FailClosedHaltWhenRevisionNeeded(t *testing.T) {
	// Fake client that constantly returns REVISION_NEEDED 75 points wrapped in Markdown code fence
	// to exercise ExtractAndCleanJSON.
	revisionJSON := "```json\n" + `{
		"verdict": "REVISION_NEEDED",
		"score": 75,
		"issues": ["主线冲突推进偏软，反转张力未达及格标准"],
		"suggestions": "加强断章悬念与正面交锋"
	}` + "\n```"

	fakeClient := &fakeQualityGateFailClosedLLMClient{
		beatsJSON: `{
			"beats": [{"phase": "破局", "tension": 7, "action": "反杀", "expectation_broken": "反派未料主角有底牌"}],
			"state_mutation": {"inventory_delta": "+密令"}
		}`,
		renderDraft: "夜色渐深，冷风呼啸。林凡拔出青玉长剑，一剑断去黑衣人退路。",
		reviewJSON:  revisionJSON,
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-failclosed",
				Title:          "停线自卫测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	var progressEvents []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-failclosed",
		ChapterIndex:    1,
		CoreConflict:    "初入险境",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
		OnProgress: func(ev WorkshopEvent) {
			progressEvents = append(progressEvents, ev)
		},
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 1. Assert chapter is NOT committed to repository
	if res.Committed {
		t.Errorf("expected Committed == false when verdict is REVISION_NEEDED (score: 75), got true")
	}
	if len(mockStore.chapters) != 0 {
		t.Errorf("expected 0 chapters committed into store, got %d", len(mockStore.chapters))
	}

	// 2. Assert verdict and score are strictly preserved without soft-flipping
	if res.Audit.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected verdict REVISION_NEEDED, got %s", res.Audit.Verdict)
	}
	if res.Audit.Score != 75 {
		t.Errorf("expected score 75, got %d", res.Audit.Score)
	}
	if res.RewriteLoops != 1 {
		t.Errorf("expected RewriteLoops 1, got %d", res.RewriteLoops)
	}
	if res.Audit.NeedsHuman != true {
		t.Errorf("expected res.Audit.NeedsHuman == true on fail-closed halt, got %v", res.Audit.NeedsHuman)
	}

	// 3. Assert machine-readable escalation marker is appended to Issues
	const expectedMarker = "【ESCALATED】已达最大重写轮次仍未通过，已停线待人工，拒绝自动入库"
	hasMarker := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, expectedMarker) {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		t.Errorf("expected machine-readable marker %q in audit issues, got: %v", expectedMarker, res.Audit.Issues)
	}

	// 4. Assert checkpoint is preserved for human intervention (Fail-Closed)
	cp, cpErr := mockStore.GetCheckpoint(context.Background(), "p-failclosed", 1)
	if cpErr != nil || cp == nil {
		t.Fatalf("expected checkpoint to be preserved for human pickup, got nil (err: %v)", cpErr)
	}
	if cp.ChapterIndex != 1 {
		t.Errorf("expected checkpoint chapter index 1, got %d", cp.ChapterIndex)
	}

	// 5. Assert OnProgress emitted PhaseComplete with "停线待人工" and valid TotalTokens
	var haltEvent *WorkshopEvent
	for _, ev := range progressEvents {
		if ev.Phase == PhaseComplete && strings.Contains(ev.Message, "停线待人工") {
			haltEvent = &ev
			break
		}
	}
	if haltEvent == nil {
		t.Errorf("expected OnProgress to emit PhaseComplete event containing '停线待人工', events: %+v", progressEvents)
	} else if haltEvent.TotalTokens <= 0 {
		t.Errorf("expected halt event to report non-zero TotalTokens, got %d", haltEvent.TotalTokens)
	}
}

func TestChapterWorkshop_AutoCommit_AcceptedPassThrough(t *testing.T) {
	// Fake client that returns ACCEPTED 90 points wrapped in Markdown code fence
	acceptedJSON := "```json\n" + `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "节奏明快，文风契合"
	}` + "\n```"

	fakeClient := &fakeQualityGateFailClosedLLMClient{
		beatsJSON: `{
			"beats": [{"phase": "破局", "tension": 8, "action": "反杀", "expectation_broken": "反派溃败"}],
			"state_mutation": {"inventory_delta": "+战利品"}
		}`,
		renderDraft: "夜幕降临，剑光纵横。林凡手起剑落，击退来犯之敌。",
		reviewJSON:  acceptedJSON,
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-accepted",
				Title:          "通过入库测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	var progressEvents []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-accepted",
		ChapterIndex:    1,
		CoreConflict:    "初入险境",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
		OnProgress: func(ev WorkshopEvent) {
			progressEvents = append(progressEvents, ev)
		},
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 1. Assert chapter is committed
	if !res.Committed {
		t.Errorf("expected Committed == true when verdict is ACCEPTED, got false")
	}
	if len(mockStore.chapters) != 1 {
		t.Errorf("expected 1 chapter committed into store, got %d", len(mockStore.chapters))
	}

	// 2. Assert verdict is ACCEPTED
	if res.Audit.Verdict != domain.ReviewVerdictAccepted {
		t.Errorf("expected verdict ACCEPTED, got %s", res.Audit.Verdict)
	}
	if res.Audit.NeedsHuman != false {
		t.Errorf("expected res.Audit.NeedsHuman == false on accepted pass-through, got %v", res.Audit.NeedsHuman)
	}

	// 3. Assert checkpoint is cleared after successful commit
	cp, _ := mockStore.GetCheckpoint(context.Background(), "p-accepted", 1)
	if cp != nil {
		t.Errorf("expected checkpoint to be cleared after commit, got %v", cp)
	}

	// 4. Assert PhaseCommitted was emitted
	hasCommittedEvent := false
	for _, ev := range progressEvents {
		if ev.Phase == PhaseCommitted {
			hasCommittedEvent = true
			break
		}
	}
	if !hasCommittedEvent {
		t.Errorf("expected PhaseCommitted progress event to be emitted")
	}
}
