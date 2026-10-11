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
// Automation & Book Queue (无人值守排程与新书队列)
// -------------------------------------------------------------

func (s *SQLiteStore) ListProjectAutomations(ctx context.Context) ([]*domain.ProjectAutomation, error) {
	query := `
		SELECT p.id, p.title, a.project_id, a.enabled, a.times_json, a.quota_kind,
		       a.quota_count, a.paused, a.paused_reason, a.last_run_at, a.last_status, a.last_message
		FROM projects p
		LEFT JOIN project_automation a ON a.project_id = p.id
		ORDER BY p.created_at DESC, p.id ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list project automations: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.ProjectAutomation, 0)
	for rows.Next() {
		var pID, pTitle string
		var aProjID, aTimesJSON, aQuotaKind, aPausedReason, aLastRunAt, aLastStatus, aLastMessage sql.NullString
		var aEnabled, aQuotaCount, aPaused sql.NullInt64
		if err := rows.Scan(
			&pID, &pTitle, &aProjID, &aEnabled, &aTimesJSON, &aQuotaKind,
			&aQuotaCount, &aPaused, &aPausedReason, &aLastRunAt, &aLastStatus, &aLastMessage,
		); err != nil {
			return nil, fmt.Errorf("scan project automation: %w", err)
		}

		if !aProjID.Valid {
			items = append(items, &domain.ProjectAutomation{
				ProjectID:    pID,
				ProjectTitle: pTitle,
				Enabled:      false,
				Times:        []string{},
				QuotaKind:    domain.QuotaKindChapters,
				QuotaCount:   5,
				Paused:       false,
				PausedReason: "",
				LastRunAt:    "",
				LastStatus:   "",
				LastMessage:  "",
			})
			continue
		}

		times := []string{}
		if aTimesJSON.Valid && aTimesJSON.String != "" {
			_ = json.Unmarshal([]byte(aTimesJSON.String), &times)
		}
		if times == nil {
			times = []string{}
		}

		qKind := domain.AutomationQuotaKind(aQuotaKind.String)
		if qKind == "" {
			qKind = domain.QuotaKindChapters
		}
		qCount := int(aQuotaCount.Int64)
		if qCount == 0 {
			qCount = 5
		}

		items = append(items, &domain.ProjectAutomation{
			ProjectID:    pID,
			ProjectTitle: pTitle,
			Enabled:      aEnabled.Int64 != 0,
			Times:        times,
			QuotaKind:    qKind,
			QuotaCount:   qCount,
			Paused:       aPaused.Int64 != 0,
			PausedReason: aPausedReason.String,
			LastRunAt:    aLastRunAt.String,
			LastStatus:   aLastStatus.String,
			LastMessage:  aLastMessage.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project automations: %w", err)
	}
	return items, nil
}

func (s *SQLiteStore) GetProjectAutomation(ctx context.Context, projectID string) (*domain.ProjectAutomation, error) {
	query := `
		SELECT a.project_id, COALESCE(p.title, ''), a.enabled, a.times_json, a.quota_kind,
		       a.quota_count, a.paused, a.paused_reason, a.last_run_at, a.last_status, a.last_message
		FROM project_automation a
		LEFT JOIN projects p ON a.project_id = p.id
		WHERE a.project_id = ?
	`
	var item domain.ProjectAutomation
	var enabled, quotaCount, paused int
	var timesJSON, quotaKind string
	err := s.db.QueryRowContext(ctx, query, projectID).Scan(
		&item.ProjectID, &item.ProjectTitle, &enabled, &timesJSON, &quotaKind,
		&quotaCount, &paused, &item.PausedReason, &item.LastRunAt, &item.LastStatus, &item.LastMessage,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("automation not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get project automation: %w", err)
	}

	item.Enabled = enabled != 0
	item.Paused = paused != 0
	item.QuotaKind = domain.AutomationQuotaKind(quotaKind)
	item.QuotaCount = quotaCount
	item.Times = []string{}
	if timesJSON != "" {
		_ = json.Unmarshal([]byte(timesJSON), &item.Times)
	}
	if item.Times == nil {
		item.Times = []string{}
	}
	return &item, nil
}

func (s *SQLiteStore) SaveProjectAutomation(ctx context.Context, a *domain.ProjectAutomation) error {
	if a == nil {
		return errors.New("nil automation")
	}
	if err := domain.ValidateAutomation(a); err != nil {
		return err
	}

	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM projects WHERE id = ?", a.ProjectID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("project %q not found: %w", a.ProjectID, ErrNotFound)
		}
		return fmt.Errorf("check project exists: %w", err)
	}

	timesJSON, err := json.Marshal(a.Times)
	if err != nil {
		return fmt.Errorf("marshal times: %w", err)
	}
	if string(timesJSON) == "null" {
		timesJSON = []byte("[]")
	}

	now := time.Now().Format(time.RFC3339)
	enabledVal := 0
	if a.Enabled {
		enabledVal = 1
	}

	query := `
		INSERT INTO project_automation (
			project_id, enabled, times_json, quota_kind, quota_count,
			paused, paused_reason, last_run_at, last_status, last_message,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 0, '', '', '', '', ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			enabled = excluded.enabled,
			times_json = excluded.times_json,
			quota_kind = excluded.quota_kind,
			quota_count = excluded.quota_count,
			updated_at = excluded.updated_at
	`
	_, err = s.db.ExecContext(ctx, query,
		a.ProjectID, enabledVal, string(timesJSON), string(a.QuotaKind), a.QuotaCount,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("save project automation: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SetAutomationPaused(ctx context.Context, projectID string, paused bool, reason string) error {
	now := time.Now().Format(time.RFC3339)
	pausedVal := 0
	if paused {
		pausedVal = 1
	}
	query := `
		UPDATE project_automation
		SET paused = ?, paused_reason = ?, updated_at = ?
		WHERE project_id = ?
	`
	res, err := s.db.ExecContext(ctx, query, pausedVal, reason, now, projectID)
	if err != nil {
		return fmt.Errorf("set automation paused: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set automation paused rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("automation for project %q: %w", projectID, ErrNotFound)
	}
	return nil
}

func (s *SQLiteStore) RecordAutomationRun(ctx context.Context, projectID, status, message string, at time.Time) error {
	now := time.Now().Format(time.RFC3339)
	atStr := at.Format(time.RFC3339)
	query := `
		UPDATE project_automation
		SET last_run_at = ?, last_status = ?, last_message = ?, updated_at = ?
		WHERE project_id = ?
	`
	res, err := s.db.ExecContext(ctx, query, atStr, status, message, now, projectID)
	if err != nil {
		return fmt.Errorf("record automation run: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("record automation run rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("automation for project %q: %w", projectID, ErrNotFound)
	}
	return nil
}

func (s *SQLiteStore) ListBookQueue(ctx context.Context) ([]*domain.BookQueueItem, error) {
	query := `
		SELECT id, title, concept, platform, volumes, auto_lock, status, project_id, last_message, created_at, updated_at
		FROM book_queue
		ORDER BY created_at ASC, id ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list book queue: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.BookQueueItem, 0)
	for rows.Next() {
		var item domain.BookQueueItem
		var autoLock int
		if err := rows.Scan(
			&item.ID, &item.Title, &item.Concept, &item.Platform, &item.Volumes,
			&autoLock, &item.Status, &item.ProjectID, &item.LastMessage, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan book queue item: %w", err)
		}
		item.AutoLock = autoLock != 0
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate book queue: %w", err)
	}
	return items, nil
}

