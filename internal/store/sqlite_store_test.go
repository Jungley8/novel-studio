package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func TestSQLiteStore_CRUD(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	// 1. Create project
	p := &domain.Project{
		ID:             "proj-001",
		Title:          "极道修仙传",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "筑基期不能跨界传信",
		Protagonist: domain.Protagonist{
			NameAndLevel: "叶天 (练气九层)",
			Inventory:    "生锈铁剑x1, 回春丹x3",
			CoreGoal:     "在宗门小比中战胜外门大师兄",
		},
	}

	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 2. Read project
	gotP, err := s.GetProject(ctx, "proj-001")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if gotP.Title != "极道修仙传" {
		t.Errorf("expected Title 极道修仙传, got %s", gotP.Title)
	}
	if gotP.Protagonist.NameAndLevel != "叶天 (练气九层)" {
		t.Errorf("expected Protagonist name 叶天 (练气九层), got %s", gotP.Protagonist.NameAndLevel)
	}

	// 3. Save chapters
	ch1 := &domain.Chapter{
		ID:           "ch-001",
		ProjectID:    "proj-001",
		ChapterIndex: 1,
		Title:        "外门受辱",
		CoreConflict: "大师兄抢夺淬体丹",
		Beats: []domain.SceneBeat{
			{Phase: "蓄力压迫", Tension: 4, Action: "大师兄当众踩踏叶天的丹药", ExpectationBroken: "主角没有暴怒，暗中隐忍"},
			{Phase: "绝地反转", Tension: 8, Action: "主角触发随身玉佩残魂", ExpectationBroken: "残魂点破大师兄功法破绽"},
		},
		StateMutation: domain.StateMutation{
			InventoryDelta: "-淬体丹x1, +残魂指点",
			PowerDelta:     "心境突破",
		},
		Content:         "破旧的青石广场上，寒风凛冽。大师兄一脚将地上的丹瓶踏得粉碎...",
		BurstinessScore: 65,
		LinterPassed:    true,
	}

	if err := s.SaveChapter(ctx, ch1); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}

	gotCh1, err := s.GetChapter(ctx, "proj-001", 1)
	if err != nil {
		t.Fatalf("GetChapter failed: %v", err)
	}
	if len(gotCh1.Beats) != 2 {
		t.Errorf("expected 2 beats, got %d", len(gotCh1.Beats))
	}
	if gotCh1.WordCount == 0 {
		t.Errorf("expected non-zero word count")
	}

	// 4. Plot Hooks
	hook := &domain.PlotHook{
		ID:             "hook-001",
		ProjectID:      "proj-001",
		Title:          "神秘残魂身份",
		Details:        "玉佩中的老者似乎认识前代掌门",
		CreatedChapter: 1,
		TargetChapter:  10,
		Status:         domain.HookStatusOpen,
		CreatedAt:      time.Now(),
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	hooks, err := s.ListPlotHooks(ctx, "proj-001")
	if err != nil {
		t.Fatalf("ListPlotHooks failed: %v", err)
	}
	if len(hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(hooks))
	}
}

