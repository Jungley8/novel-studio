package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
)

func (s *SQLiteStore) SaveCheckpoint(ctx context.Context, cp *domain.ChapterCheckpoint) error {
	if cp == nil || cp.ProjectID == "" || cp.ChapterIndex <= 0 {
		return errors.New("invalid checkpoint: project_id and positive chapter_index required")
	}
	cp.UpdatedAt = time.Now()
	payload, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal checkpoint payload: %w", err)
	}

	var beforePayload string
	orow := s.db.QueryRowContext(ctx, `SELECT payload_json FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`, cp.ProjectID, cp.ChapterIndex)
	_ = orow.Scan(&beforePayload)

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
	INSERT INTO chapter_checkpoints (project_id, chapter_index, phase, core_conflict, payload_json, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		phase = excluded.phase,
		core_conflict = excluded.core_conflict,
		payload_json = excluded.payload_json,
		updated_at = excluded.updated_at;
	`
	if _, err = tx.ExecContext(ctx, query, cp.ProjectID, cp.ChapterIndex, string(cp.Phase), cp.CoreConflict, string(payload), cp.UpdatedAt); err != nil {
		return err
	}

	title, summary := checkpointEventText(cp)
	e := newEvent(ctx, cp.ProjectID, activity.EntityCheckpoint,
		fmt.Sprintf("cp_%s_%d", cp.ProjectID, cp.ChapterIndex), cp.ChapterIndex,
		activity.EventCheckpointSaved, title, summary)
	withTextSnapshots(e, beforePayload, string(payload), []string{string(cp.Phase)})
	e.ExcerptBefore = activity.Excerpt(checkpointDraftOf(beforePayload), activity.ExcerptRunes)
	e.ExcerptAfter = activity.Excerpt(cp.DraftText, activity.ExcerptRunes)
	if err := insertEvent(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

// checkpointEventText renders a human-readable timeline title/summary so
// intermediate states (score, rewrite loops, draft size) are visible
// without opening the detail view.
func checkpointEventText(cp *domain.ChapterCheckpoint) (title, summary string) {
	draftWords := len([]rune(cp.DraftText))
	switch cp.Phase {
	case domain.CheckpointPhaseBeats:
		title = fmt.Sprintf("第 %d 章节拍已推演（%d 拍）", cp.ChapterIndex, len(cp.Beats))
		summary = cp.CoreConflict
	case domain.CheckpointPhaseDrafted:
		title = fmt.Sprintf("第 %d 章初稿已渲染（%d 字）", cp.ChapterIndex, draftWords)
		summary = cp.CoreConflict
	case domain.CheckpointPhaseAudited:
		title = fmt.Sprintf("第 %d 章已审校", cp.ChapterIndex)
		summary = cp.CoreConflict
		if cp.AuditReport != nil {
			title = fmt.Sprintf("第 %d 章已审校（%s %d 分）", cp.ChapterIndex, cp.AuditReport.Verdict, cp.AuditReport.Score)
			if len(cp.AuditReport.Issues) > 0 {
				summary = strings.Join(cp.AuditReport.Issues, "；")
			}
		}
	case domain.CheckpointPhaseRewriting:
		title = fmt.Sprintf("第 %d 章精修第 %d 轮（%d 字）", cp.ChapterIndex, cp.RewriteLoops, draftWords)
		if cp.AuditReport != nil {
			summary = fmt.Sprintf("质检 %s %d 分", cp.AuditReport.Verdict, cp.AuditReport.Score)
		} else {
			summary = cp.CoreConflict
		}
	default:
		title = fmt.Sprintf("第 %d 章断点已保存", cp.ChapterIndex)
		summary = cp.CoreConflict
	}
	return title, summary
}

// checkpointDraftOf extracts the draft text from a stored checkpoint payload
// for excerpt purposes; garbage in yields empty out, never an error.
func checkpointDraftOf(payloadJSON string) string {
	if strings.TrimSpace(payloadJSON) == "" {
		return ""
	}
	var cp domain.ChapterCheckpoint
	if err := json.Unmarshal([]byte(payloadJSON), &cp); err != nil {
		return ""
	}
	return cp.DraftText
}

func (s *SQLiteStore) GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT payload_json FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`,
		projectID, chapterIndex,
	)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No checkpoint exists
		}
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}
	var cp domain.ChapterCheckpoint
	if err := json.Unmarshal([]byte(payload), &cp); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}
	return &cp, nil
}

func (s *SQLiteStore) ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error {
	// Silent when nothing exists: clean-run clears must not pollute the timeline.
	var existed bool
	var phase, conflict string
	orow := s.db.QueryRowContext(ctx, `SELECT phase, core_conflict FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`, projectID, chapterIndex)
	if oerr := orow.Scan(&phase, &conflict); oerr == nil {
		existed = true
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

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`,
		projectID, chapterIndex,
	); err != nil {
		return err
	}

	if existed {
		e := newEvent(ctx, projectID, activity.EntityCheckpoint,
			fmt.Sprintf("cp_%s_%d", projectID, chapterIndex), chapterIndex,
			activity.EventCheckpointCleared, fmt.Sprintf("第 %d 章断点已清除", chapterIndex),
			fmt.Sprintf("清除阶段 %s · %s", phase, conflict))
		withTextSnapshots(e, phase, "", []string{"cleared"})
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

// GetLatestCheckpoint 返回指定作品最近更新的在途断点；不存在时返回 (nil, nil)。
// 切换作品时用于恢复“最近正在推进的一章”的草稿（含已归档章节的重修稿）。
func (s *SQLiteStore) GetLatestCheckpoint(ctx context.Context, projectID string) (*domain.ChapterCheckpoint, error) {
	var payload string
	err := s.db.QueryRowContext(ctx,
		`SELECT payload_json FROM chapter_checkpoints WHERE project_id = ? ORDER BY updated_at DESC LIMIT 1`,
		projectID).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest checkpoint: %w", err)
	}
	var cp domain.ChapterCheckpoint
	if err := json.Unmarshal([]byte(payload), &cp); err != nil {
		return nil, fmt.Errorf("unmarshal latest checkpoint: %w", err)
	}
	return &cp, nil
}

// ListNeedsHumanCheckpoints 返回全部带 NeedsHuman 审校标记的断点（跨项目，按 updated_at 降序）。
func (s *SQLiteStore) ListNeedsHumanCheckpoints(ctx context.Context) ([]*domain.ChapterCheckpoint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload_json, updated_at FROM chapter_checkpoints ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list needs human checkpoints: %w", err)
	}
	defer rows.Close()

	var result []*domain.ChapterCheckpoint
	for rows.Next() {
		var payload string
		var updatedAt time.Time
		if err := rows.Scan(&payload, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan checkpoint row: %w", err)
		}
		var cp domain.ChapterCheckpoint
		if err := json.Unmarshal([]byte(payload), &cp); err != nil {
			return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = updatedAt
		}
		if cp.AuditReport != nil && cp.AuditReport.NeedsHuman {
			result = append(result, &cp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("checkpoint rows: %w", err)
	}
	return result, nil
}
