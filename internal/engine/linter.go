package engine

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// CoreBannedKeywords are hard-ban AI slop words. Any single hit forces a penalty.
var CoreBannedKeywords = []string{
	// === 情绪悬浮 ===
	"不由得", "不禁", "下意识地", "情不自禁", "莫名其妙地", "百感交集",
	// === AI 万能动作 ===
	"嘴角勾起", "嘴角微扬", "嘴角上扬", "眼神复杂", "目光深邃",
	"冷哼一声", "冷笑一声", "嗤笑一声",
	"倒吸一口凉气", "倒抽一口冷气",
	"暗自思忖", "心中暗道", "心头一震", "心中一凛", "暗暗心惊", "暗自窃喜",
	// === 时间过渡 ===
	"一时间", "与此同时", "殊不知", "话音刚落", "下一刻", "刹那间", "转瞬之间",
	// === 比喻套路 ===
	"仿佛", "宛若", "犹如", "如同", "好似",
	"如释重负", "如坠冰窟", "犹如神助",
	// === 动作重复 ===
	"眉头紧锁", "眉头微皱", "双拳紧握",
	"眼中闪过一丝", "目光一凝", "瞳孔微缩",
	"点了点头", "摇了摇头",
	// === AI 微表情与副词套路 (腾讯朱雀强特征) ===
	"僵硬的弧度", "眼角没有半分笑意", "眼角毫无笑意", "瞳孔缩成了针尖", "瞳孔缩成针尖",
	"古井无波", "慢条斯理地", "几不可察地", "夜枭般的狞笑", "如泥牛入海", "如丧家之犬",
	"死寂一片", "冷汗涔涔",
	// === 环境万能句 ===
	"空气仿佛凝固", "空气中弥漫着", "气氛变得凝重",
	"鸦雀无声", "针落可闻", "天地为之一暗", "风起云涌",
	// === 打斗模板 ===
	"身形暴退", "身影如电", "凌厉的攻势",
	"强横的气息", "恐怖的威压", "破空之声", "势如破竹",
	// === 心理直白 ===
	"此刻的他", "不得不承认", "不得不说",
	"一股无形的力量", "一股莫名的感觉", "心中升起一股",
}

// DefaultBannedKeywords is kept as an alias for backward compatibility.
var DefaultBannedKeywords = CoreBannedKeywords

// WarningKeywords are allowed once but flagged when appearing ≥2 times in a chapter.
var WarningKeywords = []string{
	"竟然", "居然", "没想到",
	"深吸一口气", "长舒一口气",
	"微微一笑", "淡淡一笑",
	"缓缓说道", "沉声说道", "冷声说道",
	"总而言之", "综上所述", "仿佛在诉说", "预示着", "伴随着",
}

// Linter analyzes text against AI detection metrics and clichés using two-tier detection.
type Linter struct {
	coreBanned    []string
	warningBanned []string
}

func NewLinter(customBanned []string) *Linter {
	core := CoreBannedKeywords
	if len(customBanned) > 0 {
		core = customBanned
	}
	return &Linter{
		coreBanned:    core,
		warningBanned: WarningKeywords,
	}
}

