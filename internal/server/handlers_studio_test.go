package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupTestServerWithMockLLM(t *testing.T) (*server.Server, store.Store, *httptest.Server) {
	return setupTestServerWithCapturingMockLLM(t, nil)
}

func setupTestServerWithCapturingMockLLM(t *testing.T, captureFn func(sysPrompt string)) (*server.Server, store.Store, *httptest.Server) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if captureFn != nil {
			for _, m := range reqBody.Messages {
				if m.Role == "system" {
					captureFn(m.Content)
				}
			}
		}

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

	// 2. Project 2: 5 chapters, 1 open hook (target chapter 8 - not overdue: 8 >= own max 5)
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

	// 3. Project 3: 1 chapter, 1 open hook targeting chapter 3.
	// Distinguishes per-project overdue from global-max overdue: under a
	// global max (5, set by project 2) this hook would count overdue (3 < 5),
	// but against its own progress (max 1) it must NOT.
	p3 := &domain.Project{
		ID:             "proj-lib-3",
		Title:          "慢热种田文",
		TargetPlatform: "通用网文",
	}
	_ = s.SaveProject(ctx, p3)
	_ = s.SaveChapter(ctx, &domain.Chapter{
		ID:           "ch-3-1",
		ProjectID:    p3.ID,
		ChapterIndex: 1,
		Title:        "开篇",
		Content:      "种田正文...",
		WordCount:    2000,
		LinterPassed: true,
	})
	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:             "hook-4",
		ProjectID:      p3.ID,
		Title:          "远期灌溉渠伏线",
		TargetChapter:  3,
		Status:         domain.HookStatusOpen,
		CreatedChapter: 1,
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

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
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
		t.Errorf("project 1 overdue_hooks = %d, want 1 (target chapter 2 < own max 3)", it1.OverdueHooks)
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
		t.Errorf("project 2 overdue_hooks = %d, want 0 (target chapter 8 >= own max 5)", it2.OverdueHooks)
	}

	it3, ok := itemMap[p3.ID]
	if !ok {
		t.Fatalf("project 3 not found in response")
	}
	if it3.ArchivedChapters != 1 {
		t.Errorf("project 3 archived_chapters = %d, want 1", it3.ArchivedChapters)
	}
	if it3.OpenHooks != 1 {
		t.Errorf("project 3 open_hooks = %d, want 1", it3.OpenHooks)
	}
	if it3.OverdueHooks != 0 {
		t.Errorf("project 3 overdue_hooks = %d, want 0 (target 3 >= own max 1; must not leak project 2's progress)", it3.OverdueHooks)
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

func TestStudioChat_GeneralBrainstorm_OverviewEmptyLibrary(t *testing.T) {
	var capturedSysPrompt string
	var mu sync.Mutex
	srv, _, mockServer := setupTestServerWithCapturingMockLLM(t, func(sysPrompt string) {
		mu.Lock()
		capturedSysPrompt = sysPrompt
		mu.Unlock()
	})
	defer mockServer.Close()

	payload := map[string]string{
		"mode":    "scene_brainstorm",
		"message": "空库全局脑暴测试",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(capturedSysPrompt, "全库作品概览") {
		t.Errorf("expected captured system prompt to contain '全库作品概览', got: %s", capturedSysPrompt)
	}
	if !strings.Contains(capturedSysPrompt, "全库暂无作品") {
		t.Errorf("expected captured system prompt to contain '全库暂无作品', got: %s", capturedSysPrompt)
	}
}

func TestStudioChat_GeneralBrainstorm_OverviewMultiProject(t *testing.T) {
	var capturedSysPrompt string
	var mu sync.Mutex
	srv, s, mockServer := setupTestServerWithCapturingMockLLM(t, func(sysPrompt string) {
		mu.Lock()
		capturedSysPrompt = sysPrompt
		mu.Unlock()
	})
	defer mockServer.Close()
	ctx := context.Background()

	// 1. 项目 1：带 ThemePremise，2 章，1 个 OPEN 钩，1 个 RESOLVED 钩
	p1 := &domain.Project{
		ID:             "proj-chat-p1",
		Title:          "万古仙途",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			ThemePremise: "凡人逆修弑神，天道实为寄生真魔",
		},
	}
	_ = s.SaveProject(ctx, p1)
	for i := 1; i <= 2; i++ {
		_ = s.SaveChapter(ctx, &domain.Chapter{
			ID:           fmt.Sprintf("ch-p1-%d", i),
			ProjectID:    p1.ID,
			ChapterIndex: i,
			Title:        fmt.Sprintf("第%d章", i),
			Content:      "正文...",
		})
	}
	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:        "hook-p1-1",
		ProjectID: p1.ID,
		Title:     "古印秘密",
		Status:    domain.HookStatusOpen,
	})
	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:        "hook-p1-2",
		ProjectID: p1.ID,
		Title:     "结仇长老",
		Status:    domain.HookStatusResolved,
	})

	// 2. 项目 2：带 ThemePremise，3 章，2 个 OPEN 钩
	p2 := &domain.Project{
		ID:             "proj-chat-p2",
		Title:          "星界求生",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			ThemePremise: "高维异种入侵，开局觉醒推演外挂",
		},
	}
	_ = s.SaveProject(ctx, p2)
	for i := 1; i <= 3; i++ {
		_ = s.SaveChapter(ctx, &domain.Chapter{
			ID:           fmt.Sprintf("ch-p2-%d", i),
			ProjectID:    p2.ID,
			ChapterIndex: i,
			Title:        fmt.Sprintf("第%d章", i),
			Content:      "正文...",
		})
	}
	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:        "hook-p2-1",
		ProjectID: p2.ID,
		Title:     "深渊母虫",
		Status:    domain.HookStatusOpen,
	})
	_ = s.SavePlotHook(ctx, &domain.PlotHook{
		ID:        "hook-p2-2",
		ProjectID: p2.ID,
		Title:     "叛军内奸",
		Status:    domain.HookStatusOpen,
	})

	// 3. 发送无 project_id 请求
	payload := map[string]string{
		"mode":    "scene_brainstorm",
		"message": "两个作品风格差异大，如何设计跨界联动点子？",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()

	if !strings.Contains(capturedSysPrompt, "全库作品概览") {
		t.Errorf("expected captured system prompt to contain '全库作品概览', got: %s", capturedSysPrompt)
	}
	if !strings.Contains(capturedSysPrompt, "万古仙途") {
		t.Errorf("expected captured system prompt to contain '万古仙途', got: %s", capturedSysPrompt)
	}
	if !strings.Contains(capturedSysPrompt, "星界求生") {
		t.Errorf("expected captured system prompt to contain '星界求生', got: %s", capturedSysPrompt)
	}
	if !strings.Contains(capturedSysPrompt, "凡人逆修弑神") {
		t.Errorf("expected captured system prompt to contain p1 ThemePremise, got: %s", capturedSysPrompt)
	}
	if !strings.Contains(capturedSysPrompt, "高维异种入侵") {
		t.Errorf("expected captured system prompt to contain p2 ThemePremise, got: %s", capturedSysPrompt)
	}
	if strings.Contains(capturedSysPrompt, "全库暂无作品") {
		t.Errorf("captured system prompt should not contain '全库暂无作品' when projects exist")
	}
}

