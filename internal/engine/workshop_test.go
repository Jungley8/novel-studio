package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

type mockWorkshopLLMClient struct {
	beatsJSON            string
	renderDraft          string
	reviewJSON           string
	settleJSON           string
	suggestConflictText  string
	suggestConflictCalls int
	capturedBeatsPrompt  string
}

func (m *mockWorkshopLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		if m.settleJSON != "" {
			return m.settleJSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
	if strings.Contains(systemPrompt, "白金级剧情架构师") || strings.Contains(systemPrompt, "高潮推演专家") {
		m.suggestConflictCalls++
		if m.suggestConflictText != "" {
			return m.suggestConflictText, nil
		}
		return "推演的黄金冲突", nil
	}
	if strings.Contains(systemPrompt, "记忆压缩器") {
		return "", nil
	}
	if strings.Contains(systemPrompt, "总设计师") {
		m.capturedBeatsPrompt = userPrompt
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
	cp            *domain.ChapterCheckpoint
	activePlan    *domain.ActivePlan
	activePlanErr error
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

func (m *mockCheckpointWorkshopStore) GetActivePlan(ctx context.Context, projectID string) (*domain.ActivePlan, error) {
	if m.activePlanErr != nil {
		return nil, m.activePlanErr
	}
	return m.activePlan, nil
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

func TestChapterWorkshop_RhythmIntegration(t *testing.T) {
	beatsResp := `{
		"beats": [
			{"phase": "铺垫蓄势", "tension": 4, "action": "打坐调息，感应天地灵气"},
			{"phase": "波澜微起", "tension": 5, "action": "院外传来急促叩门声"},
			{"phase": "悬念收尾", "tension": 5, "action": "信封上赫然印着血色剑纹", "hook_type": "MYSTERY"}
		],
		"state_mutation": {
			"inventory_delta": "+血色信笺",
			"power_delta": ""
		}
	}`

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   beatsResp,
		renderDraft: "夜凉如水。林凡盘膝而坐，忽然叩门声急响。他拾起信笺，血色剑纹刺目惊心。",
		reviewJSON:  `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":""}`,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	baseStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-rhythm-int",
			Title:          "通天之路",
			TargetPlatform: "番茄脑洞",
		},
		chapters: []*domain.Chapter{},
	}
	mockStore := &mockCheckpointWorkshopStore{mockChronicleStore: baseStore}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-rhythm-int",
		ChapterIndex:    1,
		CoreConflict:    "神秘来信",
		AutoCommit:      false,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if len(res.Beats) != 3 {
		t.Errorf("expected 3 beats derived according to chapter type, got %d", len(res.Beats))
	}

	// Verify checkpoint saved final beats
	cp, cpErr := mockStore.GetCheckpoint(context.Background(), "p-rhythm-int", 1)
	if cpErr != nil || cp == nil {
		t.Fatalf("expected checkpoint saved, got nil: %v", cpErr)
	}
	if len(cp.Beats) != 3 {
		t.Errorf("expected checkpoint beats to match final beats count (3), got %d", len(cp.Beats))
	}
}

type mockSummaryWorkshopStore struct {
	*mockChronicleStore
	cp               *domain.ChapterCheckpoint
	chapterSummaries map[int]*domain.ChapterSummary
	volumeRetros     map[int]*domain.VolumeRetrospective
	saveSummaryErr   error
	saveRetroErr     error
	listSummaryErr   error
}

func newMockSummaryWorkshopStore(proj *domain.Project) *mockSummaryWorkshopStore {
	return &mockSummaryWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project:      proj,
			chapters:     []*domain.Chapter{},
			hooks:        []*domain.PlotHook{},
			codexEntries: []*domain.CodexEntry{},
		},
		chapterSummaries: make(map[int]*domain.ChapterSummary),
		volumeRetros:     make(map[int]*domain.VolumeRetrospective),
	}
}

func (m *mockSummaryWorkshopStore) SaveCheckpoint(ctx context.Context, checkpoint *domain.ChapterCheckpoint) error {
	m.cp = checkpoint
	return nil
}

func (m *mockSummaryWorkshopStore) GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error) {
	return m.cp, nil
}

func (m *mockSummaryWorkshopStore) ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error {
	m.cp = nil
	return nil
}

func (m *mockSummaryWorkshopStore) SaveChapterSummary(ctx context.Context, cs *domain.ChapterSummary) error {
	if m.saveSummaryErr != nil {
		return m.saveSummaryErr
	}
	cp := *cs
	m.chapterSummaries[cs.ChapterIndex] = &cp
	return nil
}

func (m *mockSummaryWorkshopStore) GetChapterSummary(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterSummary, error) {
	if s, ok := m.chapterSummaries[chapterIndex]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (m *mockSummaryWorkshopStore) ListChapterSummaries(ctx context.Context, projectID string, from, to int) ([]*domain.ChapterSummary, error) {
	if m.listSummaryErr != nil {
		return nil, m.listSummaryErr
	}
	var res []*domain.ChapterSummary
	for idx, s := range m.chapterSummaries {
		if (from <= 0 || idx >= from) && (to <= 0 || idx <= to) {
			cp := *s
			res = append(res, &cp)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].ChapterIndex < res[j].ChapterIndex
	})
	return res, nil
}

func (m *mockSummaryWorkshopStore) SaveVolumeRetrospective(ctx context.Context, retro *domain.VolumeRetrospective) error {
	if m.saveRetroErr != nil {
		return m.saveRetroErr
	}
	cp := *retro
	m.volumeRetros[retro.VolumeIndex] = &cp
	return nil
}

func (m *mockSummaryWorkshopStore) GetVolumeRetrospective(ctx context.Context, projectID string, volumeIndex int) (*domain.VolumeRetrospective, error) {
	if r, ok := m.volumeRetros[volumeIndex]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, nil
}

type fakeSummaryWorkshopLLMClient struct {
	beatsJSON       string
	renderDraft     string
	reviewJSON      string
	settleJSON      string
	summaryJSON     string
	summaryErr      error
	volumeRetroJSON string
	volumeRetroErr  error
	summaryCalled   bool
	retroCalled     bool
}

func (f *fakeSummaryWorkshopLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "长期记忆") || strings.Contains(systemPrompt, "章节剧情摘要") {
		f.summaryCalled = true
		if f.summaryErr != nil {
			return "", f.summaryErr
		}
		if f.summaryJSON != "" {
			return f.summaryJSON, nil
		}
		return "```json\n" + `{
			"characters": "林凡",
			"conflict": "迎击强敌夜袭",
			"result": "引动护山大阵反杀敌寇",
			"suspense": "幕后黑手尚未现身",
			"summary": "林凡在洞府前遭遇反派夜袭，果断引动护山大阵反杀镇压敌手，缴获战利品储物袋，然幕后黑手尚未现身。"
		}` + "\n```", nil
	}
	if strings.Contains(systemPrompt, "分卷剧情复盘") || strings.Contains(systemPrompt, "剧情回顾") || strings.Contains(systemPrompt, "宏观剧情架构师") {
		f.retroCalled = true
		if f.volumeRetroErr != nil {
			return "", f.volumeRetroErr
		}
		if f.volumeRetroJSON != "" {
			return f.volumeRetroJSON, nil
		}
		return "```json\n" + `{
			"happened": "林凡初入仙门连破死局，于宗门大比夺魁并反杀执法堂叛逆",
			"remaining": "成功晋升内门并获古经认可，但得罪了宗门隐世太上长老",
			"unresolved": "太上长老暗中勾结魔道，乱星海魔灾爆发的预兆已然浮现",
			"text": "【发生】林凡初入仙门连破死局，于宗门大比夺魁并反杀执法堂叛逆\\n【留下】成功晋升内门并获古经认可，但得罪了宗门隐世太上长老\\n【未回收】太上长老暗中勾结魔道，乱星海魔灾爆发的预兆已然浮现"
		}` + "\n```", nil
	}
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		if f.settleJSON != "" {
			return f.settleJSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
	if strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
		if f.reviewJSON != "" {
			return f.reviewJSON, nil
		}
		return "```json\n" + `{
			"verdict": "ACCEPTED",
			"score": 92,
			"issues": [],
			"suggestions": "合格"
		}` + "\n```", nil
	}
	if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
		if f.beatsJSON != "" {
			return f.beatsJSON, nil
		}
		return `{
			"beats": [
				{"phase": "蓄力压迫", "tension": 6, "action": "反派带队堵截主角洞府", "expectation_broken": "反派预料主角会求饶"},
				{"phase": "绝地反转", "tension": 9, "action": "主角引动护山大阵反扣杀", "expectation_broken": "反派全部被镇压"}
			],
			"state_mutation": {
				"inventory_delta": "",
				"power_delta": ""
			}
		}`, nil
	}
	if f.renderDraft != "" {
		return f.renderDraft, nil
	}
	return "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！缴获战利品。", nil
}

func (f *fakeSummaryWorkshopLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 60, CompletionTokens: 80, TotalTokens: 140}, err
}

func (f *fakeSummaryWorkshopLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 60, CompletionTokens: 80, TotalTokens: 140}}
	close(ch)
	return ch, nil
}

