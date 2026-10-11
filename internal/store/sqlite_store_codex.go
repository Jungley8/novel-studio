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
