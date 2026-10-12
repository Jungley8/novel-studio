package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
)

// PlotHookProjectID 返回指定伏笔实际归属的作品 ID；伏笔不存在时返回空字符串。
// 用于请求侧 Fail-Closed 校验：禁止用其它作品的伏笔 ID 覆盖其数据。
func (s *SQLiteStore) PlotHookProjectID(ctx context.Context, id string) (string, error) {
	var projectID string
	err := s.db.QueryRowContext(ctx, `SELECT project_id FROM plot_hooks WHERE id = ?`, id).Scan(&projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("query plot hook project: %w", err)
	}
	return projectID, nil
}

func (s *SQLiteStore) SavePlotHook(ctx context.Context, h *domain.PlotHook) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if h.ProjectID == "" {
		return errors.New("hook requires project_id")
	}
	if h.ID == "" {
		return errors.New("hook ID cannot be empty")
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}

	// Before snapshot to detect creation vs field/status transitions.
	var beforeJSON, beforeStatus string
	orow := s.db.QueryRowContext(ctx, `SELECT title, details, created_chapter, target_chapter, status FROM plot_hooks WHERE id = ?`, h.ID)
	var ot, od, os string
	var oc, otc int
	existed := false
	if oerr := orow.Scan(&ot, &od, &oc, &otc, &os); oerr == nil {
		existed = true
		beforeStatus = os
		if ob, merr := json.Marshal(map[string]any{
			"id": h.ID, "title": ot, "details": od, "created_chapter": oc,
			"target_chapter": otc, "status": os,
		}); merr == nil {
			beforeJSON = string(ob)
		}
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
	INSERT INTO plot_hooks (id, project_id, title, details, created_chapter, target_chapter, status, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		details = excluded.details,
		created_chapter = excluded.created_chapter,
		target_chapter = excluded.target_chapter,
		status = excluded.status;
	`
	if _, err := tx.ExecContext(ctx, query,
		h.ID, h.ProjectID, h.Title, h.Details, h.CreatedChapter, h.TargetChapter, string(h.Status), h.CreatedAt,
	); err != nil {
		return err
	}

	afterBytes, _ := json.Marshal(h)
	eventType := activity.EventHookUpdated
	title := fmt.Sprintf("伏笔「%s」已更新", h.Title)
	var fieldsChanged []string
	if !existed {
		eventType = activity.EventHookCreated
		title = fmt.Sprintf("埋设伏笔「%s」（目标第 %d 章）", h.Title, h.TargetChapter)
	} else {
		if ot != h.Title || od != h.Details || oc != h.CreatedChapter || otc != h.TargetChapter {
			fieldsChanged = append(fieldsChanged, "content")
		}
		if beforeStatus != string(h.Status) {
			fieldsChanged = append(fieldsChanged, "status")
			switch h.Status {
			case domain.HookStatusResolved:
				eventType = activity.EventHookResolved
				title = fmt.Sprintf("伏笔「%s」已回收", h.Title)
			case domain.HookStatusOpen:
				eventType = activity.EventHookReopened
				title = fmt.Sprintf("伏笔「%s」已重开", h.Title)
			default:
				title = fmt.Sprintf("伏笔「%s」状态→%s", h.Title, h.Status)
			}
		}
	}
	e := newEvent(ctx, h.ProjectID, activity.EntityHook, h.ID, 0, eventType, title, h.Details)
	withTextSnapshots(e, beforeJSON, string(afterBytes), fieldsChanged)
	if err := insertEvent(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteStore) ListPlotHooks(ctx context.Context, projectID string) ([]*domain.PlotHook, error) {
	query := `
	SELECT id, project_id, title, details, created_chapter, target_chapter, status, created_at
	FROM plot_hooks WHERE project_id = ? ORDER BY created_chapter ASC, target_chapter ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.PlotHook
	for rows.Next() {
		var h domain.PlotHook
		var statusStr string
		if err := rows.Scan(
			&h.ID, &h.ProjectID, &h.Title, &h.Details, &h.CreatedChapter, &h.TargetChapter, &statusStr, &h.CreatedAt,
		); err != nil {
			return nil, err
		}
		h.Status = domain.HookStatus(statusStr)
		list = append(list, &h)
	}
	return list, rows.Err()
}

func (s *SQLiteStore) DeletePlotHook(ctx context.Context, id string) error {
	var beforeJSON, projectID, hookTitle string
	orow := s.db.QueryRowContext(ctx, `SELECT project_id, title, details, status FROM plot_hooks WHERE id = ?`, id)
	var details, status string
	if oerr := orow.Scan(&projectID, &hookTitle, &details, &status); oerr == nil {
		if ob, merr := json.Marshal(map[string]any{
			"id": id, "title": hookTitle, "details": details, "status": status,
		}); merr == nil {
			beforeJSON = string(ob)
		}
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

	if _, err := tx.ExecContext(ctx, `DELETE FROM plot_hooks WHERE id = ?`, id); err != nil {
		return err
	}

	if projectID != "" {
		e := newEvent(ctx, projectID, activity.EntityHook, id, 0, activity.EventHookDeleted,
			fmt.Sprintf("伏笔「%s」已删除", hookTitle), details)
		withTextSnapshots(e, beforeJSON, "", []string{"deleted"})
		if err := insertEvent(ctx, tx, e); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}
