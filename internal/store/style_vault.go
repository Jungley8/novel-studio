package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// ensureStyleVaultTable lazily provisions the style_vault table and index.
// It is idempotent and called before any read/write operation.
func (s *SQLiteStore) ensureStyleVaultTable(ctx context.Context) error {
	if s.db == nil {
		return errors.New("sqlite database connection is nil")
	}
	schema := `
	CREATE TABLE IF NOT EXISTS style_vault (
		project_id TEXT PRIMARY KEY,
		golden_excerpts TEXT NOT NULL DEFAULT '[]',
		updated_at TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_style_vault_project_id ON style_vault(project_id);
	`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("ensure style_vault table failed: %w", err)
	}
	return nil
}

// truncateExcerpts ensures every golden excerpt is at most 500 runes.
func truncateExcerpts(excerpts []string) []string {
	if excerpts == nil {
		return []string{}
	}
	out := make([]string, len(excerpts))
	for i, e := range excerpts {
		runes := []rune(e)
		if len(runes) > 500 {
			out[i] = string(runes[:500])
		} else {
			out[i] = e
		}
	}
	return out
}

// GetStyleVault retrieves the style vault for a given projectID.
// Returns nil, nil if the record does not exist.
func (s *SQLiteStore) GetStyleVault(ctx context.Context, projectID string) (*domain.StyleVault, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id cannot be empty")
	}
	if err := s.ensureStyleVaultTable(ctx); err != nil {
		return nil, err
	}

	query := `SELECT project_id, golden_excerpts, updated_at FROM style_vault WHERE project_id = ?;`
	row := s.db.QueryRowContext(ctx, query, projectID)

	var pID, excerptsJSON string
	var updatedAt time.Time
	if err := row.Scan(&pID, &excerptsJSON, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get style vault: %w", err)
	}

	var excerpts []string
	if excerptsJSON != "" {
		if err := json.Unmarshal([]byte(excerptsJSON), &excerpts); err != nil {
			return nil, fmt.Errorf("unmarshal golden excerpts: %w", err)
		}
	}
	if excerpts == nil {
		excerpts = []string{}
	}

	return &domain.StyleVault{
		ProjectID:      pID,
		GoldenExcerpts: excerpts,
		UpdatedAt:      updatedAt,
	}, nil
}

// SaveStyleVault creates or updates the style vault record. Excerpts are truncated to 500 runes.
func (s *SQLiteStore) SaveStyleVault(ctx context.Context, vault *domain.StyleVault) error {
	if vault == nil {
		return errors.New("style vault cannot be nil")
	}
	if strings.TrimSpace(vault.ProjectID) == "" {
		return errors.New("project_id cannot be empty")
	}

	vault.GoldenExcerpts = truncateExcerpts(vault.GoldenExcerpts)
	if vault.UpdatedAt.IsZero() {
		vault.UpdatedAt = time.Now().UTC()
	}

	data, err := json.Marshal(vault.GoldenExcerpts)
	if err != nil {
		return fmt.Errorf("marshal golden excerpts: %w", err)
	}

	if err := s.ensureStyleVaultTable(ctx); err != nil {
		return err
	}

	query := `
	INSERT INTO style_vault (project_id, golden_excerpts, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(project_id) DO UPDATE SET
		golden_excerpts = excluded.golden_excerpts,
		updated_at = excluded.updated_at;
	`
	if _, err := s.db.ExecContext(ctx, query, vault.ProjectID, string(data), vault.UpdatedAt); err != nil {
		return fmt.Errorf("save style vault: %w", err)
	}
	return nil
}

// SetGoldenExcerpts sets the golden excerpts for a project, truncating each excerpt to 500 runes.
func (s *SQLiteStore) SetGoldenExcerpts(ctx context.Context, projectID string, excerpts []string) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project_id cannot be empty")
	}
	return s.SaveStyleVault(ctx, &domain.StyleVault{
		ProjectID:      projectID,
		GoldenExcerpts: excerpts,
		UpdatedAt:      time.Now().UTC(),
	})
}
