package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
	"github.com/Jungley8/novel-studio/web"
)

type Server struct {
	cfg         *config.Config
	configPath  string
	store       store.Store
	llmClient   *engine.HTTPLLMClient
	llmRouter   *engine.LLMRouter
	orch        *engine.Orchestrator
	linter      *engine.Linter
	chronicle   *engine.CanonChronicle
	qualityGate *engine.QualityGate
	workshop    *engine.ChapterWorkshop
	mux         *http.ServeMux
	fs          fs.FS
}

func New(
	cfg *config.Config,
	configPath string,
	s store.Store,
	llmClient *engine.HTTPLLMClient,
	orch *engine.Orchestrator,
	linter *engine.Linter,
) (*Server, error) {
	webFS, err := web.FS()
	if err != nil {
		return nil, fmt.Errorf("load web embed FS failed: %w", err)
	}

	if linter == nil {
		linter = engine.NewLinter(nil)
	}

	var router *engine.LLMRouter
	if llmClient != nil {
		router = engine.NewLLMRouter(llmClient)
		router.UpdateFromConfig(cfg)
	}

	chronicle := engine.NewCanonChronicle(s)
	var qgClient engine.LLMClient = llmClient
	if router != nil {
		qgClient = router
		if orch != nil {
			orch.SetClient(router)
		}
	}
	qualityGate := engine.NewQualityGate(qgClient, nil)
	workshop := engine.NewChapterWorkshop(orch, chronicle, qualityGate, s)

	srv := &Server{
		cfg:         cfg,
		configPath:  configPath,
		store:       s,
		llmClient:   llmClient,
		llmRouter:   router,
		orch:        orch,
		linter:      linter,
		chronicle:   chronicle,
		qualityGate: qualityGate,
		workshop:    workshop,
		mux:         http.NewServeMux(),
		fs:          webFS,
	}

	srv.routes()
	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Simple CORS for local development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// API routes
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/linter/analyze", s.handleLinter)
	s.mux.HandleFunc("/api/projects/bootstrap", s.handleBootstrapProject)
	s.mux.HandleFunc("/api/projects", s.handleProjects)
	s.mux.HandleFunc("/api/projects/", s.handleProjectRoutes)
	s.mux.HandleFunc("/api/scenes/", s.handleSceneRoutes)
	s.mux.HandleFunc("/api/workshop/inline-action", s.handleInlineAction)
	s.mux.HandleFunc("/api/hooks/", s.handleHookDelete)

	// Static & SPA routes
	fileServer := http.FileServer(http.FS(s.fs))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Check if file exists in embed FS
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(s.fs, path); err != nil {
			// SPA fallback
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

func maskKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "********"
	}
	return k[:3] + "..." + k[len(k)-4:]
}

