package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// Case ①: 连续归档 3 章后 uncommit 第 2 章 → 被拒绝且第 3 章状态纹丝不动
func TestSQLiteStore_UncommitChapter_NonLatestRejected(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_uncommit_nonlatest.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case1",
		Title: "非末章撤回守卫测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	_ = s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-case1_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "韩立",
		Realm:     "炼气一层",
		Inventory: "粗布衣, 桃木剑",
	})

	// Commit Chapter 1
	ch1 := &domain.Chapter{
		ID:           "ch_c1_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 初入修仙",
		CoreConflict: "山村采药遇仙缘",
		Content:      "青山苍茫，少年韩立背着药篓...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 青钢剑 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	// Commit Chapter 2
	ch2 := &domain.Chapter{
		ID:           "ch_c1_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 夺宝之争",
		CoreConflict: "峡谷夺取灵草",
		Content:      "剑光霍霍，两道身影在峡谷交错...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 玄铁重剑 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// Commit Chapter 3
	ch3 := &domain.Chapter{
		ID:           "ch_c1_3",
		ProjectID:    proj.ID,
		ChapterIndex: 3,
		Title:        "第三章 斩妖立威",
		CoreConflict: "深潭斩杀蛟龙",
		Content:      "雷鸣电闪，深潭掀起滔天巨浪，韩立一剑斩出...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 诛仙剑 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch3); err != nil {
		t.Fatalf("CommitChapter 3 failed: %v", err)
	}

	// Verify protagonist state before uncommit
	protBefore, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry failed: %v", err)
	}
	if !strings.Contains(protBefore.Inventory, "诛仙剑") || !strings.Contains(protBefore.Inventory, "玄铁重剑") {
		t.Fatalf("expected protagonist to have 诛仙剑 and 玄铁重剑, got: %s", protBefore.Inventory)
	}

	// Attempt to uncommit Chapter 2 (non-latest, chapter 3 exists)
	cp, p, err := s.UncommitChapter(ctx, proj.ID, 2)
	if err == nil {
		t.Fatalf("expected UncommitChapter(2) to fail when chapter 3 exists, but got success: cp=%v, p=%v", cp, p)
	}

	// Check error contract
	if !errors.Is(err, ErrNonLatestChapter) {
		t.Errorf("expected errors.Is(err, ErrNonLatestChapter) to be true, got err: %v", err)
	}
	var nlcErr *NonLatestChapterError
	if !errors.As(err, &nlcErr) {
		t.Errorf("expected errors.As to populate *NonLatestChapterError, got err: %v", err)
	} else {
		if nlcErr.ChapterIndex != 2 || nlcErr.MaxChapter != 3 || len(nlcErr.Downstream) != 1 || nlcErr.Downstream[0] != 3 {
			t.Errorf("unexpected NonLatestChapterError fields: %+v", nlcErr)
		}
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "cannot uncommit chapter 2") || !strings.Contains(errMsg, "uncommit chapter 3 first") {
		t.Errorf("expected error message to guide uncommitting chapter 3 first, got: %s", errMsg)
	}

	// Chapter 3 must be completely intact
	savedCh3, err := s.GetChapter(ctx, proj.ID, 3)
	if err != nil || savedCh3 == nil {
		t.Fatalf("expected chapter 3 to exist untouched, err: %v", err)
	}
	if savedCh3.Content != ch3.Content {
		t.Errorf("expected chapter 3 content to match original")
	}

	// Chapter 2 must also remain committed
	savedCh2, err := s.GetChapter(ctx, proj.ID, 2)
	if err != nil || savedCh2 == nil {
		t.Fatalf("expected chapter 2 to exist untouched, err: %v", err)
	}

	// Protagonist state must be untouched
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry failed: %v", err)
	}
	if protAfter.Inventory != protBefore.Inventory {
		t.Errorf("protagonist inventory mutated: before=%s, after=%s", protBefore.Inventory, protAfter.Inventory)
	}

	// All 3 chapters must remain
	chs, err := s.ListChapters(ctx, proj.ID)
	if err != nil || len(chs) != 3 {
		t.Fatalf("expected 3 chapters to remain, got %d", len(chs))
	}
}

