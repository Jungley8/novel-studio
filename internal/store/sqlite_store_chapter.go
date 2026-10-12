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

func (s *SQLiteStore) CommitChapter(ctx context.Context, projectID string, c *domain.Chapter) (*domain.Project, error) {
	if projectID == "" {
		return nil, errors.New("projectID cannot be empty")
	}
	c.ProjectID = projectID
	if c.ChapterIndex <= 0 {
		return nil, errors.New("chapter_index must be greater than 0")
	}
	if c.ID == "" {
		c.ID = fmt.Sprintf("ch_%s_%d", projectID, c.ChapterIndex)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Fetch current project state
	var p domain.Project
	row := tx.QueryRowContext(ctx, `SELECT id, title, target_platform, world_rules, created_at, updated_at FROM projects WHERE id = ?`, projectID)
	if err := row.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("project not found")
		}
		return nil, fmt.Errorf("query project: %w", err)
	}

	// 1b. Ledger before-snapshots: overwritten chapter (if any) + protagonist.
	var beforeChapterJSON string
	var oldRow struct {
		id, title, conflict, content string
		words                        int
	}
	orow := tx.QueryRowContext(ctx, `SELECT id, title, core_conflict, content, word_count FROM chapters WHERE project_id = ? AND chapter_index = ?`, projectID, c.ChapterIndex)
	if oerr := orow.Scan(&oldRow.id, &oldRow.title, &oldRow.conflict, &oldRow.content, &oldRow.words); oerr == nil {
		if ob, merr := json.Marshal(map[string]any{
			"id": oldRow.id, "title": oldRow.title, "core_conflict": oldRow.conflict,
			"content": oldRow.content, "word_count": oldRow.words,
		}); merr == nil {
			beforeChapterJSON = string(ob)
		}
	}

	protagonist, err := s.getProtagonistTx(ctx, tx, projectID)
	if err != nil {
		return nil, fmt.Errorf("load protagonist in tx: %w", err)
	}
	var protagonistBeforeJSON string
	if protagonist != nil {
		if pb, merr := json.Marshal(protagonist); merr == nil {
			protagonistBeforeJSON = string(pb)
		}
	}

	// 2. Save chapter within transaction
	beatsJSON, err := json.Marshal(c.Beats)
	if err != nil {
		return nil, fmt.Errorf("marshal beats: %w", err)
	}
	mutationJSON, err := json.Marshal(c.StateMutation)
	if err != nil {
		return nil, fmt.Errorf("marshal state mutation: %w", err)
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.WordCount = len([]rune(c.Content))

	var reviewJSON string
	if c.Review != nil {
		if rBytes, err := json.Marshal(c.Review); err == nil {
			reviewJSON = string(rBytes)
		}
	}

	chapterQuery := `
	INSERT INTO chapters (
		id, project_id, chapter_index, title, core_conflict,
		beats_json, state_mutation_json, content, word_count,
		burstiness_score, linter_passed, review_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		title = excluded.title,
		core_conflict = excluded.core_conflict,
		beats_json = excluded.beats_json,
		state_mutation_json = excluded.state_mutation_json,
		content = excluded.content,
		word_count = excluded.word_count,
		burstiness_score = excluded.burstiness_score,
		linter_passed = excluded.linter_passed,
		review_json = excluded.review_json;
	`
	if _, err := tx.ExecContext(ctx, chapterQuery,
		c.ID, c.ProjectID, c.ChapterIndex, c.Title, c.CoreConflict,
		string(beatsJSON), string(mutationJSON), c.Content, c.WordCount,
		c.BurstinessScore, c.LinterPassed, reviewJSON, c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("insert chapter in tx: %w", err)
	}

	if err := s.insertChapterVersionTx(tx, projectID, c, "commit"); err != nil {
		return nil, fmt.Errorf("insert chapter version in tx: %w", err)
	}

	// 3. Atomically apply StateMutation to protagonist via EntityLedger
	var newProtagonistJSON string
	if protagonist != nil {
		_, err := domain.ApplyStateMutation(protagonist, c.StateMutation)
		if err != nil {
			return nil, fmt.Errorf("apply state mutation in tx: %w", err)
		}
		protagonist.UpdatedAt = time.Now()
		var levelJSON, itemsJSON string
		if protagonist.StructuredLevel != nil {
			if lb, lerr := json.Marshal(protagonist.StructuredLevel); lerr == nil {
				levelJSON = string(lb)
			}
		}
		if len(protagonist.StructuredItems) > 0 {
			if ib, ierr := json.Marshal(protagonist.StructuredItems); ierr == nil {
				itemsJSON = string(ib)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE codex_entries SET realm = ?, inventory = ?, structured_level_json = ?, structured_items_json = ?, updated_at = ? WHERE id = ?`,
			protagonist.Realm, protagonist.Inventory, levelJSON, itemsJSON, protagonist.UpdatedAt, protagonist.ID,
		); err != nil {
			return nil, fmt.Errorf("update protagonist in tx: %w", err)
		}
		if pb, merr := json.Marshal(protagonist); merr == nil {
			newProtagonistJSON = string(pb)
		}
	}

	// 4. Update plot hooks status: mark resolved hooks as RESOLVED, and overdue open hooks as FERMENTING
	if c.Review != nil && len(c.Review.ResolvedHookIDs) > 0 {
		for _, hid := range c.Review.ResolvedHookIDs {
			hid = strings.TrimSpace(hid)
			if hid != "" {
				_, _ = tx.ExecContext(ctx,
					`UPDATE plot_hooks SET status = 'RESOLVED' WHERE project_id = ? AND (id = ? OR title = ?)`,
					projectID, hid, hid,
				)
			}
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE plot_hooks SET status = 'FERMENTING' WHERE project_id = ? AND status = 'OPEN' AND target_chapter <= ?`,
		projectID, c.ChapterIndex,
	); err != nil {
		return nil, fmt.Errorf("update plot hooks in tx: %w", err)
	}

	// 4b. Apply CharacterMutations to matching Codex character entries
	if len(c.StateMutation.CharacterMutations) > 0 {
		for _, cm := range c.StateMutation.CharacterMutations {
			cmName := strings.TrimSpace(cm.Name)
			if cmName == "" {
				continue
			}
			var entryID string
			row := tx.QueryRowContext(ctx, `
				SELECT id FROM codex_entries 
				WHERE project_id = ? AND (name = ? OR id IN (SELECT entry_id FROM codex_aliases WHERE alias = ?))
				LIMIT 1
			`, projectID, cmName, cmName)
			if row.Scan(&entryID) == nil && entryID != "" {
				progID := fmt.Sprintf("prog-%d-%s", time.Now().UnixNano(), entryID)
				payloadJSON := fmt.Sprintf(`{"status":"%s","relation":"%s"}`,
					strings.ReplaceAll(cm.StatusDelta, `"`, `\"`),
					strings.ReplaceAll(cm.RelationDelta, `"`, `\"`),
				)
				notes := strings.TrimSpace(cm.StatusDelta)
				if cm.RelationDelta != "" {
					if notes != "" {
						notes += "；"
					}
					notes += "对主角/因果变迁: " + cm.RelationDelta
				}
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO codex_progressions (id, entry_id, active_from_chapter, state_payload_json, notes, created_at)
					VALUES (?, ?, ?, ?, ?, ?)
				`, progID, entryID, c.ChapterIndex, payloadJSON, notes, time.Now())

				progAfter, _ := json.Marshal(map[string]any{
					"entry_id": entryID, "name": cmName, "chapter": c.ChapterIndex,
					"status_delta": cm.StatusDelta, "relation_delta": cm.RelationDelta,
				})
				progEvent := newEvent(ctx, projectID, activity.EntityCodexProgression, progID, c.ChapterIndex,
					activity.EventCodexProgAdded, fmt.Sprintf("「%s」状态演进（第 %d 章）", cmName, c.ChapterIndex), notes)
				withTextSnapshots(progEvent, "", string(progAfter), []string{"state_payload"})
				_ = insertEvent(ctx, tx, progEvent)

				// Update current_disposition if specified
				if cm.RelationDelta != "" {
					_, _ = tx.ExecContext(ctx, `
						UPDATE codex_entries SET current_disposition = ?, updated_at = ? WHERE id = ?
					`, cm.RelationDelta, time.Now(), entryID)
				}
			}
		}
	}

	if err := s.insertStateSnapshotTx(ctx, tx, projectID, c.ChapterIndex); err != nil {
		return nil, fmt.Errorf("insert state snapshot in tx: %w", err)
	}

	// 5. Clear checkpoint if one was saved during drafting
	_, _ = tx.ExecContext(ctx,
		`DELETE FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`,
		projectID, c.ChapterIndex,
	)

	// 6. Ledger: chapter.committed with full after snapshot + protagonist delta.
	afterBytes, _ := json.Marshal(c)
	verdictStr := ""
	score := 0
	resolvedCount := 0
	if c.Review != nil {
		verdictStr = string(c.Review.Verdict)
		score = c.Review.Score
		resolvedCount = len(c.Review.ResolvedHookIDs)
	}
	summary := fmt.Sprintf("第 %d 章归档 %d 字", c.ChapterIndex, c.WordCount)
	if verdictStr != "" {
		summary += fmt.Sprintf("，终审 %s %d 分", verdictStr, score)
	}
	if resolvedCount > 0 {
		summary += fmt.Sprintf("，回收伏笔 %d 条", resolvedCount)
	}
	if beforeChapterJSON != "" {
		summary += "（覆盖旧稿）"
	}
	commitEvent := newEvent(ctx, projectID, activity.EntityChapter, c.ID, c.ChapterIndex,
		activity.EventChapterCommitted, fmt.Sprintf("第 %d 章已归档", c.ChapterIndex), summary)
	withTextSnapshots(commitEvent, beforeChapterJSON, string(afterBytes), []string{"content", "beats", "state_mutation", "review"})
	commitEvent.ExcerptBefore = activity.Excerpt(oldRow.content, activity.ExcerptRunes)
	commitEvent.ExcerptAfter = activity.Excerpt(c.Content, activity.ExcerptRunes)
	// Protagonist delta rides along so state-machine drift is visible on the chapter event.
	var protBefore, protAfter map[string]any
	_ = json.Unmarshal([]byte(protagonistBeforeJSON), &protBefore)
	_ = json.Unmarshal([]byte(newProtagonistJSON), &protAfter)
	for _, f := range activity.ChangedFields(protBefore, protAfter) {
		commitEvent.FieldsChanged = append(commitEvent.FieldsChanged, "protagonist."+f)
	}
	if len(activity.ChangedFields(protBefore, protAfter)) > 0 {
		commitEvent.Summary += "，主角状态机已演进"
	}
	if err := insertEvent(ctx, tx, commitEvent); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	tx = nil

	return &p, nil
}

func (s *SQLiteStore) SaveChapter(ctx context.Context, c *domain.Chapter) error {
	if c.ProjectID == "" {
		return errors.New("chapter requires project_id")
	}
	if c.ChapterIndex <= 0 {
		return errors.New("chapter_index must be greater than 0")
	}

	beatsJSON, err := json.Marshal(c.Beats)
	if err != nil {
		return fmt.Errorf("marshal beats: %w", err)
	}
	mutationJSON, err := json.Marshal(c.StateMutation)
	if err != nil {
		return fmt.Errorf("marshal state mutation: %w", err)
	}

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.WordCount = len([]rune(c.Content))

	var reviewJSON string
	if c.Review != nil {
		if rBytes, err := json.Marshal(c.Review); err == nil {
			reviewJSON = string(rBytes)
		}
	}

	var beforeContent string
	orow := s.db.QueryRowContext(ctx, `SELECT content FROM chapters WHERE project_id = ? AND chapter_index = ?`, c.ProjectID, c.ChapterIndex)
	_ = orow.Scan(&beforeContent)

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
	INSERT INTO chapters (
		id, project_id, chapter_index, title, core_conflict,
		beats_json, state_mutation_json, content, word_count,
		burstiness_score, linter_passed, review_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		title = excluded.title,
		core_conflict = excluded.core_conflict,
		beats_json = excluded.beats_json,
		state_mutation_json = excluded.state_mutation_json,
		content = excluded.content,
		word_count = excluded.word_count,
		burstiness_score = excluded.burstiness_score,
		linter_passed = excluded.linter_passed,
		review_json = excluded.review_json;
	`
	if _, err = tx.ExecContext(ctx, query,
		c.ID, c.ProjectID, c.ChapterIndex, c.Title, c.CoreConflict,
		string(beatsJSON), string(mutationJSON), c.Content, c.WordCount,
		c.BurstinessScore, c.LinterPassed, reviewJSON, c.CreatedAt,
	); err != nil {
		return err
	}

	if err := s.insertChapterVersionTx(tx, c.ProjectID, c, "edit"); err != nil {
		return fmt.Errorf("insert chapter version in tx: %w", err)
	}

	afterBytes, _ := json.Marshal(c)
	e := newEvent(ctx, c.ProjectID, activity.EntityChapter, c.ID, c.ChapterIndex,
		activity.EventChapterDrafted, fmt.Sprintf("第 %d 章草稿已保存", c.ChapterIndex),
		fmt.Sprintf("草稿 %d 字（未触发状态机结算）", c.WordCount))
	withTextSnapshots(e, beforeContent, string(afterBytes), []string{"content"})
	e.ExcerptBefore = activity.Excerpt(beforeContent, activity.ExcerptRunes)
	e.ExcerptAfter = activity.Excerpt(c.Content, activity.ExcerptRunes)
	if err := insertEvent(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

// ChapterProjectID 返回指定章节 ID 实际归属的作品 ID；章节不存在时返回空字符串。
// 用于请求侧 Fail-Closed 校验：场景等子资源不得挂载其它作品的章节。
func (s *SQLiteStore) ChapterProjectID(ctx context.Context, id string) (string, error) {
	var projectID string
	err := s.db.QueryRowContext(ctx, `SELECT project_id FROM chapters WHERE id = ?`, id).Scan(&projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("query chapter project: %w", err)
	}
	return projectID, nil
}

func (s *SQLiteStore) GetChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.Chapter, error) {
	query := `
	SELECT id, project_id, chapter_index, title, core_conflict, beats_json, state_mutation_json,
	       content, word_count, burstiness_score, linter_passed, COALESCE(review_json, ''), created_at
	FROM chapters WHERE project_id = ? AND chapter_index = ?;
	`
	row := s.db.QueryRowContext(ctx, query, projectID, chapterIndex)

	var c domain.Chapter
	var beatsJSON, mutationJSON, reviewJSON string
	if err := row.Scan(
		&c.ID, &c.ProjectID, &c.ChapterIndex, &c.Title, &c.CoreConflict,
		&beatsJSON, &mutationJSON, &c.Content, &c.WordCount,
		&c.BurstinessScore, &c.LinterPassed, &reviewJSON, &c.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("chapter not found")
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(beatsJSON), &c.Beats)
	_ = json.Unmarshal([]byte(mutationJSON), &c.StateMutation)
	if reviewJSON != "" {
		var rev domain.ReviewResult
		if err := json.Unmarshal([]byte(reviewJSON), &rev); err == nil {
			c.Review = &rev
		}
	}
	return &c, nil
}

func (s *SQLiteStore) ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error) {
	query := `
	SELECT id, project_id, chapter_index, title, core_conflict, beats_json, state_mutation_json,
	       content, word_count, burstiness_score, linter_passed, COALESCE(review_json, ''), created_at
	FROM chapters WHERE project_id = ? ORDER BY chapter_index ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Chapter
	for rows.Next() {
		var c domain.Chapter
		var beatsJSON, mutationJSON, reviewJSON string
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.ChapterIndex, &c.Title, &c.CoreConflict,
			&beatsJSON, &mutationJSON, &c.Content, &c.WordCount,
			&c.BurstinessScore, &c.LinterPassed, &reviewJSON, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(beatsJSON), &c.Beats)
		_ = json.Unmarshal([]byte(mutationJSON), &c.StateMutation)
		if reviewJSON != "" {
			var rev domain.ReviewResult
			if err := json.Unmarshal([]byte(reviewJSON), &rev); err == nil {
				c.Review = &rev
			}
		}
		list = append(list, &c)
	}
	return list, rows.Err()
}

var rollbackStateMutation = domain.RollbackStateMutation

func (s *SQLiteStore) getStateSnapshotTx(ctx context.Context, tx *sql.Tx, projectID string, chapterIndex int) (*domain.ChapterStateSnapshot, error) {
	query := `
	SELECT id, project_id, chapter_index, protagonist_json, hooks_json, created_at
	FROM chapter_state_snapshots
	WHERE project_id = ? AND chapter_index = ?;
	`
	row := tx.QueryRowContext(ctx, query, projectID, chapterIndex)

	var snap domain.ChapterStateSnapshot
	if err := row.Scan(
		&snap.ID, &snap.ProjectID, &snap.ChapterIndex, &snap.ProtagonistJSON, &snap.HooksJSON, &snap.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter state snapshot in tx: %w", err)
	}
	return &snap, nil
}

// ListStateSnapshots returns all post-commit state snapshots for a project ordered by chapter_index ascending.
// Read-only supply for the bench snapshot chain; opens no transaction (mirrors ListChapters connection usage).
func (s *SQLiteStore) ListStateSnapshots(ctx context.Context, projectID string) ([]*domain.ChapterStateSnapshot, error) {
	query := `
	SELECT id, project_id, chapter_index, protagonist_json, hooks_json, created_at
	FROM chapter_state_snapshots
	WHERE project_id = ? ORDER BY chapter_index ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapter state snapshots: %w", err)
	}
	defer rows.Close()

	list := make([]*domain.ChapterStateSnapshot, 0)
	for rows.Next() {
		var snap domain.ChapterStateSnapshot
		if err := rows.Scan(
			&snap.ID, &snap.ProjectID, &snap.ChapterIndex, &snap.ProtagonistJSON, &snap.HooksJSON, &snap.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chapter state snapshot: %w", err)
		}
		list = append(list, &snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chapter state snapshots: %w", err)
	}
	return list, nil
}

func (s *SQLiteStore) UncommitChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, *domain.Project, error) {
	if projectID == "" {
		return nil, nil, errors.New("projectID cannot be empty")
	}
	if chapterIndex <= 0 {
		return nil, nil, errors.New("chapterIndex must be greater than 0")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// 0. Guard: only the latest chapter can be uncommitted
	var maxChapter int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(chapter_index), 0) FROM chapters WHERE project_id = ?`, projectID).Scan(&maxChapter); err != nil {
		return nil, nil, fmt.Errorf("query max chapter: %w", err)
	}
	if chapterIndex < maxChapter {
		rows, err := tx.QueryContext(ctx, `SELECT chapter_index FROM chapters WHERE project_id = ? AND chapter_index > ? ORDER BY chapter_index ASC`, projectID, chapterIndex)
		if err != nil {
			return nil, nil, fmt.Errorf("query downstream chapters: %w", err)
		}
		var downstream []int
		for rows.Next() {
			var idx int
			if err := rows.Scan(&idx); err != nil {
				rows.Close()
				return nil, nil, fmt.Errorf("scan downstream chapter: %w", err)
			}
			downstream = append(downstream, idx)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, fmt.Errorf("downstream chapters rows: %w", err)
		}
		return nil, nil, &NonLatestChapterError{
			ChapterIndex: chapterIndex,
			MaxChapter:   maxChapter,
			Downstream:   downstream,
		}
	}

	// 1. Fetch the chapter
	query := `
	SELECT id, project_id, chapter_index, title, core_conflict, beats_json, state_mutation_json,
	       content, word_count, burstiness_score, linter_passed, COALESCE(review_json, ''), created_at
	FROM chapters WHERE project_id = ? AND chapter_index = ?;
	`
	row := tx.QueryRowContext(ctx, query, projectID, chapterIndex)

	var c domain.Chapter
	var beatsJSON, mutationJSON, reviewJSON string
	if err := row.Scan(
		&c.ID, &c.ProjectID, &c.ChapterIndex, &c.Title, &c.CoreConflict,
		&beatsJSON, &mutationJSON, &c.Content, &c.WordCount,
		&c.BurstinessScore, &c.LinterPassed, &reviewJSON, &c.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, errors.New("chapter not found")
		}
		return nil, nil, fmt.Errorf("scan chapter: %w", err)
	}

	_ = json.Unmarshal([]byte(beatsJSON), &c.Beats)
	_ = json.Unmarshal([]byte(mutationJSON), &c.StateMutation)
	if reviewJSON != "" {
		var rev domain.ReviewResult
		if err := json.Unmarshal([]byte(reviewJSON), &rev); err == nil {
			c.Review = &rev
		}
	}

	// 2. Fetch current project
	var p domain.Project
	pRow := tx.QueryRowContext(ctx, `SELECT id, title, target_platform, world_rules, created_at, updated_at FROM projects WHERE id = ?`, projectID)
	if err := pRow.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, nil, fmt.Errorf("query project: %w", err)
	}

	// 3. Formulate checkpoint from chapter
	phase := domain.CheckpointPhaseDrafted
	var auditReport *domain.AuditReport
	if c.Review != nil {
		phase = domain.CheckpointPhaseDrafted
		auditReport = &domain.AuditReport{
			Verdict:         domain.ReviewVerdictRevision,
			Score:           c.Review.Score,
			BurstinessScore: c.BurstinessScore,
			Issues:          append([]string{"已从正史撤回为在途草稿，需重新审校与针对性精修"}, c.Review.Issues...),
			Suggestions:     c.Review.Suggestions,
			ResolvedHookIDs: c.Review.ResolvedHookIDs,
			ReviewedAt:      c.Review.ReviewedAt,
		}
	}

	cp := &domain.ChapterCheckpoint{
		ProjectID:     projectID,
		ChapterIndex:  chapterIndex,
		Phase:         phase,
		CoreConflict:  c.CoreConflict,
		Beats:         c.Beats,
		StateMutation: c.StateMutation,
		DraftText:     c.Content,
		AuditReport:   auditReport,
		RewriteLoops:  0,
		UpdatedAt:     time.Now(),
	}

	payload, err := json.Marshal(cp)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal checkpoint payload: %w", err)
	}

	cpQuery := `
	INSERT INTO chapter_checkpoints (project_id, chapter_index, phase, core_conflict, payload_json, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		phase = excluded.phase,
		core_conflict = excluded.core_conflict,
		payload_json = excluded.payload_json,
		updated_at = excluded.updated_at;
	`
	if _, err := tx.ExecContext(ctx, cpQuery,
		cp.ProjectID, cp.ChapterIndex, string(cp.Phase), cp.CoreConflict,
		string(payload), cp.UpdatedAt,
	); err != nil {
		return nil, nil, fmt.Errorf("save checkpoint in tx: %w", err)
	}

	// 4. Check state snapshot if chapterIndex >= 2 (Fail-Closed: 缺失快照时拒绝回撤，禁止逆运算降级)
	var snap *domain.ChapterStateSnapshot
	if chapterIndex >= 2 {
		var snapErr error
		snap, snapErr = s.getStateSnapshotTx(ctx, tx, projectID, chapterIndex-1)
		if snapErr != nil {
			return nil, nil, fmt.Errorf("query chapter state snapshot in tx: %w", snapErr)
		}
		if snap == nil {
			// 理由：逆运算有解析歧义，静默降级不可接受，必须严格 Fail-Closed
			return nil, nil, fmt.Errorf("cannot uncommit chapter %d: state snapshot for chapter %d is missing (pre-history chapter); manual state correction required", chapterIndex, chapterIndex-1)
		}
	}

	// 5. Delete the chapter from chapters table
	if _, err := tx.ExecContext(ctx, `DELETE FROM chapters WHERE project_id = ? AND chapter_index = ?`, projectID, chapterIndex); err != nil {
		return nil, nil, fmt.Errorf("delete chapter in tx: %w", err)
	}

	// 6. Restore world state and plot hooks
	var rollbackTrail []string
	if chapterIndex >= 2 {
		// chapterIndex >= 2 且快照存在：由第 N-1 章快照精确还原主角状态与伏笔状态
		if snap.ProtagonistJSON != "" && snap.ProtagonistJSON != "{}" && snap.ProtagonistJSON != "null" {
			var snapProt domain.CodexEntry
			if err := json.Unmarshal([]byte(snap.ProtagonistJSON), &snapProt); err != nil {
				return nil, nil, fmt.Errorf("unmarshal protagonist snapshot: %w", err)
			}
			if snapProt.ID != "" {
				var levelJSON, itemsJSON string
				if snapProt.StructuredLevel != nil {
					if lb, lerr := json.Marshal(snapProt.StructuredLevel); lerr == nil {
						levelJSON = string(lb)
					}
				}
				if len(snapProt.StructuredItems) > 0 {
					if ib, ierr := json.Marshal(snapProt.StructuredItems); ierr == nil {
						itemsJSON = string(ib)
					}
				}
				category := string(snapProt.Category)
				if category == "" {
					category = string(domain.CategoryCharacter)
				}
				trackingMode := string(snapProt.TrackingMode)
				if trackingMode == "" {
					trackingMode = string(domain.TrackingModeAutoMention)
				}
				if _, err := tx.ExecContext(ctx, `
					UPDATE codex_entries SET
						category = ?,
						name = ?,
						color_tag = ?,
						summary = ?,
						details_markdown = ?,
						tracking_mode = ?,
						archetype = ?,
						voice_tone = ?,
						want = ?,
						need = ?,
						ghost = ?,
						current_disposition = ?,
						realm = ?,
						inventory = ?,
						health_status = ?,
						structured_level_json = ?,
						structured_items_json = ?,
						alignment = ?,
						threat_level = ?,
						updated_at = ?
					WHERE id = ?
				`,
					category,
					snapProt.Name,
					snapProt.ColorTag,
					snapProt.Summary,
					snapProt.DetailsMarkdown,
					trackingMode,
					snapProt.Archetype,
					snapProt.VoiceTone,
					snapProt.Want,
					snapProt.Need,
					snapProt.Ghost,
					snapProt.CurrentDisposition,
					snapProt.Realm,
					snapProt.Inventory,
					snapProt.HealthStatus,
					levelJSON,
					itemsJSON,
					snapProt.Alignment,
					snapProt.ThreatLevel,
					time.Now(),
					snapProt.ID,
				); err != nil {
					return nil, nil, fmt.Errorf("restore protagonist codex entry in tx: %w", err)
				}
			}
		}

		if snap.HooksJSON != "" && snap.HooksJSON != "null" {
			var snapHooks []*domain.PlotHook
			if err := json.Unmarshal([]byte(snap.HooksJSON), &snapHooks); err != nil {
				return nil, nil, fmt.Errorf("unmarshal plot hooks snapshot: %w", err)
			}
			for _, h := range snapHooks {
				if h != nil && strings.TrimSpace(h.ID) != "" {
					if _, err := tx.ExecContext(ctx,
						`UPDATE plot_hooks SET status = ? WHERE project_id = ? AND id = ?`,
						string(h.Status), projectID, strings.TrimSpace(h.ID),
					); err != nil {
						return nil, nil, fmt.Errorf("restore plot hook in tx: %w", err)
					}
				}
			}
		}
	} else {
		// chapterIndex == 1:
		// 第 1 章的前态是创世空态，逆运算在此边界与成因等价，无语义风险，保留逆运算回滚路径。
		// 伏笔部分：第 1 章时快照(0)不存在，保留现有的"已回收→OPEN"还原 + FERMENTING 重算（maxRemaining=0 时 FERMENTING 会全归 OPEN，现逻辑正确）。
		protagonist, _ := s.getProtagonistTx(ctx, tx, projectID)
		if protagonist != nil {
			trail, rerr := rollbackStateMutation(protagonist, c.StateMutation)
			if rerr != nil {
				return nil, nil, fmt.Errorf("rollback protagonist mutation: %w", rerr)
			}
			rollbackTrail = trail
			protagonist.UpdatedAt = time.Now()
			var levelJSON, itemsJSON string
			if protagonist.StructuredLevel != nil {
				if lb, lerr := json.Marshal(protagonist.StructuredLevel); lerr == nil {
					levelJSON = string(lb)
				}
			}
			if len(protagonist.StructuredItems) > 0 {
				if ib, ierr := json.Marshal(protagonist.StructuredItems); ierr == nil {
					itemsJSON = string(ib)
				}
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE codex_entries SET realm = ?, inventory = ?, structured_level_json = ?, structured_items_json = ?, updated_at = ? WHERE id = ?`,
				protagonist.Realm, protagonist.Inventory, levelJSON, itemsJSON, protagonist.UpdatedAt, protagonist.ID,
			); err != nil {
				return nil, nil, fmt.Errorf("update protagonist codex entry: %w", err)
			}
		}

		if c.Review != nil && len(c.Review.ResolvedHookIDs) > 0 {
			for _, hid := range c.Review.ResolvedHookIDs {
				hid = strings.TrimSpace(hid)
				if hid != "" {
					if _, err := tx.ExecContext(ctx,
						`UPDATE plot_hooks SET status = 'OPEN' WHERE project_id = ? AND (id = ? OR title = ?)`,
						projectID, hid, hid,
					); err != nil {
						return nil, nil, fmt.Errorf("revert resolved hook in tx: %w", err)
					}
				}
			}
		}

		var maxRemaining int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(chapter_index), 0) FROM chapters WHERE project_id = ?`, projectID).Scan(&maxRemaining); err != nil {
			return nil, nil, fmt.Errorf("query max remaining chapter: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE plot_hooks 
			SET status = CASE 
				WHEN target_chapter <= ? THEN 'FERMENTING' 
				ELSE 'OPEN' 
			END 
			WHERE project_id = ? AND status IN ('OPEN', 'FERMENTING')
		`, maxRemaining, projectID); err != nil {
			return nil, nil, fmt.Errorf("recalculate plot hooks in tx: %w", err)
		}
	}

	// 7. Delete any codex progressions triggered from this chapter
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM codex_progressions 
		WHERE entry_id IN (SELECT id FROM codex_entries WHERE project_id = ?) 
		  AND active_from_chapter = ?
	`, projectID, chapterIndex); err != nil {
		return nil, nil, fmt.Errorf("delete codex progressions in tx: %w", err)
	}

	// 8. Ledger: chapter.uncommitted keeps the full withdrawn draft inspectable.
	beforeBytes, _ := json.Marshal(c)
	summary := fmt.Sprintf("正文 %d 字回退至断点，伏笔回收已还原", len([]rune(c.Content)))
	if len(rollbackTrail) > 0 {
		summary += "；状态回滚明细: " + strings.Join(rollbackTrail, "；")
	}
	uncommitEvent := newEvent(ctx, projectID, activity.EntityChapter, c.ID, chapterIndex,
		activity.EventChapterUncommitted, fmt.Sprintf("第 %d 章已撤回为草稿", chapterIndex),
		summary)
	withTextSnapshots(uncommitEvent, string(beforeBytes), "", []string{"content", "status"})
	uncommitEvent.ExcerptBefore = activity.Excerpt(c.Content, activity.ExcerptRunes)
	if err := insertEvent(ctx, tx, uncommitEvent); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}
	tx = nil

	return cp, &p, nil
}

func (s *SQLiteStore) DeleteChapter(ctx context.Context, projectID string, chapterIndex int) error {
	var beforeContent, chapterID string
	orow := s.db.QueryRowContext(ctx, `SELECT id, content FROM chapters WHERE project_id = ? AND chapter_index = ?`, projectID, chapterIndex)
	_ = orow.Scan(&chapterID, &beforeContent)
	if chapterID == "" {
		chapterID = fmt.Sprintf("ch_%s_%d", projectID, chapterIndex)
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

	// Guard: only the latest chapter can be hard-deleted (mirrors UncommitChapter).
	// Deleting a non-latest chapter would leave its state-mutation effects
	// permanently applied while the chapter itself is gone.
	var maxChapter int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(chapter_index), 0) FROM chapters WHERE project_id = ?`, projectID).Scan(&maxChapter); err != nil {
		return fmt.Errorf("query max chapter: %w", err)
	}
	if chapterIndex < maxChapter {
		rows, err := tx.QueryContext(ctx, `SELECT chapter_index FROM chapters WHERE project_id = ? AND chapter_index > ? ORDER BY chapter_index ASC`, projectID, chapterIndex)
		if err != nil {
			return fmt.Errorf("query downstream chapters: %w", err)
		}
		var downstream []int
		for rows.Next() {
			var idx int
			if err := rows.Scan(&idx); err != nil {
				rows.Close()
				return fmt.Errorf("scan downstream chapter: %w", err)
			}
			downstream = append(downstream, idx)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("downstream chapters rows: %w", err)
		}
		return &NonLatestChapterError{
			ChapterIndex: chapterIndex,
			MaxChapter:   maxChapter,
			Downstream:   downstream,
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM chapters WHERE project_id = ? AND chapter_index = ?`, projectID, chapterIndex); err != nil {
		return err
	}

	e := newEvent(ctx, projectID, activity.EntityChapter, chapterID, chapterIndex,
		activity.EventChapterDeleted, fmt.Sprintf("第 %d 章已删除", chapterIndex),
		fmt.Sprintf("删除正文 %d 字（状态机不回滚，墓碑留存）", len([]rune(beforeContent))))
	withTextSnapshots(e, beforeContent, "", []string{"deleted"})
	if err := emitLedger(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}