func (s *Server) maskedConfig() config.Config {
	cpy := *s.cfg
	cpy.APIKey = maskKey(cpy.APIKey)
	if cpy.ReviewerProvider != nil {
		rcpy := *cpy.ReviewerProvider
		rcpy.APIKey = maskKey(rcpy.APIKey)
		cpy.ReviewerProvider = &rcpy
	}
	if cpy.WriterProvider != nil {
		wcpy := *cpy.WriterProvider
		wcpy.APIKey = maskKey(wcpy.APIKey)
		cpy.WriterProvider = &wcpy
	}
	if cpy.ReasonerProvider != nil {
		rpcpy := *cpy.ReasonerProvider
		rpcpy.APIKey = maskKey(rpcpy.APIKey)
		cpy.ReasonerProvider = &rpcpy
	}
	return cpy
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, s.maskedConfig())
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		var updated config.Config
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid config JSON: "+err.Error())
			return
		}

		s.cfg.APIBase = updated.APIBase
		if updated.APIKey != "" && !strings.Contains(updated.APIKey, "...") && !strings.Contains(updated.APIKey, "***") {
			s.cfg.APIKey = updated.APIKey
		}
		s.cfg.ReasoningModel = updated.ReasoningModel
		s.cfg.WriterModel = updated.WriterModel
		if updated.ReviewerModel != "" {
			s.cfg.ReviewerModel = updated.ReviewerModel
		}

		// Update independent providers if specified
		if updated.ReviewerProvider != nil {
			if s.cfg.ReviewerProvider == nil {
				s.cfg.ReviewerProvider = &config.ProviderConfig{}
			}
			s.cfg.ReviewerProvider.APIBase = updated.ReviewerProvider.APIBase
			s.cfg.ReviewerProvider.Model = updated.ReviewerProvider.Model
			if updated.ReviewerProvider.APIKey != "" && !strings.Contains(updated.ReviewerProvider.APIKey, "...") && !strings.Contains(updated.ReviewerProvider.APIKey, "***") {
				s.cfg.ReviewerProvider.APIKey = updated.ReviewerProvider.APIKey
			}
		}

		if updated.WriterProvider != nil {
			if s.cfg.WriterProvider == nil {
				s.cfg.WriterProvider = &config.ProviderConfig{}
			}
			s.cfg.WriterProvider.APIBase = updated.WriterProvider.APIBase
			s.cfg.WriterProvider.Model = updated.WriterProvider.Model
			if updated.WriterProvider.APIKey != "" && !strings.Contains(updated.WriterProvider.APIKey, "...") && !strings.Contains(updated.WriterProvider.APIKey, "***") {
				s.cfg.WriterProvider.APIKey = updated.WriterProvider.APIKey
			}
		}

		if updated.ReasonerProvider != nil {
			if s.cfg.ReasonerProvider == nil {
				s.cfg.ReasonerProvider = &config.ProviderConfig{}
			}
			s.cfg.ReasonerProvider.APIBase = updated.ReasonerProvider.APIBase
			s.cfg.ReasonerProvider.Model = updated.ReasonerProvider.Model
			if updated.ReasonerProvider.APIKey != "" && !strings.Contains(updated.ReasonerProvider.APIKey, "...") && !strings.Contains(updated.ReasonerProvider.APIKey, "***") {
				s.cfg.ReasonerProvider.APIKey = updated.ReasonerProvider.APIKey
			}
		}

		if s.llmClient != nil {
			s.llmClient.UpdateCredentials(s.cfg.APIBase, s.cfg.APIKey)
		}
		if s.llmRouter != nil {
			s.llmRouter.UpdateFromConfig(s.cfg)
		}

		if err := s.cfg.Save(s.configPath); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save config failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, s.maskedConfig())
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleLinter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	res := s.linter.Analyze(req.Text)
	jsonResponse(w, http.StatusOK, res)
}

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
	case "chapters":
		s.handleProjectChapters(w, r, projectID)
	case "hooks":
		s.handleProjectHooks(w, r, projectID)
	case "derive-beats":
		s.handleDeriveBeats(w, r, projectID)
	case "render-scene":
		s.handleRenderScene(w, r, projectID)
	case "review-draft":
		s.handleReviewDraft(w, r, projectID)
	case "rewrite-draft":
		s.handleRewriteDraft(w, r, projectID)
	case "framework":
		s.handleProjectFramework(w, r, projectID, parts)
	case "codex":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectCodex(w, r, projectID, subParts)
	case "matrix":
		s.handleProjectMatrix(w, r, projectID)
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

		jsonResponse(w, http.StatusCreated, map[string]any{
			"chapter": c,
			"project": updatedProj,
		})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectHooks(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
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

func (s *Server) handleDeriveBeats(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int    `json:"chapter_index"`
		CoreConflict string `json:"core_conflict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
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

	out, err := s.orch.DeriveBeatsWithHorizon(r.Context(), s.cfg.ReasoningModel, horizon, req.CoreConflict)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, out)
}

func (s *Server) handleRenderScene(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int                `json:"chapter_index"`
		Beats        []domain.SceneBeat `json:"beats"`
		WordsTarget  int                `json:"words_target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	if s.orch == nil {
		errorResponse(w, http.StatusInternalServerError, "narrative orchestrator is not configured")
		return
	}

	content, _, err := s.orch.RenderScene(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.Beats, req.WordsTarget)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"content": content})
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "novel-studio",
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleReviewDraft(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int                `json:"chapter_index"`
		Beats        []domain.SceneBeat `json:"beats"`
		DraftText    string             `json:"draft_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	if s.qualityGate == nil {
		errorResponse(w, http.StatusInternalServerError, "quality gate not configured")
		return
	}

	audit, _, err := s.qualityGate.Audit(r.Context(), s.cfg.ReasoningModel, p, req.ChapterIndex, req.Beats, req.DraftText)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, audit.ToReviewResult())
}

func (s *Server) handleWorkshopProduce(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var req engine.WorkshopProduceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	req.ProjectID = projectID
	if req.ReasoningModel == "" {
		req.ReasoningModel = s.cfg.ReasoningModel
	}
	if req.WriterModel == "" {
		req.WriterModel = s.cfg.WriterModel
	}
	if req.ReviewerModel == "" {
		req.ReviewerModel = s.cfg.ReviewerModel
		if req.ReviewerModel == "" {
			req.ReviewerModel = s.cfg.ReasoningModel
		}
	}

	if s.workshop == nil {
		errorResponse(w, http.StatusInternalServerError, "workshop not configured")
		return
	}

	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.URL.Query().Get("stream") == "true"
	if isSSE {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		req.OnProgress = func(ev engine.WorkshopEvent) {
			evBytes, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "event: progress\ndata: %s\n\n", string(evBytes))
			flusher.Flush()
		}

		res, err := s.workshop.ProduceChapter(r.Context(), req)
		if err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errPayload))
			flusher.Flush()
			return
		}

		resultPayload, _ := json.Marshal(res)
		_, _ = fmt.Fprintf(w, "event: complete\ndata: %s\n\n", string(resultPayload))
		flusher.Flush()
		return
	}

	res, err := s.workshop.ProduceChapter(r.Context(), req)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, res)
}

func (s *Server) handleRewriteDraft(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex  int                  `json:"chapter_index"`
		OriginalDraft string               `json:"original_draft"`
		Review        *domain.ReviewResult `json:"review"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Review == nil {
		errorResponse(w, http.StatusBadRequest, "review result is required for rewriting")
		return
	}

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	if s.orch == nil {
		errorResponse(w, http.StatusInternalServerError, "orchestrator not configured")
		return
	}

	rewritten, _, err := s.orch.RewriteDraft(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.OriginalDraft, req.Review)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"content": rewritten})
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
	worldRules.WriteString("【天道法则与核心世界公理】\n")
	for i, axiom := range fw.WorldAxioms {
		worldRules.WriteString(fmt.Sprintf("%d. %s\n", i+1, axiom))
	}
	if len(fw.PowerLadder) > 0 {
		worldRules.WriteString("\n【战力阶梯与突破反噬】\n")
		for _, tier := range fw.PowerLadder {
			worldRules.WriteString(fmt.Sprintf("- %s: 关隘[%s] | 代价[%s]\n", tier.Realm, tier.Bottleneck, tier.Drawback))
		}
	}

	initialRealm := "练气一层"
	if len(fw.PowerLadder) > 0 {
		initialRealm = fw.PowerLadder[0].Realm
	}
	proj := &domain.Project{
		ID:             projectID,
		Title:          req.Title,
		TargetPlatform: req.TargetPlatform,
		WorldRules:     strings.TrimSpace(worldRules.String()),
		Protagonist: domain.Protagonist{
			NameAndLevel: fmt.Sprintf("主角 (%s)", initialRealm),
			Inventory:    "残破黑铁, 粗布短衫",
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

	// Auto-seed initial plot hooks into plot_hooks store
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

	jsonResponse(w, http.StatusCreated, proj)
}

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
		if err := s.store.SaveProject(ctx, p); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
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
		jsonResponse(w, http.StatusOK, p)
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

	// 2. /api/projects/:id/codex/scan
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

	// 3. /api/projects/:id/codex/relations (GET or POST) & /api/projects/:id/codex/relations/:relId (DELETE)
	if sub == "relations" {
		if len(subParts) > 1 {
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

// -------------------------------------------------------------------------
// The Matrix & Scenes Handlers
// -------------------------------------------------------------------------

func (s *Server) handleProjectMatrix(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	overview, err := s.store.GetMatrixOverview(r.Context(), projectID)
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

// -------------------------------------------------------------------------
// Manuscript Inline Actions & Transparent Prompt Preview
// -------------------------------------------------------------------------

type InlineActionRequest struct {
	Action             string `json:"action"` // rewrite, expand, shorten, sensory, dialogue, custom
	Selection          string `json:"selection"`
	Instruction        string `json:"instruction,omitempty"`
	SurroundingContext string `json:"surrounding_context,omitempty"`
	Model              string `json:"model,omitempty"`
}

type InlineActionResponse struct {
	Result     string            `json:"result"`
	Action     string            `json:"action"`
	TokensUsed engine.TokenUsage `json:"tokens_used"`
}

func (s *Server) handleInlineAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req InlineActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	req.Selection = strings.TrimSpace(req.Selection)
	if req.Selection == "" {
		errorResponse(w, http.StatusBadRequest, "selection cannot be empty")
		return
	}

	if s.llmRouter == nil {
		errorResponse(w, http.StatusInternalServerError, "llm client is not configured")
		return
	}

	var sysPrompt string
	switch req.Action {
	case "rewrite":
		sysPrompt = `你是一名专业文学小说责编与润色大师。请在严格保留核心剧情、人物立场与事实不变的前提下，润色重写所选段落，消除行文冗余与粗糙感，提升语感、节奏和文学张力。严禁输出多余解释或寒暄，仅输出重写后的正文。`
	case "expand":
		sysPrompt = `你是一名具有丰富细节感知力的小说作家。请对所选段落进行生动的细节扩写，丰富微表情、环境声色、内心独白与五感氛围，强化现场感与代入感。严禁输出多余解释，仅输出扩写后的正文。`
	case "shorten":
		sysPrompt = `你是一名犀利严苛的小说编辑。请对所选段落进行精简浓缩，去除多余修饰词、套话及次要描写，保留戏剧冲突与高光时刻，使行文更加紧凑精炼。严禁输出多余解释，仅输出精简后的正文。`
	case "sensory":
		sysPrompt = `你是一名专注于五感沉浸的文学名家。请深入发掘所选段落中的视觉、听觉、嗅觉、触觉及生理体感，用极具质感的物态细节增强沉浸感，严禁使用陈词滥调。仅输出具象化增强后的正文。`
	case "dialogue":
		sysPrompt = `你是一名顶尖电影与小说台词名家。请润色打磨所选段落中的人物对话，强化角色口吻性格特质、言语潜台词与机锋对峙，削弱说明性台词。仅输出打磨后的对话正文。`
	default:
		sysPrompt = `你是一名专业小说创作助理。请根据用户的特别指令对选中文本进行处理。严禁多余废话，仅输出处理后的正文。`
	}

	var userPrompt strings.Builder
	if req.SurroundingContext != "" {
		userPrompt.WriteString("【上下文参考】\n")
		userPrompt.WriteString(req.SurroundingContext)
		userPrompt.WriteString("\n\n")
	}
	if req.Instruction != "" {
		userPrompt.WriteString("【修改要求】\n")
		userPrompt.WriteString(req.Instruction)
		userPrompt.WriteString("\n\n")
	}
	userPrompt.WriteString("【待处理文本】\n")
	userPrompt.WriteString(req.Selection)

	targetModel := req.Model
	if targetModel == "" {
		targetModel = s.cfg.WriterModel
	}

	resultText, usage, err := s.llmRouter.ChatCompletionForRole(r.Context(), engine.RoleWriter, targetModel, sysPrompt, userPrompt.String(), 0.7)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, InlineActionResponse{
		Result:     strings.TrimSpace(resultText),
		Action:     req.Action,
		TokensUsed: usage,
	})
}

type PromptPreviewRequest struct {
	ChapterIndex int      `json:"chapter_index"`
	SceneTitle   string   `json:"scene_title,omitempty"`
	POV          string   `json:"pov,omitempty"`
	DramaticGoal string   `json:"dramatic_goal,omitempty"`
	Conflict     string   `json:"conflict,omitempty"`
	CoreEvents   []string `json:"core_events,omitempty"`
}

type PromptComponentPreview struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Content       string `json:"content"`
	TokenEstimate int    `json:"token_estimate"`
}

type PromptPreviewResponse struct {
	SystemPrompt        string                   `json:"system_prompt"`
	UserPrompt          string                   `json:"user_prompt"`
	Components          []PromptComponentPreview `json:"components"`
	TotalTokensEstimate int                      `json:"total_tokens_estimate"`
}

func estimateTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	toks := len(runes) * 3 / 4
	if toks < 1 {
		return 1
	}
	return toks
}

