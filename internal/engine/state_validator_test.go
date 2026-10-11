package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockStateValidatorLLMClient struct {
	mu            sync.Mutex
	response      string
	err           error
	callCount     int
	lastSys       string
	lastUser      string
	systemPrompts []string
	userPrompts   []string
}

func (m *mockStateValidatorLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastSys = systemPrompt
	m.lastUser = userPrompt
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.userPrompts = append(m.userPrompts, userPrompt)
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockStateValidatorLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}, err
}

func (m *mockStateValidatorLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
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

// Case 1: 空增量 → 不调 LLM（mock 计数 0）、consistent。
func TestValidateSettledState_EmptyMutation_NoLLMCall(t *testing.T) {
	client := &mockStateValidatorLLMClient{}
	baseline := &domain.CodexEntry{
		Name:      "林凡",
		Realm:     "练气三层",
		Inventory: "青灵剑",
	}
	emptyMutation := domain.StateMutation{}

	res, usage, err := ValidateSettledState(context.Background(), client, "model", baseline, emptyMutation, "正文草稿", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.callCount != 0 {
		t.Fatalf("expected 0 LLM calls for empty mutation, got %d", client.callCount)
	}
	if !res.Consistent {
		t.Errorf("expected Consistent == true, got %v", res.Consistent)
	}
	if res.ReconciliationRequired {
		t.Errorf("expected ReconciliationRequired == false, got %v", res.ReconciliationRequired)
	}
	if len(res.Observations) != 0 {
		t.Errorf("expected 0 observations, got %d", len(res.Observations))
	}
	if usage.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens, got %d", usage.TotalTokens)
	}
}

// Case 2: mock 返回 observations JSON → 解析正确（code/summary/evidence）。
func TestValidateSettledState_ObservationsParsedCorrectly(t *testing.T) {
	client := &mockStateValidatorLLMClient{
		response: `{
			"consistent": false,
			"reconciliation_required": true,
			"observations": [
				{
					"code": "item-conflict",
					"summary": "主角在正文中未消耗洗髓丹，但状态卡中被扣除",
					"evidence": ["林凡将洗髓丹收入储物袋，并未服下"]
				}
			]
		}`,
	}
	baseline := &domain.CodexEntry{
		Name:      "林凡",
		Realm:     "练气三层",
		Inventory: "洗髓丹x1",
	}
	mutation := domain.StateMutation{
		InventoryDelta: "-洗髓丹",
	}

	res, usage, err := ValidateSettledState(context.Background(), client, "model", baseline, mutation, "林凡将洗髓丹收入储物袋，并未服下。", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.callCount != 1 {
		t.Fatalf("expected 1 LLM call, got %d", client.callCount)
	}
	if res.Consistent {
		t.Errorf("expected Consistent == false, got %v", res.Consistent)
	}
	if !res.ReconciliationRequired {
		t.Errorf("expected ReconciliationRequired == true, got %v", res.ReconciliationRequired)
	}
	if len(res.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(res.Observations))
	}
	obs := res.Observations[0]
	if obs.Code != "item-conflict" {
		t.Errorf("expected Code 'item-conflict', got %q", obs.Code)
	}
	if obs.Summary != "主角在正文中未消耗洗髓丹，但状态卡中被扣除" {
		t.Errorf("expected Summary '主角在正文中未消耗洗髓丹，但状态卡中被扣除', got %q", obs.Summary)
	}
	if len(obs.Evidence) != 1 || obs.Evidence[0] != "林凡将洗髓丹收入储物袋，并未服下" {
		t.Errorf("expected Evidence ['林凡将洗髓丹收入储物袋，并未服下'], got %v", obs.Evidence)
	}
	if usage.TotalTokens <= 0 {
		t.Errorf("expected positive TotalTokens, got %d", usage.TotalTokens)
	}
}

// Case 3: 非法 JSON → 返回 error。
func TestValidateSettledState_InvalidJSON_ReturnsError(t *testing.T) {
	client := &mockStateValidatorLLMClient{
		response: "not a valid json object",
	}
	baseline := &domain.CodexEntry{Name: "林凡"}
	mutation := domain.StateMutation{PowerDelta: "+破境"}

	_, _, err := ValidateSettledState(context.Background(), client, "model", baseline, mutation, "正文", 1)
	if err == nil {
		t.Fatalf("expected error on invalid json, got nil")
	}
}

// Workshop integrated mock client supporting reconciliation lifecycle
type workshopReconcileMockClient struct {
	mu                sync.Mutex
	beatsJSON         string
	renderDraft       string
	reviewJSON        string
	settle1JSON       string
	settle2JSON       string
	settleUserPrompts []string
	validation1JSON   string
	validation2JSON   string
	validationErr     error
	validationCalls   int
	settleCalls       int
}

func (m *workshopReconcileMockClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.Contains(systemPrompt, "状态对账") {
		m.validationCalls++
		if m.validationErr != nil {
			return "", m.validationErr
		}
		if m.validationCalls == 1 && m.validation1JSON != "" {
			return m.validation1JSON, nil
		}
		if m.validation2JSON != "" {
			return m.validation2JSON, nil
		}
		return `{"consistent":true,"reconciliation_required":false,"observations":[]}`, nil
	}

	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		m.settleCalls++
		m.settleUserPrompts = append(m.settleUserPrompts, userPrompt)
		if strings.Contains(userPrompt, "上一次结算的对账反馈") && m.settle2JSON != "" {
			return m.settle2JSON, nil
		}
		if m.settle1JSON != "" {
			return m.settle1JSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}

	if strings.Contains(systemPrompt, "总设计师") {
		return m.beatsJSON, nil
	}
	if strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
		return m.reviewJSON, nil
	}

	return m.renderDraft, nil
}

