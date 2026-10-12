package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

// 跨作品租户隔离（Fail-Closed）测试组：
// 所有带作品路径的子资源操作都必须校验实体真实归属，禁止借道其它作品的 ID。
func newTenancyTestServer(t *testing.T) (*server.Server, store.Store) {
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
	linter := engine.NewLinter(nil)
	srv, err := server.New(cfg, filepath.Join(tmpDir, "cfg.json"), s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ctx := context.Background()
	for _, p := range []*domain.Project{{ID: "pA", Title: "作品A"}, {ID: "pB", Title: "作品B"}} {
		if err := s.SaveProject(ctx, p); err != nil {
			t.Fatalf("save project %s: %v", p.ID, err)
		}
	}
	return srv, s
}

func TestHooksUpdate_RejectsForeignProjectHook(t *testing.T) {
	srv, s := newTenancyTestServer(t)
	ctx := context.Background()

	hook := &domain.PlotHook{
		ID: "hook-a1", ProjectID: "pA", Title: "伏笔A",
		CreatedChapter: 1, TargetChapter: 3, Status: domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("save hook: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"id": "hook-a1", "title": "被串稿改名", "status": "OPEN"})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/pB/hooks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for foreign hook update, got %d: %s", w.Code, w.Body.String())
	}

	hooks, err := s.ListPlotHooks(ctx, "pA")
	if err != nil || len(hooks) != 1 || hooks[0].Title != "伏笔A" {
		t.Fatalf("foreign update must not mutate hook, got %+v err=%v", hooks, err)
	}

	// 本作品内更新成功
	localBody, _ := json.Marshal(map[string]any{"id": "hook-a1", "title": "伏笔A改", "status": "OPEN"})
	localReq := httptest.NewRequest(http.MethodPost, "/api/projects/pA/hooks", bytes.NewReader(localBody))
	localW := httptest.NewRecorder()
	srv.ServeHTTP(localW, localReq)
	if localW.Code != http.StatusCreated {
		t.Fatalf("expected 201 for own hook update, got %d: %s", localW.Code, localW.Body.String())
	}
}

func TestCodexProgression_RejectsForeignProjectEntry(t *testing.T) {
	srv, s := newTenancyTestServer(t)
	ctx := context.Background()

	entry := &domain.CodexEntry{
		ID: "codex-a1", ProjectID: "pA",
		Category: domain.CategoryCharacter, Name: "甲",
		TrackingMode: domain.TrackingModeAutoMention,
	}
	if err := s.SaveCodexEntry(ctx, entry); err != nil {
		t.Fatalf("save entry: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"active_from_chapter": 2, "summary": "入侵"})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/pB/codex/codex-a1/progressions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign entry progression, got %d: %s", w.Code, w.Body.String())
	}

	progs, err := s.ListCodexProgressions(ctx, "codex-a1")
	if err != nil {
		t.Fatalf("list progressions: %v", err)
	}
	if len(progs) != 0 {
		t.Fatalf("foreign progression must not be created, got %+v", progs)
	}
}

func TestCodexRelation_RejectsForeignProjectEntry(t *testing.T) {
	srv, s := newTenancyTestServer(t)
	ctx := context.Background()

	for _, id := range []string{"codex-a1", "codex-a2"} {
		if err := s.SaveCodexEntry(ctx, &domain.CodexEntry{
			ID: id, ProjectID: "pA",
			Category: domain.CategoryCharacter, Name: id,
			TrackingMode: domain.TrackingModeAutoMention,
		}); err != nil {
			t.Fatalf("save entry %s: %v", id, err)
		}
	}
	if err := s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID: "codex-b1", ProjectID: "pB",
		Category: domain.CategoryCharacter, Name: "乙",
		TrackingMode: domain.TrackingModeAutoMention,
	}); err != nil {
		t.Fatalf("save entry pB: %v", err)
	}

	// pB 的关系引用了 pA 的实体 → 400 拒绝
	body, _ := json.Marshal(map[string]any{
		"source_entry_id": "codex-b1",
		"target_entry_id": "codex-a1",
		"relation_type":   "ALLY",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/pB/codex/relations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cross-project relation, got %d: %s", w.Code, w.Body.String())
	}

	rels, err := s.ListCodexRelations(ctx, "pB", "")
	if err != nil {
		t.Fatalf("list relations: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("cross-project relation must not be created, got %+v", rels)
	}
}

func TestScenes_RejectForeignProjectOwnership(t *testing.T) {
	srv, s := newTenancyTestServer(t)
	ctx := context.Background()

	// 作品A 归档一章并创建一个场次
	chapterA := &domain.Chapter{ID: "ch-a1", ProjectID: "pA", ChapterIndex: 1, Title: "第一章"}
	if err := s.SaveChapter(ctx, chapterA); err != nil {
		t.Fatalf("save chapter: %v", err)
	}
	scene := &domain.Scene{ID: "sc-a1", ProjectID: "pA", ChapterID: "ch-a1", Title: "场次A", SceneIndex: 1}
	if err := s.SaveScene(ctx, scene); err != nil {
		t.Fatalf("save scene: %v", err)
	}

	// 1) pB 读 / 改 / 删 pA 的场次 → 404
	getReq := httptest.NewRequest(http.MethodGet, "/api/projects/pB/scenes/sc-a1", nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign scene GET, got %d", getW.Code)
	}

	putBody, _ := json.Marshal(map[string]any{"title": "被串稿", "chapter_id": "ch-a1"})
	putReq := httptest.NewRequest(http.MethodPut, "/api/projects/pB/scenes/sc-a1", bytes.NewReader(putBody))
	putW := httptest.NewRecorder()
	srv.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign scene PUT, got %d: %s", putW.Code, putW.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/projects/pB/scenes/sc-a1", nil)
	delW := httptest.NewRecorder()
	srv.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign scene DELETE, got %d", delW.Code)
	}

	got, err := s.GetScene(ctx, "sc-a1")
	if err != nil || got == nil || got.Title != "场次A" {
		t.Fatalf("foreign operations must not mutate scene, got %+v err=%v", got, err)
	}

	// 2) pB 创建场次挂载 pA 的章节 → 400
	createBody, _ := json.Marshal(map[string]any{"chapter_id": "ch-a1", "title": "越权场次", "scene_index": 1})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects/pB/scenes", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	srv.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for foreign chapter mount, got %d: %s", createW.Code, createW.Body.String())
	}

	// 3) pB 按 pA 的章节列举场次 → 404
	listReq := httptest.NewRequest(http.MethodGet, "/api/projects/pB/scenes?chapter_id=ch-a1", nil)
	listW := httptest.NewRecorder()
	srv.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for listing scenes of foreign chapter, got %d: %s", listW.Code, listW.Body.String())
	}

	// 4) 本作品按章节列举场次 → 200 且可见
	ownReq := httptest.NewRequest(http.MethodGet, "/api/projects/pA/scenes?chapter_id=ch-a1", nil)
	ownW := httptest.NewRecorder()
	srv.ServeHTTP(ownW, ownReq)
	if ownW.Code != http.StatusOK {
		t.Fatalf("expected 200 for own chapter listing, got %d: %s", ownW.Code, ownW.Body.String())
	}
	var listed []*domain.Scene
	if err := json.Unmarshal(ownW.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal scenes: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "sc-a1" {
		t.Fatalf("expected own scene listed, got %+v", listed)
	}
}

