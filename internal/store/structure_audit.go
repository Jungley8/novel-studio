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

// ensureStructureFindingsTable ensures the structure_findings table and indices exist.
func (s *SQLiteStore) ensureStructureFindingsTable(ctx context.Context) error {
	if s.db == nil {
		return errors.New("sqlite database connection is nil")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS structure_findings (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		type TEXT NOT NULL,
		severity TEXT NOT NULL,
		chapter_start INTEGER NOT NULL,
		chapter_end INTEGER NOT NULL,
		summary TEXT NOT NULL,
		evidence TEXT NOT NULL,
		suggestion TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_structure_findings_proj ON structure_findings(project_id, chapter_start);
	`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("ensure structure_findings table failed: %w", err)
	}
	return nil
}

// EnsureStructureFindingsTable is an exported wrapper for tests or manual provisioning.
func (s *SQLiteStore) EnsureStructureFindingsTable(ctx context.Context) error {
	return s.ensureStructureFindingsTable(ctx)
}

// SaveStructureFinding persists a single structure finding.
func (s *SQLiteStore) SaveStructureFinding(ctx context.Context, f *domain.StructureFinding) error {
	if f == nil {
		return errors.New("structure finding cannot be nil")
	}
	if strings.TrimSpace(f.ID) == "" {
		return errors.New("structure finding ID cannot be empty")
	}
	if strings.TrimSpace(f.ProjectID) == "" {
		return errors.New("project ID cannot be empty")
	}
	if err := f.Validate(); err != nil {
		return fmt.Errorf("invalid structure finding: %w", err)
	}

	if err := s.ensureStructureFindingsTable(ctx); err != nil {
		return err
	}

	createdAt := f.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	query := `
	INSERT OR REPLACE INTO structure_findings (
		id, project_id, type, severity, chapter_start, chapter_end,
		summary, evidence, suggestion, status, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(
		ctx,
		query,
		f.ID,
		f.ProjectID,
		string(f.Type),
		string(f.Severity),
		f.ChapterStart,
		f.ChapterEnd,
		f.Summary,
		f.Evidence,
		f.Suggestion,
		string(f.Status),
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("save structure finding failed: %w", err)
	}
	return nil
}

// ListStructureFindings retrieves all findings for a given project ordered by chapter_start, rowid.
func (s *SQLiteStore) ListStructureFindings(ctx context.Context, projectID string) ([]*domain.StructureFinding, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project ID cannot be empty")
	}

	if err := s.ensureStructureFindingsTable(ctx); err != nil {
		return nil, err
	}

	query := `
	SELECT id, project_id, type, severity, chapter_start, chapter_end,
	       summary, evidence, suggestion, status, created_at
	FROM structure_findings
	WHERE project_id = ?
	ORDER BY chapter_start ASC, rowid ASC
	`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list structure findings query failed: %w", err)
	}
	defer rows.Close()

	findings := make([]*domain.StructureFinding, 0)
	for rows.Next() {
		var f domain.StructureFinding
		var typ, sev, status string
		var createdAt time.Time

		err := rows.Scan(
			&f.ID,
			&f.ProjectID,
			&typ,
			&sev,
			&f.ChapterStart,
			&f.ChapterEnd,
			&f.Summary,
			&f.Evidence,
			&f.Suggestion,
			&status,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan structure finding row failed: %w", err)
		}
		f.Type = domain.StructureFindingType(typ)
		f.Severity = domain.StructureFindingSeverity(sev)
		f.Status = domain.StructureFindingStatus(status)
		f.CreatedAt = createdAt
		findings = append(findings, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate structure findings rows failed: %w", err)
	}

	return findings, nil
}

// UpdateStructureFindingStatus updates the status of a structure finding.
func (s *SQLiteStore) UpdateStructureFindingStatus(ctx context.Context, id, status string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("structure finding ID cannot be empty")
	}

	statusTrimmed := strings.TrimSpace(status)
	switch domain.StructureFindingStatus(statusTrimmed) {
	case domain.StructureFindingStatusOpen,
		domain.StructureFindingStatusResolved,
		domain.StructureFindingStatusIgnored:
	default:
		return errors.New("invalid structure finding status: " + statusTrimmed)
	}

	if err := s.ensureStructureFindingsTable(ctx); err != nil {
		return err
	}

	query := `UPDATE structure_findings SET status = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, statusTrimmed, id)
	if err != nil {
		return fmt.Errorf("update structure finding status failed: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected failed: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// StructureFindingProjectID returns the owning project of a finding,
// or "" when the finding does not exist (mirrors PlotHookProjectID).
func (s *SQLiteStore) StructureFindingProjectID(ctx context.Context, id string) (string, error) {
	var projectID string
	err := s.db.QueryRowContext(ctx, `SELECT project_id FROM structure_findings WHERE id = ?`, id).Scan(&projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("query structure finding project: %w", err)
	}
	return projectID, nil
}

// DeleteStructureFindingsByProject removes all structure findings for a project.
func (s *SQLiteStore) DeleteStructureFindingsByProject(ctx context.Context, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project ID cannot be empty")
	}

	if err := s.ensureStructureFindingsTable(ctx); err != nil {
		return err
	}

	query := `DELETE FROM structure_findings WHERE project_id = ?`
	if _, err := s.db.ExecContext(ctx, query, projectID); err != nil {
		return fmt.Errorf("delete structure findings by project failed: %w", err)
	}
	return nil
}
