package engine

import (
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestMentionScanner_ScanText(t *testing.T) {
	scanner := NewMentionScanner()

	entries := []*domain.CodexEntry{
		{
			ID:           "e-chufeng",
			Name:         "楚枫",
			Aliases:      []string{"疯子楚", "白衣修罗"},
			Category:     domain.CategoryCharacter,
			TrackingMode: domain.TrackingModeAutoMention,
		},
		{
			ID:           "e-zhao",
			Name:         "赵天霸",
			Aliases:      []string{"霸刀"},
			Category:     domain.CategoryCharacter,
			TrackingMode: domain.TrackingModeAutoMention,
		},
		{
			ID:           "e-worldrule",
			Name:         "天道公理",
			Category:     domain.CategoryLore,
			TrackingMode: domain.TrackingModeAlwaysInject,
		},
		{
			ID:           "e-manual",
			Name:         "无名古剑",
			Category:     domain.CategoryItem,
			TrackingMode: domain.TrackingModeManual,
		},
	}

	text := "月夜之下，白衣修罗手按刀柄，冷冷注视着前方的霸刀。两人杀气冲天。"

	matched, mentions := scanner.ScanText(text, entries)

	// e-worldrule (AlwaysInject), e-chufeng (via alias "白衣修罗"), e-zhao (via alias "霸刀")
	// e-manual is Manual so should NOT be matched even if text had it.
	if len(matched) != 3 {
		t.Fatalf("expected 3 matched entries, got %d", len(matched))
	}

	foundChufeng := false
	foundZhao := false
	foundWorldRule := false

	for _, m := range mentions {
		if m.Entry.ID == "e-chufeng" {
			foundChufeng = true
			if m.MatchedAlias != "白衣修罗" {
				t.Errorf("expected alias 白衣修罗, got %s", m.MatchedAlias)
			}
		}
		if m.Entry.ID == "e-zhao" {
			foundZhao = true
			if m.MatchedAlias != "霸刀" {
				t.Errorf("expected alias 霸刀, got %s", m.MatchedAlias)
			}
		}
		if m.Entry.ID == "e-worldrule" {
			foundWorldRule = true
		}
	}

	if !foundChufeng || !foundZhao || !foundWorldRule {
		t.Errorf("missing expected entities in matches: %+v", mentions)
	}
}
