package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

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

	CREATE TABLE IF NOT EXISTS compaction_cache (
		project_id TEXT NOT NULL,
		cache_key TEXT NOT NULL,
		compiled_text TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		PRIMARY KEY (project_id, cache_key)
	);

	CREATE TABLE IF NOT EXISTS studio_threads (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		project_id TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS studio_messages (
		id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		mode TEXT NOT NULL DEFAULT '',
		project_id TEXT NOT NULL DEFAULT '',
		token_usage TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_studio_messages_thread ON studio_messages(thread_id, created_at);

	CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
		project_id UNINDEXED, kind UNINDEXED, ref_id UNINDEXED, chapter_index UNINDEXED,
		content, raw UNINDEXED, tokenize = 'unicode61'
	);

	CREATE TABLE IF NOT EXISTS project_automation (
		project_id TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL DEFAULT 0,
		times_json TEXT NOT NULL DEFAULT '[]',
		quota_kind TEXT NOT NULL DEFAULT 'chapters',
		quota_count INTEGER NOT NULL DEFAULT 5,
		paused INTEGER NOT NULL DEFAULT 0,
		paused_reason TEXT NOT NULL DEFAULT '',
		last_run_at TEXT NOT NULL DEFAULT '',
		last_status TEXT NOT NULL DEFAULT '',
		last_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS book_queue (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		concept TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		volumes INTEGER NOT NULL DEFAULT 1,
		auto_lock INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'pending',
		project_id TEXT NOT NULL DEFAULT '',
		last_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS scheduler_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	);
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
