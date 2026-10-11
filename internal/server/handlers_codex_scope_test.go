package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

// 切换作品串稿防护（Fail-Closed）：
// 用其它作品的词条 ID 发起更新必须被 409 拒绝，且原词条数据不得被篡改。
func TestCodexUpdate_RejectsForeignProjectEntry(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewSQLiteStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	cfg := config.DefaultConfig()
	llmClient := engine.NewHTTPLLMClient("http://127.0.0.1:1", "sk-test-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)
	srv, err := server.New(cfg, filepath.Join(tmpDir, "cfg.json"), s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ctx := context.Background()
	if err := s.SaveProject(ctx, &domain.Project{ID: "pA", Title: "作品A"}); err != nil {
		t.Fatalf("save project pA: %v", err)
	}
	if err := s.SaveProject(ctx, &domain.Project{ID: "pB", Title: "作品B"}); err != nil {
		t.Fatalf("save project pB: %v", err)
	}
	entry := &domain.CodexEntry{
		ID:           "codex-a1",
		ProjectID:    "pA",
		Category:     domain.CategoryCharacter,
		Name:         "甲",
		TrackingMode: domain.TrackingModeAutoMention,
	}
	if err := s.SaveCodexEntry(ctx, entry); err != nil {
		t.Fatalf("save codex entry: %v", err)
	}

	// 从作品 B 的上下文发起对作品 A 词条的更新 → 必须 409 拒绝
	body, _ := json.Marshal(map[string]any{
		"id":       "codex-a1",
		"name":     "被串稿改名",
		"category": "CHARACTER",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/projects/pB/codex/codex-a1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for foreign project entry, got %d: %s", w.Code, w.Body.String())
	}

	// 原词条不得被篡改
	got, err := s.GetCodexEntry(ctx, "pA", "codex-a1")
	if err != nil {
		t.Fatalf("get codex entry: %v", err)
	}
	if got == nil || got.Name != "甲" {
		t.Fatalf("foreign update must not mutate entry, got %+v", got)
	}

	// 本作品内更新仍然成功
	localBody, _ := json.Marshal(map[string]any{
		"id":       "codex-a1",
		"name":     "甲改",
		"category": "CHARACTER",
	})
	localReq := httptest.NewRequest(http.MethodPut, "/api/projects/pA/codex/codex-a1", bytes.NewReader(localBody))
	localW := httptest.NewRecorder()
	srv.ServeHTTP(localW, localReq)
	if localW.Code != http.StatusOK {
		t.Fatalf("expected 200 for own project update, got %d: %s", localW.Code, localW.Body.String())
	}
	updated, err := s.GetCodexEntry(ctx, "pA", "codex-a1")
	if err != nil {
		t.Fatalf("get updated entry: %v", err)
	}
	if updated == nil || updated.Name != "甲改" {
		t.Fatalf("own project update should succeed, got %+v", updated)
	}
}
