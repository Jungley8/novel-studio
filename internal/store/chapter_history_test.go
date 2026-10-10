package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// Case ①: 连续 commit 3 章 → 3 条版本行（source="commit"）+ 3 条快照行，快照 protagonist_json 反序列化后道具数量随章节累加正确
func TestChapterHistory_Case1_ContinuousCommitAndSnapshot(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_case1.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case1",
		Title: "连续归档测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	if err := s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-case1_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "韩立",
		Realm:     "炼气一层",
		Inventory: "",
	}); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	if err := s.SavePlotHook(ctx, &domain.PlotHook{
		ID:             "hook_case1_1",
		ProjectID:      proj.ID,
		Title:          "神秘小瓶",
		Details:        "绿液催熟灵药",
		CreatedChapter: 1,
		TargetChapter:  5,
		Status:         domain.HookStatusOpen,
	}); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// Commit 3 chapters sequentially
	ch1 := &domain.Chapter{
		ID:           "ch_c1_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 仙缘",
		Content:      "青山苍茫，少年韩立背着药篓采药...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 青钢剑 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	ch2 := &domain.Chapter{
		ID:           "ch_c1_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 夺宝",
		Content:      "剑光霍霍，两道身影在峡谷交错，韩立夺得重剑...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 玄铁重剑 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	ch3 := &domain.Chapter{
		ID:           "ch_c1_3",
		ProjectID:    proj.ID,
		ChapterIndex: 3,
		Title:        "第三章 斩妖",
		Content:      "雷鸣电闪，韩立斩杀蛟龙获得诛仙剑...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 诛仙剑 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch3); err != nil {
		t.Fatalf("CommitChapter 3 failed: %v", err)
	}

	// Verify versions: each chapter has 1 version row with source="commit"
	for idx := 1; idx <= 3; idx++ {
		vers, err := s.ListChapterVersions(ctx, proj.ID, idx)
		if err != nil {
			t.Fatalf("ListChapterVersions(%d) failed: %v", idx, err)
		}
		if len(vers) != 1 {
			t.Fatalf("expected 1 version for chapter %d, got %d", idx, len(vers))
		}
		if vers[0].Source != "commit" {
			t.Fatalf("expected source 'commit' for chapter %d, got '%s'", idx, vers[0].Source)
		}
	}

	// Verify snapshots: 3 snapshot rows exist
	snap1, err := s.GetChapterStateSnapshot(ctx, proj.ID, 1)
	if err != nil || snap1 == nil {
		t.Fatalf("GetChapterStateSnapshot(1) failed: snap=%v, err=%v", snap1, err)
	}
	snap2, err := s.GetChapterStateSnapshot(ctx, proj.ID, 2)
	if err != nil || snap2 == nil {
		t.Fatalf("GetChapterStateSnapshot(2) failed: snap=%v, err=%v", snap2, err)
	}
	snap3, err := s.GetChapterStateSnapshot(ctx, proj.ID, 3)
	if err != nil || snap3 == nil {
		t.Fatalf("GetChapterStateSnapshot(3) failed: snap=%v, err=%v", snap3, err)
	}

	// Unmarshal protagonist JSON and verify accumulated items
	var p1, p2, p3 domain.CodexEntry
	if err := json.Unmarshal([]byte(snap1.ProtagonistJSON), &p1); err != nil {
		t.Fatalf("unmarshal snap1 protagonist failed: %v", err)
	}
	if err := json.Unmarshal([]byte(snap2.ProtagonistJSON), &p2); err != nil {
		t.Fatalf("unmarshal snap2 protagonist failed: %v", err)
	}
	if err := json.Unmarshal([]byte(snap3.ProtagonistJSON), &p3); err != nil {
		t.Fatalf("unmarshal snap3 protagonist failed: %v", err)
	}

	if len(p1.StructuredItems) != 1 || !strings.Contains(p1.Inventory, "青钢剑") {
		t.Fatalf("snap1 expected 1 item (青钢剑), got count=%d, inventory=%s", len(p1.StructuredItems), p1.Inventory)
	}
	if len(p2.StructuredItems) != 2 || !strings.Contains(p2.Inventory, "青钢剑") || !strings.Contains(p2.Inventory, "玄铁重剑") {
		t.Fatalf("snap2 expected 2 items, got count=%d, inventory=%s", len(p2.StructuredItems), p2.Inventory)
	}
	if len(p3.StructuredItems) != 3 || !strings.Contains(p3.Inventory, "诛仙剑") {
		t.Fatalf("snap3 expected 3 items, got count=%d, inventory=%s", len(p3.StructuredItems), p3.Inventory)
	}

	// Check hooks in snapshot
	var hooks1 []*domain.PlotHook
	if err := json.Unmarshal([]byte(snap1.HooksJSON), &hooks1); err != nil {
		t.Fatalf("unmarshal snap1 hooks failed: %v", err)
	}
	if len(hooks1) != 1 || hooks1[0].Title != "神秘小瓶" {
		t.Fatalf("expected 1 hook in snapshot, got %+v", hooks1)
	}
}

