package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupStudioTodosTestServer(t *testing.T) (*server.Server, store.Store) {
	t.Helper()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(func() {
		mockServer.Close()
	})

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "todos_test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	cfg := config.DefaultConfig()
	cfg.APIBase = mockServer.URL
	cfg.APIKey = "sk-mock-key"
	llmClient := engine.NewHTTPLLMClient(mockServer.URL, "sk-mock-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	return srv, s
}

func TestStudioTodos_Empty(t *testing.T) {
	srv, _ := setupStudioTodosTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/studio/todos", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	bodyStr := strings.TrimSpace(w.Body.String())
	if !strings.Contains(bodyStr, `"todos":[]`) {
		t.Fatalf("expected JSON to contain '\"todos\":[]', got: %s", bodyStr)
	}

	var resp server.StudioTodosResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(resp.Todos))
	}
}

func TestStudioTodos_WithNeedsHumanCheckpoint(t *testing.T) {
	srv, s := setupStudioTodosTestServer(t)
	ctx := context.Background()

	p := &domain.Project{
		ID:             "p-todo-1",
		Title:          "修真大道",
		TargetPlatform: "起点仙侠",
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	cp := &domain.ChapterCheckpoint{
		ProjectID:    "p-todo-1",
		ChapterIndex: 1,
		Phase:        domain.CheckpointPhaseAudited,
		CoreConflict: "宗门争夺灵脉",
		DraftText:    "天雷阵阵，灵脉震荡。",
		AuditReport: &domain.AuditReport{
			Verdict:    domain.ReviewVerdictRevision,
			Score:      72,
			NeedsHuman: true,
			Issues:     []string{"【ESCALATED】已达最大重写轮次仍未通过，已停线待人工，拒绝自动入库"},
		},
		RewriteLoops: 2,
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/studio/todos", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp server.StudioTodosResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if len(resp.Todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(resp.Todos))
	}

	todo := resp.Todos[0]
	if todo.ProjectID != "p-todo-1" {
		t.Errorf("expected project_id 'p-todo-1', got %q", todo.ProjectID)
	}
	if todo.ProjectTitle != "修真大道" {
		t.Errorf("expected project_title '修真大道', got %q", todo.ProjectTitle)
	}
	if todo.ChapterIndex != 1 {
		t.Errorf("expected chapter_index 1, got %d", todo.ChapterIndex)
	}
	if todo.Verdict != string(domain.ReviewVerdictRevision) {
		t.Errorf("expected verdict REVISION, got %q", todo.Verdict)
	}
	if todo.Score != 72 {
		t.Errorf("expected score 72, got %d", todo.Score)
	}
	if len(todo.Issues) != 1 || !strings.Contains(todo.Issues[0], "【ESCALATED】") {
		t.Errorf("unexpected issues: %v", todo.Issues)
	}
	if todo.UpdatedAt == "" {
		t.Errorf("expected non-empty updated_at")
	} else if _, err := time.Parse(time.RFC3339, todo.UpdatedAt); err != nil {
		t.Errorf("expected RFC3339 format for updated_at (%q), parse err: %v", todo.UpdatedAt, err)
	}
}

func TestStudioTodos_FilterArchivedChapters(t *testing.T) {
	srv, s := setupStudioTodosTestServer(t)
	ctx := context.Background()

	p := &domain.Project{
		ID:             "p-arch-1",
		Title:          "归档测试小说",
		TargetPlatform: "番茄脑洞",
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. CommitChapter 正常流：入库章并清除 checkpoint
	ch1 := &domain.Chapter{
		ID:           "ch_p-arch-1_1",
		ProjectID:    "p-arch-1",
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "第一章正文",
		WordCount:    100,
	}
	if _, err := s.CommitChapter(activity.WithActor(ctx, activity.ActorSystem), "p-arch-1", ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// 2. 模拟陈旧断点残留：章节已在 chapters 表归档，但 chapter_checkpoints 遗留了 NeedsHuman 断点
	staleCP := &domain.ChapterCheckpoint{
		ProjectID:    "p-arch-1",
		ChapterIndex: 1,
		Phase:        domain.CheckpointPhaseAudited,
		DraftText:    "残余手稿",
		AuditReport: &domain.AuditReport{
			Verdict:    domain.ReviewVerdictRevision,
			Score:      60,
			NeedsHuman: true,
			Issues:     []string{"残余问题"},
		},
	}
	if err := s.SaveCheckpoint(ctx, staleCP); err != nil {
		t.Fatalf("SaveCheckpoint stale failed: %v", err)
	}

	// 同时为未归档的第 2 章保存一个正常待办断点
	activeCP := &domain.ChapterCheckpoint{
		ProjectID:    "p-arch-1",
		ChapterIndex: 2,
		Phase:        domain.CheckpointPhaseAudited,
		DraftText:    "第2章手稿",
		AuditReport: &domain.AuditReport{
			Verdict:    domain.ReviewVerdictRevision,
			Score:      65,
			NeedsHuman: true,
			Issues:     []string{"第2章停线待办"},
		},
	}
	if err := s.SaveCheckpoint(ctx, activeCP); err != nil {
		t.Fatalf("SaveCheckpoint active failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/studio/todos", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp server.StudioTodosResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	// 应仅返回第 2 章，过滤掉已归档的第 1 章
	if len(resp.Todos) != 1 {
		t.Fatalf("expected exactly 1 todo, got %d", len(resp.Todos))
	}
	if resp.Todos[0].ChapterIndex != 2 {
		t.Errorf("expected chapter_index 2, got %d", resp.Todos[0].ChapterIndex)
	}
}

func TestStudioTodos_MethodNotAllowed(t *testing.T) {
	srv, _ := setupStudioTodosTestServer(t)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, m := range methods {
		req := httptest.NewRequest(m, "/api/studio/todos", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: expected status 405, got %d", m, w.Code)
		}
	}
}