// Analyze evaluates the burstiness standard deviation, two-tier cliché scanning,
// and 4 structural narrative metrics: dialogue ratio, paragraph variance, n-grams, exclamation density.
func (l *Linter) Analyze(text string) domain.LinterResult {
	if strings.TrimSpace(text) == "" {
		return domain.LinterResult{
			BurstinessScore:    0,
			HitBannedWords:     []string{},
			DialogueRatio:      0,
			ParagraphVariance:  0,
			TopRepeatedNgrams:  []string{},
			ExclamationDensity: 0,
			EmpiricalTells:     []string{},
			Passed:             false,
			Message:            "文本为空",
		}
	}

	// 1. Scan core banned words (any single hit = penalty)
	hits := make([]string, 0)
	for _, word := range l.coreBanned {
		if strings.Contains(text, word) {
			hits = append(hits, word)
		}
	}

	// 2. Scan warning words (frequency >= 2 = penalty)
	for _, word := range l.warningBanned {
		count := strings.Count(text, word)
		if count >= 2 {
			hits = append(hits, fmt.Sprintf("%s(×%d)", word, count))
		}
	}

	// 2.5 Scan formulaic similes (像.../如...)
	simileCount := CountSimileClichés(text)
	if simileCount >= 3 {
		hits = append(hits, fmt.Sprintf("书面比喻泛滥(命中%d处公式化比喻)", simileCount))
	}

	// 2.8 Scan empirical AI writing tells (翻案腔, 密集顿号, 提示语冒号, 翻译腔壳子等)
	empiricalTells := ScanEmpiricalAITells(text)
	for _, tell := range empiricalTells {
		hits = append(hits, tell)
	}

	// 3. Compute Burstiness (sentence length standard deviation)
	sentences := splitSentences(text)
	burstiness := calculateBurstiness(sentences)

	// 4. Compute Structural Narrative Metrics
	dialogueRatio := calculateDialogueRatio(text)
	paraVariance := calculateParagraphVariance(text)
	topNgrams := calculateRepeatedNgrams(text)
	if topNgrams == nil {
		topNgrams = []string{}
	}
	exclDensity := calculateExclamationDensity(text)

	runesLen := len([]rune(text))
	exclExcessive := IsExclamationExcessive(text, exclDensity)
	isTele, teleMsg := DetectTelegraphicFragmentation(text)
	if isTele {
		hits = append(hits, teleMsg)
	}
	passed := len(hits) == 0 && burstiness >= 45 && !exclExcessive && !isTele
	if runesLen >= 300 {
		if dialogueRatio > 0.75 || (dialogueRatio < 0.05 && runesLen > 800) {
			passed = false
		}
	}

	var parts []string
	if len(hits) > 0 {
		parts = append(parts, "命中AI高频套词或句式硬伤")
	}
	if burstiness < 45 {
		parts = append(parts, "句长节奏过于平缓(易被平台反AI检测识别)")
	}
	if isTele {
		parts = append(parts, teleMsg)
	}
	if exclExcessive {
		parts = append(parts, fmt.Sprintf("感叹号过密(每千字%.1f个)", exclDensity))
	}
	if runesLen >= 300 && dialogueRatio > 0.75 {
		parts = append(parts, fmt.Sprintf("对话占比过高(%.1f%%)", dialogueRatio*100))
	}
	if runesLen > 800 && dialogueRatio < 0.05 {
		parts = append(parts, "通篇缺乏对话角色交互")
	}

	msg := "质检通过，句式方差与叙事维度优良，无AI模式化套词。"
	if !passed {
		msg = "质检预警: " + strings.Join(parts, "；")
	}

	return domain.LinterResult{
		BurstinessScore:    burstiness,
		HitBannedWords:     hits,
		DialogueRatio:      dialogueRatio,
		ParagraphVariance:  paraVariance,
		TopRepeatedNgrams:  topNgrams,
		ExclamationDensity: exclDensity,
		EmpiricalTells:     empiricalTells,
		Passed:             passed,
		Message:            msg,
	}
}

func calculateDialogueRatio(text string) float64 {
	runes := []rune(text)
	total := len(runes)
	if total == 0 {
		return 0
	}
	var inDialogue bool
	var endQuote rune
	dialogueCount := 0

	for _, r := range runes {
		if !inDialogue {
			switch r {
			case '「':
				inDialogue = true
				endQuote = '」'
				dialogueCount++
			case '“':
				inDialogue = true
				endQuote = '”'
				dialogueCount++
			case '"':
				inDialogue = true
				endQuote = '"'
				dialogueCount++
			}
		} else {
			dialogueCount++
			if r == endQuote {
				inDialogue = false
			}
		}
	}
	ratio := float64(dialogueCount) / float64(total)
	return math.Round(ratio*1000) / 1000
}

func calculateParagraphVariance(text string) int {
	lines := strings.Split(text, "\n")
	var lengths []float64
	var sum float64
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			l := float64(len([]rune(trimmed)))
			lengths = append(lengths, l)
			sum += l
		}
	}
	if len(lengths) <= 1 {
		return 0
	}
	mean := sum / float64(len(lengths))
	var varianceSum float64
	for _, l := range lengths {
		diff := l - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(lengths))
	stdDev := math.Sqrt(variance)
	return int(math.Round(stdDev))
}

func calculateExclamationDensity(text string) float64 {
	runes := []rune(text)
	total := len(runes)
	if total == 0 {
		return 0
	}
	count := 0
	for _, r := range runes {
		if r == '！' || r == '!' {
			count++
		}
	}
	density := (float64(count) * 1000.0) / float64(total)
	return math.Round(density*100) / 100
}

