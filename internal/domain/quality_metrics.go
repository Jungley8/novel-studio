package domain

import "time"

// ChapterQualityMetrics 章节质量指标（评测闭环数据源；commit 成功后落库）。
type ChapterQualityMetrics struct {
	ProjectID              string    `json:"project_id"`
	ChapterIndex           int       `json:"chapter_index"`
	RunID                  string    `json:"run_id,omitempty"`
	ActualWords            int       `json:"actual_words"`
	WordsTarget            int       `json:"words_target"`
	WordQuotaRatio         float64   `json:"word_quota_ratio"`
	UnderTarget            bool      `json:"under_target"`
	Verdict                string    `json:"verdict"`
	Score                  int       `json:"score"`
	RewriteLoops           int       `json:"rewrite_loops"`
	UnresolvedObservations int       `json:"unresolved_observations"`
	HookClaimsTotal        int       `json:"hook_claims_total"`
	HookClaimsValidated    int       `json:"hook_claims_validated"`
	HookClaimsRejected     int       `json:"hook_claims_rejected"`
	CompactionFailed       bool      `json:"compaction_failed"`
	Outcome                string    `json:"outcome"`
	CreatedAt              time.Time `json:"created_at"`
}

// 所有终态都落库，避免只记成功章的幸存者偏差
const (
	ChapterOutcomeCommitted        = "committed"         // 自动验收通过并原子入库
	ChapterOutcomeNeedsHuman       = "needs_human"       // 语义停线：达最大重写轮次仍未通过
	ChapterOutcomeTechnicalBlocked = "technical_blocked" // 技术阻断：状态结算未通过，拒绝自动入库
	ChapterOutcomeProduced         = "produced"          // 已产出未入库（AutoCommit=false，待人工审阅）
)