func (m *workshopReconcileMockClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}, err
}

func (m *workshopReconcileMockClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
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

// Case 4: workshop 集成：settler 首答错误投影 → validator 返回 reconciliation_required → 断言发生第二次 settle 且其 userPrompt 含"上一次结算的对账反馈" → 重验 clean → 结果 mutation 为修正版、事件含"状态投影需要对账"、usage 含四段调用。
func TestChapterWorkshop_SettlerReconciliation_Succeeds(t *testing.T) {
	beatsResp := `{"beats":[{"phase":"对决","tension":8,"action":"拔剑斩敌"}],"state_mutation":{"inventory_delta":"+玄铁重剑"}}`
	draft := "林凡在石室角落里寻得一截降龙木，随后又拔出玄铁重剑握于掌中，气势如虹。"
	reviewResp := `{"verdict":"ACCEPTED","score":95,"issues":[],"suggestions":"通过"}`

	// Settle 1: 错误投影，只记录了降龙木
	settle1 := `{
		"inventory_delta": "+降龙木",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{"field": "inventory", "quote": "在石室角落里寻得一截降龙木", "chapter_index": 1}
		]
	}`

	// Validation 1: 发现漏记重剑，要求对账
	val1 := `{
		"consistent": false,
		"reconciliation_required": true,
		"observations": [
			{
				"code": "missing-weapon",
				"summary": "正文重点获得了玄铁重剑，但结算仅记录了降龙木",
				"evidence": ["拔出玄铁重剑握于掌中"]
			}
		]
	}`

	// Settle 2: 依据反馈修正为玄铁重剑
	settle2 := `{
		"inventory_delta": "+玄铁重剑",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{"field": "inventory", "quote": "拔出玄铁重剑握于掌中", "chapter_index": 1}
		]
	}`

	// Validation 2: 重验 clean
	val2 := `{"consistent": true, "reconciliation_required": false, "observations": []}`

	mockClient := &workshopReconcileMockClient{
		beatsJSON:       beatsResp,
		renderDraft:     draft,
		reviewJSON:      reviewResp,
		settle1JSON:     settle1,
		settle2JSON:     settle2,
		validation1JSON: val1,
		validation2JSON: val2,
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-reconcile-1",
				Title:          "重算对账测试",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀伐果断",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-reconcile-1_pro",
					ProjectID: "p-reconcile-1",
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "筑基初期",
					Inventory: "短刃",
				},
			},
			chapters: []*domain.Chapter{},
			hooks:    []*domain.PlotHook{},
		},
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	var events []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-reconcile-1",
		ChapterIndex:    1,
		CoreConflict:    "夺取至宝",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-v3",
		ReviewerModel:   "mock-rev",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 1,
		OnProgress: func(ev WorkshopEvent) {
			events = append(events, ev)
		},
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 断言发生第二次 settle
	if mockClient.settleCalls != 2 {
		t.Fatalf("expected 2 settle calls, got %d", mockClient.settleCalls)
	}
	if len(mockClient.settleUserPrompts) < 2 {
		t.Fatalf("expected at least 2 recorded settle user prompts, got %d", len(mockClient.settleUserPrompts))
	}
	secondSettlePrompt := mockClient.settleUserPrompts[1]
	if !strings.Contains(secondSettlePrompt, "上一次结算的对账反馈") {
		t.Errorf("expected 2nd settle user prompt to contain '上一次结算的对账反馈', got: %s", secondSettlePrompt)
	}
	if !strings.Contains(secondSettlePrompt, "[missing-weapon] 正文重点获得了玄铁重剑，但结算仅记录了降龙木") {
		t.Errorf("expected 2nd settle user prompt to contain observation feedback, got: %s", secondSettlePrompt)
	}

	// 重验 clean → 结果 mutation 为修正版
	if res.StateMutation.InventoryDelta != "+玄铁重剑" {
		t.Errorf("expected corrected StateMutation inventory '+玄铁重剑', got %q", res.StateMutation.InventoryDelta)
	}

	// 事件含"状态投影需要对账"
	foundReconcileEvent := false
	for _, ev := range events {
		if strings.Contains(ev.Message, "状态投影需要对账，正在仅重算本章结算层") {
			foundReconcileEvent = true
			break
		}
	}
	if !foundReconcileEvent {
		t.Errorf("expected progress events to contain '状态投影需要对账，正在仅重算本章结算层'")
	}

	// 结算对账链路经历了 4 段调用：settle 1 + validate 1 + settle 2 + validate 2
	if mockClient.validationCalls != 2 {
		t.Errorf("expected 2 validation calls, got %d", mockClient.validationCalls)
	}
	// 4段调用总 token 应该至少包含 4 * 150 = 600 tokens
	// 加上 beats (150) + render (150) + review (150) = 1050
	if res.TotalUsage.TotalTokens < 600 {
		t.Errorf("expected usage to reflect all stages including reconciliation, got totalTokens=%d", res.TotalUsage.TotalTokens)
	}
}

