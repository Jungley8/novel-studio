package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestBench_NotFound(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/non-existent-proj/bench", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("expected 4xx status for non-existent project, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBench_MethodNotAllowed(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:    "proj-bench-method",
		Title: "测试方法限制",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/projects/proj-bench-method/bench", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: expected 405, got %d: %s", method, w.Code, w.Body.String())
		}
	}
}

func TestBench_EmptyProject(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:                 "proj-bench-empty",
		Title:              "空章体检测试",
		DefaultWordsTarget: 3000,
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/proj-bench-empty/bench", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report engine.BenchReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal BenchReport failed: %v, raw: %s", err, w.Body.String())
	}

	if report.ProjectID != "proj-bench-empty" {
		t.Errorf("expected project_id 'proj-bench-empty', got '%s'", report.ProjectID)
	}
	if report.ProjectTitle != "空章体检测试" {
		t.Errorf("expected project_title '空章体检测试', got '%s'", report.ProjectTitle)
	}
	if report.Chapters != 0 {
		t.Errorf("expected 0 chapters, got %d", report.Chapters)
	}
	if report.WordCount.Target != 3000 {
		t.Errorf("expected target 3000, got %d", report.WordCount.Target)
	}
	if report.WordCount.Avg != 0 || report.WordCount.Min != 0 || report.WordCount.Max != 0 || report.WordCount.UnderTarget != 0 {
		t.Errorf("expected zero word_count stats for 0 chapters, got %+v", report.WordCount)
	}
	if report.Hooks.Open != 0 || len(report.Hooks.Overdue) != 0 {
		t.Errorf("expected 0 hooks for empty project, got %+v", report.Hooks)
	}
	if !report.LedgerReplay.Clean {
		t.Errorf("expected clean ledger replay for empty project, got dirty: %+v", report.LedgerReplay)
	}
	if report.Echo.TotalPhrases != 0 || report.Echo.ChaptersHit != 0 {
		t.Errorf("expected zero echo for empty project, got %+v", report.Echo)
	}
}

func TestBench_WithChaptersAndHooks(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:                 "proj-bench-full",
		Title:              "实装章节体检",
		DefaultWordsTarget: 2000,
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 存两章
	ch1 := &domain.Chapter{
		ID:           "ch-1",
		ProjectID:    "proj-bench-full",
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "风雪呼啸，少年踏上青云路。剑气纵横三万里，一剑光寒十九洲。",
		WordCount:    30,
		Beats: []domain.SceneBeat{
			{Phase: "开篇", Tension: 5, Action: "出门"},
		},
	}
	if err := s.SaveChapter(ctx, ch1); err != nil {
		t.Fatalf("SaveChapter 1 failed: %v", err)
	}

	ch2 := &domain.Chapter{
		ID:           "ch-2",
		ProjectID:    "proj-bench-full",
		ChapterIndex: 2,
		Title:        "第二章",
		Content:      "风雪渐歇，少年在客栈中静坐调息。忽听门外一阵喧哗之声。",
		WordCount:    26,
		Beats: []domain.SceneBeat{
			{Phase: "发展", Tension: 6, Action: "入客栈"},
		},
	}
	if err := s.SaveChapter(ctx, ch2); err != nil {
		t.Fatalf("SaveChapter 2 failed: %v", err)
	}

	// 存一个逾期伏笔 (TargetChapter 1 < 最大已归档章号 2)
	hookOverdue := &domain.PlotHook{
		ID:            "hook-1",
		ProjectID:     "proj-bench-full",
		Title:         "神秘黑衣人身份",
		TargetChapter: 1,
		Status:        domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hookOverdue); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/proj-bench-full/bench", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report engine.BenchReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if report.Chapters != 2 {
		t.Errorf("expected 2 chapters, got %d", report.Chapters)
	}
	if report.Hooks.Open != 1 {
		t.Errorf("expected 1 open hook, got %d", report.Hooks.Open)
	}
	if len(report.Hooks.Overdue) != 1 {
		t.Fatalf("expected 1 overdue hook, got %d", len(report.Hooks.Overdue))
	}
	if report.Hooks.Overdue[0].ID != "hook-1" {
		t.Errorf("expected overdue hook id 'hook-1', got '%s'", report.Hooks.Overdue[0].ID)
	}
}
