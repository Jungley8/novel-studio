package engine

import (
	"math"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// DefaultBannedKeywords contains frequent AI slop / cliches in Chinese web fiction.
var DefaultBannedKeywords = []string{
	"不由得", "仿佛", "宛若", "嘴角勾起", "眼神复杂",
	"一时间", "殊不知", "与此同时", "冷哼一声", "眼中闪过一丝",
	"倒吸一口凉气", "如释重负", "心头一震", "暗自思忖",
}

// Linter analyzes text against AI detection metrics and clichés.
type Linter struct {
	bannedKeywords []string
}

func NewLinter(customBanned []string) *Linter {
	words := DefaultBannedKeywords
	if len(customBanned) > 0 {
		words = customBanned
	}
	return &Linter{bannedKeywords: words}
}

// Analyze evaluates the burstiness standard deviation and scans for clichés.
func (l *Linter) Analyze(text string) domain.LinterResult {
	if strings.TrimSpace(text) == "" {
		return domain.LinterResult{
			BurstinessScore: 0,
			HitBannedWords:  nil,
			Passed:          false,
			Message:         "文本为空",
		}
	}

	// 1. Scan for banned words
	var hits []string
	for _, word := range l.bannedKeywords {
		if strings.Contains(text, word) {
			hits = append(hits, word)
		}
	}

	// 2. Compute Burstiness (sentence length standard deviation)
	sentences := splitSentences(text)
	burstiness := calculateBurstiness(sentences)

	passed := len(hits) == 0 && burstiness >= 45

	msg := "质检通过，句式方差优良，无AI模式化套词。"
	if !passed {
		var parts []string
		if len(hits) > 0 {
			parts = append(parts, "命中AI高频套词")
		}
		if burstiness < 45 {
			parts = append(parts, "句长节奏过于平缓(易被平台反AI检测识别)")
		}
		msg = "质检预警: " + strings.Join(parts, "；")
	}

	return domain.LinterResult{
		BurstinessScore: burstiness,
		HitBannedWords:  hits,
		Passed:          passed,
		Message:         msg,
	}
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for _, r := range text {
		switch r {
		case '。', '！', '？', '\n':
			s := strings.TrimSpace(current.String())
			if len([]rune(s)) > 0 {
				sentences = append(sentences, s)
			}
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	s := strings.TrimSpace(current.String())
	if len([]rune(s)) > 0 {
		sentences = append(sentences, s)
	}
	return sentences
}

func calculateBurstiness(sentences []string) int {
	n := len(sentences)
	if n <= 1 {
		return 30
	}

	var sum float64
	lengths := make([]float64, n)
	for i, s := range sentences {
		l := float64(len([]rune(s)))
		lengths[i] = l
		sum += l
	}

	mean := sum / float64(n)
	var varianceSum float64
	for _, l := range lengths {
		diff := l - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(n)
	stdDev := math.Sqrt(variance)

	// Normalized score: standard deviation * 3.5 clamped to 100
	score := int(math.Round(stdDev * 3.5))
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}