// Case ②: uncommit 末章 → 主角状态正确回退、checkpoint 已生成、hook 还原
func TestSQLiteStore_UncommitChapter_LatestSuccess(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_uncommit_latest.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case2",
		Title: "末章撤回成功测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	_ = s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-case2_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "张小凡",
		Realm:     "凡人境",
		Inventory: "烧火棍",
	})

	// Plot hook resolved in chapter 2
	hook := &domain.PlotHook{
		ID:            "hook-case2-1",
		ProjectID:     proj.ID,
		Title:         "噬血珠之谜",
		Details:       "草庙村惨案留下的血珠",
		TargetChapter: 2,
		Status:        "OPEN",
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// Commit Chapter 1
	ch1 := &domain.Chapter{
		ID:           "ch_c2_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 草庙村惊变",
		CoreConflict: "全村罹难",
		Content:      "风雨交加之夜，青云门下草庙村...",
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	// Commit Chapter 2: resolves hook and mutates protagonist
	ch2 := &domain.Chapter{
		ID:           "ch_c2_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 拜入大竹峰",
		CoreConflict: "入门考核与法宝觉醒",
		Content:      "张小凡手持烧火棍，体内太极玄清道灵力汹涌运转...",
		Beats: []domain.SceneBeat{
			{Phase: "蓄力压迫", Tension: 5, Action: "师兄刁难"},
			{Phase: "破局爆发", Tension: 9, Action: "灵珠认主"},
		},
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 嗜血珠 (+1)",
			PowerDelta:     "领悟太极玄清道，突破玉清一层",
		},
		Review: &domain.ReviewResult{
			Verdict:         domain.ReviewVerdictAccepted,
			Score:           92,
			ResolvedHookIDs: []string{"hook-case2-1"},
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// Check hook is RESOLVED
	hooksAfterCommit, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil || len(hooksAfterCommit) == 0 {
		t.Fatalf("ListPlotHooks failed: %v", err)
	}
	if hooksAfterCommit[0].Status != "RESOLVED" {
		t.Fatalf("expected hook to be RESOLVED after commit, got %s", hooksAfterCommit[0].Status)
	}

	// Check protagonist mutated
	protMid, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry failed: %v", err)
	}
	if !strings.Contains(protMid.Inventory, "嗜血珠") || !strings.Contains(protMid.Realm, "玉清一层") {
		t.Fatalf("expected protagonist mutated, got realm=%s, inv=%s", protMid.Realm, protMid.Inventory)
	}

	// Uncommit chapter 2 (latest chapter)
	cp, p, err := s.UncommitChapter(ctx, proj.ID, 2)
	if err != nil {
		t.Fatalf("UncommitChapter 2 failed: %v", err)
	}
	if cp == nil || p == nil {
		t.Fatalf("expected non-nil checkpoint and project, got cp=%v, p=%v", cp, p)
	}

	// Verify checkpoint created
	if cp.ChapterIndex != 2 || cp.Phase != domain.CheckpointPhaseDrafted || cp.DraftText != ch2.Content {
		t.Errorf("unexpected checkpoint data: %+v", cp)
	}
	if cp.RewriteLoops != 0 {
		t.Errorf("expected RewriteLoops=0, got %d", cp.RewriteLoops)
	}
	if cp.AuditReport == nil || cp.AuditReport.Score != 92 || cp.AuditReport.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("unexpected checkpoint audit report: %+v", cp.AuditReport)
	}

	// Verify chapter 2 removed from chapters table
	chs, err := s.ListChapters(ctx, proj.ID)
	if err != nil || len(chs) != 1 {
		t.Fatalf("expected 1 chapter remaining after uncommit, got %d", len(chs))
	}
	if chs[0].ChapterIndex != 1 {
		t.Errorf("expected remaining chapter to be index 1, got %d", chs[0].ChapterIndex)
	}

	// Verify protagonist state rolled back
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry failed: %v", err)
	}
	if strings.Contains(protAfter.Inventory, "嗜血珠") {
		t.Errorf("expected 嗜血珠 removed from inventory after rollback, got %s", protAfter.Inventory)
	}
	if strings.Contains(protAfter.Realm, "玉清一层") {
		t.Errorf("expected realm reverted to 凡人境, got %s", protAfter.Realm)
	}

	// Verify hook restored
	hooksAfterUncommit, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil || len(hooksAfterUncommit) == 0 {
		t.Fatalf("ListPlotHooks failed: %v", err)
	}
	// Target chapter is 2, remaining chapter is 1; target_chapter <= 1 is false, so it must revert to OPEN
	if hooksAfterUncommit[0].Status != "OPEN" {
		t.Errorf("expected hook status to revert to OPEN, got %s", hooksAfterUncommit[0].Status)
	}
}

