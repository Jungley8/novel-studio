package engine_test

import (
	"strings"
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
远处的雨声正以一种不可遏制的势头撞击着整座破败的城门，瓦砾碎裂的声响敲打着催命的鼓点。
「别追了。」林凡的声音很轻。`
	resGood := l.Analyze(goodText)
	if len(resGood.HitBannedWords) > 0 {
		t.Errorf("unexpected hit banned words: %v", resGood.HitBannedWords)
	}
	if resGood.BurstinessScore < 40 {
		t.Errorf("expected high burstiness score, got %d", resGood.BurstinessScore)
	}
	if !resGood.Passed {
		t.Errorf("expected goodText to pass linting: %s", resGood.Message)
	}
	if resGood.DialogueRatio <= 0 {
		t.Errorf("expected non-zero dialogue ratio for quotes, got %f", resGood.DialogueRatio)
	}
	if resGood.ParagraphVariance <= 0 {
		t.Errorf("expected non-zero paragraph variance, got %d", resGood.ParagraphVariance)
	}
}

func TestLinter_TwoTierWarningKeywords(t *testing.T) {
	l := engine.NewLinter(nil)

	// Single occurrence of warning keyword should NOT be penalized
	singleWarning := `雪落无声。林凡缓缓拔出佩剑，竟然只用了一招便斩断了石碑。
长街寂静，唯有残风掠过残垣断壁。
他转身离去。`
	resSingle := l.Analyze(singleWarning)
	for _, hit := range resSingle.HitBannedWords {
		if strings.Contains(hit, "竟然") {
			t.Errorf("single warning keyword should not be flagged: %s", hit)
		}
	}

	// Multiple occurrences of warning keyword (>= 2) should be flagged
	multiWarning := `他竟然能跨阶而战。
对手没想到他竟然还能站起来。`
	resMulti := l.Analyze(multiWarning)
	foundWarning := false
	for _, hit := range resMulti.HitBannedWords {
		if strings.Contains(hit, "竟然(×2)") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected warning keyword repeated 2 times to be flagged, got %v", resMulti.HitBannedWords)
	}
}

func TestLinter_ExclamationDensityAndNgrams(t *testing.T) {
	l := engine.NewLinter(nil)

	// High exclamation mark density
	shoutText := `杀！死！冲！快跑！小心！受死吧！`
	resExcl := l.Analyze(shoutText)
	if resExcl.ExclamationDensity < 100 {
		t.Errorf("expected high exclamation density, got %f", resExcl.ExclamationDensity)
	}
	if resExcl.Passed {
		t.Errorf("excessive exclamation should fail linting")
	}

	// Repeated n-grams text
	repeatedText := `天地玄黄，宇宙洪荒。天地玄黄之气滚滚而来。天地玄黄在翻腾。`
	resRep := l.Analyze(repeatedText)
	if len(resRep.TopRepeatedNgrams) == 0 {
		t.Errorf("expected repeated n-grams to be detected")
	}
	found := false
	for _, ng := range resRep.TopRepeatedNgrams {
		if strings.Contains(ng, "天地玄黄") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '天地玄黄' in repeated ngrams, got %v", resRep.TopRepeatedNgrams)
	}
}