func TestCheckpointLatest_ReturnsMostRecentlyUpdated(t *testing.T) {
	srv, s := newTenancyTestServer(t)
	ctx := context.Background()

	if err := s.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
		ProjectID: "pA", ChapterIndex: 1, Phase: domain.CheckpointPhaseDrafted, DraftText: "第一稿",
	}); err != nil {
		t.Fatalf("save checkpoint 1: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := s.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
		ProjectID: "pA", ChapterIndex: 2, Phase: domain.CheckpointPhaseDrafted, DraftText: "第二稿",
	}); err != nil {
		t.Fatalf("save checkpoint 2: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/pA/checkpoint?latest=true", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cp domain.ChapterCheckpoint
	if err := json.Unmarshal(w.Body.Bytes(), &cp); err != nil {
		t.Fatalf("unmarshal checkpoint: %v", err)
	}
	if cp.ChapterIndex != 2 || cp.DraftText != "第二稿" {
		t.Fatalf("expected latest checkpoint chapter 2, got %+v", cp)
	}

	// 无断点的作品返回 null
	emptyReq := httptest.NewRequest(http.MethodGet, "/api/projects/pB/checkpoint?latest=true", nil)
	emptyW := httptest.NewRecorder()
	srv.ServeHTTP(emptyW, emptyReq)
	if emptyW.Code != http.StatusOK || string(bytes.TrimSpace(emptyW.Body.Bytes())) != "null" {
		t.Fatalf("expected 200 null for empty project, got %d: %s", emptyW.Code, emptyW.Body.String())
	}
}
