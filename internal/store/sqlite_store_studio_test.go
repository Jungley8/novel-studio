package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupStudioTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_studio.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

func TestSQLiteStore_StudioThread_CreateAndGet(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	th := &domain.StudioThread{
		ID:        "th_test_1",
		Title:     "测试线程标题",
		ProjectID: "proj_123",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.CreateStudioThread(ctx, th); err != nil {
		t.Fatalf("CreateStudioThread failed: %v", err)
	}

	got, err := s.GetStudioThread(ctx, th.ID)
	if err != nil {
		t.Fatalf("GetStudioThread failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil thread")
	}

	if got.ID != th.ID {
		t.Errorf("expected ID %s, got %s", th.ID, got.ID)
	}
	if got.Title != th.Title {
		t.Errorf("expected Title %s, got %s", th.Title, got.Title)
	}
	if got.ProjectID != th.ProjectID {
		t.Errorf("expected ProjectID %s, got %s", th.ProjectID, got.ProjectID)
	}
	if got.MessageCount != 0 {
		t.Errorf("expected MessageCount 0, got %d", got.MessageCount)
	}
	if got.LastMessagePreview != "" {
		t.Errorf("expected empty LastMessagePreview, got %q", got.LastMessagePreview)
	}
}

func TestSQLiteStore_StudioThread_GetNotFound(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	got, err := s.GetStudioThread(ctx, "non_existent_thread")
	if err != nil {
		t.Fatalf("expected nil error for not found, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil thread for not found, got %+v", got)
	}
}

func TestSQLiteStore_StudioThread_DeleteNotFound(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	err := s.DeleteStudioThread(ctx, "non_existent_thread")
	if err == nil {
		t.Fatalf("expected error for non existent thread delete, got nil")
	}
	if !strings.Contains(err.Error(), "studio thread not found: non_existent_thread") {
		t.Fatalf("expected 'studio thread not found: non_existent_thread', got: %v", err)
	}
}

func TestSQLiteStore_StudioThread_AppendAndUpdatedAtPromotion(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	t0 := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	th := &domain.StudioThread{
		ID:        "th_promo",
		Title:     "提升测试",
		CreatedAt: t0,
		UpdatedAt: t0,
	}
	if err := s.CreateStudioThread(ctx, th); err != nil {
		t.Fatalf("CreateStudioThread failed: %v", err)
	}

	// 1. 追加用户消息
	t1 := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	msg1 := &domain.StudioMessage{
		ID:        "msg_1",
		ThreadID:  th.ID,
		Role:      "user",
		Content:   "第一条消息内容",
		CreatedAt: t1,
	}
	if err := s.AppendStudioMessage(ctx, msg1); err != nil {
		t.Fatalf("AppendStudioMessage msg1 failed: %v", err)
	}

	got1, err := s.GetStudioThread(ctx, th.ID)
	if err != nil {
		t.Fatalf("GetStudioThread failed: %v", err)
	}
	if !got1.UpdatedAt.Equal(t1) {
		t.Errorf("expected UpdatedAt promoted to %v, got %v", t1, got1.UpdatedAt)
	}
	if got1.MessageCount != 1 {
		t.Errorf("expected MessageCount 1, got %d", got1.MessageCount)
	}
	if got1.LastMessagePreview != "第一条消息内容" {
		t.Errorf("expected preview %q, got %q", "第一条消息内容", got1.LastMessagePreview)
	}

	// 2. 追加助手消息
	t2 := time.Now().Truncate(time.Second)
	msg2 := &domain.StudioMessage{
		ID:         "msg_2",
		ThreadID:   th.ID,
		Role:       "assistant",
		Content:    "助手回复内容",
		TokenUsage: `{"total_tokens":42}`,
		CreatedAt:  t2,
	}
	if err := s.AppendStudioMessage(ctx, msg2); err != nil {
		t.Fatalf("AppendStudioMessage msg2 failed: %v", err)
	}

	got2, err := s.GetStudioThread(ctx, th.ID)
	if err != nil {
		t.Fatalf("GetStudioThread failed: %v", err)
	}
	if !got2.UpdatedAt.Equal(t2) {
		t.Errorf("expected UpdatedAt promoted to %v, got %v", t2, got2.UpdatedAt)
	}
	if got2.MessageCount != 2 {
		t.Errorf("expected MessageCount 2, got %d", got2.MessageCount)
	}
	if got2.LastMessagePreview != "助手回复内容" {
		t.Errorf("expected preview %q, got %q", "助手回复内容", got2.LastMessagePreview)
	}

	// 3. 追加消息到不存在的线程应报错
	msgOrphan := &domain.StudioMessage{
		ID:       "msg_orphan",
		ThreadID: "non_existent_th",
		Role:     "user",
		Content:  "孤儿消息",
	}
	if err := s.AppendStudioMessage(ctx, msgOrphan); err == nil {
		t.Errorf("expected error when appending to non-existent thread, got nil")
	}
}

func TestSQLiteStore_StudioThread_ListAndPreviewTruncation(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// 线程 1：短消息
	th1 := &domain.StudioThread{
		ID:        "th_short",
		Title:     "短消息线程",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	}
	if err := s.CreateStudioThread(ctx, th1); err != nil {
		t.Fatalf("CreateStudioThread th1 failed: %v", err)
	}
	_ = s.AppendStudioMessage(ctx, &domain.StudioMessage{
		ID:        "msg_s1",
		ThreadID:  th1.ID,
		Role:      "user",
		Content:   "短内容",
		CreatedAt: now.Add(-8 * time.Minute),
	})

	// 线程 2：超过 50 个字符的长消息
	th2 := &domain.StudioThread{
		ID:        "th_long",
		Title:     "长消息线程",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now.Add(-5 * time.Minute),
	}
	if err := s.CreateStudioThread(ctx, th2); err != nil {
		t.Fatalf("CreateStudioThread th2 failed: %v", err)
	}

	longContent := "这是一个测试用的超长消息内容，用来严格验证当消息文本超过五十个字符时，系统是否会精准地截断到前五十个字符并追加省略号标记。"
	runes := []rune(longContent)
	expectedPreview := string(runes[:50]) + "…"

	_ = s.AppendStudioMessage(ctx, &domain.StudioMessage{
		ID:        "msg_l1",
		ThreadID:  th2.ID,
		Role:      "assistant",
		Content:   longContent,
		CreatedAt: now.Add(-2 * time.Minute),
	})

	threads, err := s.ListStudioThreads(ctx)
	if err != nil {
		t.Fatalf("ListStudioThreads failed: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}

	// 按 updated_at 降序：th2 更新时间比 th1 晚，排在第 0 位
	if threads[0].ID != th2.ID {
		t.Errorf("expected first thread to be %s, got %s", th2.ID, threads[0].ID)
	}
	if threads[0].MessageCount != 1 {
		t.Errorf("expected th2 message_count 1, got %d", threads[0].MessageCount)
	}
	if threads[0].LastMessagePreview != expectedPreview {
		t.Errorf("expected th2 preview %q, got %q", expectedPreview, threads[0].LastMessagePreview)
	}

	if threads[1].ID != th1.ID {
		t.Errorf("expected second thread to be %s, got %s", th1.ID, threads[1].ID)
	}
	if threads[1].MessageCount != 1 {
		t.Errorf("expected th1 message_count 1, got %d", threads[1].MessageCount)
	}
	if threads[1].LastMessagePreview != "短内容" {
		t.Errorf("expected th1 preview '短内容', got %q", threads[1].LastMessagePreview)
	}
}

func TestSQLiteStore_StudioThread_DeleteCascade(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	th := &domain.StudioThread{
		ID:        "th_del",
		Title:     "待删线程",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateStudioThread(ctx, th); err != nil {
		t.Fatalf("CreateStudioThread failed: %v", err)
	}

	// 插入 2 条消息
	for i := 1; i <= 2; i++ {
		_ = s.AppendStudioMessage(ctx, &domain.StudioMessage{
			ID:        string(rune('a' + i)),
			ThreadID:  th.ID,
			Role:      "user",
			Content:   "消息内容",
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}

	msgs, err := s.ListStudioMessages(ctx, th.ID)
	if err != nil {
		t.Fatalf("ListStudioMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages before delete, got %d", len(msgs))
	}

	// 级联删除
	if err := s.DeleteStudioThread(ctx, th.ID); err != nil {
		t.Fatalf("DeleteStudioThread failed: %v", err)
	}

	// 验证线程已不存在
	gotTh, err := s.GetStudioThread(ctx, th.ID)
	if err != nil {
		t.Fatalf("GetStudioThread failed: %v", err)
	}
	if gotTh != nil {
		t.Fatalf("expected nil thread after delete, got %+v", gotTh)
	}

	// 验证消息已级联清零
	msgsAfter, err := s.ListStudioMessages(ctx, th.ID)
	if err != nil {
		t.Fatalf("ListStudioMessages after delete failed: %v", err)
	}
	if len(msgsAfter) != 0 {
		t.Fatalf("expected 0 messages after cascade delete, got %d", len(msgsAfter))
	}

	// 再次删除应返回明确错误
	if err := s.DeleteStudioThread(ctx, th.ID); err == nil {
		t.Fatalf("expected error on deleting non-existent thread, got nil")
	}
}

func TestSQLiteStore_StudioMessages_Ordering(t *testing.T) {
	s := setupStudioTestStore(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	th := &domain.StudioThread{
		ID:        "th_order",
		Title:     "消息顺序测试",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateStudioThread(ctx, th); err != nil {
		t.Fatalf("CreateStudioThread failed: %v", err)
	}

	m1 := &domain.StudioMessage{
		ID:        "m1",
		ThreadID:  th.ID,
		Role:      "user",
		Content:   "第一句",
		CreatedAt: now.Add(1 * time.Second),
	}
	m2 := &domain.StudioMessage{
		ID:        "m2",
		ThreadID:  th.ID,
		Role:      "assistant",
		Content:   "第二句",
		CreatedAt: now.Add(2 * time.Second),
	}
	m3 := &domain.StudioMessage{
		ID:        "m3",
		ThreadID:  th.ID,
		Role:      "user",
		Content:   "第三句",
		CreatedAt: now.Add(3 * time.Second),
	}

	_ = s.AppendStudioMessage(ctx, m1)
	_ = s.AppendStudioMessage(ctx, m2)
	_ = s.AppendStudioMessage(ctx, m3)

	msgs, err := s.ListStudioMessages(ctx, th.ID)
	if err != nil {
		t.Fatalf("ListStudioMessages failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].ID != "m1" || msgs[1].ID != "m2" || msgs[2].ID != "m3" {
		t.Errorf("messages not in created_at ASC order: [%s, %s, %s]", msgs[0].ID, msgs[1].ID, msgs[2].ID)
	}
}
