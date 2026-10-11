package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func TestSQLiteStore_ChapterMetrics(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_metrics.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projectID := "p_test_metrics"
	p := &domain.Project{
		ID:             projectID,
		Title:          "指标测试项目",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "凡人修仙体系",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. Roundtrip
	t.Run("roundtrip save and list", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		m1 := &domain.ChapterQualityMetrics{
			ProjectID:              projectID,
			ChapterIndex:           1,
			RunID:                  "run-101",
			ActualWords:            2200,
			WordsTarget:            2000,
			WordQuotaRatio:         1.1,
			UnderTarget:            false,
			Verdict:                "ACCEPTED",
			Score:                  92,
			RewriteLoops:           0,
			UnresolvedObservations: 0,
			HookClaimsTotal:        3,
			HookClaimsValidated:    3,
			HookClaimsRejected:     0,
			CompactionFailed:       false,
			CreatedAt:              now,
		}

		if err := s.SaveChapterMetrics(ctx, m1); err != nil {
			t.Fatalf("SaveChapterMetrics failed: %v", err)
		}

		list, err := s.ListChapterMetrics(ctx, projectID)
		if err != nil {
			t.Fatalf("ListChapterMetrics failed: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 item, got %d", len(list))
		}
		got := list[0]
		if got.ProjectID != m1.ProjectID || got.ChapterIndex != m1.ChapterIndex || got.RunID != m1.RunID {
			t.Errorf("identity mismatch: %+v", got)
		}
		if got.ActualWords != m1.ActualWords || got.WordsTarget != m1.WordsTarget || got.WordQuotaRatio != m1.WordQuotaRatio {
			t.Errorf("words/ratio mismatch: %+v", got)
		}
		if got.UnderTarget != m1.UnderTarget || got.Verdict != m1.Verdict || got.Score != m1.Score {
			t.Errorf("verdict/score mismatch: %+v", got)
		}
		if got.RewriteLoops != m1.RewriteLoops || got.UnresolvedObservations != m1.UnresolvedObservations {
			t.Errorf("loops/observations mismatch: %+v", got)
		}
		if got.HookClaimsTotal != m1.HookClaimsTotal || got.HookClaimsValidated != m1.HookClaimsValidated || got.HookClaimsRejected != m1.HookClaimsRejected {
			t.Errorf("hooks mismatch: %+v", got)
		}
		if got.CompactionFailed != m1.CompactionFailed {
			t.Errorf("compaction_failed mismatch: got %v, expected %v", got.CompactionFailed, m1.CompactionFailed)
		}
	})

	// 2. Upsert 覆盖
	t.Run("upsert overwrites all fields", func(t *testing.T) {
		mUpdated := &domain.ChapterQualityMetrics{
			ProjectID:              projectID,
			ChapterIndex:           1,
			RunID:                  "run-101-retry",
			ActualWords:            2400,
			WordsTarget:            2000,
			WordQuotaRatio:         1.2,
			UnderTarget:            false,
			Verdict:                "ACCEPTED",
			Score:                  98,
			RewriteLoops:           1,
			UnresolvedObservations: 0,
			HookClaimsTotal:        4,
			HookClaimsValidated:    4,
			HookClaimsRejected:     0,
			CompactionFailed:       false,
			CreatedAt:              time.Now(),
		}

		if err := s.SaveChapterMetrics(ctx, mUpdated); err != nil {
			t.Fatalf("SaveChapterMetrics upsert failed: %v", err)
		}

		list, err := s.ListChapterMetrics(ctx, projectID)
		if err != nil {
			t.Fatalf("ListChapterMetrics failed: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 item after upsert, got %d", len(list))
		}
		got := list[0]
		if got.ActualWords != 2400 || got.Score != 98 || got.RewriteLoops != 1 || got.RunID != "run-101-retry" {
			t.Errorf("upsert fields not updated: %+v", got)
		}
	})

	// 3. List 升序 (chapter_index ASC)
	t.Run("list sorted by chapter_index ASC", func(t *testing.T) {
		m3 := &domain.ChapterQualityMetrics{
			ProjectID:    projectID,
			ChapterIndex: 3,
			ActualWords:  2000,
			WordsTarget:  2000,
			Verdict:      "ACCEPTED",
			Score:        88,
		}
		m2 := &domain.ChapterQualityMetrics{
			ProjectID:    projectID,
			ChapterIndex: 2,
			ActualWords:  1500,
			WordsTarget:  2000,
			UnderTarget:  true,
			Verdict:      "ACCEPTED",
			Score:        80,
		}

		if err := s.SaveChapterMetrics(ctx, m3); err != nil {
			t.Fatalf("Save m3 failed: %v", err)
		}
		if err := s.SaveChapterMetrics(ctx, m2); err != nil {
			t.Fatalf("Save m2 failed: %v", err)
		}

		list, err := s.ListChapterMetrics(ctx, projectID)
		if err != nil {
			t.Fatalf("ListChapterMetrics failed: %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("expected 3 items, got %d", len(list))
		}
		if list[0].ChapterIndex != 1 || list[1].ChapterIndex != 2 || list[2].ChapterIndex != 3 {
			t.Fatalf("expected ASC order [1, 2, 3], got [%d, %d, %d]",
				list[0].ChapterIndex, list[1].ChapterIndex, list[2].ChapterIndex)
		}
	})

	// 4. Project 隔离
	t.Run("project isolation", func(t *testing.T) {
		otherProjID := "p_other_metrics"
		list, err := s.ListChapterMetrics(ctx, otherProjID)
		if err != nil {
			t.Fatalf("ListChapterMetrics failed: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected 0 items for other project, got %d", len(list))
		}
	})
}
