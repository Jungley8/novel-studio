package domain

// PlantedHookCandidate 伏笔登记候选：本章新埋下的伏笔 + 正文证据引用（供登记证据门校验）。
type PlantedHookCandidate struct {
	Title         string `json:"title"`
	Details       string `json:"details"`
	Quote         string `json:"quote"`          // 正文兑现引用（≥8 字，须为正文子串）
	TargetChapter int    `json:"target_chapter"` // 建议回收章（≤当前章时归一为当前章+5）
}
