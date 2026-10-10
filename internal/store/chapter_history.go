// Package store - chapter_history.go
// 本模块是未来"非末章撤回→截断重放"的前置基建；已存在历史项目不会有快照回填（不提供回填），只对本次合入后的写入生效。
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// insertChapterVersionTx persists a new chapter version within an existing transaction.
// If the latest version for the same (projectID, chapterIndex) already has identical content_hash,
// it is skipped to avoid redundant noise rows.
func (s *SQLiteStore) insertChapterVersionTx(tx *sql.Tx, projectID string, c *domain.Chapter, source string) error {
	sum := sha256.Sum256([]byte(c.Content))
	contentHash := hex.EncodeToString(sum[:])

	var latestHash string
	err := tx.QueryRow(`
		SELECT content_hash FROM chapter_versions
		WHERE project_id = ? AND chapter_index = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, projectID, c.ChapterIndex).Scan(&latestHash)
	if err == nil {
		if latestHash == contentHash {
			return nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check latest chapter version: %w", err)
	}

	verID := fmt.Sprintf("ver_%d_%d_%d", time.Now().UnixNano(), c.ChapterIndex, rand.Intn(10000))
	wordCount := c.WordCount
	if wordCount <= 0 && len(c.Content) > 0 {
		wordCount = len([]rune(c.Content))
	}
	createdAt := time.Now()

	_, err = tx.Exec(`
		INSERT INTO chapter_versions (
			id, project_id, chapter_index, source, word_count, content_hash, content, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, verID, projectID, c.ChapterIndex, source, wordCount, contentHash, c.Content, createdAt)
	if err != nil {
		return fmt.Errorf("insert chapter version: %w", err)
	}
	return nil
}

// insertStateSnapshotTx records a complete world state and hooks snapshot at chapterIndex within an existing transaction.
// If a snapshot already exists for the (projectID, chapterIndex), it is overwritten with the latest state.
func (s *SQLiteStore) insertStateSnapshotTx(ctx context.Context, tx *sql.Tx, projectID string, chapterIndex int) error {
	protagonist, err := s.getProtagonistTx(ctx, tx, projectID)
	if err != nil {
		return fmt.Errorf("get protagonist in tx for snapshot: %w", err)
	}
	protagonistJSON := "{}"
	if protagonist != nil {
		pb, err := json.Marshal(protagonist)
		if err != nil {
			return fmt.Errorf("marshal protagonist for snapshot: %w", err)
		}
		protagonistJSON = string(pb)
	}

	queryHooks := `
	SELECT id, project_id, title, details, created_chapter, target_chapter, status, created_at
	FROM plot_hooks WHERE project_id = ? ORDER BY created_chapter ASC, target_chapter ASC;
	`
	rows, err := tx.QueryContext(ctx, queryHooks, projectID)
	if err != nil {
		return fmt.Errorf("query plot hooks in tx for snapshot: %w", err)
	}
	defer rows.Close()

	hooks := make([]*domain.PlotHook, 0)
	for rows.Next() {
		var h domain.PlotHook
		var statusStr string
		if err := rows.Scan(
			&h.ID, &h.ProjectID, &h.Title, &h.Details, &h.CreatedChapter, &h.TargetChapter, &statusStr, &h.CreatedAt,
		); err != nil {
			return fmt.Errorf("scan plot hook in tx for snapshot: %w", err)
		}
		h.Status = domain.HookStatus(statusStr)
		hooks = append(hooks, &h)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate plot hooks in tx for snapshot: %w", err)
	}

	hooksBytes, err := json.Marshal(hooks)
	if err != nil {
		return fmt.Errorf("marshal plot hooks for snapshot: %w", err)
	}
	hooksJSON := string(hooksBytes)

	snapID := fmt.Sprintf("snap_%d_%d_%d", time.Now().UnixNano(), chapterIndex, rand.Intn(10000))
	createdAt := time.Now()

	insertQuery := `
	INSERT INTO chapter_state_snapshots (
		id, project_id, chapter_index, protagonist_json, hooks_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		protagonist_json = excluded.protagonist_json,
		hooks_json = excluded.hooks_json,
		created_at = excluded.created_at;
	`
	if _, err := tx.ExecContext(ctx, insertQuery, snapID, projectID, chapterIndex, protagonistJSON, hooksJSON, createdAt); err != nil {
		return fmt.Errorf("insert chapter state snapshot in tx: %w", err)
	}
	return nil
}

// ListChapterVersions returns metadata for all versions of a chapter ordered by created_at descending (without content).
func (s *SQLiteStore) ListChapterVersions(ctx context.Context, projectID string, chapterIndex int) ([]*domain.ChapterVersionMeta, error) {
	query := `
	SELECT id, project_id, chapter_index, source, word_count, content_hash, created_at
	FROM chapter_versions
	WHERE project_id = ? AND chapter_index = ?
	ORDER BY created_at DESC;
	`
	rows, err := s.db.QueryContext(ctx, query, projectID, chapterIndex)
	if err != nil {
		return nil, fmt.Errorf("list chapter versions: %w", err)
	}
	defer rows.Close()

	list := make([]*domain.ChapterVersionMeta, 0)
	for rows.Next() {
		var m domain.ChapterVersionMeta
		if err := rows.Scan(
			&m.ID, &m.ProjectID, &m.ChapterIndex, &m.Source, &m.WordCount, &m.ContentHash, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chapter version meta: %w", err)
		}
		list = append(list, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chapter versions: %w", err)
	}
	return list, nil
}

// GetChapterVersion returns a single chapter version including full prose content. Returns (nil, nil) if not found.
func (s *SQLiteStore) GetChapterVersion(ctx context.Context, id string) (*domain.ChapterVersion, error) {
	query := `
	SELECT id, project_id, chapter_index, source, word_count, content_hash, content, created_at
	FROM chapter_versions
	WHERE id = ?;
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var v domain.ChapterVersion
	if err := row.Scan(
		&v.ID, &v.ProjectID, &v.ChapterIndex, &v.Source, &v.WordCount, &v.ContentHash, &v.Content, &v.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter version: %w", err)
	}
	return &v, nil
}

// GetChapterStateSnapshot retrieves the post-commit state snapshot for a given chapter. Returns (nil, nil) if not found.
func (s *SQLiteStore) GetChapterStateSnapshot(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterStateSnapshot, error) {
	query := `
	SELECT id, project_id, chapter_index, protagonist_json, hooks_json, created_at
	FROM chapter_state_snapshots
	WHERE project_id = ? AND chapter_index = ?;
	`
	row := s.db.QueryRowContext(ctx, query, projectID, chapterIndex)

	var snap domain.ChapterStateSnapshot
	if err := row.Scan(
		&snap.ID, &snap.ProjectID, &snap.ChapterIndex, &snap.ProtagonistJSON, &snap.HooksJSON, &snap.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter state snapshot: %w", err)
	}
	return &snap, nil
}
