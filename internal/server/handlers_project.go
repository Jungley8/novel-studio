package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListProjects(ctx)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if list == nil {
			list = []*domain.Project{}
		}
		jsonResponse(w, http.StatusOK, list)
	case http.MethodPost:
		var p domain.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("proj_%d", time.Now().UnixNano())
		}
		if err := s.store.SaveProject(ctx, &p); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, p)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.Split(path, "/")
	projectID := parts[0]
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "missing project id")
		return
	}

	ctx := r.Context()

	// 1. /api/projects/:id (GET / DELETE / POST)
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			p, err := s.store.GetProject(ctx, projectID)
			if err != nil {
				errorResponse(w, http.StatusNotFound, "project not found")
				return
			}
			jsonResponse(w, http.StatusOK, p)
		case http.MethodDelete:
			if err := s.store.DeleteProject(ctx, projectID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		case http.MethodPost:
			var p domain.Project
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			p.ID = projectID
			if err := s.store.SaveProject(ctx, &p); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, p)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	action := parts[1]
	switch action {
	case "suggest-conflict":
		s.handleSuggestConflict(w, r, projectID)
	case "chapters":
		if len(parts) >= 3 {
			if parts[2] == "suggest-conflict" {
				s.handleSuggestConflict(w, r, projectID)
				return
			}
			s.handleProjectChapterSub(w, r, projectID, parts[2:])
			return
		}
		s.handleProjectChapters(w, r, projectID)
	case "hooks":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectHooks(w, r, projectID, subParts)
	case "statemachine":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectStateMachine(w, r, projectID, subParts)
	case "derive-beats":
		s.handleDeriveBeats(w, r, projectID)
	case "render-scene":
		s.handleRenderScene(w, r, projectID)
	case "review-draft":
		s.handleReviewDraft(w, r, projectID)
	case "rewrite-draft":
		s.handleRewriteDraft(w, r, projectID)
	case "checkpoint":
		s.handleProjectCheckpoint(w, r, projectID)
	case "framework":
		s.handleProjectFramework(w, r, projectID, parts)
	case "codex":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectCodex(w, r, projectID, subParts)
	case "matrix":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectMatrix(w, r, projectID, subParts)
	case "scenes":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectScenes(w, r, projectID, subParts)
	case "prompt-preview":
		s.handlePromptPreview(w, r, projectID)
	case "chat":
		s.handleWorkshopChat(w, r, projectID)
	case "analytics":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleAnalytics(w, r, projectID, subParts)
	case "workshop":
		if len(parts) >= 3 && parts[2] == "produce" {
			s.handleWorkshopProduce(w, r, projectID)
			return
		}
		errorResponse(w, http.StatusBadRequest, "invalid workshop action")
	case "export":
		if len(parts) >= 3 {
			s.handleProjectExport(w, r, projectID, parts[2])
			return
		}
		errorResponse(w, http.StatusBadRequest, "invalid export format")
	case "harmonize":
		s.handleProjectHarmonize(w, r, projectID)
	case "suggest-human-touches":
		s.handleSuggestHumanTouches(w, r, projectID)
	case "sanitize-ai":
		s.handleSanitizeAI(w, r, projectID)
	default:
		errorResponse(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleProjectChapters(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if chapters == nil {
			chapters = []*domain.Chapter{}
		}
		jsonResponse(w, http.StatusOK, chapters)
	case http.MethodPost:
		var c domain.Chapter
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		c.ProjectID = projectID
		if c.ID == "" {
			c.ID = fmt.Sprintf("ch_%s_%d", projectID, c.ChapterIndex)
		}

		updatedProj, err := s.store.CommitChapter(ctx, projectID, &c)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		_ = s.store.ClearCheckpoint(ctx, projectID, c.ChapterIndex)

		jsonResponse(w, http.StatusCreated, map[string]any{
			"chapter": c,
			"project": updatedProj,
		})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectChapterSub(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	chapterIndex, err := strconv.Atoi(subParts[0])
	if err != nil || chapterIndex <= 0 {
		errorResponse(w, http.StatusBadRequest, "invalid chapter index")
		return
	}

	ctx := r.Context()

	// 1. /api/projects/:id/chapters/:index/uncommit (POST)
	if len(subParts) >= 2 && (subParts[1] == "uncommit" || subParts[1] == "revert-draft") {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		cp, updatedProj, err := s.store.UncommitChapter(ctx, projectID, chapterIndex)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"status":     "uncommitted",
			"message":    fmt.Sprintf("第 %d 章已成功从正史撤回为草稿", chapterIndex),
			"checkpoint": cp,
			"project":    updatedProj,
		})
		return
	}

	// 2. /api/projects/:id/chapters/:index (GET / DELETE)
	switch r.Method {
	case http.MethodGet:
		chapter, err := s.store.GetChapter(ctx, projectID, chapterIndex)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "chapter not found")
			return
		}
		jsonResponse(w, http.StatusOK, chapter)
	case http.MethodDelete:
		toDraft := r.URL.Query().Get("to_draft") == "true"
		if toDraft {
			cp, updatedProj, err := s.store.UncommitChapter(ctx, projectID, chapterIndex)
			if err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]any{
				"status":     "uncommitted",
				"message":    fmt.Sprintf("第 %d 章已成功撤回为草稿", chapterIndex),
				"checkpoint": cp,
				"project":    updatedProj,
			})
			return
		}

		if err := s.store.DeleteChapter(ctx, projectID, chapterIndex); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"status":  "deleted",
			"message": fmt.Sprintf("第 %d 章已成功删除", chapterIndex),
		})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectCheckpoint(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		idxStr := r.URL.Query().Get("chapter_index")
		idx, _ := strconv.Atoi(idxStr)
		if idx <= 0 {
			idx = 1
		}
		cp, err := s.store.GetCheckpoint(ctx, projectID, idx)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if cp == nil {
			jsonResponse(w, http.StatusOK, nil)
			return
		}
		jsonResponse(w, http.StatusOK, cp)
	case http.MethodPost:
		var cp domain.ChapterCheckpoint
		if err := json.NewDecoder(r.Body).Decode(&cp); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		cp.ProjectID = projectID
		if cp.ChapterIndex <= 0 {
			errorResponse(w, http.StatusBadRequest, "invalid chapter_index")
			return
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = time.Now()
		}
		if err := s.store.SaveCheckpoint(ctx, &cp); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, cp)
	case http.MethodDelete:
		idxStr := r.URL.Query().Get("chapter_index")
		idx, _ := strconv.Atoi(idxStr)
		if idx <= 0 {
			idx = 1
		}
		if err := s.store.ClearCheckpoint(ctx, projectID, idx); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "cleared"})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectHooks(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()
	if len(subParts) > 0 && subParts[0] == "ai-extract" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		chapters, _ := s.store.ListChapters(ctx, projectID)
		existingHooks, _ := s.store.ListPlotHooks(ctx, projectID)
		newHooks, _, err := s.orch.ExtractPlotHooks(ctx, s.cfg.ReasoningModel, proj, chapters, existingHooks)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "ai extract plot hooks failed: "+err.Error())
			return
		}
		for i := range newHooks {
			_ = s.store.SavePlotHook(ctx, &newHooks[i])
		}
		jsonResponse(w, http.StatusOK, newHooks)
		return
	}

	switch r.Method {
	case http.MethodGet:
		hooks, err := s.store.ListPlotHooks(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if hooks == nil {
			hooks = []*domain.PlotHook{}
		}
		jsonResponse(w, http.StatusOK, hooks)
	case http.MethodPost:
		var h domain.PlotHook
		if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		h.ProjectID = projectID
		if h.ID == "" {
			h.ID = fmt.Sprintf("hook_%d", time.Now().UnixNano())
		}
		if err := s.store.SavePlotHook(ctx, &h); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, h)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectStateMachine(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()
	if len(subParts) > 0 && subParts[0] == "ai-analyze" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		updated, _, err := s.orch.AnalyzeProtagonistState(ctx, s.cfg.ReasoningModel, proj, chapters)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "analyze protagonist state failed: "+err.Error())
			return
		}
		proj.Protagonist = *updated
		if err := s.store.SaveProject(ctx, proj); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save project failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, proj.Protagonist)
		return
	}

	switch r.Method {
	case http.MethodGet:
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		jsonResponse(w, http.StatusOK, proj.Protagonist)
	case http.MethodPost:
		var prot domain.Protagonist
		if err := json.NewDecoder(r.Body).Decode(&prot); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		proj.Protagonist = prot
		if err := s.store.SaveProject(ctx, proj); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save project failed: "+err.Error())
			return
		}

		protID := fmt.Sprintf("codex_%s_pro", projectID)
		protEntry, _ := s.store.GetCodexEntry(ctx, projectID, protID)
		if protEntry != nil {
			protName := "主角"
			if parts := strings.Split(prot.NameAndLevel, "("); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
				protName = strings.TrimSpace(parts[0])
			}
			protEntry.Name = protName
			protEntry.Aliases = []string{"主角", prot.NameAndLevel}
			protEntry.Summary = fmt.Sprintf("全书主角 | 状态: %s | 动机: %s", prot.HealthStatus, prot.CoreGoal)
			protEntry.DetailsMarkdown = fmt.Sprintf("【主角实时档案】\n- 设定/境界：%s\n- 核心长线追求：%s\n- 随身底牌与物品：%s\n- 当前状态：%s", prot.NameAndLevel, prot.CoreGoal, prot.Inventory, prot.HealthStatus)
			protEntry.UpdatedAt = time.Now()
			_ = s.store.SaveCodexEntry(ctx, protEntry)
		}

		jsonResponse(w, http.StatusOK, proj.Protagonist)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleHookDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	hookID := strings.TrimPrefix(r.URL.Path, "/api/hooks/")
	if err := s.store.DeletePlotHook(r.Context(), hookID); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type BootstrapProjectRequest struct {
	Title          string `json:"title"`
	TargetPlatform string `json:"target_platform"`
	Concept        string `json:"concept"`
	ReasoningModel string `json:"reasoning_model,omitempty"`
}

