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
	settleJSON  string
}

func (m *mockWorkshopLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		if m.settleJSON != "" {
			return m.settleJSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
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
	settleJSON  string
}

func (f *fakeQualityGateFailClosedLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		if f.settleJSON != "" {
			return f.settleJSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
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

// ---------------------------------------------------------------------------
// Settler Integration Tests
// ---------------------------------------------------------------------------

// Test Case ①: 消耗未持有道具的 draft → 阻塞不归档，断点保留在 PhaseDrafted
func TestChapterWorkshop_Settler_UnownedItemBlocked(t *testing.T) {
	fakeClient := &fakeQualityGateFailClosedLLMClient{
		beatsJSON: `{
			"beats": [{"phase": "破局", "tension": 7, "action": "对敌", "expectation_broken": "反派惊讶"}],
			"state_mutation": {"inventory_delta": "+战利品"}
		}`,
		renderDraft: "生死危机之下，林凡毫不犹豫咬破舌尖，吞服下救命的万年续命金丹，强行稳住心脉。",
		reviewJSON: "```json\n" + `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "通过"
		}` + "\n```",
		settleJSON: `{
			"inventory_delta": "-万年续命金丹",
			"power_delta": "",
			"character_mutations": [],
			"evidence": [
				{
					"field": "inventory",
					"quote": "吞服下救命的万年续命金丹，强行稳住心脉",
					"chapter_index": 1
				}
			]
		}`,
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-case1",
				Title:          "未持有道具消耗测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
			codexEntries: []*domain.CodexEntry{
				{
					ID:           "c-prot-1",
					ProjectID:    "p-case1",
					Category:     domain.CategoryCharacter,
					Archetype:    "PROTAGONIST",
					TrackingMode: domain.TrackingModeAlwaysInject,
					Name:         "林凡",
					Realm:        "练气三层",
					Inventory:    "粗布道袍x1, 铁木剑x1", // 不持有 万年续命金丹
				},
			},
		},
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-case1",
		ChapterIndex:    1,
		CoreConflict:    "绝境求生",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 1. Assert TechnicalBlocked is true and chapter is NOT committed
	if !res.TechnicalBlocked {
		t.Errorf("expected TechnicalBlocked == true for consuming unowned item, got false")
	}
	if res.Committed {
		t.Errorf("expected Committed == false, got true")
	}
	if len(mockStore.chapters) != 0 {
		t.Errorf("expected 0 chapters committed into store, got %d", len(mockStore.chapters))
	}

	// 2. Assert checkpoint is preserved at PhaseDrafted
	cp, cpErr := mockStore.GetCheckpoint(context.Background(), "p-case1", 1)
	if cpErr != nil || cp == nil {
		t.Fatalf("expected checkpoint to be preserved at PhaseDrafted, got nil (err: %v)", cpErr)
	}
	if cp.Phase != domain.CheckpointPhaseDrafted {
		t.Errorf("expected checkpoint phase %s, got %s", domain.CheckpointPhaseDrafted, cp.Phase)
	}
}

// Test Case ②: 正文与 beats 预测矛盾时 commit 用的是结算值不是预测值（并在重写后重新结算）
func TestChapterWorkshop_Settler_OverridesBeatsMutation_AndReSettlesOnRewrite(t *testing.T) {
	client := &fakeRewriteSettlerClient{
		beatsJSON: `{
			"beats": [{"phase": "破局", "tension": 7, "action": "寻宝", "expectation_broken": "反派落空"}],
			"state_mutation": {"inventory_delta": "+降龙木 (beats预言)"}
		}`,
		draft1Text: "林凡破开石门禁制，在石室角落里寻得一枚白玉灵瓶，将其收入囊中。",
		draft1SettleJSON: `{
			"inventory_delta": "+白玉灵瓶",
			"power_delta": "",
			"character_mutations": [],
			"evidence": [
				{
					"field": "inventory",
					"quote": "在石室角落里寻得一枚白玉灵瓶",
					"chapter_index": 1
				}
			]
		}`,
		draft1ReviewJSON: "```json\n" + `{
			"verdict": "REVISION_NEEDED",
			"score": 70,
			"issues": ["剧情平淡，必须采摘万年灵芝"],
			"suggestions": "重写为采摘万年灵芝"
		}` + "\n```",
		draft2Text: "林凡一剑荡平妖兽，在崖壁之上采下绝品万年灵芝，香气扑鼻。",
		draft2SettleJSON: `{
			"inventory_delta": "+万年灵芝",
			"power_delta": "",
			"character_mutations": [],
			"evidence": [
				{
					"field": "inventory",
					"quote": "在崖壁之上采下绝品万年灵芝",
					"chapter_index": 1
				}
			]
		}`,
		draft2ReviewJSON: "```json\n" + `{
			"verdict": "ACCEPTED",
			"score": 92,
			"issues": [],
			"suggestions": "合格"
		}` + "\n```",
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-case2",
				Title:          "结算覆盖与重写再结算测试",
				TargetPlatform: "起点仙侠",
			},
			chapters: []*domain.Chapter{},
			codexEntries: []*domain.CodexEntry{
				{
					ID:           "c-prot-2",
					ProjectID:    "p-case2",
					Category:     domain.CategoryCharacter,
					Archetype:    "PROTAGONIST",
					TrackingMode: domain.TrackingModeAlwaysInject,
					Name:         "林凡",
					Realm:        "练气期",
					Inventory:    "青灵丹x5",
				},
			},
		},
	}

	orch := NewOrchestrator(client)
	qGate := NewQualityGate(client, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-case2",
		ChapterIndex:    1,
		CoreConflict:    "秘境夺宝",
		AutoCommit:      true,
		MaxRewriteLoops: 2,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to be committed")
	}
	if len(mockStore.chapters) != 1 {
		t.Fatalf("expected 1 chapter committed, got %d", len(mockStore.chapters))
	}

	committedChapter := mockStore.chapters[0]

	// 1. Assert committed mutation uses settled value (+万年灵芝), NOT beats prediction (+降龙木)
	if committedChapter.StateMutation.InventoryDelta == "+降龙木 (beats预言)" {
		t.Errorf("committed mutation hallucinated beats prediction '+降龙木 (beats预言)' instead of settled text")
	}

	// 2. Assert committed mutation re-settled after rewrite (+万年灵芝), NOT old draft's mutation (+白玉灵瓶)
	if committedChapter.StateMutation.InventoryDelta != "+万年灵芝" {
		t.Errorf("expected committed mutation '+万年灵芝' (from re-settle on revised draft), got %q", committedChapter.StateMutation.InventoryDelta)
	}
	if res.StateMutation.InventoryDelta != "+万年灵芝" {
		t.Errorf("expected result StateMutation '+万年灵芝', got %q", res.StateMutation.InventoryDelta)
	}
}

type fakeRewriteSettlerClient struct {
	beatsJSON        string
	draft1Text       string
	draft1SettleJSON string
	draft1ReviewJSON string
	draft2Text       string
	draft2SettleJSON string
	draft2ReviewJSON string
}

func (c *fakeRewriteSettlerClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "精修专家") {
		return c.draft2Text, nil
	}
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") {
		if strings.Contains(userPrompt, "万年灵芝") {
			return c.draft2SettleJSON, nil
		}
		return c.draft1SettleJSON, nil
	}
	if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
		if strings.Contains(userPrompt, "万年灵芝") {
			return c.draft2ReviewJSON, nil
		}
		return c.draft1ReviewJSON, nil
	}
	if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") {
		return c.beatsJSON, nil
	}
	return c.draft1Text, nil
}