// 1. 测试要求：fake 下摘要落库章号正确
func TestChapterWorkshop_Summary_SaveChapterSummaryCorrectChapterIndex(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-summary-idx",
		Title:          "万域封神",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	fakeClient := &fakeSummaryWorkshopLLMClient{}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-summary-idx",
		ChapterIndex:    5,
		CoreConflict:    "洞府决战",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to be committed")
	}
	if !fakeClient.summaryCalled {
		t.Fatalf("expected SummarizeChapter to be invoked via fakeClient")
	}

	// 断言落库章号正确
	savedSummary, ok := mockStore.chapterSummaries[5]
	if !ok || savedSummary == nil {
		t.Fatalf("expected summary to be saved for chapter 5, got nil")
	}
	if savedSummary.ChapterIndex != 5 {
		t.Errorf("expected ChapterIndex == 5, got %d", savedSummary.ChapterIndex)
	}
	if savedSummary.ProjectID != "p-summary-idx" {
		t.Errorf("expected ProjectID == 'p-summary-idx', got %s", savedSummary.ProjectID)
	}
	if !strings.Contains(savedSummary.Text, "林凡") || !strings.Contains(savedSummary.Text, "反派") {
		t.Errorf("unexpected summary text: %s", savedSummary.Text)
	}
}

// 2. 测试要求：构造卷终章触发回顾
func TestChapterWorkshop_Summary_VolumeRetrospectiveOnVolumeEnd(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-vol-end",
		Title:          "仙凡道途",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "第一卷 仙门风云",
					CoreGoal:          "拜入宗门立足",
					Climax:            "大比夺魁斩叛逆",
					EstimatedChapters: 2, // 卷1起止为 1~2，终章为第2章
				},
				{
					VolumeIndex:       2,
					Title:             "第二卷 乱星海域",
					CoreGoal:          "渡海求药",
					Climax:            "镇杀乱星妖王",
					EstimatedChapters: 3, // 卷2起止为 3~5，终章为第5章
				},
			},
		},
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	fakeClient := &fakeSummaryWorkshopLLMClient{}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	// 第 1 章（非卷终章）：提交成功后不应触发卷回顾
	req1 := WorkshopProduceRequest{
		ProjectID:       "p-vol-end",
		ChapterIndex:    1,
		CoreConflict:    "入宗试炼",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res1, err := workshop.ProduceChapter(context.Background(), req1)
	if err != nil {
		t.Fatalf("ProduceChapter chapter 1 failed: %v", err)
	}
	if !res1.Committed {
		t.Fatalf("expected chapter 1 to be committed")
	}
	if mockStore.chapterSummaries[1] == nil {
		t.Fatalf("expected chapter 1 summary to be saved")
	}
	if fakeClient.retroCalled {
		t.Errorf("expected volume retrospective NOT to be called for chapter 1 (not EndChapter)")
	}
	if len(mockStore.volumeRetros) != 0 {
		t.Errorf("expected 0 volume retrospectives for chapter 1, got %d", len(mockStore.volumeRetros))
	}

	// 第 2 章（卷1终章）：提交成功后应触发本卷回顾
	req2 := WorkshopProduceRequest{
		ProjectID:       "p-vol-end",
		ChapterIndex:    2,
		CoreConflict:    "宗门大比夺魁",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res2, err := workshop.ProduceChapter(context.Background(), req2)
	if err != nil {
		t.Fatalf("ProduceChapter chapter 2 failed: %v", err)
	}
	if !res2.Committed {
		t.Fatalf("expected chapter 2 to be committed")
	}
	if mockStore.chapterSummaries[2] == nil {
		t.Fatalf("expected chapter 2 summary to be saved")
	}
	if !fakeClient.retroCalled {
		t.Fatalf("expected volume retrospective to be called for chapter 2 (EndChapter)")
	}

	retro, ok := mockStore.volumeRetros[1]
	if !ok || retro == nil {
		t.Fatalf("expected volume retrospective for volume 1 to be saved")
	}
	if retro.VolumeIndex != 1 {
		t.Errorf("expected VolumeIndex == 1, got %d", retro.VolumeIndex)
	}
	if retro.FromChapter != 1 || retro.ToChapter != 2 {
		t.Errorf("expected FromChapter=1, ToChapter=2, got %d~%d", retro.FromChapter, retro.ToChapter)
	}
	if !strings.Contains(retro.Text, "发生") || !strings.Contains(retro.Text, "留下") || !strings.Contains(retro.Text, "未回收") {
		t.Errorf("expected retro text to contain 发生/留下/未回收, got: %s", retro.Text)
	}
}

// 3. 测试要求：摘要 LLM 失败时提交仍成功且有告警
func TestChapterWorkshop_Summary_LLMFailureDoesNotBlockCommitWithWarning(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-summary-fail",
		Title:          "天道纪元",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	fakeClient := &fakeSummaryWorkshopLLMClient{
		summaryErr: errors.New("simulated LLM rate limit on chapter summary"),
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-summary-fail",
		ChapterIndex:    1,
		CoreConflict:    "绝地求生",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("expected ProduceChapter not to fail when summary fails, got err: %v", err)
	}

	// 1. 提交仍然成功
	if !res.Committed {
		t.Fatalf("expected chapter to be committed despite summary LLM failure, got Committed=false")
	}
	if len(mockStore.chapters) != 1 {
		t.Errorf("expected 1 chapter committed in store, got %d", len(mockStore.chapters))
	}

	// 2. 技术阻塞状态未受影响
	if res.TechnicalBlocked {
		t.Errorf("expected TechnicalBlocked == false, got true")
	}

	// 3. 摘要未落库
	if mockStore.chapterSummaries[1] != nil {
		t.Errorf("expected no chapter summary saved on failure, got %v", mockStore.chapterSummaries[1])
	}

	// 4. 产生【摘要警告】
	hasSummaryWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【摘要警告】") {
			hasSummaryWarning = true
			break
		}
	}
	if !hasSummaryWarning {
		t.Errorf("expected Audit.Issues to contain '【摘要警告】', got: %v", res.Audit.Issues)
	}

	// 5. 断点正常清除，没有残留或被错误保存
	if mockStore.cp != nil {
		t.Errorf("expected checkpoint to be cleared after commit, got: %+v", mockStore.cp)
	}
}

// 4. 陷阱校验：摘要 save 失败不准碰 checkpoint，且不阻断归档
func TestChapterWorkshop_Summary_SaveSummaryFailureDoesNotTouchCheckpoint(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-summary-save-fail",
		Title:          "万域封神",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	mockStore.saveSummaryErr = errors.New("simulated disk I/O error on SaveChapterSummary")
	fakeClient := &fakeSummaryWorkshopLLMClient{}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-summary-save-fail",
		ChapterIndex:    1,
		CoreConflict:    "洞府遇袭",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("expected ProduceChapter not to fail on SaveChapterSummary error, got: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to be committed despite SaveChapterSummary error")
	}

	// 摘要 save 失败不准碰 checkpoint，commit 后断点必须正常清除
	if mockStore.cp != nil {
		t.Errorf("expected checkpoint to be cleared after commit, got non-nil checkpoint: %+v", mockStore.cp)
	}

	// 必须记录摘要警告
	hasWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【摘要警告】") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected Audit.Issues to contain '【摘要警告】', got: %v", res.Audit.Issues)
	}
}

// 5. 测试要求：拒稿（REVISION_NEEDED）跑 ProduceChapter(AutoCommit=true) 未归档，断言 store 中无该章摘要
func TestChapterWorkshop_Summary_RejectedHasNoSummary(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-rejected-no-summary",
		Title:          "拒稿无摘要测试",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	fakeClient := &fakeSummaryWorkshopLLMClient{
		reviewJSON: "```json\n" + `{
			"verdict": "REVISION_NEEDED",
			"score": 70,
			"issues": ["情节拖沓，冲突平淡"],
			"suggestions": "请重写"
		}` + "\n```",
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-rejected-no-summary",
		ChapterIndex:    1,
		CoreConflict:    "初入险境",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.Committed {
		t.Fatalf("expected chapter NOT to be committed on REVISION_NEEDED, got Committed=true")
	}

	// 断言 store 里无该章摘要
	summary, err := mockStore.GetChapterSummary(context.Background(), "p-rejected-no-summary", 1)
	if err != nil {
		t.Fatalf("GetChapterSummary error: %v", err)
	}
	if summary != nil {
		t.Fatalf("expected nil chapter summary in store for rejected chapter, got: %+v", summary)
	}

	summaries, err := mockStore.ListChapterSummaries(context.Background(), "p-rejected-no-summary", 0, 0)
	if err != nil {
		t.Fatalf("ListChapterSummaries error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 chapter summaries in store for rejected chapter, got %d", len(summaries))
	}
	if len(mockStore.chapterSummaries) != 0 {
		t.Fatalf("expected 0 chapter summaries in mockStore map for rejected chapter, got %d", len(mockStore.chapterSummaries))
	}
}

// 6. 测试要求：AutoCommit=false 预演成功渲染 → 无摘要落库（预演不污染记忆）
func TestChapterWorkshop_Summary_AutoCommitFalseHasNoSummary(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-preview-no-summary",
		Title:          "预演不污染记忆测试",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	fakeClient := &fakeSummaryWorkshopLLMClient{}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-preview-no-summary",
		ChapterIndex:    1,
		CoreConflict:    "破局交锋",
		AutoCommit:      false,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.Committed {
		t.Fatalf("expected chapter NOT to be committed when AutoCommit=false, got Committed=true")
	}

	summary, err := mockStore.GetChapterSummary(context.Background(), "p-preview-no-summary", 1)
	if err != nil {
		t.Fatalf("GetChapterSummary error: %v", err)
	}
	if summary != nil {
		t.Fatalf("expected nil chapter summary for AutoCommit=false preview, got: %+v", summary)
	}
	if len(mockStore.chapterSummaries) != 0 {
		t.Fatalf("expected 0 chapter summaries in store for AutoCommit=false preview, got %d", len(mockStore.chapterSummaries))
	}
}

// 7. 测试要求：mutation 覆盖——失败路径的 TotalUsage 含摘要调用量（error + 非零 usage）
func TestChapterWorkshop_Summary_FailurePathTokenAccounting(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-summary-tokens",
		Title:          "摘要失败路径记账测试",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	// fakeClient 让 SummarizeChapter 返回非法 JSON，从而触发后处理解析错误并返回非零 usage
	fakeClient := &fakeSummaryWorkshopLLMClient{
		summaryJSON: "this is invalid json response from LLM",
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-summary-tokens",
		ChapterIndex:    1,
		CoreConflict:    "决战时刻",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to be committed despite summary post-processing error")
	}

	// 验证包含摘要失败告警
	hasWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【摘要警告】生成章节摘要失败") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected audit issues to contain summary warning, got: %v", res.Audit.Issues)
	}

	// 验证 token 记账：
	// 前序流程（beats + draft + ReviewCourt + settle）共 6 次调用（840 tokens）
	// 加上失败的章节摘要 1 次调用（140 tokens），总计 7 次调用 = 980 tokens
	const expectedTotalTokens = 980
	const expectedPromptTokens = 420
	const expectedCompletionTokens = 560
	if res.TotalUsage.TotalTokens != expectedTotalTokens {
		t.Errorf("expected TotalUsage.TotalTokens == %d (including failed summary usage), got %d", expectedTotalTokens, res.TotalUsage.TotalTokens)
	}
	if res.TotalUsage.PromptTokens != expectedPromptTokens {
		t.Errorf("expected TotalUsage.PromptTokens == %d, got %d", expectedPromptTokens, res.TotalUsage.PromptTokens)
	}
	if res.TotalUsage.CompletionTokens != expectedCompletionTokens {
		t.Errorf("expected TotalUsage.CompletionTokens == %d, got %d", expectedCompletionTokens, res.TotalUsage.CompletionTokens)
	}
}

