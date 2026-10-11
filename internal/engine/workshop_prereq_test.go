package engine

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockPrereqLLMClient struct {
	mu          sync.Mutex
	calls       int
	callPrompts []string
}

func (m *mockPrereqLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	m.mu.Lock()
	m.calls++
	m.callPrompts = append(m.callPrompts, userPrompt)
	m.mu.Unlock()

	if strings.Contains(systemPrompt, "分卷分节架构师") {
		from, to := 1, 1
		if sm := regexp.MustCompile(`章节范围：第 (\d+) 章 ~ 第 (\d+) 章`).FindStringSubmatch(userPrompt); len(sm) == 3 {
			from, _ = strconv.Atoi(sm[1])
			to, _ = strconv.Atoi(sm[2])
		}
		return fmt.Sprintf(`{"sections":[{"title":"主线推进","from_chapter":%d,"to_chapter":%d,"goal":"推进本卷主线","key_events":["关键推进"]}]}`, from, to), nil
	}
	if strings.Contains(systemPrompt, "剧情规划师") {
		var cards []string
		if sm := regexp.MustCompile(`(?:请为以下目标章号逐章推演意图卡：|推演意图卡：)(.*)`).FindStringSubmatch(userPrompt); len(sm) == 2 {
			chStrs := strings.Split(strings.TrimSpace(sm[1]), ",")
			for _, chStr := range chStrs {
				chNum, _ := strconv.Atoi(strings.TrimSpace(chStr))
				if chNum > 0 {
					cards = append(cards, fmt.Sprintf(`{"chapter_index":%d,"conflict":"推进主线冲突","tension_target":5}`, chNum))
				}
			}
		}
		if len(cards) == 0 {
			cards = append(cards, `{"chapter_index":1,"conflict":"推进主线冲突","tension_target":5}`)
		}
		return fmt.Sprintf("[%s]", strings.Join(cards, ",")), nil
	}
	if strings.Contains(systemPrompt, "状态对账") {
		return `{"consistent":true,"reconciliation_required":false,"observations":[]}`, nil
	}
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
	if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
		return `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"通过"}`, nil
	}
	if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
		return `{
			"beats": [
				{"phase": "蓄力", "tension": 6, "action": "对峙", "expectation_broken": "反派失算"},
				{"phase": "破局", "tension": 9, "action": "逆转", "expectation_broken": "全面击溃"}
			],
			"state_mutation": {"inventory_delta": "+战利品", "power_delta": "突破"}
		}`, nil
	}
	return "夜色渐深，冷风呼啸。林凡拔出青玉长剑，一剑断去黑衣人退路。", nil
}

func (m *mockPrereqLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 60, CompletionTokens: 80, TotalTokens: 140}, err
}

func (m *mockPrereqLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 60, CompletionTokens: 80, TotalTokens: 140}}
	close(ch)
	return ch, nil
}

// 1. VolumeArcForChapter 表驱动测试（命中/未命中/Start-End 留空由 Backfill 补/框架 nil）
func TestVolumeArcForChapter(t *testing.T) {
	tests := []struct {
		name         string
		framework    *domain.ProjectFramework
		chapterIndex int
		wantFound    bool
		wantVolIndex int
	}{
		{
			name:         "framework is nil",
			framework:    nil,
			chapterIndex: 1,
			wantFound:    false,
		},
		{
			name:         "empty volume arcs",
			framework:    &domain.ProjectFramework{},
			chapterIndex: 1,
			wantFound:    false,
		},
		{
			name: "chapterIndex <= 0",
			framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
				},
			},
			chapterIndex: 0,
			wantFound:    false,
		},
		{
			name: "hit explicit range",
			framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
					{VolumeIndex: 2, StartChapter: 11, EndChapter: 20},
				},
			},
			chapterIndex: 15,
			wantFound:    true,
			wantVolIndex: 2,
		},
		{
			name: "miss explicit range",
			framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
				},
			},
			chapterIndex: 15,
			wantFound:    false,
		},
		{
			name: "hit with Start-End blank backfilled",
			framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, EstimatedChapters: 10}, // 1..10
					{VolumeIndex: 2, EstimatedChapters: 20}, // 11..30
				},
			},
			chapterIndex: 25,
			wantFound:    true,
			wantVolIndex: 2,
		},
		{
			name: "miss with Start-End blank backfilled",
			framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, EstimatedChapters: 10}, // 1..10
				},
			},
			chapterIndex: 15,
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VolumeArcForChapter(tt.framework, tt.chapterIndex)
			if !tt.wantFound {
				if got != nil {
					t.Fatalf("expected nil arc, got: %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected found arc, got nil")
			}
			if got.VolumeIndex != tt.wantVolIndex {
				t.Fatalf("expected volume index %d, got %d", tt.wantVolIndex, got.VolumeIndex)
			}
		})
	}
}