// Case ③: commit 产生 FERMENTING 后 uncommit → hook 回到 OPEN（无剩余章时）/保持 FERMENTING（仍有后续章覆盖 target 时）
func TestSQLiteStore_UncommitChapter_HookFermentingRecalculation(t *testing.T) {
	ctx := context.Background()

	// Sub-case A: uncommit when no remaining chapters -> hook reverts to OPEN
	t.Run("RevertToOpenWhenNoRemainingChapters", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test_hook_a.db")
		s, err := NewSQLiteStore(dbPath)
		if err != nil {
			t.Fatalf("NewSQLiteStore failed: %v", err)
		}
		defer s.Close()

		projID := "proj-case3a"
		if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "伏笔重算测试A"}); err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		hook := &domain.PlotHook{
			ID:            "hook-3a-1",
			ProjectID:     projID,
			Title:         "天劫将至",
			TargetChapter: 1,
			Status:        "OPEN",
		}
		if err := s.SavePlotHook(ctx, hook); err != nil {
			t.Fatalf("SavePlotHook failed: %v", err)
		}

		ch1 := &domain.Chapter{
			ID:           "ch_3a_1",
			ProjectID:    projID,
			ChapterIndex: 1,
			Title:        "第一章 引劫",
			Content:      "天地变色...",
		}
		if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
			t.Fatalf("CommitChapter 1 failed: %v", err)
		}

		// Because target_chapter (1) <= chapter_index (1), hook becomes FERMENTING
		hooks, err := s.ListPlotHooks(ctx, projID)
		if err != nil || len(hooks) == 0 {
			t.Fatalf("ListPlotHooks failed: %v", err)
		}
		if hooks[0].Status != "FERMENTING" {
			t.Fatalf("expected hook to be FERMENTING after commit, got %s", hooks[0].Status)
		}

		// Uncommit Chapter 1 -> no remaining chapters (maxRemaining = 0)
		if _, _, err := s.UncommitChapter(ctx, projID, 1); err != nil {
			t.Fatalf("UncommitChapter failed: %v", err)
		}

		// Hook must revert to OPEN
		hooksAfter, err := s.ListPlotHooks(ctx, projID)
		if err != nil || len(hooksAfter) == 0 {
			t.Fatalf("ListPlotHooks failed: %v", err)
		}
		if hooksAfter[0].Status != "OPEN" {
			t.Errorf("expected hook to revert to OPEN, got %s", hooksAfter[0].Status)
		}
	})

	// Sub-case B: uncommit when remaining chapters still cover target -> hook remains FERMENTING
	t.Run("StayFermentingWhenRemainingChaptersCoverTarget", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test_hook_b.db")
		s, err := NewSQLiteStore(dbPath)
		if err != nil {
			t.Fatalf("NewSQLiteStore failed: %v", err)
		}
		defer s.Close()

		projID := "proj-case3b"
		if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "伏笔重算测试B"}); err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		// Hook 1: target_chapter = 1 (covered by chapter 1)
		hook1 := &domain.PlotHook{
			ID:            "hook-3b-cover",
			ProjectID:     projID,
			Title:         "覆盖伏笔",
			TargetChapter: 1,
			Status:        "OPEN",
		}
		// Hook 2: target_chapter = 2 (covered by chapter 2, but NOT chapter 1)
		hook2 := &domain.PlotHook{
			ID:            "hook-3b-exceed",
			ProjectID:     projID,
			Title:         "后续伏笔",
			TargetChapter: 2,
			Status:        "OPEN",
		}
		if err := s.SavePlotHook(ctx, hook1); err != nil {
			t.Fatalf("SavePlotHook 1 failed: %v", err)
		}
		if err := s.SavePlotHook(ctx, hook2); err != nil {
			t.Fatalf("SavePlotHook 2 failed: %v", err)
		}

		// Commit Chapter 1
		ch1 := &domain.Chapter{
			ID:           "ch_3b_1",
			ProjectID:    projID,
			ChapterIndex: 1,
			Title:        "第一章",
			Content:      "第一章正文...",
		}
		if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
			t.Fatalf("CommitChapter 1 failed: %v", err)
		}

		// Commit Chapter 2
		ch2 := &domain.Chapter{
			ID:           "ch_3b_2",
			ProjectID:    projID,
			ChapterIndex: 2,
			Title:        "第二章",
			Content:      "第二章正文...",
		}
		if _, err := s.CommitChapter(ctx, projID, ch2); err != nil {
			t.Fatalf("CommitChapter 2 failed: %v", err)
		}

		// Both hooks should now be FERMENTING
		hooksMid, err := s.ListPlotHooks(ctx, projID)
		if err != nil {
			t.Fatalf("ListPlotHooks failed: %v", err)
		}
		hookStatusMap := make(map[string]string)
		for _, h := range hooksMid {
			hookStatusMap[h.ID] = string(h.Status)
		}
		if hookStatusMap["hook-3b-cover"] != "FERMENTING" || hookStatusMap["hook-3b-exceed"] != "FERMENTING" {
			t.Fatalf("expected both hooks FERMENTING, got %+v", hookStatusMap)
		}

		// Uncommit Chapter 2 -> Chapter 1 remains (maxRemaining = 1)
		if _, _, err := s.UncommitChapter(ctx, projID, 2); err != nil {
			t.Fatalf("UncommitChapter 2 failed: %v", err)
		}

		// Hook 1 (target 1 <= 1): MUST REMAIN FERMENTING
		// Hook 2 (target 2 <= 1 is false): MUST REVERT TO OPEN
		hooksFinal, err := s.ListPlotHooks(ctx, projID)
		if err != nil {
			t.Fatalf("ListPlotHooks failed: %v", err)
		}
		finalStatusMap := make(map[string]string)
		for _, h := range hooksFinal {
			finalStatusMap[h.ID] = string(h.Status)
		}
		if finalStatusMap["hook-3b-cover"] != "FERMENTING" {
			t.Errorf("expected hook-3b-cover to remain FERMENTING, got %s", finalStatusMap["hook-3b-cover"])
		}
		if finalStatusMap["hook-3b-exceed"] != "OPEN" {
			t.Errorf("expected hook-3b-exceed to revert to OPEN, got %s", finalStatusMap["hook-3b-exceed"])
		}
	})
}

