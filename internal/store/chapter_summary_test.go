package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestChapterSummaryStore_RoundtripAndProjectIsolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_summary.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	now := time.Now().Truncate(time.Second)

	// 1. Prepare data for Project A
	csA1 := &domain.ChapterSummary{
		ProjectID:    "proj-alpha",
		ChapterIndex: 1,
		Text:         "林凡在黑煞谷遭青云门截杀，反手祭出残剑斩敌首，坠入葬仙渊。",
		Model:        "deepseek-reasoner",
		Tokens:       180,
		CreatedAt:    now,
	}
	csA2 := &domain.ChapterSummary{
		ProjectID:    "proj-alpha",
		ChapterIndex: 2,
		Text:         "林凡在深渊洞府偶获上古剑诀，强行炼化魔血恢复伤势，引动雷劫异象。",
		Model:        "deepseek-reasoner",
		Tokens:       195,
		CreatedAt:    now.Add(time.Minute),
	}

	// 2. Prepare data for Project B
	csB1 := &domain.ChapterSummary{
		ProjectID:    "proj-beta",
		ChapterIndex: 1,
		Text:         "楚云飞初入太虚丹宗，于外门大比一拳击溃执事之子，夺得地火丹炉。",
		Model:        "deepseek-reasoner",
		Tokens:       175,
		CreatedAt:    now,
	}

	// 3. Save summaries
	if err := s.SaveChapterSummary(ctx, csA1); err != nil {
		t.Fatalf("SaveChapterSummary csA1 failed: %v", err)
	}
	if err := s.SaveChapterSummary(ctx, csA2); err != nil {
		t.Fatalf("SaveChapterSummary csA2 failed: %v", err)
	}
	if err := s.SaveChapterSummary(ctx, csB1); err != nil {
		t.Fatalf("SaveChapterSummary csB1 failed: %v", err)
	}

	// 4. Test Roundtrip for Project A
	gotA1, err := s.GetChapterSummary(ctx, "proj-alpha", 1)
	if err != nil {
		t.Fatalf("GetChapterSummary proj-alpha ch1 failed: %v", err)
	}
	if gotA1 == nil {
		t.Fatalf("expected non-nil summary for proj-alpha ch1")
	}
	if gotA1.Text != csA1.Text || gotA1.Model != csA1.Model || gotA1.Tokens != csA1.Tokens {
		t.Errorf("roundtrip mismatch for proj-alpha ch1: got %+v, want %+v", gotA1, csA1)
	}

	gotA2, err := s.GetChapterSummary(ctx, "proj-alpha", 2)
	if err != nil {
		t.Fatalf("GetChapterSummary proj-alpha ch2 failed: %v", err)
	}
	if gotA2 == nil || gotA2.Text != csA2.Text {
		t.Errorf("roundtrip mismatch for proj-alpha ch2: got %+v, want %+v", gotA2, csA2)
	}

	// 5. Test Cross-Project Isolation
	// Project B should NOT be able to read Project A's chapter 2
	gotB2, err := s.GetChapterSummary(ctx, "proj-beta", 2)
	if err != nil {
		t.Fatalf("GetChapterSummary proj-beta ch2 should not error: %v", err)
	}
	if gotB2 != nil {
		t.Errorf("cross-project leak: proj-beta should not read proj-alpha's chapter 2, got: %+v", gotB2)
	}

	// Project B chapter 1 should return its own text, not Project A's
	gotB1, err := s.GetChapterSummary(ctx, "proj-beta", 1)
	if err != nil {
		t.Fatalf("GetChapterSummary proj-beta ch1 failed: %v", err)
	}
	if gotB1 == nil || gotB1.Text != csB1.Text {
		t.Errorf("proj-beta ch1 mismatch: got %+v, want %+v", gotB1, csB1)
	}

	// List summaries isolation
	listA, err := s.ListChapterSummaries(ctx, "proj-alpha", 1, 10)
	if err != nil {
		t.Fatalf("ListChapterSummaries proj-alpha failed: %v", err)
	}
	if len(listA) != 2 {
		t.Fatalf("expected 2 summaries for proj-alpha, got %d", len(listA))
	}
	for _, item := range listA {
		if item.ProjectID != "proj-alpha" {
			t.Errorf("found alien project in listA: %s", item.ProjectID)
		}
	}

	listB, err := s.ListChapterSummaries(ctx, "proj-beta", 1, 10)
	if err != nil {
		t.Fatalf("ListChapterSummaries proj-beta failed: %v", err)
	}
	if len(listB) != 1 {
		t.Fatalf("expected 1 summary for proj-beta, got %d", len(listB))
	}
	if listB[0].ProjectID != "proj-beta" || listB[0].Text != csB1.Text {
		t.Errorf("proj-beta list content mismatch: %+v", listB[0])
	}

	// 6. Volume Retrospective Roundtrip & Isolation
	vrA := &domain.VolumeRetrospective{
		ProjectID:   "proj-alpha",
		VolumeIndex: 1,
		Text:        "【发生】林凡破局斩敌。【留下】道基受损待愈。【未回收】深渊古剑秘密未解。",
		FromChapter: 1,
		ToChapter:   40,
		CreatedAt:   now,
	}
	if err := s.SaveVolumeRetrospective(ctx, vrA); err != nil {
		t.Fatalf("SaveVolumeRetrospective failed: %v", err)
	}

	gotVrA, err := s.GetVolumeRetrospective(ctx, "proj-alpha", 1)
	if err != nil {
		t.Fatalf("GetVolumeRetrospective proj-alpha failed: %v", err)
	}
	if gotVrA == nil || gotVrA.Text != vrA.Text || gotVrA.FromChapter != 1 || gotVrA.ToChapter != 40 {
		t.Errorf("volume retrospective roundtrip mismatch: got %+v, want %+v", gotVrA, vrA)
	}

	// Project B should NOT see Project A's retrospective
	gotVrB, err := s.GetVolumeRetrospective(ctx, "proj-beta", 1)
	if err != nil {
		t.Fatalf("GetVolumeRetrospective proj-beta failed: %v", err)
	}
	if gotVrB != nil {
		t.Errorf("cross-project leak: proj-beta should not read proj-alpha's volume retrospective, got: %+v", gotVrB)
	}

	// 7. Upsert Verification
	csA1Updated := &domain.ChapterSummary{
		ProjectID:    "proj-alpha",
		ChapterIndex: 1,
		Text:         "林凡在黑煞谷遭青云门截杀，祭残剑斩敌首后重伤坠渊。",
		Model:        "deepseek-reasoner",
		Tokens:       210,
		CreatedAt:    now.Add(2 * time.Minute),
	}
	if err := s.SaveChapterSummary(ctx, csA1Updated); err != nil {
		t.Fatalf("upsert SaveChapterSummary failed: %v", err)
	}
	gotA1Updated, err := s.GetChapterSummary(ctx, "proj-alpha", 1)
	if err != nil {
		t.Fatalf("GetChapterSummary after upsert failed: %v", err)
	}
	if gotA1Updated.Text != csA1Updated.Text || gotA1Updated.Tokens != 210 {
		t.Errorf("upsert failed to update fields: got %+v", gotA1Updated)
	}
}