// 2. EnsureVolumeIntents 测试：全齐 → 零 LLM 调用；缺章 → 生成+SaveChapterIntent 持久化+usage>0；卷不存在 → error；>60 章分块
func TestEnsureVolumeIntents_FailClosed(t *testing.T) {
	ctx := context.Background()
	proj := &domain.Project{
		ID:    "p-prereq-fail",
		Title: "测试工程",
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	client := &mockPrereqLLMClient{}
	orch := NewOrchestrator(client)

	// workshop nil
	var nilWs *ChapterWorkshop
	if _, err := nilWs.EnsureVolumeIntents(ctx, "p-prereq-fail", 1, "model"); err == nil {
		t.Fatalf("expected error for nil workshop")
	}

	// orch nil
	wsOrchNil := &ChapterWorkshop{store: mockStore}
	if _, err := wsOrchNil.EnsureVolumeIntents(ctx, "p-prereq-fail", 1, "model"); err == nil {
		t.Fatalf("expected error for nil orchestrator")
	}

	// store nil
	wsStoreNil := &ChapterWorkshop{orch: orch}
	if _, err := wsStoreNil.EnsureVolumeIntents(ctx, "p-prereq-fail", 1, "model"); err == nil {
		t.Fatalf("expected error for nil store")
	}

	// project not found
	ws := &ChapterWorkshop{orch: orch, store: mockStore}
	if _, err := ws.EnsureVolumeIntents(ctx, "non-existent-proj", 1, "model"); err == nil {
		t.Fatalf("expected error for non-existent project")
	}

	// project framework nil
	if _, err := ws.EnsureVolumeIntents(ctx, "p-prereq-fail", 1, "model"); err == nil {
		t.Fatalf("expected error for project with nil framework")
	}

	// volume index not found
	proj.Framework = &domain.ProjectFramework{
		VolumeArcs: []domain.VolumeArc{
			{VolumeIndex: 1, StartChapter: 1, EndChapter: 5},
		},
	}
	if _, err := ws.EnsureVolumeIntents(ctx, "p-prereq-fail", 99, "model"); err == nil {
		t.Fatalf("expected error for non-existent volume index")
	}
}

func TestEnsureVolumeIntents_AllPresent_ZeroCalls(t *testing.T) {
	ctx := context.Background()
	proj := &domain.Project{
		ID:    "p-intents-all",
		Title: "意图全齐测试",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{VolumeIndex: 1, StartChapter: 1, EndChapter: 3},
			},
		},
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	for ch := 1; ch <= 3; ch++ {
		_ = mockStore.SaveChapterIntent(ctx, &domain.ChapterIntent{
			ProjectID:     "p-intents-all",
			VolumeIndex:   1,
			ChapterIndex:  ch,
			Conflict:      "既有意图",
			TensionTarget: 5,
			Status:        domain.IntentStatusDraft,
		})
	}

	client := &mockPrereqLLMClient{}
	orch := NewOrchestrator(client)
	ws := &ChapterWorkshop{orch: orch, store: mockStore}

	usage, err := ws.EnsureVolumeIntents(ctx, "p-intents-all", 1, "reasoning-model")
	if err != nil {
		t.Fatalf("EnsureVolumeIntents failed: %v", err)
	}
	if usage.TotalTokens != 0 {
		t.Errorf("expected 0 token usage when all intents present, got: %+v", usage)
	}
	if client.calls != 0 {
		t.Errorf("expected 0 LLM calls when all intents present, got: %d", client.calls)
	}
}

func TestEnsureVolumeIntents_MissingChapters_GeneratesAndPersists(t *testing.T) {
	ctx := context.Background()
	proj := &domain.Project{
		ID:    "p-intents-missing",
		Title: "意图补齐测试",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{VolumeIndex: 1, StartChapter: 1, EndChapter: 3},
			},
		},
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	// 仅存入第 2 章，缺少第 1、3 章
	_ = mockStore.SaveChapterIntent(ctx, &domain.ChapterIntent{
		ProjectID:     "p-intents-missing",
		VolumeIndex:   1,
		ChapterIndex:  2,
		Conflict:      "已有第2章意图",
		TensionTarget: 5,
		Status:        domain.IntentStatusDraft,
	})

	client := &mockPrereqLLMClient{}
	orch := NewOrchestrator(client)
	ws := &ChapterWorkshop{orch: orch, store: mockStore}

	usage, err := ws.EnsureVolumeIntents(ctx, "p-intents-missing", 1, "reasoning-model")
	if err != nil {
		t.Fatalf("EnsureVolumeIntents failed: %v", err)
	}
	if usage.TotalTokens <= 0 {
		t.Errorf("expected non-zero token usage, got: %+v", usage)
	}
	if client.calls != 1 {
		t.Errorf("expected 1 LLM call to fill missing intents, got: %d", client.calls)
	}

	// 验证全部 3 章意图卡均已持久化
	allIntents, err := mockStore.ListChapterIntents(ctx, "p-intents-missing", 1)
	if err != nil {
		t.Fatalf("ListChapterIntents failed: %v", err)
	}
	if len(allIntents) != 3 {
		t.Fatalf("expected 3 persisted intents, got %d", len(allIntents))
	}
}