func (s *Server) handleBootstrapProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req BootstrapProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		errorResponse(w, http.StatusBadRequest, "project title cannot be empty")
		return
	}
	reasoningModel := req.ReasoningModel
	if reasoningModel == "" {
		reasoningModel = s.cfg.ReasoningModel
	}

	fw, _, err := s.orch.BootstrapFramework(r.Context(), reasoningModel, engine.FrameworkBootstrapRequest{
		Title:          req.Title,
		TargetPlatform: req.TargetPlatform,
		CoreConcept:    req.Concept,
	})
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "bootstrap framework failed: "+err.Error())
		return
	}

	projectID := fmt.Sprintf("proj_%d", time.Now().UnixNano())
	var worldRules strings.Builder
	worldRules.WriteString("【世界底层规则与核心公理】\n")
	for i, axiom := range fw.WorldAxioms {
		worldRules.WriteString(fmt.Sprintf("%d. %s\n", i+1, axiom))
	}
	if len(fw.PowerLadder) > 0 {
		worldRules.WriteString("\n【战力阶梯与能力代价】\n")
		for _, tier := range fw.PowerLadder {
			worldRules.WriteString(fmt.Sprintf("- %s: 关隘[%s] | 代价[%s]\n", tier.Realm, tier.Bottleneck, tier.Drawback))
		}
	}

	initialRealm := "初入门槛"
	if len(fw.PowerLadder) > 0 && fw.PowerLadder[0].Realm != "" {
		initialRealm = fw.PowerLadder[0].Realm
	}
	proj := &domain.Project{
		ID:             projectID,
		Title:          req.Title,
		TargetPlatform: req.TargetPlatform,
		WorldRules:     strings.TrimSpace(worldRules.String()),
		Protagonist: domain.Protagonist{
			NameAndLevel: fmt.Sprintf("主角 (%s)", initialRealm),
			Inventory:    "随身物品待起草",
			CoreGoal:     fw.ThemePremise,
			HealthStatus: "良好",
		},
		Framework: fw,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.store.SaveProject(r.Context(), proj); err != nil {
		errorResponse(w, http.StatusInternalServerError, "save project failed: "+err.Error())
		return
	}

	// 同步主角至设定集 (Codex)
	protName := "主角"
	if parts := strings.Split(proj.Protagonist.NameAndLevel, "("); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		protName = strings.TrimSpace(parts[0])
	}
	protEntry := &domain.CodexEntry{
		ID:              fmt.Sprintf("codex_%s_pro", projectID),
		ProjectID:       projectID,
		Category:        domain.CategoryCharacter,
		Name:            protName,
		Aliases:         []string{"主角", proj.Protagonist.NameAndLevel},
		Summary:         fmt.Sprintf("全书主角 | 初始境界: %s | 核心长线动机: %s", initialRealm, fw.ThemePremise),
		DetailsMarkdown: fmt.Sprintf("【全书核心主角档案】\n- 初始境界：%s\n- 核心终极目标：%s\n- 随身底牌与物品：%s\n- 当前状态：%s", initialRealm, proj.Protagonist.CoreGoal, proj.Protagonist.Inventory, proj.Protagonist.HealthStatus),
		TrackingMode:    domain.TrackingModeAutoMention,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	_ = s.store.SaveCodexEntry(r.Context(), protEntry)

	// 同步势力阵营至设定集
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
		_ = s.store.SaveCodexEntry(r.Context(), entry)
	}

	// 同步关键人物至设定集
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
		_ = s.store.SaveCodexEntry(r.Context(), entry)
	}

	for i, sh := range fw.SeedHooks {
		hook := &domain.PlotHook{
			ID:             fmt.Sprintf("hook_%s_%d", projectID, i+1),
			ProjectID:      projectID,
			Title:          sh.Title,
			Details:        sh.Details,
			CreatedChapter: sh.CreatedChapter,
			TargetChapter:  sh.TargetChapter,
			Status:         domain.HookStatusOpen,
			CreatedAt:      time.Now(),
		}
		_ = s.store.SavePlotHook(r.Context(), hook)
	}

	// 全自动级联关系图谱推演 (Extract Codex Relations Cascade)
	codexEntries, _ := s.store.ListCodexEntries(r.Context(), projectID, "")
	if len(codexEntries) >= 2 && s.orch != nil {
		relations, _, relErr := s.orch.ExtractCodexRelations(
			r.Context(),
			s.cfg.ReasoningModel,
			proj,
			codexEntries,
			proj.WorldRules+"\n\n全书核心立意："+fw.ThemePremise,
		)
		if relErr == nil && len(relations) > 0 {
			for _, rel := range relations {
				_ = s.store.SaveCodexRelation(r.Context(), projectID, &rel)
			}
		}
	}

	// 全自动推演第 1 卷第 1 章开局黄金冲突并初始化草稿检查点
	if s.chronicle != nil && s.orch != nil {
		horizon, hErr := s.chronicle.AssembleHorizon(r.Context(), projectID, 1)
		if hErr == nil && horizon != nil {
			conflict1, _, cErr := s.orch.SuggestChapterConflict(r.Context(), s.cfg.ReasoningModel, horizon)
			if cErr == nil && strings.TrimSpace(conflict1) != "" {
				_ = s.store.SaveCheckpoint(r.Context(), &domain.ChapterCheckpoint{
					ProjectID:    projectID,
					ChapterIndex: 1,
					Phase:        domain.CheckpointPhaseInit,
					CoreConflict: conflict1,
					UpdatedAt:    time.Now(),
				})
			}
		}
	}

	jsonResponse(w, http.StatusCreated, proj)
}

func (s *Server) handleProjectExport(w http.ResponseWriter, r *http.Request, projectID, format string) {
	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	chapters, _ := s.store.ListChapters(ctx, projectID)
	hooks, _ := s.store.ListPlotHooks(ctx, projectID)

	switch format {
	case "markdown":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", p.Title))
		sb.WriteString(fmt.Sprintf("> 目标平台: %s | 导出时间: %s\n\n", p.TargetPlatform, time.Now().Format("2006-01-02 15:04:05")))
		sb.WriteString("## 世界公理与设定\n\n" + p.WorldRules + "\n\n---\n\n")

		for _, c := range chapters {
			sb.WriteString(fmt.Sprintf("## 第 %d 章: %s\n\n", c.ChapterIndex, c.Title))
			sb.WriteString(c.Content + "\n\n---\n\n")
		}

		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_full.md", p.Title))
		_, _ = w.Write([]byte(sb.String()))

	case "json":
		snapshot := map[string]any{
			"project":   p,
			"chapters":  chapters,
			"hooks":     hooks,
			"export_at": time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_snapshot.json", p.Title))
		_ = json.NewEncoder(w).Encode(snapshot)

	default:
		errorResponse(w, http.StatusBadRequest, "unsupported export format")
	}
}
