package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupAutomationTestStore(t *testing.T) (store.Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_automation.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	projectID := "proj-auto-1"
	p := &domain.Project{
		ID:             projectID,
		Title:          "排程测试作品",
		TargetPlatform: "番茄脑洞",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(context.Background(), p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	return s, projectID
}

func TestSQLiteStore_ProjectAutomation_DefaultRow(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupAutomationTestStore(t)

	// ListProjectAutomations should return a default row for the project that has no configured automation
	items, err := s.ListProjectAutomations(ctx)
	if err != nil {
		t.Fatalf("ListProjectAutomations failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 automation item, got %d", len(items))
	}

	item := items[0]
	if item.ProjectID != projectID {
		t.Errorf("expected project ID %q, got %q", projectID, item.ProjectID)
	}
	if item.ProjectTitle != "排程测试作品" {
		t.Errorf("expected title '排程测试作品', got %q", item.ProjectTitle)
	}
	if item.Enabled {
		t.Errorf("expected enabled false, got true")
	}
	if item.Times == nil {
		t.Errorf("expected non-nil Times slice")
	}
	if len(item.Times) != 0 {
		t.Errorf("expected empty Times slice, got %v", item.Times)
	}
	if item.QuotaKind != domain.QuotaKindChapters {
		t.Errorf("expected quota kind chapters, got %q", item.QuotaKind)
	}
	if item.QuotaCount != 5 {
		t.Errorf("expected quota count 5, got %d", item.QuotaCount)
	}
	if item.Paused {
		t.Errorf("expected paused false, got true")
	}
	if item.PausedReason != "" || item.LastRunAt != "" || item.LastStatus != "" || item.LastMessage != "" {
		t.Errorf("expected empty paused_reason/last_* fields, got %+v", item)
	}
}

func TestSQLiteStore_ProjectAutomation_GetNotFound(t *testing.T) {
	ctx := context.Background()
	s, _ := setupAutomationTestStore(t)

	_, err := s.GetProjectAutomation(ctx, "non-existent-proj")
	if err == nil {
		t.Fatalf("expected error for non-existent automation, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected errors.Is(err, store.ErrNotFound), got %v", err)
	}
}

func TestSQLiteStore_ProjectAutomation_SaveValidationFailClosed(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupAutomationTestStore(t)

	// Enabled but empty times
	invalidAuto := &domain.ProjectAutomation{
		ProjectID:  projectID,
		Enabled:    true,
		Times:      []string{},
		QuotaKind:  domain.QuotaKindChapters,
		QuotaCount: 5,
	}
	if err := s.SaveProjectAutomation(ctx, invalidAuto); err == nil {
		t.Fatalf("expected validation error for enabled with empty times, got nil")
	}

	// Non-existent project
	validAutoNonExistentProj := &domain.ProjectAutomation{
		ProjectID:  "ghost-project",
		Enabled:    false,
		Times:      []string{},
		QuotaKind:  domain.QuotaKindChapters,
		QuotaCount: 5,
	}
	err := s.SaveProjectAutomation(ctx, validAutoNonExistentProj)
	if err == nil {
		t.Fatalf("expected error for non-existent project, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected errors.Is(err, store.ErrNotFound), got %v", err)
	}
}

func TestSQLiteStore_ProjectAutomation_CRUDAndNoOverwrite(t *testing.T) {
	ctx := context.Background()
	s, projectID := setupAutomationTestStore(t)

	// 1. Initial Save
	auto := &domain.ProjectAutomation{
		ProjectID:  projectID,
		Enabled:    true,
		Times:      []string{"09:00", "21:00"},
		QuotaKind:  domain.QuotaKindChapters,
		QuotaCount: 10,
	}
	if err := s.SaveProjectAutomation(ctx, auto); err != nil {
		t.Fatalf("SaveProjectAutomation failed: %v", err)
	}

	got, err := s.GetProjectAutomation(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProjectAutomation failed: %v", err)
	}
	if !got.Enabled || len(got.Times) != 2 || got.QuotaCount != 10 {
		t.Errorf("unexpected saved automation: %+v", got)
	}
	if got.ProjectTitle != "排程测试作品" {
		t.Errorf("expected title '排程测试作品', got %q", got.ProjectTitle)
	}

	// 2. Set paused & record run
	if err := s.SetAutomationPaused(ctx, projectID, true, "quality_gate_failed"); err != nil {
		t.Fatalf("SetAutomationPaused failed: %v", err)
	}
	runTime := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	if err := s.RecordAutomationRun(ctx, projectID, "halted", "卡在第3章", runTime); err != nil {
		t.Fatalf("RecordAutomationRun failed: %v", err)
	}

	pausedGot, err := s.GetProjectAutomation(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProjectAutomation failed: %v", err)
	}
	if !pausedGot.Paused || pausedGot.PausedReason != "quality_gate_failed" {
		t.Errorf("unexpected paused state: %+v", pausedGot)
	}
	if pausedGot.LastStatus != "halted" || pausedGot.LastMessage != "卡在第3章" {
		t.Errorf("unexpected last run state: %+v", pausedGot)
	}

	// 3. SaveProjectAutomation must NOT overwrite paused or last_*
	updateAuto := &domain.ProjectAutomation{
		ProjectID:  projectID,
		Enabled:    false,
		Times:      []string{},
		QuotaKind:  domain.QuotaKindVolumes,
		QuotaCount: 2,
	}
	if err := s.SaveProjectAutomation(ctx, updateAuto); err != nil {
		t.Fatalf("SaveProjectAutomation update failed: %v", err)
	}

	afterUpdate, err := s.GetProjectAutomation(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProjectAutomation failed: %v", err)
	}
	if afterUpdate.Enabled != false || afterUpdate.QuotaKind != domain.QuotaKindVolumes || afterUpdate.QuotaCount != 2 {
		t.Errorf("expected updated fields to change, got: %+v", afterUpdate)
	}
	// Verify paused and last_* were preserved
	if !afterUpdate.Paused || afterUpdate.PausedReason != "quality_gate_failed" {
		t.Errorf("paused fields were overwritten: %+v", afterUpdate)
	}
	if afterUpdate.LastStatus != "halted" || afterUpdate.LastMessage != "卡在第3章" || afterUpdate.LastRunAt == "" {
		t.Errorf("last run fields were overwritten: %+v", afterUpdate)
	}

	// 4. Resume (SetAutomationPaused false)
	if err := s.SetAutomationPaused(ctx, projectID, false, ""); err != nil {
		t.Fatalf("SetAutomationPaused false failed: %v", err)
	}
	resumed, err := s.GetProjectAutomation(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProjectAutomation failed: %v", err)
	}
	if resumed.Paused || resumed.PausedReason != "" {
		t.Errorf("expected resumed state, got %+v", resumed)
	}

	// 5. SetAutomationPaused and RecordAutomationRun for non-existent return ErrNotFound
	if err := s.SetAutomationPaused(ctx, "non-existent", true, "x"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent SetAutomationPaused, got %v", err)
	}
	if err := s.RecordAutomationRun(ctx, "non-existent", "ok", "msg", time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent RecordAutomationRun, got %v", err)
	}
}

func TestSQLiteStore_BookQueue_FIFO_CRUD(t *testing.T) {
	ctx := context.Background()
	s, _ := setupAutomationTestStore(t)

	// 1. Initially empty
	items, err := s.ListBookQueue(ctx)
	if err != nil {
		t.Fatalf("ListBookQueue failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty queue, got %d items", len(items))
	}

	// 2. Insert items with slight delay or distinct timestamps
	item1 := &domain.BookQueueItem{
		ID:       "queue-item-1",
		Title:    "第一部作品",
		Concept:  "概念设定 1",
		Platform: "起点",
		Volumes:  2,
		AutoLock: true,
	}
	if err := s.SaveBookQueueItem(ctx, item1); err != nil {
		t.Fatalf("SaveBookQueueItem 1 failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	item2 := &domain.BookQueueItem{
		ID:       "queue-item-2",
		Title:    "第二部作品",
		Concept:  "概念设定 2",
		Platform: "番茄",
		Volumes:  1,
		AutoLock: false,
	}
	if err := s.SaveBookQueueItem(ctx, item2); err != nil {
		t.Fatalf("SaveBookQueueItem 2 failed: %v", err)
	}

	// 3. FIFO order check
	items, err = s.ListBookQueue(ctx)
	if err != nil {
		t.Fatalf("ListBookQueue failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "queue-item-1" || items[1].ID != "queue-item-2" {
		t.Errorf("expected FIFO order [item-1, item-2], got [%s, %s]", items[0].ID, items[1].ID)
	}
	if !items[0].AutoLock || items[1].AutoLock {
		t.Errorf("unexpected AutoLock values: item1=%v, item2=%v", items[0].AutoLock, items[1].AutoLock)
	}

	// 4. Get individual item
	got1, err := s.GetBookQueueItem(ctx, "queue-item-1")
	if err != nil {
		t.Fatalf("GetBookQueueItem failed: %v", err)
	}
	if got1.Title != "第一部作品" || got1.Status != domain.QueueStatusPending {
		t.Errorf("unexpected item content: %+v", got1)
	}

	// 5. Update existing item (check created_at does not change)
	item1Updated := &domain.BookQueueItem{
		ID:          "queue-item-1",
		Title:       "第一部作品(已改名)",
		Concept:     "概念设定 1 修改",
		Platform:    "起点",
		Volumes:     3,
		AutoLock:    true,
		Status:      domain.QueueStatusRunning,
		ProjectID:   "proj-created-1",
		LastMessage: "正在生成第1卷",
	}
	if err := s.SaveBookQueueItem(ctx, item1Updated); err != nil {
		t.Fatalf("SaveBookQueueItem update failed: %v", err)
	}
	got1After, err := s.GetBookQueueItem(ctx, "queue-item-1")
	if err != nil {
		t.Fatalf("GetBookQueueItem after update failed: %v", err)
	}
	if got1After.Title != "第一部作品(已改名)" || got1After.Status != domain.QueueStatusRunning || got1After.ProjectID != "proj-created-1" {
		t.Errorf("updated fields not matching: %+v", got1After)
	}
	if got1After.CreatedAt != got1.CreatedAt {
		t.Errorf("created_at was changed on update: before=%s, after=%s", got1.CreatedAt, got1After.CreatedAt)
	}

	// 6. Delete item
	if err := s.DeleteBookQueueItem(ctx, "queue-item-2"); err != nil {
		t.Fatalf("DeleteBookQueueItem failed: %v", err)
	}
	_, err = s.GetBookQueueItem(ctx, "queue-item-2")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for deleted item, got %v", err)
	}

	// 7. Delete non-existent returns ErrNotFound
	if err := s.DeleteBookQueueItem(ctx, "ghost-item"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for ghost-item delete, got %v", err)
	}
}

func TestSQLiteStore_ResetStaleQueueRunning(t *testing.T) {
	ctx := context.Background()
	s, _ := setupAutomationTestStore(t)

	item1 := &domain.BookQueueItem{
		ID:      "q-run-1",
		Title:   "运行中1",
		Concept: "概念1",
		Volumes: 1,
		Status:  domain.QueueStatusRunning,
	}
	item2 := &domain.BookQueueItem{
		ID:      "q-pending-2",
		Title:   "待处理2",
		Concept: "概念2",
		Volumes: 1,
		Status:  domain.QueueStatusPending,
	}
	item3 := &domain.BookQueueItem{
		ID:      "q-run-3",
		Title:   "运行中3",
		Concept: "概念3",
		Volumes: 1,
		Status:  domain.QueueStatusRunning,
	}

	_ = s.SaveBookQueueItem(ctx, item1)
	_ = s.SaveBookQueueItem(ctx, item2)
	_ = s.SaveBookQueueItem(ctx, item3)

	resetCount, err := s.ResetStaleQueueRunning(ctx)
	if err != nil {
		t.Fatalf("ResetStaleQueueRunning failed: %v", err)
	}
	if resetCount != 2 {
		t.Errorf("expected 2 running items reset, got %d", resetCount)
	}

	got1, _ := s.GetBookQueueItem(ctx, "q-run-1")
	if got1.Status != domain.QueueStatusPending || got1.LastMessage != "服务重启，运行中的任务已重新排队" {
		t.Errorf("unexpected reset item 1: %+v", got1)
	}

	got2, _ := s.GetBookQueueItem(ctx, "q-pending-2")
	if got2.Status != domain.QueueStatusPending || got2.LastMessage != "" {
		t.Errorf("pending item should not be modified: %+v", got2)
	}
}

func TestSQLiteStore_SchedulerState(t *testing.T) {
	ctx := context.Background()
	s, _ := setupAutomationTestStore(t)

	// Non-existent key returns empty string, nil error
	val, err := s.GetSchedulerState(ctx, "unknown_key")
	if err != nil {
		t.Fatalf("GetSchedulerState for unknown key returned error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}

	// Set and Get
	if err := s.SetSchedulerState(ctx, "last_tick", "2026-10-11T12:00:00Z"); err != nil {
		t.Fatalf("SetSchedulerState failed: %v", err)
	}
	val, err = s.GetSchedulerState(ctx, "last_tick")
	if err != nil {
		t.Fatalf("GetSchedulerState failed: %v", err)
	}
	if val != "2026-10-11T12:00:00Z" {
		t.Errorf("expected '2026-10-11T12:00:00Z', got %q", val)
	}

	// Update existing key
	if err := s.SetSchedulerState(ctx, "last_tick", "2026-10-11T13:00:00Z"); err != nil {
		t.Fatalf("SetSchedulerState update failed: %v", err)
	}
	val, err = s.GetSchedulerState(ctx, "last_tick")
	if err != nil {
		t.Fatalf("GetSchedulerState failed: %v", err)
	}
	if val != "2026-10-11T13:00:00Z" {
		t.Errorf("expected updated '2026-10-11T13:00:00Z', got %q", val)
	}
}
