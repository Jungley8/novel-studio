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
)

// -------------------------------------------------------------
// Activity Ledger (内容生产与变更事件账本) 写入与查询基础设施
// -------------------------------------------------------------

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