func TestStudioLibrary_TokenAndCostAggregation(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	// 1. 构造 2 个项目
	p1 := &domain.Project{
		ID:             "proj-cost-1",
		Title:          "修仙成本录",
		TargetPlatform: "起点仙侠",
	}
	if err := s.SaveProject(ctx, p1); err != nil {
		t.Fatalf("save project 1 failed: %v", err)
	}

	p2 := &domain.Project{
		ID:             "proj-cost-2",
		Title:          "零消耗之书",
		TargetPlatform: "番茄脑洞",
	}
	if err := s.SaveProject(ctx, p2); err != nil {
		t.Fatalf("save project 2 failed: %v", err)
	}

	// 2. 构造若干 activity 事件（含 token_usage JSON）
	// 事件 1: 标准完整 TokenUsage JSON
	tu1 := engine.TokenUsage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	tu1JSON, _ := json.Marshal(tu1)
	if err := s.AppendEvent(ctx, &activity.ActivityEvent{
		ProjectID:  p1.ID,
		EntityType: activity.EntityWorkshop,
		EntityID:   "run-cost-1",
		EventType:  activity.EventWorkshopCompleted,
		Actor:      activity.ActorSystem,
		Source:     "engine",
		Title:      "第 1 章生产完成",
		TokenUsage: string(tu1JSON),
	}); err != nil {
		t.Fatalf("append event 1 failed: %v", err)
	}

	// 事件 2: 另一条标准完整 TokenUsage JSON
	tu2 := engine.TokenUsage{PromptTokens: 2500, CompletionTokens: 1200, TotalTokens: 3700}
	tu2JSON, _ := json.Marshal(tu2)
	if err := s.AppendEvent(ctx, &activity.ActivityEvent{
		ProjectID:  p1.ID,
		EntityType: activity.EntityWorkshop,
		EntityID:   "run-cost-2",
		EventType:  activity.EventWorkshopCompleted,
		Actor:      activity.ActorSystem,
		Source:     "engine",
		Title:      "第 2 章生产完成",
		TokenUsage: string(tu2JSON),
	}); err != nil {
		t.Fatalf("append event 2 failed: %v", err)
	}

	// 事件 3: total 缺失回退测试 (仅 prompt_tokens 与 completion_tokens，total_tokens 缺失)
	tu3JSON := `{"prompt_tokens":800,"completion_tokens":400}`
	if err := s.AppendEvent(ctx, &activity.ActivityEvent{
		ProjectID:  p1.ID,
		EntityType: activity.EntityWorkshop,
		EntityID:   "run-cost-3",
		EventType:  activity.EventWorkshopCompleted,
		Actor:      activity.ActorSystem,
		Source:     "engine",
		Title:      "第 3 章生产完成",
		TokenUsage: tu3JSON,
	}); err != nil {
		t.Fatalf("append event 3 failed: %v", err)
	}

	// 项目 2 无 token 事件（记录一条无 token_usage 的活动事件）
	if err := s.AppendEvent(ctx, &activity.ActivityEvent{
		ProjectID:  p2.ID,
		EntityType: activity.EntityWorkshop,
		EntityID:   "run-cost-p2-empty",
		EventType:  activity.EventWorkshopCompleted,
		Actor:      activity.ActorSystem,
		Source:     "engine",
		Title:      "草稿初创（无 token 消耗）",
	}); err != nil {
		t.Fatalf("append event for p2 failed: %v", err)
	}

	// 3. 请求 GET /api/studio/library
	req := httptest.NewRequest(http.MethodGet, "/api/studio/library", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var items []server.StudioLibraryItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal library items error: %v", err)
	}

	itemMap := make(map[string]server.StudioLibraryItem)
	for _, it := range items {
		itemMap[it.ProjectID] = it
	}

	// 4. 断言项目 1 (prompt/completion/total 聚合正确、estimated_cost_usd == EstimateCostUSD 精确相等)
	it1, ok := itemMap[p1.ID]
	if !ok {
		t.Fatalf("project 1 (%s) not found in response", p1.ID)
	}

	expectedPrompt := 1000 + 2500 + 800        // 4300
	expectedComp := 500 + 1200 + 400           // 2100
	expectedTotal := 1500 + 3700 + (800 + 400) // 6400
	expectedCost1 := engine.EstimateCostUSD(engine.TokenUsage{
		PromptTokens:     expectedPrompt,
		CompletionTokens: expectedComp,
		TotalTokens:      expectedTotal,
	})

	if it1.PromptTokens != expectedPrompt {
		t.Errorf("p1 prompt_tokens = %d, want %d", it1.PromptTokens, expectedPrompt)
	}
	if it1.CompletionTokens != expectedComp {
		t.Errorf("p1 completion_tokens = %d, want %d", it1.CompletionTokens, expectedComp)
	}
	if it1.TokenUsage != expectedTotal {
		t.Errorf("p1 token_usage (total) = %d, want %d", it1.TokenUsage, expectedTotal)
	}
	if it1.EstimatedCostUSD != expectedCost1 {
		t.Errorf("p1 estimated_cost_usd = %v, want %v", it1.EstimatedCostUSD, expectedCost1)
	}

	// 5. 断言项目 2 (无 token 事件的项目费用为 0)
	it2, ok := itemMap[p2.ID]
	if !ok {
		t.Fatalf("project 2 (%s) not found in response", p2.ID)
	}

	if it2.PromptTokens != 0 {
		t.Errorf("p2 prompt_tokens = %d, want 0", it2.PromptTokens)
	}
	if it2.CompletionTokens != 0 {
		t.Errorf("p2 completion_tokens = %d, want 0", it2.CompletionTokens)
	}
	if it2.TokenUsage != 0 {
		t.Errorf("p2 token_usage = %d, want 0", it2.TokenUsage)
	}
	if it2.EstimatedCostUSD != 0.0 {
		t.Errorf("p2 estimated_cost_usd = %v, want 0.0", it2.EstimatedCostUSD)
	}
}
