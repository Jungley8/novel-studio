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

func setupTestServerWithMockLLM(t *testing.T) (*server.Server, store.Store, *httptest.Server) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "这是工坊智囊提供的头脑风暴建议：引入宿敌卧底的反转线索。",
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     50,
				"completion_tokens": 25,
				"total_tokens":      75,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

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
	return srv, s, mockServer
}

func TestStudioLibrary_Empty(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/studio/library", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var items []server.StudioLibraryItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected empty library slice, got %d items", len(items))
	}
}

func TestStudioLibrary_MultiProject(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	// 1. Project 1: 3 chapters, 1 open hook (target chapter 2 - overdue when global max is 5), 1 resolved hook
	p1 := &domain.Project{
		ID:             "proj-lib-1",
		Title:          "万古仙途",
		TargetPlatform: "起点仙侠",
	}
	_ = s.SaveProject(ctx, p1)

	for i := 1; i <= 3; i++ {
		ch := &domain.Chapter{
			ID:           "ch-1-" + string(rune('0'+i)),
			ProjectID:    p1.ID,
			ChapterIndex: i,
			Title:        "第一卷 第" + string(rune('0'+i)) + "章",
			Content:      "正文内容...",
			WordCount:    3000,
			LinterPassed: true,
		}
		_ = s.SaveChapter(ctx, ch)
	}

	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:             "hook-1",
		ProjectID:      p1.ID,
		Title:          "神秘青铜残片",
		TargetChapter:  2,
		Status:         domain.HookStatusOpen,
		CreatedChapter: 1,
	})
	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:             "hook-2",
		ProjectID:      p1.ID,
		Title:          "身世玉佩解密",
		TargetChapter:  2,
		Status:         domain.HookStatusResolved,
		CreatedChapter: 1,
	})

	// 2. Project 2: 5 chapters (sets global max archived chapter to 5), 1 open hook (target chapter 8 - not overdue)
	p2 := &domain.Project{
		ID:             "proj-lib-2",
		Title:          "星界求生",
		TargetPlatform: "番茄脑洞",
	}
	_ = s.SaveProject(ctx, p2)

	for i := 1; i <= 5; i++ {
		ch := &domain.Chapter{
			ID:           "ch-2-" + string(rune('0'+i)),
			ProjectID:    p2.ID,
			ChapterIndex: i,
			Title:        "星际降临 第" + string(rune('0'+i)) + "章",
			Content:      "星战正文...",
			WordCount:    2500,
			LinterPassed: true,
		}
		_ = s.SaveChapter(ctx, ch)
	}

	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:             "hook-3",
		ProjectID:      p2.ID,
		Title:          "高维异种伏线",
		TargetChapter:  8,
		Status:         domain.HookStatusOpen,
		CreatedChapter: 3,
	})

	// 3. Request GET /api/studio/library
	req := httptest.NewRequest(http.MethodGet, "/api/studio/library", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var items []server.StudioLibraryItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	itemMap := make(map[string]server.StudioLibraryItem)
	for _, it := range items {
		itemMap[it.ProjectID] = it
	}

	it1, ok := itemMap[p1.ID]
	if !ok {
		t.Fatalf("project 1 not found in response")
	}
	if it1.ArchivedChapters != 3 {
		t.Errorf("project 1 archived_chapters = %d, want 3", it1.ArchivedChapters)
	}
	if it1.OpenHooks != 1 {
		t.Errorf("project 1 open_hooks = %d, want 1", it1.OpenHooks)
	}
	if it1.OverdueHooks != 1 {
		t.Errorf("project 1 overdue_hooks = %d, want 1 (target chapter 2 < global max 5)", it1.OverdueHooks)
	}

	it2, ok := itemMap[p2.ID]
	if !ok {
		t.Fatalf("project 2 not found in response")
	}
	if it2.ArchivedChapters != 5 {
		t.Errorf("project 2 archived_chapters = %d, want 5", it2.ArchivedChapters)
	}
	if it2.OpenHooks != 1 {
		t.Errorf("project 2 open_hooks = %d, want 1", it2.OpenHooks)
	}
	if it2.OverdueHooks != 0 {
		t.Errorf("project 2 overdue_hooks = %d, want 0 (target chapter 8 >= global max 5)", it2.OverdueHooks)
	}
}

func TestStudioChat_MissingProject404(t *testing.T) {
	srv, _, mockServer := setupTestServerWithMockLLM(t)
	defer mockServer.Close()

	payload := map[string]string{
		"project_id": "non_existent_project_id",
		"mode":       "scene_brainstorm",
		"message":    "帮我想一个大反转",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for missing project, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStudioChat_GeneralBrainstorm200(t *testing.T) {
	srv, _, mockServer := setupTestServerWithMockLLM(t)
	defer mockServer.Close()

	// project_id 为空即通用脑暴
	payload := map[string]string{
		"mode":    "scene_brainstorm",
		"message": "给我一个赛博朋克修仙的世界观核心冲突点子",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for general brainstorm, got %d: %s", w.Code, w.Body.String())
	}

	var resp server.StudioChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Reply == "" {
		t.Errorf("expected non-empty reply in studio chat")
	}
	if resp.Mode != "scene_brainstorm" {
		t.Errorf("expected mode scene_brainstorm, got %s", resp.Mode)
	}
}

func TestStudioChat_WithProject200(t *testing.T) {
	srv, s, mockServer := setupTestServerWithMockLLM(t)
	defer mockServer.Close()
	ctx := context.Background()

	p := &domain.Project{
		ID:             "proj-chat-valid",
		Title:          "大周仙吏",
		TargetPlatform: "起点仙侠",
		WorldRules:     "妖魔乱世，衙门捉妖",
	}
	_ = s.SaveProject(ctx, p)

	payload := map[string]string{
		"project_id": p.ID,
		"mode":       "character_roleplay",
		"message":    "掌门此刻心情如何？",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for project chat, got %d: %s", w.Code, w.Body.String())
	}

	var resp server.StudioChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Reply == "" {
		t.Errorf("expected non-empty reply in studio chat")
	}
	if resp.ProjectID != p.ID {
		t.Errorf("expected project_id %s, got %s", p.ID, resp.ProjectID)
	}
}

func TestStudio_MethodNotAllowedAndValidation(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// POST /api/studio/library -> 405
	reqLibPost := httptest.NewRequest(http.MethodPost, "/api/studio/library", nil)
	wLibPost := httptest.NewRecorder()
	srv.ServeHTTP(wLibPost, reqLibPost)
	if wLibPost.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST /api/studio/library, got %d", wLibPost.Code)
	}

	// GET /api/studio/chat -> 405
	reqChatGet := httptest.NewRequest(http.MethodGet, "/api/studio/chat", nil)
	wChatGet := httptest.NewRecorder()
	srv.ServeHTTP(wChatGet, reqChatGet)
	if wChatGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET /api/studio/chat, got %d", wChatGet.Code)
	}

	// POST /api/studio/chat with empty message -> 400
	emptyMsgBody, _ := json.Marshal(map[string]string{"message": ""})
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(emptyMsgBody))
	wEmpty := httptest.NewRecorder()
	srv.ServeHTTP(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty message, got %d", wEmpty.Code)
	}
}
