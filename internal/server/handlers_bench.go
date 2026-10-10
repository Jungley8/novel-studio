package server

import (
	"context"
	"net/http"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

// handleProjectBench handles GET /api/projects/:id/bench
func (s *Server) handleProjectBench(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil || project == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	chapters, err := s.store.ListChapters(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list chapters failed: "+err.Error())
		return
	}

	hooks, err := s.store.ListPlotHooks(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list plot hooks failed: "+err.Error())
		return
	}

	protagonist, _ := s.store.GetProtagonistEntry(ctx, projectID)

	styleVaultSeeded := false
	if svStore, ok := s.store.(interface {
		GetStyleVault(ctx context.Context, projectID string) (*domain.StyleVault, error)
	}); ok {
		if vault, err := svStore.GetStyleVault(ctx, projectID); err == nil && vault != nil {
			styleVaultSeeded = true
		}
	}

	report := engine.RunBenchProbes(project, chapters, hooks, protagonist, styleVaultSeeded)
	jsonResponse(w, http.StatusOK, report)
}
