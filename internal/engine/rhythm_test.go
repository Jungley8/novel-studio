package engine

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockRhythmLLMClient struct {
	mu           sync.Mutex
	beatsCalls   int
	beatsSys     []string
	beatsPrompts []string
	beatsJSON    string
	renderDraft  string
	reviewJSON   string
	settleJSON   string
}

func (m *mockRhythmLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		if m.settleJSON != "" {
			return m.settleJSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
	if strings.Contains(systemPrompt, "总设计师") {
		m.mu.Lock()
		m.beatsCalls++
		m.beatsSys = append(m.beatsSys, systemPrompt)
		m.beatsPrompts = append(m.beatsPrompts, userPrompt)
		m.mu.Unlock()
		return m.beatsJSON, nil
	}
	if strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") {
		if m.reviewJSON != "" {
			return m.reviewJSON, nil
		}
		return `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":""}`, nil
	}
	if m.renderDraft != "" {
		return m.renderDraft, nil
	}
	return "清晨破晓，剑意冲霄。", nil
}

func (m *mockRhythmLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}, err
}

func (m *mockRhythmLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
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

// Case ③: fake LLM 连续返回同型 beats → 触发重推且第二次换了章型（断言第二次请求的 prompt 含新章型指令）
func TestWorkshop_RhythmRetry_ConsecutiveRepetition(t *testing.T) {
	// 模拟返回 ESCALATION 类型的 beats (4拍，均分中高张力)
	escalationJSON := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 5, "action": "反派暗中集结人手", "expectation_broken": "反派自以为神不知鬼不觉"},
			{"phase": "试探下套", "tension": 6, "action": "主角佯装不知步入圈套", "expectation_broken": "主角早已布设后手"},
			{"phase": "矛盾升级", "tension": 7, "action": "双方正面摊牌争锋相对", "expectation_broken": "反派惊觉主角有备而来"},
			{"phase": "章末留钩", "tension": 7, "action": "阵法启动封锁四方", "expectation_broken": "局势彻底胶着", "hook_type": "CLIFFHANGER"}
		],
		"state_mutation": {
			"inventory_delta": "",
			"power_delta": ""
		}
	}`

	mockClient := &mockRhythmLLMClient{
		beatsJSON: escalationJSON,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	// 前序已有 3 章连续 ESCALATION
	escBeats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 5, Action: "反派施压"},
		{Phase: "试探下套", Tension: 6, Action: "主角反击"},
		{Phase: "矛盾激化", Tension: 7, Action: "激战正酣"},
		{Phase: "章末留钩", Tension: 7, Action: "留下悬念", HookType: "REVERSAL"},
	}

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-rhythm",
			Title:          "九天剑主",
			TargetPlatform: "番茄脑洞",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, Title: "第一卷 锋芒初试", CoreGoal: "洗刷冤屈", Climax: "宗门大比斩仇敌", EstimatedChapters: 10},
				},
			},
		},
		chapters: []*domain.Chapter{
			{ID: "c1", ProjectID: "p-rhythm", ChapterIndex: 1, Title: "第1章", Beats: escBeats},
			{ID: "c2", ProjectID: "p-rhythm", ChapterIndex: 2, Title: "第2章", Beats: escBeats},
			{ID: "c3", ProjectID: "p-rhythm", ChapterIndex: 3, Title: "第3章", Beats: escBeats},
		},
		hooks: []*domain.PlotHook{},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	res, err := workshop.ProduceChapter(context.Background(), WorkshopProduceRequest{
		ProjectID:      "p-rhythm",
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

	// 断言 1: 触发了重推，beats LLM 调用次数必须为 2
	if mockClient.beatsCalls != 2 {
		t.Fatalf("expected 2 beats LLM calls (triggered rhythm retry), but got %d", mockClient.beatsCalls)
	}

	// 断言 2: 第二次请求的 prompt 必须包含新章型指令（换了章型，不再是原始的 targetType / ESCALATION）
	firstSys := mockClient.beatsSys[0]
	secondSys := mockClient.beatsSys[1]
	firstPrompt := mockClient.beatsPrompts[0]
	secondPrompt := mockClient.beatsPrompts[1]

	if firstSys == secondSys && firstPrompt == secondPrompt {
		t.Errorf("second request prompt did not change chapter type instructions")
	}

	// 确认第二次请求包含了新章型定位 (如 CLIMAX、AFTERMATH 或 SETUP 等)
	if !strings.Contains(secondSys, "【本章节奏定位：") || !strings.Contains(secondPrompt, "【本章节奏定位：") {
		t.Errorf("second request prompt missing chapter type guidance: sys=%s, prompt=%s", secondSys, secondPrompt)
	}

	// 确认两次请求的章型指令不同
	hasDifferentType := false
	for _, ct := range []string{"CLIMAX", "AFTERMATH", "SETUP", "INTERLUDE", "ESCALATION"} {
		inFirst := strings.Contains(firstSys, ct)
		inSecond := strings.Contains(secondSys, ct)
		if inFirst != inSecond {
			hasDifferentType = true
			break
		}
	}
	if !hasDifferentType {
		t.Errorf("expected different chapter types between first and second calls, got first=%s, second=%s", firstSys, secondSys)
	}

	// 断言 3: 重推后仍连续同型超限时，审计 Issues 留痕 【节奏提示】连续同型4章
	if res.Audit == nil {
		t.Fatalf("expected audit report to be present")
	}
	hasRhythmWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【节奏提示】连续同型4章") {
			hasRhythmWarning = true
			break
		}
	}
	if !hasRhythmWarning {
		t.Errorf("expected audit report issues to contain '【节奏提示】连续同型4章', got: %v", res.Audit.Issues)
	}
}

// 验证 Hook 重复触发重推
func TestWorkshop_RhythmRetry_HookRepeat(t *testing.T) {
	// 前序章节最后拍是 CLIFFHANGER
	prevBeats := []domain.SceneBeat{
		{Phase: "交锋", Tension: 5, Action: "交锋"},
		{Phase: "收束", Tension: 6, Action: "收束", HookType: "CLIFFHANGER"},
	}

	// 新章 beats 也输出了 CLIFFHANGER
	hookRepeatJSON := `{
		"beats": [
			{"phase": "展开", "tension": 6, "action": "新章节展开"},
			{"phase": "收尾", "tension": 6, "action": "新章节留钩", "hook_type": "CLIFFHANGER"}
		],
		"state_mutation": {"inventory_delta": "", "power_delta": ""}
	}`

	mockClient := &mockRhythmLLMClient{
		beatsJSON: hookRepeatJSON,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:    "p-hook-repeat",
			Title: "刀道极境",
		},
		chapters: []*domain.Chapter{
			{ID: "c1", ProjectID: "p-hook-repeat", ChapterIndex: 1, Beats: prevBeats},
		},
		hooks: []*domain.PlotHook{},
	}

	chronicle := NewCanonChronicle(mockStore)
	workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	_, err := workshop.ProduceChapter(context.Background(), WorkshopProduceRequest{
		ProjectID:      "p-hook-repeat",
		ChapterIndex:   2,
		CoreConflict:   "刀意试炼",
		ReasoningModel: "mock-reasoner",
		WriterModel:    "mock-writer",
		ReviewerModel:  "mock-reviewer",
	})
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	// 确认同钩类型触发了重推 (2次 beats 调用)
	if mockClient.beatsCalls != 2 {
		t.Errorf("expected 2 beats LLM calls for repeated hook, got %d", mockClient.beatsCalls)
	}
}

// 验证 CurrentVolume 为 nil 时的分支覆盖（前瞻为空，beats 照常推演）
func TestOrchestrator_DeriveBeatsForChapterType_NilVolume(t *testing.T) {
	beatsJSON := `{
		"beats": [
			{"phase": "试探", "tension": 4, "action": "探查敌情"},
			{"phase": "收束", "tension": 5, "action": "返回营地"}
		],
		"state_mutation": {"inventory_delta": "", "power_delta": ""}
	}`
	mockClient := &mockRhythmLLMClient{beatsJSON: beatsJSON}
	orch := NewOrchestrator(mockClient)

	horizon := &CanonHorizon{
		Project: &domain.Project{
			Title: "上古秘录",
		},
		TargetChapter: 1,
		CurrentVolume: nil, // 显式为 nil
	}

	out, err := orch.DeriveBeatsForChapterType(context.Background(), "mock-model", horizon, "试探冲突", domain.ChapterTypeEscalation)
	if err != nil {
		t.Fatalf("DeriveBeatsForChapterType failed with nil volume: %v", err)
	}
	if len(out.Beats) != 2 {
		t.Errorf("expected 2 beats, got %d", len(out.Beats))
	}
	if len(mockClient.beatsPrompts) != 1 {
		t.Fatalf("expected 1 prompt call, got %d", len(mockClient.beatsPrompts))
	}
	// 确认 prompt 中不包含分卷前瞻
	if strings.Contains(mockClient.beatsPrompts[0], "【卷内节奏前瞻") {
		t.Errorf("userPrompt should not contain rhythm foresight when CurrentVolume is nil")
	}
}

// 验证五种章型指令在 DeriveBeatsForChapterType 中的正确注入
func TestOrchestrator_DeriveBeatsForChapterType_AllTypesGuidance(t *testing.T) {
	beatsJSON := `{"beats":[],"state_mutation":{"inventory_delta":"","power_delta":""}}`
	mockClient := &mockRhythmLLMClient{beatsJSON: beatsJSON}
	orch := NewOrchestrator(mockClient)

	horizon := &CanonHorizon{
		Project:       &domain.Project{Title: "仙朝"},
		TargetChapter: 1,
	}

	types := []domain.ChapterType{
		domain.ChapterTypeClimax,
		domain.ChapterTypeAftermath,
		domain.ChapterTypeSetup,
		domain.ChapterTypeInterlude,
		domain.ChapterTypeEscalation,
	}

	for _, ct := range types {
		mockClient.beatsSys = nil
		_, err := orch.DeriveBeatsForChapterType(context.Background(), "mock-model", horizon, "冲突", ct)
		if err != nil {
			t.Fatalf("type %s derivation failed: %v", ct, err)
		}
		if len(mockClient.beatsSys) != 1 {
			t.Fatalf("expected 1 system prompt recorded, got %d", len(mockClient.beatsSys))
		}
		sys := mockClient.beatsSys[0]
		if !strings.Contains(sys, string(ct)) {
			t.Errorf("expected system prompt to contain chapter type %s, got %s", ct, sys)
		}
	}
}