func TestChapterSummaryStore_EnsureTablesIdempotent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_idempotent.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	// Call ensureSummaryTables consecutively multiple times
	if err := s.ensureSummaryTables(ctx); err != nil {
		t.Fatalf("first call to ensureSummaryTables failed: %v", err)
	}
	if err := s.ensureSummaryTables(ctx); err != nil {
		t.Fatalf("second call to ensureSummaryTables failed: %v", err)
	}
	if err := s.EnsureSummaryTables(ctx); err != nil {
		t.Fatalf("third call to EnsureSummaryTables failed: %v", err)
	}

	// Verify database operations work properly after multiple ensure calls
	cs := &domain.ChapterSummary{
		ProjectID:    "proj-idem",
		ChapterIndex: 1,
		Text:         "测试摘要内容",
		Model:        "test-model",
		Tokens:       50,
		CreatedAt:    time.Now(),
	}
	if err := s.SaveChapterSummary(ctx, cs); err != nil {
		t.Fatalf("SaveChapterSummary after multiple ensure calls failed: %v", err)
	}
	got, err := s.GetChapterSummary(ctx, "proj-idem", 1)
	if err != nil || got == nil || got.Text != cs.Text {
		t.Fatalf("GetChapterSummary failed after multiple ensure calls: %v, got: %+v", err, got)
	}
}