// Case ④: RollbackStateMutation 失败 → 返回 error 且库内状态不变
func TestSQLiteStore_UncommitChapter_RollbackFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_uncommit_fail.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-case4",
		Title: "回滚失败事务一致性测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	_ = s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-case4_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "白小纯",
		Realm:     "凝气一层",
		Inventory: "铁锅",
	})

	ch1 := &domain.Chapter{
		ID:           "ch_c4_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 灵溪宗",
		CoreConflict: "偷吃灵鸡",
		Content:      "白小纯看着手中的灵鸡，口水直流...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 龟纹锅 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// Verify chapter is committed and protagonist has 龟纹锅
	protBefore, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil || !strings.Contains(protBefore.Inventory, "龟纹锅") {
		t.Fatalf("expected protagonist to have 龟纹锅 before uncommit, got %v", protBefore)
	}

	// Temporarily inject a failing rollbackStateMutation helper
	origRollback := rollbackStateMutation
	rollbackStateMutation = func(e *domain.CodexEntry, mutation domain.StateMutation) ([]string, error) {
		return nil, errors.New("simulated state mutation rollback failure")
	}
	defer func() {
		rollbackStateMutation = origRollback
	}()

	// Attempt UncommitChapter
	cp, p, err := s.UncommitChapter(ctx, proj.ID, 1)
	if err == nil {
		t.Fatalf("expected UncommitChapter to fail on rollback error, got success cp=%v, p=%v", cp, p)
	}
	if !strings.Contains(err.Error(), "simulated state mutation rollback failure") {
		t.Errorf("expected error to contain simulated failure message, got: %v", err)
	}

	// Verify database transaction rolled back: Chapter 1 STILL exists
	savedCh, err := s.GetChapter(ctx, proj.ID, 1)
	if err != nil || savedCh == nil {
		t.Fatalf("expected chapter 1 to still exist in DB, err: %v", err)
	}
	if savedCh.Content != ch1.Content {
		t.Errorf("chapter 1 content altered despite rollback failure")
	}

	// Verify protagonist state is intact
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry failed: %v", err)
	}
	if !strings.Contains(protAfter.Inventory, "龟纹锅") {
		t.Errorf("protagonist state was corrupted despite rollback failure: %s", protAfter.Inventory)
	}

	// Verify no draft checkpoint was persisted
	storedCp, _ := s.GetCheckpoint(ctx, proj.ID, 1)
	if storedCp != nil && storedCp.Phase == domain.CheckpointPhaseDrafted {
		t.Errorf("unexpected draft checkpoint committed despite rollback failure: %+v", storedCp)
	}
}

