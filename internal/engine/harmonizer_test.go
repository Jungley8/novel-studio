package engine

import (
	"strings"
	"testing"
)

func TestHarmonizer_CensorHarmonization(t *testing.T) {
	h := NewHarmonizer()

	rawText := "楚掌柜被开膛破肚，血肉模糊，尸体跌落在地。顾渊掐断喉管，将他碎成肉泥。"
	harmonized, items := h.HarmonizeSensitiveWords(rawText)

	if len(items) == 0 {
		t.Fatalf("expected harmonized items, got 0")
	}

	// Verify gore words were replaced
	for _, forbidden := range []string{"开膛破肚", "血肉模糊", "掐断喉管", "碎成肉泥"} {
		if strings.Contains(harmonized, forbidden) {
			t.Errorf("expected '%s' to be harmonized out, but still present in: %s", forbidden, harmonized)
		}
	}

	// Verify resulting text is non-empty and readable
	if len(harmonized) < 20 {
		t.Errorf("harmonized text too short: %s", harmonized)
	}
}

func TestHarmonizer_AdversarialPerturbation(t *testing.T) {
	h := NewHarmonizer()

	rawText := "顾渊走过去看着楚掌柜。顾渊十分愤怒，拿出了凡骨铁印。楚掌柜站起来，眼神复杂。"
	perturbed := h.Perturb(rawText, 0.8)

	if perturbed == rawText {
		t.Errorf("expected perturbed text to differ from raw text")
	}

	// Verify length remains reasonable
	if len([]rune(perturbed)) < len([]rune(rawText))-10 {
		t.Errorf("perturbed text lost too much content: %s", perturbed)
	}
}

func TestHarmonizer_SuggestHumanTouches(t *testing.T) {
	h := NewHarmonizer()

	text := "风雪很大，顾渊走入聚仙楼。楚掌柜坐在案后，给顾渊倒了一碗毒酒。顾渊识破毒酒，反手打断楚掌柜的手腕。"
	suggestions := h.SuggestHumanTouches(text)

	if len(suggestions) == 0 {
		t.Fatalf("expected human touch suggestions, got 0")
	}

	hasPhysiology := false
	hasTriviality := false
	for _, s := range suggestions {
		if s.Category == "PHYSIOLOGY" {
			hasPhysiology = true
		}
		if s.Category == "TRIVIALITY" {
			hasTriviality = true
		}
	}

	if !hasPhysiology && !hasTriviality {
		t.Errorf("expected physiology or triviality suggestion, got: %+v", suggestions)
	}
}

func TestHarmonizer_FullProcess(t *testing.T) {
	h := NewHarmonizer()

	rawText := "楚掌柜被开膛破肚，尸体横陈在官道上。顾渊十分冰冷，走过去拿出了短刀。"
	processed, report := h.FullProcess(rawText, 0.7)

	if len(report.HarmonizedItems) == 0 {
		t.Errorf("expected harmonized items in report")
	}
	if processed == rawText {
		t.Errorf("expected processed text to differ from raw text")
	}
	if len(report.SuggestedTouches) == 0 {
		t.Errorf("expected suggested touches in report")
	}
}
