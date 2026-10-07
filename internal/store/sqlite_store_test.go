package store_test

import (
	"context"
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
	if updatedProj.Protagonist.Inventory != "粗布衣, +洗髓丹x1" {
		t.Errorf("unexpected inventory: %s", updatedProj.Protagonist.Inventory)
	}
	if updatedProj.Protagonist.NameAndLevel != "林凡 (练气一层) (灵力初醒)" {
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
}