func calculateRepeatedNgrams(text string) []string {
	var hanRunes []rune
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			hanRunes = append(hanRunes, r)
		}
	}
	if len(hanRunes) < 10 {
		return nil
	}

	counts := make(map[string]int)
	for n := 3; n <= 4; n++ {
		for i := 0; i <= len(hanRunes)-n; i++ {
			gram := string(hanRunes[i : i+n])
			counts[gram]++
		}
	}

	type gramFreq struct {
		gram  string
		count int
	}
	var candidates []gramFreq
	threshold := 4
	if len(hanRunes) < 1000 {
		threshold = 3
	}
	for g, c := range counts {
		if c >= threshold {
			candidates = append(candidates, gramFreq{gram: g, count: c})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count == candidates[j].count {
			return len(candidates[i].gram) > len(candidates[j].gram)
		}
		return candidates[i].count > candidates[j].count
	})

	var result []string
	seen := make(map[string]bool)
	for _, c := range candidates {
		isSub := false
		for s := range seen {
			if strings.Contains(s, c.gram) {
				isSub = true
				break
			}
		}
		if !isSub {
			seen[c.gram] = true
			result = append(result, fmt.Sprintf("%s(×%d)", c.gram, c.count))
			if len(result) >= 5 {
				break
			}
		}
	}
	return result
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

// IsExclamationExcessive checks if exclamation marks are overused in the given text.
func IsExclamationExcessive(text string, density float64) bool {
	runes := []rune(text)
	if len(runes) == 0 {
		return false
	}
	count := 0
	for _, r := range runes {
		if r == '！' || r == '!' {
			count++
		}
	}
	// For short text (< 500 characters), only flag if heavily shouting (>= 5 exclamations)
	if len(runes) < 500 {
		return count >= 5 && density > 10.0
	}
	// For chapter-length text (>= 500 characters), flag if density > 4.0 per 1000 characters
	return density > 4.0 && count >= 4
}

// CountSimileClichés scans for formulaic simile patterns (像...一样/如...般/宛如/犹如).
func CountSimileClichés(text string) int {
	patterns := []string{
		"像干枯", "像生锈", "像钝刀", "犹如", "宛如", "如同", "好似",
		"夜枭般", "泥牛入海", "丧家之犬", "古井无波", "般地", "般的",
	}
	count := 0
	for _, p := range patterns {
		count += strings.Count(text, p)
	}
	return count
}

// DetectTelegraphicFragmentation checks if text suffers from broken telegraphic sentences
// (e.g. subject/predicate/object omitted, lines degraded into 1-4 character isolated fragments,
// or game combat stats announced in dialogue).
func DetectTelegraphicFragmentation(text string) (bool, string) {
	lines := strings.Split(text, "\n")
	nonEmptyLines := 0
	ultraShortLines := 0
	consecutiveUltraShort := 0
	maxConsecutiveUltraShort := 0
	singleWordLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			consecutiveUltraShort = 0
			continue
		}
		nonEmptyLines++
		runes := []rune(trimmed)
		runeLen := len(runes)

		// Line with <= 2 runes (e.g. "响。", "咔。", "烫。")
		if runeLen <= 2 {
			singleWordLines++
		}

		if runeLen <= 4 {
			ultraShortLines++
			consecutiveUltraShort++
			if consecutiveUltraShort > maxConsecutiveUltraShort {
				maxConsecutiveUltraShort = consecutiveUltraShort
			}
		} else {
			consecutiveUltraShort = 0
		}
	}

	if nonEmptyLines >= 5 {
		// Rule 1: 3 or more consecutive ultra-short lines (e.g. "一滴。\n砸桌。\n响。")
		if maxConsecutiveUltraShort >= 3 {
			return true, fmt.Sprintf("存在连续%d行电报式残疾断句(单字单词孤立成行，缺失主谓宾完整结构)", maxConsecutiveUltraShort)
		}

		// Rule 2: Over 25% ultra-short lines
		ratio := float64(ultraShortLines) / float64(nonEmptyLines)
		if ratio > 0.25 && ultraShortLines >= 4 {
			return true, fmt.Sprintf("电报式碎片短行占比过高(%.1f%%，缺失主谓宾自然结构)", ratio*100)
		}

		// Rule 3: Multiple 1-2 character isolated lines
		if singleWordLines >= 2 {
			return true, fmt.Sprintf("出现%d处单字独行成句(如'响。'、'烫。')，行文严重失真", singleWordLines)
		}
	}

	// Rule 4: Game skill announcement patterns in quotes or standalone
	if strings.Contains(text, "耗香火") || strings.Contains(text, "耗香灰") || strings.Contains(text, "炼气三层，耗") {
		return true, "出现游戏数值式技能战报台词(如'耗香火三缕')，破坏沉浸式小说叙事"
	}

	return false, ""
}

