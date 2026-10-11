package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestBuildRenderIntentDirective(t *testing.T) {
	tests := []struct {
		name     string
		intent   *domain.ChapterIntent
		expected string
	}{
		{
			name:     "nil_intent",
			intent:   nil,
			expected: "",
		},
		{
			name: "non_locked_draft",
			intent: &domain.ChapterIntent{
				POV:          "第一人称",
				EmotionCurve: []string{"压抑", "爆发"},
				Status:       domain.IntentStatusDraft,
			},
			expected: "",
		},
		{
			name: "non_locked_stale",
			intent: &domain.ChapterIntent{
				POV:          "第一人称",
				EmotionCurve: []string{"压抑", "爆发"},
				Status:       domain.IntentStatusStale,
			},
			expected: "",
		},
		{
			name: "locked_only_pov",
			intent: &domain.ChapterIntent{
				POV:    "第一人称限制视角",
				Status: domain.IntentStatusLocked,
			},
			expected: "【本章意图卡指令（作者已锁定）】\n视角：第一人称限制视角\n\n",
		},
		{
			name: "locked_only_curve",
			intent: &domain.ChapterIntent{
				EmotionCurve: []string{"压抑", "反转", "高潮"},
				Status:       domain.IntentStatusLocked,
			},
			expected: "【本章意图卡指令（作者已锁定）】\n情绪曲线：压抑 → 反转 → 高潮\n\n",
		},
		{
			name: "locked_both_pov_and_curve_in_order",
			intent: &domain.ChapterIntent{
				POV:          "林凡主视角",
				EmotionCurve: []string{"困惑", "震惊", "决然"},
				Status:       domain.IntentStatusLocked,
			},
			expected: "【本章意图卡指令（作者已锁定）】\n视角：林凡主视角\n情绪曲线：困惑 → 震惊 → 决然\n\n",
		},
		{
			name: "locked_curve_with_empty_items_filtered",
			intent: &domain.ChapterIntent{
				POV:          "陆青",
				EmotionCurve: []string{"  ", "隐忍", "", "\t", "爆发", "  "},
				Status:       domain.IntentStatusLocked,
			},
			expected: "【本章意图卡指令（作者已锁定）】\n视角：陆青\n情绪曲线：隐忍 → 爆发\n\n",
		},
		{
			name: "locked_pov_and_curve_all_empty",
			intent: &domain.ChapterIntent{
				POV:          "   ",
				EmotionCurve: []string{"", "   ", "\t"},
				Status:       domain.IntentStatusLocked,
			},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRenderIntentDirective(tc.intent)
			if got != tc.expected {
				t.Fatalf("buildRenderIntentDirective() = %q, want %q", got, tc.expected)
			}
			if tc.expected != "" {
				const header = "【本章意图卡指令（作者已锁定）】"
				if !strings.HasPrefix(got, header) {
					t.Errorf("expected directive to start with %q, got: %q", header, got)
				}
				if !strings.HasSuffix(got, "\n\n") {
					t.Errorf("expected directive to end with \\n\\n, got: %q", got)
				}
			}
		})
	}
}

func TestChapterWorkshop_RenderIntentDirectiveAccounted(t *testing.T) {
	intent := &domain.ChapterIntent{
		ProjectID:    "p-renderacct",
		ChapterIndex: 1,
		VolumeIndex:  1,
		Conflict:     "山门大比决战",
		POV:          "林凡",
		EmotionCurve: []string{"蓄势", "爆发"},
		Status:       domain.IntentStatusLocked,
	}

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   `{"beats": [{"phase": "破局", "tension": 7, "action": "亮剑"}]}`,
		renderDraft: "正文内容",
		reviewJSON:  `{"verdict": "ACCEPTED", "score": 90, "issues": [], "suggestions": "通过"}`,
	}
	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	mockStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-renderacct",
				Title:          "万域封神",
				TargetPlatform: "番茄脑洞",
				WorldRules:     "天地不仁以万物为刍狗",
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:        "codex_p-renderacct_pro",
					ProjectID: "p-renderacct",
					Category:  domain.CategoryCharacter,
					Archetype: "PROTAGONIST",
					Name:      "林凡",
					Realm:     "筑基初期",
					Inventory: "青光剑",
				},
			},
			chapters: []*domain.Chapter{},
			hooks:    []*domain.PlotHook{},
		},
	}
	_ = mockStore.SaveChapterIntent(context.Background(), intent)

	chronicle := NewCanonChronicle(mockStore)
	chronicle.SetConfig(&config.Config{
		ContextBudgetTokens: 1,
	})

	ws := NewChapterWorkshop(orch, chronicle, qGate, mockStore)

	var events []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-renderacct",
		ChapterIndex:    1,
		CoreConflict:    "山门大比决战",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 1,
		OnProgress: func(ev WorkshopEvent) {
			events = append(events, ev)
		},
	}

	_, err := ws.ProduceChapter(context.Background(), req)
	if err == nil {
		t.Fatalf("expected ProduceChapter to fail with budget overflow, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "context budget exceeded by protected items") {
		t.Fatalf("expected protected items overflow error, got: %s", errMsg)
	}

	// 断言错误 message 的 breakdown 含 `inject/intent-render: ` 及 token 数
	if !strings.Contains(errMsg, "inject/intent-render: ") {
		t.Fatalf("expected error message breakdown to contain 'inject/intent-render: ', got: %s", errMsg)
	}

	re := regexp.MustCompile(`inject/intent-render:\s*(\d+)\s*tokens`)
	match := re.FindStringSubmatch(errMsg)
	if len(match) < 2 {
		t.Fatalf("expected breakdown to match 'inject/intent-render: <tokens> tokens', got error: %s", errMsg)
	}
	tokens, convErr := strconv.Atoi(match[1])
	if convErr != nil || tokens <= 0 {
		t.Fatalf("expected positive token count for inject/intent-render, got %s (err: %v)", match[1], convErr)
	}
}

