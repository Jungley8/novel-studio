package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// compactionCacheMaxRowsPerProject 每项目保留的缓存条数上限。
// upsert 会刷新 created_at，热条目自然留存；超出部分按 created_at DESC, cache_key DESC 裁剪（确定性）。
// 语义说明：缓存键含 dropped 指纹+剩余预算，旧条目几乎不会再命中，裁剪只损失理论命中，不影响正确性。
const compactionCacheMaxRowsPerProject = 500

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

	pruneQuery := `
	DELETE FROM compaction_cache
	WHERE project_id = ?
	  AND cache_key NOT IN (
		SELECT cache_key FROM compaction_cache
		WHERE project_id = ?
		ORDER BY created_at DESC, cache_key DESC
		LIMIT ?
	  );
	`
	if _, err := s.db.ExecContext(ctx, pruneQuery, projectID, projectID, compactionCacheMaxRowsPerProject); err != nil {
		return fmt.Errorf("prune compaction cache: %w", err)
	}
	return nil
}
