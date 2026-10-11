package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestFilterResolvedHookClaims_TableDriven(t *testing.T) {
	activeHooks := []*domain.PlotHook{
		{ID: "hook-1", Title: "残铁伏笔", Status: domain.HookStatusOpen},
		{ID: "hook-2", Title: "青云玉佩", Status: domain.HookStatusOpen},
		{ID: "hook-3", Title: "密匣真相", Status: domain.HookStatusOpen},
	}
	canonDraft := "长夜漫漫，林玄拔出青玉长剑，一剑断去黑衣人退路，同时揭开了密匣背后的身世之谜，并引动了古老阵法运转。"

	tests := []struct {
		name              string
		claims            []domain.HookResolutionClaim
		hooks             []*domain.PlotHook
		draft             string
		wantValidIDs      []string
		wantRejectReasons []string
	}{
		{
			name: "全部通过（保序）",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-2", Quote: "一剑断去黑衣人退路"},
				{ID: "hook-1", Quote: "林玄拔出青玉长剑"},
				{ID: "hook-3", Quote: "揭开了密匣背后的身世之谜"},
			},
			hooks:             activeHooks,
			draft:             canonDraft,
			wantValidIDs:      []string{"hook-2", "hook-1", "hook-3"},
			wantRejectReasons: nil,
		},
		{
			name: "unknown-id",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-unknown-999", Quote: "一剑断去黑衣人退路"},
			},
			hooks:             activeHooks,
			draft:             canonDraft,
			wantValidIDs:      nil,
			wantRejectReasons: []string{RejectionUnknownHookID},
		},
		{
			name: "missing-hook-id",
			claims: []domain.HookResolutionClaim{
				{ID: "", Quote: "一剑断去黑衣人退路"},
			},
			hooks:             activeHooks,
			draft:             canonDraft,
			wantValidIDs:      nil,
			wantRejectReasons: []string{RejectionMissingHookID},
		},
		{
			name: "quote 过短",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-1", Quote: "青玉长剑"}, // 4 runes < 8
			},
			hooks:             activeHooks,
			draft:             canonDraft,
			wantValidIDs:      nil,
			wantRejectReasons: []string{RejectionInsufficientEvidence},
		},
		{
			name: "quote 不在正文",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-1", Quote: "魔教尊者早已在千里之外遁走"}, // 13 runes >= 8, not in draft
			},
			hooks:             activeHooks,
			draft:             canonDraft,
			wantValidIDs:      nil,
			wantRejectReasons: []string{RejectionEvidenceNotInDraft},
		},
		{
			name: "重复声明",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-1", Quote: "林玄拔出青玉长剑"},
				{ID: "hook-1", Quote: "一剑断去黑衣人退路"},
			},
			hooks:             activeHooks,
			draft:             canonDraft,
			wantValidIDs:      []string{"hook-1"},
			wantRejectReasons: []string{RejectionDuplicateClaim},
		},
		{
			name: "混合（部分通过）",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-1", Quote: "林玄拔出青玉长剑"},     // valid
				{ID: "ghost-id", Quote: "一剑断去黑衣人退路"},  // unknown-id
				{ID: "hook-2", Quote: "断去退路"},         // quote < 8 -> insufficient-evidence
				{ID: "hook-3", Quote: "九天雷劫撕裂虚空深处之门"}, // quote not in draft -> evidence-not-in-draft
				{ID: "hook-1", Quote: "揭开了密匣背后的身世之谜"}, // duplicate-claim
			},
			hooks:        activeHooks,
			draft:        canonDraft,
			wantValidIDs: []string{"hook-1"},
			wantRejectReasons: []string{
				RejectionUnknownHookID,
				RejectionInsufficientEvidence,
				RejectionEvidenceNotInDraft,
				RejectionDuplicateClaim,
			},
		},
		{
			name: "nil hooks",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-1", Quote: "林玄拔出青玉长剑"},
			},
			hooks:             []*domain.PlotHook{nil, {ID: "hook-1"}, nil},
			draft:             canonDraft,
			wantValidIDs:      []string{"hook-1"},
			wantRejectReasons: nil,
		},
		{
			name: "空正文",
			claims: []domain.HookResolutionClaim{
				{ID: "hook-1", Quote: "林玄拔出青玉长剑"},
				{ID: "hook-2", Quote: "一剑断去黑衣人退路"},
			},
			hooks:             activeHooks,
			draft:             "",
			wantValidIDs:      nil,
			wantRejectReasons: []string{RejectionInsufficientEvidence, RejectionInsufficientEvidence},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotIDs, gotRejected := FilterResolvedHookClaims(tc.claims, tc.hooks, tc.draft)

			// Assert valid IDs
			if len(gotIDs) != len(tc.wantValidIDs) {
				t.Fatalf("validIDs count mismatch: got %v, want %v", gotIDs, tc.wantValidIDs)
			}
			for i := range gotIDs {
				if gotIDs[i] != tc.wantValidIDs[i] {
					t.Errorf("validIDs[%d] = %q, want %q", i, gotIDs[i], tc.wantValidIDs[i])
				}
			}

			// Assert reject reasons
			if len(gotRejected) != len(tc.wantRejectReasons) {
				t.Fatalf("rejected count mismatch: got %d (%+v), want %d (%v)",
					len(gotRejected), gotRejected, len(tc.wantRejectReasons), tc.wantRejectReasons)
			}
			for i, r := range gotRejected {
				if r.Reason != tc.wantRejectReasons[i] {
					t.Errorf("rejected[%d].Reason = %q, want %q", i, r.Reason, tc.wantRejectReasons[i])
				}
			}
		})
	}
}

