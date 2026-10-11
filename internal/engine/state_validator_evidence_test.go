package engine

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestFilterStateObservations(t *testing.T) {
	draft := "林凡在石室角落里寻得一截降龙木，随后又拔出玄铁重剑握于掌中，气势如虹。"

	tests := []struct {
		name         string
		observations []StateValidationObservation
		draftText    string
		wantKept     []StateValidationObservation
		wantDropped  int
	}{
		{
			name: "1. 有效引用 → 保留",
			observations: []StateValidationObservation{
				{
					Code:     "valid-obs",
					Summary:  "有效观察",
					Evidence: []string{"在石室角落里寻得一截降龙木"},
				},
			},
			draftText: draft,
			wantKept: []StateValidationObservation{
				{
					Code:     "valid-obs",
					Summary:  "有效观察",
					Evidence: []string{"在石室角落里寻得一截降龙木"},
				},
			},
			wantDropped: 0,
		},
		{
			name: "2. 引用过短（<8 字）→ 丢弃",
			observations: []StateValidationObservation{
				{
					Code:     "short-obs",
					Summary:  "过短引用观察",
					Evidence: []string{"气势如虹"},
				},
			},
			draftText:   draft,
			wantKept:    nil,
			wantDropped: 1,
		},
		{
			name: "3. 引用不在正文 → 丢弃",
			observations: []StateValidationObservation{
				{
					Code:     "hallucinated-obs",
					Summary:  "幻觉引用观察",
					Evidence: []string{"林凡吞服了九转玄丹功力大增"},
				},
			},
			draftText:   draft,
			wantKept:    nil,
			wantDropped: 1,
		},
		{
			name: "4. 同条观察混合有效/无效引用 → 保留有效引用",
			observations: []StateValidationObservation{
				{
					Code:    "mixed-obs",
					Summary: "混合引用观察",
					Evidence: []string{
						"在石室角落里寻得一截降龙木", // 有效（>=8字且在正文）
						"气势如虹", // 过短（<8字）
						"林凡吞服了九转玄丹功力大增", // 幻觉（不在正文）
						"拔出玄铁重剑握于掌中",    // 有效（>=8字且在正文）
					},
				},
			},
			draftText: draft,
			wantKept: []StateValidationObservation{
				{
					Code:    "mixed-obs",
					Summary: "混合引用观察",
					Evidence: []string{
						"在石室角落里寻得一截降龙木",
						"拔出玄铁重剑握于掌中",
					},
				},
			},
			wantDropped: 0,
		},
		{
			name: "5. 全部观察无有效引用 → kept 空 + droppedCount 正确",
			observations: []StateValidationObservation{
				{
					Code:     "bad-obs-1",
					Summary:  "过短引用",
					Evidence: []string{"短引用"},
				},
				{
					Code:     "bad-obs-2",
					Summary:  "幻觉引用",
					Evidence: []string{"完全不存在于正文里的长引用描述片段"},
				},
				{
					Code:     "bad-obs-3",
					Summary:  "无引用",
					Evidence: []string{},
				},
			},
			draftText:   draft,
			wantKept:    nil,
			wantDropped: 3,
		},
		{
			name:         "6. 空 observations",
			observations: []StateValidationObservation{},
			draftText:    draft,
			wantKept:     nil,
			wantDropped:  0,
		},
		{
			name: "7. 空正文",
			observations: []StateValidationObservation{
				{
					Code:     "obs-1",
					Summary:  "正文为空时观察",
					Evidence: []string{"拔出玄铁重剑握于掌中"},
				},
				{
					Code:     "obs-2",
					Summary:  "正文为空时观察2",
					Evidence: []string{"在石室角落里寻得一截降龙木"},
				},
			},
			draftText:   "",
			wantKept:    nil,
			wantDropped: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKept, gotDropped := filterStateObservations(tt.observations, tt.draftText)
			if gotDropped != tt.wantDropped {
				t.Errorf("filterStateObservations() gotDropped = %d, want %d", gotDropped, tt.wantDropped)
			}
			if len(gotKept) == 0 && len(tt.wantKept) == 0 {
				// Both empty/nil, acceptable
				return
			}
			if !reflect.DeepEqual(gotKept, tt.wantKept) {
				t.Errorf("filterStateObservations() gotKept = %+v, want %+v", gotKept, tt.wantKept)
			}
		})
	}
}

