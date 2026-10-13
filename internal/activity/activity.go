package activity

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Entity types tracked by the activity ledger.
const (
	EntityProject          = "project"
	EntityFramework        = "framework"
	EntityChapter          = "chapter"
	EntityHook             = "hook"
	EntityCodexEntry       = "codex_entry"
	EntityCodexProgression = "codex_progression"
	EntityCodexRelation    = "codex_relation"
	EntityScene            = "scene"
	EntitySceneMarker      = "scene_marker"
	EntityCheckpoint       = "checkpoint"
	EntityWorkshop         = "workshop"
	EntityInline           = "inline"
)

// Domain event types. Checkpoint phase detail rides in Title/Summary,
// so the timeline can show every intermediate draft state.
const (
	EventProjectCreated   = "project.created"
	EventProjectUpdated   = "project.updated"
	EventProjectDeleted   = "project.deleted"
	EventFrameworkBoot    = "framework.bootstrapped"
	EventFrameworkUpdated = "framework.updated"

	EventChapterCommitted   = "chapter.committed"
	EventChapterUncommitted = "chapter.uncommitted"
	EventChapterDrafted     = "chapter.drafted"
	EventChapterDeleted     = "chapter.deleted"

	EventHookCreated  = "hook.created"
	EventHookUpdated  = "hook.updated"
	EventHookResolved = "hook.resolved"
	EventHookReopened = "hook.reopened"
	EventHookDeleted  = "hook.deleted"

	EventCodexEntryCreated = "codex.entry_created"
	EventCodexEntryUpdated = "codex.entry_updated"
	EventCodexEntryDeleted = "codex.entry_deleted"
	EventCodexProgAdded    = "codex.progression_added"
	EventCodexRelAdded     = "codex.relation_added"
	EventCodexRelDeleted   = "codex.relation_deleted"

	EventSceneUpserted     = "scene.upserted"
	EventSceneDeleted      = "scene.deleted"
	EventSceneMarkerAdded  = "scene.marker_added"
	EventSceneMarkerDelete = "scene.marker_deleted"

	EventCheckpointSaved   = "checkpoint.saved"
	EventCheckpointCleared = "checkpoint.cleared"
	EventCheckpointResumed = "checkpoint.resumed"

	EventWorkshopCompleted = "workshop.pipeline_completed"
	EventInlineApplied     = "inline.applied"
)

// Actors that produce content or mutate state.
const (
	ActorUser       = "user"
	ActorAIWriter   = "ai_writer"
	ActorAIReviewer = "ai_reviewer"
	ActorAIRewriter = "ai_rewriter"
	ActorAIReasoner = "ai_reasoner"
	ActorSystem     = "system"
)

// Sources distinguish manual UI edits from autonomous pipelines.
const (
	SourceUIManual   = "ui_manual"
	SourceAutoPipe   = "auto_pipeline"
	SourceCLIProduce = "cli_produce"
	SourceCLIGenesis = "cli_genesis"
	SourceBootstrap  = "bootstrap"
	SourceInline     = "inline_action"
	SourceAIExtract  = "ai_extract"
	SourceAIAnalyze  = "ai_analyze"
	SourceAIGenerate = "ai_generate"
	SourceAIChat     = "ai_chat"
	SourceDirect     = "direct"
	SourceImport     = "import"
)

// Payload codecs for before/after snapshots.
const (
	CodecNone = "none"
	CodecGzip = "gzip"
)

// ExcerptRunes is the plain-text preview length kept uncompressed for list views.
const ExcerptRunes = 250

