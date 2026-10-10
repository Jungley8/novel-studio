package engine_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type mockForecastLLMClient struct {
	capturedRole   engine.LLMRole
	responseToEmit string
	errToEmit      error
}

func (m *mockForecastLLMClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if r, ok := engine.RoleFromContext(ctx); ok {
		m.capturedRole = r
	}
	if m.errToEmit != nil {
		return "", m.errToEmit
	}
	return m.responseToEmit, nil
}

func (m *mockForecastLLMClient) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, engine.TokenUsage, error) {
	resp, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return resp, engine.TokenUsage{}, err
}

func (m *mockForecastLLMClient) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (<-chan engine.StreamChunk, error) {
	return nil, fmt.Errorf("streaming not implemented in mock")
}

func TestCreateForecast_ExactBranchCount(t *testing.T) {
	horizonCtx := &engine.CanonHorizon{
		Project: &domain.Project{
			ID:    "proj_test_fc",
			Title: "长生仙途",
		},
		TargetChapter: 4,
		GlobalSummary: "主角在前三章斩杀了宗门叛徒，取得筑基丹。",
		ProtagonistEntry: &domain.CodexEntry{
			Name:      "林尘",
			Realm:     "练气九层",
			Inventory: "筑基丹x1, 残破青锋剑",
		},
		AllActiveHooks: []*domain.PlotHook{
			{ID: "hook_1", Title: "青云宗暗流", Status: domain.HookStatusOpen, TargetChapter: 8},
		},
	}

	t.Run("succeeds when LLM returns exact N branches wrapped in markdown json block", func(t *testing.T) {
		mockResp := "```json\n" + `{
  "branches": [
    {
      "id": "b1",
      "beats_summary": "服丹突破，引动雷劫，反杀伏击者",
      "decisions": "林尘决意闭死关强行冲阶",
      "expected_deltas": "+筑基初期修为, -筑基丹x1",
      "risks": "根基不稳，可能招来执法长老注意",
      "intent_match": "契合激进突破意图"
    },
    {
      "id": "b2",
      "beats_summary": "伪装身份逃入黑市，暗中打探宗门内奸",
      "decisions": "林尘隐忍不发，易容潜伏",
      "expected_deltas": "+易容面具, 结识散修黑市盟友",
      "risks": "可能错失最佳筑基天时",
      "intent_match": "契合谨慎苟道意图"
    },
    {
      "id": "b3",
      "beats_summary": "主动上交部分线索求得掌门庇护",
      "decisions": "将叛徒信件交由掌门明察",
      "expected_deltas": "+宗门功勋, 获得正道护法",
      "risks": "可能直接激怒幕后黑手提前下杀手",
      "intent_match": "契合借势借力意图"
    }
  ]
}` + "\n```"

		client := &mockForecastLLMClient{responseToEmit: mockResp}
		fc, err := engine.CreateForecast(
			context.Background(),
			client,
			"deepseek-reasoner",
			horizonCtx,
			"林尘面对宗门追查与突破抉择",
			3, // N = 3
			3, // H = 3
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fc == nil {
			t.Fatal("expected non-nil forecast")
		}
		if len(fc.Branches) != 3 {
			t.Fatalf("expected exactly 3 branches, got %d", len(fc.Branches))
		}
		if fc.BaseChapter != "3" {
			t.Errorf("expected BaseChapter '3', got '%s'", fc.BaseChapter)
		}
		if fc.Horizon != 3 {
			t.Errorf("expected Horizon 3, got %d", fc.Horizon)
		}
		if fc.Status != domain.ForecastStatusActive {
			t.Errorf("expected Status 'active', got '%s'", fc.Status)
		}
		if client.capturedRole != engine.RoleReasoner {
			t.Errorf("expected LLM role %s, got %s", engine.RoleReasoner, client.capturedRole)
		}
	})

	t.Run("fails when LLM returns fewer branches than requested without truncation", func(t *testing.T) {
		mockResp := `{
  "branches": [
    {
      "id": "b1",
      "beats_summary": "分支1",
      "decisions": "抉择1",
      "expected_deltas": "变化1",
      "risks": "风险1",
      "intent_match": "匹配1"
    },
    {
      "id": "b2",
      "beats_summary": "分支2",
      "decisions": "抉择2",
      "expected_deltas": "变化2",
      "risks": "风险2",
      "intent_match": "匹配2"
    }
  ]
}`
		client := &mockForecastLLMClient{responseToEmit: mockResp}
		_, err := engine.CreateForecast(
			context.Background(),
			client,
			"deepseek-reasoner",
			horizonCtx,
			"分歧点",
			3, // Requested 3, returned 2
			3,
		)
		if err == nil {
			t.Fatal("expected error when LLM returns fewer branches than requested, got nil")
		}
	})

	t.Run("fails when LLM returns more branches than requested without truncation", func(t *testing.T) {
		mockResp := `{
  "branches": [
    {"id": "b1", "beats_summary": "s1", "decisions": "d1", "expected_deltas": "e1", "risks": "r1", "intent_match": "m1"},
    {"id": "b2", "beats_summary": "s2", "decisions": "d2", "expected_deltas": "e2", "risks": "r2", "intent_match": "m2"},
    {"id": "b3", "beats_summary": "s3", "decisions": "d3", "expected_deltas": "e3", "risks": "r3", "intent_match": "m3"},
    {"id": "b4", "beats_summary": "s4", "decisions": "d4", "expected_deltas": "e4", "risks": "r4", "intent_match": "m4"}
  ]
}`
		client := &mockForecastLLMClient{responseToEmit: mockResp}
		_, err := engine.CreateForecast(
			context.Background(),
			client,
			"deepseek-reasoner",
			horizonCtx,
			"分歧点",
			3, // Requested 3, returned 4
			3,
		)
		if err == nil {
			t.Fatal("expected error when LLM returns more branches than requested, got nil")
		}
	})

	t.Run("validates input boundary invariants", func(t *testing.T) {
		client := &mockForecastLLMClient{responseToEmit: `{"branches":[]}`}

		// Empty divergence
		if _, err := engine.CreateForecast(context.Background(), client, "m", horizonCtx, "", 3, 3); err == nil {
			t.Error("expected error for empty divergence")
		}

		// BranchCount out of range [2, 5]
		if _, err := engine.CreateForecast(context.Background(), client, "m", horizonCtx, "div", 1, 3); err == nil {
			t.Error("expected error for branchCount 1 (< 2)")
		}
		if _, err := engine.CreateForecast(context.Background(), client, "m", horizonCtx, "div", 6, 3); err == nil {
			t.Error("expected error for branchCount 6 (> 5)")
		}

		// Horizon out of range [3, 5]
		if _, err := engine.CreateForecast(context.Background(), client, "m", horizonCtx, "div", 3, 2); err == nil {
			t.Error("expected error for horizon 2 (< 3)")
		}
		if _, err := engine.CreateForecast(context.Background(), client, "m", horizonCtx, "div", 3, 6); err == nil {
			t.Error("expected error for horizon 6 (> 5)")
		}
	})
}