// 8. 测试要求：分卷回顾失败路径的 TotalUsage 包含回顾调用量（error + 非零 usage）
func TestChapterWorkshop_VolumeRetrospective_FailurePathTokenAccounting(t *testing.T) {
	proj := &domain.Project{
		ID:             "p-retro-tokens",
		Title:          "卷回顾失败路径记账测试",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "第一卷 仙门风云",
					CoreGoal:          "拜入宗门立足",
					Climax:            "大比夺魁斩叛逆",
					EstimatedChapters: 1, // 第1章即为终章
				},
			},
		},
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	fakeClient := &fakeSummaryWorkshopLLMClient{
		volumeRetroJSON: "this is invalid json response from LLM for volume retrospective",
	}

	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-retro-tokens",
		ChapterIndex:    1,
		CoreConflict:    "卷终大决战",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to be committed")
	}

	hasRetroWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【摘要警告】生成分卷回顾失败") {
			hasRetroWarning = true
			break
		}
	}
	if !hasRetroWarning {
		t.Errorf("expected audit issues to contain retro failure warning, got: %v", res.Audit.Issues)
	}

	// 验证 token 记账：
	// 前序流程 6 次（840）+ 章节摘要 1 次（140）+ 失败的卷回顾 1 次（140）= 8 次调用 = 1120 tokens
	const expectedTotalTokens = 1120
	const expectedPromptTokens = 480
	const expectedCompletionTokens = 640
	if res.TotalUsage.TotalTokens != expectedTotalTokens {
		t.Errorf("expected TotalUsage.TotalTokens == %d (including failed volume retrospective usage), got %d", expectedTotalTokens, res.TotalUsage.TotalTokens)
	}
	if res.TotalUsage.PromptTokens != expectedPromptTokens {
		t.Errorf("expected TotalUsage.PromptTokens == %d, got %d", expectedPromptTokens, res.TotalUsage.PromptTokens)
	}
	if res.TotalUsage.CompletionTokens != expectedCompletionTokens {
		t.Errorf("expected TotalUsage.CompletionTokens == %d, got %d", expectedCompletionTokens, res.TotalUsage.CompletionTokens)
	}
}

// mockEchoListErrorStore wraps mockSummaryWorkshopStore to simulate ListChapters error during block 5.55.
type mockEchoListErrorStore struct {
	*mockSummaryWorkshopStore
	failAfterCalls int
	calls          int
}

func (m *mockEchoListErrorStore) ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error) {
	m.calls++
	if m.failAfterCalls > 0 && m.calls > m.failAfterCalls {
		return nil, errors.New("simulated database I/O error on ListChapters")
	}
	return m.mockSummaryWorkshopStore.ListChapters(ctx, projectID)
}

func makeEchoTestFakeClient(echoDraft string) *fakeSummaryWorkshopLLMClient {
	return &fakeSummaryWorkshopLLMClient{
		beatsJSON: "```json\n" + `{
			"beats": [
				{"phase": "蓄力压迫", "tension": 6, "action": "反派带队堵截洞府", "expectation_broken": "反派预料求饶"},
				{"phase": "绝地反转", "tension": 9, "action": "引动护山大阵反杀", "expectation_broken": "反派被镇压"}
			],
			"state_mutation": {"inventory_delta": "", "power_delta": ""}
		}` + "\n```",
		settleJSON: "```json\n" + `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}` + "\n```",
		reviewJSON: "```json\n" + `{
			"verdict": "ACCEPTED",
			"score": 92,
			"issues": [],
			"suggestions": "文风契合"
		}` + "\n```",
		summaryJSON: "```json\n" + `{
			"characters": "林凡",
			"conflict": "迎击夜袭",
			"result": "胜",
			"suspense": "",
			"summary": "林凡反杀敌手"
		}` + "\n```",
		renderDraft: echoDraft,
	}
}

// TestChapterWorkshop_CrossChapterEcho_GE3ChaptersTriggersWarning verifies that when >=3 chapters in
// recent 20 contain the same 4-gram and draft contains it, 【跨章重复警告】 is appended to Issues.
func TestChapterWorkshop_CrossChapterEcho_GE3ChaptersTriggersWarning(t *testing.T) {
	const echoPhrase = "剑拔弩张"
	const projectID = "p-echo-ge3"

	proj := &domain.Project{
		ID:             projectID,
		Title:          "万域封神",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)

	// Populate 20 preceding chapters: 17 neutral, 3 with echoPhrase
	for i := 1; i <= 20; i++ {
		content := "夜幕低垂，冷风呼啸，林凡盘膝闭目吐纳天地灵气。"
		if i >= 18 { // chapters 18, 19, 20 (3 chapters contain echoPhrase)
			content = "四周杀机毕露，气氛剑拔弩张，体内灵力奔涌澎湃。"
		}
		mockStore.chapters = append(mockStore.chapters, &domain.Chapter{
			ID:           fmt.Sprintf("ch_%s_%d", projectID, i),
			ProjectID:    projectID,
			ChapterIndex: i,
			Title:        fmt.Sprintf("第 %d 章", i),
			Content:      content,
		})
	}

	draft := "夜色渐深，杀机逼近。门前气氛剑拔弩张，林凡指尖轻扣石壁，大阵雷光轰然暴起！"
	fakeClient := makeEchoTestFakeClient(draft)
	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    21,
		CoreConflict:    "洞府遇袭决死破局",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.Committed {
		t.Errorf("expected chapter to be committed (echo warning is non-blocking)")
	}
	if res.TechnicalBlocked {
		t.Errorf("expected TechnicalBlocked == false")
	}
	if res.Audit == nil {
		t.Fatalf("expected non-nil Audit report")
	}

	expectedWarning := fmt.Sprintf("【跨章重复警告】短语「%s」在近 20 章中反复出现，建议换写。", echoPhrase)
	hasEchoWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, expectedWarning) {
			hasEchoWarning = true
			break
		}
	}
	if !hasEchoWarning {
		t.Errorf("expected Audit.Issues to contain %q, got: %v", expectedWarning, res.Audit.Issues)
	}
}

// TestChapterWorkshop_CrossChapterEcho_Only2ChaptersDoesNotWarn verifies that when only 2 chapters in
// recent 20 contain the 4-gram, no echo warning is issued.
func TestChapterWorkshop_CrossChapterEcho_Only2ChaptersDoesNotWarn(t *testing.T) {
	const echoPhrase = "剑拔弩张"
	const projectID = "p-echo-only2"

	proj := &domain.Project{
		ID:             projectID,
		Title:          "万域封神",
		TargetPlatform: "番茄脑洞",
	}
	mockStore := newMockSummaryWorkshopStore(proj)

	// Populate 20 preceding chapters: 18 neutral, 2 with echoPhrase
	for i := 1; i <= 20; i++ {
		content := "夜幕低垂，冷风呼啸，林凡盘膝闭目吐纳天地灵气。"
		if i >= 19 { // chapters 19, 20 (only 2 chapters contain echoPhrase)
			content = "四周杀机毕露，气氛剑拔弩张，体内灵力奔涌澎湃。"
		}
		mockStore.chapters = append(mockStore.chapters, &domain.Chapter{
			ID:           fmt.Sprintf("ch_%s_%d", projectID, i),
			ProjectID:    projectID,
			ChapterIndex: i,
			Title:        fmt.Sprintf("第 %d 章", i),
			Content:      content,
		})
	}

	draft := "夜色渐深，杀机逼近。门前气氛剑拔弩张，林凡指尖轻扣石壁，大阵雷光轰然暴起！"
	fakeClient := makeEchoTestFakeClient(draft)
	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    21,
		CoreConflict:    "洞府遇袭决死破局",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.Audit == nil {
		t.Fatalf("expected non-nil Audit report")
	}

	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【跨章重复警告】") {
			t.Errorf("expected no 【跨章重复警告】 when only 2 chapters hit, got: %s", issue)
		}
	}
}

