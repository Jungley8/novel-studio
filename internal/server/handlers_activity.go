package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Jungley8/novel-studio/internal/activity"
)

// activitySourceForProjectRoute tags every project-scoped request with its
// ledger source at a single choke point, so sub-handlers need no changes:
// manual UI edits stay ui_manual while AI/pipeline paths are labeled.
func activitySourceForProjectRoute(ctx context.Context, parts []string) context.Context {
	source := activity.SourceUIManual
	if len(parts) >= 2 {
		switch parts[1] {
		case "workshop":
			source = activity.SourceAutoPipe
		case "chat":
			source = activity.SourceAIChat
		case "framework":
			if len(parts) >= 3 && parts[2] == "bootstrap" {
				source = activity.SourceBootstrap
			}
		case "hooks":
			if len(parts) >= 3 && parts[2] == "ai-extract" {
				source = activity.SourceAIExtract
			}
		case "statemachine":
			if len(parts) >= 3 && parts[2] == "ai-analyze" {
				source = activity.SourceAIAnalyze
			}
		case "codex":
			if len(parts) >= 3 {
				switch parts[2] {
				case "ai-generate":
					source = activity.SourceAIGenerate
				case "relations":
					if len(parts) >= 4 && parts[3] == "ai-extract" {
						source = activity.SourceAIExtract
					}
				}
			}
		case "matrix":
			if len(parts) >= 3 && parts[2] == "ai-generate" {
				source = activity.SourceAIGenerate
			}
		}
	}
	return activity.WithSource(ctx, source)
}

// handleProjectActivity serves the append-only activity ledger:
// GET /api/projects/:id/activity[?entity_type=&entity_id=&chapter_index=
//
//	&event_type=&include_checkpoint=true|false&limit=&cursor=]
//
// GET /api/projects/:id/activity/:eventId
//
// List responses never decompress blobs (excerpts only); the detail
// endpoint decodes full before/after snapshots.
func (s *Server) handleProjectActivity(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()

	if len(subParts) > 0 && strings.TrimSpace(subParts[0]) != "" {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ev, err := s.store.GetEvent(ctx, projectID, strings.TrimSpace(subParts[0]))
		if err != nil {
			errorResponse(w, http.StatusNotFound, "activity event not found")
			return
		}
		jsonResponse(w, http.StatusOK, ev)
		return
	}

	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	q := activity.EventQuery{ProjectID: projectID}
	params := r.URL.Query()
	q.EntityType = strings.TrimSpace(params.Get("entity_type"))
	q.EntityID = strings.TrimSpace(params.Get("entity_id"))
	q.EventTypePrefix = strings.TrimSpace(params.Get("event_type"))
	// Checkpoints are visible by default; pass include_checkpoint=false to hide them.
	q.IncludeCheckpoint = params.Get("include_checkpoint") != "false"
	if idxStr := strings.TrimSpace(params.Get("chapter_index")); idxStr != "" {
		if idx, err := strconv.Atoi(idxStr); err == nil && idx > 0 {
			q.ChapterIndex = idx
		}
	}
	if limStr := strings.TrimSpace(params.Get("limit")); limStr != "" {
		if lim, err := strconv.Atoi(limStr); err == nil && lim > 0 {
			q.Limit = lim
		}
	}
	if curStr := strings.TrimSpace(params.Get("cursor")); curStr != "" {
		if cur, err := strconv.ParseInt(curStr, 10, 64); err == nil && cur > 0 {
			q.Cursor = cur
		}
	}

	events, nextCursor, err := s.store.ListEvents(ctx, q)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []*activity.ActivityEvent{}
	}
	var next string
	if nextCursor > 0 {
		next = strconv.FormatInt(nextCursor, 10)
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"events":      events,
		"next_cursor": next,
	})
}
