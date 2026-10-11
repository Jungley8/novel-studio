package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// handleStudioAutomation handles GET /api/studio/automation.
func (s *Server) handleStudioAutomation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := s.store.ListProjectAutomations(r.Context())
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list automations: "+err.Error())
		return
	}
	if items == nil {
		items = []*domain.ProjectAutomation{}
	}

	jsonResponse(w, http.StatusOK, map[string]any{"items": items})
}

// handleStudioAutomationRoutes handles /api/studio/automation/{pid} and /api/studio/automation/{pid}/resume.
func (s *Server) handleStudioAutomationRoutes(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/studio/automation/")
	parts := strings.Split(strings.Trim(suffix, "/"), "/")

	if len(parts) == 1 && parts[0] != "" {
		pid := parts[0]
		if r.Method != http.MethodPut {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			Enabled    bool                       `json:"enabled"`
			Times      []string                   `json:"times"`
			QuotaKind  domain.AutomationQuotaKind `json:"quota_kind"`
			QuotaCount int                        `json:"quota_count"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		ctx := r.Context()
		if _, err := s.store.GetProject(ctx, pid); err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}

		times := req.Times
		if times == nil {
			times = []string{}
		}

		auto := &domain.ProjectAutomation{
			ProjectID:  pid,
			Enabled:    req.Enabled,
			Times:      times,
			QuotaKind:  req.QuotaKind,
			QuotaCount: req.QuotaCount,
		}

		if err := domain.ValidateAutomation(auto); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := s.store.SaveProjectAutomation(ctx, auto); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			errorResponse(w, http.StatusInternalServerError, "save automation: "+err.Error())
			return
		}

		saved, err := s.store.GetProjectAutomation(ctx, pid)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get saved automation: "+err.Error())
			return
		}

		jsonResponse(w, http.StatusOK, saved)
		return
	}

	if len(parts) == 2 && parts[0] != "" && parts[1] == "resume" {
		pid := parts[0]
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		ctx := r.Context()
		if err := s.store.SetAutomationPaused(ctx, pid, false, ""); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errorResponse(w, http.StatusNotFound, "automation not found")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "resume automation: "+err.Error())
			return
		}

		jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	http.NotFound(w, r)
}

// handleStudioQueue handles GET /api/studio/queue and POST /api/studio/queue.
func (s *Server) handleStudioQueue(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListBookQueue(r.Context())
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list queue: "+err.Error())
			return
		}
		if items == nil {
			items = []*domain.BookQueueItem{}
		}
		jsonResponse(w, http.StatusOK, map[string]any{"items": items})

	case http.MethodPost:
		var req struct {
			Title    string `json:"title"`
			Concept  string `json:"concept"`
			Platform string `json:"platform"`
			Volumes  int    `json:"volumes"`
			AutoLock *bool  `json:"auto_lock"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		autoLock := true
		if req.AutoLock != nil {
			autoLock = *req.AutoLock
		}

		item := &domain.BookQueueItem{
			ID:       fmt.Sprintf("queue_%d", time.Now().UnixNano()),
			Title:    req.Title,
			Concept:  req.Concept,
			Platform: req.Platform,
			Volumes:  req.Volumes,
			AutoLock: autoLock,
			Status:   domain.QueueStatusPending,
		}

		if err := domain.ValidateBookQueueItem(item); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := s.store.SaveBookQueueItem(r.Context(), item); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save queue item: "+err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, item)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleStudioQueueRoutes handles PUT /api/studio/queue/{id} and DELETE /api/studio/queue/{id}.
func (s *Server) handleStudioQueueRoutes(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/studio/queue/")
	id := strings.Trim(suffix, "/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()
	switch r.Method {
	case http.MethodPut:
		existing, err := s.store.GetBookQueueItem(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errorResponse(w, http.StatusNotFound, "queue item not found")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "get queue item: "+err.Error())
			return
		}

		if existing.Status != domain.QueueStatusPending {
			errorResponse(w, http.StatusConflict, fmt.Sprintf("cannot update queue item with status %q", existing.Status))
			return
		}

		var req struct {
			Title    string `json:"title"`
			Concept  string `json:"concept"`
			Platform string `json:"platform"`
			Volumes  int    `json:"volumes"`
			AutoLock *bool  `json:"auto_lock"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		autoLock := existing.AutoLock
		if req.AutoLock != nil {
			autoLock = *req.AutoLock
		}

		updated := &domain.BookQueueItem{
			ID:          existing.ID,
			Title:       req.Title,
			Concept:     req.Concept,
			Platform:    req.Platform,
			Volumes:     req.Volumes,
			AutoLock:    autoLock,
			Status:      existing.Status,
			ProjectID:   existing.ProjectID,
			LastMessage: existing.LastMessage,
			CreatedAt:   existing.CreatedAt,
		}

		if err := domain.ValidateBookQueueItem(updated); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := s.store.SaveBookQueueItem(ctx, updated); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save queue item: "+err.Error())
			return
		}

		reloaded, err := s.store.GetBookQueueItem(ctx, id)
		if err == nil {
			updated = reloaded
		}
		jsonResponse(w, http.StatusOK, updated)

	case http.MethodDelete:
		existing, err := s.store.GetBookQueueItem(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errorResponse(w, http.StatusNotFound, "queue item not found")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "get queue item: "+err.Error())
			return
		}

		if existing.Status == domain.QueueStatusRunning {
			errorResponse(w, http.StatusConflict, "cannot delete running queue item")
			return
		}

		if err := s.store.DeleteBookQueueItem(ctx, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errorResponse(w, http.StatusNotFound, "queue item not found")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "delete queue item: "+err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