// TestChapterWorkshop_CrossChapterEcho_ListChaptersErrorAppendsWarning verifies that when ListChapters
// fails, 【回声警告】检测失败 is appended to Issues without blocking chapter commit.
func TestChapterWorkshop_CrossChapterEcho_ListChaptersErrorAppendsWarning(t *testing.T) {
	const projectID = "p-echo-listerr"

	proj := &domain.Project{
		ID:             projectID,
		Title:          "万域封神",
		TargetPlatform: "番茄脑洞",
	}
	baseStore := newMockSummaryWorkshopStore(proj)
	mockStore := &mockEchoListErrorStore{
		mockSummaryWorkshopStore: baseStore,
		failAfterCalls:           2,
	}

	draft := "夜色渐深，杀机逼近。林凡指尖轻扣石壁，大阵雷光轰然暴起！"
	fakeClient := makeEchoTestFakeClient(draft)
	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    1,
		CoreConflict:    "洞府遇袭决死破局",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("expected ProduceChapter not to fail on ListChapters error, got: %v", err)
	}

	if !res.Committed {
		t.Errorf("expected chapter to be committed (echo warning is non-blocking)")
	}
	if res.Audit == nil {
		t.Fatalf("expected non-nil Audit report")
	}

	hasListWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【回声警告】检测失败") {
			hasListWarning = true
			break
		}
	}
	if !hasListWarning {
		t.Errorf("expected Audit.Issues to contain '【回声警告】检测失败', got: %v", res.Audit.Issues)
	}
}

func TestChapterWorkshop_PlanInjection(t *testing.T) {
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

	setupWorkshop := func(plan *domain.ActivePlan, planErr error) (*ChapterWorkshop, *mockWorkshopLLMClient, *[]WorkshopEvent) {
		mockClient := &mockWorkshopLLMClient{
			beatsJSON:   beatsResp,
			renderDraft: "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
			reviewJSON:  reviewResp,
		}
		orch := NewOrchestrator(mockClient)
		qGate := NewQualityGate(mockClient, nil)
		mockStore := &mockCheckpointWorkshopStore{
			mockChronicleStore: &mockChronicleStore{
				project: &domain.Project{
					ID:             "p-plan-inj",
					Title:          "万域封神",
					TargetPlatform: "番茄脑洞",
					WorldRules:     "杀生夺灵，因果必报",
				},
				codexEntries: []*domain.CodexEntry{
					{
						ID:        "codex_p-plan-inj_pro",
						ProjectID: "p-plan-inj",
						Category:  domain.CategoryCharacter,
						Archetype: "PROTAGONIST",
						Name:      "林凡",
						Realm:     "练气圆满",
						Inventory: "青玉短剑",
					},
				},
				chapters: []*domain.Chapter{},
				hooks:    []*domain.PlotHook{},
			},
			activePlan:    plan,
			activePlanErr: planErr,
		}
		chronicle := NewCanonChronicle(mockStore)
		workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)
		var events []WorkshopEvent
		return workshop, mockClient, &events
	}

	baseReq := func(events *[]WorkshopEvent) WorkshopProduceRequest {
		return WorkshopProduceRequest{
			ProjectID:       "p-plan-inj",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭抢夺古经",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      false,
			MaxRewriteLoops: 1,
			OnProgress: func(ev WorkshopEvent) {
				*events = append(*events, ev)
			},
		}
	}

	t.Run("case1_active_with_plantext", func(t *testing.T) {
		plan := &domain.ActivePlan{
			ProjectID: "p-plan-inj",
			Status:    domain.PlanStatusActive,
			PlanText:  "计划：主角假意投降魔教，实则借雷劫暗算魔尊",
		}
		ws, client, events := setupWorkshop(plan, nil)
		res, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}
		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}
		if !strings.Contains(client.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats userPrompt to contain '作者选定的发展路线', got: %s", client.capturedBeatsPrompt)
		}
		if !strings.Contains(client.capturedBeatsPrompt, "计划：主角假意投降魔教，实则借雷劫暗算魔尊") {
			t.Errorf("expected beats userPrompt to contain plan text, got: %s", client.capturedBeatsPrompt)
		}
		foundEvent := false
		for _, ev := range *events {
			if strings.Contains(ev.Message, "已注入作者选定的发展路线") {
				foundEvent = true
				break
			}
		}
		if !foundEvent {
			t.Errorf("expected progress event about plan injection to be emitted")
		}
	})

	t.Run("case2_nil_plan", func(t *testing.T) {
		ws, client, events := setupWorkshop(nil, nil)
		res, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}
		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}
		if strings.Contains(client.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats userPrompt NOT to contain candidate plan when plan is nil, got: %s", client.capturedBeatsPrompt)
		}
	})

	t.Run("case3_stale_or_consumed_plan", func(t *testing.T) {
		planStale := &domain.ActivePlan{
			ProjectID: "p-plan-inj",
			Status:    domain.PlanStatusStale,
			PlanText:  "已过期计划内容",
		}
		ws, client, events := setupWorkshop(planStale, nil)
		_, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}
		if strings.Contains(client.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats userPrompt NOT to contain stale plan, got: %s", client.capturedBeatsPrompt)
		}

		planConsumed := &domain.ActivePlan{
			ProjectID: "p-plan-inj",
			Status:    domain.PlanStatusConsumed,
			PlanText:  "已消费计划内容",
		}
		ws2, client2, events2 := setupWorkshop(planConsumed, nil)
		_, err = ws2.ProduceChapter(context.Background(), baseReq(events2))
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}
		if strings.Contains(client2.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats userPrompt NOT to contain consumed plan, got: %s", client2.capturedBeatsPrompt)
		}
	})

	t.Run("case4_getactiveplan_error", func(t *testing.T) {
		planErr := errors.New("simulated store disk failure")
		ws, client, events := setupWorkshop(nil, planErr)
		res, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err != nil {
			t.Fatalf("ProduceChapter must succeed when GetActivePlan fails, got err: %v", err)
		}
		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}
		if strings.Contains(client.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats userPrompt NOT to contain plan when store errored, got: %s", client.capturedBeatsPrompt)
		}
		hasWarning := false
		for _, issue := range res.Audit.Issues {
			if strings.Contains(issue, "【计划注入警告】") && strings.Contains(issue, "simulated store disk failure") {
				hasWarning = true
				break
			}
		}
		if !hasWarning {
			t.Errorf("expected Audit.Issues to contain plan injection warning, got: %v", res.Audit.Issues)
		}
	})
}

