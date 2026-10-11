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

// -------------------------------------------------------------
// Chapter Intents (章节意图卡) 存储实现
// -------------------------------------------------------------

func (s *SQLiteStore) SaveChapterIntent(ctx context.Context, intent *domain.ChapterIntent) error {
	if intent == nil {
		return errors.New("chapter intent cannot be nil")
	}
	if strings.TrimSpace(intent.ProjectID) == "" {
		return errors.New("chapter intent project_id cannot be empty")
	}
	if err := intent.Validate(); err != nil {
		return fmt.Errorf("validate chapter intent: %w", err)
	}

	intent.UpdatedAt = time.Now()
	payload, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("marshal chapter intent: %w", err)
	}

	query := `
	INSERT INTO chapter_intents (project_id, chapter_index, volume_index, intent_json, updated_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		volume_index = excluded.volume_index,
		intent_json = excluded.intent_json,
		updated_at = excluded.updated_at;
	`
	if _, err := s.db.ExecContext(ctx, query, intent.ProjectID, intent.ChapterIndex, intent.VolumeIndex, string(payload), intent.UpdatedAt); err != nil {
		return fmt.Errorf("save chapter intent: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetChapterIntent(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterIntent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT intent_json FROM chapter_intents WHERE project_id = ? AND chapter_index = ?;`,
		projectID, chapterIndex,
	)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // 不存在返回 (nil, nil)
		}
		return nil, fmt.Errorf("get chapter intent: %w", err)
	}
	var intent domain.ChapterIntent
	if err := json.Unmarshal([]byte(payload), &intent); err != nil {
		return nil, fmt.Errorf("unmarshal chapter intent: %w", err)
	}
	return &intent, nil
}

func (s *SQLiteStore) ListChapterIntents(ctx context.Context, projectID string, volumeIndex int) ([]*domain.ChapterIntent, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if volumeIndex <= 0 {
		query := `SELECT intent_json FROM chapter_intents WHERE project_id = ? ORDER BY chapter_index ASC;`
		rows, err = s.db.QueryContext(ctx, query, projectID)
	} else {
		query := `SELECT intent_json FROM chapter_intents WHERE project_id = ? AND volume_index = ? ORDER BY chapter_index ASC;`
		rows, err = s.db.QueryContext(ctx, query, projectID, volumeIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("query chapter intents: %w", err)
	}
	defer rows.Close()

	list := make([]*domain.ChapterIntent, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("scan chapter intent: %w", err)
		}
		var intent domain.ChapterIntent
		if err := json.Unmarshal([]byte(payload), &intent); err != nil {
			return nil, fmt.Errorf("unmarshal chapter intent: %w", err)
		}
		list = append(list, &intent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows chapter intents: %w", err)
	}
	return list, nil
}

func (s *SQLiteStore) DeleteChapterIntent(ctx context.Context, projectID string, chapterIndex int) error {
	query := `DELETE FROM chapter_intents WHERE project_id = ? AND chapter_index = ?;`
	if _, err := s.db.ExecContext(ctx, query, projectID, chapterIndex); err != nil {
		return fmt.Errorf("delete chapter intent: %w", err)
	}
	return nil
}

func (s *SQLiteStore) MarkIntentsStale(ctx context.Context, projectID string, volumeIndex int) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `SELECT chapter_index, intent_json FROM chapter_intents WHERE project_id = ? AND volume_index = ?;`
	rows, err := tx.QueryContext(ctx, query, projectID, volumeIndex)
	if err != nil {
		return 0, fmt.Errorf("query intents for stale mark: %w", err)
	}

	type intentRow struct {
		chapterIndex int
		payload      string
	}
	var rowsList []intentRow
	for rows.Next() {
		var r intentRow
		if err := rows.Scan(&r.chapterIndex, &r.payload); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan intent row: %w", err)
		}
		rowsList = append(rowsList, r)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows error: %w", err)
	}

	var changed int64
	for _, r := range rowsList {
		var intent domain.ChapterIntent
		if err := json.Unmarshal([]byte(r.payload), &intent); err != nil {
			return 0, fmt.Errorf("unmarshal intent for chapter %d: %w", r.chapterIndex, err)
		}
		if intent.Status == domain.IntentStatusLocked {
			intent.Status = domain.IntentStatusStale
			intent.UpdatedAt = time.Now()
			newPayload, err := json.Marshal(&intent)
			if err != nil {
				return 0, fmt.Errorf("marshal updated intent: %w", err)
			}
			_, err = tx.ExecContext(ctx,
				`UPDATE chapter_intents SET intent_json = ?, updated_at = ? WHERE project_id = ? AND chapter_index = ?;`,
				string(newPayload), intent.UpdatedAt, projectID, r.chapterIndex,
			)
			if err != nil {
				return 0, fmt.Errorf("update stale intent: %w", err)
			}
			changed++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return changed, nil
}
