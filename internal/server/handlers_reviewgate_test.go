package server_test

import (
	"bytes"
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
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupReviewGateTestServer(t *testing.T) (*server.Server, store.Store, *httptest.Server) {
	t.Helper()

	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		var systemPrompt string
		for _, m := range req.Messages {
			if m.Role == "system" {
				systemPrompt = m.Content
			}
		}

		content := `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"通过"}`
		if strings.Contains(systemPrompt, "逻辑分庭总审编") {
			content = `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"","resolved_hooks":[{"id":"hook-valid","quote":"夜色渐深，冷风呼啸"},{"id":"hook-ghost","quote":"夜色渐深，冷风呼啸"}]}`
		} else if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
			content = `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"通过"}`
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": content,
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     80,
				"completion_tokens": 160,
				"total_tokens":      240,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "reviewgate_test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.APIBase = mockLLM.URL
	cfg.APIKey = "sk-mock-key"

	llmClient := engine.NewHTTPLLMClient(mockLLM.URL, "sk-mock-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	return srv, s, mockLLM
}

func TestHandleReviewDraft_HookEvidenceGate(t *testing.T) {
	srv, s, mockLLM := setupReviewGateTestServer(t)
	defer mockLLM.Close()

	ctx := context.Background()

	// 1. 造数：SaveProject (p1) + SavePlotHook (hook-valid)
	proj := &domain.Project{
		ID:    "p1",
		Title: "测试项目",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	validHook := &domain.PlotHook{
		ID:             "hook-valid",
		ProjectID:      "p1",
		Title:          "有效伏笔",
		Details:        "冷风呼啸下的决战伏笔",
		CreatedChapter: 1,
		TargetChapter:  5,
		Status:         domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, validHook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// 2. POST /api/projects/p1/review-draft
	reqBody := `{"chapter_index":1,"draft_text":"夜色渐深，冷风呼啸。林凡拔出青玉长剑，一剑断去黑衣人退路。","beats":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/review-draft", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Verdict         string   `json:"verdict"`
		Score           int      `json:"score"`
		Issues          []string `json:"issues"`
		Suggestions     string   `json:"suggestions"`
		ResolvedHookIDs []string `json:"resolved_hook_ids"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	// 断言 1: 响应 resolved_hook_ids == ["hook-valid"]（不含 hook-ghost）
	if len(resp.ResolvedHookIDs) != 1 || resp.ResolvedHookIDs[0] != "hook-valid" {
		t.Errorf("expected resolved_hook_ids == [hook-valid], got %v", resp.ResolvedHookIDs)
	}

	// 断言 2: 响应 issues 含 "伏笔兑现证据不足"
	foundRejectionIssue := false
	for _, issue := range resp.Issues {
		if strings.Contains(issue, "伏笔兑现证据不足") {
			foundRejectionIssue = true
			break
		}
	}
	if !foundRejectionIssue {
		t.Errorf("expected issues to contain '伏笔兑现证据不足', got %v", resp.Issues)
	}

	// 断言 3: checkpoint 的 AuditReport.ResolvedHookIDs == ["hook-valid"]
	cp, err := s.GetCheckpoint(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("GetCheckpoint failed: %v", err)
	}
	if cp == nil || cp.AuditReport == nil {
		t.Fatalf("expected checkpoint with AuditReport, got %v", cp)
	}
	if len(cp.AuditReport.ResolvedHookIDs) != 1 || cp.AuditReport.ResolvedHookIDs[0] != "hook-valid" {
		t.Errorf("expected checkpoint ResolvedHookIDs == [hook-valid], got %v", cp.AuditReport.ResolvedHookIDs)
	}

	// 3. 负例：把 hook-valid 覆盖为 RESOLVED 再重发 → resolved_hook_ids 为空
	validHook.Status = domain.HookStatusResolved
	if err := s.SavePlotHook(ctx, validHook); err != nil {
		t.Fatalf("SavePlotHook update to RESOLVED failed: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/projects/p1/review-draft", bytes.NewBufferString(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for second request, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 struct {
		Verdict         string   `json:"verdict"`
		Score           int      `json:"score"`
		Issues          []string `json:"issues"`
		Suggestions     string   `json:"suggestions"`
		ResolvedHookIDs []string `json:"resolved_hook_ids"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("failed to decode response2 JSON: %v", err)
	}

	if len(resp2.ResolvedHookIDs) != 0 {
		t.Errorf("expected empty resolved_hook_ids when hook is resolved, got %v", resp2.ResolvedHookIDs)
	}

	cp2, err := s.GetCheckpoint(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("GetCheckpoint failed after second request: %v", err)
	}
	if cp2 == nil || cp2.AuditReport == nil {
		t.Fatalf("expected checkpoint with AuditReport after second request, got %v", cp2)
	}
	if len(cp2.AuditReport.ResolvedHookIDs) != 0 {
		t.Errorf("expected empty checkpoint ResolvedHookIDs when hook is resolved, got %v", cp2.AuditReport.ResolvedHookIDs)
	}
}
