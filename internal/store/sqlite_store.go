package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
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

	CREATE TABLE IF NOT EXISTS chapter_intents (
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		volume_index INTEGER NOT NULL DEFAULT 0,
		intent_json TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		PRIMARY KEY(project_id, chapter_index),
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_chapters_project ON chapters(project_id, chapter_index);
	CREATE INDEX IF NOT EXISTS idx_hooks_project ON plot_hooks(project_id, status);
	CREATE INDEX IF NOT EXISTS idx_checkpoints_project ON chapter_checkpoints(project_id, chapter_index);
	CREATE INDEX IF NOT EXISTS idx_chapter_intents_proj_vol ON chapter_intents(project_id, volume_index);

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
		realm TEXT DEFAULT '',
		inventory TEXT DEFAULT '',
		health_status TEXT DEFAULT '',
		structured_level_json TEXT DEFAULT '',
		structured_items_json TEXT DEFAULT '',
		alignment TEXT DEFAULT '',
		threat_level TEXT DEFAULT '',
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

	-- project_events has intentionally NO foreign key to projects: the ledger
	-- is a tombstone history and must survive project deletion.
	CREATE TABLE IF NOT EXISTS project_events (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		seq INTEGER NOT NULL,
		run_id TEXT NOT NULL DEFAULT '',
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		chapter_index INTEGER DEFAULT 0,
		event_type TEXT NOT NULL,
		actor TEXT NOT NULL,
		source TEXT NOT NULL,
		title TEXT NOT NULL,
		summary TEXT,
		excerpt_before TEXT,
		excerpt_after TEXT,
		before_blob BLOB,
		after_blob BLOB,
		codec TEXT NOT NULL DEFAULT 'none',
		fields_changed TEXT,
		chars_added INTEGER DEFAULT 0,
		chars_removed INTEGER DEFAULT 0,
		token_usage TEXT,
		created_at TIMESTAMP NOT NULL
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_events_proj_seq ON project_events(project_id, seq);
	CREATE INDEX IF NOT EXISTS idx_events_entity ON project_events(project_id, entity_type, entity_id);
	CREATE INDEX IF NOT EXISTS idx_events_chapter ON project_events(project_id, chapter_index);
	CREATE INDEX IF NOT EXISTS idx_events_run ON project_events(project_id, run_id);

	CREATE TABLE IF NOT EXISTS narrative_forecasts (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		base_chapter TEXT NOT NULL,
		divergence TEXT NOT NULL,
		branch_count INTEGER NOT NULL,
		horizon INTEGER NOT NULL,
		fingerprint TEXT NOT NULL,
		status TEXT NOT NULL,
		branches_json TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_forecasts_project ON narrative_forecasts(project_id, status);

	CREATE TABLE IF NOT EXISTS project_active_plan (
		project_id TEXT PRIMARY KEY,
		forecast_id TEXT NOT NULL,
		branch_id TEXT NOT NULL,
		plan_text TEXT NOT NULL,
		status TEXT NOT NULL,
		plan_chapters_json TEXT DEFAULT '',
		expected_deltas TEXT DEFAULT '',
		warning_message TEXT DEFAULT '',
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
		FOREIGN KEY(forecast_id) REFERENCES narrative_forecasts(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS chapter_versions (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		source TEXT NOT NULL,
		word_count INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chapter_state_snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		protagonist_json TEXT NOT NULL,
		hooks_json TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_chapter_versions_proj_chap ON chapter_versions(project_id, chapter_index, created_at);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_chapter_state_snapshots_proj_chap ON chapter_state_snapshots(project_id, chapter_index);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Backward compatible schema patch for existing DBs
	_, _ = s.db.Exec(`ALTER TABLE project_active_plan ADD COLUMN warning_message TEXT DEFAULT '';`)
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
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN realm TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN inventory TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN health_status TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN structured_level_json TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN structured_items_json TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN alignment TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE codex_entries ADD COLUMN threat_level TEXT DEFAULT '';`)

	// One-time data migration from legacy projects.protagonist_json into codex_entries
	var colCount int
	_ = s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('projects') WHERE name = 'protagonist_json'`).Scan(&colCount)
	if colCount > 0 {
		type legacyRow struct {
			pid   string
			pjson string
		}
		var legRows []legacyRow
		rows, err := s.db.Query(`SELECT id, protagonist_json FROM projects WHERE protagonist_json IS NOT NULL AND protagonist_json != ''`)
		if err == nil {
			for rows.Next() {
				var pid, pjson string
				if err := rows.Scan(&pid, &pjson); err == nil && pjson != "" {
					legRows = append(legRows, legacyRow{pid: pid, pjson: pjson})
				}
			}
			_ = rows.Close()

			for _, lr := range legRows {
				var checkCount int
				_ = s.db.QueryRow(`SELECT count(*) FROM codex_entries WHERE project_id = ? AND archetype = 'PROTAGONIST'`, lr.pid).Scan(&checkCount)
				if checkCount == 0 {
					var legacy struct {
						NameAndLevel    string `json:"name_and_level"`
						Inventory       string `json:"inventory"`
						CoreGoal        string `json:"core_goal"`
						HealthStatus    string `json:"health_status"`
						StructuredLevel any    `json:"structured_level"`
						StructuredItems any    `json:"structured_items"`
					}
					if uerr := json.Unmarshal([]byte(lr.pjson), &legacy); uerr == nil {
						name := legacy.NameAndLevel
						realm := ""
						if parts := strings.Split(legacy.NameAndLevel, "("); len(parts) > 1 {
							name = strings.TrimSpace(parts[0])
							realm = strings.TrimSpace(strings.TrimSuffix(parts[1], ")"))
						} else if parts := strings.Split(legacy.NameAndLevel, "（"); len(parts) > 1 {
							name = strings.TrimSpace(parts[0])
							realm = strings.TrimSpace(strings.TrimSuffix(parts[1], "）"))
						}
						if name == "" {
							name = "主角"
						}
						levelBytes, _ := json.Marshal(legacy.StructuredLevel)
						itemsBytes, _ := json.Marshal(legacy.StructuredItems)
						eid := fmt.Sprintf("codex_%s_pro", lr.pid)
						_, _ = s.db.Exec(`
							INSERT INTO codex_entries (
								id, project_id, category, name, color_tag, summary, details_markdown,
								tracking_mode, archetype, voice_tone, core_motivation, current_disposition,
								realm, inventory, health_status, structured_level_json, structured_items_json,
								created_at, updated_at
							) VALUES (?, ?, 'CHARACTER', ?, '#fbbf24', ?, ?, 'ALWAYS_INJECT', 'PROTAGONIST', '', ?, '', ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
							ON CONFLICT(id) DO NOTHING
						`, eid, lr.pid, name, "全书核心主角", "【全书核心主角档案】", legacy.CoreGoal, realm, legacy.Inventory, legacy.HealthStatus, string(levelBytes), string(itemsBytes))
					}
				}
			}
		}
	}

	// One-time data migration from legacy projects.framework_json.factions into codex_entries
	type frameworkRow struct {
		pid   string
		fjson string
	}
	var fRows []frameworkRow
	fQueryRows, err := s.db.Query(`SELECT id, framework_json FROM projects WHERE framework_json IS NOT NULL AND framework_json != ''`)
	if err == nil {
		for fQueryRows.Next() {
			var pid, fjson string
			if err := fQueryRows.Scan(&pid, &fjson); err == nil && fjson != "" {
				fRows = append(fRows, frameworkRow{pid: pid, fjson: fjson})
			}
		}
		_ = fQueryRows.Close()

		for _, fr := range fRows {
			var fw struct {
				Factions []domain.Faction `json:"factions"`
			}
			if err := json.Unmarshal([]byte(fr.fjson), &fw); err == nil && len(fw.Factions) > 0 {
				for i, fac := range fw.Factions {
					if strings.TrimSpace(fac.Name) == "" {
						continue
					}
					var checkCount int
					_ = s.db.QueryRow(`SELECT count(*) FROM codex_entries WHERE project_id = ? AND category = 'FACTION' AND name = ?`, fr.pid, fac.Name).Scan(&checkCount)
					if checkCount == 0 {
						entryID := fmt.Sprintf("codex_%s_fac_%d", fr.pid, time.Now().UnixNano()+int64(i))
						_, _ = s.db.Exec(`
							INSERT INTO codex_entries (
								id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
								archetype, voice_tone, core_motivation, current_disposition, realm, inventory, health_status,
								structured_level_json, structured_items_json, alignment, threat_level, created_at, updated_at
							) VALUES (?, ?, 'FACTION', ?, '', ?, ?, 'AUTO_MENTION', '', '', '', '', '', '', '', '', '', ?, ?, datetime('now'), datetime('now'))
							ON CONFLICT(id) DO NOTHING
						`, entryID, fr.pid, fac.Name, fmt.Sprintf("立场: %s | 威胁度: %s", fac.Alignment, fac.ThreatLevel), fac.Doctrine, fac.Alignment, fac.ThreatLevel)
					}
				}
			}
		}
	}
	return nil
}

// -------------------------------------------------------------
// Activity Ledger (内容生产与变更事件账本) 写入与查询基础设施
// -------------------------------------------------------------

// execer is satisfied by both *sql.DB and *sql.Tx so state mutations and
// their ledger events always share one transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// newEvent fills authorship attribution from ctx (actor/source/run_id).
func newEvent(ctx context.Context, projectID, entityType, entityID string, chapterIndex int, eventType, title, summary string) *activity.ActivityEvent {
	return &activity.ActivityEvent{
		ID:           activity.NewID(),
		ProjectID:    projectID,
		RunID:        activity.RunIDFromCtx(ctx),
		EntityType:   entityType,
		EntityID:     entityID,
		ChapterIndex: chapterIndex,
		EventType:    eventType,
		Actor:        activity.ActorFromCtx(ctx),
		Source:       activity.SourceFromCtx(ctx),
		Title:        title,
		Summary:      summary,
		CreatedAt:    time.Now(),
	}
}

// withTextSnapshots attaches full before/after snapshots plus the plain-text
// list-view signals (excerpts, char deltas). Large payloads are gzipped.
func withTextSnapshots(e *activity.ActivityEvent, before, after string, fieldsChanged []string) {
	added, removed := activity.CharsDelta(before, after)
	e.ExcerptBefore = activity.Excerpt(before, activity.ExcerptRunes)
	e.ExcerptAfter = activity.Excerpt(after, activity.ExcerptRunes)
	e.Before = before
	e.After = after
	e.FieldsChanged = fieldsChanged
	e.CharsAdded = added
	e.CharsRemoved = removed
}

// insertEvent persists one ledger record. Seq is allocated by an in-statement
// scalar subquery so the write is atomic even for non-transactional callers;
// inside an explicit tx it joins that tx for all-or-nothing guarantees.
func insertEvent(ctx context.Context, ex execer, e *activity.ActivityEvent) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if e.ID == "" {
		e.ID = activity.NewID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.Codec == "" {
		e.Codec = activity.CodecNone
	}

	beforeBlob, beforeCodec, err := activity.Encode([]byte(e.Before))
	if err != nil {
		return fmt.Errorf("encode event before snapshot: %w", err)
	}
	afterBlob, afterCodec, err := activity.Encode([]byte(e.After))
	if err != nil {
		return fmt.Errorf("encode event after snapshot: %w", err)
	}
	// One codec per row: if either side needed gzip, force it on the other
	// non-empty side as well.
	codec := activity.CodecNone
	if beforeCodec == activity.CodecGzip || afterCodec == activity.CodecGzip {
		codec = activity.CodecGzip
		if beforeBlob, err = activity.EncodeForced([]byte(e.Before), codec); err != nil {
			return fmt.Errorf("encode event before snapshot: %w", err)
		}
		if afterBlob, err = activity.EncodeForced([]byte(e.After), codec); err != nil {
			return fmt.Errorf("encode event after snapshot: %w", err)
		}
	}

	fieldsChangedJSON := ""
	if len(e.FieldsChanged) > 0 {
		if fb, merr := json.Marshal(e.FieldsChanged); merr == nil {
			fieldsChangedJSON = string(fb)
		}
	}

	query := `
	INSERT INTO project_events (
		id, project_id, seq, run_id, entity_type, entity_id, chapter_index,
		event_type, actor, source, title, summary, excerpt_before, excerpt_after,
		before_blob, after_blob, codec, fields_changed, chars_added, chars_removed,
		token_usage, created_at
	) VALUES (
		?, ?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM project_events WHERE project_id = ?),
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	);
	`
	_, err = ex.ExecContext(ctx, query,
		e.ID, e.ProjectID, e.ProjectID,
		e.RunID, e.EntityType, e.EntityID, e.ChapterIndex,
		e.EventType, e.Actor, e.Source, e.Title, e.Summary,
		e.ExcerptBefore, e.ExcerptAfter,
		beforeBlob, afterBlob, codec, fieldsChangedJSON,
		e.CharsAdded, e.CharsRemoved, e.TokenUsage, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert activity event: %w", err)
	}
	return nil
}

// AppendEvent records an explicitly built event (workshop aggregates,
// inline actions, bootstrap markers). Store-layer CRUD emits its own events
// internally; callers must not double-report the same mutation.
func (s *SQLiteStore) AppendEvent(ctx context.Context, e *activity.ActivityEvent) error {
	if e == nil {
		return errors.New("activity event is nil")
	}
	return insertEvent(ctx, s.db, e)
}

// emitLedger records an auto-emitted CRUD event inside the caller's tx.
// Unlike insertEvent it never fails the enclosing write: when the mutation
// cannot be attributed to a project (empty id bypassing the project-scoped
// routes), the state change still commits and the ledger simply skips a row
// it could not describe. The ledger is secondary; the mutation is primary.
func emitLedger(ctx context.Context, tx *sql.Tx, e *activity.ActivityEvent) error {
	if e == nil || strings.TrimSpace(e.ProjectID) == "" {
		return nil
	}
	return insertEvent(ctx, tx, e)
}

// ListEvents returns the timeline newest-first without decompressing blobs:
// Before/After stay empty, excerpts carry the preview.
func (s *SQLiteStore) ListEvents(ctx context.Context, q activity.EventQuery) ([]*activity.ActivityEvent, int64, error) {
	if strings.TrimSpace(q.ProjectID) == "" {
		return nil, 0, errors.New("list events requires project_id")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var sb strings.Builder
	sb.WriteString(`
	SELECT id, project_id, seq, run_id, entity_type, entity_id, chapter_index,
	       event_type, actor, source, title, COALESCE(summary, ''),
	       COALESCE(excerpt_before, ''), COALESCE(excerpt_after, ''),
	       COALESCE(fields_changed, ''), chars_added, chars_removed,
	       COALESCE(token_usage, ''), created_at
	FROM project_events WHERE project_id = ?
	`)
	args := []any{q.ProjectID}
	if q.EntityType != "" {
		sb.WriteString(` AND entity_type = ?`)
		args = append(args, q.EntityType)
	}
	if q.EntityID != "" {
		sb.WriteString(` AND entity_id = ?`)
		args = append(args, q.EntityID)
	}
	if q.ChapterIndex > 0 {
		sb.WriteString(` AND chapter_index = ?`)
		args = append(args, q.ChapterIndex)
	}
	if q.EventTypePrefix != "" {
		sb.WriteString(` AND event_type LIKE ?`)
		args = append(args, q.EventTypePrefix+"%")
	}
	if !q.IncludeCheckpoint {
		sb.WriteString(` AND event_type NOT LIKE 'checkpoint.%'`)
	}
	if q.Cursor > 0 {
		sb.WriteString(` AND seq < ?`)
		args = append(args, q.Cursor)
	}
	sb.WriteString(` ORDER BY seq DESC LIMIT ?`)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var list []*activity.ActivityEvent
	for rows.Next() {
		var e activity.ActivityEvent
		var fieldsChangedJSON, tokenUsage string
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.Seq, &e.RunID, &e.EntityType, &e.EntityID,
			&e.ChapterIndex, &e.EventType, &e.Actor, &e.Source, &e.Title, &e.Summary,
			&e.ExcerptBefore, &e.ExcerptAfter, &fieldsChangedJSON,
			&e.CharsAdded, &e.CharsRemoved, &tokenUsage, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan event row: %w", err)
		}
		if fieldsChangedJSON != "" {
			_ = json.Unmarshal([]byte(fieldsChangedJSON), &e.FieldsChanged)
		}
		if tokenUsage != "" {
			e.TokenUsage = tokenUsage
		}
		list = append(list, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}

	var nextCursor int64
	if len(list) > limit {
		list = list[:limit]
		nextCursor = list[len(list)-1].Seq
	}
	return list, nextCursor, nil
}

// GetEvent returns one event with full before/after snapshots decoded.
func (s *SQLiteStore) GetEvent(ctx context.Context, projectID, eventID string) (*activity.ActivityEvent, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(eventID) == "" {
		return nil, errors.New("get event requires project_id and event id")
	}
	row := s.db.QueryRowContext(ctx, `
	SELECT id, project_id, seq, run_id, entity_type, entity_id, chapter_index,
	       event_type, actor, source, title, COALESCE(summary, ''),
	       COALESCE(excerpt_before, ''), COALESCE(excerpt_after, ''),
	       before_blob, after_blob, codec,
	       COALESCE(fields_changed, ''), chars_added, chars_removed,
	       COALESCE(token_usage, ''), created_at
	FROM project_events WHERE project_id = ? AND id = ?
	`, projectID, eventID)

	var e activity.ActivityEvent
	var fieldsChangedJSON, tokenUsage string
	var beforeBlob, afterBlob []byte
	if err := row.Scan(
		&e.ID, &e.ProjectID, &e.Seq, &e.RunID, &e.EntityType, &e.EntityID,
		&e.ChapterIndex, &e.EventType, &e.Actor, &e.Source, &e.Title, &e.Summary,
		&e.ExcerptBefore, &e.ExcerptAfter,
		&beforeBlob, &afterBlob, &e.Codec,
		&fieldsChangedJSON, &e.CharsAdded, &e.CharsRemoved,
		&tokenUsage, &e.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("activity event not found")
		}
		return nil, fmt.Errorf("scan event: %w", err)
	}
	if before, derr := activity.Decode(beforeBlob, e.Codec); derr != nil {
		return nil, fmt.Errorf("decode event before snapshot: %w", derr)
	} else if len(before) > 0 {
		e.Before = string(before)
	}
	if after, derr := activity.Decode(afterBlob, e.Codec); derr != nil {
		return nil, fmt.Errorf("decode event after snapshot: %w", derr)
	} else if len(after) > 0 {
		e.After = string(after)
	}
	if fieldsChangedJSON != "" {
		_ = json.Unmarshal([]byte(fieldsChangedJSON), &e.FieldsChanged)
	}
	if tokenUsage != "" {
		e.TokenUsage = tokenUsage
	}
	return &e, nil
}

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

	e := newEvent(ctx, p.ID, activity.EntityProject, p.ID, 0, eventType, title, "")
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

	protagonist, _ := s.getProtagonistTx(ctx, tx, projectID)
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

// ErrNonLatestChapter is returned when uncommitting a chapter that is not the latest committed chapter.
var ErrNonLatestChapter = errors.New("cannot uncommit non-latest chapter")

// NonLatestChapterError records details when attempting to uncommit a non-latest chapter.
type NonLatestChapterError struct {
	ChapterIndex int
	MaxChapter   int
	Downstream   []int
}

func (e *NonLatestChapterError) Error() string {
	var downstreamStr string
	if len(e.Downstream) == 1 {
		downstreamStr = fmt.Sprintf("%d", e.Downstream[0])
	} else if len(e.Downstream) > 1 {
		downstreamStr = fmt.Sprintf("%d..%d", e.Downstream[0], e.Downstream[len(e.Downstream)-1])
	}
	return fmt.Sprintf("cannot uncommit chapter %d: chapters %s exist; uncommit chapter %d first or rewrite from latest",
		e.ChapterIndex, downstreamStr, e.MaxChapter)
}

func (e *NonLatestChapterError) Unwrap() error {
	return ErrNonLatestChapter
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
						core_motivation = ?,
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
					snapProt.CoreMotivation,
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
			if _, err := rollbackStateMutation(protagonist, c.StateMutation); err != nil {
				return nil, nil, fmt.Errorf("rollback protagonist mutation: %w", err)
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
	_, _ = tx.ExecContext(ctx, `
		DELETE FROM codex_progressions 
		WHERE entry_id IN (SELECT id FROM codex_entries WHERE project_id = ?) 
		  AND active_from_chapter = ?
	`, projectID, chapterIndex)

	// 8. Ledger: chapter.uncommitted keeps the full withdrawn draft inspectable.
	beforeBytes, _ := json.Marshal(c)
	uncommitEvent := newEvent(ctx, projectID, activity.EntityChapter, c.ID, chapterIndex,
		activity.EventChapterUncommitted, fmt.Sprintf("第 %d 章已撤回为草稿", chapterIndex),
		fmt.Sprintf("正文 %d 字回退至断点，伏笔回收已还原", len([]rune(c.Content))))
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

	// Before snapshot for created-vs-updated detection.
	var beforeJSON string
	existed := false
	if entry.ID != "" {
		if old, oerr := s.GetCodexEntry(ctx, entry.ProjectID, entry.ID); oerr == nil && old != nil {
			existed = true
			if ob, merr := json.Marshal(old); merr == nil {
				beforeJSON = string(ob)
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var levelJSON, itemsJSON string
	if entry.StructuredLevel != nil {
		if lb, lerr := json.Marshal(entry.StructuredLevel); lerr == nil {
			levelJSON = string(lb)
		}
	}
	if len(entry.StructuredItems) > 0 {
		if ib, ierr := json.Marshal(entry.StructuredItems); ierr == nil {
			itemsJSON = string(ib)
		}
	}

	query := `
	INSERT INTO codex_entries (
		id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
		archetype, voice_tone, core_motivation, current_disposition,
		realm, inventory, health_status, structured_level_json, structured_items_json,
		alignment, threat_level,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		realm = excluded.realm,
		inventory = excluded.inventory,
		health_status = excluded.health_status,
		structured_level_json = excluded.structured_level_json,
		structured_items_json = excluded.structured_items_json,
		alignment = excluded.alignment,
		threat_level = excluded.threat_level,
		updated_at = excluded.updated_at;
	`
	_, err = tx.ExecContext(ctx, query,
		entry.ID, entry.ProjectID, string(entry.Category), entry.Name,
		entry.ColorTag, entry.Summary, entry.DetailsMarkdown, string(entry.TrackingMode),
		entry.Archetype, entry.VoiceTone, entry.CoreMotivation, entry.CurrentDisposition,
		entry.Realm, entry.Inventory, entry.HealthStatus, levelJSON, itemsJSON,
		entry.Alignment, entry.ThreatLevel,
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

	afterBytes, _ := json.Marshal(entry)
	eventType := activity.EventCodexEntryUpdated
	title := fmt.Sprintf("百科「%s」已更新", entry.Name)
	if !existed {
		eventType = activity.EventCodexEntryCreated
		title = fmt.Sprintf("百科新建「%s」（%s）", entry.Name, entry.Category)
	}
	var fieldsChanged []string
	if existed && beforeJSON != "" {
		var oldEntry domain.CodexEntry
		if uerr := json.Unmarshal([]byte(beforeJSON), &oldEntry); uerr == nil {
			var ob, ab map[string]any
			obb, _ := json.Marshal(map[string]any{
				"name": oldEntry.Name, "summary": oldEntry.Summary,
				"details": oldEntry.DetailsMarkdown, "aliases": oldEntry.Aliases,
				"tracking": string(oldEntry.TrackingMode),
			})
			abb, _ := json.Marshal(map[string]any{
				"name": entry.Name, "summary": entry.Summary,
				"details": entry.DetailsMarkdown, "aliases": entry.Aliases,
				"tracking": string(entry.TrackingMode),
			})
			_ = json.Unmarshal(obb, &ob)
			_ = json.Unmarshal(abb, &ab)
			fieldsChanged = activity.ChangedFields(ob, ab)
		}
	}
	e := newEvent(ctx, entry.ProjectID, activity.EntityCodexEntry, entry.ID, 0, eventType, title, entry.Summary)
	withTextSnapshots(e, beforeJSON, string(afterBytes), fieldsChanged)
	e.ExcerptBefore = activity.Excerpt(codexExcerptOf(beforeJSON), activity.ExcerptRunes)
	e.ExcerptAfter = activity.Excerpt(entry.Summary+"\n"+entry.DetailsMarkdown, activity.ExcerptRunes)
	if err := insertEvent(ctx, tx, e); err != nil {
		return err
	}

	return tx.Commit()
}

// codexExcerptOf pulls a readable preview out of a stored entry snapshot.
func codexExcerptOf(entryJSON string) string {
	if strings.TrimSpace(entryJSON) == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(entryJSON), &m); err != nil {
		return ""
	}
	summary, _ := m["summary"].(string)
	details, _ := m["details_markdown"].(string)
	if summary == "" {
		summary, _ = m["Summary"].(string)
	}
	if details == "" {
		var v string
		v, _ = m["details"].(string)
		details = v
	}
	if summary != "" && details != "" {
		return summary + "\n" + details
	}
	return summary + details
}

func (s *SQLiteStore) GetCodexEntry(ctx context.Context, projectID, id string) (*domain.CodexEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
		       COALESCE(archetype, ''), COALESCE(voice_tone, ''), COALESCE(core_motivation, ''), COALESCE(current_disposition, ''),
		       COALESCE(realm, ''), COALESCE(inventory, ''), COALESCE(health_status, ''),
		       COALESCE(structured_level_json, ''), COALESCE(structured_items_json, ''),
		       COALESCE(alignment, ''), COALESCE(threat_level, ''),
		       created_at, updated_at
		FROM codex_entries
		WHERE project_id = ? AND id = ?
	`, projectID, id)

	var e domain.CodexEntry
	var cat, tm, colorTag, summary, details, arch, vt, motiv, disp sql.NullString
	var realm, inv, health, levelJSON, itemsJSON, align, threat sql.NullString
	if err := row.Scan(
		&e.ID, &e.ProjectID, &cat, &e.Name, &colorTag, &summary, &details, &tm,
		&arch, &vt, &motiv, &disp,
		&realm, &inv, &health, &levelJSON, &itemsJSON,
		&align, &threat,
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
	e.Realm = realm.String
	e.Inventory = inv.String
	e.HealthStatus = health.String
	e.Alignment = align.String
	e.ThreatLevel = threat.String
	if levelJSON.String != "" {
		var pl domain.PowerLevel
		if json.Unmarshal([]byte(levelJSON.String), &pl) == nil {
			e.StructuredLevel = &pl
		}
	}
	if itemsJSON.String != "" {
		var itms []domain.InventoryItem
		if json.Unmarshal([]byte(itemsJSON.String), &itms) == nil {
			e.StructuredItems = itms
		}
	}

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

func (s *SQLiteStore) GetProtagonistEntry(ctx context.Context, projectID string) (*domain.CodexEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
		       COALESCE(archetype, ''), COALESCE(voice_tone, ''), COALESCE(core_motivation, ''), COALESCE(current_disposition, ''),
		       COALESCE(realm, ''), COALESCE(inventory, ''), COALESCE(health_status, ''),
		       COALESCE(structured_level_json, ''), COALESCE(structured_items_json, ''),
		       COALESCE(alignment, ''), COALESCE(threat_level, ''),
		       created_at, updated_at
		FROM codex_entries
		WHERE project_id = ? AND (archetype = 'PROTAGONIST' OR (category = 'CHARACTER' AND (name = '主角' OR id LIKE '%pro%')))
		ORDER BY CASE WHEN archetype = 'PROTAGONIST' THEN 0 ELSE 1 END, created_at ASC
		LIMIT 1
	`, projectID)

	var e domain.CodexEntry
	var cat, tm, colorTag, summary, details, arch, vt, motiv, disp sql.NullString
	var realm, inv, health, levelJSON, itemsJSON, align, threat sql.NullString
	if err := row.Scan(
		&e.ID, &e.ProjectID, &cat, &e.Name, &colorTag, &summary, &details, &tm,
		&arch, &vt, &motiv, &disp,
		&realm, &inv, &health, &levelJSON, &itemsJSON,
		&align, &threat,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan protagonist entry: %w", err)
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
	e.Realm = realm.String
	e.Inventory = inv.String
	e.HealthStatus = health.String
	e.Alignment = align.String
	e.ThreatLevel = threat.String
	if levelJSON.String != "" {
		var pl domain.PowerLevel
		if json.Unmarshal([]byte(levelJSON.String), &pl) == nil {
			e.StructuredLevel = &pl
		}
	}
	if itemsJSON.String != "" {
		var itms []domain.InventoryItem
		if json.Unmarshal([]byte(itemsJSON.String), &itms) == nil {
			e.StructuredItems = itms
		}
	}

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

func (s *SQLiteStore) getProtagonistTx(ctx context.Context, tx *sql.Tx, projectID string) (*domain.CodexEntry, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
		       COALESCE(archetype, ''), COALESCE(voice_tone, ''), COALESCE(core_motivation, ''), COALESCE(current_disposition, ''),
		       COALESCE(realm, ''), COALESCE(inventory, ''), COALESCE(health_status, ''),
		       COALESCE(structured_level_json, ''), COALESCE(structured_items_json, ''),
		       COALESCE(alignment, ''), COALESCE(threat_level, ''),
		       created_at, updated_at
		FROM codex_entries
		WHERE project_id = ? AND (archetype = 'PROTAGONIST' OR (category = 'CHARACTER' AND (name = '主角' OR id LIKE '%pro%')))
		ORDER BY CASE WHEN archetype = 'PROTAGONIST' THEN 0 ELSE 1 END, created_at ASC
		LIMIT 1
	`, projectID)

	var e domain.CodexEntry
	var cat, tm, colorTag, summary, details, arch, vt, motiv, disp sql.NullString
	var realm, inv, health, levelJSON, itemsJSON, align, threat sql.NullString
	if err := row.Scan(
		&e.ID, &e.ProjectID, &cat, &e.Name, &colorTag, &summary, &details, &tm,
		&arch, &vt, &motiv, &disp,
		&realm, &inv, &health, &levelJSON, &itemsJSON,
		&align, &threat,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
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
	e.Realm = realm.String
	e.Inventory = inv.String
	e.HealthStatus = health.String
	e.Alignment = align.String
	e.ThreatLevel = threat.String
	if levelJSON.String != "" {
		var pl domain.PowerLevel
		if json.Unmarshal([]byte(levelJSON.String), &pl) == nil {
			e.StructuredLevel = &pl
		}
	}
	if itemsJSON.String != "" {
		var itms []domain.InventoryItem
		if json.Unmarshal([]byte(itemsJSON.String), &itms) == nil {
			e.StructuredItems = itms
		}
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
			       COALESCE(realm, ''), COALESCE(inventory, ''), COALESCE(health_status, ''),
			       COALESCE(structured_level_json, ''), COALESCE(structured_items_json, ''),
			       COALESCE(alignment, ''), COALESCE(threat_level, ''),
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
			       COALESCE(realm, ''), COALESCE(inventory, ''), COALESCE(health_status, ''),
			       COALESCE(structured_level_json, ''), COALESCE(structured_items_json, ''),
			       COALESCE(alignment, ''), COALESCE(threat_level, ''),
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
		var realm, inv, health, levelJSON, itemsJSON, align, threat sql.NullString
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &cat, &e.Name, &colorTag, &summary, &details, &tm,
			&arch, &vt, &motiv, &disp,
			&realm, &inv, &health, &levelJSON, &itemsJSON,
			&align, &threat,
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
		e.Realm = realm.String
		e.Inventory = inv.String
		e.HealthStatus = health.String
		e.Alignment = align.String
		e.ThreatLevel = threat.String
		if levelJSON.String != "" {
			var pl domain.PowerLevel
			if json.Unmarshal([]byte(levelJSON.String), &pl) == nil {
				e.StructuredLevel = &pl
			}
		}
		if itemsJSON.String != "" {
			var itms []domain.InventoryItem
			if json.Unmarshal([]byte(itemsJSON.String), &itms) == nil {
				e.StructuredItems = itms
			}
		}
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
	var beforeJSON, entryName string
	if old, oerr := s.GetCodexEntry(ctx, projectID, id); oerr == nil && old != nil {
		entryName = old.Name
		if ob, merr := json.Marshal(old); merr == nil {
			beforeJSON = string(ob)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `DELETE FROM codex_entries WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		return fmt.Errorf("delete codex entry: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("codex entry not found")
	}

	e := newEvent(ctx, projectID, activity.EntityCodexEntry, id, 0, activity.EventCodexEntryDeleted,
		fmt.Sprintf("百科「%s」已删除", entryName), "条目及其别名/进展/关系已级联删除，墓碑留存")
	withTextSnapshots(e, beforeJSON, "", []string{"deleted"})
	e.ExcerptBefore = activity.Excerpt(codexExcerptOf(beforeJSON), activity.ExcerptRunes)
	if err := emitLedger(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
	INSERT INTO codex_progressions (id, entry_id, active_from_chapter, state_payload_json, notes, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		active_from_chapter = excluded.active_from_chapter,
		state_payload_json = excluded.state_payload_json,
		notes = excluded.notes;
	`
	if _, err := tx.ExecContext(ctx, query,
		prog.ID, prog.EntryID, prog.ActiveFromChapter, prog.StatePayloadJSON, prog.Notes, prog.CreatedAt,
	); err != nil {
		return err
	}

	var projectID, entryName string
	_ = tx.QueryRowContext(ctx, `SELECT project_id, name FROM codex_entries WHERE id = ?`, entryID).Scan(&projectID, &entryName)
	if projectID != "" {
		afterBytes, _ := json.Marshal(prog)
		e := newEvent(ctx, projectID, activity.EntityCodexProgression, prog.ID, prog.ActiveFromChapter,
			activity.EventCodexProgAdded, fmt.Sprintf("「%s」阶段演进（第 %d 章起）", entryName, prog.ActiveFromChapter), prog.Notes)
		withTextSnapshots(e, "", string(afterBytes), []string{"state_payload"})
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
	INSERT INTO codex_relations (id, project_id, source_entry_id, target_entry_id, relation_type, description, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		relation_type = excluded.relation_type,
		description = excluded.description;
	`
	if _, err := tx.ExecContext(ctx, query,
		rel.ID, rel.ProjectID, rel.SourceEntryID, rel.TargetEntryID, rel.RelationType, rel.Description, rel.CreatedAt,
	); err != nil {
		return err
	}

	afterBytes, _ := json.Marshal(rel)
	e := newEvent(ctx, projectID, activity.EntityCodexRelation, rel.ID, 0,
		activity.EventCodexRelAdded, fmt.Sprintf("关系 %s→%s（%s）", rel.SourceEntryID, rel.TargetEntryID, rel.RelationType), rel.Description)
	withTextSnapshots(e, "", string(afterBytes), []string{"relation"})
	if err := emitLedger(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
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
	var beforeJSON string
	orow := s.db.QueryRowContext(ctx, `SELECT source_entry_id, target_entry_id, relation_type, description FROM codex_relations WHERE project_id = ? AND id = ?`, projectID, id)
	var src, tgt, rtype, desc string
	if oerr := orow.Scan(&src, &tgt, &rtype, &desc); oerr == nil {
		if ob, merr := json.Marshal(map[string]any{
			"id": id, "source": src, "target": tgt, "type": rtype, "description": desc,
		}); merr == nil {
			beforeJSON = string(ob)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM codex_relations WHERE project_id = ? AND id = ?`, projectID, id); err != nil {
		return err
	}

	e := newEvent(ctx, projectID, activity.EntityCodexRelation, id, 0,
		activity.EventCodexRelDeleted, fmt.Sprintf("关系 %s→%s 已删除", src, tgt), desc)
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

	var beforeProse string
	orow := s.db.QueryRowContext(ctx, `SELECT prose_content FROM scenes WHERE id = ?`, sc.ID)
	existed := false
	if oerr := orow.Scan(&beforeProse); oerr == nil {
		existed = true
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

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
	if _, err := tx.ExecContext(ctx, query,
		sc.ID, sc.ChapterID, sc.ProjectID, sc.SceneIndex, sc.Title, sc.LocationEntryID,
		sc.DramaticGoal, sc.ConflictBarrier, sc.TensionLevel, string(beatsJSON), string(stateMutJSON),
		sc.ProseContent, sc.WordCount, sc.ExcludeFromAI, sc.CreatedAt, sc.UpdatedAt,
	); err != nil {
		return err
	}

	afterBytes, _ := json.Marshal(sc)
	title := fmt.Sprintf("场次「%s」已保存（%d 字）", sc.Title, sc.WordCount)
	if !existed {
		title = fmt.Sprintf("场次「%s」已创建", sc.Title)
	}
	e := newEvent(ctx, sc.ProjectID, activity.EntityScene, sc.ID, 0,
		activity.EventSceneUpserted, title, sc.DramaticGoal)
	withTextSnapshots(e, beforeProse, string(afterBytes), []string{"prose_content"})
	e.ExcerptBefore = activity.Excerpt(beforeProse, activity.ExcerptRunes)
	e.ExcerptAfter = activity.Excerpt(sc.ProseContent, activity.ExcerptRunes)
	if err := emitLedger(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
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
	var beforeProse, projectID, sceneTitle string
	orow := s.db.QueryRowContext(ctx, `SELECT project_id, title, prose_content FROM scenes WHERE id = ?`, id)
	_ = orow.Scan(&projectID, &sceneTitle, &beforeProse)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM scenes WHERE id = ?`, id); err != nil {
		return err
	}

	if projectID != "" {
		e := newEvent(ctx, projectID, activity.EntityScene, id, 0,
			activity.EventSceneDeleted, fmt.Sprintf("场次「%s」已删除", sceneTitle), "场次及其批注已级联删除，墓碑留存")
		withTextSnapshots(e, beforeProse, "", []string{"deleted"})
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

func (s *SQLiteStore) SaveSceneMarker(ctx context.Context, m *domain.SceneMarker) error {
	if m.ID == "" {
		m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

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
	if _, err := tx.ExecContext(ctx, query,
		m.ID, m.SceneID, string(m.MarkerType), m.Color, m.TextRangeStart, m.TextRangeEnd, m.Content, m.Resolved, m.CreatedAt,
	); err != nil {
		return err
	}

	var projectID string
	_ = tx.QueryRowContext(ctx, `SELECT project_id FROM scenes WHERE id = ?`, m.SceneID).Scan(&projectID)
	if projectID != "" {
		afterBytes, _ := json.Marshal(m)
		e := newEvent(ctx, projectID, activity.EntitySceneMarker, m.ID, 0,
			activity.EventSceneMarkerAdded, fmt.Sprintf("批注 [%s] %s", m.MarkerType, m.Content), fmt.Sprintf("场次 %s · 区间 %d-%d", m.SceneID, m.TextRangeStart, m.TextRangeEnd))
		withTextSnapshots(e, "", string(afterBytes), []string{"marker"})
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
	var sceneID, content, markerType string
	orow := s.db.QueryRowContext(ctx, `SELECT scene_id, marker_type, content FROM scene_markers WHERE id = ?`, id)
	_ = orow.Scan(&sceneID, &markerType, &content)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM scene_markers WHERE id = ?`, id); err != nil {
		return err
	}

	var projectID string
	if sceneID != "" {
		_ = tx.QueryRowContext(ctx, `SELECT project_id FROM scenes WHERE id = ?`, sceneID).Scan(&projectID)
	}
	if projectID != "" {
		e := newEvent(ctx, projectID, activity.EntitySceneMarker, id, 0,
			activity.EventSceneMarkerDelete, fmt.Sprintf("批注 [%s] 已删除", markerType), content)
		withTextSnapshots(e, content, "", []string{"deleted"})
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

// -----------------------------------------------------------------------------
// The Forecast & Narrative Tree (前瞻推演与活跃规划)
// -----------------------------------------------------------------------------

// ComputeFingerprint calculates the deterministic fingerprint of a project's canon state:
// Rule: fmt.Sprintf("%d:%s:%d", maxArchivedChapterIndex, lastChapterID, openOrFermentingHooksCount)
// - maxArchivedChapterIndex: MAX(chapter_index) from chapters table (0 if none)
// - lastChapterID: id of the chapter with MAX(chapter_index) ("" if none)
// - openOrFermentingHooksCount: COUNT(*) of plot_hooks where status IN ('OPEN', 'FERMENTING')
// Any new archived chapter, uncommitted chapter, or hook transition deterministically alters this fingerprint.
func (s *SQLiteStore) ComputeFingerprint(ctx context.Context, projectID string) string {
	var maxChapter int
	var lastChapterID sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(chapter_index), 0), 
		       COALESCE((SELECT id FROM chapters WHERE project_id = ? ORDER BY chapter_index DESC LIMIT 1), '')
		FROM chapters WHERE project_id = ?`, projectID, projectID).Scan(&maxChapter, &lastChapterID)

	var hookCount int
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM plot_hooks 
		WHERE project_id = ? AND status IN ('OPEN', 'FERMENTING')`, projectID).Scan(&hookCount)

	lastID := ""
	if lastChapterID.Valid {
		lastID = lastChapterID.String
	}
	return fmt.Sprintf("%d:%s:%d", maxChapter, lastID, hookCount)
}

// SaveForecast persists a new narrative forecast and enforces the single-active invariant:
// existing active forecasts for the project are archived before saving the new one.
// Activity ledger records an event with ID and branch count without dumping the large branches_json.
func (s *SQLiteStore) SaveForecast(ctx context.Context, f *domain.Forecast) error {
	if f == nil {
		return errors.New("forecast cannot be nil")
	}
	f.ProjectID = strings.TrimSpace(f.ProjectID)
	if f.ProjectID == "" {
		return errors.New("forecast project_id cannot be empty")
	}
	f.ID = strings.TrimSpace(f.ID)
	if f.ID == "" {
		f.ID = fmt.Sprintf("fc_%d", time.Now().UnixNano())
	}
	if f.Fingerprint == "" {
		f.Fingerprint = s.ComputeFingerprint(ctx, f.ProjectID)
	}
	if f.Status == "" {
		f.Status = domain.ForecastStatusActive
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}

	branchesJSON, err := json.Marshal(f.Branches)
	if err != nil {
		return fmt.Errorf("marshal branches failed: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Single-active invariant: archive existing active forecasts for this project
	if _, err := tx.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE project_id = ? AND status = ?`,
		domain.ForecastStatusArchived, f.ProjectID, domain.ForecastStatusActive,
	); err != nil {
		return fmt.Errorf("archive previous active forecasts failed: %w", err)
	}

	// Insert the new forecast
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO narrative_forecasts (
			id, project_id, base_chapter, divergence, branch_count, horizon, fingerprint, status, branches_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.ProjectID, f.BaseChapter, f.Divergence, f.BranchCount, f.Horizon, f.Fingerprint, f.Status, string(branchesJSON), f.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert narrative forecast failed: %w", err)
	}

	// Activity ledger: only records ID and branch count, NEVER branches_json snapshot
	baseIdx, _ := strconv.Atoi(f.BaseChapter)
	e := newEvent(ctx, f.ProjectID, "forecast", f.ID, baseIdx, "forecast.created", "生成前瞻推演",
		fmt.Sprintf("前瞻分支数: %d, 跨度: %d", f.BranchCount, f.Horizon))
	if err := emitLedger(ctx, tx, e); err != nil {
		return fmt.Errorf("emit forecast ledger failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

// LoadForecast retrieves a forecast by ID, evaluating its fingerprint against the current canon state.
// If the fingerprint mismatches, the forecast and its associated active plan are transitioned to stale.
func (s *SQLiteStore) LoadForecast(ctx context.Context, projectID, forecastID string) (*domain.Forecast, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, base_chapter, divergence, branch_count, horizon, fingerprint, status, branches_json, created_at
		FROM narrative_forecasts
		WHERE project_id = ? AND id = ?`, projectID, forecastID)

	var f domain.Forecast
	var branchesJSON string
	if err := row.Scan(
		&f.ID, &f.ProjectID, &f.BaseChapter, &f.Divergence, &f.BranchCount, &f.Horizon, &f.Fingerprint, &f.Status, &branchesJSON, &f.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("forecast %s not found", forecastID)
		}
		return nil, err
	}

	if branchesJSON != "" {
		_ = json.Unmarshal([]byte(branchesJSON), &f.Branches)
	}

	// Fingerprint check: compare stored fingerprint with current deterministic canon fingerprint
	currFP := s.ComputeFingerprint(ctx, projectID)
	if f.Status == domain.ForecastStatusActive && f.Fingerprint != currFP {
		f.Status = domain.ForecastStatusStale
		_, _ = s.db.ExecContext(ctx, `UPDATE narrative_forecasts SET status = ? WHERE id = ?`, domain.ForecastStatusStale, f.ID)
		_, _ = s.db.ExecContext(ctx, `
			UPDATE project_active_plan 
			SET status = ?, updated_at = ? 
			WHERE project_id = ? AND forecast_id = ? AND status = ?`,
			domain.PlanStatusStale, time.Now(), projectID, f.ID, domain.PlanStatusActive,
		)
	}

	return &f, nil
}

// ListForecasts returns all forecasts for a project, refreshing stale statuses for any active forecasts.
func (s *SQLiteStore) ListForecasts(ctx context.Context, projectID string) ([]*domain.Forecast, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, base_chapter, divergence, branch_count, horizon, fingerprint, status, branches_json, created_at
		FROM narrative_forecasts
		WHERE project_id = ?
		ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	currFP := s.ComputeFingerprint(ctx, projectID)
	var list []*domain.Forecast

	for rows.Next() {
		var f domain.Forecast
		var branchesJSON string
		if err := rows.Scan(
			&f.ID, &f.ProjectID, &f.BaseChapter, &f.Divergence, &f.BranchCount, &f.Horizon, &f.Fingerprint, &f.Status, &branchesJSON, &f.CreatedAt,
		); err != nil {
			return nil, err
		}
		if branchesJSON != "" {
			_ = json.Unmarshal([]byte(branchesJSON), &f.Branches)
		}

		if f.Status == domain.ForecastStatusActive && f.Fingerprint != currFP {
			f.Status = domain.ForecastStatusStale
			_, _ = s.db.ExecContext(ctx, `UPDATE narrative_forecasts SET status = ? WHERE id = ?`, domain.ForecastStatusStale, f.ID)
			_, _ = s.db.ExecContext(ctx, `
				UPDATE project_active_plan 
				SET status = ?, updated_at = ? 
				WHERE project_id = ? AND forecast_id = ? AND status = ?`,
				domain.PlanStatusStale, time.Now(), projectID, f.ID, domain.PlanStatusActive,
			)
		}

		list = append(list, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Forecast{}
	}
	return list, nil
}

// SelectBranch adopts a branch from a forecast as the project's single active plan.
// STRICT ISOLATION GUARD: This function writes ONLY to narrative_forecasts and project_active_plan.
// Any modifications to chapters, codex_entries, plot_hooks, or runtime state are forbidden.
func (s *SQLiteStore) SelectBranch(ctx context.Context, projectID, forecastID, branchID string) (*domain.ActivePlan, error) {
	fc, err := s.LoadForecast(ctx, projectID, forecastID)
	if err != nil {
		return nil, err
	}

	var selectedBranch *domain.ForecastBranch
	for i := range fc.Branches {
		if fc.Branches[i].ID == branchID {
			selectedBranch = &fc.Branches[i]
			break
		}
	}
	if selectedBranch == nil {
		return nil, fmt.Errorf("branch %s not found in forecast %s", branchID, forecastID)
	}

	baseIdx, _ := strconv.Atoi(fc.BaseChapter)
	var planChapters []domain.PlanChapter
	for i := 1; i <= fc.Horizon; i++ {
		chIdx := baseIdx + i
		planChapters = append(planChapters, domain.PlanChapter{
			ChapterIndex: chIdx,
			Title:        fmt.Sprintf("第%d章", chIdx),
			Summary:      selectedBranch.BeatsSummary,
			Status:       domain.PlanStatusActive,
		})
	}

	planChaptersJSON, err := json.Marshal(planChapters)
	if err != nil {
		return nil, fmt.Errorf("marshal plan chapters failed: %w", err)
	}

	planText := fmt.Sprintf("分支: %s\n节拍概述: %s\n角色抉择: %s\n预期变化: %s\n潜在风险: %s\n意图匹配: %s",
		selectedBranch.ID, selectedBranch.BeatsSummary, selectedBranch.Decisions,
		selectedBranch.ExpectedDeltas, selectedBranch.Risks, selectedBranch.IntentMatch)

	now := time.Now()
	activePlan := &domain.ActivePlan{
		ProjectID:      projectID,
		ForecastID:     forecastID,
		BranchID:       branchID,
		PlanText:       planText,
		Status:         domain.PlanStatusActive,
		PlanChapters:   planChapters,
		ExpectedDeltas: selectedBranch.ExpectedDeltas,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Write narrative_forecasts status
	if _, err := tx.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE id = ?`,
		domain.ForecastStatusActive, forecastID,
	); err != nil {
		return nil, fmt.Errorf("update forecast status failed: %w", err)
	}

	// 2. Write project_active_plan (single active plan guaranteed by project_id PRIMARY KEY)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_active_plan (
			project_id, forecast_id, branch_id, plan_text, status,
			plan_chapters_json, expected_deltas, warning_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			forecast_id = excluded.forecast_id,
			branch_id = excluded.branch_id,
			plan_text = excluded.plan_text,
			status = excluded.status,
			plan_chapters_json = excluded.plan_chapters_json,
			expected_deltas = excluded.expected_deltas,
			warning_message = excluded.warning_message,
			updated_at = excluded.updated_at`,
		projectID, forecastID, branchID, planText, domain.PlanStatusActive,
		string(planChaptersJSON), selectedBranch.ExpectedDeltas, "", now, now,
	); err != nil {
		return nil, fmt.Errorf("upsert active plan failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil

	return activePlan, nil
}

// GetActivePlan retrieves the active plan for a project with memory-derived consumption status
// and keyword overlap divergence checking against committed chapters.
func (s *SQLiteStore) GetActivePlan(ctx context.Context, projectID string) (*domain.ActivePlan, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT project_id, forecast_id, branch_id, plan_text, status,
		       plan_chapters_json, expected_deltas, warning_message, created_at, updated_at
		FROM project_active_plan
		WHERE project_id = ?`, projectID)

	var plan domain.ActivePlan
	var chaptersJSON string
	if err := row.Scan(
		&plan.ProjectID, &plan.ForecastID, &plan.BranchID, &plan.PlanText, &plan.Status,
		&chaptersJSON, &plan.ExpectedDeltas, &plan.WarningMessage, &plan.CreatedAt, &plan.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if chaptersJSON != "" {
		_ = json.Unmarshal([]byte(chaptersJSON), &plan.PlanChapters)
	}

	// Check forecast fingerprint status
	fc, err := s.LoadForecast(ctx, projectID, plan.ForecastID)

	// Read chapters for memory-derived consumed status & divergence check
	chapters, err := s.ListChapters(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters for active plan check failed: %w", err)
	}

	archivedMap := make(map[int]*domain.Chapter, len(chapters))
	for _, ch := range chapters {
		archivedMap[ch.ChapterIndex] = ch
	}

	// In-memory derivation of plan_chapters consumption (does not write to DB)
	allConsumed := len(plan.PlanChapters) > 0
	hasCoveredChapter := false
	for i := range plan.PlanChapters {
		idx := plan.PlanChapters[i].ChapterIndex
		if archivedMap[idx] != nil {
			plan.PlanChapters[i].Status = domain.PlanStatusConsumed
			hasCoveredChapter = true
		} else {
			plan.PlanChapters[i].Status = domain.PlanStatusActive
			allConsumed = false
		}
	}

	// Divergence detection: check ExpectedDeltas against state_mutation_json of archived chapters
	if hasCoveredChapter {
		var actualMutations strings.Builder
		for _, pc := range plan.PlanChapters {
			if ch, ok := archivedMap[pc.ChapterIndex]; ok {
				actualMutations.WriteString(" ")
				actualMutations.WriteString(ch.StateMutation.InventoryDelta)
				actualMutations.WriteString(" ")
				actualMutations.WriteString(ch.StateMutation.PowerDelta)
				for _, m := range ch.StateMutation.CharacterMutations {
					actualMutations.WriteString(" ")
					actualMutations.WriteString(m.Name)
					actualMutations.WriteString(" ")
					actualMutations.WriteString(m.StatusDelta)
					actualMutations.WriteString(" ")
					actualMutations.WriteString(m.RelationDelta)
				}
			}
		}

		diverged, warning := checkPlanDivergence(plan.ExpectedDeltas, actualMutations.String())
		if diverged {
			plan.Status = domain.PlanStatusDiverged
			plan.WarningMessage = warning
			// Update status in own table without auto-recalculating
			_, _ = s.db.ExecContext(ctx, `
				UPDATE project_active_plan 
				SET status = ?, warning_message = ?, updated_at = ? 
				WHERE project_id = ?`,
				domain.PlanStatusDiverged, warning, time.Now(), projectID)
		}
	}

	if plan.Status != domain.PlanStatusDiverged {
		if allConsumed {
			plan.Status = domain.PlanStatusConsumed
		} else if fc != nil && fc.Status == domain.ForecastStatusStale {
			if plan.Status == domain.PlanStatusActive {
				plan.Status = domain.PlanStatusStale
				_, _ = s.db.ExecContext(ctx, `
					UPDATE project_active_plan 
					SET status = ?, updated_at = ? 
					WHERE project_id = ?`, domain.PlanStatusStale, time.Now(), projectID)
			}
		}
	}

	return &plan, nil
}

// MarkForecastStale explicitly marks a forecast and its associated active plan as stale.
func (s *SQLiteStore) MarkForecastStale(ctx context.Context, projectID, forecastID string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE project_id = ? AND id = ?`,
		domain.ForecastStatusStale, projectID, forecastID,
	); err != nil {
		return err
	}

	_, _ = s.db.ExecContext(ctx, `
		UPDATE project_active_plan 
		SET status = ?, updated_at = ? 
		WHERE project_id = ? AND forecast_id = ?`,
		domain.PlanStatusStale, time.Now(), projectID, forecastID,
	)
	return nil
}

// checkPlanDivergence checks if active plan's ExpectedDeltas diverges from the state_mutation_json of archived chapters.
// Rule:
// 1. Extract significant keywords from ExpectedDeltas (>=2 characters, filtered for punctuation).
// 2. If expected keywords is non-empty, match against actual state mutations committed in canon.
// 3. If keyword overlap ratio is lower than 20% threshold, story mutations diverged from plan expectations.
// 4. Returns diverged=true and author-facing warning message.
func checkPlanDivergence(expectedDeltas string, actualMutations string) (bool, string) {
	expectedWords := extractKeywords(expectedDeltas)
	if len(expectedWords) == 0 {
		return false, ""
	}

	matched := 0
	actualLower := strings.ToLower(actualMutations)
	for _, w := range expectedWords {
		if strings.Contains(actualLower, strings.ToLower(w)) {
			matched++
		}
	}

	overlapRatio := float64(matched) / float64(len(expectedWords))
	const threshold = 0.20 // 20% overlap threshold
	if overlapRatio < threshold {
		warning := fmt.Sprintf("正史剧情变迁与计划预期严重偏离（关键词重合度 %.1f%% 低于阈值 %.1f%%），建议重新前瞻推演", overlapRatio*100, threshold*100)
		return true, warning
	}
	return false, ""
}

// extractKeywords splits text into clean semantic words/tokens, filtering out punctuation and stop chars.
func extractKeywords(text string) []string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '，', '。', '、', '！', '？', '：', '；', '（', '）', '【', '】', '“', '”', ',', '.', '!', '?', ':', ';', '(', ')', '[', ']', '+', '-', '*', '/', '\n', '\r', '\t', ' ':
			return ' '
		default:
			return r
		}
	}, text)

	fields := strings.Fields(clean)
	var keywords []string
	seen := make(map[string]bool)
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if len([]rune(f)) >= 2 && !seen[f] {
			seen[f] = true
			keywords = append(keywords, f)
		}
	}
	return keywords
}
