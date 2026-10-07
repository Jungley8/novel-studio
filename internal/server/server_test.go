package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupTestServer(t *testing.T) (*server.Server, store.Store, *config.Config) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	cfg := config.DefaultConfig()
	srv, err := server.New(cfg, cfgPath, s)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	return srv, s, cfg
}

func TestServer_ConfigAndProjects(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// 1. GET /api/config
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// 2. POST /api/projects
	projPayload := domain.Project{
		Title:          "万古神帝",
		TargetPlatform: "起点仙侠",
	}
	body, _ := json.Marshal(projPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created domain.Project
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Title != "万古神帝" {
		t.Errorf("expected Title 万古神帝, got %s", created.Title)
	}

	// 3. GET /api/projects
	req = httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []*domain.Project
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("expected 1 project, got %d", len(list))
	}

	// 4. POST /api/linter/analyze
	lintPayload := map[string]string{"text": "他不由得冷笑了一声。"}
	body, _ = json.Marshal(lintPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/linter/analyze", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var linterRes domain.LinterResult
	_ = json.Unmarshal(w.Body.Bytes(), &linterRes)
	if len(linterRes.HitBannedWords) == 0 {
		t.Errorf("expected hit banned words for cliché")
	}

	// 5. GET / (static index.html)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for index.html, got %d", w.Code)
	}
}
