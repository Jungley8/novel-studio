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
	CreatedAt              time.Time `json:"created_at"`
}