func TestChapterWorkshop_ChapterIntent_Integration(t *testing.T) {
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

	setup := func(intent *domain.ChapterIntent) (*ChapterWorkshop, *mockWorkshopLLMClient, *mockChronicleStore) {
		client := &mockWorkshopLLMClient{
			beatsJSON:   beatsResp,
			renderDraft: "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
			reviewJSON:  reviewResp,
		}
		orch := NewOrchestrator(client)
		qGate := NewQualityGate(client, nil)
		store := &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-intent",
				Title:          "万域封神",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀生夺灵，因果必报",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-intent_pro",
					ProjectID: "p-intent",
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
		if intent != nil {
			_ = store.SaveChapterIntent(context.Background(), intent)
		}
		chronicle := NewCanonChronicle(store)
		ws := NewChapterWorkshop(orch, chronicle, qGate, store)
		return ws, client, store
	}

	t.Run("case1_intent_conflict_auto_beats_prompt_and_no_suggest", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:     "p-intent",
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "拍卖会暗杀",
			TensionTarget: 8,
			Status:        domain.IntentStatusLocked,
		}
		ws, client, _ := setup(intent)

		var events []WorkshopEvent
		res, err := ws.ProduceChapter(context.Background(), WorkshopProduceRequest{
			ProjectID:       "p-intent",
			ChapterIndex:    1,
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
			OnProgress: func(ev WorkshopEvent) {
				events = append(events, ev)
			},
		})
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		// 1. beats prompt contains "【本章意图卡" and "拍卖会暗杀"
		if !strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected beats prompt to contain '【本章意图卡', got: %s", client.capturedBeatsPrompt)
		}
		if !strings.Contains(client.capturedBeatsPrompt, "拍卖会暗杀") {
			t.Errorf("expected beats prompt to contain '拍卖会暗杀', got: %s", client.capturedBeatsPrompt)
		}

		// 2. result.CoreConflict == "拍卖会暗杀"
		if res.CoreConflict != "拍卖会暗杀" {
			t.Errorf("expected res.CoreConflict '拍卖会暗杀', got: %q", res.CoreConflict)
		}

		// 3. no suggest LLM call occurred
		if client.suggestConflictCalls != 0 {
			t.Errorf("expected 0 suggestConflictCalls, got: %d", client.suggestConflictCalls)
		}

		// Verify event was emitted
		foundEmit := false
		for _, ev := range events {
			if strings.Contains(ev.Message, "本章冲突取自意图卡：拍卖会暗杀") {
				foundEmit = true
				break
			}
		}
		if !foundEmit {
			t.Errorf("expected event '本章冲突取自意图卡：拍卖会暗杀' to be emitted")
		}
	})

	t.Run("case2_priority_explicit_conflict_overrides_intent", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:     "p-intent",
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "拍卖会暗杀",
			TensionTarget: 8,
			Status:        domain.IntentStatusLocked,
		}
		ws, client, _ := setup(intent)

		res, err := ws.ProduceChapter(context.Background(), WorkshopProduceRequest{
			ProjectID:       "p-intent",
			ChapterIndex:    1,
			CoreConflict:    "手动指定冲突",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
		})
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if res.CoreConflict != "手动指定冲突" {
			t.Errorf("expected res.CoreConflict '手动指定冲突', got: %q", res.CoreConflict)
		}
		if !strings.Contains(client.capturedBeatsPrompt, "【本章必须打破的核心冲突】\n手动指定冲突") {
			t.Errorf("expected beats prompt to contain explicit core conflict, got: %s", client.capturedBeatsPrompt)
		}
	})

	t.Run("case3_no_intent_regression_zero_behavior_change", func(t *testing.T) {
		ws, client, _ := setup(nil)

		res, err := ws.ProduceChapter(context.Background(), WorkshopProduceRequest{
			ProjectID:       "p-intent",
			ChapterIndex:    1,
			CoreConflict:    "常规剧情冲突",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
		})
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected beats prompt NOT to contain '【本章意图卡' when intent is nil, got: %s", client.capturedBeatsPrompt)
		}
		if res.CoreConflict != "常规剧情冲突" {
			t.Errorf("expected res.CoreConflict '常规剧情冲突', got: %q", res.CoreConflict)
		}
	})

	t.Run("case4_stale_intent_skipped_with_warning", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:     "p-intent",
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "旧版过时冲突",
			TensionTarget: 6,
			Status:        domain.IntentStatusStale,
		}
		ws, client, _ := setup(intent)

		res, err := ws.ProduceChapter(context.Background(), WorkshopProduceRequest{
			ProjectID:       "p-intent",
			ChapterIndex:    1,
			CoreConflict:    "常规冲突",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
		})
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}
		expectedWarning := "【意图卡警告】第 1 章意图卡已过期（STALE，上游卷纲已变更），已跳过注入；请重推意图卡并重新锁定后再生产"
		hasWarning := false
		for _, iss := range res.Audit.Issues {
			if strings.Contains(iss, expectedWarning) {
				hasWarning = true
				break
			}
		}
		if !hasWarning {
			t.Errorf("expected Audit.Issues to contain %q, got: %v", expectedWarning, res.Audit.Issues)
		}
		if strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected beats prompt NOT to contain '【本章意图卡', got: %s", client.capturedBeatsPrompt)
		}
		if strings.Contains(client.capturedBeatsPrompt, "旧版过时冲突") {
			t.Errorf("expected beats prompt NOT to contain '旧版过时冲突', got: %s", client.capturedBeatsPrompt)
		}
		if res.CoreConflict != "常规冲突" {
			t.Errorf("expected res.CoreConflict '常规冲突', got: %q", res.CoreConflict)
		}
	})

	t.Run("case5_draft_intent_skipped_with_hint", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:     "p-intent",
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "草稿冲突",
			TensionTarget: 6,
			Status:        domain.IntentStatusDraft,
		}
		ws, client, _ := setup(intent)

		res, err := ws.ProduceChapter(context.Background(), WorkshopProduceRequest{
			ProjectID:       "p-intent",
			ChapterIndex:    1,
			CoreConflict:    "常规冲突",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
		})
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}
		expectedHint := "【意图卡提示】第 1 章意图卡为草稿（DRAFT，未锁定），已跳过注入；锁定后本章节拍才会对齐该意图"
		hasHint := false
		for _, iss := range res.Audit.Issues {
			if strings.Contains(iss, expectedHint) {
				hasHint = true
				break
			}
		}
		if !hasHint {
			t.Errorf("expected Audit.Issues to contain %q, got: %v", expectedHint, res.Audit.Issues)
		}
		if strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected beats prompt NOT to contain '【本章意图卡', got: %s", client.capturedBeatsPrompt)
		}
		if strings.Contains(client.capturedBeatsPrompt, "草稿冲突") {
			t.Errorf("expected beats prompt NOT to contain '草稿冲突', got: %s", client.capturedBeatsPrompt)
		}
		if res.CoreConflict != "常规冲突" {
			t.Errorf("expected res.CoreConflict '常规冲突', got: %q", res.CoreConflict)
		}
	})

	t.Run("case6_stale_intent_must_not_hijack_template_conflict", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:     "p-intent",
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "旧版过时冲突",
			TensionTarget: 6,
			Status:        domain.IntentStatusStale,
		}
		ws, client, _ := setup(intent)
		client.suggestConflictText = "智能推演的常规冲突"

		res, err := ws.ProduceChapter(context.Background(), WorkshopProduceRequest{
			ProjectID:       "p-intent",
			ChapterIndex:    1,
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
		})
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if res.CoreConflict != "智能推演的常规冲突" {
			t.Errorf("expected res.CoreConflict '智能推演的常规冲突', got: %q", res.CoreConflict)
		}
		if client.suggestConflictCalls < 1 {
			t.Errorf("expected client.suggestConflictCalls >= 1, got: %d", client.suggestConflictCalls)
		}
		if strings.Contains(client.capturedBeatsPrompt, "旧版过时冲突") {
			t.Errorf("expected capturedBeatsPrompt NOT to contain '旧版过时冲突', got: %s", client.capturedBeatsPrompt)
		}
		if strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected capturedBeatsPrompt NOT to contain '【本章意图卡', got: %s", client.capturedBeatsPrompt)
		}
	})
}

func TestBuildChapterIntentPromptBlock(t *testing.T) {
	t.Run("case5_full_fields_and_emotion_tension", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:     "p-full",
			ChapterIndex:  5,
			VolumeIndex:   1,
			Conflict:      "拍卖会暗杀与夺宝",
			POV:           "林凡",
			EmotionCurve:  []string{"压抑", "震惊", "亢奋"},
			TensionTarget: 8,
			HookPlants:    []string{"黑衣人遗留令牌", "拍卖行暗印"},
			HookPayoffs:   []string{"此前获得的残破阵盘"},
			ExpectedMutation: domain.StateMutation{
				InventoryDelta: "获得残破阵盘，消耗灵石百枚",
				PowerDelta:     "神念突破",
				CharacterMutations: []domain.CharacterMutation{
					{
						Name:        "赵执事",
						StatusDelta: "重伤濒死",
					},
				},
			},
			Status: domain.IntentStatusLocked,
		}

		block := buildChapterIntentPromptBlock(intent)

		expectedSubstrings := []string{
			"【本章意图卡（作者已锁定，节拍必须服务该意图）】",
			"核心冲突：拍卖会暗杀与夺宝",
			"POV 视角：林凡",
			"情绪曲线：压抑 → 震惊 → 亢奋",
			"张力目标：8/10",
			"埋设伏笔：黑衣人遗留令牌、拍卖行暗印",
			"回收伏笔：此前获得的残破阵盘",
			"预期状态变迁（作者预期，仅供参考；严禁为迁就预期扭曲因果与人物行为，最终以写后结算为准）：",
			`"inventory_delta":"获得残破阵盘，消耗灵石百枚"`,
			`"power_delta":"神念突破"`,
			`"name":"赵执事"`,
			"铁律：冲突不得偏离此意图；与正史、状态机、世界公理冲突时以正史为准，并在节拍中体现调整与过渡。",
		}

		for _, sub := range expectedSubstrings {
			if !strings.Contains(block, sub) {
				t.Errorf("expected block to contain %q, but got:\n%s", sub, block)
			}
		}
	})

	t.Run("case5_omitted_empty_fields", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:    "p-minimal",
			ChapterIndex: 2,
			Conflict:     "极简冲突",
		}

		block := buildChapterIntentPromptBlock(intent)

		if !strings.Contains(block, "【本章意图卡（作者已锁定，节拍必须服务该意图）】") {
			t.Errorf("expected block to contain title")
		}
		if !strings.Contains(block, "核心冲突：极简冲突") {
			t.Errorf("expected block to contain conflict")
		}
		if !strings.Contains(block, "铁律：冲突不得偏离此意图；与正史、状态机、世界公理冲突时以正史为准，并在节拍中体现调整与过渡。") {
			t.Errorf("expected block to contain iron law")
		}

		omittedPrefixes := []string{
			"POV 视角：",
			"情绪曲线：",
			"张力目标：",
			"埋设伏笔：",
			"回收伏笔：",
			"预期状态变迁：",
		}
		for _, prefix := range omittedPrefixes {
			if strings.Contains(block, prefix) {
				t.Errorf("expected block NOT to contain %q when field is empty, but got:\n%s", prefix, block)
			}
		}
	})

	t.Run("nil_intent_returns_empty", func(t *testing.T) {
		if got := buildChapterIntentPromptBlock(nil); got != "" {
			t.Errorf("expected empty string for nil intent, got %q", got)
		}
	})
}

func TestChapterWorkshop_TechnicalBlocked_SetsNeedsHuman(t *testing.T) {
	beatsResp := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 6, "action": "反派带队堵截主角洞府", "expectation_broken": "反派预料主角会求饶"},
			{"phase": "绝地反转", "tension": 9, "action": "主角引动护山大阵反扣杀", "expectation_broken": "反派全部被镇压"}
		],
		"state_mutation": {
			"inventory_delta": "",
			"power_delta": ""
		}
	}`

	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "质量达标"
	}`

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   beatsResp,
		renderDraft: "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
		reviewJSON:  reviewResp,
		settleJSON:  "{unparseable-json-response",
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-tech-halt",
				Title:          "技术阻断停线测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
			codexEntries: []*domain.CodexEntry{
				{
					ID:           "c-prot-1",
					ProjectID:    "p-tech-halt",
					Category:     domain.CategoryCharacter,
					Archetype:    "PROTAGONIST",
					TrackingMode: domain.TrackingModeAlwaysInject,
					Name:         "林凡",
					Realm:        "练气圆满",
					Inventory:    "青玉短剑",
				},
			},
		},
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-tech-halt",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭抢夺古经",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.TechnicalBlocked {
		t.Fatalf("expected TechnicalBlocked == true, got false")
	}
	if res.Committed {
		t.Errorf("expected Committed == false, got true")
	}
	if res.Audit == nil || !res.Audit.NeedsHuman {
		t.Fatalf("expected res.Audit.NeedsHuman == true on technical block, got %v", res.Audit)
	}

	hasHaltMarker := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "技术阻断停线") {
			hasHaltMarker = true
			break
		}
	}
	if !hasHaltMarker {
		t.Errorf("expected audit issues to contain '技术阻断停线', got %v", res.Audit.Issues)
	}

	if len(mockStore.chapters) != 0 {
		t.Errorf("expected 0 chapters committed into store, got %d", len(mockStore.chapters))
	}
}

