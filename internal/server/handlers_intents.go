package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func (s *Server) handleProjectIntents(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()

	// 1. GET /api/projects/{id}/intents?volume=N (列表)
	if len(subParts) == 0 {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		volumeIndex := 0
		if volStr := r.URL.Query().Get("volume"); volStr != "" {
			var err error
			volumeIndex, err = strconv.Atoi(volStr)
			if err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid volume parameter: "+err.Error())
				return
			}
		}

		intents, err := s.store.ListChapterIntents(ctx, projectID, volumeIndex)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if intents == nil {
			intents = []*domain.ChapterIntent{}
		}
		jsonResponse(w, http.StatusOK, intents)
		return
	}

	// 2. POST /api/projects/{id}/intents/generate (批量推演)
	if len(subParts) == 1 && subParts[0] == "generate" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req struct {
			VolumeIndex int `json:"volume_index"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		if req.VolumeIndex <= 0 {
			errorResponse(w, http.StatusBadRequest, "volume_index must be positive")
			return
		}

		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil || proj == nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}

		if proj.Framework == nil {
			errorResponse(w, http.StatusBadRequest, "framework not bootstrapped")
			return
		}

		arcs := engine.BackfillVolumeArcs(proj.Framework.VolumeArcs)
		var targetArc *domain.VolumeArc
		for _, arc := range arcs {
			if arc.VolumeIndex == req.VolumeIndex {
				targetArc = &arc
				break
			}
		}
		if targetArc == nil {
			errorResponse(w, http.StatusNotFound, "volume arc not found")
			return
		}

		existingList, err := s.store.ListChapterIntents(ctx, projectID, req.VolumeIndex)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		existing := make(map[int]bool, len(existingList))
		for _, item := range existingList {
			if item != nil {
				existing[item.ChapterIndex] = true
			}
		}

		if s.orch == nil {
			errorResponse(w, http.StatusInternalServerError, "orchestrator not initialized")
			return
		}

		reasoningModel := ""
		if s.cfg != nil {
			reasoningModel = s.cfg.ReasoningModel
		}

		intents, tokens, err := s.orch.GenerateVolumeIntents(ctx, reasoningModel, proj, *targetArc, existing)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		for i := range intents {
			if err := s.store.SaveChapterIntent(ctx, &intents[i]); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		if intents == nil {
			intents = []domain.ChapterIntent{}
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"created": len(intents),
			"intents": intents,
			"tokens":  tokens,
		})
		return
	}

	// 3. /api/projects/{id}/intents/{chapter}
	if len(subParts) == 1 {
		chapterIndex, err := strconv.Atoi(subParts[0])
		if err != nil || chapterIndex <= 0 {
			errorResponse(w, http.StatusBadRequest, "invalid chapter index")
			return
		}

		switch r.Method {
		case http.MethodGet:
			intent, err := s.store.GetChapterIntent(ctx, projectID, chapterIndex)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if intent == nil {
				errorResponse(w, http.StatusNotFound, "chapter intent not found")
				return
			}
			jsonResponse(w, http.StatusOK, intent)

		case http.MethodPut:
			var intent domain.ChapterIntent
			if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}

			// 服务端强制 project_id、chapter_index 取路径（忽略 body 里的，防注入）
			intent.ProjectID = projectID
			intent.ChapterIndex = chapterIndex

			targetStatus := intent.Status
			if targetStatus == "" {
				intent.Status = domain.IntentStatusDraft
			} else if targetStatus == domain.IntentStatusLocked {
				// 用户提交 LOCKED 时走 TransitionTo（fail-closed：非法意图不许 LOCKED，返回 400）
				intent.Status = domain.IntentStatusDraft
				if existing, _ := s.store.GetChapterIntent(ctx, projectID, chapterIndex); existing != nil && existing.Status == domain.IntentStatusStale {
					intent.Status = domain.IntentStatusStale
				}
				if err := intent.TransitionTo(domain.IntentStatusLocked); err != nil {
					errorResponse(w, http.StatusBadRequest, err.Error())
					return
				}
			}

			if err := s.store.SaveChapterIntent(ctx, &intent); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, &intent)

		case http.MethodDelete:
			if err := s.store.DeleteChapterIntent(ctx, projectID, chapterIndex); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	errorResponse(w, http.StatusNotFound, "not found")
}