// TestWorkshop_HookResolutionEvidenceGate_Integration tests Workshop integration with ReviewCourt mock.
// Reuses mockWorkshopLLMClient and mockChronicleStore from workshop_test.go.
func TestWorkshop_HookResolutionEvidenceGate_Integration(t *testing.T) {
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

	t.Run("quote 不在正文", func(t *testing.T) {
		reviewNotInDraft := `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "表现良好",
			"resolved_hooks": [{"id": "hook-101", "quote": "此片段完全不存在于正文草稿中"}]
		}`

		mockClient := &mockWorkshopLLMClient{
			beatsJSON:   beatsResp,
			renderDraft: renderDraft,
			reviewJSON:  reviewNotInDraft,
		}

		orch := NewOrchestrator(mockClient)
		qGate := NewQualityGate(mockClient, nil)

		mockStore := &mockChronicleStore{
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
					ID:            "hook-101",
					ProjectID:     "p-ws-hook",
					Title:         "残铁古剑伏笔",
					Status:        domain.HookStatusOpen,
					TargetChapter: 1,
					Details:       "残铁古剑的来历与秘密",
				},
			},
		}

		chronicle := NewCanonChronicle(mockStore)
		workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

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

		var events []WorkshopEvent
		req.OnProgress = func(ev WorkshopEvent) {
			events = append(events, ev)
		}

		res, err := workshop.ProduceChapter(context.Background(), req)
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		// 断言 1: quote 不在正文 → res.Audit.ResolvedHookIDs 为空
		if len(res.Audit.ResolvedHookIDs) != 0 {
			t.Errorf("expected empty ResolvedHookIDs when quote not in draft, got %v", res.Audit.ResolvedHookIDs)
		}

		// 断言 2: Issues 含 【伏笔兑现证据不足】
		foundIssue := false
		for _, iss := range res.Audit.Issues {
			if strings.Contains(iss, "【伏笔兑现证据不足】") {
				foundIssue = true
				break
			}
		}
		if !foundIssue {
			t.Errorf("expected Issues to contain 【伏笔兑现证据不足】, got %v", res.Audit.Issues)
		}

		// 断言 3: 事件含 "伏笔回收证据门"
		foundEvent := false
		for _, ev := range events {
			if strings.Contains(ev.Message, "伏笔回收证据门") {
				foundEvent = true
				break
			}
		}
		if !foundEvent {
			t.Errorf("expected events to contain 伏笔回收证据门, got %v", events)
		}
	})

	t.Run("quote 在正文", func(t *testing.T) {
		reviewInDraft := `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "表现良好",
			"resolved_hooks": [{"id": "hook-101", "quote": "残铁古剑在这一刻爆发出璀璨神辉"}]
		}`

		mockClient := &mockWorkshopLLMClient{
			beatsJSON:   beatsResp,
			renderDraft: renderDraft,
			reviewJSON:  reviewInDraft,
		}

		orch := NewOrchestrator(mockClient)
		qGate := NewQualityGate(mockClient, nil)

		mockStore := &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-ws-hook2",
				Title:          "万域封神",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "杀生夺灵，因果必报",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-ws-hook2_pro",
					ProjectID: "p-ws-hook2",
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
					ID:            "hook-101",
					ProjectID:     "p-ws-hook2",
					Title:         "残铁古剑伏笔",
					Status:        domain.HookStatusOpen,
					TargetChapter: 1,
					Details:       "残铁古剑的来历与秘密",
				},
			},
		}

		chronicle := NewCanonChronicle(mockStore)
		workshop := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

		req := WorkshopProduceRequest{
			ProjectID:       "p-ws-hook2",
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

		// 断言 1: quote 在正文 → ID 保留
		if len(res.Audit.ResolvedHookIDs) != 1 || res.Audit.ResolvedHookIDs[0] != "hook-101" {
			t.Errorf("expected ResolvedHookIDs to contain hook-101, got %v", res.Audit.ResolvedHookIDs)
		}

		// 断言 2: 无拒绝 issue
		for _, iss := range res.Audit.Issues {
			if strings.Contains(iss, "【伏笔兑现证据不足】") {
				t.Errorf("unexpected 伏笔兑现证据不足 issue when quote in draft: %s", iss)
			}
		}
	})
}