func TestEnsureVolumeIntents_ChunkingOver60(t *testing.T) {
	ctx := context.Background()
	proj := &domain.Project{
		ID:    "p-intents-chunk65",
		Title: "超过60章分块推演测试",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "超长大卷",
					EstimatedChapters: 65, // Start: 1, End: 65 (65 > 60)
				},
			},
		},
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	client := &mockPrereqLLMClient{}
	orch := NewOrchestrator(client)
	ws := &ChapterWorkshop{orch: orch, store: mockStore}

	usage, err := ws.EnsureVolumeIntents(ctx, "p-intents-chunk65", 1, "reasoning-model")
	if err != nil {
		t.Fatalf("EnsureVolumeIntents should not fail on 65 chapters due to chunking, got: %v", err)
	}
	// 65 章按 ≤60 分块应调用 2 次 (60 + 5)
	if client.calls != 2 {
		t.Fatalf("expected 2 chunked LLM calls for 65 chapters, got %d", client.calls)
	}
	if usage.TotalTokens != 280 {
		t.Errorf("expected total tokens 280 (140 * 2), got: %d", usage.TotalTokens)
	}

	allIntents, err := mockStore.ListChapterIntents(ctx, "p-intents-chunk65", 1)
	if err != nil {
		t.Fatalf("ListChapterIntents failed: %v", err)
	}
	if len(allIntents) != 65 {
		t.Fatalf("expected 65 persisted intents, got %d", len(allIntents))
	}
}

// 3. EnsureProductionPrerequisites 测试：框架 nil → 零调用；章越界 → 零调用；命中且皆缺 → 意图+分节两次 LLM 调用；分节已存在 → 仅意图调用
func TestEnsureProductionPrerequisites(t *testing.T) {
	ctx := context.Background()

	t.Run("framework nil returns zero calls", func(t *testing.T) {
		proj := &domain.Project{ID: "p-prereq-nofw", Title: "无框架"}
		mockStore := newMockSummaryWorkshopStore(proj)
		client := &mockPrereqLLMClient{}
		ws := &ChapterWorkshop{orch: NewOrchestrator(client), store: mockStore}

		usage, err := ws.EnsureProductionPrerequisites(ctx, "p-prereq-nofw", 1, "model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.TotalTokens != 0 || client.calls != 0 {
			t.Fatalf("expected 0 calls and 0 tokens, got calls=%d, usage=%+v", client.calls, usage)
		}
	})

	t.Run("chapter out of bounds returns zero calls", func(t *testing.T) {
		proj := &domain.Project{
			ID:    "p-prereq-oob",
			Title: "越界测试",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
				},
			},
		}
		mockStore := newMockSummaryWorkshopStore(proj)
		client := &mockPrereqLLMClient{}
		ws := &ChapterWorkshop{orch: NewOrchestrator(client), store: mockStore}

		usage, err := ws.EnsureProductionPrerequisites(ctx, "p-prereq-oob", 99, "model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.TotalTokens != 0 || client.calls != 0 {
			t.Fatalf("expected 0 calls and 0 tokens, got calls=%d, usage=%+v", client.calls, usage)
		}
	})

	t.Run("hit and both missing calls intents and sections", func(t *testing.T) {
		proj := &domain.Project{
			ID:    "p-prereq-both",
			Title: "意图与分节皆缺",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, StartChapter: 1, EndChapter: 3},
				},
			},
		}
		mockStore := newMockSummaryWorkshopStore(proj)
		client := &mockPrereqLLMClient{}
		ws := &ChapterWorkshop{orch: NewOrchestrator(client), store: mockStore}

		usage, err := ws.EnsureProductionPrerequisites(ctx, "p-prereq-both", 1, "model")
		if err != nil {
			t.Fatalf("EnsureProductionPrerequisites failed: %v", err)
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 LLM calls (intents + sections), got %d", client.calls)
		}
		if usage.TotalTokens != 280 {
			t.Errorf("expected 280 tokens (140 * 2), got %d", usage.TotalTokens)
		}

		// 验证 store 中分节与意图卡均已持久化
		savedProj, err := mockStore.GetProject(ctx, "p-prereq-both")
		if err != nil || savedProj == nil || len(savedProj.Framework.VolumeArcs[0].Sections) == 0 {
			t.Fatalf("expected volume sections to be persisted in project")
		}
		intents, err := mockStore.ListChapterIntents(ctx, "p-prereq-both", 1)
		if err != nil || len(intents) != 3 {
			t.Fatalf("expected 3 intents persisted, got %d", len(intents))
		}
	})

	t.Run("sections already exist only calls intents", func(t *testing.T) {
		proj := &domain.Project{
			ID:    "p-prereq-sec-exist",
			Title: "分节已存在",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{
						VolumeIndex:  1,
						StartChapter: 1,
						EndChapter:   3,
						Sections: []domain.VolumeSection{
							{Title: "已有分节", FromChapter: 1, ToChapter: 3, Goal: "目标"},
						},
					},
				},
			},
		}
		mockStore := newMockSummaryWorkshopStore(proj)
		client := &mockPrereqLLMClient{}
		ws := &ChapterWorkshop{orch: NewOrchestrator(client), store: mockStore}

		usage, err := ws.EnsureProductionPrerequisites(ctx, "p-prereq-sec-exist", 1, "model")
		if err != nil {
			t.Fatalf("EnsureProductionPrerequisites failed: %v", err)
		}
		if client.calls != 1 {
			t.Fatalf("expected exactly 1 LLM call (only intents), got %d", client.calls)
		}
		if usage.TotalTokens != 140 {
			t.Errorf("expected 140 tokens, got %d", usage.TotalTokens)
		}
	})
}