// TestChapterWorkshop_RealSQLiteStore_VolumeRetrospectiveOnVolumeEnd 验证使用真实 *store.SQLiteStore
// 且 VolumeArcs 仅有 EstimatedChapters（StartChapter/EndChapter 为 0）时，
// 经 ProduceChapter 到卷终章后分卷回顾能击穿指针别名并真实落库持久化。
func TestChapterWorkshop_RealSQLiteStore_VolumeRetrospectiveOnVolumeEnd(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "real_retro_test.db")
	realStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create real SQLiteStore: %v", err)
	}
	defer realStore.Close()

	projectID := "proj-real-retro"
	proj := &domain.Project{
		ID:             projectID,
		Title:          "修真大道",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "第一卷 筑基篇",
					CoreGoal:          "夺丹筑基",
					Climax:            "力斩强敌破劫入道",
					EstimatedChapters: 2, // 1~2, 终章为第2章（Start/End 留空验证 BackfillVolumeArcs）
				},
				{
					VolumeIndex:       2,
					Title:             "第二卷 金丹篇",
					CoreGoal:          "凝结金丹",
					Climax:            "金丹大成",
					EstimatedChapters: 3,
				},
			},
		},
	}
	if err := realStore.SaveProject(ctx, proj); err != nil {
		t.Fatalf("failed to save project: %v", err)
	}

	fakeClient := &fakeSummaryWorkshopLLMClient{}
	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(realStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, realStore)

	// 第 1 章：提交成功，但不触发分卷回顾
	req1 := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    1,
		CoreConflict:    "初入仙门",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res1, err := workshop.ProduceChapter(ctx, req1)
	if err != nil {
		t.Fatalf("ProduceChapter chapter 1 failed: %v", err)
	}
	if !res1.Committed {
		t.Fatalf("expected chapter 1 to be committed")
	}

	retro1, err := realStore.GetVolumeRetrospective(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("GetVolumeRetrospective error: %v", err)
	}
	if retro1 != nil {
		t.Fatalf("expected no retrospective for chapter 1, got %+v", retro1)
	}

	// 第 2 章（卷1终章）：提交成功，触发分卷回顾并真实落库！
	req2 := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    2,
		CoreConflict:    "筑基大战",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res2, err := workshop.ProduceChapter(ctx, req2)
	if err != nil {
		t.Fatalf("ProduceChapter chapter 2 failed: %v", err)
	}
	if !res2.Committed {
		t.Fatalf("expected chapter 2 to be committed")
	}

	// 核心断言：击穿指针别名，验证真实 SQLiteStore 落库！
	retro2, err := realStore.GetVolumeRetrospective(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("GetVolumeRetrospective volume 1 failed: %v", err)
	}
	if retro2 == nil {
		t.Fatalf("expected volume retrospective for volume 1 to be persisted in real SQLiteStore, got nil")
	}
	if retro2.VolumeIndex != 1 {
		t.Errorf("expected VolumeIndex == 1, got %d", retro2.VolumeIndex)
	}
	if retro2.FromChapter != 1 || retro2.ToChapter != 2 {
		t.Errorf("expected chapter range 1..2, got %d..%d", retro2.FromChapter, retro2.ToChapter)
	}
	if !strings.Contains(retro2.Text, "发生") || !strings.Contains(retro2.Text, "留下") || !strings.Contains(retro2.Text, "未回收") {
		t.Errorf("expected retro text containing 发生/留下/未回收, got %s", retro2.Text)
	}
}

// TestChapterWorkshop_RealSQLiteStore_ExplicitVolumeSpans 验证显式卷界（StartChapter/EndChapter 与 EstimatedChapters 不一致）
// 在真实 SQLiteStore 下能准确以显式卷界触发回顾，且落库的 FromChapter/ToChapter 与显式卷界一致。
func TestChapterWorkshop_RealSQLiteStore_ExplicitVolumeSpans(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "real_explicit_retro_test.db")
	realStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create real SQLiteStore: %v", err)
	}
	defer realStore.Close()

	projectID := "proj-explicit-retro"
	proj := &domain.Project{
		ID:             projectID,
		Title:          "万道争锋",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "第一卷 问剑",
					CoreGoal:          "问剑试炼",
					Climax:            "剑斩心魔",
					StartChapter:      1,
					EndChapter:        2,
					EstimatedChapters: 10, // 与显式起止 1..2 不一致
				},
			},
		},
	}
	if err := realStore.SaveProject(ctx, proj); err != nil {
		t.Fatalf("failed to save project: %v", err)
	}

	fakeClient := &fakeSummaryWorkshopLLMClient{}
	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(realStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, realStore)

	// 第 1 章
	req1 := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    1,
		CoreConflict:    "试剑开篇",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res1, err := workshop.ProduceChapter(ctx, req1)
	if err != nil {
		t.Fatalf("ProduceChapter chapter 1 failed: %v", err)
	}
	if !res1.Committed {
		t.Fatalf("expected chapter 1 to be committed")
	}

	// 第 2 章（显式终章，但 EstimatedChapters 为 10）
	req2 := WorkshopProduceRequest{
		ProjectID:       projectID,
		ChapterIndex:    2,
		CoreConflict:    "剑斩心魔破关",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res2, err := workshop.ProduceChapter(ctx, req2)
	if err != nil {
		t.Fatalf("ProduceChapter chapter 2 failed: %v", err)
	}
	if !res2.Committed {
		t.Fatalf("expected chapter 2 to be committed")
	}

	retro, err := realStore.GetVolumeRetrospective(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("GetVolumeRetrospective failed: %v", err)
	}
	if retro == nil {
		t.Fatalf("expected volume retrospective to be saved at explicit end chapter 2, got nil")
	}
	if retro.FromChapter != 1 || retro.ToChapter != 2 {
		t.Errorf("expected explicit chapter range 1..2, got %d..%d (EstimatedChapters was 10)", retro.FromChapter, retro.ToChapter)
	}
}

// fallbackMockStore provides a mock store where GetProject returns nil Framework,
// while CommitChapter returns a Project with Framework (unbackfilled VolumeArcs),
// simulating the fallback path to UpdatedProject and testing the BackfillVolumeArcs guard.
type fallbackMockStore struct {
	*mockSummaryWorkshopStore
	fallbackProj *domain.Project
}

func (f *fallbackMockStore) CommitChapter(ctx context.Context, projectID string, chapter *domain.Chapter) (*domain.Project, error) {
	_, err := f.mockSummaryWorkshopStore.mockChronicleStore.CommitChapter(ctx, projectID, chapter)
	if err != nil {
		return nil, err
	}
	return f.fallbackProj, nil
}