func TestSQLiteStore_CommitChapterAtomic(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "commit_test.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	p := &domain.Project{
		ID:             "proj-atomic",
		Title:          "修仙传",
		TargetPlatform: "番茄",
		Protagonist: domain.Protagonist{
			NameAndLevel: "林凡 (练气一层)",
			Inventory:    "粗布衣",
		},
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Add an open hook targeting chapter 2
	hook := &domain.PlotHook{
		ID:             "hook-atomic-1",
		ProjectID:      "proj-atomic",
		Title:          "药铺怪老头",
		CreatedChapter: 1,
		TargetChapter:  2,
		Status:         domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// Commit chapter 2 with state mutations
	ch2 := &domain.Chapter{
		ChapterIndex: 2,
		Title:        "第二章 偶遇奇遇",
		Content:      "林凡走入药铺，老头神秘一笑，塞给他一颗洗髓丹。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "+洗髓丹x1",
			PowerDelta:     "灵力初醒",
		},
	}

	updatedProj, err := s.CommitChapter(ctx, "proj-atomic", ch2)
	if err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// 1. Verify updated protagonist
	if updatedProj.Protagonist.Inventory != "粗布衣, 洗髓丹x1" {
		t.Errorf("unexpected inventory: %s", updatedProj.Protagonist.Inventory)
	}
	if updatedProj.Protagonist.NameAndLevel != "林凡 (灵力初醒)" {
		t.Errorf("unexpected name and level: %s", updatedProj.Protagonist.NameAndLevel)
	}

	// 2. Verify chapter was persisted
	savedCh, err := s.GetChapter(ctx, "proj-atomic", 2)
	if err != nil {
		t.Fatalf("GetChapter failed: %v", err)
	}
	if savedCh.Title != "第二章 偶遇奇遇" {
		t.Errorf("expected chapter title 第二章 偶遇奇遇, got %s", savedCh.Title)
	}

	// 3. Verify hook status was updated to FERMENTING since target_chapter <= 2
	hooks, err := s.ListPlotHooks(ctx, "proj-atomic")
	if err != nil {
		t.Fatalf("ListPlotHooks failed: %v", err)
	}
	if len(hooks) != 1 || hooks[0].Status != domain.HookStatusFermenting {
		t.Errorf("expected hook status FERMENTING, got %v", hooks[0].Status)
	}

	// 4. Test Hook Resolution via Review.ResolvedHookIDs in Chapter 3
	ch3 := &domain.Chapter{
		ChapterIndex: 3,
		Title:        "第三章 身世揭晓",
		Content:      "老头揭下面具，正是前朝掌门...",
		Review: &domain.ReviewResult{
			Verdict:         domain.ReviewVerdictAccepted,
			Score:           90,
			ResolvedHookIDs: []string{"hook-atomic-1"},
		},
	}
	_, err = s.CommitChapter(ctx, "proj-atomic", ch3)
	if err != nil {
		t.Fatalf("CommitChapter ch3 failed: %v", err)
	}
	hooksAfter, err := s.ListPlotHooks(ctx, "proj-atomic")
	if err != nil {
		t.Fatalf("ListPlotHooks after ch3 failed: %v", err)
	}
	if hooksAfter[0].Status != domain.HookStatusResolved {
		t.Errorf("expected hook status RESOLVED, got %v", hooksAfter[0].Status)
	}
}

func TestSQLiteStore_Checkpoint(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "checkpoint_test.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	p := &domain.Project{
		ID:             "proj-cp",
		Title:          "断点测试",
		TargetPlatform: "通用",
	}
	_ = s.SaveProject(ctx, p)

	// Save checkpoint
	cp := &domain.ChapterCheckpoint{
		ProjectID:    "proj-cp",
		ChapterIndex: 1,
		Phase:        domain.CheckpointPhaseDrafted,
		CoreConflict: "主角被围困",
		Beats: []domain.SceneBeat{
			{Phase: "蓄力压迫", Tension: 6, Action: "敌人封锁退路"},
		},
		DraftText: "四面楚歌，杀声震天。",
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Retrieve checkpoint
	got, err := s.GetCheckpoint(ctx, "proj-cp", 1)
	if err != nil {
		t.Fatalf("GetCheckpoint failed: %v", err)
	}
	if got == nil || got.DraftText != "四面楚歌，杀声震天。" {
		t.Fatalf("unexpected checkpoint data: %v", got)
	}
	if got.Phase != domain.CheckpointPhaseDrafted {
		t.Errorf("expected phase DRAFTED, got %s", got.Phase)
	}

	// Clear checkpoint
	if err := s.ClearCheckpoint(ctx, "proj-cp", 1); err != nil {
		t.Fatalf("ClearCheckpoint failed: %v", err)
	}
	cleared, err := s.GetCheckpoint(ctx, "proj-cp", 1)
	if err != nil {
		t.Fatalf("GetCheckpoint after clear failed: %v", err)
	}
	if cleared != nil {
		t.Errorf("expected nil checkpoint after clear, got %v", cleared)
	}
}

func TestSQLiteStore_NullFrameworkJsonCompatibility(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_null_fw.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	p := &domain.Project{
		ID:             "proj-legacy",
		Title:          "万古旧传",
		TargetPlatform: "起点仙侠",
		WorldRules:     "天地不仁",
		Protagonist: domain.Protagonist{
			NameAndLevel: "古修士 (金丹期)",
		},
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Simulate existing DB where framework_json is NULL
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db failed: %v", err)
	}
	if _, err := rawDB.Exec("UPDATE projects SET framework_json = NULL WHERE id = 'proj-legacy';"); err != nil {
		rawDB.Close()
		t.Fatalf("raw exec failed: %v", err)
	}
	rawDB.Close()

	// Must succeed without "converting NULL to string" error
	got, err := s.GetProject(ctx, "proj-legacy")
	if err != nil {
		t.Fatalf("GetProject with NULL framework_json failed: %v", err)
	}
	if got.Framework != nil {
		t.Errorf("expected nil framework, got %v", got.Framework)
	}

	list, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects with NULL framework_json failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 project in list, got %d", len(list))
	}
}

