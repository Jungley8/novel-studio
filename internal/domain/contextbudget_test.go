package domain_test

import (
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single ASCII", "a", 0},
		{"two ASCII", "ab", 1},
		{"chinese four runes", "你好世界", 2},
		{"chinese mixed runes", "你好世界！这是一段测试文本，用于粗估Token。", 12},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.EstimateTokens(tc.input)
			if got != tc.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tc.input, got, tc.expected)
			}
		})
	}
}

func TestContextTrace_Fields(t *testing.T) {
	trace := domain.ContextTrace{
		ProtectedIDs:    []string{"global/rules", "rolling/ch1"},
		CompressibleIDs: []string{"callback/hook1"},
		DroppedIDs:      []string{"callback/hook2"},
		BudgetTokens:    12000,
		EstimatedTokens: 4500,
		Truncated:       true,
	}

	if !trace.Truncated {
		t.Errorf("expected Truncated=true")
	}
	if len(trace.ProtectedIDs) != 2 || trace.ProtectedIDs[0] != "global/rules" {
		t.Errorf("unexpected ProtectedIDs: %v", trace.ProtectedIDs)
	}
	if len(trace.CompressibleIDs) != 1 || trace.CompressibleIDs[0] != "callback/hook1" {
		t.Errorf("unexpected CompressibleIDs: %v", trace.CompressibleIDs)
	}
	if len(trace.DroppedIDs) != 1 || trace.DroppedIDs[0] != "callback/hook2" {
		t.Errorf("unexpected DroppedIDs: %v", trace.DroppedIDs)
	}
	if trace.BudgetTokens != 12000 || trace.EstimatedTokens != 4500 {
		t.Errorf("unexpected token counts: budget=%d estimated=%d", trace.BudgetTokens, trace.EstimatedTokens)
	}
}