// TestChapterWorkshop_VolumeRetrospective_FallbackUpdatedProjectRequiresBackfill 验证当 activeProj
// 未经 chronicle 预先回填（如 fallback 到 UpdatedProject 或 horizon 未预填起止时），
// workshop.go 必须在 deriveVolumeSpans 前显式调用 BackfillVolumeArcs，否则显式卷界 spans 为 {0,0} 导致静默跳过。
func TestChapterWorkshop_VolumeRetrospective_FallbackUpdatedProjectRequiresBackfill(t *testing.T) {
	ctx := context.Background()
	// horizon.Project 的 Framework 为 nil
	baseProj := &domain.Project{
		ID:             "p-fallback-test",
		Title:          "回退路径测试",
		TargetPlatform: "起点仙侠",
	}
	// UpdatedProject 带有 Framework，但 VolumeArcs 未经回填（StartChapter/EndChapter 均为 0）
	updatedProjWithFw := &domain.Project{
		ID:             "p-fallback-test",
		Title:          "回退路径测试",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "第一卷",
					EstimatedChapters: 2,
					// StartChapter: 0, EndChapter: 0 (未回填)
				},
			},
		},
	}
	mockStore := &fallbackMockStore{
		mockSummaryWorkshopStore: newMockSummaryWorkshopStore(baseProj),
		fallbackProj:             updatedProjWithFw,
	}
	fakeClient := &fakeSummaryWorkshopLLMClient{}
	orch := NewOrchestrator(fakeClient)
	qGate := NewQualityGate(fakeClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	// 第 1 章
	req1 := WorkshopProduceRequest{
		ProjectID:       "p-fallback-test",
		ChapterIndex:    1,
		CoreConflict:    "开局",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res1, err := workshop.ProduceChapter(ctx, req1)
	if err != nil {
		t.Fatalf("ProduceChapter 1 failed: %v", err)
	}
	if !res1.Committed {
		t.Fatalf("expected chapter 1 committed")
	}

	// 第 2 章（终章）
	req2 := WorkshopProduceRequest{
		ProjectID:       "p-fallback-test",
		ChapterIndex:    2,
		CoreConflict:    "终章突破",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}
	res2, err := workshop.ProduceChapter(ctx, req2)
	if err != nil {
		t.Fatalf("ProduceChapter 2 failed: %v", err)
	}
	if !res2.Committed {
		t.Fatalf("expected chapter 2 committed")
	}

	// 若 workshop.go 漏掉 BackfillVolumeArcs，spans 为 {0,0}，ResolveVolume 返回 -1，分卷回顾被静默跳过！
	// 只有通过 BackfillVolumeArcs 归一化后，才会在第 2 章成功触发分卷回顾并保存。
	if len(mockStore.volumeRetros) == 0 {
		t.Fatalf("expected volume retrospective to be saved via fallback UpdatedProject with BackfillVolumeArcs, got 0")
	}
	retro := mockStore.volumeRetros[1]
	if retro == nil || retro.FromChapter != 1 || retro.ToChapter != 2 {
		t.Fatalf("expected retro range 1..2, got %+v", retro)
	}
}

// ---------------------------------------------------------------------------
// P2-9 altType fallback tests (workshop.go rhythm-retry block).
//
// The `if altType == ""` fallback below the alt-candidate loop was dead code:
// altType used to be initialised to ChapterTypeEscalation (non-empty), so the
// fallback could never fire. It is now declared as the zero value
// (`var altType domain.ChapterType`) so the fallback is armed. The fallback
// itself is unchanged: actual==Escalation -> Setup, otherwise -> Escalation.
// An empty value can never leak into DeriveBeatsForChapterType: the fallback
// runs unconditionally when altType == "", and DeriveBeatsForChapterType also
// fail-closes "" to Escalation (orchestrator.go).
// ---------------------------------------------------------------------------

// Regression (normal path): hook-repeat triggers one retry and the retry uses
// the first eligible alt candidate. Preceding chapter is SETUP/CLIFFHANGER,
// first derivation returns ESCALATION/CLIFFHANGER (same hook -> retry);
// with no volume framework targetType is ESCALATION, so actual==target and
// the loop skips both -> first eligible pick is CLIMAX.
func TestChapterWorkshop_AltRetry_PicksFirstEligibleCandidate(t *testing.T) {
	setupBeats := []domain.SceneBeat{
		{Phase: "暗流涌动", Tension: 4, Action: "暗中打探消息"},
		{Phase: "局势隐变", Tension: 5, Action: "收到密信", HookType: "CLIFFHANGER"},
	}
	if got := domain.ClassifyChapterType(setupBeats); got != domain.ChapterTypeSetup {
		t.Fatalf("test premise broken: preceding beats classify as %s, want SETUP", got)
	}

	escalationJSON := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 6, "action": "反派集结人手", "expectation_broken": "反派自以为隐秘"},
			{"phase": "章末留钩", "tension": 7, "action": "阵法封锁四方", "expectation_broken": "局势胶着", "hook_type": "CLIFFHANGER"}
		],
		"state_mutation": {"inventory_delta": "", "power_delta": ""}
	}`

	mockClient := &mockRhythmLLMClient{beatsJSON: escalationJSON}
	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-alt-normal",
			Title:          "九天剑主",
			TargetPlatform: "番茄脑洞",
		},
		chapters: []*domain.Chapter{
			{ID: "c1", ProjectID: "p-alt-normal", ChapterIndex: 1, Title: "第1章", Beats: setupBeats},
		},
		hooks: []*domain.PlotHook{},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	_, err := workshop.ProduceChapter(context.Background(), WorkshopProduceRequest{
		ProjectID:      "p-alt-normal",
		ChapterIndex:   2,
		CoreConflict:   "宗门执法堂决战",
		ReasoningModel: "mock-reasoner",
		WriterModel:    "mock-writer",
		ReviewerModel:  "mock-reviewer",
		WordsTarget:    2000,
	})
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if mockClient.beatsCalls != 2 {
		t.Fatalf("expected 2 beats LLM calls (hook-repeat retry), got %d", mockClient.beatsCalls)
	}
	if !strings.Contains(mockClient.beatsSys[0], "【本章节奏定位：ESCALATION") {
		t.Errorf("expected first derivation to use ESCALATION, got sys=%q", mockClient.beatsSys[0])
	}
	if !strings.Contains(mockClient.beatsSys[1], "【本章节奏定位：CLIMAX") {
		t.Errorf("expected retry (altType) to use CLIMAX, got sys=%q", mockClient.beatsSys[1])
	}
}

// Fallback-armed path, maximally-blocked history: 3 trailing ESCALATION
// chapters block ESCALATION in both the target-candidate loop (target becomes
// CLIMAX) and as the first alt skip (actual==ESCALATION). The alt loop must
// still pick the next eligible candidate (AFTERMATH) — the `altType == ""`
// fallback only fires when the loop assigns nothing, which the trailing-run
// counting makes unreachable while the candidate/skip sets stay 5-choose-2
// (at most one type can hold a trailing run >= 3, so at least two eligible
// candidates always have consec == 1). This pins the fixed declaration's
// observable behaviour: loop pick wins, fallback stays dormant.
func TestChapterWorkshop_AltRetry_BlockedHistoryStillPicksLoopCandidate(t *testing.T) {
	escalationJSON := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 6, "action": "反派集结人手", "expectation_broken": "反派自以为隐秘"},
			{"phase": "章末留钩", "tension": 7, "action": "阵法封锁四方", "expectation_broken": "局势胶着", "hook_type": "CLIFFHANGER"}
		],
		"state_mutation": {"inventory_delta": "", "power_delta": ""}
	}`

	mockClient := &mockRhythmLLMClient{beatsJSON: escalationJSON}
	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	escBeats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 5, Action: "反派施压"},
		{Phase: "试探下套", Tension: 6, Action: "主角反击"},
		{Phase: "矛盾激化", Tension: 7, Action: "激战正酣"},
		{Phase: "章末留钩", Tension: 7, Action: "留下悬念", HookType: "REVERSAL"},
	}
	if got := domain.ClassifyChapterType(escBeats); got != domain.ChapterTypeEscalation {
		t.Fatalf("test premise broken: preceding beats classify as %s, want ESCALATION", got)
	}

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-alt-blocked",
			Title:          "九天剑主",
			TargetPlatform: "番茄脑洞",
		},
		chapters: []*domain.Chapter{
			{ID: "c1", ProjectID: "p-alt-blocked", ChapterIndex: 1, Title: "第1章", Beats: escBeats},
			{ID: "c2", ProjectID: "p-alt-blocked", ChapterIndex: 2, Title: "第2章", Beats: escBeats},
			{ID: "c3", ProjectID: "p-alt-blocked", ChapterIndex: 3, Title: "第3章", Beats: escBeats},
		},
		hooks: []*domain.PlotHook{},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	_, err := workshop.ProduceChapter(context.Background(), WorkshopProduceRequest{
		ProjectID:      "p-alt-blocked",
		ChapterIndex:   4,
		CoreConflict:   "宗门执法堂决战",
		ReasoningModel: "mock-reasoner",
		WriterModel:    "mock-writer",
		ReviewerModel:  "mock-reviewer",
		WordsTarget:    2000,
	})
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if mockClient.beatsCalls != 2 {
		t.Fatalf("expected 2 beats LLM calls (consec>=4 retry), got %d", mockClient.beatsCalls)
	}
	// Target loop skips consec-blocked ESCALATION -> CLIMAX; retry skips
	// actual(ESCALATION)+target(CLIMAX) -> AFTERMATH. The "" fallback would
	// have produced SETUP here (actual==ESCALATION); asserting AFTERMATH pins
	// that the loop pick wins and no empty/unexpected type leaks through.
	if !strings.Contains(mockClient.beatsSys[1], "【本章节奏定位：AFTERMATH") {
		t.Errorf("expected retry (altType) to use AFTERMATH, got sys=%q", mockClient.beatsSys[1])
	}
}

