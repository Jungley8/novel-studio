package engine

import (
	"testing"
	"unicode/utf8"
)

func TestMockGenerator_ProseLengthAndLinter(t *testing.T) {
	linter := NewLinter(nil)

	for ch := 1; ch <= 12; ch++ {
		text := generateMockProseForChapter(ch)
		runeCount := utf8.RuneCountInString(text)
		if runeCount < 2500 {
			t.Errorf("chapter %d prose too short: got %d runes, want >= 2500", ch, runeCount)
		}

		res := linter.Analyze(text)
		if len(res.HitBannedWords) > 0 {
			t.Errorf("chapter %d hit banned words: %v", ch, res.HitBannedWords)
		}
		if res.BurstinessScore < 45 {
			t.Errorf("chapter %d burstiness too low: got %d, want >= 45", ch, res.BurstinessScore)
		}
	}
}