// ActivityEvent is the append-only ledger record for every content
// production or mutation. Before/After hold full snapshots (large ones
// gzip-compressed at the storage layer); list queries only expose the
// plain-text excerpt fields and never decompress blobs.
type ActivityEvent struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Seq           int64     `json:"seq"`
	RunID         string    `json:"run_id,omitempty"`
	EntityType    string    `json:"entity_type"`
	EntityID      string    `json:"entity_id"`
	ChapterIndex  int       `json:"chapter_index,omitempty"`
	EventType     string    `json:"event_type"`
	Actor         string    `json:"actor"`
	Source        string    `json:"source"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary,omitempty"`
	ExcerptBefore string    `json:"excerpt_before,omitempty"`
	ExcerptAfter  string    `json:"excerpt_after,omitempty"`
	Before        string    `json:"before,omitempty"`
	After         string    `json:"after,omitempty"`
	Codec         string    `json:"codec,omitempty"`
	FieldsChanged []string  `json:"fields_changed,omitempty"`
	CharsAdded    int       `json:"chars_added,omitempty"`
	CharsRemoved  int       `json:"chars_removed,omitempty"`
	TokenUsage    string    `json:"token_usage,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Validate enforces Fail-Closed writes: incomplete events are rejected
// instead of being stored degraded.
func (e *ActivityEvent) Validate() error {
	if e == nil {
		return fmt.Errorf("activity event is nil")
	}
	if e.ProjectID == "" {
		return fmt.Errorf("activity event requires project_id")
	}
	if e.EntityType == "" {
		return fmt.Errorf("activity event requires entity_type")
	}
	if e.EntityID == "" {
		return fmt.Errorf("activity event requires entity_id")
	}
	if e.EventType == "" {
		return fmt.Errorf("activity event requires event_type")
	}
	return nil
}

// EventQuery filters the timeline. Cursor is an exclusive upper bound on
// Seq with descending order (newest first); empty cursor starts from latest.
type EventQuery struct {
	ProjectID         string
	EntityType        string
	EntityID          string
	ChapterIndex      int
	EventTypePrefix   string
	IncludeCheckpoint bool
	Limit             int
	Cursor            int64
}

// ---- context attribution ----

type ctxKey string

const (
	ctxKeyActor   ctxKey = "novelstudio/activity-actor"
	ctxKeySource  ctxKey = "novelstudio/activity-source"
	ctxKeyRunID   ctxKey = "novelstudio/activity-run"
	ctxKeySummary ctxKey = "novelstudio/activity-summary"
)

// WithActor tags content authorship (user vs which AI role).
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, ctxKeyActor, actor)
}

// WithSource tags the entry path (manual UI vs autonomous pipeline vs CLI).
func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, ctxKeySource, source)
}

// WithRunID correlates every event of one pipeline/manual run for timeline folding.
func WithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, ctxKeyRunID, runID)
}

// WithSummary attaches a narrative summary into context so transactional store writes can carry it.
func WithSummary(ctx context.Context, summary string) context.Context {
	return context.WithValue(ctx, ctxKeySummary, summary)
}

// SummaryFromCtx returns the custom summary passed via context, if any.
func SummaryFromCtx(ctx context.Context) string {
	return stringFromCtx(ctx, ctxKeySummary)
}

func stringFromCtx(ctx context.Context, key ctxKey) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}

// ActorFromCtx defaults to user: this is a single-operator desktop app,
// every action originates from the local author unless stated otherwise.
func ActorFromCtx(ctx context.Context) string {
	if a := stringFromCtx(ctx, ctxKeyActor); a != "" {
		return a
	}
	return ActorUser
}

// SourceFromCtx defaults to direct for store calls made outside HTTP/CLI wiring.
func SourceFromCtx(ctx context.Context) string {
	if s := stringFromCtx(ctx, ctxKeySource); s != "" {
		return s
	}
	return SourceDirect
}

// RunIDFromCtx returns "" when the caller did not correlate a run.
func RunIDFromCtx(ctx context.Context) string {
	return stringFromCtx(ctx, ctxKeyRunID)
}

// NewRunID mints a timeline-folding correlation id.
func NewRunID() string {
	return fmt.Sprintf("run_%d_%04d", time.Now().UnixNano(), idCounter.Add(1)%10000)
}

// idCounter disambiguates ids minted within the same nanosecond.
var idCounter atomic.Uint64

// NewID mints a globally unique event id.
func NewID() string {
	return fmt.Sprintf("evt_%d_%04d", time.Now().UnixNano(), idCounter.Add(1)%10000)
}
