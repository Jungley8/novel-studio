package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
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
	s.mux.HandleFunc("/api/config/test", s.handleConfigTest)
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "novel-studio",
		"time":    time.Now().Format(time.RFC3339),
	})
}
