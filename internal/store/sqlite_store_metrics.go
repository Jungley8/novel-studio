package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// SaveChapterMetrics persists or updates chapter quality metrics with an upsert.
func (s *SQLiteStore) SaveChapterMetrics(ctx context.Context, m *domain.ChapterQualityMetrics) error {
	if m == nil {
		return errors.New("chapter metrics cannot be nil")
	}
	if strings.TrimSpace(m.ProjectID) == "" {
		return errors.New("chapter metrics project_id cannot be empty")
	}
	if m.ChapterIndex <= 0 {
		return errors.New("chapter metrics chapter_index must be positive")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}

	underTargetInt := 0
	if m.UnderTarget {
		underTargetInt = 1
	}
	compactionFailedInt := 0
	if m.CompactionFailed {
		compactionFailedInt = 1
	}

	query := `
	INSERT INTO chapter_quality_metrics (
		project_id, chapter_index, run_id, actual_words, words_target,
		word_quota_ratio, under_target, verdict, score, rewrite_loops,
		unresolved_observations, hook_claims_total, hook_claims_validated,
		hook_claims_rejected, compaction_failed, created_at
	) VALUES (
		?, ?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?
	)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		run_id = excluded.run_id,
		actual_words = excluded.actual_words,
		words_target = excluded.words_target,
		word_quota_ratio = excluded.word_quota_ratio,
		under_target = excluded.under_target,
		verdict = excluded.verdict,
		score = excluded.score,
		rewrite_loops = excluded.rewrite_loops,
		unresolved_observations = excluded.unresolved_observations,
		hook_claims_total = excluded.hook_claims_total,
		hook_claims_validated = excluded.hook_claims_validated,
		hook_claims_rejected = excluded.hook_claims_rejected,
		compaction_failed = excluded.compaction_failed,
		created_at = excluded.created_at;
	`

	_, err := s.db.ExecContext(ctx, query,
		m.ProjectID, m.ChapterIndex, m.RunID, m.ActualWords, m.WordsTarget,
		m.WordQuotaRatio, underTargetInt, m.Verdict, m.Score, m.RewriteLoops,
		m.UnresolvedObservations, m.HookClaimsTotal, m.HookClaimsValidated,
		m.HookClaimsRejected, compactionFailedInt, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save chapter metrics: %w", err)
	}
	return nil
}

// ListChapterMetrics retrieves all metrics for a given project, sorted by chapter_index ASC.
func (s *SQLiteStore) ListChapterMetrics(ctx context.Context, projectID string) ([]*domain.ChapterQualityMetrics, error) {
	query := `
	SELECT
		project_id, chapter_index, COALESCE(run_id, ''), actual_words, words_target,
		word_quota_ratio, under_target, verdict, score, rewrite_loops,
		unresolved_observations, hook_claims_total, hook_claims_validated,
		hook_claims_rejected, compaction_failed, created_at
	FROM chapter_quality_metrics
	WHERE project_id = ?
	ORDER BY chapter_index ASC;
	`

	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("query chapter metrics: %w", err)
	}
	defer rows.Close()

	var list []*domain.ChapterQualityMetrics
	for rows.Next() {
		var m domain.ChapterQualityMetrics
		var underTargetInt, compactionFailedInt int
		if err := rows.Scan(
			&m.ProjectID, &m.ChapterIndex, &m.RunID, &m.ActualWords, &m.WordsTarget,
			&m.WordQuotaRatio, &underTargetInt, &m.Verdict, &m.Score, &m.RewriteLoops,
			&m.UnresolvedObservations, &m.HookClaimsTotal, &m.HookClaimsValidated,
			&m.HookClaimsRejected, &compactionFailedInt, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chapter metrics: %w", err)
		}
		m.UnderTarget = underTargetInt != 0
		m.CompactionFailed = compactionFailedInt != 0
		list = append(list, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chapter metrics: %w", err)
	}
	return list, nil
}
