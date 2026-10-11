package server

import (
	"net/http"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/metrics"
)

// handleProjectMetrics handles GET /api/projects/:id/metrics
func (s *Server) handleProjectMetrics(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project id is required")
		return
	}

	ctx := r.Context()
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil || project == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	rows, err := s.store.ListChapterMetrics(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list chapter metrics failed: "+err.Error())
		return
	}

	if rows == nil {
		rows = []*domain.ChapterQualityMetrics{}
	}

	summary := metrics.Aggregate(rows)
	jsonResponse(w, http.StatusOK, map[string]any{
		"summary":  summary,
		"chapters": rows,
	})
}