func TestRenderSceneStream_IntentDirectiveInjected(t *testing.T) {
	var capturedUserPrompts []string
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			var req struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(bodyBytes, &req); err == nil {
				mu.Lock()
				for _, m := range req.Messages {
					if m.Role == "user" {
						capturedUserPrompts = append(capturedUserPrompts, m.Content)
					}
				}
				mu.Unlock()
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"漫天飞雪中，剑鸣破空。"}}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	}))
	defer ts.Close()

	client := NewHTTPLLMClient(ts.URL, "test-key")
	orch := NewOrchestrator(client)

	project := &domain.Project{
		ID:             "p-render-test",
		Title:          "剑道独尊",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "剑道唯心",
	}
	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 6, Action: "风雪封山"},
		{Phase: "章末留钩", Tension: 9, Action: "剑气破空", HookType: "CLIFFHANGER"},
	}

	t.Run("locked_intent_directive_injected", func(t *testing.T) {
		mu.Lock()
		capturedUserPrompts = nil
		mu.Unlock()

		horizonLocked := &CanonHorizon{
			Project:       project,
			TargetChapter: 2,
			ConsumedIntent: &domain.ChapterIntent{
				ProjectID:    "p-render-test",
				ChapterIndex: 2,
				POV:          "第一人称限制视角",
				EmotionCurve: []string{"压抑", "爆发"},
				Status:       domain.IntentStatusLocked,
			},
		}

		_, _, err := orch.RenderSceneStream(context.Background(), "writer-model", horizonLocked, beats, 2000, nil)
		if err != nil {
			t.Fatalf("RenderSceneStream failed: %v", err)
		}

		mu.Lock()
		prompts := append([]string(nil), capturedUserPrompts...)
		mu.Unlock()

		if len(prompts) == 0 {
			t.Fatalf("expected captured user prompts, got none")
		}

		lastPrompt := prompts[len(prompts)-1]

		const header = "【本章意图卡指令（作者已锁定）】"
		if !strings.Contains(lastPrompt, header) {
			t.Errorf("expected userPrompt to contain %q, got: %s", header, lastPrompt)
		}
		if !strings.Contains(lastPrompt, "视角：第一人称限制视角") {
			t.Errorf("expected userPrompt to contain '视角：第一人称限制视角', got: %s", lastPrompt)
		}
		if !strings.Contains(lastPrompt, "情绪曲线：压抑 → 爆发") {
			t.Errorf("expected userPrompt to contain '情绪曲线：压抑 → 爆发', got: %s", lastPrompt)
		}

		directiveIdx := strings.Index(lastPrompt, header)
		beatsIdx := strings.Index(lastPrompt, "【必须执行的场景节拍")
		if directiveIdx == -1 || beatsIdx == -1 {
			t.Fatalf("expected both header and beats block in userPrompt")
		}
		if directiveIdx >= beatsIdx {
			t.Errorf("expected directive block (idx=%d) to precede beats block (idx=%d)", directiveIdx, beatsIdx)
		}
	})

	t.Run("no_intent_regression", func(t *testing.T) {
		mu.Lock()
		capturedUserPrompts = nil
		mu.Unlock()

		horizonNoIntent := &CanonHorizon{
			Project:        project,
			TargetChapter:  2,
			ConsumedIntent: nil,
		}

		_, _, err := orch.RenderSceneStream(context.Background(), "writer-model", horizonNoIntent, beats, 2000, nil)
		if err != nil {
			t.Fatalf("RenderSceneStream failed: %v", err)
		}

		mu.Lock()
		prompts := append([]string(nil), capturedUserPrompts...)
		mu.Unlock()

		if len(prompts) == 0 {
			t.Fatalf("expected captured user prompts, got none")
		}

		lastPrompt := prompts[len(prompts)-1]

		const header = "【本章意图卡指令"
		if strings.Contains(lastPrompt, header) {
			t.Errorf("expected userPrompt NOT to contain %q when intent is nil, got: %s", header, lastPrompt)
		}
	})
}
