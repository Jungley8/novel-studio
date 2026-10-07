package engine_test

import (
	"testing"

	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestLinter_Analyze(t *testing.T) {
	l := engine.NewLinter(nil)

	// Case 1: Bad text with clichés and flat sentence lengths
	badText := `他不由得冷笑了一声。眼神复杂地看着对方。与此同时心中暗自思忖。仿佛一切都已成定局。`
	resBad := l.Analyze(badText)
	if len(resBad.HitBannedWords) == 0 {
		t.Errorf("expected hit banned words for badText")
	}
	if resBad.Passed {
		t.Errorf("expected badText to fail linting")
	}

	// Case 2: Good text with dynamic sentence lengths (high burstiness) and no clichés
	goodText := `刀落。
血顺着青石板的沟壑一路蔓延开去，最终浸湿了那柄残破的玄铁短剑。
他没回头。
远处的雨声正以一种不可遏制的势头撞击着整座破败的城门，瓦砾碎裂的声响如同催命的鼓点。`
	resGood := l.Analyze(goodText)
	if len(resGood.HitBannedWords) > 0 {
		t.Errorf("unexpected hit banned words: %v", resGood.HitBannedWords)
	}
	if resGood.BurstinessScore < 45 {
		t.Errorf("expected high burstiness score, got %d", resGood.BurstinessScore)
	}
	if !resGood.Passed {
		t.Errorf("expected goodText to pass linting")
	}
}