func TestWorkshop_StateObservationEvidenceGate(t *testing.T) {
	draft := "林凡在石室角落里寻得一截降龙木，随后又拔出玄铁重剑握于掌中，气势如虹。"
	beatsResp := `{"beats":[{"phase":"对决","tension":8,"action":"拔剑斩敌"}],"state_mutation":{"inventory_delta":"+玄铁重剑"}}`
	reviewResp := `{"verdict":"ACCEPTED","score":95,"issues":[],"suggestions":"通过"}`
	settleResp := `{
		"inventory_delta": "+降龙木",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{"field": "inventory", "quote": "在石室角落里寻得一截降龙木", "chapter_index": 1}
		]
	}`

	createWorkshop := func(mockClient *workshopReconcileMockClient, projectID string) *ChapterWorkshop {
		mockStore := &mockCheckpointWorkshopStore{
			mockChronicleStore: &mockChronicleStore{
				project: &domain.Project{
					ID:             projectID,
					Title:          "证据门集成测试",
					TargetPlatform: "番茄脑洞",
					WorldRules:     "杀伐果断",
				},
				codexEntries: []*domain.CodexEntry{
					{
						ID:        "codex_" + projectID + "_pro",
						ProjectID: projectID,
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
		return NewChapterWorkshop(orch, chronicle, qGate, mockStore)
	}

	t.Run("HallucinatedEvidence_DroppedAndReplacedByPlaceholder", func(t *testing.T) {
		valHallucinated := `{
			"consistent": false,
			"reconciliation_required": true,
			"observations": [
				{
					"code": "hallucinated-conflict",
					"summary": "主角在正文中服用了九转玄丹但未记录",
					"evidence": ["林凡服下了九转玄丹周身金光大盛"]
				}
			]
		}`

		mockClient := &workshopReconcileMockClient{
			beatsJSON:       beatsResp,
			renderDraft:     draft,
			reviewJSON:      reviewResp,
			settle1JSON:     settleResp,
			settle2JSON:     settleResp,
			validation1JSON: valHallucinated,
			validation2JSON: valHallucinated,
		}

		workshop := createWorkshop(mockClient, "p-valgate-hallucinated")

		req := WorkshopProduceRequest{
			ProjectID:       "p-valgate-hallucinated",
			ChapterIndex:    1,
			CoreConflict:    "夺取至宝",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-rev",
			WordsTarget:     0,
			AutoCommit:      false,
			MaxRewriteLoops: 1,
		}

		res, err := workshop.ProduceChapter(context.Background(), req)
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if len(res.StateValidationObservations) != 1 {
			t.Fatalf("expected exactly 1 StateValidationObservation, got %d (%+v)", len(res.StateValidationObservations), res.StateValidationObservations)
		}

		obs := res.StateValidationObservations[0]
		if obs.Code != "unverifiable-observation" {
			t.Errorf("expected observation code 'unverifiable-observation', got %q", obs.Code)
		}
		expectedSummary := "1 条观察因引用无法在正文中验证已被丢弃"
		if obs.Summary != expectedSummary {
			t.Errorf("expected observation summary %q, got %q", expectedSummary, obs.Summary)
		}
		if strings.Contains(obs.Summary, "hallucinated-conflict") || strings.Contains(obs.Summary, "九转玄丹") {
			t.Errorf("expected placeholder not to leak hallucinated details, got: %s", obs.Summary)
		}

		// 断言 Issues 含该占位摘要，且不含幻觉引用原文
		foundAuditPlaceholder := false
		for _, issue := range res.Audit.Issues {
			if strings.Contains(issue, "unverifiable-observation") && strings.Contains(issue, expectedSummary) {
				foundAuditPlaceholder = true
			}
			if strings.Contains(issue, "hallucinated-conflict") || strings.Contains(issue, "九转玄丹") {
				t.Errorf("expected audit issues not to contain hallucinated observation, but found: %s", issue)
			}
		}
		if !foundAuditPlaceholder {
			t.Errorf("expected Audit.Issues to contain placeholder observation issue, got: %v", res.Audit.Issues)
		}
	})

	t.Run("RealEvidence_PreservedAsIs", func(t *testing.T) {
		valReal := `{
			"consistent": false,
			"reconciliation_required": true,
			"observations": [
				{
					"code": "missing-weapon-obs",
					"summary": "正文白纸黑字提及玄铁重剑但未记录",
					"evidence": ["拔出玄铁重剑握于掌中"]
				}
			]
		}`

		mockClient := &workshopReconcileMockClient{
			beatsJSON:       beatsResp,
			renderDraft:     draft,
			reviewJSON:      reviewResp,
			settle1JSON:     settleResp,
			settle2JSON:     settleResp,
			validation1JSON: valReal,
			validation2JSON: valReal,
		}

		workshop := createWorkshop(mockClient, "p-valgate-real")

		req := WorkshopProduceRequest{
			ProjectID:       "p-valgate-real",
			ChapterIndex:    1,
			CoreConflict:    "夺取至宝",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-v3",
			ReviewerModel:   "mock-rev",
			WordsTarget:     0,
			AutoCommit:      false,
			MaxRewriteLoops: 1,
		}

		res, err := workshop.ProduceChapter(context.Background(), req)
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if len(res.StateValidationObservations) != 1 {
			t.Fatalf("expected exactly 1 StateValidationObservation, got %d (%+v)", len(res.StateValidationObservations), res.StateValidationObservations)
		}

		obs := res.StateValidationObservations[0]
		if obs.Code != "missing-weapon-obs" {
			t.Errorf("expected observation code 'missing-weapon-obs', got %q", obs.Code)
		}
		if obs.Summary != "正文白纸黑字提及玄铁重剑但未记录" {
			t.Errorf("expected observation summary '正文白纸黑字提及玄铁重剑但未记录', got %q", obs.Summary)
		}
		if len(obs.Evidence) != 1 || obs.Evidence[0] != "拔出玄铁重剑握于掌中" {
			t.Errorf("expected observation evidence ['拔出玄铁重剑握于掌中'], got %v", obs.Evidence)
		}
	})
}
