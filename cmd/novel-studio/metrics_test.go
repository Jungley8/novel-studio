package main

import (
	"math"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestAggregateMetrics_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    []*domain.ChapterQualityMetrics
		expected MetricsSummary
	}{
		{
			name:  "empty set",
			input: []*domain.ChapterQualityMetrics{},
			expected: MetricsSummary{
				Chapters:                       0,
				UnderTargetChapters:            0,
				RewriteChapters:                0,
				UnclosedReconciliationChapters: 0,
				CompactionFailedChapters:       0,
				WordPassRate:                   0.0,
				AvgScore:                       0.0,
				RewriteRate:                    0.0,
				HookClaimsTotal:                0,
				HookClaimsValidated:            0,
				HookClaimsRejected:             0,
				HookPassRate:                   0.0,
			},
		},
		{
			name: "all pass set",
			input: []*domain.ChapterQualityMetrics{
				{
					ProjectID:              "p-test",
					ChapterIndex:           1,
					ActualWords:            2200,
					WordsTarget:            2000,
					WordQuotaRatio:         1.1,
					UnderTarget:            false,
					Verdict:                "ACCEPTED",
					Score:                  95,
					RewriteLoops:           0,
					UnresolvedObservations: 0,
					HookClaimsTotal:        2,
					HookClaimsValidated:    2,
					HookClaimsRejected:     0,
					CompactionFailed:       false,
					CreatedAt:              time.Now(),
				},
				{
					ProjectID:              "p-test",
					ChapterIndex:           2,
					ActualWords:            2100,
					WordsTarget:            2000,
					WordQuotaRatio:         1.05,
					UnderTarget:            false,
					Verdict:                "ACCEPTED",
					Score:                  85,
					RewriteLoops:           0,
					UnresolvedObservations: 0,
					HookClaimsTotal:        1,
					HookClaimsValidated:    1,
					HookClaimsRejected:     0,
					CompactionFailed:       false,
					CreatedAt:              time.Now(),
				},
			},
			expected: MetricsSummary{
				Chapters:                       2,
				UnderTargetChapters:            0,
				RewriteChapters:                0,
				UnclosedReconciliationChapters: 0,
				CompactionFailedChapters:       0,
				WordPassRate:                   1.0,
				AvgScore:                       90.0,
				RewriteRate:                    0.0,
				HookClaimsTotal:                3,
				HookClaimsValidated:            3,
				HookClaimsRejected:             0,
				HookPassRate:                   1.0,
			},
		},
		{
			name: "mixed set",
			input: []*domain.ChapterQualityMetrics{
				{
					ProjectID:              "p-test",
					ChapterIndex:           1,
					ActualWords:            2100,
					WordsTarget:            2000,
					WordQuotaRatio:         1.05,
					UnderTarget:            false,
					Verdict:                "ACCEPTED",
					Score:                  90,
					RewriteLoops:           0,
					UnresolvedObservations: 0,
					HookClaimsTotal:        2,
					HookClaimsValidated:    1,
					HookClaimsRejected:     1,
					CompactionFailed:       false,
					CreatedAt:              time.Now(),
				},
				{
					ProjectID:              "p-test",
					ChapterIndex:           2,
					ActualWords:            1200,
					WordsTarget:            2000,
					WordQuotaRatio:         0.6,
					UnderTarget:            true,
					Verdict:                "REJECTED",
					Score:                  50,
					RewriteLoops:           2,
					UnresolvedObservations: 3,
					HookClaimsTotal:        2,
					HookClaimsValidated:    1,
					HookClaimsRejected:     1,
					CompactionFailed:       true,
					CreatedAt:              time.Now(),
				},
			},
			expected: MetricsSummary{
				Chapters:                       2,
				UnderTargetChapters:            1,
				RewriteChapters:                1,
				UnclosedReconciliationChapters: 1,
				CompactionFailedChapters:       1,
				WordPassRate:                   0.5,
				AvgScore:                       70.0,
				RewriteRate:                    0.5,
				HookClaimsTotal:                4,
				HookClaimsValidated:            2,
				HookClaimsRejected:             2,
				HookPassRate:                   0.5,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := aggregateMetrics(tc.input)

			if got.Chapters != tc.expected.Chapters {
				t.Errorf("Chapters: expected %d, got %d", tc.expected.Chapters, got.Chapters)
			}
			if got.UnderTargetChapters != tc.expected.UnderTargetChapters {
				t.Errorf("UnderTargetChapters: expected %d, got %d", tc.expected.UnderTargetChapters, got.UnderTargetChapters)
			}
			if got.RewriteChapters != tc.expected.RewriteChapters {
				t.Errorf("RewriteChapters: expected %d, got %d", tc.expected.RewriteChapters, got.RewriteChapters)
			}
			if got.UnclosedReconciliationChapters != tc.expected.UnclosedReconciliationChapters {
				t.Errorf("UnclosedReconciliationChapters: expected %d, got %d", tc.expected.UnclosedReconciliationChapters, got.UnclosedReconciliationChapters)
			}
			if got.CompactionFailedChapters != tc.expected.CompactionFailedChapters {
				t.Errorf("CompactionFailedChapters: expected %d, got %d", tc.expected.CompactionFailedChapters, got.CompactionFailedChapters)
			}
			if math.Abs(got.WordPassRate-tc.expected.WordPassRate) > 1e-6 {
				t.Errorf("WordPassRate: expected %f, got %f", tc.expected.WordPassRate, got.WordPassRate)
			}
			if math.Abs(got.AvgScore-tc.expected.AvgScore) > 1e-6 {
				t.Errorf("AvgScore: expected %f, got %f", tc.expected.AvgScore, got.AvgScore)
			}
			if math.Abs(got.RewriteRate-tc.expected.RewriteRate) > 1e-6 {
				t.Errorf("RewriteRate: expected %f, got %f", tc.expected.RewriteRate, got.RewriteRate)
			}
			if got.HookClaimsTotal != tc.expected.HookClaimsTotal {
				t.Errorf("HookClaimsTotal: expected %d, got %d", tc.expected.HookClaimsTotal, got.HookClaimsTotal)
			}
			if got.HookClaimsValidated != tc.expected.HookClaimsValidated {
				t.Errorf("HookClaimsValidated: expected %d, got %d", tc.expected.HookClaimsValidated, got.HookClaimsValidated)
			}
			if got.HookClaimsRejected != tc.expected.HookClaimsRejected {
				t.Errorf("HookClaimsRejected: expected %d, got %d", tc.expected.HookClaimsRejected, got.HookClaimsRejected)
			}
			if math.Abs(got.HookPassRate-tc.expected.HookPassRate) > 1e-6 {
				t.Errorf("HookPassRate: expected %f, got %f", tc.expected.HookPassRate, got.HookPassRate)
			}
		})
	}
}
