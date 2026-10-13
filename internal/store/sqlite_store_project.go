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

func (s *SQLiteStore) SaveProject(ctx context.Context, p *domain.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.ID == "" {
		return errors.New("project ID cannot be empty")
	}

	// Before snapshot for the ledger (best effort: absent means creation).
	var beforeJSON, beforeFrameworkJSON string
	existed := false
	if old, oerr := s.GetProject(ctx, p.ID); oerr == nil && old != nil {
		existed = true
		if ob, merr := json.Marshal(old); merr == nil {
			beforeJSON = string(ob)
		}
		if old.Framework != nil {
			if fb, merr := json.Marshal(old.Framework); merr == nil {
				beforeFrameworkJSON = string(fb)
			}
		}
	}

	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	var frameworkJSON string
	if p.Framework != nil {
		if fb, ferr := json.Marshal(p.Framework); ferr == nil {
			frameworkJSON = string(fb)
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
	INSERT INTO projects (id, title, target_platform, world_rules, framework_json, default_words_target, default_narrative_style, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		target_platform = excluded.target_platform,
		world_rules = excluded.world_rules,
		framework_json = excluded.framework_json,
		default_words_target = excluded.default_words_target,
		default_narrative_style = excluded.default_narrative_style,
		updated_at = excluded.updated_at;
	`
	if _, err = tx.ExecContext(ctx, query,
		p.ID, p.Title, p.TargetPlatform, p.WorldRules, frameworkJSON, p.DefaultWordsTarget, p.DefaultNarrativeStyle, p.CreatedAt, p.UpdatedAt,
	); err != nil {
		return err
	}

	afterBytes, _ := json.Marshal(p)
	afterJSON := string(afterBytes)
	eventType := activity.EventProjectUpdated
	title := fmt.Sprintf("更新作品《%s》", p.Title)
	var fieldsChanged []string
	if !existed {
		eventType = activity.EventProjectCreated
		title = fmt.Sprintf("创建作品《%s》", p.Title)
	} else {
		if frameworkJSON != beforeFrameworkJSON {
			eventType = activity.EventFrameworkUpdated
			title = fmt.Sprintf("更新创世总纲《%s》", p.Title)
		}
		var oldProj domain.Project
		if beforeJSON != "" {
			_ = json.Unmarshal([]byte(beforeJSON), &oldProj)
		}
		if oldProj.Title != p.Title {
			fieldsChanged = append(fieldsChanged, "title")
		}
		if oldProj.WorldRules != p.WorldRules {
			fieldsChanged = append(fieldsChanged, "world_rules")
		}
		if frameworkJSON != beforeFrameworkJSON {
			fieldsChanged = append(fieldsChanged, "framework")
		}
		if oldProj.DefaultWordsTarget != p.DefaultWordsTarget || oldProj.DefaultNarrativeStyle != p.DefaultNarrativeStyle {
			fieldsChanged = append(fieldsChanged, "defaults")
		}
	}

	summary := activity.SummaryFromCtx(ctx)
	e := newEvent(ctx, p.ID, activity.EntityProject, p.ID, 0, eventType, title, summary)
	if eventType == activity.EventFrameworkUpdated {
		e.EntityType = activity.EntityFramework
	}
	withTextSnapshots(e, beforeJSON, afterJSON, fieldsChanged)
	if err := insertEvent(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteStore) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	query := `SELECT id, title, target_platform, world_rules, COALESCE(framework_json, ''), COALESCE(default_words_target, 2000), COALESCE(default_narrative_style, 'hardboiled'), created_at, updated_at FROM projects WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var p domain.Project
	var frameworkJSON string
	if err := row.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &frameworkJSON, &p.DefaultWordsTarget, &p.DefaultNarrativeStyle, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	if strings.TrimSpace(frameworkJSON) != "" {
		var fw domain.ProjectFramework
		if err := json.Unmarshal([]byte(frameworkJSON), &fw); err == nil {
			p.Framework = &fw
		}
	}
	return &p, nil
}

func (s *SQLiteStore) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	query := `SELECT id, title, target_platform, world_rules, COALESCE(framework_json, ''), COALESCE(default_words_target, 2000), COALESCE(default_narrative_style, 'hardboiled'), created_at, updated_at FROM projects ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Project
	for rows.Next() {
		var p domain.Project
		var frameworkJSON string
		if err := rows.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &frameworkJSON, &p.DefaultWordsTarget, &p.DefaultNarrativeStyle, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(frameworkJSON) != "" {
			var fw domain.ProjectFramework
			if err := json.Unmarshal([]byte(frameworkJSON), &fw); err == nil {
				p.Framework = &fw
			}
		}
		list = append(list, &p)
	}
	return list, rows.Err()
}

func (s *SQLiteStore) DeleteProject(ctx context.Context, id string) error {
	var beforeJSON string
	if old, oerr := s.GetProject(ctx, id); oerr == nil && old != nil {
		if ob, merr := json.Marshal(old); merr == nil {
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

	// 显式清理无外键 ON DELETE CASCADE 约束的附随表，确保零孤儿数据残留
	orphanCleanupQueries := []string{
		`DELETE FROM chapter_quality_metrics WHERE project_id = ?`,
		`DELETE FROM chapter_versions WHERE project_id = ?`,
		`DELETE FROM chapter_state_snapshots WHERE project_id = ?`,
		`DELETE FROM compaction_cache WHERE project_id = ?`,
		`DELETE FROM project_automation WHERE project_id = ?`,
		`DELETE FROM memory_fts WHERE project_id = ?`,
		`DELETE FROM scenes WHERE project_id = ?`,
		`DELETE FROM book_queue WHERE project_id = ?`,
		`DELETE FROM studio_messages WHERE project_id = ?`,
		`DELETE FROM studio_threads WHERE project_id = ?`,
	}
	for _, q := range orphanCleanupQueries {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return fmt.Errorf("cleanup project table %q: %w", q, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id); err != nil {
		return err
	}

	// The ledger table has no FK to projects, so this tombstone survives deletion.
	e := newEvent(ctx, id, activity.EntityProject, id, 0, activity.EventProjectDeleted,
		"删除作品", "作品及其章节/伏笔/百科已删除，此条为账本留存的墓碑记录")
	withTextSnapshots(e, beforeJSON, "", []string{"deleted"})
	if err := emitLedger(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}
