package store

import (
	"context"
	"encoding/json"
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

// Case ⑤: commit 3 章（含道具增减 mutation + 一条被第 3 章回收的 hook + 跨 3 章的 FERMENTING 钩子）
// → uncommit 第 3 章 → 主角 Inventory/StructuredItems 与 GetChapterStateSnapshot(2) 的 protagonist_json 逐字段相等，
// hooks status 与快照(2) 逐条相等（跑通"精确还原"主张）
func TestSQLiteStore_UncommitChapter_SnapshotExactRestore(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_snapshot_exact_restore.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-snap-exact",
		Title: "快照精确还原测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 初始主角：林动，拥有石符
	protInitial := &domain.CodexEntry{
		ID:        "codex_snap_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "林动",
		Realm:     "地元境初期",
		Inventory: "石符x1",
		StructuredItems: []domain.InventoryItem{
			{Name: "石符", Quantity: 1, Quality: "神物", AcquiredAt: 0},
		},
	}
	if err := s.SaveCodexEntry(ctx, protInitial); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	// Hook 1: 跨 3 章的 FERMENTING 钩子（target_chapter = 1，第 1 章归档后即进入 FERMENTING 并跨越全部 3 章）
	hookFermenting := &domain.PlotHook{
		ID:            "hook-snap-ferment",
		ProjectID:     proj.ID,
		Title:         "大荒古碑封印松动",
		Details:       "封印异动，魔气隐现",
		TargetChapter: 1,
		Status:        domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hookFermenting); err != nil {
		t.Fatalf("SavePlotHook fermenting failed: %v", err)
	}

	// Hook 2: 被第 3 章回收的 hook（target_chapter = 3，前两章为 OPEN，第 3 章 review 回收为 RESOLVED）
	hookResolved := &domain.PlotHook{
		ID:            "hook-snap-resolved",
		ProjectID:     proj.ID,
		Title:         "天鳞古戟争夺",
		Details:       "古墓府核心秘宝",
		TargetChapter: 3,
		Status:        domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hookResolved); err != nil {
		t.Fatalf("SavePlotHook resolved failed: %v", err)
	}

	// Commit 第 1 章：获得 纯元丹 (+10)
	ch1 := &domain.Chapter{
		ID:           "ch_snap_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 初识石符",
		CoreConflict: "淬炼灵药",
		Content:      "林动握紧石符，温润之感自掌心传开...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 纯元丹 (+10)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	// Commit 第 2 章：获得 天鳞古戟 (+1)
	ch2 := &domain.Chapter{
		ID:           "ch_snap_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 墓府探幽",
		CoreConflict: "突破地元境后期",
		Content:      "墓府深处，戟芒吞吐如龙...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 天鳞古戟 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// 获取第 2 章快照作为基准预期
	snap2, err := s.GetChapterStateSnapshot(ctx, proj.ID, 2)
	if err != nil || snap2 == nil {
		t.Fatalf("GetChapterStateSnapshot(2) failed: %v, snap: %v", err, snap2)
	}
	var snap2Prot domain.CodexEntry
	if err := json.Unmarshal([]byte(snap2.ProtagonistJSON), &snap2Prot); err != nil {
		t.Fatalf("unmarshal snap2 protagonist failed: %v", err)
	}
	var snap2Hooks []*domain.PlotHook
	if err := json.Unmarshal([]byte(snap2.HooksJSON), &snap2Hooks); err != nil {
		t.Fatalf("unmarshal snap2 hooks failed: %v", err)
	}

	// Commit 第 3 章：消耗纯元丹 (-5)、获得涅槃丹 (+1)、回收 hook-snap-resolved
	ch3 := &domain.Chapter{
		ID:           "ch_snap_3",
		ProjectID:    proj.ID,
		ChapterIndex: 3,
		Title:        "第三章 戟化金龙",
		CoreConflict: "血战强敌",
		Content:      "金龙呼啸，天鳞古戟横扫千军，强敌尽伏...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "消耗物品: 纯元丹 (-5); 获得物品: 涅槃丹 (+1)",
		},
		Review: &domain.ReviewResult{
			Verdict:         domain.ReviewVerdictAccepted,
			Score:           95,
			ResolvedHookIDs: []string{"hook-snap-resolved"},
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch3); err != nil {
		t.Fatalf("CommitChapter 3 failed: %v", err)
	}

	// 确认第 3 章 commit 后状态已改变
	protMid, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry mid failed: %v", err)
	}
	if !strings.Contains(protMid.Inventory, "涅槃丹") {
		t.Fatalf("expected protagonist to have 涅槃丹 in ch3, got: %s", protMid.Inventory)
	}
	hooksMid, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListPlotHooks mid failed: %v", err)
	}
	midResolved := false
	for _, h := range hooksMid {
		if h.ID == "hook-snap-resolved" && h.Status == domain.HookStatusResolved {
			midResolved = true
		}
	}
	if !midResolved {
		t.Fatalf("expected hook-snap-resolved to be RESOLVED in ch3")
	}

	// Uncommit 第 3 章
	cp, p, err := s.UncommitChapter(ctx, proj.ID, 3)
	if err != nil {
		t.Fatalf("UncommitChapter 3 failed: %v", err)
	}
	if cp == nil || p == nil {
		t.Fatalf("expected non-nil checkpoint and project, got cp=%v, p=%v", cp, p)
	}

	// 验证主角状态：Inventory 与 StructuredItems 以及各项字段与快照(2)逐字段严格相等
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry after failed: %v", err)
	}
	if protAfter.Inventory != snap2Prot.Inventory {
		t.Errorf("protagonist Inventory mismatch: got %q, expected snap2 %q", protAfter.Inventory, snap2Prot.Inventory)
	}
	if len(protAfter.StructuredItems) != len(snap2Prot.StructuredItems) {
		t.Errorf("protagonist StructuredItems length mismatch: got %d, expected %d", len(protAfter.StructuredItems), len(snap2Prot.StructuredItems))
	} else {
		for i := range protAfter.StructuredItems {
			if protAfter.StructuredItems[i] != snap2Prot.StructuredItems[i] {
				t.Errorf("protagonist StructuredItems[%d] mismatch: got %+v, expected %+v", i, protAfter.StructuredItems[i], snap2Prot.StructuredItems[i])
			}
		}
	}
	if protAfter.Realm != snap2Prot.Realm {
		t.Errorf("protagonist Realm mismatch: got %q, expected snap2 %q", protAfter.Realm, snap2Prot.Realm)
	}
	if protAfter.CurrentDisposition != snap2Prot.CurrentDisposition {
		t.Errorf("protagonist CurrentDisposition mismatch: got %q, expected snap2 %q", protAfter.CurrentDisposition, snap2Prot.CurrentDisposition)
	}

	// 验证伏笔状态：hooks status 与快照(2)逐条严格相等
	hooksAfter, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListPlotHooks after failed: %v", err)
	}
	snapHooksMap := make(map[string]domain.HookStatus)
	for _, h := range snap2Hooks {
		snapHooksMap[h.ID] = h.Status
	}
	if len(hooksAfter) != len(snap2Hooks) {
		t.Errorf("hooks count mismatch: got %d, expected %d", len(hooksAfter), len(snap2Hooks))
	}
	for _, h := range hooksAfter {
		expectedStatus, ok := snapHooksMap[h.ID]
		if !ok {
			t.Errorf("unexpected hook ID %s in database", h.ID)
			continue
		}
		if h.Status != expectedStatus {
			t.Errorf("hook %s status mismatch: got %s, expected snap2 %s", h.ID, h.Status, expectedStatus)
		}
	}
}

