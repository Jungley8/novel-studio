package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type forecastStore interface {
	SaveForecast(ctx context.Context, f *domain.Forecast) error
	LoadForecast(ctx context.Context, projectID, forecastID string) (*domain.Forecast, error)
	ListForecasts(ctx context.Context, projectID string) ([]*domain.Forecast, error)
	SelectBranch(ctx context.Context, projectID, forecastID, branchID string) (*domain.ActivePlan, error)
	GetActivePlan(ctx context.Context, projectID string) (*domain.ActivePlan, error)
	MarkForecastStale(ctx context.Context, projectID, forecastID string) error
	ComputeFingerprint(ctx context.Context, projectID string) (string, error)
}

func (s *Server) getLLMClient() engine.LLMClient {
	if s.llmRouter != nil {
		return s.llmRouter
	}
	return s.llmClient
}

func (s *Server) getForecastStore() forecastStore {
	if fs, ok := s.store.(forecastStore); ok {
		return fs
	}
	return nil
}

// handleProjectForecast handles:
// - POST /api/projects/:id/forecast {divergence, branch_count, horizon}
// - GET  /api/projects/:id/forecast
// - GET  /api/projects/:id/forecast/:fid
// - POST /api/projects/:id/forecast/:fid/select {branch_id}
// - GET  /api/projects/:id/forecast/active-plan
func (s *Server) handleProjectForecast(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	fStore := s.getForecastStore()
	if fStore == nil {
		errorResponse(w, http.StatusInternalServerError, "store does not support forecast operations")
		return
	}

	// 1. /api/projects/:id/forecast (POST create / GET list)
	if len(subParts) == 0 || (len(subParts) == 1 && subParts[0] == "") {
		switch r.Method {
		case http.MethodPost:
			var req struct {
				Divergence  string `json:"divergence"`
				BranchCount int    `json:"branch_count"`
				Horizon     int    `json:"horizon"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
				return
			}

			req.Divergence = strings.TrimSpace(req.Divergence)
			if req.Divergence == "" {
				errorResponse(w, http.StatusBadRequest, "divergence cannot be empty")
				return
			}
			if req.BranchCount < 2 || req.BranchCount > 5 {
				errorResponse(w, http.StatusBadRequest, "branch_count must be between 2 and 5")
				return
			}
			if req.Horizon < 3 || req.Horizon > 5 {
				errorResponse(w, http.StatusBadRequest, "horizon must be between 3 and 5")
				return
			}

			if s.chronicle == nil {
				errorResponse(w, http.StatusInternalServerError, "chronicle is not configured")
				return
			}

			chapters, err := s.store.ListChapters(r.Context(), projectID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, "list chapters failed: "+err.Error())
				return
			}
			maxChapter := 0
			for _, ch := range chapters {
				if ch.ChapterIndex > maxChapter {
					maxChapter = ch.ChapterIndex
				}
			}
			targetChapter := maxChapter + 1

			horizonCtx, err := s.chronicle.AssembleHorizon(r.Context(), projectID, targetChapter)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, "assemble horizon failed: "+err.Error())
				return
			}

			client := s.getLLMClient()
			if client == nil {
				errorResponse(w, http.StatusInternalServerError, "llm client is not configured")
				return
			}

			model := ""
			if s.cfg != nil {
				model = s.cfg.ReasoningModel
				if model == "" {
					model = s.cfg.WriterModel
				}
			}

			fc, err := engine.CreateForecast(r.Context(), client, model, horizonCtx, req.Divergence, req.BranchCount, req.Horizon)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, "create forecast failed: "+err.Error())
				return
			}

			fp, err := fStore.ComputeFingerprint(r.Context(), projectID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, "compute fingerprint failed: "+err.Error())
				return
			}
			fc.Fingerprint = fp
			if err := fStore.SaveForecast(r.Context(), fc); err != nil {
				errorResponse(w, http.StatusInternalServerError, "save forecast failed: "+err.Error())
				return
			}

			jsonResponse(w, http.StatusCreated, fc)

		case http.MethodGet:
			list, err := fStore.ListForecasts(r.Context(), projectID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, "list forecasts failed: "+err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, list)

		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 2. /api/projects/:id/forecast/active-plan or /api/projects/:id/forecast/plan (GET)
	if len(subParts) == 1 && (subParts[0] == "active-plan" || subParts[0] == "plan") {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		plan, err := fStore.GetActivePlan(r.Context(), projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get active plan failed: "+err.Error())
			return
		}
		if plan == nil {
			errorResponse(w, http.StatusNotFound, "no active plan found")
			return
		}
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	// 3. /api/projects/:id/forecast/:fid (GET detail & recalculate stale)
	if len(subParts) == 1 {
		fid := subParts[0]
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		fc, err := fStore.LoadForecast(r.Context(), projectID, fid)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, fc)
		return
	}

	// 4. /api/projects/:id/forecast/:fid/select (POST select branch)
	if len(subParts) >= 2 && subParts[1] == "select" {
		fid := subParts[0]
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			BranchID string `json:"branch_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.BranchID = strings.TrimSpace(req.BranchID)
		if req.BranchID == "" {
			errorResponse(w, http.StatusBadRequest, "branch_id cannot be empty")
			return
		}

		plan, err := fStore.SelectBranch(r.Context(), projectID, fid, req.BranchID)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	errorResponse(w, http.StatusNotFound, "not found")
}
