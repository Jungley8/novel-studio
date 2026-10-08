package engine

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

// HarmonizedItem records a single sensitive or high-risk phrase replacement.
type HarmonizedItem struct {
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
	Category    string `json:"category"` // "GORE" | "VULGAR" | "SENSITIVE"
}

// HumanTouchSuggestion provides actionable advice for human-in-the-loop imperfection injection.
type HumanTouchSuggestion struct {
	Category        string `json:"category"`         // "PHYSIOLOGY" | "TRIVIALITY" | "COLLOQUIALISM" | "CADENCE"
	OriginalContext string `json:"original_context"` // 触发建议的原文章节上下文
	Suggestion      string `json:"suggestion"`       // 具体可注入的人味细节
	Rationale       string `json:"rationale"`        // 破除朱雀等 AIGC 检测似然特征的原理
}

// HarmonizeReport provides a summary of all anti-detection and compliance modifications.
type HarmonizeReport struct {
	HarmonizedItems    []HarmonizedItem       `json:"harmonized_items"`
	SuggestedTouches   []HumanTouchSuggestion `json:"suggested_touches"`
	OriginalWordCount  int                    `json:"original_word_count"`
	ProcessedWordCount int                    `json:"processed_word_count"`
}

// Harmonizer handles domestic platform content harmonization and adversarial AI perturbation.
type Harmonizer struct {
	mu           sync.RWMutex
	censorRules  map[string]censorEntry
	perturbRules map[string]string
	rnd          *rand.Rand
}

type censorEntry struct {
	replacement string
	category    string
}

func NewHarmonizer() *Harmonizer {
	h := &Harmonizer{
		censorRules:  make(map[string]censorEntry),
		perturbRules: make(map[string]string),
		rnd:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	h.initDefaultRules()
	return h
}

func (h *Harmonizer) initDefaultRules() {
	// 1. Domestic platform high-risk Gore & Violence rules
	goreList := map[string]string{
		"开膛破肚": "重创倒地",
		"血肉模糊": "一片狼藉",
		"脑浆四溅": "重创毙命",
		"掐断喉管": "扼住要害",
		"碎成肉泥": "筋骨尽断",
		"五马分尸": "凌厉灭杀",
		"大卸八块": "彻底击溃",
		"尸体":   "残躯",
		"死人":   "亡者",
		"剥皮抽筋": "尽废根基",
		"鲜血狂喷": "受创暴退",
		"残肢断臂": "散落断刃",
		"血流成河": "战局惨烈",
		"割下首级": "斩落头冠",
		"碎尸万段": "万劫不复",
	}
	for k, v := range goreList {
		h.censorRules[k] = censorEntry{replacement: v, category: "GORE"}
	}

	// 2. Vulgar curses to martial/period colloquialisms
	vulgarList := map[string]string{
		"操你妈": "直娘贼",
		"干你娘": "狗贼纳命来",
		"傻逼":  "蠢材",
		"他妈的": "娘的",
		"去死吧": "受死吧",
		"下三滥": "卑劣之徒",
	}
	for k, v := range vulgarList {
		h.censorRules[k] = censorEntry{replacement: v, category: "VULGAR"}
	}

	// 3. Adversarial Lexical Jitter (Low-probability human phrasing replacing LLM high-PPL clichés)
	h.perturbRules = map[string]string{
		"走过去":  "大步踏过去",
		"走入":   "跨步迈进",
		"看着":   "乜斜着盯牢",
		"站起来":  "霍然起身",
		"十分":   "分明是",
		"拿出了":  "怀里摸出",
		"眼神复杂": "嘴角紧抿",
		"停顿了":  "手头微滞",
		"立刻":   "当即",
		"忽然":   "冷不丁",
		"缓慢":   "慢吞吞",
		"剧烈":   "生猛",
		"非常":   "端的是",
		"仔细观察": "眯眼打量",
		"心中明白": "心如明镜",
	}
}

// HarmonizeSensitiveWords replaces high-risk domestic web novel keywords with safe, passing alternatives.
func (h *Harmonizer) HarmonizeSensitiveWords(text string) (string, []HarmonizedItem) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := text
	var harmonized []HarmonizedItem

	for orig, entry := range h.censorRules {
		if strings.Contains(result, orig) {
			result = strings.ReplaceAll(result, orig, entry.replacement)
			harmonized = append(harmonized, HarmonizedItem{
				Original:    orig,
				Replacement: entry.replacement,
				Category:    entry.category,
			})
		}
	}

	return result, harmonized
}