// Case 5: unresolved：validator 恒不一致 → 结果 StateValidationObservations 非空、Audit.Issues 含 【状态对账】、TechnicalBlocked==false（ACCEPTED 正常入库）。
func TestChapterWorkshop_SettlerReconciliation_Unresolved_NotBlocking(t *testing.T) {
	beatsResp := `{"beats":[{"phase":"对决","tension":8,"action":"拔剑斩敌"}],"state_mutation":{"inventory_delta":"+玄铁重剑"}}`
	draft := "林凡在石室角落里寻得一截降龙木，随后又拔出玄铁重剑握于掌中，气势如虹。"
	reviewResp := `{"verdict":"ACCEPTED","score":95,"issues":[],"suggestions":"通过"}`

	settle := `{
		"inventory_delta": "+降龙木",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{"field": "inventory", "quote": "在石室角落里寻得一截降龙木", "chapter_index": 1}
		]
	}`

	// 始终返回对账不一致
	valUnresolved := `{
		"consistent": false,
		"reconciliation_required": true,
		"observations": [
			{
				"code": "persistent-discrepancy",
				"summary": "主角境界突破在正文中描写隐晦，无法确证",
				"evidence": ["气势如虹"]
			}
		]
	}`

	mockClient := &workshopReconcileMockClient{
		beatsJSON:       beatsResp,
		renderDraft:     draft,
		reviewJSON:      reviewResp,
		settle1JSON:     settle,
		settle2JSON:     settle,
		validation1JSON: valUnresolved,
		validation2JSON: valUnresolved,
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-reconcile-unresolved",
				Title:          "未闭合对账测试",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀伐果断",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-reconcile-unresolved_pro",
					ProjectID: "p-reconcile-unresolved",
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "筑基初期",
					Inventory: "短刃",
				},
			},
			chapters: []*domain.Chapter{},
			hooks:    []*domain.PlotHook{},
		},
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-reconcile-unresolved",
		ChapterIndex:    1,
		CoreConflict:    "夺取至宝",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-v3",
		ReviewerModel:   "mock-rev",
		WordsTarget:     2000,
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 结果 StateValidationObservations 非空
	if len(res.StateValidationObservations) == 0 {
		t.Fatalf("expected non-empty StateValidationObservations on unresolved reconciliation")
	}
	if res.StateValidationObservations[0].Code != "persistent-discrepancy" {
		t.Errorf("expected observation code 'persistent-discrepancy', got %q", res.StateValidationObservations[0].Code)
	}

	// Audit.Issues 含 【状态对账】
	foundAuditIssue := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【状态对账】") && strings.Contains(issue, "[persistent-discrepancy]") {
			foundAuditIssue = true
			break
		}
	}
	if !foundAuditIssue {
		t.Errorf("expected Audit.Issues to contain '【状态对账】' issue, got: %v", res.Audit.Issues)
	}

	// TechnicalBlocked == false (ACCEPTED 正常入库)
	if res.TechnicalBlocked {
		t.Errorf("expected TechnicalBlocked == false, got true")
	}
	if !res.Committed {
		t.Errorf("expected chapter to be committed (ACCEPTED 正常入库), got committed=false")
	}
}