// Case ⑥: 手工删掉快照(2) 行 → uncommit 第 3 章 → 返回错误且包含 "snapshot" 字样，
// 且库里章节/主角/hooks 均未变（fail-closed 原子性）
func TestSQLiteStore_UncommitChapter_MissingSnapshotFailClosed(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_snapshot_fail_closed.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-snap-failclosed",
		Title: "快照缺失Fail-Closed测试",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	prot := &domain.CodexEntry{
		ID:        "codex_snap_fc_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "牧尘",
		Realm:     "灵动境初期",
		Inventory: "大须弥魔柱",
	}
	if err := s.SaveCodexEntry(ctx, prot); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	hook := &domain.PlotHook{
		ID:            "hook-fc-1",
		ProjectID:     proj.ID,
		Title:         "北苍灵院考核",
		TargetChapter: 3,
		Status:        domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// Commit 3 chapters
	ch1 := &domain.Chapter{
		ID:           "ch_fc_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 北灵之乱",
		Content:      "灵路归来的少年...",
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	ch2 := &domain.Chapter{
		ID:           "ch_fc_2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章 灵路相逢",
		Content:      "洛璃持剑而立...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 洛神剑意 (+1)",
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	ch3 := &domain.Chapter{
		ID:           "ch_fc_3",
		ProjectID:    proj.ID,
		ChapterIndex: 3,
		Title:        "第三章 考核过关",
		Content:      "灵力光幕冲天而起...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 灵值牌 (+1)",
		},
		Review: &domain.ReviewResult{
			Verdict:         domain.ReviewVerdictAccepted,
			Score:           90,
			ResolvedHookIDs: []string{"hook-fc-1"},
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch3); err != nil {
		t.Fatalf("CommitChapter 3 failed: %v", err)
	}

	// 记录 uncommit 前的库内完整状态
	ch3Before, err := s.GetChapter(ctx, proj.ID, 3)
	if err != nil || ch3Before == nil {
		t.Fatalf("GetChapter 3 before failed: %v", err)
	}
	protBefore, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil || protBefore == nil {
		t.Fatalf("GetProtagonistEntry before failed: %v", err)
	}
	hooksBefore, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListPlotHooks before failed: %v", err)
	}

	// 手工删掉快照(2)行
	res, err := s.db.Exec(`DELETE FROM chapter_state_snapshots WHERE project_id = ? AND chapter_index = 2`, proj.ID)
	if err != nil {
		t.Fatalf("delete snapshot 2 failed: %v", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		t.Fatalf("expected at least 1 snapshot row deleted, got 0")
	}

	// Attempt uncommit 第 3 章
	cp, p, err := s.UncommitChapter(ctx, proj.ID, 3)
	if err == nil {
		t.Fatalf("expected UncommitChapter to fail due to missing snapshot, got success cp=%v, p=%v", cp, p)
	}

	// 返回错误且包含 "snapshot" 字样
	if !strings.Contains(err.Error(), "snapshot") {
		t.Errorf("expected error message to contain 'snapshot', got: %v", err)
	}

	// 库里章节纹丝不动：第 3 章依然存在且内容不变
	ch3After, err := s.GetChapter(ctx, proj.ID, 3)
	if err != nil || ch3After == nil {
		t.Fatalf("expected chapter 3 to still exist in DB, err: %v", err)
	}
	if ch3After.Content != ch3Before.Content {
		t.Errorf("chapter 3 content altered despite snapshot failure")
	}

	// 库里主角状态纹丝不动
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry after failed: %v", err)
	}
	if protAfter.Inventory != protBefore.Inventory || protAfter.Realm != protBefore.Realm {
		t.Errorf("protagonist altered: before inv=%s, after inv=%s", protBefore.Inventory, protAfter.Inventory)
	}

	// 库里 hooks 纹丝不动
	hooksAfter, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListPlotHooks after failed: %v", err)
	}
	if len(hooksAfter) != len(hooksBefore) {
		t.Errorf("hooks count changed: before=%d, after=%d", len(hooksBefore), len(hooksAfter))
	} else {
		for i := range hooksAfter {
			if hooksAfter[i].Status != hooksBefore[i].Status {
				t.Errorf("hook[%d] status altered: before=%s, after=%s", i, hooksBefore[i].Status, hooksAfter[i].Status)
			}
		}
	}

	// Checkpoint 表中未生成草稿
	storedCp, _ := s.GetCheckpoint(ctx, proj.ID, 3)
	if storedCp != nil && storedCp.Phase == domain.CheckpointPhaseDrafted {
		t.Errorf("unexpected draft checkpoint committed despite fail-closed error: %+v", storedCp)
	}
}

// Case ⑦: 第 1 章 uncommit 路径回归测试（创世状态回滚 + FERMENTING 全部归为 OPEN）
func TestSQLiteStore_UncommitChapter_Chapter1GenesisRegression(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ch1_regression.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	proj := &domain.Project{
		ID:    "proj-ch1-reg",
		Title: "第1章创世回退回归",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	prot := &domain.CodexEntry{
		ID:        "codex_ch1_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "孟浩",
		Realm:     "凝气一层",
		Inventory: "布衣",
	}
	if err := s.SaveCodexEntry(ctx, prot); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	hook := &domain.PlotHook{
		ID:            "hook-ch1-1",
		ProjectID:     proj.ID,
		Title:         "靠山宗秘宝",
		TargetChapter: 1,
		Status:        domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_reg_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 读书人孟浩",
		Content:      "大青山下，书生远眺...",
		StateMutation: domain.StateMutation{
			InventoryDelta: "获得物品: 铜镜 (+1)",
		},
		Review: &domain.ReviewResult{
			Verdict:         domain.ReviewVerdictAccepted,
			Score:           88,
			ResolvedHookIDs: []string{"hook-ch1-1"},
		},
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	cp, p, err := s.UncommitChapter(ctx, proj.ID, 1)
	if err != nil {
		t.Fatalf("UncommitChapter 1 failed: %v", err)
	}
	if cp == nil || p == nil {
		t.Fatalf("expected non-nil checkpoint and project")
	}

	// 确认主角物品铜镜被逆运算移除
	protAfter, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetProtagonistEntry failed: %v", err)
	}
	if strings.Contains(protAfter.Inventory, "铜镜") {
		t.Errorf("expected 铜镜 removed by inverse mutation, got: %s", protAfter.Inventory)
	}

	// 确认伏笔在 maxRemaining=0 时重算为 OPEN
	hooksAfter, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil || len(hooksAfter) == 0 {
		t.Fatalf("ListPlotHooks failed: %v", err)
	}
	if hooksAfter[0].Status != domain.HookStatusOpen {
		t.Errorf("expected hook status OPEN, got: %s", hooksAfter[0].Status)
	}
}
