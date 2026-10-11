package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// GetActivePlan must bubble LoadForecast failures (Fail-Closed) instead of
// silently continuing with a nil forecast.
func TestSQLiteStore_GetActivePlan_LoadForecastErrorBubbles(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_planfix_forecast_err.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_planfix_fc"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "计划修复验证"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	if _, err := s.CommitChapter(ctx, projID, &domain.Chapter{
		ID:           "ch_planfix_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "正史开篇内容",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	fp, err := s.ComputeFingerprint(ctx, projID)
	if err != nil {
		t.Fatalf("ComputeFingerprint failed: %v", err)
	}
	fc := &domain.Forecast{
		ID:          "fc_planfix_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧验证",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: fp,
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}
	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	// Sanity: active plan reads fine before sabotage.
	if _, err := s.GetActivePlan(ctx, projID); err != nil {
		t.Fatalf("GetActivePlan before sabotage failed: %v", err)
	}

	// Sabotage: rename a column LoadForecast SELECTs, so its first touchpoint
	// fails with "no such column" while the project_active_plan row stays
	// intact. (DROP TABLE is not equivalent here: the FK
	// project_active_plan(forecast_id) -> narrative_forecasts(id) is
	// ON DELETE CASCADE, and SQLite's DROP TABLE fires implicit deletes,
	// which would cascade-wipe the plan row and turn the probe into (nil,nil).)
	if _, err := s.db.Exec(`ALTER TABLE narrative_forecasts RENAME COLUMN fingerprint TO fingerprint_broken`); err != nil {
		t.Fatalf("RENAME COLUMN sabotage failed: %v", err)
	}

	plan, err := s.GetActivePlan(ctx, projID)
	if err == nil {
		t.Fatalf("expected GetActivePlan to bubble LoadForecast failure, got success: %+v", plan)
	}
	if !strings.Contains(err.Error(), "load forecast for active plan check failed") {
		t.Errorf("expected error to carry step identifier %q, got: %v",
			"load forecast for active plan check failed", err)
	}
}

// DeleteChapter must reject non-latest chapters like UncommitChapter does,
// so "delete N+1 then uncommit N" can no longer orphan mutation effects.
func TestSQLiteStore_DeleteChapter_NonLatestRejected(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_planfix_delete_guard.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_planfix_del"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "删除守卫验证"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	ch1Content := "第一章正文不可丢失"
	if _, err := s.CommitChapter(ctx, projID, &domain.Chapter{
		ID:           "ch_planfix_del_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      ch1Content,
		CreatedAt:    time.Now().Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}
	if _, err := s.CommitChapter(ctx, projID, &domain.Chapter{
		ID:           "ch_planfix_del_2",
		ProjectID:    projID,
		ChapterIndex: 2,
		Title:        "第二章",
		Content:      "第二章正文",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// Deleting a non-latest chapter must be rejected.
	if err := s.DeleteChapter(ctx, projID, 1); err == nil {
		t.Fatalf("expected DeleteChapter(1) to fail while chapter 2 exists, got success")
	} else {
		if !errors.Is(err, ErrNonLatestChapter) {
			t.Errorf("expected errors.Is(err, ErrNonLatestChapter) to be true, got: %v", err)
		}
		var nlcErr *NonLatestChapterError
		if !errors.As(err, &nlcErr) {
			t.Errorf("expected errors.As to populate *NonLatestChapterError, got: %v", err)
		} else if nlcErr.ChapterIndex != 1 || nlcErr.MaxChapter != 2 || len(nlcErr.Downstream) != 1 || nlcErr.Downstream[0] != 2 {
			t.Errorf("unexpected NonLatestChapterError fields: %+v", nlcErr)
		}
	}

	// Chapter 1 must still be intact.
	still, err := s.GetChapter(ctx, projID, 1)
	if err != nil || still == nil {
		t.Fatalf("expected chapter 1 to remain readable, err: %v", err)
	}
	if still.Content != ch1Content {
		t.Errorf("expected chapter 1 content intact, got %q", still.Content)
	}

	// Deleting the latest chapter still succeeds.
	if err := s.DeleteChapter(ctx, projID, 2); err != nil {
		t.Fatalf("expected DeleteChapter(2 latest) to succeed, got: %v", err)
	}
	if _, err := s.GetChapter(ctx, projID, 2); err == nil {
		t.Fatalf("expected chapter 2 to be gone after latest delete, got success")
	}
}
