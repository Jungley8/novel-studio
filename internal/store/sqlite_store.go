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
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	// Optimize for concurrency & single-file responsiveness
	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set pragma failed: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		target_platform TEXT NOT NULL,
		world_rules TEXT,
		protagonist_json TEXT,
		framework_json TEXT,
		default_words_target INTEGER DEFAULT 2000,
		default_narrative_style TEXT DEFAULT 'hardboiled',
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chapters (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		title TEXT NOT NULL,
		core_conflict TEXT,
		beats_json TEXT,
		state_mutation_json TEXT,
		content TEXT,
		word_count INTEGER DEFAULT 0,
		burstiness_score INTEGER DEFAULT 0,
		linter_passed BOOLEAN DEFAULT 0,
		review_json TEXT,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
		UNIQUE(project_id, chapter_index)
	);

	CREATE TABLE IF NOT EXISTS plot_hooks (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		title TEXT NOT NULL,
		details TEXT,
		created_chapter INTEGER NOT NULL,
		target_chapter INTEGER NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS chapter_checkpoints (
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		phase TEXT NOT NULL,
		core_conflict TEXT,
		payload_json TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		PRIMARY KEY(project_id, chapter_index),
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_chapters_project ON chapters(project_id, chapter_index);
	CREATE INDEX IF NOT EXISTS idx_hooks_project ON plot_hooks(project_id, status);
	CREATE INDEX IF NOT EXISTS idx_checkpoints_project ON chapter_checkpoints(project_id, chapter_index);

	CREATE TABLE IF NOT EXISTS codex_entries (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		category TEXT NOT NULL,
		name TEXT NOT NULL,
		color_tag TEXT,
		summary TEXT,
		details_markdown TEXT,
		tracking_mode TEXT DEFAULT 'AUTO_MENTION',
		archetype TEXT DEFAULT '',
		voice_tone TEXT DEFAULT '',
		core_motivation TEXT DEFAULT '',
		current_disposition TEXT DEFAULT '',
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS codex_aliases (
		id TEXT PRIMARY KEY,
		entry_id TEXT NOT NULL,
		alias TEXT NOT NULL,
		FOREIGN KEY(entry_id) REFERENCES codex_entries(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS codex_progressions (
		id TEXT PRIMARY KEY,
		entry_id TEXT NOT NULL,
		active_from_chapter INTEGER NOT NULL,
		state_payload_json TEXT NOT NULL,
		notes TEXT,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(entry_id) REFERENCES codex_entries(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS codex_relations (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		source_entry_id TEXT NOT NULL,
		target_entry_id TEXT NOT NULL,
		relation_type TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_codex_aliases ON codex_aliases(alias);
	CREATE INDEX IF NOT EXISTS idx_codex_entries_proj ON codex_entries(project_id, category);
	CREATE INDEX IF NOT EXISTS idx_codex_progressions ON codex_progressions(entry_id, active_from_chapter);
	CREATE INDEX IF NOT EXISTS idx_codex_relations ON codex_relations(project_id, source_entry_id);

	CREATE TABLE IF NOT EXISTS scenes (
		id TEXT PRIMARY KEY,
		chapter_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		scene_index INTEGER NOT NULL,
		title TEXT,
		location_entry_id TEXT,
		dramatic_goal TEXT,
		conflict_barrier TEXT,
		tension_level INTEGER DEFAULT 5,
		beats_json TEXT NOT NULL,
		state_mutation_json TEXT,
		prose_content TEXT,
		word_count INTEGER DEFAULT 0,
		exclude_from_ai BOOLEAN DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		FOREIGN KEY(chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS scene_markers (
		id TEXT PRIMARY KEY,
		scene_id TEXT NOT NULL,
		marker_type TEXT NOT NULL,
		color TEXT NOT NULL,
		text_range_start INTEGER NOT NULL,
		text_range_end INTEGER NOT NULL,
		content TEXT NOT NULL,
		resolved BOOLEAN DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(scene_id) REFERENCES scenes(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_scenes_chapter ON scenes(chapter_id, scene_index);
	CREATE INDEX IF NOT EXISTS idx_scenes_project ON scenes(project_id);
	CREATE INDEX IF NOT EXISTS idx_scene_markers_scene ON scene_markers(scene_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Backward compatible schema patch for existing DBs
	_, _ = s.db.Exec(`ALTER TABLE chapters ADD COLUMN review_json TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE projects ADD COLUMN framework_json TEXT;`)
	_, _ = s.db.Exec(`UPDATE projects SET framework_json = '' WHERE framework_json IS NULL;`)
	_, _ = s.db.Exec(`ALTER TABLE projects ADD COLUMN default_words_target INTEGER DEFAULT 2000;`)
	_, _ = s.db.Exec(`ALTER TABLE projects ADD COLUMN default_narrative_style TEXT DEFAULT 'hardboiled';`)
	_, _ = s.db.Exec(`UPDATE projects SET default_words_target = 2000 WHERE default_words_target IS NULL OR default_words_target = 0;`)
	_, _ = s.db.Exec(`UPDATE projects SET default_narrative_style = 'hardboiled' WHERE default_narrative_style IS NULL OR default_narrative_style = '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN archetype TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN voice_tone TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN core_motivation TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN current_disposition TEXT DEFAULT '';`)
	return nil
}

func (s *SQLiteStore) SaveProject(ctx context.Context, p *domain.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.ID == "" {
		return errors.New("project ID cannot be empty")
	}

	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	protagonistJSON, err := json.Marshal(p.Protagonist)
	if err != nil {
		return fmt.Errorf("marshal protagonist: %w", err)
	}

	var frameworkJSON string
	if p.Framework != nil {
		if fb, ferr := json.Marshal(p.Framework); ferr == nil {
			frameworkJSON = string(fb)
		}
	}

	query := `
	INSERT INTO projects (id, title, target_platform, world_rules, protagonist_json, framework_json, default_words_target, default_narrative_style, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		target_platform = excluded.target_platform,
		world_rules = excluded.world_rules,
		protagonist_json = excluded.protagonist_json,
		framework_json = excluded.framework_json,
		default_words_target = excluded.default_words_target,
		default_narrative_style = excluded.default_narrative_style,
		updated_at = excluded.updated_at;
	`
	_, err = s.db.ExecContext(ctx, query,
		p.ID, p.Title, p.TargetPlatform, p.WorldRules, string(protagonistJSON), frameworkJSON, p.DefaultWordsTarget, p.DefaultNarrativeStyle, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	query := `SELECT id, title, target_platform, world_rules, protagonist_json, COALESCE(framework_json, ''), COALESCE(default_words_target, 2000), COALESCE(default_narrative_style, 'hardboiled'), created_at, updated_at FROM projects WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var p domain.Project
	var protagonistJSON, frameworkJSON string
	if err := row.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &protagonistJSON, &frameworkJSON, &p.DefaultWordsTarget, &p.DefaultNarrativeStyle, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(protagonistJSON), &p.Protagonist); err != nil {
		return nil, fmt.Errorf("unmarshal protagonist: %w", err)
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
	query := `SELECT id, title, target_platform, world_rules, protagonist_json, COALESCE(framework_json, ''), COALESCE(default_words_target, 2000), COALESCE(default_narrative_style, 'hardboiled'), created_at, updated_at FROM projects ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Project
	for rows.Next() {
		var p domain.Project
		var protagonistJSON, frameworkJSON string
		if err := rows.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &protagonistJSON, &frameworkJSON, &p.DefaultWordsTarget, &p.DefaultNarrativeStyle, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(protagonistJSON), &p.Protagonist)
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
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	return err
}

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
	var protagonistJSON string
	row := tx.QueryRowContext(ctx, `SELECT id, title, target_platform, world_rules, protagonist_json, created_at, updated_at FROM projects WHERE id = ?`, projectID)
	if err := row.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &protagonistJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("project not found")
		}
		return nil, fmt.Errorf("query project: %w", err)
	}
	if err := json.Unmarshal([]byte(protagonistJSON), &p.Protagonist); err != nil {
		return nil, fmt.Errorf("unmarshal protagonist: %w", err)
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

	// 3. Atomically apply StateMutation to protagonist via EntityLedger
	updatedProtagonist, _, err := domain.ApplyStateMutation(p.Protagonist, c.StateMutation)
	if err != nil {
		return nil, fmt.Errorf("apply state mutation in tx: %w", err)
	}
	p.Protagonist = updatedProtagonist
	p.UpdatedAt = time.Now()

	newProtagonistJSON, err := json.Marshal(p.Protagonist)
	if err != nil {
		return nil, fmt.Errorf("marshal updated protagonist: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE projects SET protagonist_json = ?, updated_at = ? WHERE id = ?`,
		string(newProtagonistJSON), p.UpdatedAt, p.ID,
	); err != nil {
		return nil, fmt.Errorf("update project protagonist in tx: %w", err)
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

				// Update current_disposition if specified
				if cm.RelationDelta != "" {
					_, _ = tx.ExecContext(ctx, `
						UPDATE codex_entries SET current_disposition = ?, updated_at = ? WHERE id = ?
					`, cm.RelationDelta, time.Now(), entryID)
				}
			}
		}
	}

	// 5. Clear checkpoint if one was saved during drafting
	_, _ = tx.ExecContext(ctx,
		`DELETE FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`,
		projectID, c.ChapterIndex,
	)

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
	_, err = s.db.ExecContext(ctx, query,
		c.ID, c.ProjectID, c.ChapterIndex, c.Title, c.CoreConflict,
		string(beatsJSON), string(mutationJSON), c.Content, c.WordCount,
		c.BurstinessScore, c.LinterPassed, reviewJSON, c.CreatedAt,
	)
	return err
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
	var protagonistJSON string
	pRow := tx.QueryRowContext(ctx, `SELECT id, title, target_platform, world_rules, protagonist_json, created_at, updated_at FROM projects WHERE id = ?`, projectID)
	if err := pRow.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &protagonistJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, nil, fmt.Errorf("query project: %w", err)
	}
	if err := json.Unmarshal([]byte(protagonistJSON), &p.Protagonist); err != nil {
		return nil, nil, fmt.Errorf("unmarshal protagonist: %w", err)
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

	// 4. Delete the chapter from chapters table
	if _, err := tx.ExecContext(ctx, `DELETE FROM chapters WHERE project_id = ? AND chapter_index = ?`, projectID, chapterIndex); err != nil {
		return nil, nil, fmt.Errorf("delete chapter in tx: %w", err)
	}

	// 5. Rollback protagonist mutations
	rolledBackProtagonist, _, rerr := domain.RollbackStateMutation(p.Protagonist, c.StateMutation)
	if rerr == nil {
		p.Protagonist = rolledBackProtagonist
	}

	p.UpdatedAt = time.Now()
	newProtagonistJSON, _ := json.Marshal(p.Protagonist)
	if _, err := tx.ExecContext(ctx,
		`UPDATE projects SET protagonist_json = ?, updated_at = ? WHERE id = ?`,
		string(newProtagonistJSON), p.UpdatedAt, p.ID,
	); err != nil {
		return nil, nil, fmt.Errorf("update project protagonist in tx: %w", err)
	}

	// 6. Revert resolved plot hooks if any were resolved in this chapter
	if c.Review != nil && len(c.Review.ResolvedHookIDs) > 0 {
		for _, hid := range c.Review.ResolvedHookIDs {
			hid = strings.TrimSpace(hid)
			if hid != "" {
				_, _ = tx.ExecContext(ctx,
					`UPDATE plot_hooks SET status = 'OPEN' WHERE project_id = ? AND (id = ? OR title = ?)`,
					projectID, hid, hid,
				)
			}
		}
	}

	// 7. Delete any codex progressions triggered from this chapter
	_, _ = tx.ExecContext(ctx, `
		DELETE FROM codex_progressions 
		WHERE entry_id IN (SELECT id FROM codex_entries WHERE project_id = ?) 
		  AND active_from_chapter = ?
	`, projectID, chapterIndex)

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}
	tx = nil

	return cp, &p, nil
}

func (s *SQLiteStore) DeleteChapter(ctx context.Context, projectID string, chapterIndex int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chapters WHERE project_id = ? AND chapter_index = ?`, projectID, chapterIndex)
	return err
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
	_, err := s.db.ExecContext(ctx, query,
		h.ID, h.ProjectID, h.Title, h.Details, h.CreatedChapter, h.TargetChapter, string(h.Status), h.CreatedAt,
	)
	return err
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
	_, err := s.db.ExecContext(ctx, `DELETE FROM plot_hooks WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) SaveCheckpoint(ctx context.Context, cp *domain.ChapterCheckpoint) error {
	if cp == nil || cp.ProjectID == "" || cp.ChapterIndex <= 0 {
		return errors.New("invalid checkpoint: project_id and positive chapter_index required")
	}
	cp.UpdatedAt = time.Now()
	payload, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal checkpoint payload: %w", err)
	}

	query := `
	INSERT INTO chapter_checkpoints (project_id, chapter_index, phase, core_conflict, payload_json, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(project_id, chapter_index) DO UPDATE SET
		phase = excluded.phase,
		core_conflict = excluded.core_conflict,
		payload_json = excluded.payload_json,
		updated_at = excluded.updated_at;
	`
	_, err = s.db.ExecContext(ctx, query, cp.ProjectID, cp.ChapterIndex, string(cp.Phase), cp.CoreConflict, string(payload), cp.UpdatedAt)
	return err
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
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM chapter_checkpoints WHERE project_id = ? AND chapter_index = ?`,
		projectID, chapterIndex,
	)
	return err
}

// -------------------------------------------------------------
// The Codex (全域世界观百科) 存储实现
// -------------------------------------------------------------

func (s *SQLiteStore) SaveCodexEntry(ctx context.Context, entry *domain.CodexEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("codex-%d", time.Now().UnixNano())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.UpdatedAt = time.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
	INSERT INTO codex_entries (
		id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
		archetype, voice_tone, core_motivation, current_disposition, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		category = excluded.category,
		name = excluded.name,
		color_tag = excluded.color_tag,
		summary = excluded.summary,
		details_markdown = excluded.details_markdown,
		tracking_mode = excluded.tracking_mode,
		archetype = excluded.archetype,
		voice_tone = excluded.voice_tone,
		core_motivation = excluded.core_motivation,
		current_disposition = excluded.current_disposition,
		updated_at = excluded.updated_at;
	`
	_, err = tx.ExecContext(ctx, query,
		entry.ID, entry.ProjectID, string(entry.Category), entry.Name,
		entry.ColorTag, entry.Summary, entry.DetailsMarkdown, string(entry.TrackingMode),
		entry.Archetype, entry.VoiceTone, entry.CoreMotivation, entry.CurrentDisposition,
		entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert codex entry: %w", err)
	}

	// Update aliases
	if _, err := tx.ExecContext(ctx, `DELETE FROM codex_aliases WHERE entry_id = ?`, entry.ID); err != nil {
		return fmt.Errorf("delete old aliases: %w", err)
	}

	for _, alias := range entry.Aliases {
		aliasClean := strings.TrimSpace(alias)
		if aliasClean == "" || aliasClean == entry.Name {
			continue
		}
		aliasID := fmt.Sprintf("alias-%d-%s", time.Now().UnixNano(), aliasClean)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO codex_aliases (id, entry_id, alias) VALUES (?, ?, ?)`,
			aliasID, entry.ID, aliasClean,
		); err != nil {
			return fmt.Errorf("insert alias %s: %w", aliasClean, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetCodexEntry(ctx context.Context, projectID, id string) (*domain.CodexEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
		       COALESCE(archetype, ''), COALESCE(voice_tone, ''), COALESCE(core_motivation, ''), COALESCE(current_disposition, ''),
		       created_at, updated_at
		FROM codex_entries
		WHERE project_id = ? AND id = ?
	`, projectID, id)

	var e domain.CodexEntry
	var cat, tm, colorTag, summary, details, arch, vt, motiv, disp sql.NullString
	if err := row.Scan(
		&e.ID, &e.ProjectID, &cat, &e.Name, &colorTag, &summary, &details, &tm,
		&arch, &vt, &motiv, &disp,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan codex entry: %w", err)
	}
	e.Category = domain.CodexCategory(cat.String)
	e.TrackingMode = domain.TrackingMode(tm.String)
	e.ColorTag = colorTag.String
	e.Summary = summary.String
	e.DetailsMarkdown = details.String
	e.Archetype = arch.String
	e.VoiceTone = vt.String
	e.CoreMotivation = motiv.String
	e.CurrentDisposition = disp.String

	// Fetch aliases
	aliasRows, err := s.db.QueryContext(ctx, `SELECT alias FROM codex_aliases WHERE entry_id = ? ORDER BY alias ASC`, e.ID)
	if err == nil {
		defer aliasRows.Close()
		for aliasRows.Next() {
			var a string
			if err := aliasRows.Scan(&a); err == nil && a != "" {
				e.Aliases = append(e.Aliases, a)
			}
		}
	}

	// Fetch progressions
	progs, err := s.ListCodexProgressions(ctx, e.ID)
	if err == nil {
		e.Progressions = progs
	}

	// Fetch relations
	rels, err := s.ListCodexRelations(ctx, projectID, e.ID)
	if err == nil {
		e.Relations = rels
	}

	return &e, nil
}

func (s *SQLiteStore) ListCodexEntries(ctx context.Context, projectID string, category domain.CodexCategory) ([]*domain.CodexEntry, error) {
	var query string
	var args []interface{}
	if category != "" {
		query = `
			SELECT id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
			       COALESCE(archetype, ''), COALESCE(voice_tone, ''), COALESCE(core_motivation, ''), COALESCE(current_disposition, ''),
			       created_at, updated_at
			FROM codex_entries
			WHERE project_id = ? AND category = ?
			ORDER BY name ASC
		`
		args = []interface{}{projectID, string(category)}
	} else {
		query = `
			SELECT id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
			       COALESCE(archetype, ''), COALESCE(voice_tone, ''), COALESCE(core_motivation, ''), COALESCE(current_disposition, ''),
			       created_at, updated_at
			FROM codex_entries
			WHERE project_id = ?
			ORDER BY category ASC, name ASC
		`
		args = []interface{}{projectID}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list codex entries: %w", err)
	}
	defer rows.Close()

	var entries []*domain.CodexEntry
	entryMap := make(map[string]*domain.CodexEntry)
	for rows.Next() {
		var e domain.CodexEntry
		var cat, tm, colorTag, summary, details, arch, vt, motiv, disp sql.NullString
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &cat, &e.Name, &colorTag, &summary, &details, &tm,
			&arch, &vt, &motiv, &disp,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan codex row: %w", err)
		}
		e.Category = domain.CodexCategory(cat.String)
		e.TrackingMode = domain.TrackingMode(tm.String)
		e.ColorTag = colorTag.String
		e.Summary = summary.String
		e.DetailsMarkdown = details.String
		e.Archetype = arch.String
		e.VoiceTone = vt.String
		e.CoreMotivation = motiv.String
		e.CurrentDisposition = disp.String
		entries = append(entries, &e)
		entryMap[e.ID] = &e
	}

	if len(entries) == 0 {
		return entries, nil
	}

	// Batch load aliases
	aliasRows, err := s.db.QueryContext(ctx, `
		SELECT a.entry_id, a.alias
		FROM codex_aliases a
		JOIN codex_entries e ON a.entry_id = e.id
		WHERE e.project_id = ?
	`, projectID)
	if err == nil {
		defer aliasRows.Close()
		for aliasRows.Next() {
			var entryID, alias string
			if err := aliasRows.Scan(&entryID, &alias); err == nil {
				if ent, ok := entryMap[entryID]; ok && alias != "" {
					ent.Aliases = append(ent.Aliases, alias)
				}
			}
		}
	}

	// Batch load progressions
	progRows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.entry_id, p.active_from_chapter, p.state_payload_json, COALESCE(p.notes, ''), p.created_at
		FROM codex_progressions p
		JOIN codex_entries e ON p.entry_id = e.id
		WHERE e.project_id = ?
		ORDER BY p.active_from_chapter ASC
	`, projectID)
	if err == nil {
		defer progRows.Close()
		for progRows.Next() {
			var p domain.Progression
			if err := progRows.Scan(&p.ID, &p.EntryID, &p.ActiveFromChapter, &p.StatePayloadJSON, &p.Notes, &p.CreatedAt); err == nil {
				if ent, ok := entryMap[p.EntryID]; ok {
					ent.Progressions = append(ent.Progressions, p)
				}
			}
		}
	}

	return entries, nil
}

func (s *SQLiteStore) DeleteCodexEntry(ctx context.Context, projectID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM codex_entries WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		return fmt.Errorf("delete codex entry: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("codex entry not found")
	}
	return nil
}

func (s *SQLiteStore) SaveCodexProgression(ctx context.Context, entryID string, prog *domain.Progression) error {
	if prog.ID == "" {
		prog.ID = fmt.Sprintf("prog-%d", time.Now().UnixNano())
	}
	prog.EntryID = entryID
	if prog.CreatedAt.IsZero() {
		prog.CreatedAt = time.Now()
	}

	query := `
	INSERT INTO codex_progressions (id, entry_id, active_from_chapter, state_payload_json, notes, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		active_from_chapter = excluded.active_from_chapter,
		state_payload_json = excluded.state_payload_json,
		notes = excluded.notes;
	`
	_, err := s.db.ExecContext(ctx, query,
		prog.ID, prog.EntryID, prog.ActiveFromChapter, prog.StatePayloadJSON, prog.Notes, prog.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) ListCodexProgressions(ctx context.Context, entryID string) ([]domain.Progression, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entry_id, active_from_chapter, state_payload_json, COALESCE(notes, ''), created_at
		FROM codex_progressions
		WHERE entry_id = ?
		ORDER BY active_from_chapter ASC
	`, entryID)
	if err != nil {
		return nil, fmt.Errorf("query progressions: %w", err)
	}
	defer rows.Close()

	var progs []domain.Progression
	for rows.Next() {
		var p domain.Progression
		if err := rows.Scan(&p.ID, &p.EntryID, &p.ActiveFromChapter, &p.StatePayloadJSON, &p.Notes, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan progression: %w", err)
		}
		progs = append(progs, p)
	}
	return progs, nil
}

func (s *SQLiteStore) SaveCodexRelation(ctx context.Context, projectID string, rel *domain.EntityRelation) error {
	if rel.ID == "" {
		rel.ID = fmt.Sprintf("rel-%d", time.Now().UnixNano())
	}
	rel.ProjectID = projectID
	if rel.CreatedAt.IsZero() {
		rel.CreatedAt = time.Now()
	}

	query := `
	INSERT INTO codex_relations (id, project_id, source_entry_id, target_entry_id, relation_type, description, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		relation_type = excluded.relation_type,
		description = excluded.description;
	`
	_, err := s.db.ExecContext(ctx, query,
		rel.ID, rel.ProjectID, rel.SourceEntryID, rel.TargetEntryID, rel.RelationType, rel.Description, rel.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) ListCodexRelations(ctx context.Context, projectID string, entryID string) ([]domain.EntityRelation, error) {
	var query string
	var args []interface{}
	if entryID != "" {
		query = `
			SELECT r.id, r.project_id, r.source_entry_id, r.target_entry_id, COALESCE(t.name, ''), r.relation_type, COALESCE(r.description, ''), r.created_at
			FROM codex_relations r
			LEFT JOIN codex_entries t ON r.target_entry_id = t.id
			WHERE r.project_id = ? AND r.source_entry_id = ?
			ORDER BY r.created_at ASC
		`
		args = []interface{}{projectID, entryID}
	} else {
		query = `
			SELECT r.id, r.project_id, r.source_entry_id, r.target_entry_id, COALESCE(t.name, ''), r.relation_type, COALESCE(r.description, ''), r.created_at
			FROM codex_relations r
			LEFT JOIN codex_entries t ON r.target_entry_id = t.id
			WHERE r.project_id = ?
			ORDER BY r.created_at ASC
		`
		args = []interface{}{projectID}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	var rels []domain.EntityRelation
	for rows.Next() {
		var r domain.EntityRelation
		if err := rows.Scan(
			&r.ID, &r.ProjectID, &r.SourceEntryID, &r.TargetEntryID, &r.TargetName, &r.RelationType, &r.Description, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		rels = append(rels, r)
	}
	return rels, nil
}

func (s *SQLiteStore) DeleteCodexRelation(ctx context.Context, projectID string, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM codex_relations WHERE project_id = ? AND id = ?`, projectID, id)
	return err
}

// -------------------------------------------------------------
// The Matrix (场景场次与矩阵大纲) 存储实现
// -------------------------------------------------------------

func (s *SQLiteStore) SaveScene(ctx context.Context, sc *domain.Scene) error {
	if err := sc.Validate(); err != nil {
		return err
	}
	if sc.ID == "" {
		sc.ID = fmt.Sprintf("sc-%d", time.Now().UnixNano())
	}
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = time.Now()
	}
	sc.UpdatedAt = time.Now()

	beatsJSON, _ := json.Marshal(sc.Beats)
	stateMutJSON, _ := json.Marshal(sc.StateMutation)
	if sc.WordCount <= 0 && sc.ProseContent != "" {
		sc.WordCount = len([]rune(sc.ProseContent))
	}

	query := `
	INSERT INTO scenes (
		id, chapter_id, project_id, scene_index, title, location_entry_id, dramatic_goal, conflict_barrier,
		tension_level, beats_json, state_mutation_json, prose_content, word_count, exclude_from_ai, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		chapter_id = excluded.chapter_id,
		scene_index = excluded.scene_index,
		title = excluded.title,
		location_entry_id = excluded.location_entry_id,
		dramatic_goal = excluded.dramatic_goal,
		conflict_barrier = excluded.conflict_barrier,
		tension_level = excluded.tension_level,
		beats_json = excluded.beats_json,
		state_mutation_json = excluded.state_mutation_json,
		prose_content = excluded.prose_content,
		word_count = excluded.word_count,
		exclude_from_ai = excluded.exclude_from_ai,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query,
		sc.ID, sc.ChapterID, sc.ProjectID, sc.SceneIndex, sc.Title, sc.LocationEntryID,
		sc.DramaticGoal, sc.ConflictBarrier, sc.TensionLevel, string(beatsJSON), string(stateMutJSON),
		sc.ProseContent, sc.WordCount, sc.ExcludeFromAI, sc.CreatedAt, sc.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetScene(ctx context.Context, id string) (*domain.Scene, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chapter_id, project_id, scene_index, title, location_entry_id, dramatic_goal, conflict_barrier,
		       tension_level, beats_json, state_mutation_json, prose_content, word_count, exclude_from_ai, created_at, updated_at
		FROM scenes WHERE id = ?
	`, id)

	var sc domain.Scene
	var title, loc, goal, barrier, prose sql.NullString
	var beatsStr, stateMutStr string
	if err := row.Scan(
		&sc.ID, &sc.ChapterID, &sc.ProjectID, &sc.SceneIndex, &title, &loc, &goal, &barrier,
		&sc.TensionLevel, &beatsStr, &stateMutStr, &prose, &sc.WordCount, &sc.ExcludeFromAI, &sc.CreatedAt, &sc.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan scene: %w", err)
	}
	sc.Title = title.String
	sc.LocationEntryID = loc.String
	sc.DramaticGoal = goal.String
	sc.ConflictBarrier = barrier.String
	sc.ProseContent = prose.String
	_ = json.Unmarshal([]byte(beatsStr), &sc.Beats)
	_ = json.Unmarshal([]byte(stateMutStr), &sc.StateMutation)

	markers, err := s.ListSceneMarkers(ctx, sc.ID)
	if err == nil {
		sc.Markers = markers
	}

	return &sc, nil
}

func (s *SQLiteStore) ListScenes(ctx context.Context, chapterID string) ([]*domain.Scene, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chapter_id, project_id, scene_index, title, location_entry_id, dramatic_goal, conflict_barrier,
		       tension_level, beats_json, state_mutation_json, prose_content, word_count, exclude_from_ai, created_at, updated_at
		FROM scenes WHERE chapter_id = ? ORDER BY scene_index ASC
	`, chapterID)
	if err != nil {
		return nil, fmt.Errorf("list scenes: %w", err)
	}
	defer rows.Close()

	var scenes []*domain.Scene
	for rows.Next() {
		var sc domain.Scene
		var title, loc, goal, barrier, prose sql.NullString
		var beatsStr, stateMutStr string
		if err := rows.Scan(
			&sc.ID, &sc.ChapterID, &sc.ProjectID, &sc.SceneIndex, &title, &loc, &goal, &barrier,
			&sc.TensionLevel, &beatsStr, &stateMutStr, &prose, &sc.WordCount, &sc.ExcludeFromAI, &sc.CreatedAt, &sc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scene row: %w", err)
		}
		sc.Title = title.String
		sc.LocationEntryID = loc.String
		sc.DramaticGoal = goal.String
		sc.ConflictBarrier = barrier.String
		sc.ProseContent = prose.String
		_ = json.Unmarshal([]byte(beatsStr), &sc.Beats)
		_ = json.Unmarshal([]byte(stateMutStr), &sc.StateMutation)
		scenes = append(scenes, &sc)
	}
	return scenes, nil
}

func (s *SQLiteStore) DeleteScene(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scenes WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) SaveSceneMarker(ctx context.Context, m *domain.SceneMarker) error {
	if m.ID == "" {
		m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}

	query := `
	INSERT INTO scene_markers (id, scene_id, marker_type, color, text_range_start, text_range_end, content, resolved, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		marker_type = excluded.marker_type,
		color = excluded.color,
		text_range_start = excluded.text_range_start,
		text_range_end = excluded.text_range_end,
		content = excluded.content,
		resolved = excluded.resolved;
	`
	_, err := s.db.ExecContext(ctx, query,
		m.ID, m.SceneID, string(m.MarkerType), m.Color, m.TextRangeStart, m.TextRangeEnd, m.Content, m.Resolved, m.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) ListSceneMarkers(ctx context.Context, sceneID string) ([]domain.SceneMarker, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scene_id, marker_type, color, text_range_start, text_range_end, content, resolved, created_at
		FROM scene_markers WHERE scene_id = ? ORDER BY text_range_start ASC
	`, sceneID)
	if err != nil {
		return nil, fmt.Errorf("list markers: %w", err)
	}
	defer rows.Close()

	var markers []domain.SceneMarker
	for rows.Next() {
		var m domain.SceneMarker
		var mt string
		if err := rows.Scan(
			&m.ID, &m.SceneID, &mt, &m.Color, &m.TextRangeStart, &m.TextRangeEnd, &m.Content, &m.Resolved, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan marker: %w", err)
		}
		m.MarkerType = domain.MarkerType(mt)
		markers = append(markers, m)
	}
	return markers, nil
}

func (s *SQLiteStore) DeleteSceneMarker(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scene_markers WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) GetMatrixOverview(ctx context.Context, projectID string) (*domain.MatrixOverview, error) {
	proj, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	chapters, err := s.ListChapters(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}

	overview := &domain.MatrixOverview{
		ProjectID: projectID,
	}

	chapterRows := make([]*domain.MatrixChapterRow, 0, len(chapters))
	totalWords := 0
	totalScenes := 0

	for _, ch := range chapters {
		scenes, err := s.ListScenes(ctx, ch.ID)
		if err != nil {
			scenes = []*domain.Scene{}
		}
		chWords := ch.WordCount
		for _, sc := range scenes {
			chWords += sc.WordCount
			totalScenes++
		}
		totalWords += chWords
		chapterRows = append(chapterRows, &domain.MatrixChapterRow{
			Chapter: ch,
			Scenes:  scenes,
		})
	}
	overview.TotalWords = totalWords
	overview.TotalScenes = totalScenes

	if proj.Framework != nil && len(proj.Framework.VolumeArcs) > 0 {
		startIdx := 1
		for _, arc := range proj.Framework.VolumeArcs {
			est := arc.EstimatedChapters
			if est <= 0 {
				est = 30
			}
			endIdx := startIdx + est - 1

			group := &domain.MatrixVolumeGroup{
				VolumeIndex: arc.VolumeIndex,
				Title:       arc.Title,
				Theme:       arc.Theme,
				CoreGoal:    arc.CoreGoal,
				Climax:      arc.Climax,
			}

			var volWords int
			var tensionSum int
			var sceneCount int

			for _, row := range chapterRows {
				if row.Chapter.ChapterIndex >= startIdx && row.Chapter.ChapterIndex <= endIdx {
					group.Chapters = append(group.Chapters, row)
					volWords += row.Chapter.WordCount
					for _, sc := range row.Scenes {
						tensionSum += sc.TensionLevel
						sceneCount++
					}
				}
			}

			group.TotalWords = volWords
			if sceneCount > 0 {
				group.AvgTension = float64(tensionSum) / float64(sceneCount)
			}
			overview.Volumes = append(overview.Volumes, group)
			startIdx = endIdx + 1
		}
	} else {
		defVol := &domain.MatrixVolumeGroup{
			VolumeIndex: 1,
			Title:       "第一卷",
			Chapters:    chapterRows,
			TotalWords:  totalWords,
		}
		overview.Volumes = append(overview.Volumes, defVol)
	}

	return overview, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