func (c *fakeRewriteSettlerClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := c.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 80, CompletionTokens: 160, TotalTokens: 240}, err
}

func (c *fakeRewriteSettlerClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := c.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 80, CompletionTokens: 160, TotalTokens: 240}}
	close(ch)
	return ch, nil
}

// Test Case ③: 配角名匹配失败 → unmatched_names 出现且不报错中断
func TestChapterWorkshop_Settler_UnmatchedCharacterNames(t *testing.T) {
	fakeClient := &fakeQualityGateFailClosedLLMClient{
		beatsJSON: `{
			"beats": [{"phase": "交锋", "tension": 8, "action": "击退敌人", "expectation_broken": "敌人溃败"}],
			"state_mutation": {"inventory_delta": ""}
		}`,
		renderDraft: "夜战密林。李四道心受损，狂吐鲜血不止，骇然退避百丈。",
		reviewJSON: "```json\n" + `{
			"verdict": "ACCEPTED",
			"score": 88,
			"issues": [],
			"suggestions": "合格"
		}` + "\n```",
		settleJSON: `{
			"inventory_delta": "",
			"power_delta": "",
			"character_mutations": [
				{
					"name": "李四",
					"status_delta": "道心受损，吐血倒退",
					"relation_delta": "骇然退避"
				}
			],
			"evidence": [
				{
					"field": "character:李四",
					"quote": "李四道心受损，狂吐鲜血不止",
					"chapter_index": 1
				}
			]
		}`,
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-case3",
				Title:          "配角名未匹配测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "c-chara-zhangsan",
					ProjectID: "p-case3",
					Category:  domain.CategoryCharacter,
					Name:      "张三", // 仅有张三，没有李四
				},
			},
		},
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-case3",
		ChapterIndex:    1,
		CoreConflict:    "遭遇战",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("expected ProduceChapter to complete without error, got: %v", err)
	}

	// 1. Assert unmatched_names contains "李四"
	if len(res.UnmatchedNames) != 1 || res.UnmatchedNames[0] != "李四" {
		t.Errorf("expected UnmatchedNames to contain '李四', got %v", res.UnmatchedNames)
	}

	// 2. Assert Audit Issues contains notice about unmatched name
	hasWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "李四") && strings.Contains(issue, "unmatched_name") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected Audit.Issues to contain warning about '李四', got: %v", res.Audit.Issues)
	}

	// 3. Assert production did NOT abort, chapter was committed successfully
	if !res.Committed {
		t.Errorf("expected chapter to be committed despite unmatched character name, got Committed=false")
	}
}