// ScanEmpiricalAITells scans for the high-confidence empirical AI writing tells identified in empirical writing studies.
func ScanEmpiricalAITells(text string) []string {
	var tells []string

	// 1. 翻案腔 (Contrarian False Dilemmas): 不是...而是..., 看似...实则..., 与其说...不如说...
	contrarianRegexes := []*regexp.Regexp{
		regexp.MustCompile(`(不是|并非)[^\n。！？]{1,25}而是`),
		regexp.MustCompile(`看似[^\n。！？]{1,25}(实则|其实)`),
		regexp.MustCompile(`与其说[^\n。！？]{1,25}不如说`),
		regexp.MustCompile(`表面[^\n。！？]{1,25}(实际|实则)`),
		regexp.MustCompile(`你以为[^\n。！？]{1,25}其实`),
		regexp.MustCompile(`不在于[^\n。！？]{1,25}而在于`),
	}
	for _, re := range contrarianRegexes {
		if m := re.FindString(text); m != "" {
			tells = append(tells, fmt.Sprintf("翻案腔虚立假靶子(%s)", m))
		}
	}

	// 2. 密集顿号罗列 (Dense Dunhao in single clause)
	clauses := strings.FieldsFunc(text, func(r rune) bool {
		return r == '，' || r == '。' || r == '！' || r == '？' || r == '；' || r == '\n' || r == ',' || r == '.' || r == ';'
	})
	denseDunhaoCount := 0
	for _, c := range clauses {
		dh := strings.Count(c, "、")
		if dh >= 3 {
			denseDunhaoCount++
		}
	}
	if denseDunhaoCount > 0 {
		tells = append(tells, fmt.Sprintf("单句密集顿号罗列清单(%d处并列超标)", denseDunhaoCount))
	}

	// 3. 提示语+冒号空转
	colonPromptRe := regexp.MustCompile(`(一句话总结|核心是|关键在于|原因如下|结论是|本质上|换句话说)[：:]`)
	if m := colonPromptRe.FindString(text); m != "" {
		tells = append(tells, fmt.Sprintf("提示语冒号空转句(%s)", m))
	}

	// 4. 机械时间从句壳子 (当...时，)
	whenRe := regexp.MustCompile(`当[^\n，。！？]{2,25}时，`)
	whenMatches := whenRe.FindAllString(text, -1)
	if len(whenMatches) >= 2 {
		tells = append(tells, fmt.Sprintf("机械时间从句壳子'当...时'(命中%d处)", len(whenMatches)))
	}

	// 5. 前置话题壳 (对于...来说/而言，)
	topicRe := regexp.MustCompile(`(对于|就|关于)[^\n，。！？]{2,20}(来说|而言)，`)
	topicMatches := topicRe.FindAllString(text, -1)
	if len(topicMatches) >= 2 {
		tells = append(tells, fmt.Sprintf("前置话题壳'对于...而言'(命中%d处)", len(topicMatches)))
	}

	// 6. 揭晓式破折号 (——)
	dashCount := strings.Count(text, "——")
	if dashCount >= 2 {
		tells = append(tells, fmt.Sprintf("揭晓式破折号停顿滥用(%d处)", dashCount))
	}

	// 7. 机械复述提示词 (这意味着/这表明)
	repeatPromptRe := regexp.MustCompile(`[。；]\s*(这意味着|这表明|这说明)[，,]?`)
	if m := repeatPromptRe.FindString(text); m != "" {
		tells = append(tells, "机械复述句提示词(这意味着/这表明)")
	}

	// 8. 理想化职业拟人喻体
	anthroRe := regexp.MustCompile(`(像|宛如|犹如|相当于)(一位|一个)(智慧的|全能的|不知疲倦的|沉默的)?(导师|管家|秘书|助手|顾问|审查员)`)
	if m := anthroRe.FindString(text); m != "" {
		tells = append(tells, fmt.Sprintf("理想化工具职业拟人喻体(%s)", m))
	}

	if tells == nil {
		tells = []string{}
	}
	return tells
}
