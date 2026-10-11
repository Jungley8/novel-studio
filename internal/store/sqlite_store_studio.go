package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// --- Studio Chat Persistence Methods ---

// CreateStudioThread creates a new studio conversation thread.
func (s *SQLiteStore) CreateStudioThread(ctx context.Context, t *domain.StudioThread) error {
	if t == nil {
		return errors.New("studio thread cannot be nil")
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("th_%d", time.Now().UnixNano())
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.CreatedAt
	}

	query := `
	INSERT INTO studio_threads (id, title, project_id, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?);
	`
	if _, err := s.db.ExecContext(ctx, query, t.ID, t.Title, t.ProjectID, t.CreatedAt, t.UpdatedAt); err != nil {
		return fmt.Errorf("create studio thread: %w", err)
	}
	return nil
}

// GetStudioThread retrieves a studio thread by ID, or returns (nil, nil) if not found.
func (s *SQLiteStore) GetStudioThread(ctx context.Context, id string) (*domain.StudioThread, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, project_id, created_at, updated_at FROM studio_threads WHERE id = ?;`,
		id,
	)
	var t domain.StudioThread
	if err := row.Scan(&t.ID, &t.Title, &t.ProjectID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get studio thread: %w", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM studio_messages WHERE thread_id = ?;`, id).Scan(&count); err != nil {
		return nil, fmt.Errorf("get studio thread count: %w", err)
	}
	t.MessageCount = count

	if count > 0 {
		var lastContent string
		rowLast := s.db.QueryRowContext(ctx, `SELECT content FROM studio_messages WHERE thread_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1;`, id)
		if err := rowLast.Scan(&lastContent); err == nil {
			runes := []rune(lastContent)
			if len(runes) > 50 {
				t.LastMessagePreview = string(runes[:50]) + "…"
			} else {
				t.LastMessagePreview = lastContent
			}
		}
	}

	return &t, nil
}

// ListStudioThreads lists all studio threads ordered by updated_at DESC.
func (s *SQLiteStore) ListStudioThreads(ctx context.Context) ([]*domain.StudioThread, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, project_id, created_at, updated_at FROM studio_threads ORDER BY updated_at DESC, rowid DESC;`,
	)
	if err != nil {
		return nil, fmt.Errorf("list studio threads: %w", err)
	}
	defer rows.Close()

	threads := make([]*domain.StudioThread, 0)
	for rows.Next() {
		var t domain.StudioThread
		if err := rows.Scan(&t.ID, &t.Title, &t.ProjectID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan studio thread: %w", err)
		}
		threads = append(threads, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list studio threads rows: %w", err)
	}

	for _, t := range threads {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM studio_messages WHERE thread_id = ?;`, t.ID).Scan(&count); err != nil {
			return nil, fmt.Errorf("count studio messages for %s: %w", t.ID, err)
		}
		t.MessageCount = count

		if count > 0 {
			var lastContent string
			rowLast := s.db.QueryRowContext(ctx, `SELECT content FROM studio_messages WHERE thread_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1;`, t.ID)
			if err := rowLast.Scan(&lastContent); err == nil {
				runes := []rune(lastContent)
				if len(runes) > 50 {
					t.LastMessagePreview = string(runes[:50]) + "…"
				} else {
					t.LastMessagePreview = lastContent
				}
			}
		}
	}

	return threads, nil
}

// DeleteStudioThread removes a thread and its messages in a transaction.
// Returns an error if the thread does not exist.
func (s *SQLiteStore) DeleteStudioThread(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM studio_threads WHERE id = ?;`, id).Scan(&count); err != nil {
		return fmt.Errorf("check studio thread existence: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("studio thread not found: %s", id)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM studio_messages WHERE thread_id = ?;`, id); err != nil {
		return fmt.Errorf("delete studio messages: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM studio_threads WHERE id = ?;`, id); err != nil {
		return fmt.Errorf("delete studio thread: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete studio thread: %w", err)
	}
	committed = true
	return nil
}

// AppendStudioMessage appends a message to a thread and updates the thread's updated_at in a transaction.
func (s *SQLiteStore) AppendStudioMessage(ctx context.Context, m *domain.StudioMessage) error {
	if m == nil {
		return errors.New("studio message cannot be nil")
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
	INSERT INTO studio_messages (id, thread_id, role, content, mode, project_id, token_usage, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`
	if _, err := tx.ExecContext(ctx, query, m.ID, m.ThreadID, m.Role, m.Content, m.Mode, m.ProjectID, m.TokenUsage, m.CreatedAt); err != nil {
		return fmt.Errorf("insert studio message: %w", err)
	}

	res, err := tx.ExecContext(ctx, `UPDATE studio_threads SET updated_at = ? WHERE id = ?;`, m.CreatedAt, m.ThreadID)
	if err != nil {
		return fmt.Errorf("update studio thread updated_at: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update studio thread rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("studio thread not found: %s", m.ThreadID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit append studio message: %w", err)
	}
	committed = true
	return nil
}

// ListStudioMessages lists all messages for a thread ordered by created_at ASC.
func (s *SQLiteStore) ListStudioMessages(ctx context.Context, threadID string) ([]*domain.StudioMessage, error) {
	query := `
	SELECT id, thread_id, role, content, mode, project_id, token_usage, created_at
	FROM studio_messages
	WHERE thread_id = ?
	ORDER BY created_at ASC, rowid ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, threadID)
	if err != nil {
		return nil, fmt.Errorf("list studio messages: %w", err)
	}
	defer rows.Close()

	messages := make([]*domain.StudioMessage, 0)
	for rows.Next() {
		var m domain.StudioMessage
		if err := rows.Scan(
			&m.ID, &m.ThreadID, &m.Role, &m.Content, &m.Mode, &m.ProjectID, &m.TokenUsage, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan studio message: %w", err)
		}
		messages = append(messages, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list studio messages rows: %w", err)
	}
	return messages, nil
}
