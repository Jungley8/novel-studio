package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupTestStore(t *testing.T) (store.Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_intents.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	projectID := "proj-test-intents"
	p := &domain.Project{
		ID:             projectID,
		Title:          "意图卡测试项目",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "凡人修仙体系",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(context.Background(), p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	return s, projectID
}

func TestSQLiteStore_ChapterIntent_Roundtrip(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupTestStore(t)

	mutation := domain.StateMutation{
		InventoryDelta: "+龙鳞草 x2, -下品灵石 x10",
		PowerDelta:     "突破筑基前期巅峰",
		CharacterMutations: []domain.CharacterMutation{
			{
				Name:          "林动",
				StatusDelta:   "突破淬体九重，心境杀伐果断",
				RelationDelta: "与青云宗执事结下死仇",
			},
		},
	}

	intent := &domain.ChapterIntent{
		ID:               "intent-ch-1",
		ProjectID:        projectID,
		ChapterIndex:     1,
		VolumeIndex:      1,
		Conflict:         "林动在后山遭遇强敌伏击，被迫动用底牌",
		POV:              "林动",
		EmotionCurve:     []string{"压抑", "危机", "爆发", "释然"},
		HookPlants:       []string{"后山深处的异光"},
		HookPayoffs:      []string{"此前获得的残破阵盘"},
		ExpectedMutation: mutation,
		TensionTarget:    8,
		Status:           domain.IntentStatusLocked,
	}

	// 1. Save valid intent
	if err := s.SaveChapterIntent(ctx, intent); err != nil {
		t.Fatalf("SaveChapterIntent failed: %v", err)
	}

	// 2. Get and verify all fields (including nested ExpectedMutation)
	got, err := s.GetChapterIntent(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("GetChapterIntent failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected intent to exist, got nil")
	}

	if got.ProjectID != projectID {
		t.Errorf("expected ProjectID %q, got %q", projectID, got.ProjectID)
	}
	if got.ChapterIndex != 1 {
		t.Errorf("expected ChapterIndex 1, got %d", got.ChapterIndex)
	}
	if got.VolumeIndex != 1 {
		t.Errorf("expected VolumeIndex 1, got %d", got.VolumeIndex)
	}
	if got.Conflict != intent.Conflict {
		t.Errorf("expected Conflict %q, got %q", intent.Conflict, got.Conflict)
	}
	if got.POV != "林动" {
		t.Errorf("expected POV '林动', got %q", got.POV)
	}
	if len(got.EmotionCurve) != 4 || got.EmotionCurve[2] != "爆发" {
		t.Errorf("unexpected EmotionCurve: %v", got.EmotionCurve)
	}
	if len(got.HookPlants) != 1 || got.HookPlants[0] != "后山深处的异光" {
		t.Errorf("unexpected HookPlants: %v", got.HookPlants)
	}
	if len(got.HookPayoffs) != 1 || got.HookPayoffs[0] != "此前获得的残破阵盘" {
		t.Errorf("unexpected HookPayoffs: %v", got.HookPayoffs)
	}
	if got.TensionTarget != 8 {
		t.Errorf("expected TensionTarget 8, got %d", got.TensionTarget)
	}
	if got.Status != domain.IntentStatusLocked {
		t.Errorf("expected Status %q, got %q", domain.IntentStatusLocked, got.Status)
	}
	if got.ExpectedMutation.InventoryDelta != mutation.InventoryDelta {
		t.Errorf("expected InventoryDelta %q, got %q", mutation.InventoryDelta, got.ExpectedMutation.InventoryDelta)
	}
	if got.ExpectedMutation.PowerDelta != mutation.PowerDelta {
		t.Errorf("expected PowerDelta %q, got %q", mutation.PowerDelta, got.ExpectedMutation.PowerDelta)
	}
	if len(got.ExpectedMutation.CharacterMutations) != 1 || got.ExpectedMutation.CharacterMutations[0].Name != "林动" {
		t.Errorf("unexpected CharacterMutations: %v", got.ExpectedMutation.CharacterMutations)
	}
	if got.ExpectedMutation.CharacterMutations[0].StatusDelta != "突破淬体九重，心境杀伐果断" {
		t.Errorf("unexpected StatusDelta: %q", got.ExpectedMutation.CharacterMutations[0].StatusDelta)
	}
	if got.ExpectedMutation.CharacterMutations[0].RelationDelta != "与青云宗执事结下死仇" {
		t.Errorf("unexpected RelationDelta: %q", got.ExpectedMutation.CharacterMutations[0].RelationDelta)
	}

	// 3. Fail-Closed validation tests on SaveChapterIntent
	if err := s.SaveChapterIntent(ctx, nil); err == nil {
		t.Error("expected error for nil intent, got nil")
	}

	intentNoProj := *intent
	intentNoProj.ProjectID = ""
	if err := s.SaveChapterIntent(ctx, &intentNoProj); err == nil {
		t.Error("expected error for empty project_id, got nil")
	}

	intentBadChapter := *intent
	intentBadChapter.ChapterIndex = 0
	if err := s.SaveChapterIntent(ctx, &intentBadChapter); err == nil {
		t.Error("expected error for chapter_index <= 0, got nil")
	}

	intentBadTensionLow := *intent
	intentBadTensionLow.TensionTarget = 0
	if err := s.SaveChapterIntent(ctx, &intentBadTensionLow); err == nil {
		t.Error("expected error for tension_target < 1, got nil")
	}

	intentBadTensionHigh := *intent
	intentBadTensionHigh.TensionTarget = 11
	if err := s.SaveChapterIntent(ctx, &intentBadTensionHigh); err == nil {
		t.Error("expected error for tension_target > 10, got nil")
	}

	intentEmptyConflict := *intent
	intentEmptyConflict.Conflict = "   "
	if err := s.SaveChapterIntent(ctx, &intentEmptyConflict); err == nil {
		t.Error("expected error for empty conflict, got nil")
	}

	intentBadStatus := *intent
	intentBadStatus.Status = "INVALID_STATUS"
	if err := s.SaveChapterIntent(ctx, &intentBadStatus); err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

func TestSQLiteStore_ChapterIntent_GetNonExistent(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupTestStore(t)

	got, err := s.GetChapterIntent(ctx, projectID, 999)
	if err != nil {
		t.Fatalf("GetChapterIntent on non-existent failed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected (nil, nil) for non-existent intent, got: %+v", got)
	}
}

func TestSQLiteStore_ChapterIntent_ListFilter(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupTestStore(t)

	// Insert chapters 1, 2 in Vol 1 and 3, 4 in Vol 2 (out of order to test ordering)
	intentsToInsert := []*domain.ChapterIntent{
		{
			ProjectID:     projectID,
			ChapterIndex:  3,
			VolumeIndex:   2,
			Conflict:      "第三章冲突",
			TensionTarget: 6,
			Status:        domain.IntentStatusDraft,
		},
		{
			ProjectID:     projectID,
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "第一章冲突",
			TensionTarget: 7,
			Status:        domain.IntentStatusDraft,
		},
		{
			ProjectID:     projectID,
			ChapterIndex:  4,
			VolumeIndex:   2,
			Conflict:      "第四章冲突",
			TensionTarget: 8,
			Status:        domain.IntentStatusDraft,
		},
		{
			ProjectID:     projectID,
			ChapterIndex:  2,
			VolumeIndex:   1,
			Conflict:      "第二章冲突",
			TensionTarget: 5,
			Status:        domain.IntentStatusDraft,
		},
	}

	for _, ci := range intentsToInsert {
		if err := s.SaveChapterIntent(ctx, ci); err != nil {
			t.Fatalf("SaveChapterIntent ch %d failed: %v", ci.ChapterIndex, err)
		}
	}

	// 1. volumeIndex <= 0: list all, ordered by chapter_index ASC
	all, err := s.ListChapterIntents(ctx, projectID, 0)
	if err != nil {
		t.Fatalf("ListChapterIntents(0) failed: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 intents, got %d", len(all))
	}
	for i, expectedCh := range []int{1, 2, 3, 4} {
		if all[i].ChapterIndex != expectedCh {
			t.Errorf("at index %d, expected chapter %d, got %d", i, expectedCh, all[i].ChapterIndex)
		}
	}

	// Negative volumeIndex also lists all
	allNeg, err := s.ListChapterIntents(ctx, projectID, -1)
	if err != nil {
		t.Fatalf("ListChapterIntents(-1) failed: %v", err)
	}
	if len(allNeg) != 4 {
		t.Fatalf("expected 4 intents for negative volume, got %d", len(allNeg))
	}

	// 2. volumeIndex == 1: only Volume 1 (chapters 1, 2)
	vol1, err := s.ListChapterIntents(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("ListChapterIntents(1) failed: %v", err)
	}
	if len(vol1) != 2 {
		t.Fatalf("expected 2 intents for volume 1, got %d", len(vol1))
	}
	if vol1[0].ChapterIndex != 1 || vol1[1].ChapterIndex != 2 {
		t.Errorf("expected [1, 2] for volume 1, got [%d, %d]", vol1[0].ChapterIndex, vol1[1].ChapterIndex)
	}

	// 3. volumeIndex == 2: only Volume 2 (chapters 3, 4)
	vol2, err := s.ListChapterIntents(ctx, projectID, 2)
	if err != nil {
		t.Fatalf("ListChapterIntents(2) failed: %v", err)
	}
	if len(vol2) != 2 {
		t.Fatalf("expected 2 intents for volume 2, got %d", len(vol2))
	}
	if vol2[0].ChapterIndex != 3 || vol2[1].ChapterIndex != 4 {
		t.Errorf("expected [3, 4] for volume 2, got [%d, %d]", vol2[0].ChapterIndex, vol2[1].ChapterIndex)
	}

	// 4. Non-existent volume: returns empty slice, not nil
	empty, err := s.ListChapterIntents(ctx, projectID, 99)
	if err != nil {
		t.Fatalf("ListChapterIntents(99) failed: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("expected empty slice, got %v", empty)
	}
}

func TestSQLiteStore_ChapterIntent_DeleteIdempotent(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupTestStore(t)

	intent := &domain.ChapterIntent{
		ProjectID:     projectID,
		ChapterIndex:  10,
		VolumeIndex:   1,
		Conflict:      "第十章决战",
		TensionTarget: 9,
		Status:        domain.IntentStatusDraft,
	}
	if err := s.SaveChapterIntent(ctx, intent); err != nil {
		t.Fatalf("SaveChapterIntent failed: %v", err)
	}

	// First delete: succeeds
	if err := s.DeleteChapterIntent(ctx, projectID, 10); err != nil {
		t.Fatalf("DeleteChapterIntent failed: %v", err)
	}

	// Verify it's gone
	got, err := s.GetChapterIntent(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("GetChapterIntent failed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}

	// Second delete: idempotent, must succeed with nil error
	if err := s.DeleteChapterIntent(ctx, projectID, 10); err != nil {
		t.Fatalf("idempotent DeleteChapterIntent failed: %v", err)
	}
}

func TestSQLiteStore_ChapterIntent_MarkIntentsStale(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupTestStore(t)

	// Prepare Volume 1:
	// - Chapter 1: LOCKED
	// - Chapter 2: DRAFT
	// - Chapter 3: STALE
	// Prepare Volume 2:
	// - Chapter 4: LOCKED
	intents := []*domain.ChapterIntent{
		{
			ProjectID:     projectID,
			ChapterIndex:  1,
			VolumeIndex:   1,
			Conflict:      "第一章：已被锁定",
			TensionTarget: 7,
			Status:        domain.IntentStatusLocked,
		},
		{
			ProjectID:     projectID,
			ChapterIndex:  2,
			VolumeIndex:   1,
			Conflict:      "第二章：草稿意图",
			TensionTarget: 5,
			Status:        domain.IntentStatusDraft,
		},
		{
			ProjectID:     projectID,
			ChapterIndex:  3,
			VolumeIndex:   1,
			Conflict:      "第三章：既有陈旧意图",
			TensionTarget: 6,
			Status:        domain.IntentStatusStale,
		},
		{
			ProjectID:     projectID,
			ChapterIndex:  4,
			VolumeIndex:   2,
			Conflict:      "第四章：第二卷锁定意图",
			TensionTarget: 8,
			Status:        domain.IntentStatusLocked,
		},
	}

	for _, ci := range intents {
		if err := s.SaveChapterIntent(ctx, ci); err != nil {
			t.Fatalf("SaveChapterIntent ch %d failed: %v", ci.ChapterIndex, err)
		}
	}

	// Trigger MarkIntentsStale on Volume 1
	changed, err := s.MarkIntentsStale(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("MarkIntentsStale failed: %v", err)
	}
	// Only Chapter 1 (LOCKED) should change to STALE! (changed == 1)
	if changed != 1 {
		t.Fatalf("expected exactly 1 row changed, got %d", changed)
	}

	// Verify Chapter 1 became STALE
	ch1, err := s.GetChapterIntent(ctx, projectID, 1)
	if err != nil || ch1 == nil {
		t.Fatalf("GetChapterIntent 1 failed: %v", err)
	}
	if ch1.Status != domain.IntentStatusStale {
		t.Errorf("expected Chapter 1 status STALE, got %s", ch1.Status)
	}

	// Verify Chapter 2 remains DRAFT (not touched)
	ch2, err := s.GetChapterIntent(ctx, projectID, 2)
	if err != nil || ch2 == nil {
		t.Fatalf("GetChapterIntent 2 failed: %v", err)
	}
	if ch2.Status != domain.IntentStatusDraft {
		t.Errorf("expected Chapter 2 status DRAFT, got %s", ch2.Status)
	}

	// Verify Chapter 3 remains STALE (not counted as changed)
	ch3, err := s.GetChapterIntent(ctx, projectID, 3)
	if err != nil || ch3 == nil {
		t.Fatalf("GetChapterIntent 3 failed: %v", err)
	}
	if ch3.Status != domain.IntentStatusStale {
		t.Errorf("expected Chapter 3 status STALE, got %s", ch3.Status)
	}

	// Verify Chapter 4 in Volume 2 remains LOCKED (different volume)
	ch4, err := s.GetChapterIntent(ctx, projectID, 4)
	if err != nil || ch4 == nil {
		t.Fatalf("GetChapterIntent 4 failed: %v", err)
	}
	if ch4.Status != domain.IntentStatusLocked {
		t.Errorf("expected Chapter 4 in Volume 2 status LOCKED, got %s", ch4.Status)
	}

	// Second call: no LOCKED intents left in Volume 1, so changed count must be 0
	changed2, err := s.MarkIntentsStale(ctx, projectID, 1)
	if err != nil {
		t.Fatalf("second MarkIntentsStale failed: %v", err)
	}
	if changed2 != 0 {
		t.Errorf("expected 0 rows changed on second run, got %d", changed2)
	}
}