// Test Case ④: Evidence 缺失的结算输出 → 拒绝归档入库
func TestChapterWorkshop_Settler_MissingEvidenceBlocked(t *testing.T) {
	fakeClient := &fakeQualityGateFailClosedLLMClient{
		beatsJSON: `{
			"beats": [{"phase": "修整", "tension": 5, "action": "打坐", "expectation_broken": "风平浪静"}],
			"state_mutation": {"inventory_delta": "+九天息壤"}
		}`,
		renderDraft: "林凡盘坐调息，周身灵气汇聚如云。",
		reviewJSON: "```json\n" + `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "通过"
		}` + "\n```",
		settleJSON: `{
			"inventory_delta": "+九天息壤",
			"power_delta": "",
			"character_mutations": [],
			"evidence": []
		}`,
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-case4",
				Title:          "证据缺失拒绝测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
		},
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-case4",
		ChapterIndex:    1,
		CoreConflict:    "静室修炼",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 1. Assert TechnicalBlocked is true and chapter is NOT committed
	if !res.TechnicalBlocked {
		t.Errorf("expected TechnicalBlocked == true for missing evidence, got false")
	}
	if res.Committed {
		t.Errorf("expected Committed == false, got true")
	}
	if len(mockStore.chapters) != 0 {
		t.Errorf("expected 0 chapters committed, got %d", len(mockStore.chapters))
	}

	// 2. Assert checkpoint preserved at PhaseDrafted
	cp, cpErr := mockStore.GetCheckpoint(context.Background(), "p-case4", 1)
	if cpErr != nil || cp == nil {
		t.Fatalf("expected checkpoint preserved, got nil (err: %v)", cpErr)
	}
	if cp.Phase != domain.CheckpointPhaseDrafted {
		t.Errorf("expected checkpoint phase %s, got %s", domain.CheckpointPhaseDrafted, cp.Phase)
	}
}