func (s *Server) handlePromptPreview(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PromptPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.ChapterIndex <= 0 {
		req.ChapterIndex = 1
	}

	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil || p == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	horizon, err := s.chronicle.AssembleHorizon(ctx, projectID, req.ChapterIndex)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "assemble horizon: "+err.Error())
		return
	}

	// 1. Framework & Style Guidelines component
	frameworkContent := fmt.Sprintf("【世界观与公理】\n%s\n\n【目标平台调性】\n%s", p.WorldRules, p.TargetPlatform)
	if p.Framework != nil && p.Framework.ThemePremise != "" {
		frameworkContent += fmt.Sprintf("\n\n【核心立意与主线】\n%s", p.Framework.ThemePremise)
	}

	// 2. Canon Horizon component
	canonContent := fmt.Sprintf("【前序章节剧情记忆】\n%s", horizon.RollingCanonText)
	if horizon.TailAnchor != "" {
		canonContent += fmt.Sprintf("\n\n【上章末尾腔调锚定】\n%s", horizon.TailAnchor)
	}

	// 3. The Codex Component
	codexContent := horizon.CodexContextText
	if codexContent == "" {
		codexContent = "(当前章节未匹配到需强制注入的百科实体)"
	}

	// 4. Scene Beat Specification
	var sceneSpec strings.Builder
	sceneSpec.WriteString(fmt.Sprintf("【目标章节】第 %d 章\n", req.ChapterIndex))
	if req.SceneTitle != "" {
		sceneSpec.WriteString(fmt.Sprintf("【场景标题】%s\n", req.SceneTitle))
	}
	if req.POV != "" {
		sceneSpec.WriteString(fmt.Sprintf("【视角人物(POV)】%s\n", req.POV))
	}
	if req.DramaticGoal != "" {
		sceneSpec.WriteString(fmt.Sprintf("【戏剧目标】%s\n", req.DramaticGoal))
	}
	if req.Conflict != "" {
		sceneSpec.WriteString(fmt.Sprintf("【阻碍与冲突】%s\n", req.Conflict))
	}
	if len(req.CoreEvents) > 0 {
		sceneSpec.WriteString("【核心事件节拍】\n")
		for i, ev := range req.CoreEvents {
			sceneSpec.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ev))
		}
	}

	components := []PromptComponentPreview{
		{
			Name:          "世界观与风格公理 (Global Framework)",
			Description:   "包含世界设定、平台调性约束与核心爽点规则",
			Content:       frameworkContent,
			TokenEstimate: estimateTokens(frameworkContent),
		},
		{
			Name:          "因果故事线记忆 (Canon Horizon)",
			Description:   "包含前序章节滚雪球摘要与上章尾部文风锚点",
			Content:       canonContent,
			TokenEstimate: estimateTokens(canonContent),
		},
		{
			Name:          "全域百科与关联 (The Codex Injected)",
			Description:   "动态扫描注入的角色阶段属性、别名、物品状态及两两关系",
			Content:       codexContent,
			TokenEstimate: estimateTokens(codexContent),
		},
		{
			Name:          "场景戏剧规格 (Scene Dramaturgy)",
			Description:   "当前场次的 POV、戏剧目标、冲突阻碍与核心节拍事实",
			Content:       sceneSpec.String(),
			TokenEstimate: estimateTokens(sceneSpec.String()),
		},
	}

	totalTokens := 0
	for _, c := range components {
		totalTokens += c.TokenEstimate
	}

	systemPrompt := `你是一名冷峻、极具电影镜头感的小说名家。严格遵循因果设定与声口约束，杜绝AI套话与伪高潮。`
	var userPrompt strings.Builder
	userPrompt.WriteString(frameworkContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(canonContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(codexContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(sceneSpec.String())

	jsonResponse(w, http.StatusOK, PromptPreviewResponse{
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt.String(),
		Components:          components,
		TotalTokensEstimate: totalTokens,
	})
}

// -------------------------------------------------------------------------
// Workshop Contextual Chat & Analytics
// -------------------------------------------------------------------------

type WorkshopChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

type WorkshopChatRequest struct {
	Role         string                `json:"role"` // scene_brainstorm, character_roleplay, editor_critique
	CharacterID  string                `json:"character_id,omitempty"`
	ChapterIndex int                   `json:"chapter_index,omitempty"`
	Messages     []WorkshopChatMessage `json:"messages"`
	Model        string                `json:"model,omitempty"`
}

type WorkshopChatResponse struct {
	Message    WorkshopChatMessage `json:"message"`
	TokensUsed engine.TokenUsage   `json:"tokens_used"`
}

func (s *Server) handleWorkshopChat(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req WorkshopChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if len(req.Messages) == 0 {
		errorResponse(w, http.StatusBadRequest, "messages cannot be empty")
		return
	}

	if s.llmRouter == nil {
		errorResponse(w, http.StatusInternalServerError, "llm client is not configured")
		return
	}

	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil || p == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	var sysPrompt string
	switch req.Role {
	case "character_roleplay":
		if req.CharacterID != "" {
			entry, err := s.store.GetCodexEntry(ctx, projectID, req.CharacterID)
			if err == nil && entry != nil {
				prog := entry.ActiveProgression(req.ChapterIndex)
				rels, _ := s.store.ListCodexRelations(ctx, projectID, entry.ID)
				var relsStr strings.Builder
				for _, rel := range rels {
					relsStr.WriteString(fmt.Sprintf("- 对【%s】关系: %s (%s)\n", rel.TargetName, rel.RelationType, rel.Description))
				}
				sysPrompt = fmt.Sprintf(`你是小说《%s》中的角色【%s】（别名：%s）。
【角色简介】%s
【详细设定】%s
【当前章节阶段（第 %d 章）】%s
【当前人际关系】
%s
【角色指令】你必须严格代入该角色的第一人称口吻、思维方式、情绪立场和知识边界与创作者进行交谈。严禁跳戏（No breaking character）。`,
					p.Title, entry.Name, strings.Join(entry.Aliases, "、"),
					entry.Summary, entry.DetailsMarkdown,
					req.ChapterIndex, prog.Notes, relsStr.String())
			}
		}
		if sysPrompt == "" {
			sysPrompt = fmt.Sprintf(`你是小说《%s》中的核心角色。请根据上下文保持角色口吻与世界观设定进行对话。`, p.Title)
		}
	case "scene_brainstorm":
		sysPrompt = fmt.Sprintf(`你是小说《%s》的资深剧情策划与头脑风暴助理。
世界观规则：%s
你的职责是协助作者攻克卡文瓶颈、设计反转桥段、推演动机与冲突阻碍。回复必须富有启发性、直接切入戏剧冲突核心。`, p.Title, p.WorldRules)
	default: // editor_critique
		sysPrompt = fmt.Sprintf(`你是小说《%s》的严厉资深责任总编。
请站在严密因果逻辑、读者心流体验与节奏把控的角度，针对创作者的剧情设想进行犀利点评，指出潜在逻辑漏洞、战力崩塌或情节套路，并提出建设性的重构建议。`, p.Title)
	}

	var conversation strings.Builder
	for _, m := range req.Messages {
		conversation.WriteString(fmt.Sprintf("%s: %s\n\n", strings.ToUpper(m.Role), m.Content))
	}

	targetModel := req.Model
	if targetModel == "" {
		targetModel = s.cfg.ReasoningModel
	}

	resText, usage, err := s.llmRouter.ChatCompletionForRole(ctx, engine.RoleReasoner, targetModel, sysPrompt, conversation.String(), 0.7)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, WorkshopChatResponse{
		Message: WorkshopChatMessage{
			Role:    "assistant",
			Content: strings.TrimSpace(resText),
		},
		TokensUsed: usage,
	})
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

