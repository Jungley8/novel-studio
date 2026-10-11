package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestFilterPlantedHookCandidates_TableDriven(t *testing.T) {
	chapterText := "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！残铁古剑在这一刻爆发出璀璨神辉！"
	existingHooks := []*domain.PlotHook{
		{
			ID:            "h-1",
			Title:         "既有宗门旧怨",
			Status:        domain.HookStatusOpen,
			TargetChapter: 10,
		},
	}

	tests := []struct {
		name            string
		candidates      []domain.PlantedHookCandidate
		expectedValid   int
		expectedRej     int
		checkValid      func(t *testing.T, valid []domain.PlantedHookCandidate)
		checkRejections func(t *testing.T, rejected []PlantedHookRejection)
	}{
		{
			name: "通过",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "残铁古剑异动",
					Details:       "剑身爆发出璀璨神辉",
					Quote:         "残铁古剑在这一刻爆发出璀璨神辉！",
					TargetChapter: 6,
				},
			},
			expectedValid: 1,
			expectedRej:   0,
			checkValid: func(t *testing.T, valid []domain.PlantedHookCandidate) {
				if valid[0].Title != "残铁古剑异动" {
					t.Errorf("unexpected title: %s", valid[0].Title)
				}
				if valid[0].Quote != "残铁古剑在这一刻爆发出璀璨神辉！" {
					t.Errorf("unexpected quote: %s", valid[0].Quote)
				}
			},
		},
		{
			name: "title 空",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "   ",
					Quote:         "残铁古剑在这一刻爆发出璀璨神辉！",
					TargetChapter: 6,
				},
			},
			expectedValid: 0,
			expectedRej:   1,
			checkRejections: func(t *testing.T, rejected []PlantedHookRejection) {
				if rejected[0].Reason != RejectionMissingTitle {
					t.Errorf("expected reason %s, got %s", RejectionMissingTitle, rejected[0].Reason)
				}
			},
		},
		{
			name: "与既有伏笔重名",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "  既有宗门旧怨  ",
					Quote:         "残铁古剑在这一刻爆发出璀璨神辉！",
					TargetChapter: 6,
				},
			},
			expectedValid: 0,
			expectedRej:   1,
			checkRejections: func(t *testing.T, rejected []PlantedHookRejection) {
				if rejected[0].Reason != RejectionDuplicateTitle {
					t.Errorf("expected reason %s, got %s", RejectionDuplicateTitle, rejected[0].Reason)
				}
			},
		},
		{
			name: "批内重名",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "神秘黑衣修士首领",
					Quote:         "三名黑衣修士逼近洞府门前。",
					TargetChapter: 5,
				},
				{
					Title:         "神秘黑衣修士首领",
					Quote:         "阵纹骤亮，雷光咆哮如龙！",
					TargetChapter: 7,
				},
			},
			expectedValid: 1,
			expectedRej:   1,
			checkValid: func(t *testing.T, valid []domain.PlantedHookCandidate) {
				if valid[0].Title != "神秘黑衣修士首领" {
					t.Errorf("unexpected title: %s", valid[0].Title)
				}
			},
			checkRejections: func(t *testing.T, rejected []PlantedHookRejection) {
				if rejected[0].Reason != RejectionDuplicateTitle {
					t.Errorf("expected reason %s, got %s", RejectionDuplicateTitle, rejected[0].Reason)
				}
			},
		},
		{
			name: "quote <8 字",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "短引用伏笔",
					Quote:         "夜幕低垂。", // 5 runes
					TargetChapter: 6,
				},
			},
			expectedValid: 0,
			expectedRej:   1,
			checkRejections: func(t *testing.T, rejected []PlantedHookRejection) {
				if rejected[0].Reason != RejectionInsufficientEvidence {
					t.Errorf("expected reason %s, got %s", RejectionInsufficientEvidence, rejected[0].Reason)
				}
			},
		},
		{
			name: "quote 不在正文",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "虚假引用伏笔",
					Quote:         "这段引文完全不存在于当前章节正文中",
					TargetChapter: 6,
				},
			},
			expectedValid: 0,
			expectedRej:   1,
			checkRejections: func(t *testing.T, rejected []PlantedHookRejection) {
				if rejected[0].Reason != RejectionEvidenceNotInDraft {
					t.Errorf("expected reason %s, got %s", RejectionEvidenceNotInDraft, rejected[0].Reason)
				}
			},
		},
		{
			name: "trim 归一断言",
			candidates: []domain.PlantedHookCandidate{
				{
					Title:         "   空白边缘伏笔   ",
					Quote:         "   残铁古剑在这一刻爆发出璀璨神辉！   ",
					TargetChapter: 6,
				},
			},
			expectedValid: 1,
			expectedRej:   0,
			checkValid: func(t *testing.T, valid []domain.PlantedHookCandidate) {
				if valid[0].Title != "空白边缘伏笔" {
					t.Errorf("expected trimmed title, got %q", valid[0].Title)
				}
				if valid[0].Quote != "残铁古剑在这一刻爆发出璀璨神辉！" {
					t.Errorf("expected trimmed quote, got %q", valid[0].Quote)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, rejected := FilterPlantedHookCandidates(tt.candidates, chapterText, existingHooks)
			if len(valid) != tt.expectedValid {
				t.Fatalf("expected %d valid, got %d", tt.expectedValid, len(valid))
			}
			if len(rejected) != tt.expectedRej {
				t.Fatalf("expected %d rejected, got %d", tt.expectedRej, len(rejected))
			}
			if tt.checkValid != nil {
				tt.checkValid(t, valid)
			}
			if tt.checkRejections != nil {
				tt.checkRejections(t, rejected)
			}
		})
	}
}