// Perturb injects adversarial lexical and cadence variance to disrupt LLM log-likelihood detection.
func (h *Harmonizer) Perturb(text string, intensity float64) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if intensity <= 0 {
		return text
	}
	if intensity > 1.0 {
		intensity = 1.0
	}

	result := text

	// 1. Lexical perturbation
	for orig, repl := range h.perturbRules {
		if strings.Contains(result, orig) {
			// Apply with probability based on intensity
			if h.rnd.Float64() <= intensity {
				result = strings.Replace(result, orig, repl, 1)
			}
		}
	}

	// 2. Cadence perturbation: break mechanical comma rhythms
	lines := strings.Split(result, "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len([]rune(trimmed)) > 25 && strings.Count(trimmed, "，") >= 3 {
			if h.rnd.Float64() <= intensity*0.5 {
				// Replace second comma with an em-dash for human breath cadence
				parts := strings.Split(trimmed, "，")
				if len(parts) >= 3 {
					trimmed = parts[0] + "，" + parts[1] + "——" + strings.Join(parts[2:], "，")
				}
			}
		}
		newLines = append(newLines, trimmed)
	}

	return strings.Join(newLines, "\n")
}

// SuggestHumanTouches generates actionable recommendations for author-driven life imperfections.
func (h *Harmonizer) SuggestHumanTouches(text string) []HumanTouchSuggestion {
	var suggestions []HumanTouchSuggestion

	// 1. Physiology suggestion
	if strings.Contains(text, "雪") || strings.Contains(text, "风") || strings.Contains(text, "寒") || strings.Contains(text, "夜") {
		suggestions = append(suggestions, HumanTouchSuggestion{
			Category:        "PHYSIOLOGY",
			OriginalContext: "风雪与寒夜场景",
			Suggestion:      "在主角身上插入1处具体的生理不适体感（如：'鞋袜湿透冰得大脚趾发木'，或'三天没见热食胃里直泛酸水'）。",
			Rationale:       "大模型只会描写客观宏观天气，人类作者在恶劣环境中天然具有‘认知隧道效应’，生理不适可瞬间打破低熵似然度。",
		})
	}

	// 2. Triviality suggestion
	if strings.Contains(text, "酒") || strings.Contains(text, "店") || strings.Contains(text, "楼") || strings.Contains(text, "案") {
		suggestions = append(suggestions, HumanTouchSuggestion{
			Category:        "TRIVIALITY",
			OriginalContext: "客栈与室内交互场景",
			Suggestion:      "在对峙或谈话间隙，插入1处与主线剧情毫无关系的荒诞生活闲笔（如：'柜台角落一只老猫正嗅着凉油'，或'炭盆里湿柴冒烟呛得人揉眼'）。",
			Rationale:       "大模型写文效率过高、纯度100%零废话。插入无关生活闲笔能为文本注入真实人类文本特有的‘生活颗粒感’。",
		})
	}

	// 3. Colloquialism suggestion
	if strings.Contains(text, "“") || strings.Contains(text, "”") {
		suggestions = append(suggestions, HumanTouchSuggestion{
			Category:        "COLLOQUIALISM",
			OriginalContext: "角色对话言语交锋",
			Suggestion:      "在角色台词中加入带市井粗粝感的口癖或吞字打断（如：'算得哪门子账'，或'娘的'，或在关键处结巴一瞬）。",
			Rationale:       "打破大模型书面规整的乒乓球式问答，口语粗粝感是腾讯朱雀等分类器最显著的人类语言指纹。",
		})
	}

	// 4. Cadence suggestion
	suggestions = append(suggestions, HumanTouchSuggestion{
		Category:        "CADENCE",
		OriginalContext: "打斗与危机高潮段落",
		Suggestion:      "将一处顺畅完整的动作链刻意切碎成1~3字短句，并故意删去后半句的解释（留白给读者脑补）。",
		Rationale:       "大模型习惯逐帧解释动作起因与结果，人类写手在高潮处会省略主谓结构、直给冲击感。",
	})

	return suggestions
}

// FullProcess runs both censor harmonization and adversarial perturbation, returning a comprehensive report.
func (h *Harmonizer) FullProcess(text string, perturbIntensity float64) (string, HarmonizeReport) {
	origRunes := len([]rune(text))

	// 1. Censor Harmonization (Safety & Compliance)
	harmonizedText, items := h.HarmonizeSensitiveWords(text)

	// 2. Adversarial Perturbation (Anti-AI Likelihood Jitter)
	finalText := h.Perturb(harmonizedText, perturbIntensity)

	// 3. Suggest Human Touches (Human-in-the-loop assistance)
	suggestions := h.SuggestHumanTouches(finalText)

	return finalText, HarmonizeReport{
		HarmonizedItems:    items,
		SuggestedTouches:   suggestions,
		OriginalWordCount:  origRunes,
		ProcessedWordCount: len([]rune(finalText)),
	}
}
