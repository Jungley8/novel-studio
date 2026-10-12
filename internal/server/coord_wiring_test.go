package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

// 并发互斥（前台编辑租约 + 后台生产锁）的接口层接线测试（白盒：直接操控 srv.coord）。

func newCoordTestServer(t *testing.T) (*Server, store.Store) {
	t.Helper()
	tmpDir := t.TempDir()
	s, err := store.NewSQLiteStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cfg := config.DefaultConfig()
	llmClient := engine.NewHTTPLLMClient("http://127.0.0.1:1", "sk-test-key")
	orch := engine.NewOrchestrator(llmClient)
	srv, err := New(cfg, filepath.Join(tmpDir, "cfg.json"), s, llmClient, orch, engine.NewLinter(nil))
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	if err := s.SaveProject(context.Background(), &domain.Project{ID: "p1", Title: "协调器测试"}); err != nil {
		t.Fatalf("save project: %v", err)
	}
	return srv, s
}

func TestEditLeaseEndpoint_Roundtrip(t *testing.T) {
	srv, _ := newCoordTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/edit-lease", strings.NewReader(`{"ttl_seconds": 60}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("acquire expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Status    string `json:"status"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal lease response: %v", err)
	}
	if resp.Status != "held" || resp.ExpiresAt == "" {
		t.Fatalf("unexpected lease response: %+v", resp)
	}
	if !srv.coord.EditActive("p1") {
		t.Fatal("edit lease should be active after acquire")
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/projects/p1/edit-lease", nil)
	dw := httptest.NewRecorder()
	srv.ServeHTTP(dw, del)
	if dw.Code != http.StatusOK {
		t.Fatalf("release expected 200, got %d: %s", dw.Code, dw.Body.String())
	}
	if srv.coord.EditActive("p1") {
		t.Fatal("edit lease should be released")
	}
}

func TestCanonWrites_RejectedWhileProductionActive(t *testing.T) {
	srv, s := newCoordTestServer(t)
	ctx := context.Background()

	release, err := srv.coord.BeginProduction("p1", "排程自动续产")
	if err != nil {
		t.Fatalf("begin production: %v", err)
	}
	srv.coord.SetProductionChapter("p1", 3)

	cases := []struct {
		name, method, path, body string
	}{
		{"commit chapter", http.MethodPost, "/api/projects/p1/chapters", `{"chapter_index":3,"title":"t","content":"c"}`},
		{"uncommit chapter", http.MethodPost, "/api/projects/p1/chapters/3/uncommit", ``},
		{"delete chapter", http.MethodDelete, "/api/projects/p1/chapters/3", ``},
		{"workshop produce", http.MethodPost, "/api/projects/p1/workshop/produce", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusConflict {
				t.Fatalf("expected 409 while production active, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "排程自动续产") {
				t.Fatalf("409 message should carry production label, got: %s", w.Body.String())
			}
		})
	}

	chapters, err := s.ListChapters(ctx, "p1")
	if err != nil {
		t.Fatalf("list chapters: %v", err)
	}
	if len(chapters) != 0 {
		t.Fatalf("canon must be untouched during production, got %d chapters", len(chapters))
	}

	// 生产结束后正史写入口恢复
	release()
	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/chapters", strings.NewReader(`{"chapter_index":3,"title":"t","content":"正文内容"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 after production released, got %d: %s", w.Code, w.Body.String())
	}
}
