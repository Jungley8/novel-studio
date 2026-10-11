package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetCompactionCache reads a compaction cache entry by projectID and cacheKey.
// Returns ("", nil) if the entry is not found.
func (s *SQLiteStore) GetCompactionCache(ctx context.Context, projectID, cacheKey string) (string, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT compiled_text FROM compaction_cache WHERE project_id = ? AND cache_key = ?;`,
		projectID, cacheKey,
	)
	var compiledText string
	if err := row.Scan(&compiledText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get compaction cache: %w", err)
	}
	return compiledText, nil
}

// PutCompactionCache saves or updates a compaction cache entry using upsert.
func (s *SQLiteStore) PutCompactionCache(ctx context.Context, projectID, cacheKey, compiledText string) error {
	now := time.Now()
	query := `
	INSERT INTO compaction_cache (project_id, cache_key, compiled_text, created_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(project_id, cache_key) DO UPDATE SET
		compiled_text = excluded.compiled_text,
		created_at = excluded.created_at;
	`
	if _, err := s.db.ExecContext(ctx, query, projectID, cacheKey, compiledText, now); err != nil {
		return fmt.Errorf("put compaction cache: %w", err)
	}
	return nil
}
