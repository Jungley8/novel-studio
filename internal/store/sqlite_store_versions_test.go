package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func TestSQLiteStore_ChapterVersionHistory(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "versions_test.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	p := &domain.Project{
		ID:    "proj-ver",
		Title: "版本历史测试",
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. CommitChapter（第 1 章，含正文）→ 列表恰 1 条、Source=="commit"、WordCount 正确
	contentV1 := "第一版正文内容测试。"
	ch := &domain.Chapter{
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      contentV1,
	}
	if _, err := s.CommitChapter(ctx, p.ID, ch); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}
	list1, err := s.ListChapterVersions(ctx, p.ID, 1)
	if err != nil {
		t.Fatalf("ListChapterVersions failed: %v", err)
	}
	if len(list1) != 1 {
		t.Fatalf("expected 1 version after commit, got %d", len(list1))
	}
	if list1[0].Source != "commit" {
		t.Errorf("expected source 'commit', got '%s'", list1[0].Source)
	}
	if list1[0].WordCount != len([]rune(contentV1)) {
		t.Errorf("expected word_count %d, got %d", len([]rune(contentV1)), list1[0].WordCount)
	}

	// 2. SaveChapter 修改正文（"edit" 路径）→ 列表 2 条且新的在前（DESC）
	time.Sleep(10 * time.Millisecond)
	contentV2 := "第二版修改后的正文内容完全不同。"
	edit := &domain.Chapter{
		ID:           ch.ID,
		ProjectID:    p.ID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      contentV2,
	}
	if err := s.SaveChapter(ctx, edit); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}
	list2, err := s.ListChapterVersions(ctx, p.ID, 1)
	if err != nil {
		t.Fatalf("ListChapterVersions after edit failed: %v", err)
	}
	if len(list2) != 2 {
		t.Fatalf("expected 2 versions after edit, got %d", len(list2))
	}
	if list2[0].Source != "edit" {
		t.Errorf("expected newest version source 'edit', got '%s'", list2[0].Source)
	}
	if !list2[0].CreatedAt.After(list2[1].CreatedAt) && !list2[0].CreatedAt.Equal(list2[1].CreatedAt) {
		t.Errorf("expected newest version first (DESC), got %v then %v", list2[0].CreatedAt, list2[1].CreatedAt)
	}

	// 3. 同内容再次 SaveChapter → 仍 2 条（hash 去重）
	if err := s.SaveChapter(ctx, edit); err != nil {
		t.Fatalf("SaveChapter identical failed: %v", err)
	}
	list3, err := s.ListChapterVersions(ctx, p.ID, 1)
	if err != nil {
		t.Fatalf("ListChapterVersions after identical save failed: %v", err)
	}
	if len(list3) != 2 {
		t.Errorf("expected dedup to keep 2 versions, got %d", len(list3))
	}

	// 4. GetChapterVersion(projectID, 新版 id) 正文与保存一致
	got, err := s.GetChapterVersion(ctx, p.ID, list2[0].ID)
	if err != nil {
		t.Fatalf("GetChapterVersion failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected version %s, got nil", list2[0].ID)
	}
	if got.Content != contentV2 {
		t.Errorf("expected content %q, got %q", contentV2, got.Content)
	}

	// 5. 租户隔离：GetChapterVersion(另一 projectID, 同 id) → (nil, nil)
	other := &domain.Project{ID: "proj-ver-other", Title: "另一作品"}
	if err := s.SaveProject(ctx, other); err != nil {
		t.Fatalf("SaveProject other failed: %v", err)
	}
	cross, err := s.GetChapterVersion(ctx, other.ID, list2[0].ID)
	if err != nil {
		t.Fatalf("expected nil error for cross-project read, got: %v", err)
	}
	if cross != nil {
		t.Errorf("expected nil for cross-project version read, got: %+v", cross)
	}

	// 6. 不存在的 id → (nil, nil)
	missing, err := s.GetChapterVersion(ctx, p.ID, "ver_not_exist_xyz")
	if err != nil {
		t.Fatalf("expected nil error for missing version, got: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing version id, got: %+v", missing)
	}
}