func TestExtractPlantedHooks(t *testing.T) {
	ctx := context.Background()
	proj := &domain.Project{ID: "p-test", Title: "测试小说"}

	t.Run("nil client", func(t *testing.T) {
		_, _, err := ExtractPlantedHooks(ctx, nil, "model", proj, 1, "正文内容足够长", nil)
		if err == nil {
			t.Fatal("expected error on nil client")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		mockClient := &mockWorkshopLLMClient{}
		_, _, err := ExtractPlantedHooks(ctx, mockClient, "model", nil, 1, "正文内容足够长", nil)
		if err == nil {
			t.Fatal("expected error on nil project")
		}
	})

	t.Run("empty chapterText", func(t *testing.T) {
		mockClient := &mockWorkshopLLMClient{}
		_, _, err := ExtractPlantedHooks(ctx, mockClient, "model", proj, 1, "   ", nil)
		if err == nil {
			t.Fatal("expected error on empty chapter text")
		}
	})

	t.Run("normal extraction and target chapter normalization", func(t *testing.T) {
		mockClient := &mockWorkshopLLMClient{
			plantedHooksJSON: `[
				{
					"title": "伏笔一",
					"details": "线索细节",
					"quote": "正文引用一八个字以上",
					"target_chapter": 1
				},
				{
					"title": "伏笔二",
					"details": "线索细节",
					"quote": "正文引用二八个字以上",
					"target_chapter": 10
				}
			]`,
		}
		existing := []*domain.PlotHook{
			{Title: "旧伏笔", Status: domain.HookStatusOpen, TargetChapter: 3},
		}
		candidates, usage, err := ExtractPlantedHooks(ctx, mockClient, "model", proj, 1, "正文包含正文引用一八个字以上和正文引用二八个字以上", existing)
		if err != nil {
			t.Fatalf("ExtractPlantedHooks failed: %v", err)
		}
		if len(candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(candidates))
		}
		if candidates[0].TargetChapter != 6 { // chapterIndex 1 + 5 = 6
			t.Errorf("expected TargetChapter 6, got %d", candidates[0].TargetChapter)
		}
		if candidates[1].TargetChapter != 10 {
			t.Errorf("expected TargetChapter 10, got %d", candidates[1].TargetChapter)
		}
		if usage.TotalTokens == 0 {
			t.Errorf("expected non-zero usage")
		}
	})
}

type hookRegRecordingStore struct {
	*mockChronicleStore
	saved []*domain.PlotHook
}

func (s *hookRegRecordingStore) SavePlotHook(ctx context.Context, hook *domain.PlotHook) error {
	s.saved = append(s.saved, hook)
	return nil
}

