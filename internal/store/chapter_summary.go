package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// ensureSummaryTables lazily provisions the chapter_summaries and volume_retrospectives
// tables and indices if they do not already exist. It is idempotent.
func (s *SQLiteStore) ensureSummaryTables(ctx context.Context) error {
	if s.db == nil {
		return errors.New("sqlite database connection is nil")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS chapter_summaries (
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		text TEXT NOT NULL,
		model TEXT NOT NULL DEFAULT '',
		tokens INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		PRIMARY KEY (project_id, chapter_index)
	);
	CREATE INDEX IF NOT EXISTS idx_chapter_summaries_proj_idx ON chapter_summaries(project_id, chapter_index);

	CREATE TABLE IF NOT EXISTS volume_retrospectives (
		project_id TEXT NOT NULL,
		volume_index INTEGER NOT NULL,
		text TEXT NOT NULL,
		from_chapter INTEGER NOT NULL DEFAULT 0,
		to_chapter INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		PRIMARY KEY (project_id, volume_index)
	);
	CREATE INDEX IF NOT EXISTS idx_volume_retrospectives_proj_vol ON volume_retrospectives(project_id, volume_index);
	`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("ensure summary tables failed: %w", err)
	}
	return nil
}

// EnsureSummaryTables is an exported wrapper around ensureSummaryTables for callers and tests.
func (s *SQLiteStore) EnsureSummaryTables(ctx context.Context) error {
	return s.ensureSummaryTables(ctx)
}

// SaveChapterSummary persists a chapter summary, updating existing record if present.
func (s *SQLiteStore) SaveChapterSummary(ctx context.Context, cs *domain.ChapterSummary) error {
	if cs == nil {
		return errors.New("chapter summary cannot be nil")
	}
	if strings.TrimSpace(cs.ProjectID) == "" {
		return errors.New("project_id cannot be empty")
	}
	if cs.ChapterIndex <= 0 {
		return errors.New("chapter_index must be greater than 0")
	}
	if cs.CreatedAt.IsZero() {
		cs.CreatedAt = time.Now()
	}

	if err := s.ensureSummaryTables(ctx); err != nil {
		return err
	}

	query := `
	INSERT INTO chapter_summaries (project_id, chapter_index, text, model, tokens, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		text = excluded.text,
		model = excluded.model,
		tokens = excluded.tokens,
		created_at = excluded.created_at;
	`
	if _, err := s.db.ExecContext(ctx, query,
		cs.ProjectID,
		cs.ChapterIndex,
		cs.Text,
		cs.Model,
		cs.Tokens,
		cs.CreatedAt,
	); err != nil {
		return fmt.Errorf("save chapter summary: %w", err)
	}
	return nil
}

// GetChapterSummary retrieves a chapter summary by projectID and chapterIndex.
// Returns nil, nil if the summary does not exist.
func (s *SQLiteStore) GetChapterSummary(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterSummary, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id cannot be empty")
	}
	if err := s.ensureSummaryTables(ctx); err != nil {
		return nil, err
	}

	query := `
	SELECT project_id, chapter_index, text, model, tokens, created_at
	FROM chapter_summaries
	WHERE project_id = ? AND chapter_index = ?;
	`
	row := s.db.QueryRowContext(ctx, query, projectID, chapterIndex)

	var cs domain.ChapterSummary
	if err := row.Scan(
		&cs.ProjectID,
		&cs.ChapterIndex,
		&cs.Text,
		&cs.Model,
		&cs.Tokens,
		&cs.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter summary: %w", err)
	}
	return &cs, nil
}

// ListChapterSummaries retrieves chapter summaries for a project within [from, to] chapter index range.
// Non-positive boundaries are ignored (e.g. from <= 0 means no lower bound, to <= 0 means no upper bound).
func (s *SQLiteStore) ListChapterSummaries(ctx context.Context, projectID string, from, to int) ([]*domain.ChapterSummary, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id cannot be empty")
	}
	if err := s.ensureSummaryTables(ctx); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(`SELECT project_id, chapter_index, text, model, tokens, created_at FROM chapter_summaries WHERE project_id = ?`)
	args := []any{projectID}

	if from > 0 {
		sb.WriteString(` AND chapter_index >= ?`)
		args = append(args, from)
	}
	if to > 0 {
		sb.WriteString(` AND chapter_index <= ?`)
		args = append(args, to)
	}
	sb.WriteString(` ORDER BY chapter_index ASC;`)

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list chapter summaries: %w", err)
	}
	defer rows.Close()

	list := make([]*domain.ChapterSummary, 0)
	for rows.Next() {
		var cs domain.ChapterSummary
		if err := rows.Scan(
			&cs.ProjectID,
			&cs.ChapterIndex,
			&cs.Text,
			&cs.Model,
			&cs.Tokens,
			&cs.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chapter summary row: %w", err)
		}
		list = append(list, &cs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chapter summaries: %w", err)
	}
	return list, nil
}

// SaveVolumeRetrospective persists a volume retrospective, updating existing record if present.
func (s *SQLiteStore) SaveVolumeRetrospective(ctx context.Context, retro *domain.VolumeRetrospective) error {
	if retro == nil {
		return errors.New("volume retrospective cannot be nil")
	}
	if strings.TrimSpace(retro.ProjectID) == "" {
		return errors.New("project_id cannot be empty")
	}
	if retro.VolumeIndex <= 0 {
		return errors.New("volume_index must be greater than 0")
	}
	if retro.CreatedAt.IsZero() {
		retro.CreatedAt = time.Now()
	}

	if err := s.ensureSummaryTables(ctx); err != nil {
		return err
	}

	query := `
	INSERT INTO volume_retrospectives (project_id, volume_index, text, from_chapter, to_chapter, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, volume_index) DO UPDATE SET
		text = excluded.text,
		from_chapter = excluded.from_chapter,
		to_chapter = excluded.to_chapter,
		created_at = excluded.created_at;
	`
	if _, err := s.db.ExecContext(ctx, query,
		retro.ProjectID,
		retro.VolumeIndex,
		retro.Text,
		retro.FromChapter,
		retro.ToChapter,
		retro.CreatedAt,
	); err != nil {
		return fmt.Errorf("save volume retrospective: %w", err)
	}
	return nil
}

// GetVolumeRetrospective retrieves a volume retrospective by projectID and volumeIndex.
// Returns nil, nil if the retrospective does not exist.
func (s *SQLiteStore) GetVolumeRetrospective(ctx context.Context, projectID string, volumeIndex int) (*domain.VolumeRetrospective, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id cannot be empty")
	}
	if err := s.ensureSummaryTables(ctx); err != nil {
		return nil, err
	}

	query := `
	SELECT project_id, volume_index, text, from_chapter, to_chapter, created_at
	FROM volume_retrospectives
	WHERE project_id = ? AND volume_index = ?;
	`
	row := s.db.QueryRowContext(ctx, query, projectID, volumeIndex)

	var vr domain.VolumeRetrospective
	if err := row.Scan(
		&vr.ProjectID,
		&vr.VolumeIndex,
		&vr.Text,
		&vr.FromChapter,
		&vr.ToChapter,
		&vr.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get volume retrospective: %w", err)
	}
	return &vr, nil
}
