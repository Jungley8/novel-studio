package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleProjectHarmonize(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Content   string  `json:"content"`
		Intensity float64 `json:"intensity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		errorResponse(w, http.StatusBadRequest, "content cannot be empty")
		return
	}
	intensity := req.Intensity
	if intensity <= 0 {
		intensity = 0.6
	}

	harmonizer := s.workshop.Harmonizer()
	processed, report := harmonizer.FullProcess(req.Content, intensity)
	jsonResponse(w, http.StatusOK, map[string]any{
		"processed_content": processed,
		"report":            report,
	})
}

func (s *Server) handleSuggestHumanTouches(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	harmonizer := s.workshop.Harmonizer()
	suggestions := harmonizer.SuggestHumanTouches(req.Content)
	jsonResponse(w, http.StatusOK, map[string]any{
		"suggestions": suggestions,
	})
}

func (s *Server) handleSanitizeAI(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.workshop == nil || s.workshop.Harmonizer() == nil {
		errorResponse(w, http.StatusInternalServerError, "harmonizer not configured")
		return
	}
	harmonizer := s.workshop.Harmonizer()
	sanitized, items := harmonizer.DeterministicSanitize(req.Content)
	jsonResponse(w, http.StatusOK, map[string]any{
		"sanitized_content": sanitized,
		"items":             items,
		"items_count":       len(items),
	})
}

func (s *Server) handleSuggestConflict(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int `json:"chapter_index"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ChapterIndex <= 0 {
		req.ChapterIndex = 1
	}

	if s.orch == nil || s.chronicle == nil {
		errorResponse(w, http.StatusInternalServerError, "orchestrator or chronicle is not configured")
		return
	}

	horizon, err := s.chronicle.AssembleHorizon(r.Context(), projectID, req.ChapterIndex)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	conflict, usage, err := s.orch.SuggestChapterConflict(r.Context(), s.cfg.ReasoningModel, horizon)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"chapter_index": req.ChapterIndex,
		"core_conflict": conflict,
		"usage":         usage,
	})
}