func TestChapterWorkshop_PlanAndIntentAccountedBudget(t *testing.T) {
	setupWorkshopWithPlanAndIntent := func(plan *domain.ActivePlan, intent *domain.ChapterIntent) (*ChapterWorkshop, *mockWorkshopLLMClient, *mockCheckpointWorkshopStore, *[]WorkshopEvent) {
		beatsResp := `{
			"beats": [
				{"phase": "蓄力压迫", "tension": 6, "action": "反派逼近洞府", "expectation_broken": "反派预料主角会求饶"},
				{"phase": "绝地反转", "tension": 9, "action": "主角引动大阵反杀", "expectation_broken": "反派被镇压"}
			],
			"state_mutation": {
				"inventory_delta": "+战利品",
				"power_delta": "威名大振"
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
			renderDraft: "夜幕低垂。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
			reviewJSON:  reviewResp,
		}
		orch := NewOrchestrator(mockClient)
		qGate := NewQualityGate(mockClient, nil)
		mockStore := &mockCheckpointWorkshopStore{
			mockChronicleStore: &mockChronicleStore{
				project: &domain.Project{
					ID:             "p-planacct",
					Title:          "万域封神",
					TargetPlatform: "番茄脑洞",
					WorldRules:     "杀生夺灵，因果必报",
				},
				codexEntries: []*domain.CodexEntry{
					{
						ID:        "codex_p-planacct_pro",
						ProjectID: "p-planacct",
						Category:  domain.CategoryCharacter,
						Archetype: "PROTAGONIST",
						Name:      "林凡",
						Realm:     "练气圆满",
						Inventory: "青玉短剑",
					},
				},
				chapters: []*domain.Chapter{},
				hooks:    []*domain.PlotHook{},
			},
			activePlan: plan,
		}
		if intent != nil {
			_ = mockStore.SaveChapterIntent(context.Background(), intent)
		}
		chronicle := NewCanonChronicle(mockStore)
		workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)
		var events []WorkshopEvent
		return workshop, mockClient, mockStore, &events
	}

	baseReq := func(events *[]WorkshopEvent) WorkshopProduceRequest {
		return WorkshopProduceRequest{
			ProjectID:       "p-planacct",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭抢夺古经",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      false,
			MaxRewriteLoops: 1,
			OnProgress: func(ev WorkshopEvent) {
				*events = append(*events, ev)
			},
		}
	}

	t.Run("case1_locked_intent_active_plan_normal_budget", func(t *testing.T) {
		plan := &domain.ActivePlan{
			ProjectID: "p-planacct",
			Status:    domain.PlanStatusActive,
			PlanText:  "计划：主角假意投降魔教，实则借雷劫暗算魔尊",
		}
		intent := &domain.ChapterIntent{
			ProjectID:    "p-planacct",
			ChapterIndex: 1,
			VolumeIndex:  1,
			Conflict:     "魔宗大殿夺宝",
			POV:          "林凡",
			Status:       domain.IntentStatusLocked,
		}
		ws, client, _, events := setupWorkshopWithPlanAndIntent(plan, intent)
		ws.chronicle.SetBudgetTokens(12000)

		res, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}
		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}

		// Prompt assertions
		if !strings.Contains(client.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats prompt to contain plan header, got: %s", client.capturedBeatsPrompt)
		}
		if !strings.Contains(client.capturedBeatsPrompt, "计划：主角假意投降魔教，实则借雷劫暗算魔尊") {
			t.Errorf("expected beats prompt to contain plan text, got: %s", client.capturedBeatsPrompt)
		}
		if !strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected beats prompt to contain intent header, got: %s", client.capturedBeatsPrompt)
		}
		if !strings.Contains(client.capturedBeatsPrompt, "魔宗大殿夺宝") {
			t.Errorf("expected beats prompt to contain intent conflict, got: %s", client.capturedBeatsPrompt)
		}

		// Event ordering assertions:
		// plan event -> intent event -> horizon assembled event
		planIdx, intentIdx, assembledIdx := -1, -1, -1
		for i, ev := range *events {
			if strings.Contains(ev.Message, "已注入作者选定的发展路线") {
				planIdx = i
			}
			if strings.Contains(ev.Message, "已装载第 1 章意图卡（状态 LOCKED）") {
				intentIdx = i
			}
			if strings.Contains(ev.Message, "正史视界组装完成") {
				assembledIdx = i
			}
		}
		if planIdx == -1 {
			t.Errorf("expected plan injection event to be emitted")
		}
		if intentIdx == -1 {
			t.Errorf("expected intent loaded event to be emitted")
		}
		if assembledIdx == -1 {
			t.Errorf("expected horizon assembled event to be emitted")
		}
		if !(planIdx < intentIdx && intentIdx < assembledIdx) {
			t.Errorf("expected event order plan (%d) < intent (%d) < assembled (%d)", planIdx, intentIdx, assembledIdx)
		}
	})

	t.Run("case2_budget_exceeded_by_extras_fail_closed", func(t *testing.T) {
		plan := &domain.ActivePlan{
			ProjectID: "p-planacct",
			Status:    domain.PlanStatusActive,
			PlanText:  "计划：主角假意投降魔教，实则借雷劫暗算魔尊",
		}
		intent := &domain.ChapterIntent{
			ProjectID:    "p-planacct",
			ChapterIndex: 1,
			VolumeIndex:  1,
			Conflict:     "魔宗大殿夺宝",
			POV:          "林凡",
			Status:       domain.IntentStatusLocked,
		}
		ws, _, _, events := setupWorkshopWithPlanAndIntent(plan, intent)

		// Probe base protected tokens without extras
		probeChron := NewCanonChronicle(ws.store)
		probeChron.SetBudgetTokens(1)
		_, traceProbe, errProbe := probeChron.AssembleHorizonTraced(context.Background(), "p-planacct", 1)
		if errProbe == nil || traceProbe == nil {
			t.Fatalf("expected protected overflow probe error, got: %v", errProbe)
		}
		baseProtectedTokens := traceProbe.EstimatedTokens

		// Set budget to baseProtectedTokens + 5.
		// Without extras, baseProtectedTokens <= budget would pass.
		// But with extras (plan + intent), totalProtected > budget, so AssembleHorizonWithExtras must fail-closed.
		ws.chronicle.SetBudgetTokens(baseProtectedTokens + 5)

		_, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err == nil {
			t.Fatalf("expected ProduceChapter to fail with budget overflow, got nil")
		}
		if !strings.Contains(err.Error(), "assemble canon horizon failed") {
			t.Errorf("expected error to contain 'assemble canon horizon failed', got: %v", err)
		}
		if !strings.Contains(err.Error(), "context budget exceeded by protected items") {
			t.Errorf("expected error to mention protected overflow, got: %v", err)
		}

		// Fail-closed event assertion: NO plan or intent injected events emitted before assemble failed!
		for _, ev := range *events {
			if strings.Contains(ev.Message, "已注入作者选定的发展路线") {
				t.Errorf("fail-closed violation: emitted plan injection event despite assemble failure: %s", ev.Message)
			}
			if strings.Contains(ev.Message, "已装载第 1 章意图卡") {
				t.Errorf("fail-closed violation: emitted intent load event despite assemble failure: %s", ev.Message)
			}
		}
	})

	t.Run("case3_draft_intent_no_plan_zero_extras", func(t *testing.T) {
		intent := &domain.ChapterIntent{
			ProjectID:    "p-planacct",
			ChapterIndex: 1,
			VolumeIndex:  1,
			Conflict:     "草稿冲突",
			POV:          "林凡",
			Status:       domain.IntentStatusDraft,
		}
		ws, client, _, events := setupWorkshopWithPlanAndIntent(nil, intent)

		// Probe base protected tokens
		probeChron := NewCanonChronicle(ws.store)
		probeChron.SetBudgetTokens(1)
		_, traceProbe, _ := probeChron.AssembleHorizonTraced(context.Background(), "p-planacct", 1)
		baseProtectedTokens := traceProbe.EstimatedTokens

		// Set budget to baseProtectedTokens + 10.
		// Since plan is nil and intent is DRAFT, extras is ZERO.
		// Thus totalProtected = baseProtectedTokens <= budget, so it must SUCCEED!
		ws.chronicle.SetBudgetTokens(baseProtectedTokens + 10)

		res, err := ws.ProduceChapter(context.Background(), baseReq(events))
		if err != nil {
			t.Fatalf("expected ProduceChapter to succeed with zero extras under tight budget, got: %v", err)
		}
		if res.Audit == nil {
			t.Fatalf("expected non-nil Audit report")
		}

		if strings.Contains(client.capturedBeatsPrompt, "作者选定的发展路线") {
			t.Errorf("expected beats prompt NOT to contain plan text, got: %s", client.capturedBeatsPrompt)
		}
		if strings.Contains(client.capturedBeatsPrompt, "【本章意图卡") {
			t.Errorf("expected beats prompt NOT to contain intent block, got: %s", client.capturedBeatsPrompt)
		}

		foundDraftSkipEvent := false
		for _, ev := range *events {
			if strings.Contains(ev.Message, "第 1 章意图卡为草稿（DRAFT，未锁定），已跳过注入") {
				foundDraftSkipEvent = true
				break
			}
		}
		if !foundDraftSkipEvent {
			t.Errorf("expected draft skip event to be emitted")
		}
	})
}

// 5. TestChapterWorkshop_CompactionFailureWarnsInIssues：让 chronicle 进入"有丢弃"的小预算场景
// （chronicle.SetConfig 极小 budget，参照既有 budget 测试构造），mock 客户端在"记忆压缩器" system prompt 下返回空串
// （在 mockWorkshopLLMClient.ChatCompletion 加一个该分派分支，空串=压缩失败）→ 断言 res.Audit.Issues 含 【记忆压缩警告】。
func TestChapterWorkshop_CompactionFailureWarnsInIssues(t *testing.T) {
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
			ID:             "p-compact-warn",
			Title:          "万域封神",
			TargetPlatform: "番茄脑洞",
			WorldRules:     "杀生夺灵，因果必报",
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:        "codex_p-ws_pro",
				ProjectID: "p-compact-warn",
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
				Name:      "林凡",
				Realm:     "练气圆满",
				Inventory: "青玉短剑",
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 1, Title: "第1章", Content: "起步篇章。"},
			{ChapterIndex: 2, Title: "第2章", Content: "磨砺篇章。"},
			{ChapterIndex: 3, Title: "第3章", Content: "蓄势篇章。"},
			{ChapterIndex: 4, Title: "第4章", Content: "暗涌篇章。"},
		},
		hooks: []*domain.PlotHook{
			{
				ID:             "hook_far_drop",
				Title:          "远期超长伏笔",
				Details:        strings.Repeat("潜藏深渊地下的古老妖皇封印，涉及万界大劫因果。", 10),
				CreatedChapter: 1,
				TargetChapter:  30,
				Status:         domain.HookStatusOpen,
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	chronicle.SetConfig(&config.Config{
		ContextBudgetTokens: 1,
	})
	_, probeTrace, _ := chronicle.AssembleHorizonTraced(context.Background(), "p-compact-warn", 5)
	if probeTrace == nil {
		t.Fatalf("expected non-nil probeTrace")
	}
	baseProtectedTokens := probeTrace.EstimatedTokens

	// Set small budget via chronicle.SetConfig: baseProtected + 150 (enough for >= MinCompactionBudgetTokens 100, but hook_far_drop exceeds 150)
	chronicle.SetConfig(&config.Config{
		ContextBudgetTokens: baseProtectedTokens + 150,
	})

	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	var events []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-compact-warn",
		ChapterIndex:    5,
		CoreConflict:    "宗门反派夜袭抢夺古经",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-v3",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      true,
		MaxRewriteLoops: 2,
		OnProgress: func(ev WorkshopEvent) {
			events = append(events, ev)
		},
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.Audit == nil {
		t.Fatalf("expected non-nil Audit report")
	}

	hasCompactionWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【记忆压缩警告】") {
			hasCompactionWarning = true
			break
		}
	}
	if !hasCompactionWarning {
		t.Errorf("expected res.Audit.Issues to contain '【记忆压缩警告】', got: %v", res.Audit.Issues)
	}

	foundCompactionEvent := false
	for _, ev := range events {
		if strings.Contains(ev.Message, "【记忆压缩警告】") {
			foundCompactionEvent = true
			break
		}
	}
	if !foundCompactionEvent {
		t.Errorf("expected workshop event with '【记忆压缩警告】' to be emitted")
	}
}