// Case ②: 第 2 章同内容重提交 → 版本行不新增（去重）；改内容重提交 → 新增 1 条（source="commit"），快照被覆盖为最新态
func TestChapterHistory_Case2_DeduplicationAndOverwrite(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_case2.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case2",
		Title: "重提交与去重测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	if err := s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-case2_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "韩立",
		Realm:     "炼气一层",
		Inventory: "粗布衣",
	}); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	ch2 := &domain.Chapter{
		ID:           "ch_c2_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 初始版本",
		Content:      "这是第二章初始的正文内容，保持不变以测试去重逻辑。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 灵石 (+10)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("first CommitChapter failed: %v", err)
	}

	vers1, err := s.ListChapterVersions(ctx, proj.ID, 2)
	if err != nil {
		t.Fatalf("ListChapterVersions failed: %v", err)
	}
	if len(vers1) != 1 {
		t.Fatalf("expected 1 version after initial commit, got %d", len(vers1))
	}

	// Re-commit with IDENTICAL content -> must deduplicate (no new version row)
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("resubmit with identical content failed: %v", err)
	}

	vers2, err := s.ListChapterVersions(ctx, proj.ID, 2)
	if err != nil {
		t.Fatalf("ListChapterVersions after identical resubmit failed: %v", err)
	}
	if len(vers2) != 1 {
		t.Fatalf("expected deduplication to keep 1 version, but got %d", len(vers2))
	}

	// Re-commit with MODIFIED content -> must add 1 new version row (source="commit")
	ch2Modified := *ch2
	ch2Modified.Content = "这是第二章大幅修改后的新正文内容，哈希值已发生改变！"
	ch2Modified.StateMutation = domain.StateMutation{
		InventoryDelta: "获得物品: 龙鳞甲 (+1)",
	}
	if _, err := s.CommitChapter(ctx, proj.ID, &ch2Modified); err != nil {
		t.Fatalf("resubmit with modified content failed: %v", err)
	}

	vers3, err := s.ListChapterVersions(ctx, proj.ID, 2)
	if err != nil {
		t.Fatalf("ListChapterVersions after modified resubmit failed: %v", err)
	}
	if len(vers3) != 2 {
		t.Fatalf("expected 2 versions after modified resubmit, got %d", len(vers3))
	}
	if vers3[0].Source != "commit" {
		t.Fatalf("expected latest version source='commit', got '%s'", vers3[0].Source)
	}

	// Verify snapshot was overwritten with the latest state
	snap2, err := s.GetChapterStateSnapshot(ctx, proj.ID, 2)
	if err != nil || snap2 == nil {
		t.Fatalf("GetChapterStateSnapshot(2) failed: snap=%v, err=%v", snap2, err)
	}
	var p domain.CodexEntry
	if err := json.Unmarshal([]byte(snap2.ProtagonistJSON), &p); err != nil {
		t.Fatalf("unmarshal snap2 protagonist failed: %v", err)
	}
	if !strings.Contains(p.Inventory, "龙鳞甲") {
		t.Fatalf("expected snapshot to be updated with 龙鳞甲, got: %s", p.Inventory)
	}
}

// Case ③: SaveChapter 改第 1 章正文 → 新增 1 条 source="edit" 版本行
func TestChapterHistory_Case3_SaveChapterEditVersion(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_case3.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case3",
		Title: "SaveChapter手工编辑版本测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_c3_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "第一章原始正文内容。",
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	versInitial, err := s.ListChapterVersions(ctx, proj.ID, 1)
	if err != nil || len(versInitial) != 1 {
		t.Fatalf("expected 1 initial version, got %d, err: %v", len(versInitial), err)
	}
	if versInitial[0].Source != "commit" {
		t.Fatalf("expected source='commit', got: %s", versInitial[0].Source)
	}

	// Edit chapter prose via SaveChapter
	ch1.Content = "第一章正文被作者手工细致修改了文字。"
	if err := s.SaveChapter(ctx, ch1); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}

	versAfterEdit, err := s.ListChapterVersions(ctx, proj.ID, 1)
	if err != nil {
		t.Fatalf("ListChapterVersions failed: %v", err)
	}
	if len(versAfterEdit) != 2 {
		t.Fatalf("expected 2 versions after edit, got %d", len(versAfterEdit))
	}
	if versAfterEdit[0].Source != "edit" {
		t.Fatalf("expected latest version source='edit', got '%s'", versAfterEdit[0].Source)
	}
	if versAfterEdit[1].Source != "commit" {
		t.Fatalf("expected older version source='commit', got '%s'", versAfterEdit[1].Source)
	}
}

