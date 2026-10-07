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

	content, err := s.orch.RenderScene(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.Beats, req.WordsTarget)
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

	audit, err := s.qualityGate.Audit(r.Context(), s.cfg.ReasoningModel, p, req.ChapterIndex, req.Beats, req.DraftText)
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

	rewritten, err := s.orch.RewriteDraft(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.OriginalDraft, req.Review)
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

	fw, err := s.orch.BootstrapFramework(r.Context(), reasoningModel, engine.FrameworkBootstrapRequest{
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

		fw, bErr := s.orch.BootstrapFramework(ctx, model, engine.FrameworkBootstrapRequest{
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