// Case 6: validator 基础设施错误：mock 校验分支返回 error → 断言仍走单次 reconcile（inkos 语义）、重验错误 → observations 含 state-validation-unavailable。
func TestChapterWorkshop_SettlerReconciliation_ValidatorError_ReconcileAndUnavailable(t *testing.T) {
	beatsResp := `{"beats":[{"phase":"对决","tension":8,"action":"拔剑斩敌"}],"state_mutation":{"inventory_delta":"+玄铁重剑"}}`
	draft := "林凡在石室角落里寻得一截降龙木，随后又拔出玄铁重剑握于掌中，气势如虹。"
	reviewResp := `{"verdict":"ACCEPTED","score":95,"issues":[],"suggestions":"通过"}`

	settle := `{
		"inventory_delta": "+降龙木",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{"field": "inventory", "quote": "在石室角落里寻得一截降龙木", "chapter_index": 1}
		]
	}`

	mockClient := &workshopReconcileMockClient{
		beatsJSON:     beatsResp,
		renderDraft:   draft,
		reviewJSON:    reviewResp,
		settle1JSON:   settle,
		settle2JSON:   settle,
		validationErr: errors.New("mock validator network timeout"),
	}

	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-reconcile-err",
				Title:          "校验器错误测试",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀伐果断",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-reconcile-err_pro",
					ProjectID: "p-reconcile-err",
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "筑基初期",
					Inventory: "短刃",
				},
			},
			chapters: []*domain.Chapter{},
			hooks:    []*domain.PlotHook{},
		},
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-reconcile-err",
		ChapterIndex:    1,
		CoreConflict:    "夺取至宝",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-v3",
		ReviewerModel:   "mock-rev",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 1,
	}

	res, err := workshop.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 断言仍走单次 reconcile（inkos 语义）：重算了结算
	if mockClient.settleCalls != 2 {
		t.Errorf("expected 2 settle calls under validator error (single reconcile), got %d", mockClient.settleCalls)
	}

	// 重验错误 → observations 含 state-validation-unavailable
	foundUnavailable := false
	for _, o := range res.StateValidationObservations {
		if o.Code == "state-validation-unavailable" && strings.Contains(o.Summary, "mock validator network timeout") {
			foundUnavailable = true
			break
		}
	}
	if !foundUnavailable {
		t.Errorf("expected observations to contain 'state-validation-unavailable' with err text, got: %v", res.StateValidationObservations)
	}

	// 校验器基础设施故障不阻断入库
	if res.TechnicalBlocked {
		t.Errorf("expected TechnicalBlocked == false, got true")
	}
}
