package metrics_test

import (
	"math"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/metrics"
)

func TestAggregate_Empty(t *testing.T) {
	summary := metrics.Aggregate([]*domain.ChapterQualityMetrics{})
	if summary.Chapters != 0 {
		t.Errorf("expected 0 chapters, got %d", summary.Chapters)
	}
	if summary.WordPassRate != 0.0 || summary.AvgScore != 0.0 || summary.HookPassRate != 0.0 {
		t.Errorf("expected zero rates for empty input, got %+v", summary)
	}
}

func TestAggregate_MixedOutcomes(t *testing.T) {
	items := []*domain.ChapterQualityMetrics{
		{
			ProjectID:              "p-metrics",
			ChapterIndex:           1,
			ActualWords:            2200,
			WordsTarget:            2000,
			WordQuotaRatio:         1.1,
			UnderTarget:            false,
			Verdict:                "ACCEPTED",
			Score:                  90,
			RewriteLoops:           0,
			UnresolvedObservations: 0,
			HookClaimsTotal:        2,
			HookClaimsValidated:    2,
			HookClaimsRejected:     0,
			CompactionFailed:       false,
			Outcome:                domain.ChapterOutcomeCommitted,
			CreatedAt:              time.Now(),
		},
		{
			ProjectID:              "p-metrics",
			ChapterIndex:           2,
			ActualWords:            1500,
			WordsTarget:            2000,
			WordQuotaRatio:         0.75,
			UnderTarget:            true,
			Verdict:                "REJECTED",
			Score:                  60,
			RewriteLoops:           2,
			UnresolvedObservations: 1,
			HookClaimsTotal:        2,
			HookClaimsValidated:    1,
			HookClaimsRejected:     1,
			CompactionFailed:       false,
			Outcome:                domain.ChapterOutcomeNeedsHuman,
			CreatedAt:              time.Now(),
		},
		{
			ProjectID:              "p-metrics",
			ChapterIndex:           3,
			ActualWords:            2000,
			WordsTarget:            2000,
			WordQuotaRatio:         1.0,
			UnderTarget:            false,
			Verdict:                "ACCEPTED",
			Score:                  80,
			RewriteLoops:           0,
			UnresolvedObservations: 0,
			HookClaimsTotal:        0,
			HookClaimsValidated:    0,
			HookClaimsRejected:     0,
			CompactionFailed:       true,
			Outcome:                domain.ChapterOutcomeTechnicalBlocked,
			CreatedAt:              time.Now(),
		},
		{
			ProjectID:              "p-metrics",
			ChapterIndex:           4,
			ActualWords:            2100,
			WordsTarget:            2000,
			WordQuotaRatio:         1.05,
			UnderTarget:            false,
			Verdict:                "ACCEPTED",
			Score:                  90,
			RewriteLoops:           0,
			UnresolvedObservations: 0,
			HookClaimsTotal:        1,
			HookClaimsValidated:    1,
			HookClaimsRejected:     0,
			CompactionFailed:       false,
			Outcome:                domain.ChapterOutcomeProduced,
			CreatedAt:              time.Now(),
		},
	}

	summary := metrics.Aggregate(items)

	if summary.Chapters != 4 {
		t.Fatalf("expected 4 chapters, got %d", summary.Chapters)
	}
	if summary.CommittedChapters != 1 {
		t.Errorf("expected 1 committed chapter, got %d", summary.CommittedChapters)
	}
	if summary.NeedsHumanChapters != 1 {
		t.Errorf("expected 1 needs_human chapter, got %d", summary.NeedsHumanChapters)
	}
	if summary.TechnicalBlockedChapters != 1 {
		t.Errorf("expected 1 technical_blocked chapter, got %d", summary.TechnicalBlockedChapters)
	}
	if summary.ProducedChapters != 1 {
		t.Errorf("expected 1 produced chapter, got %d", summary.ProducedChapters)
	}
	if summary.UnderTargetChapters != 1 {
		t.Errorf("expected 1 under_target chapter, got %d", summary.UnderTargetChapters)
	}
	if summary.RewriteChapters != 1 {
		t.Errorf("expected 1 rewrite chapter, got %d", summary.RewriteChapters)
	}
	if summary.UnclosedReconciliationChapters != 1 {
		t.Errorf("expected 1 unclosed reconciliation chapter, got %d", summary.UnclosedReconciliationChapters)
	}
	if summary.CompactionFailedChapters != 1 {
		t.Errorf("expected 1 compaction failed chapter, got %d", summary.CompactionFailedChapters)
	}

	expectedWordPassRate := 0.75 // 1 - 1/4
	if math.Abs(summary.WordPassRate-expectedWordPassRate) > 1e-6 {
		t.Errorf("expected WordPassRate %f, got %f", expectedWordPassRate, summary.WordPassRate)
	}

	expectedAvgScore := 80.0 // (90+60+80+90)/4 = 320/4 = 80
	if math.Abs(summary.AvgScore-expectedAvgScore) > 1e-6 {
		t.Errorf("expected AvgScore %f, got %f", expectedAvgScore, summary.AvgScore)
	}

	expectedRewriteRate := 0.25 // 1/4
	if math.Abs(summary.RewriteRate-expectedRewriteRate) > 1e-6 {
		t.Errorf("expected RewriteRate %f, got %f", expectedRewriteRate, summary.RewriteRate)
	}

	if summary.HookClaimsTotal != 5 {
		t.Errorf("expected 5 hook claims, got %d", summary.HookClaimsTotal)
	}
	if summary.HookClaimsValidated != 4 {
		t.Errorf("expected 4 hook claims validated, got %d", summary.HookClaimsValidated)
	}
	if summary.HookClaimsRejected != 1 {
		t.Errorf("expected 1 hook claim rejected, got %d", summary.HookClaimsRejected)
	}
	expectedHookPassRate := 4.0 / 5.0
	if math.Abs(summary.HookPassRate-expectedHookPassRate) > 1e-6 {
		t.Errorf("expected HookPassRate %f, got %f", expectedHookPassRate, summary.HookPassRate)
	}
}