// Case ⑤: ch1 归档带境界突破 → ch2 归档仅道具变动 → UncommitChapter(ch2) → 断言主角 Realm 仍为 ch1 突破后的境界、StructuredLevel.History 长度不变
func TestSQLiteStore_UncommitChapter_PowerDeltaGuard(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_uncommit_power_delta_guard.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-guard-e2e",
		Title: "回滚PowerDelta守卫端到端测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	_ = s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-guard_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "韩立",
		Realm:     "练气一层",
		Inventory: "粗布衣",
	})

	// Chapter 1: 归档带境界突破 (练气一层 ➔ 突破练气二层)
	ch1 := &domain.Chapter{
		ID:           "ch_guard_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 突破境界",
		CoreConflict: "突破练气二层",
		Content:      "天地灵气汇聚，韩立运转长春功突破...",
		StateMutation: domain.StateMutation{
			PowerDelta: "突破练气二层",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	// Verify ch1 breakthrough applied
	protCh1, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil || protCh1 == nil {
		t.Fatalf("GetProtagonistEntry after ch1 failed: %v", err)
	}
	if !strings.Contains(protCh1.Realm, "练气二层") {
		t.Fatalf("expected protagonist realm to be 练气二层, got %s", protCh1.Realm)
	}
	if protCh1.StructuredLevel == nil || len(protCh1.StructuredLevel.History) != 1 {
		t.Fatalf("expected StructuredLevel.History length 1 after ch1, got %+v", protCh1.StructuredLevel)
	}

	// Chapter 2: 归档仅道具变动 (power_delta: "")
	ch2 := &domain.Chapter{
		ID:           "ch_guard_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 仅道具变动",
		CoreConflict: "山洞寻宝",
		Content:      "韩立在洞府中得到一枚青凝镜...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 青凝镜 (+1)",
			PowerDelta:     "",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// Verify ch2 inventory added, realm still 练气二层
	protCh2, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil || protCh2 == nil {
		t.Fatalf("GetProtagonistEntry after ch2 failed: %v", err)
	}
	if !strings.Contains(protCh2.Inventory, "青凝镜") {
		t.Fatalf("expected 青凝镜 in inventory, got %s", protCh2.Inventory)
	}
	if !strings.Contains(protCh2.Realm, "练气二层") {
		t.Fatalf("expected realm 练气二层 before uncommit, got %s", protCh2.Realm)
	}
	if protCh2.StructuredLevel == nil || len(protCh2.StructuredLevel.History) != 1 {
		t.Fatalf("expected StructuredLevel.History length 1 before uncommit, got %+v", protCh2.StructuredLevel)
	}

	// Uncommit Chapter 2
	cp, p, err := s.UncommitChapter(ctx, proj.ID, 2)
	if err != nil {
		t.Fatalf("UncommitChapter 2 failed: %v", err)
	}
	if cp == nil || p == nil {
		t.Fatalf("expected non-nil checkpoint and project, got cp=%v, p=%v", cp, p)
	}

	// Assert: Protagonist Realm remains ch1 breakthrough realm, StructuredLevel.History length unchanged, inventory reverted
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil || protAfter == nil {
		t.Fatalf("GetProtagonistEntry after uncommit failed: %v", err)
	}
	if !strings.Contains(protAfter.Realm, "练气二层") {
		t.Errorf("expected protagonist realm to remain 练气二层 after uncommitting ch2 (no power delta), got %s", protAfter.Realm)
	}
	if protAfter.StructuredLevel == nil || len(protAfter.StructuredLevel.History) != 1 {
		t.Errorf("expected StructuredLevel.History length to remain 1 after uncommitting ch2, got %+v", protAfter.StructuredLevel)
	}
	if strings.Contains(protAfter.Inventory, "青凝镜") {
		t.Errorf("expected 青凝镜 removed from inventory after uncommitting ch2, got %s", protAfter.Inventory)
	}
}
