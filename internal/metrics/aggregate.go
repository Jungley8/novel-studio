package metrics

import "github.com/Jungley8/novel-studio/internal/domain"

// Summary encapsulates aggregated chapter quality statistics for a project.
type Summary struct {
	Chapters                       int     `json:"chapters"`
	CommittedChapters              int     `json:"committed_chapters"`
	NeedsHumanChapters             int     `json:"needs_human_chapters"`
	TechnicalBlockedChapters       int     `json:"technical_blocked_chapters"`
	ProducedChapters               int     `json:"produced_chapters"`
	UnderTargetChapters            int     `json:"under_target_chapters"`
	RewriteChapters                int     `json:"rewrite_chapters"`
	UnclosedReconciliationChapters int     `json:"unclosed_reconciliation_chapters"`
	CompactionFailedChapters       int     `json:"compaction_failed_chapters"`
	WordPassRate                   float64 `json:"word_pass_rate"`
	AvgScore                       float64 `json:"avg_score"`
	RewriteRate                    float64 `json:"rewrite_rate"`
	HookClaimsTotal                int     `json:"hook_claims_total"`
	HookClaimsValidated            int     `json:"hook_claims_validated"`
	HookClaimsRejected             int     `json:"hook_claims_rejected"`
	HookPassRate                   float64 `json:"hook_pass_rate"`
}

// Aggregate aggregates chapter quality metrics for reporting.
func Aggregate(items []*domain.ChapterQualityMetrics) Summary {
	var summary Summary
	if len(items) == 0 {
		return summary
	}

	summary.Chapters = len(items)
	totalScore := 0

	for _, it := range items {
		if it == nil {
			continue
		}
		switch it.Outcome {
		case domain.ChapterOutcomeCommitted:
			summary.CommittedChapters++
		case domain.ChapterOutcomeNeedsHuman:
			summary.NeedsHumanChapters++
		case domain.ChapterOutcomeTechnicalBlocked:
			summary.TechnicalBlockedChapters++
		case domain.ChapterOutcomeProduced:
			summary.ProducedChapters++
		}
		if it.UnderTarget {
			summary.UnderTargetChapters++
		}
		if it.RewriteLoops > 0 {
			summary.RewriteChapters++
		}
		if it.UnresolvedObservations > 0 {
			summary.UnclosedReconciliationChapters++
		}
		if it.CompactionFailed {
			summary.CompactionFailedChapters++
		}
		totalScore += it.Score
		summary.HookClaimsTotal += it.HookClaimsTotal
		summary.HookClaimsValidated += it.HookClaimsValidated
		summary.HookClaimsRejected += it.HookClaimsRejected
	}

	summary.WordPassRate = 1.0 - float64(summary.UnderTargetChapters)/float64(summary.Chapters)
	summary.AvgScore = float64(totalScore) / float64(summary.Chapters)
	summary.RewriteRate = float64(summary.RewriteChapters) / float64(summary.Chapters)

	if summary.HookClaimsTotal > 0 {
		summary.HookPassRate = float64(summary.HookClaimsValidated) / float64(summary.HookClaimsTotal)
	}

	return summary
}