func (s *SQLiteStore) GetBookQueueItem(ctx context.Context, id string) (*domain.BookQueueItem, error) {
	query := `
		SELECT id, title, concept, platform, volumes, auto_lock, status, project_id, last_message, created_at, updated_at
		FROM book_queue
		WHERE id = ?
	`
	var item domain.BookQueueItem
	var autoLock int
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.Title, &item.Concept, &item.Platform, &item.Volumes,
		&autoLock, &item.Status, &item.ProjectID, &item.LastMessage, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("book queue item %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get book queue item: %w", err)
	}
	item.AutoLock = autoLock != 0
	return &item, nil
}

func (s *SQLiteStore) SaveBookQueueItem(ctx context.Context, item *domain.BookQueueItem) error {
	if item == nil {
		return errors.New("nil item")
	}
	if err := domain.ValidateBookQueueItem(item); err != nil {
		return err
	}
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("book queue item id cannot be empty")
	}

	status := item.Status
	if status == "" {
		status = domain.QueueStatusPending
	}
	autoLockVal := 0
	if item.AutoLock {
		autoLockVal = 1
	}
	now := time.Now().Format(time.RFC3339)

	query := `
		INSERT INTO book_queue (
			id, title, concept, platform, volumes, auto_lock,
			status, project_id, last_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			concept = excluded.concept,
			platform = excluded.platform,
			volumes = excluded.volumes,
			auto_lock = excluded.auto_lock,
			status = excluded.status,
			project_id = excluded.project_id,
			last_message = excluded.last_message,
			updated_at = excluded.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		item.ID, item.Title, item.Concept, item.Platform, item.Volumes, autoLockVal,
		status, item.ProjectID, item.LastMessage, now, now,
	)
	if err != nil {
		return fmt.Errorf("save book queue item: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteBookQueueItem(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM book_queue WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete book queue item: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete book queue item rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("book queue item %q: %w", id, ErrNotFound)
	}
	return nil
}

func (s *SQLiteStore) ResetStaleQueueRunning(ctx context.Context) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	query := `UPDATE book_queue SET status = 'pending', last_message = '服务重启，运行中的任务已重新排队', updated_at = ? WHERE status = 'running'`
	res, err := s.db.ExecContext(ctx, query, now)
	if err != nil {
		return 0, fmt.Errorf("reset stale queue running: %w", err)
	}
	return res.RowsAffected()
}

func (s *SQLiteStore) GetSchedulerState(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM scheduler_state WHERE key = ?", key).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get scheduler state: %w", err)
	}
	return val, nil
}

func (s *SQLiteStore) SetSchedulerState(ctx context.Context, key, value string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
		INSERT INTO scheduler_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`
	_, err := s.db.ExecContext(ctx, query, key, value, now)
	if err != nil {
		return fmt.Errorf("set scheduler state: %w", err)
	}
	return nil
}
