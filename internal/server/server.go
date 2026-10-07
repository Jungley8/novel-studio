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
	cfg        *config.Config
	configPath string
	store      store.Store
	linter     *engine.Linter
	mux        *http.ServeMux
	fs         fs.FS
}

func New(cfg *config.Config, configPath string, s store.Store) (*Server, error) {
	webFS, err := web.FS()
	if err != nil {
		return nil, fmt.Errorf("load web embed FS failed: %w", err)
	}

	srv := &Server{
		cfg:        cfg,
		configPath: configPath,
		store:      s,
		linter:     engine.NewLinter(nil),
		mux:        http.NewServeMux(),
		fs:         webFS,
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
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/linter/analyze", s.handleLinter)
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

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, s.cfg)
	case http.MethodPost:
		var updated config.Config
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid config JSON: "+err.Error())
			return
		}
		s.cfg.APIBase = updated.APIBase
		s.cfg.APIKey = updated.APIKey
		s.cfg.ReasoningModel = updated.ReasoningModel
		s.cfg.WriterModel = updated.WriterModel
		if err := s.cfg.Save(s.configPath); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save config failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, s.cfg)
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
			c.ID = fmt.Sprintf("ch_%d_%d", c.ChapterIndex, time.Now().UnixNano())
		}
		if err := s.store.SaveChapter(ctx, &c); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// Automatically mutate protagonist state machine if deltas provided
		p, err := s.store.GetProject(ctx, projectID)
		if err == nil && (c.StateMutation.InventoryDelta != "" || c.StateMutation.PowerDelta != "") {
			if c.StateMutation.InventoryDelta != "" {
				p.Protagonist.Inventory = strings.TrimSpace(p.Protagonist.Inventory + ", " + c.StateMutation.InventoryDelta)
			}
			if c.StateMutation.PowerDelta != "" {
				p.Protagonist.NameAndLevel = strings.TrimSpace(p.Protagonist.NameAndLevel + " (" + c.StateMutation.PowerDelta + ")")
			}
			_ = s.store.SaveProject(ctx, p)
		}

		jsonResponse(w, http.StatusCreated, c)
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

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	hooks, _ := s.store.ListPlotHooks(r.Context(), projectID)

	client := engine.NewHTTPLLMClient(s.cfg.APIBase, s.cfg.APIKey)
	orch := engine.NewOrchestrator(client)

	out, err := orch.DeriveBeats(r.Context(), s.cfg.ReasoningModel, p, req.ChapterIndex, req.CoreConflict, hooks)
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

	client := engine.NewHTTPLLMClient(s.cfg.APIBase, s.cfg.APIKey)
	orch := engine.NewOrchestrator(client)

	content, err := orch.RenderScene(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.Beats, req.WordsTarget)
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
