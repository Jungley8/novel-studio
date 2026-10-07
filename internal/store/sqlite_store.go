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
	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set pragma failed: %w", err)
	}

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
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Backward compatible schema patch for existing DBs
	_, _ = s.db.Exec(`ALTER TABLE chapters ADD COLUMN review_json TEXT;`)
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

	query := `
	INSERT INTO projects (id, title, target_platform, world_rules, protagonist_json, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		target_platform = excluded.target_platform,
		world_rules = excluded.world_rules,
		protagonist_json = excluded.protagonist_json,
		updated_at = excluded.updated_at;
	`
	_, err = s.db.ExecContext(ctx, query,
		p.ID, p.Title, p.TargetPlatform, p.WorldRules, string(protagonistJSON), p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	query := `SELECT id, title, target_platform, world_rules, protagonist_json, created_at, updated_at FROM projects WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var p domain.Project
	var protagonistJSON string
	if err := row.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &protagonistJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(protagonistJSON), &p.Protagonist); err != nil {
		return nil, fmt.Errorf("unmarshal protagonist: %w", err)
	}
	return &p, nil
}

func (s *SQLiteStore) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	query := `SELECT id, title, target_platform, world_rules, protagonist_json, created_at, updated_at FROM projects ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Project
	for rows.Next() {
		var p domain.Project
		var protagonistJSON string
		if err := rows.Scan(&p.ID, &p.Title, &p.TargetPlatform, &p.WorldRules, &protagonistJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(protagonistJSON), &p.Protagonist)
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

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