func TestWorkshop_PlantedHookRegistration_Integration(t *testing.T) {
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
	renderDraft := "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！残铁古剑在这一刻爆发出璀璨神辉！"
	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "表现良好"
	}`

	plantedHooksJSON := `[
		{
			"title": "新发现的残铁古剑",
			"details": "剑身内部似乎封印着一缕残魂",
			"quote": "残铁古剑在这一刻爆发出璀璨神辉！",
			"target_chapter": 0
		},
		{
			"title": "虚假线索伏笔",
			"details": "这段引文根本不在正文中",
			"quote": "这段引用文本完全不存在于当前章节正文中",
			"target_chapter": 10
		},
		{
			"title": "既有伏笔标题",
			"details": "重复的伏笔内容",
			"quote": "残铁古剑在这一刻爆发出璀璨神辉！",
			"target_chapter": 8
		}
	]`

	t.Run("自动登记与证据门过滤", func(t *testing.T) {
		mockClient := &mockWorkshopLLMClient{
			beatsJSON:        beatsResp,
			renderDraft:      renderDraft,
			reviewJSON:       reviewResp,
			plantedHooksJSON: plantedHooksJSON,
		}

		orch := NewOrchestrator(mockClient)
		qGate := NewQualityGate(mockClient, nil)

		baseMockStore := &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-ws-hook",
				Title:          "万域封神",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀生夺灵，因果必报",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-ws-hook_pro",
					ProjectID: "p-ws-hook",
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "练气圆满",
					Inventory: "青玉短剑",
				},
			},
			chapters: []*domain.Chapter{},
			hooks: []*domain.PlotHook{
				{
					ID:             "hook-existing-1",
					ProjectID:      "p-ws-hook",
					Title:          "既有伏笔标题",
					Status:         domain.HookStatusOpen,
					CreatedChapter: 1,
					TargetChapter:  5,
				},
			},
		}

		recStore := &hookRegRecordingStore{
			mockChronicleStore: baseMockStore,
		}

		chronicle := NewCanonChronicle(recStore)
		workshop := NewChapterWorkshop(orch, chronicle, qGate, recStore)

		req := WorkshopProduceRequest{
			ProjectID:       "p-ws-hook",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭抢夺古经",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
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

		// 断言 1: saved 恰 1 条
		if len(recStore.saved) != 1 {
			t.Fatalf("expected 1 saved hook, got %d", len(recStore.saved))
		}

		savedHook := recStore.saved[0]
		if savedHook.Title != "新发现的残铁古剑" {
			t.Errorf("expected Title '新发现的残铁古剑', got %q", savedHook.Title)
		}
		if savedHook.Status != domain.HookStatusOpen {
			t.Errorf("expected Status HookStatusOpen, got %v", savedHook.Status)
		}
		if savedHook.CreatedChapter != 1 {
			t.Errorf("expected CreatedChapter 1, got %d", savedHook.CreatedChapter)
		}
		if savedHook.ProjectID != "p-ws-hook" {
			t.Errorf("expected ProjectID 'p-ws-hook', got %q", savedHook.ProjectID)
		}
		if savedHook.TargetChapter != 6 {
			t.Errorf("expected TargetChapter 6 (normalized from 0), got %d", savedHook.TargetChapter)
		}

		// 断言 2: Issues 含 【伏笔登记】 与 【伏笔登记被拒】
		hasRegisteredIssue := false
		hasRejection1 := false
		hasRejection2 := false
		for _, iss := range res.Audit.Issues {
			if strings.Contains(iss, "【伏笔登记】新增伏笔 1 条: 新发现的残铁古剑") {
				hasRegisteredIssue = true
			}
			if strings.Contains(iss, "【伏笔登记被拒】「虚假线索伏笔」: evidence-not-in-draft") {
				hasRejection1 = true
			}
			if strings.Contains(iss, "【伏笔登记被拒】「既有伏笔标题」: duplicate-title") {
				hasRejection2 = true
			}
		}

		if !hasRegisteredIssue {
			t.Errorf("expected 【伏笔登记】 issue in audit report, got issues: %v", res.Audit.Issues)
		}
		if !hasRejection1 {
			t.Errorf("expected rejection issue for 虚假线索伏笔, got issues: %v", res.Audit.Issues)
		}
		if !hasRejection2 {
			t.Errorf("expected rejection issue for 既有伏笔标题, got issues: %v", res.Audit.Issues)
		}
	})

	t.Run("非法 JSON 返回警告且不阻断归档", func(t *testing.T) {
		mockClient := &mockWorkshopLLMClient{
			beatsJSON:        beatsResp,
			renderDraft:      renderDraft,
			reviewJSON:       reviewResp,
			plantedHooksJSON: `{invalid json content`,
		}

		orch := NewOrchestrator(mockClient)
		qGate := NewQualityGate(mockClient, nil)

		baseMockStore := &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-ws-hook",
				Title:          "万域封神",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀生夺灵，因果必报",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-ws-hook_pro",
					ProjectID: "p-ws-hook",
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "练气圆满",
					Inventory: "青玉短剑",
				},
			},
			chapters: []*domain.Chapter{},
			hooks: []*domain.PlotHook{
				{
					ID:             "hook-existing-1",
					ProjectID:      "p-ws-hook",
					Title:          "既有伏笔标题",
					Status:         domain.HookStatusOpen,
					CreatedChapter: 1,
					TargetChapter:  5,
				},
			},
		}

		recStore := &hookRegRecordingStore{
			mockChronicleStore: baseMockStore,
		}

		chronicle := NewCanonChronicle(recStore)
		workshop := NewChapterWorkshop(orch, chronicle, qGate, recStore)

		req := WorkshopProduceRequest{
			ProjectID:       "p-ws-hook",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭抢夺古经",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      true,
			MaxRewriteLoops: 1,
		}

		res, err := workshop.ProduceChapter(context.Background(), req)
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if !res.Committed {
			t.Fatalf("expected chapter to be committed despite registration warning")
		}

		if len(recStore.saved) != 0 {
			t.Errorf("expected 0 saved hooks on invalid json, got %d", len(recStore.saved))
		}

		hasWarning := false
		for _, iss := range res.Audit.Issues {
			if strings.Contains(iss, "【伏笔登记警告】伏笔提取失败") {
				hasWarning = true
				break
			}
		}
		if !hasWarning {
			t.Errorf("expected 【伏笔登记警告】 issue in audit report, got: %v", res.Audit.Issues)
		}
	})
}
