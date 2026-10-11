package engine

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockSectionsLLMClient struct {
	mu         sync.Mutex
	callCount  int
	response   string
	responseFn func(systemPrompt, userPrompt string) (string, error)
	err        error
}

func (m *mockSectionsLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	if m.responseFn != nil {
		return m.responseFn(systemPrompt, userPrompt)
	}
	return m.response, nil
}

func (m *mockSectionsLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	resp, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return resp, TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}, err
}

func (m *mockSectionsLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	resp, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: resp}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}}
	close(ch)
	return ch, nil
}

func TestValidateVolumeSections_TableDriven(t *testing.T) {
	baseArc := domain.VolumeArc{
		VolumeIndex:  1,
		Title:        "第一卷 仙路迢迢",
		StartChapter: 1,
		EndChapter:   30,
	}

	tests := []struct {
		name     string
		arc      domain.VolumeArc
		sections []domain.VolumeSection
		wantErr  bool
		errSub   string
	}{
		{
			name: "valid single section full coverage",
			arc: domain.VolumeArc{
				StartChapter: 1,
				EndChapter:   10,
			},
			sections: []domain.VolumeSection{
				{Title: "第1节 筑基", Goal: "突破筑基期", FromChapter: 1, ToChapter: 10},
			},
			wantErr: false,
		},
		{
			name: "valid 3 sections continuous coverage",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "第一节 初入仙门", Goal: "拜入外门并熟悉门规", FromChapter: 1, ToChapter: 10, KeyEvents: []string{"拜师", "分房"}},
				{Title: "第二节 外门试炼", Goal: "在试炼中崭露头角", FromChapter: 11, ToChapter: 20, KeyEvents: []string{"秘境探索", "夺得令牌"}},
				{Title: "第三节 宗门大比", Goal: "夺魁晋升内门", FromChapter: 21, ToChapter: 30, KeyEvents: []string{"擂台决战", "引动异象"}},
			},
			wantErr: false,
		},
		{
			name: "valid max 6 sections",
			arc: domain.VolumeArc{
				StartChapter: 1,
				EndChapter:   6,
			},
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 1},
				{Title: "S2", Goal: "G2", FromChapter: 2, ToChapter: 2},
				{Title: "S3", Goal: "G3", FromChapter: 3, ToChapter: 3},
				{Title: "S4", Goal: "G4", FromChapter: 4, ToChapter: 4},
				{Title: "S5", Goal: "G5", FromChapter: 5, ToChapter: 5},
				{Title: "S6", Goal: "G6", FromChapter: 6, ToChapter: 6},
			},
			wantErr: false,
		},
		{
			name:     "empty sections list",
			arc:      baseArc,
			sections: []domain.VolumeSection{},
			wantErr:  true,
			errSub:   "at least 1",
		},
		{
			name: "exceeds maximum 6 sections",
			arc: domain.VolumeArc{
				StartChapter: 1,
				EndChapter:   7,
			},
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 1},
				{Title: "S2", Goal: "G2", FromChapter: 2, ToChapter: 2},
				{Title: "S3", Goal: "G3", FromChapter: 3, ToChapter: 3},
				{Title: "S4", Goal: "G4", FromChapter: 4, ToChapter: 4},
				{Title: "S5", Goal: "G5", FromChapter: 5, ToChapter: 5},
				{Title: "S6", Goal: "G6", FromChapter: 6, ToChapter: 6},
				{Title: "S7", Goal: "G7", FromChapter: 7, ToChapter: 7},
			},
			wantErr: true,
			errSub:  "exceeds maximum of 6",
		},
		{
			name: "empty title in section",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "  ", Goal: "目标", FromChapter: 1, ToChapter: 30},
			},
			wantErr: true,
			errSub:  "title cannot be empty",
		},
		{
			name: "empty goal in section",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "节标题", Goal: "", FromChapter: 1, ToChapter: 30},
			},
			wantErr: true,
			errSub:  "goal cannot be empty",
		},
		{
			name: "fromChapter less than arc start",
			arc: domain.VolumeArc{
				StartChapter: 5,
				EndChapter:   15,
			},
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 4, ToChapter: 15},
			},
			wantErr: true,
			errSub:  "is before volume start",
		},
		{
			name: "toChapter exceeds arc end",
			arc: domain.VolumeArc{
				StartChapter: 1,
				EndChapter:   10,
			},
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 11},
			},
			wantErr: true,
			errSub:  "exceeds volume end",
		},
		{
			name: "fromChapter exceeds toChapter",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 15, ToChapter: 10},
			},
			wantErr: true,
			errSub:  "exceeds to_chapter",
		},
		{
			name: "first section does not match arc start",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 2, ToChapter: 30},
			},
			wantErr: true,
			errSub:  "does not match volume start",
		},
		{
			name: "coverage gap between sections",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 10},
				{Title: "S2", Goal: "G2", FromChapter: 12, ToChapter: 30}, // 缺少第 11 章
			},
			wantErr: true,
			errSub:  "gap or overlap",
		},
		{
			name: "coverage overlap between sections",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 15},
				{Title: "S2", Goal: "G2", FromChapter: 15, ToChapter: 30}, // 第 15 章重叠
			},
			wantErr: true,
			errSub:  "gap or overlap",
		},
		{
			name: "last section does not match arc end",
			arc:  baseArc,
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 29},
			},
			wantErr: true,
			errSub:  "does not match volume end",
		},
		{
			name: "invalid arc range",
			arc: domain.VolumeArc{
				StartChapter: 10,
				EndChapter:   5,
			},
			sections: []domain.VolumeSection{
				{Title: "S1", Goal: "G1", FromChapter: 1, ToChapter: 5},
			},
			wantErr: true,
			errSub:  "invalid volume arc chapter range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVolumeSections(tt.arc, tt.sections)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSub)
				}
				if tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("expected error containing %q, got: %v", tt.errSub, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestGenerateVolumeSections(t *testing.T) {
	arc := domain.VolumeArc{
		VolumeIndex:       1,
		Title:             "第一卷 初入修仙界",
		Theme:             "苟道长生",
		CoreGoal:          "拜入宗门筑基成功",
		Climax:            "筑基天劫与强敌伏杀",
		StartChapter:      1,
		EndChapter:        10,
		EstimatedChapters: 10,
		KeyPayoffs:        []string{"神秘玉佩的身世之谜"},
	}
	proj := &domain.Project{
		ID:             "p-test-sections",
		Title:          "长生修仙传",
		TargetPlatform: "起点仙侠",
		WorldRules:     "灵气衰退，强者恒强",
	}

	t.Run("nil client returns fail-closed error", func(t *testing.T) {
		_, _, err := GenerateVolumeSections(context.Background(), nil, "test-model", proj, arc)
		if err == nil {
			t.Fatal("expected error for nil client, got nil")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("valid JSON returns parsed and validated sections", func(t *testing.T) {
		validJSON := `{
			"sections": [
				{
					"title": "初临修仙界",
					"from_chapter": 1,
					"to_chapter": 4,
					"goal": "摸清修仙界法则并在坊市落脚",
					"key_events": ["落入荒山", "坊市租房"]
				},
				{
					"title": "坊市苟道",
					"from_chapter": 5,
					"to_chapter": 8,
					"goal": "制符积攒灵石准备拜门",
					"key_events": ["初学制符", "击退劫修"]
				},
				{
					"title": "宗门考核与筑基",
					"from_chapter": 9,
					"to_chapter": 10,
					"goal": "通过入门考核并顺利突破",
					"key_events": ["擂台考核", "破境筑基"]
				}
			]
		}`
		client := &mockSectionsLLMClient{response: validJSON}
		sections, usage, err := GenerateVolumeSections(context.Background(), client, "test-model", proj, arc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sections) != 3 {
			t.Fatalf("expected 3 sections, got %d", len(sections))
		}
		if sections[0].Title != "初临修仙界" || sections[0].FromChapter != 1 || sections[0].ToChapter != 4 {
			t.Errorf("section 0 mismatch: %+v", sections[0])
		}
		if sections[2].ToChapter != 10 {
			t.Errorf("section 2 end mismatch: %+v", sections[2])
		}
		if usage.TotalTokens == 0 {
			t.Errorf("expected non-zero usage, got %+v", usage)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		client := &mockSectionsLLMClient{response: "not a json string"}
		_, _, err := GenerateVolumeSections(context.Background(), client, "test-model", proj, arc)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})

	t.Run("invalid coverage returns validation error", func(t *testing.T) {
		invalidCoverageJSON := `{
			"sections": [
				{
					"title": "第一节",
					"from_chapter": 1,
					"to_chapter": 5,
					"goal": "阶段一"
				},
				{
					"title": "第二节断层",
					"from_chapter": 7,
					"to_chapter": 10,
					"goal": "阶段二"
				}
			]
		}`
		client := &mockSectionsLLMClient{response: invalidCoverageJSON}
		_, _, err := GenerateVolumeSections(context.Background(), client, "test-model", proj, arc)
		if err == nil {
			t.Fatal("expected error for discontinuous sections, got nil")
		}
		if !strings.Contains(err.Error(), "validate volume sections") {
			t.Errorf("expected validate volume sections error, got: %v", err)
		}
	})
}

func TestEnsureVolumeSections(t *testing.T) {
	projID := "p-ensure-sections"

	setupStoreAndWorkshop := func(client LLMClient, proj *domain.Project) (*ChapterWorkshop, *mockVolBatchStore) {
		s := newMockVolBatchStore(proj)
		orch := NewOrchestrator(client)
		chronicle := NewCanonChronicle(s)
		qGate := NewQualityGate(client, nil)
		ws := NewChapterWorkshop(orch, chronicle, qGate, s)
		return ws, s
	}

	t.Run("already has sections skips LLM call (idempotent)", func(t *testing.T) {
		proj := &domain.Project{
			ID:    projID,
			Title: "已有分节作品",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{
						VolumeIndex:  1,
						Title:        "第一卷",
						StartChapter: 1,
						EndChapter:   5,
						Sections: []domain.VolumeSection{
							{Title: "节1", Goal: "目标1", FromChapter: 1, ToChapter: 5},
						},
					},
				},
			},
		}
		client := &mockSectionsLLMClient{response: "should not be called"}
		ws, _ := setupStoreAndWorkshop(client, proj)

		usage, err := ws.EnsureVolumeSections(context.Background(), projID, 1, "test-model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.callCount != 0 {
			t.Errorf("expected 0 LLM calls for existing sections, got %d", client.callCount)
		}
		if usage.TotalTokens != 0 {
			t.Errorf("expected 0 usage for existing sections, got %+v", usage)
		}
	})

	t.Run("no sections generates and persists into store", func(t *testing.T) {
		proj := &domain.Project{
			ID:    projID,
			Title: "未分节作品",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{
						VolumeIndex:  1,
						Title:        "第一卷",
						StartChapter: 1,
						EndChapter:   5,
					},
				},
			},
		}
		genJSON := `{
			"sections": [
				{
					"title": "阶段一",
					"from_chapter": 1,
					"to_chapter": 3,
					"goal": "初入江湖",
					"key_events": ["拜师"]
				},
				{
					"title": "阶段二",
					"from_chapter": 4,
					"to_chapter": 5,
					"goal": "首战告捷",
					"key_events": ["胜出"]
				}
			]
		}`
		client := &mockSectionsLLMClient{response: genJSON}
		ws, s := setupStoreAndWorkshop(client, proj)

		usage, err := ws.EnsureVolumeSections(context.Background(), projID, 1, "test-model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.callCount != 1 {
			t.Errorf("expected 1 LLM call, got %d", client.callCount)
		}
		if usage.TotalTokens == 0 {
			t.Errorf("expected non-zero usage, got %+v", usage)
		}

		// Re-fetch project from store and assert Sections persisted
		savedProj, err := s.GetProject(context.Background(), projID)
		if err != nil {
			t.Fatalf("GetProject failed: %v", err)
		}
		if savedProj == nil || savedProj.Framework == nil || len(savedProj.Framework.VolumeArcs) == 0 {
			t.Fatalf("saved project or framework invalid")
		}
		arc := savedProj.Framework.VolumeArcs[0]
		if len(arc.Sections) != 2 {
			t.Fatalf("expected 2 persisted sections, got %d", len(arc.Sections))
		}
		if arc.Sections[0].Title != "阶段一" || arc.Sections[1].Title != "阶段二" {
			t.Errorf("persisted sections mismatch: %+v", arc.Sections)
		}
	})

	t.Run("volume index not found returns error", func(t *testing.T) {
		proj := &domain.Project{
			ID:    projID,
			Title: "单卷作品",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{
						VolumeIndex:  1,
						Title:        "第一卷",
						StartChapter: 1,
						EndChapter:   5,
					},
				},
			},
		}
		client := &mockSectionsLLMClient{}
		ws, _ := setupStoreAndWorkshop(client, proj)

		_, err := ws.EnsureVolumeSections(context.Background(), projID, 99, "test-model")
		if err == nil {
			t.Fatal("expected error for non-existent volume index, got nil")
		}
		if !strings.Contains(err.Error(), "volume index 99 not found") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestBuildVolumeContextBlock(t *testing.T) {
	arc := &domain.VolumeArc{
		VolumeIndex: 1,
		Title:       "初入仙门",
		CoreGoal:    "通过考核",
		Climax:      "击败内门首席",
		Sections: []domain.VolumeSection{
			{
				Title:       "外门考核",
				FromChapter: 1,
				ToChapter:   5,
				Goal:        "在考核中拿到前十",
				KeyEvents:   []string{"击败黑虎", "获取信物"},
			},
			{
				Title:       "内门试炼",
				FromChapter: 6,
				ToChapter:   10,
				Goal:        "晋升真传弟子",
			},
		},
	}

	t.Run("nil arc returns empty string", func(t *testing.T) {
		res := buildVolumeContextBlock(nil, 1)
		if res != "" {
			t.Fatalf("expected empty string for nil arc, got %q", res)
		}
	})

	t.Run("arc with no sections returns base text", func(t *testing.T) {
		arcNoSections := &domain.VolumeArc{
			VolumeIndex: 1,
			Title:       "初入仙门",
			CoreGoal:    "通过考核",
			Climax:      "击败内门首席",
		}
		res := buildVolumeContextBlock(arcNoSections, 3)
		expected := "\n【当前分卷主线任务】\n第 1 卷：《初入仙门》\n- 卷核心主线目标：通过考核\n- 卷终极大高潮：击败内门首席\n本章必须严格服务于本卷主线因果推进。\n"
		if res != expected {
			t.Fatalf("expected:\n%s\ngot:\n%s", expected, res)
		}
	})

	t.Run("target hits section 1 with key events", func(t *testing.T) {
		res := buildVolumeContextBlock(arc, 3)
		if !strings.Contains(res, "【本卷分节·本章所在节（第 1-5 章）：外门考核】") {
			t.Errorf("missing section header: %s", res)
		}
		if !strings.Contains(res, "- 节目标：在考核中拿到前十") {
			t.Errorf("missing section goal: %s", res)
		}
		if !strings.Contains(res, "- 关键事件：击败黑虎；获取信物") {
			t.Errorf("missing key events: %s", res)
		}
		if !strings.Contains(res, "本章必须严格服务于本卷主线因果推进与本节节目标。\n") {
			t.Errorf("missing updated closing sentence: %s", res)
		}
	})

	t.Run("target hits section 2 without key events", func(t *testing.T) {
		res := buildVolumeContextBlock(arc, 8)
		if !strings.Contains(res, "【本卷分节·本章所在节（第 6-10 章）：内门试炼】") {
			t.Errorf("missing section header: %s", res)
		}
		if !strings.Contains(res, "- 节目标：晋升真传弟子") {
			t.Errorf("missing section goal: %s", res)
		}
		if strings.Contains(res, "- 关键事件：") {
			t.Errorf("should not contain key events line when empty: %s", res)
		}
		if !strings.Contains(res, "本章必须严格服务于本卷主线因果推进与本节节目标。\n") {
			t.Errorf("missing updated closing sentence: %s", res)
		}
	})

	t.Run("target out of section range returns base text", func(t *testing.T) {
		res := buildVolumeContextBlock(arc, 99)
		expected := "\n【当前分卷主线任务】\n第 1 卷：《初入仙门》\n- 卷核心主线目标：通过考核\n- 卷终极大高潮：击败内门首席\n本章必须严格服务于本卷主线因果推进。\n"
		if res != expected {
			t.Fatalf("expected base text, got: %s", res)
		}
	})
}
