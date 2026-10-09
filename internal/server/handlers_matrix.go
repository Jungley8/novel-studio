package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func (s *Server) handleProjectFramework(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	// POST /api/projects/:id/framework/bootstrap
	if len(parts) >= 3 && parts[2] == "bootstrap" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Concept        string `json:"concept"`
			ReasoningModel string `json:"reasoning_model,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		concept := req.Concept
		if concept == "" {
			concept = p.WorldRules
		}
		model := req.ReasoningModel
		if model == "" {
			model = s.cfg.ReasoningModel
		}

		fw, _, bErr := s.orch.BootstrapFramework(ctx, model, engine.FrameworkBootstrapRequest{
			Title:          p.Title,
			TargetPlatform: p.TargetPlatform,
			CoreConcept:    concept,
		})
		if bErr != nil {
			errorResponse(w, http.StatusInternalServerError, "bootstrap framework failed: "+bErr.Error())
			return
		}

		p.Framework = fw
		if p.WorldRules == "" && len(fw.WorldAxioms) > 0 {
			var sb strings.Builder
			sb.WriteString("【世界底层规则与核心公理】\n")
			for i, ax := range fw.WorldAxioms {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ax))
			}
			p.WorldRules = sb.String()
		}
		if p.Protagonist.CoreGoal == "" {
			p.Protagonist.CoreGoal = fw.ThemePremise
		}

		if err := s.store.SaveProject(ctx, p); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 同步势力阵营至设定集 (Codex)
		for _, fac := range fw.Factions {
			if strings.TrimSpace(fac.Name) == "" {
				continue
			}
			entry := &domain.CodexEntry{
				ID:              fmt.Sprintf("codex_%s_fac_%d", projectID, time.Now().UnixNano()),
				ProjectID:       projectID,
				Category:        domain.CategoryFaction,
				Name:            fac.Name,
				Summary:         fmt.Sprintf("立场: %s | 威胁度: %s", fac.Alignment, fac.ThreatLevel),
				DetailsMarkdown: fac.Doctrine,
				TrackingMode:    domain.TrackingModeAutoMention,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			_ = s.store.SaveCodexEntry(ctx, entry)
		}

		// 同步关键角色谱系至设定集 (Codex)
		for _, kc := range fw.KeyCharacters {
			if strings.TrimSpace(kc.Name) == "" {
				continue
			}
			entry := &domain.CodexEntry{
				ID:              fmt.Sprintf("codex_%s_char_%d", projectID, time.Now().UnixNano()),
				ProjectID:       projectID,
				Category:        domain.CategoryCharacter,
				Name:            kc.Name,
				Summary:         fmt.Sprintf("定位: %s | 境界: %s | 动机: %s", kc.Role, kc.Realm, kc.Goal),
				DetailsMarkdown: fmt.Sprintf("宿命终局: %s", kc.FateArc),
				TrackingMode:    domain.TrackingModeAutoMention,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			_ = s.store.SaveCodexEntry(ctx, entry)
		}

		// Seed hooks
		for i, sh := range fw.SeedHooks {
			hook := &domain.PlotHook{
				ID:             fmt.Sprintf("hook_%s_fw_%d", projectID, i+1),
				ProjectID:      projectID,
				Title:          sh.Title,
				Details:        sh.Details,
				CreatedChapter: sh.CreatedChapter,
				TargetChapter:  sh.TargetChapter,
				Status:         domain.HookStatusOpen,
				CreatedAt:      time.Now(),
			}
			_ = s.store.SavePlotHook(ctx, hook)
		}

		// 全自动级联关系图谱推演 (Extract Codex Relations Cascade)
		codexEntries, _ := s.store.ListCodexEntries(ctx, projectID, "")
		if len(codexEntries) >= 2 && s.orch != nil {
			relations, _, relErr := s.orch.ExtractCodexRelations(
				ctx,
				s.cfg.ReasoningModel,
				p,
				codexEntries,
				p.WorldRules+"\n\n全书核心立意："+fw.ThemePremise,
			)
			if relErr == nil && len(relations) > 0 {
				for _, rel := range relations {
					_ = s.store.SaveCodexRelation(ctx, projectID, &rel)
				}
			}
		}

		// 全自动推演第 1 卷第 1 章开局黄金冲突并初始化草稿检查点
		if s.chronicle != nil && s.orch != nil {
			horizon, hErr := s.chronicle.AssembleHorizon(ctx, projectID, 1)
			if hErr == nil && horizon != nil {
				conflict1, _, cErr := s.orch.SuggestChapterConflict(ctx, s.cfg.ReasoningModel, horizon)
				if cErr == nil && strings.TrimSpace(conflict1) != "" {
					_ = s.store.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
						ProjectID:    projectID,
						ChapterIndex: 1,
						Phase:        domain.CheckpointPhaseInit,
						CoreConflict: conflict1,
						UpdatedAt:    time.Now(),
					})
				}
			}
		}

		jsonResponse(w, http.StatusOK, p.Framework)
		return
	}

	// GET or PUT /api/projects/:id/framework
	switch r.Method {
	case http.MethodGet:
		if p.Framework == nil {
			jsonResponse(w, http.StatusOK, map[string]any{"framework": nil, "message": "总纲尚未推演生成"})
			return
		}
		jsonResponse(w, http.StatusOK, p.Framework)
	case http.MethodPut:
		var fw domain.ProjectFramework
		if err := json.NewDecoder(r.Body).Decode(&fw); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid framework JSON: "+err.Error())
			return
		}
		p.Framework = &fw
		if err := s.store.SaveProject(ctx, p); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, p.Framework)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectCodex(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()

	// 1. /api/projects/:id/codex (list or create)
	if len(subParts) == 0 {
		switch r.Method {
		case http.MethodGet:
			category := domain.CodexCategory(r.URL.Query().Get("category"))
			entries, err := s.store.ListCodexEntries(ctx, projectID, category)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if entries == nil {
				entries = []*domain.CodexEntry{}
			}
			jsonResponse(w, http.StatusOK, entries)
		case http.MethodPost:
			var entry domain.CodexEntry
			if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			entry.ProjectID = projectID
			if err := s.store.SaveCodexEntry(ctx, &entry); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, entry)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	sub := subParts[0]

	// 2. /api/projects/:id/codex/ai-generate
	if sub == "ai-generate" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req engine.CodexGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		entry, _, err := s.orch.GenerateCodexEntry(ctx, s.cfg.ReasoningModel, proj, req)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "ai generate codex entry failed: "+err.Error())
			return
		}
		if err := s.store.SaveCodexEntry(ctx, entry); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save generated codex entry failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, entry)
		return
	}

	// 3. /api/projects/:id/codex/scan
	if sub == "scan" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		entries, err := s.store.ListCodexEntries(ctx, projectID, "")
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		scanner := engine.NewMentionScanner()
		matched, mentions := scanner.ScanText(req.Text, entries)
		if matched == nil {
			matched = []*domain.CodexEntry{}
		}
		if mentions == nil {
			mentions = []engine.MatchedMention{}
		}
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"matched_entries": matched,
			"mentions":        mentions,
		})
		return
	}

	// 4. /api/projects/:id/codex/relations (GET or POST) & /api/projects/:id/codex/relations/:relId (DELETE)
	if sub == "relations" {
		if len(subParts) > 1 {
			if subParts[1] == "ai-extract" {
				if r.Method != http.MethodPost {
					errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				proj, err := s.store.GetProject(ctx, projectID)
				if err != nil {
					errorResponse(w, http.StatusNotFound, "project not found")
					return
				}
				entries, err := s.store.ListCodexEntries(ctx, projectID, "")
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				chapters, _ := s.store.ListChapters(ctx, projectID)
				var chapterText strings.Builder
				for _, ch := range chapters {
					chapterText.WriteString(fmt.Sprintf("第%d章 %s: %s\n", ch.ChapterIndex, ch.Title, ch.CoreConflict))
				}
				rels, _, err := s.orch.ExtractCodexRelations(ctx, s.cfg.ReasoningModel, proj, entries, chapterText.String())
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, "ai extract relations failed: "+err.Error())
					return
				}
				for i := range rels {
					_ = s.store.SaveCodexRelation(ctx, projectID, &rels[i])
				}
				jsonResponse(w, http.StatusOK, rels)
				return
			}

			relID := subParts[1]
			if r.Method == http.MethodDelete {
				if err := s.store.DeleteCodexRelation(ctx, projectID, relID); err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			}
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		switch r.Method {
		case http.MethodGet:
			entryID := r.URL.Query().Get("entry_id")
			rels, err := s.store.ListCodexRelations(ctx, projectID, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if rels == nil {
				rels = []domain.EntityRelation{}
			}
			jsonResponse(w, http.StatusOK, rels)
		case http.MethodPost:
			var rel domain.EntityRelation
			if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			rel.ProjectID = projectID
			if err := s.store.SaveCodexRelation(ctx, projectID, &rel); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, rel)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 4. /api/projects/:id/codex/:entryId
	entryID := sub
	if len(subParts) == 1 {
		switch r.Method {
		case http.MethodGet:
			entry, err := s.store.GetCodexEntry(ctx, projectID, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if entry == nil {
				errorResponse(w, http.StatusNotFound, "codex entry not found")
				return
			}
			jsonResponse(w, http.StatusOK, entry)
		case http.MethodPut:
			var entry domain.CodexEntry
			if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			entry.ID = entryID
			entry.ProjectID = projectID
			if err := s.store.SaveCodexEntry(ctx, &entry); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, entry)
		case http.MethodDelete:
			if err := s.store.DeleteCodexEntry(ctx, projectID, entryID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 5. /api/projects/:id/codex/:entryId/progressions
	if len(subParts) == 2 && subParts[1] == "progressions" {
		switch r.Method {
		case http.MethodGet:
			progs, err := s.store.ListCodexProgressions(ctx, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if progs == nil {
				progs = []domain.Progression{}
			}
			jsonResponse(w, http.StatusOK, progs)
		case http.MethodPost:
			var prog domain.Progression
			if err := json.NewDecoder(r.Body).Decode(&prog); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			if err := s.store.SaveCodexProgression(ctx, entryID, &prog); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, prog)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	errorResponse(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleProjectMatrix(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()
	if len(subParts) > 0 && subParts[0] == "ai-generate" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req engine.MatrixSceneGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		scenes, _, err := s.orch.GenerateMatrixScenes(ctx, s.cfg.ReasoningModel, proj, req)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "generate matrix scenes failed: "+err.Error())
			return
		}
		chapterID := ""
		chapters, _ := s.store.ListChapters(ctx, projectID)
		for _, ch := range chapters {
			if ch.ChapterIndex == req.ChapterIndex {
				chapterID = ch.ID
				break
			}
		}
		for i := range scenes {
			if chapterID != "" {
				scenes[i].ChapterID = chapterID
			} else {
				scenes[i].ChapterID = fmt.Sprintf("ch_%d", req.ChapterIndex)
			}
			_ = s.store.SaveScene(ctx, &scenes[i])
		}
		jsonResponse(w, http.StatusOK, scenes)
		return
	}

	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	overview, err := s.store.GetMatrixOverview(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, overview)
}

func (s *Server) handleProjectScenes(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()

	// 1. /api/projects/:id/scenes (list or create)
	if len(subParts) == 0 {
		switch r.Method {
		case http.MethodGet:
			chapterID := r.URL.Query().Get("chapter_id")
			var scenes []*domain.Scene
			if chapterID != "" {
				var err error
				scenes, err = s.store.ListScenes(ctx, chapterID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
			} else {
				chapters, err := s.store.ListChapters(ctx, projectID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				for _, ch := range chapters {
					scs, err := s.store.ListScenes(ctx, ch.ID)
					if err == nil {
						scenes = append(scenes, scs...)
					}
				}
			}
			if scenes == nil {
				scenes = []*domain.Scene{}
			}
			jsonResponse(w, http.StatusOK, scenes)
		case http.MethodPost:
			var sc domain.Scene
			if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			sc.ProjectID = projectID
			if sc.ID == "" {
				sc.ID = fmt.Sprintf("sc-%d", time.Now().UnixNano())
			}
			if err := sc.Validate(); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := s.store.SaveScene(ctx, &sc); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, sc)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 2. /api/projects/:id/scenes/:sceneId
	sceneID := subParts[0]
	if len(subParts) == 1 {
		switch r.Method {
		case http.MethodGet:
			sc, err := s.store.GetScene(ctx, sceneID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if sc == nil {
				errorResponse(w, http.StatusNotFound, "scene not found")
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodPut:
			var sc domain.Scene
			if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			sc.ID = sceneID
			sc.ProjectID = projectID
			if err := sc.Validate(); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := s.store.SaveScene(ctx, &sc); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodDelete:
			if err := s.store.DeleteScene(ctx, sceneID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 3. /api/projects/:id/scenes/:sceneId/markers
	if len(subParts) >= 2 && subParts[1] == "markers" {
		if len(subParts) == 2 {
			switch r.Method {
			case http.MethodGet:
				markers, err := s.store.ListSceneMarkers(ctx, sceneID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if markers == nil {
					markers = []domain.SceneMarker{}
				}
				jsonResponse(w, http.StatusOK, markers)
			case http.MethodPost:
				var m domain.SceneMarker
				if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
					errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
					return
				}
				m.SceneID = sceneID
				if m.ID == "" {
					m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
				}
				if err := s.store.SaveSceneMarker(ctx, &m); err != nil {
					errorResponse(w, http.StatusBadRequest, err.Error())
					return
				}
				jsonResponse(w, http.StatusCreated, m)
			default:
				errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(subParts) == 3 && r.Method == http.MethodDelete {
			markerID := subParts[2]
			if err := s.store.DeleteSceneMarker(ctx, markerID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}
	}

	errorResponse(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleSceneRoutes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rawPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/scenes/"), "/")
	if rawPath == "" {
		errorResponse(w, http.StatusBadRequest, "missing scene id")
		return
	}
	parts := strings.Split(rawPath, "/")
	sceneID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			sc, err := s.store.GetScene(ctx, sceneID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if sc == nil {
				errorResponse(w, http.StatusNotFound, "scene not found")
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodPut:
			var sc domain.Scene
			if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			sc.ID = sceneID
			if err := sc.Validate(); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := s.store.SaveScene(ctx, &sc); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodDelete:
			if err := s.store.DeleteScene(ctx, sceneID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if len(parts) >= 2 && parts[1] == "markers" {
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				markers, err := s.store.ListSceneMarkers(ctx, sceneID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if markers == nil {
					markers = []domain.SceneMarker{}
				}
				jsonResponse(w, http.StatusOK, markers)
			case http.MethodPost:
				var m domain.SceneMarker
				if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
					errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
					return
				}
				m.SceneID = sceneID
				if m.ID == "" {
					m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
				}
				if err := s.store.SaveSceneMarker(ctx, &m); err != nil {
					errorResponse(w, http.StatusBadRequest, err.Error())
					return
				}
				jsonResponse(w, http.StatusCreated, m)
			default:
				errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}
		if len(parts) == 3 && r.Method == http.MethodDelete {
			markerID := parts[2]
			if err := s.store.DeleteSceneMarker(ctx, markerID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}
	}

	errorResponse(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if len(subParts) == 0 {
		errorResponse(w, http.StatusBadRequest, "missing analytics type")
		return
	}

	ctx := r.Context()
	action := subParts[0]

	switch action {
	case "heatmap":
		entries, err := s.store.ListCodexEntries(ctx, projectID, "")
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		type ChapterMeta struct {
			ChapterIndex int    `json:"chapter_index"`
			Title        string `json:"title"`
			WordCount    int    `json:"word_count"`
		}
		type EntityHeatmapItem struct {
			EntryID       string               `json:"entry_id"`
			Name          string               `json:"name"`
			Category      domain.CodexCategory `json:"category"`
			ColorTag      string               `json:"color_tag"`
			TotalMentions int                  `json:"total_mentions"`
			ChapterCounts map[int]int          `json:"chapter_counts"`
		}

		var chapMetas []ChapterMeta
		for _, ch := range chapters {
			chapMetas = append(chapMetas, ChapterMeta{
				ChapterIndex: ch.ChapterIndex,
				Title:        ch.Title,
				WordCount:    ch.WordCount,
			})
		}

		scanner := engine.NewMentionScanner()
		itemsMap := make(map[string]*EntityHeatmapItem)
		for _, ent := range entries {
			itemsMap[ent.ID] = &EntityHeatmapItem{
				EntryID:       ent.ID,
				Name:          ent.Name,
				Category:      ent.Category,
				ColorTag:      ent.ColorTag,
				ChapterCounts: make(map[int]int),
			}
		}

		for _, ch := range chapters {
			_, mentions := scanner.ScanText(ch.Content, entries)
			for _, m := range mentions {
				if item, ok := itemsMap[m.Entry.ID]; ok {
					item.ChapterCounts[ch.ChapterIndex]++
					item.TotalMentions++
				}
			}
		}

		var heatmapItems []EntityHeatmapItem
		for _, ent := range entries {
			if item, ok := itemsMap[ent.ID]; ok {
				heatmapItems = append(heatmapItems, *item)
			}
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"chapters": chapMetas,
			"entities": heatmapItems,
		})

	case "tension":
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		type TensionPoint struct {
			ChapterIndex int    `json:"chapter_index"`
			ChapterTitle string `json:"chapter_title"`
			SceneIndex   int    `json:"scene_index"`
			SceneTitle   string `json:"scene_title"`
			TensionLevel int    `json:"tension_level"`
			DramaticGoal string `json:"dramatic_goal"`
			WordCount    int    `json:"word_count"`
		}

		var points []TensionPoint
		totalTension := 0

		for _, ch := range chapters {
			scenes, err := s.store.ListScenes(ctx, ch.ID)
			if err == nil && len(scenes) > 0 {
				for _, sc := range scenes {
					points = append(points, TensionPoint{
						ChapterIndex: ch.ChapterIndex,
						ChapterTitle: ch.Title,
						SceneIndex:   sc.SceneIndex,
						SceneTitle:   sc.Title,
						TensionLevel: sc.TensionLevel,
						DramaticGoal: sc.DramaticGoal,
						WordCount:    sc.WordCount,
					})
					totalTension += sc.TensionLevel
				}
			} else {
				// Fallback to chapter row if no atomic scenes yet
				points = append(points, TensionPoint{
					ChapterIndex: ch.ChapterIndex,
					ChapterTitle: ch.Title,
					SceneIndex:   1,
					SceneTitle:   ch.Title,
					TensionLevel: 5,
					DramaticGoal: "推进剧情",
					WordCount:    ch.WordCount,
				})
				totalTension += 5
			}
		}

		avgTension := 0.0
		if len(points) > 0 {
			avgTension = float64(totalTension) / float64(len(points))
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"points":      points,
			"avg_tension": avgTension,
		})

	default:
		errorResponse(w, http.StatusBadRequest, "unsupported analytics type")
	}
}
