package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestStructureFindingStore_CRUDAndIsolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_structure_audit.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	now := time.Now().Truncate(time.Second)

	f1 := &domain.StructureFinding{
		ID:           "find_1",
		ProjectID:    "proj_A",
		Type:         domain.StructureFindingTypePlotHole,
		Severity:     domain.StructureFindingSeverityHigh,
		ChapterStart: 10,
		ChapterEnd:   12,
		Summary:      "第10章反派死而复生未给解释",
		Evidence:     "第8章明确斩首，第10章无任何法术说明再次出现",
		Suggestion:   "补充替身傀儡或假死符说明",
		Status:       domain.StructureFindingStatusOpen,
		CreatedAt:    now,
	}

	f2 := &domain.StructureFinding{
		ID:           "find_2",
		ProjectID:    "proj_A",
		Type:         domain.StructureFindingTypePacing,
		Severity:     domain.StructureFindingSeverityMedium,
		ChapterStart: 3,
		ChapterEnd:   6,
		Summary:      "连续3章拍卖会剧情拖沓",
		Evidence:     "第3-6章无核心冲突推进，均为配角叫价",
		Suggestion:   "压缩为单章高潮拍卖",
		Status:       domain.StructureFindingStatusOpen,
		CreatedAt:    now.Add(time.Second),
	}

	f3 := &domain.StructureFinding{
		ID:           "find_3",
		ProjectID:    "proj_B",
		Type:         domain.StructureFindingTypeDroppedHook,
		Severity:     domain.StructureFindingSeverityLow,
		ChapterStart: 1,
		ChapterEnd:   15,
		Summary:      "开局信物玉佩写丢",
		Evidence:     "第1章重点描写，后续14章未再提及",
		Suggestion:   "在中期探索遗迹时触发玉佩共鸣",
		Status:       domain.StructureFindingStatusOpen,
		CreatedAt:    now,
	}

	if err := s.SaveProject(ctx, &domain.Project{ID: "proj_A", Title: "Project A", TargetPlatform: "起点"}); err != nil {
		t.Fatalf("save proj_A failed: %v", err)
	}
	if err := s.SaveProject(ctx, &domain.Project{ID: "proj_B", Title: "Project B", TargetPlatform: "番茄"}); err != nil {
		t.Fatalf("save proj_B failed: %v", err)
	}

	// 1. Save findings
	if err := s.SaveStructureFinding(ctx, f1); err != nil {
		t.Fatalf("save f1 failed: %v", err)
	}
	if err := s.SaveStructureFinding(ctx, f2); err != nil {
		t.Fatalf("save f2 failed: %v", err)
	}
	if err := s.SaveStructureFinding(ctx, f3); err != nil {
		t.Fatalf("save f3 failed: %v", err)
	}

	// 2. List and check ordering (chapter_start ASC, so f2(ch 3) must be before f1(ch 10))
	listA, err := s.ListStructureFindings(ctx, "proj_A")
	if err != nil {
		t.Fatalf("list proj_A failed: %v", err)
	}
	if len(listA) != 2 {
		t.Fatalf("expected 2 findings for proj_A, got: %d", len(listA))
	}
	if listA[0].ID != "find_2" || listA[0].ChapterStart != 3 {
		t.Errorf("expected first item to be find_2 (ch 3), got: %+v", listA[0])
	}
	if listA[1].ID != "find_1" || listA[1].ChapterStart != 10 {
		t.Errorf("expected second item to be find_1 (ch 10), got: %+v", listA[1])
	}

	// Verify project isolation
	listB, err := s.ListStructureFindings(ctx, "proj_B")
	if err != nil {
		t.Fatalf("list proj_B failed: %v", err)
	}
	if len(listB) != 1 || listB[0].ID != "find_3" {
		t.Fatalf("expected 1 finding for proj_B, got: %v", listB)
	}

	// 3. Update status
	if err := s.UpdateStructureFindingStatus(ctx, "find_1", string(domain.StructureFindingStatusResolved)); err != nil {
		t.Fatalf("update status to RESOLVED failed: %v", err)
	}
	updatedList, err := s.ListStructureFindings(ctx, "proj_A")
	if err != nil {
		t.Fatalf("list after status update failed: %v", err)
	}
	if updatedList[1].Status != domain.StructureFindingStatusResolved {
		t.Errorf("expected find_1 status RESOLVED, got: %s", updatedList[1].Status)
	}

	// Update with invalid status should fail
	if err := s.UpdateStructureFindingStatus(ctx, "find_1", "INVALID_STATUS"); err == nil {
		t.Fatal("expected error on invalid status update")
	}

	// 4. Delete findings for project A
	if err := s.DeleteStructureFindingsByProject(ctx, "proj_A"); err != nil {
		t.Fatalf("delete findings for proj_A failed: %v", err)
	}

	listAAfterDel, err := s.ListStructureFindings(ctx, "proj_A")
	if err != nil {
		t.Fatalf("list after delete failed: %v", err)
	}
	if len(listAAfterDel) != 0 {
		t.Fatalf("expected 0 findings after deletion, got: %d", len(listAAfterDel))
	}

	// proj_B findings must remain untouched
	listBAfterDel, err := s.ListStructureFindings(ctx, "proj_B")
	if err != nil {
		t.Fatalf("list proj_B after deleting proj_A failed: %v", err)
	}
	if len(listBAfterDel) != 1 {
		t.Fatalf("expected 1 finding remaining for proj_B, got: %d", len(listBAfterDel))
	}
}