// 4. 集成（变异靶点）：含框架+卷弧+缺意图/分节的项目跑 ProduceChapter → 断言运行后 store 中 Sections 非空、意图卡已持久化、res.TotalUsage 含前置 usage
func TestChapterWorkshop_EnsureProductionPrerequisites_Integration(t *testing.T) {
	ctx := context.Background()
	proj := &domain.Project{
		ID:             "p-prereq-integrate",
		Title:          "生产前置集成验证",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "第一卷 仙门初试",
					CoreGoal:          "拜入宗门",
					Climax:            "大比夺魁",
					StartChapter:      1,
					EndChapter:        2,
					EstimatedChapters: 2,
				},
			},
		},
	}
	mockStore := newMockSummaryWorkshopStore(proj)
	client := &mockPrereqLLMClient{}
	orch := NewOrchestrator(client)
	qGate := NewQualityGate(client, nil)
	chronicle := NewCanonChronicle(mockStore)
	ws := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	req := WorkshopProduceRequest{
		ProjectID:       "p-prereq-integrate",
		ChapterIndex:    1,
		CoreConflict:    "宗门叩关",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
	}

	res, err := ws.ProduceChapter(ctx, req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}
	if !res.Committed {
		t.Fatalf("expected chapter 1 to be committed")
	}

	// 核心断言 1：Store 中 Sections 非空（生产前置持久化）
	savedProj, err := mockStore.GetProject(ctx, "p-prereq-integrate")
	if err != nil || savedProj == nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if savedProj.Framework == nil || len(savedProj.Framework.VolumeArcs) == 0 {
		t.Fatalf("saved project framework or volume arcs is nil/empty")
	}
	if len(savedProj.Framework.VolumeArcs[0].Sections) == 0 {
		t.Fatalf("expected persisted volume sections to be non-empty after ProduceChapter")
	}

	// 核心断言 2：Store 中章节意图卡已持久化
	intents, err := mockStore.ListChapterIntents(ctx, "p-prereq-integrate", 1)
	if err != nil {
		t.Fatalf("ListChapterIntents failed: %v", err)
	}
	if len(intents) == 0 {
		t.Fatalf("expected persisted chapter intents to be non-empty after ProduceChapter")
	}

	// 核心断言 3：res.TotalUsage 包含了前置调用的 token usage（至少意图 140 + 分节 140 = 280 tokens）
	// 前序正常流程为 6 次 (840) + 前置 2 次 (280) = 1120 tokens
	if res.TotalUsage.TotalTokens < 280 {
		t.Fatalf("expected res.TotalUsage.TotalTokens >= 280 to include prerequisites, got %d", res.TotalUsage.TotalTokens)
	}
}