// Case ④: ListChapterVersions 排序与字段；GetChapterVersion 带正文；不存在 ID 返回 nil, nil
func TestChapterHistory_Case4_ReadAPIsSortingAndFields(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_case4.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case4",
		Title: "读取API测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_c4_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "第一版初始正文内容。",
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	ch1.Content = "第二版作者修改后的内容。"
	if err := s.SaveChapter(ctx, ch1); err != nil {
		t.Fatalf("SaveChapter edit 1 failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	ch1.Content = "第三版最终定稿正文。"
	if err := s.SaveChapter(ctx, ch1); err != nil {
		t.Fatalf("SaveChapter edit 2 failed: %v", err)
	}

	vers, err := s.ListChapterVersions(ctx, proj.ID, 1)
	if err != nil {
		t.Fatalf("ListChapterVersions failed: %v", err)
	}
	if len(vers) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(vers))
	}

	// Verify descending order by created_at
	if !vers[0].CreatedAt.After(vers[1].CreatedAt) && !vers[0].CreatedAt.Equal(vers[1].CreatedAt) {
		t.Fatalf("expected vers[0] to be newer than or equal to vers[1]")
	}
	if !vers[1].CreatedAt.After(vers[2].CreatedAt) && !vers[1].CreatedAt.Equal(vers[2].CreatedAt) {
		t.Fatalf("expected vers[1] to be newer than or equal to vers[2]")
	}

	// Verify fields of metadata
	for _, m := range vers {
		if m.ID == "" {
			t.Fatalf("meta ID cannot be empty")
		}
		if m.ProjectID != proj.ID {
			t.Fatalf("expected project ID '%s', got '%s'", proj.ID, m.ProjectID)
		}
		if m.ChapterIndex != 1 {
			t.Fatalf("expected chapter index 1, got %d", m.ChapterIndex)
		}
		if m.ContentHash == "" {
			t.Fatalf("expected content_hash to be populated")
		}
		if m.WordCount <= 0 {
			t.Fatalf("expected word_count > 0, got %d", m.WordCount)
		}
		if m.CreatedAt.IsZero() {
			t.Fatalf("expected created_at to be valid")
		}
	}

	// GetChapterVersion with content
	vLatest, err := s.GetChapterVersion(ctx, vers[0].ID)
	if err != nil || vLatest == nil {
		t.Fatalf("GetChapterVersion(%s) failed: v=%v, err=%v", vers[0].ID, vLatest, err)
	}
	if vLatest.Content != "第三版最终定稿正文。" {
		t.Fatalf("expected content '第三版最终定稿正文。', got '%s'", vLatest.Content)
	}
	if vLatest.ID != vers[0].ID || vLatest.ContentHash != vers[0].ContentHash {
		t.Fatalf("meta fields mismatch on ChapterVersion struct")
	}

	// Non-existent ID returns nil, nil
	vMissing, err := s.GetChapterVersion(ctx, "non_existent_id_xyz")
	if err != nil {
		t.Fatalf("expected nil error for missing version, got: %v", err)
	}
	if vMissing != nil {
		t.Fatalf("expected nil version for missing ID, got: %+v", vMissing)
	}
}

// Case ⑤: GetChapterStateSnapshot 不存在返回 nil, nil
func TestChapterHistory_Case5_GetNonExistentSnapshot(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_case5.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	snap, err := s.GetChapterStateSnapshot(ctx, "proj_not_exist", 999)
	if err != nil {
		t.Fatalf("expected nil error for non-existent snapshot, got: %v", err)
	}
	if snap != nil {
		t.Fatalf("expected nil snapshot for non-existent chapter, got: %+v", snap)
	}
}

// Case ⑥: migrate 幂等：对同一 DB 调两次 NewSQLiteStore（第二次打开已有库）不报错
func TestChapterHistory_Case6_MigrateIdempotency(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_case6.db")

	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("first NewSQLiteStore failed: %v", err)
	}

	proj := &domain.Project{
		ID:    "proj-case6",
		Title: "迁移幂等性测试",
	}
	if err := s1.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	ch := &domain.Chapter{
		ID:           "ch_c6_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "幂等性验证正文。",
	}
	if _, err := s1.CommitChapter(ctx, proj.ID, ch); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}
	_ = s1.Close()

	// Reopen the existing database
	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("second NewSQLiteStore on existing DB failed: %v", err)
	}
	defer s2.Close()

	// Verify data is intact
	vers, err := s2.ListChapterVersions(ctx, proj.ID, 1)
	if err != nil {
		t.Fatalf("ListChapterVersions after reopen failed: %v", err)
	}
	if len(vers) != 1 {
		t.Fatalf("expected 1 version after reopen, got %d", len(vers))
	}

	snap, err := s2.GetChapterStateSnapshot(ctx, proj.ID, 1)
	if err != nil || snap == nil {
		t.Fatalf("GetChapterStateSnapshot after reopen failed: snap=%v, err=%v", snap, err)
	}
}
