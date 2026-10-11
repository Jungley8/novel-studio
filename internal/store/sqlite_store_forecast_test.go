package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func mustComputeFingerprint(t testing.TB, s *SQLiteStore, ctx context.Context, projectID string) string {
	t.Helper()
	fp, err := s.ComputeFingerprint(ctx, projectID)
	if err != nil {
		t.Fatalf("ComputeFingerprint failed: %v", err)
	}
	return fp
}

// Case ②: select 后三表（chapters/codex/hooks）零变化
func TestSQLiteStore_SelectBranch_IsolationGuard(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_select_isolation.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_isolation_test"
	proj := &domain.Project{
		ID:         projID,
		Title:      "隔离铁律验证录",
		WorldRules: "天道因果不可违逆",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. Seed chapters table
	ch1 := &domain.Chapter{
		ID:           "ch_iso_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "引子与异变",
		CoreConflict: "山村遭劫",
		Content:      "黑夜笼罩了整个山村，雷霆乍现。",
		WordCount:    200,
		CreatedAt:    time.Now().Add(-2 * time.Hour),
	}
	ch2 := &domain.Chapter{
		ID:           "ch_iso_2",
		ProjectID:    projID,
		ChapterIndex: 2,
		Title:        "出山求仙",
		CoreConflict: "路遇猛虎",
		Content:      "少年背负行囊，踏上寻仙之道。",
		WordCount:    250,
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}
	if _, err := s.CommitChapter(ctx, projID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// 2. Seed codex entries
	codex1 := &domain.CodexEntry{
		ID:        "codex_iso_pro",
		ProjectID: projID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "苏白",
		Realm:     "炼体初阶",
		Inventory: "干粮, 生锈短刀",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.SaveCodexEntry(ctx, codex1); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	// 3. Seed plot hooks
	hook1 := &domain.PlotHook{
		ID:             "hook_iso_1",
		ProjectID:      projID,
		Title:          "神秘玉佩的秘密",
		Details:        "母亲临终前交代的古玉碎片",
		CreatedChapter: 1,
		TargetChapter:  10,
		Status:         domain.HookStatusOpen,
		CreatedAt:      time.Now(),
	}
	if err := s.SavePlotHook(ctx, hook1); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// Snapshot row counts and detailed contents before SelectBranch
	var countChaptersBefore, countCodexBefore, countHooksBefore int
	_ = s.db.QueryRow(`SELECT count(*) FROM chapters WHERE project_id = ?`, projID).Scan(&countChaptersBefore)
	_ = s.db.QueryRow(`SELECT count(*) FROM codex_entries WHERE project_id = ?`, projID).Scan(&countCodexBefore)
	_ = s.db.QueryRow(`SELECT count(*) FROM plot_hooks WHERE project_id = ?`, projID).Scan(&countHooksBefore)

	if countChaptersBefore != 2 || countCodexBefore != 1 || countHooksBefore != 1 {
		t.Fatalf("unexpected initial counts: ch=%d, codex=%d, hooks=%d",
			countChaptersBefore, countCodexBefore, countHooksBefore)
	}

	// Insert narrative forecast
	fc := &domain.Forecast{
		ID:          "fc_iso_1",
		ProjectID:   projID,
		BaseChapter: "2",
		Divergence:  "是否拜入青云宗",
		BranchCount: 3,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{
				ID:             "b_join",
				BeatsSummary:   "通过考核拜入宗门",
				Decisions:      "展现部分剑道天赋",
				ExpectedDeltas: "+青云令, 晋升外门弟子",
				Risks:          "引来内门师兄妒忌",
				IntentMatch:    "正规修真路线",
			},
			{
				ID:             "b_rogue",
				BeatsSummary:   "拒绝招揽沦为散修",
				Decisions:      "离开青云山脉潜入黑市",
				ExpectedDeltas: "+黑市情报, 获得残缺古经",
				Risks:          "缺少宗门大阵庇护随时被杀",
				IntentMatch:    "孤独求道路线",
			},
			{
				ID:             "b_spy",
				BeatsSummary:   "受神秘人胁迫做内应",
				Decisions:      "假意拜师实则窃密",
				ExpectedDeltas: "+潜行法宝, 暗中联络符",
				Risks:          "身份败露直接处决",
				IntentMatch:    "险中求胜路线",
			},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	// Call SelectBranch
	activePlan, err := s.SelectBranch(ctx, projID, fc.ID, "b_join")
	if err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}
	if activePlan == nil || activePlan.BranchID != "b_join" {
		t.Fatalf("unexpected active plan: %+v", activePlan)
	}

	// Strict assertion: zero modifications to chapters, codex_entries, and plot_hooks
	var countChaptersAfter, countCodexAfter, countHooksAfter int
	_ = s.db.QueryRow(`SELECT count(*) FROM chapters WHERE project_id = ?`, projID).Scan(&countChaptersAfter)
	_ = s.db.QueryRow(`SELECT count(*) FROM codex_entries WHERE project_id = ?`, projID).Scan(&countCodexAfter)
	_ = s.db.QueryRow(`SELECT count(*) FROM plot_hooks WHERE project_id = ?`, projID).Scan(&countHooksAfter)

	if countChaptersAfter != countChaptersBefore {
		t.Fatalf("ISOLATION VIOLATION: chapters count changed from %d to %d after SelectBranch",
			countChaptersBefore, countChaptersAfter)
	}
	if countCodexAfter != countCodexBefore {
		t.Fatalf("ISOLATION VIOLATION: codex_entries count changed from %d to %d after SelectBranch",
			countCodexBefore, countCodexAfter)
	}
	if countHooksAfter != countHooksBefore {
		t.Fatalf("ISOLATION VIOLATION: plot_hooks count changed from %d to %d after SelectBranch",
			countHooksBefore, countHooksAfter)
	}

	// Verify exact content unchanged
	var loadedCh2Title string
	_ = s.db.QueryRow(`SELECT title FROM chapters WHERE id = ?`, "ch_iso_2").Scan(&loadedCh2Title)
	if loadedCh2Title != "出山求仙" {
		t.Errorf("chapter content mutated: expected '出山求仙', got '%s'", loadedCh2Title)
	}

	var loadedCodexRealm string
	_ = s.db.QueryRow(`SELECT realm FROM codex_entries WHERE id = ?`, "codex_iso_pro").Scan(&loadedCodexRealm)
	if loadedCodexRealm != "炼体初阶" {
		t.Errorf("codex entry mutated: expected '炼体初阶', got '%s'", loadedCodexRealm)
	}

	var loadedHookStatus string
	_ = s.db.QueryRow(`SELECT status FROM plot_hooks WHERE id = ?`, "hook_iso_1").Scan(&loadedHookStatus)
	if loadedHookStatus != string(domain.HookStatusOpen) {
		t.Errorf("plot hook mutated: expected 'OPEN', got '%s'", loadedHookStatus)
	}
}

// Case ③ (v2 rewrite): archiving a new chapter is normal execution per plan — forecast stays ACTIVE.
// v2 product ruling: only upstream rewrites stale a plan (framework premise change, history
// rewritten below base, hook-set surgery). Positional shift (maxChapter/lastID) and hook status
// flow are execution, not rewrites.
func TestSQLiteStore_ExecutionChapterArchive_KeepsForecastActive(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_forecast_stale.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_stale_test"
	_ = s.SaveProject(ctx, &domain.Project{ID: projID, Title: "指纹失效测试"})

	ch1 := &domain.Chapter{
		ID:           "ch_stale_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "初始正史内容",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	// Create forecast based on chapter 1
	fc := &domain.Forecast{
		ID:          "fc_stale_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧测试",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	// Select branch to create active plan
	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	// Reading forecast right now should be active
	loaded1, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast failed: %v", err)
	}
	if loaded1.Status != domain.ForecastStatusActive {
		t.Fatalf("expected active status before new chapter, got %s", loaded1.Status)
	}

	// New chapter is archived / committed into canon
	ch2 := &domain.Chapter{
		ID:           "ch_stale_2",
		ProjectID:    projID,
		ChapterIndex: 2,
		Title:        "第二章正史归档",
		Content:      "新的正史发展已经发生",
		CreatedAt:    time.Now(),
	}
	if _, err := s.CommitChapter(ctx, projID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// Reading forecast now MUST stay active: archiving chapter 2 per plan is execution,
	// not an upstream rewrite, so the v2 fingerprint (framework + hook identities) is unchanged
	// and maxChapter(2) >= baseChapter(1).
	loaded2, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast after chapter archive failed: %v", err)
	}
	if loaded2.Status != domain.ForecastStatusActive {
		t.Fatalf("expected forecast status to stay 'active' after in-plan chapter archive, got '%s'", loaded2.Status)
	}

	// Active plan must also stay active (consumed derivation is in-memory; divergence needs
	// ExpectedDeltas keywords, absent here)
	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusActive {
		t.Fatalf("expected active plan status to stay 'active', got '%s'", activePlan.Status)
	}
}

// Case ④: 第二次 create 后旧 active 为 archived 且全 project 只有一行 active
func TestSQLiteStore_SingleActiveInvariant_OnCreateForecast(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_single_active.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_single_active"
	_ = s.SaveProject(ctx, &domain.Project{ID: projID, Title: "单一活跃不变式测试"})

	// v2: seed the base chapter (production always branches from existing canon:
	// base == maxChapter at creation, so base must exist for the forecast to stay active)
	ch0 := &domain.Chapter{
		ID:           "ch_sa_base_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "正史开篇",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch0); err != nil {
		t.Fatalf("CommitChapter base failed: %v", err)
	}

	fc1 := &domain.Forecast{
		ID:          "fc_first",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "第一次分歧",
		BranchCount: 2,
		Horizon:     3,
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "首轮节拍1"},
			{ID: "b2", BeatsSummary: "首轮节拍2"},
		},
		CreatedAt: time.Now().Add(-10 * time.Minute),
	}
	if err := s.SaveForecast(ctx, fc1); err != nil {
		t.Fatalf("SaveForecast 1 failed: %v", err)
	}

	// Verify first forecast is active
	f1Loaded, err := s.LoadForecast(ctx, projID, fc1.ID)
	if err != nil || f1Loaded.Status != domain.ForecastStatusActive {
		t.Fatalf("expected fc1 active, got %v / status %s", err, f1Loaded.Status)
	}

	// Create second forecast for the same project
	fc2 := &domain.Forecast{
		ID:          "fc_second",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "第二次分歧（推翻旧方案）",
		BranchCount: 3,
		Horizon:     3,
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "次轮节拍1"},
			{ID: "b2", BeatsSummary: "次轮节拍2"},
			{ID: "b3", BeatsSummary: "次轮节拍3"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc2); err != nil {
		t.Fatalf("SaveForecast 2 failed: %v", err)
	}

	// 1. Verify old active is now archived
	f1After, err := s.LoadForecast(ctx, projID, fc1.ID)
	if err != nil {
		t.Fatalf("LoadForecast fc1 failed: %v", err)
	}
	if f1After.Status != domain.ForecastStatusArchived {
		t.Fatalf("expected fc1 status to be 'archived' after fc2 creation, got '%s'", f1After.Status)
	}

	// 2. Verify new forecast is active
	f2After, err := s.LoadForecast(ctx, projID, fc2.ID)
	if err != nil {
		t.Fatalf("LoadForecast fc2 failed: %v", err)
	}
	if f2After.Status != domain.ForecastStatusActive {
		t.Fatalf("expected fc2 status to be 'active', got '%s'", f2After.Status)
	}

	// 3. Verify exactly ONE active row exists in narrative_forecasts for this project
	var activeCount int
	if err := s.db.QueryRow(`
		SELECT count(*) 
		FROM narrative_forecasts 
		WHERE project_id = ? AND status = ?`,
		projID, domain.ForecastStatusActive,
	).Scan(&activeCount); err != nil {
		t.Fatalf("query active count failed: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active forecast row for project, got %d", activeCount)
	}
}

// In-memory derived consumption test
func TestSQLiteStore_GetActivePlan_DerivedConsumed(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_derived_consumed.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_consume_test"
	_ = s.SaveProject(ctx, &domain.Project{ID: projID, Title: "消费派生测试"})

	// v2: seed the base chapter (production branches from existing canon)
	ch1base := &domain.Chapter{
		ID:           "ch_c_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章正史",
		Content:      "主角下山",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1base); err != nil {
		t.Fatalf("CommitChapter base failed: %v", err)
	}

	// Forecast covering chapter 2 and chapter 3
	fc := &domain.Forecast{
		ID:          "fc_consume_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "下山历练路线",
		BranchCount: 2,
		Horizon:     2, // Covers chapters 2 and 3
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "连破两境", ExpectedDeltas: "+青云剑"},
			{ID: "b2", BeatsSummary: "隐居苦修", ExpectedDeltas: "+凝神丹"},
		},
		CreatedAt: time.Now(),
	}
	_ = s.SaveForecast(ctx, fc)
	_, _ = s.SelectBranch(ctx, projID, fc.ID, "b1")

	// Initially, neither ch2 nor ch3 exists in chapters table
	plan0, err := s.GetActivePlan(ctx, projID)
	if err != nil || plan0 == nil {
		t.Fatalf("GetActivePlan 0 failed: %v", err)
	}
	for _, pc := range plan0.PlanChapters {
		if pc.Status != domain.PlanStatusActive {
			t.Errorf("expected plan chapter %d to be active, got %s", pc.ChapterIndex, pc.Status)
		}
	}

	// Commit chapter 2 to canon
	ch2 := &domain.Chapter{
		ID:           "ch_c_2",
		ProjectID:    projID,
		ChapterIndex: 2,
		Title:        "第二章正史",
		Content:      "主角获得青云剑",
		StateMutation: domain.StateMutation{
			InventoryDelta: "+青云剑",
		},
		CreatedAt: time.Now(),
	}
	_, _ = s.CommitChapter(ctx, projID, ch2)

	// Now GetActivePlan should in-memory derive chapter 2 as consumed, while chapter 3 remains active
	plan1, err := s.GetActivePlan(ctx, projID)
	if err != nil || plan1 == nil {
		t.Fatalf("GetActivePlan 1 failed: %v", err)
	}
	if plan1.PlanChapters[0].ChapterIndex != 2 || plan1.PlanChapters[0].Status != domain.PlanStatusConsumed {
		t.Errorf("expected chapter 2 to be derived consumed, got %+v", plan1.PlanChapters[0])
	}
	if plan1.PlanChapters[1].ChapterIndex != 3 || plan1.PlanChapters[1].Status != domain.PlanStatusActive {
		t.Errorf("expected chapter 3 to be active, got %+v", plan1.PlanChapters[1])
	}

	// Verify plan_chapters_json in database was NOT modified (in-memory derivation)
	var rawJSON string
	_ = s.db.QueryRow(`SELECT plan_chapters_json FROM project_active_plan WHERE project_id = ?`, projID).Scan(&rawJSON)
	if rawJSON == "" {
		t.Error("expected database to have original plan_chapters_json")
	}
}

// Divergence detection test
func TestSQLiteStore_GetActivePlan_DivergedDetection(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_divergence.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_diverge_test"
	_ = s.SaveProject(ctx, &domain.Project{ID: projID, Title: "偏离检测测试"})

	// v2: seed the base chapter (production branches from existing canon)
	ch1base := &domain.Chapter{
		ID:           "ch_div_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章正史",
		Content:      "主角与王长老结仇",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1base); err != nil {
		t.Fatalf("CommitChapter base failed: %v", err)
	}

	fc := &domain.Forecast{
		ID:          "fc_div_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "斩杀强敌还是结盟",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{
				ID:             "b_kill",
				BeatsSummary:   "血战斩敌",
				ExpectedDeltas: "斩杀王长老，强夺降魔杵，突破化神期",
			},
			{
				ID:             "b_peace",
				BeatsSummary:   "秘密结盟",
				ExpectedDeltas: "结盟王长老，互换心经",
			},
		},
		CreatedAt: time.Now(),
	}
	_ = s.SaveForecast(ctx, fc)
	_, _ = s.SelectBranch(ctx, projID, fc.ID, "b_kill")

	// Canon chapter 2 is written with completely contradictory state mutation
	ch2 := &domain.Chapter{
		ID:           "ch_div_2",
		ProjectID:    projID,
		ChapterIndex: 2,
		Title:        "冰释前嫌",
		Content:      "双方放下了兵刃，共饮灵茶。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "赠予灵茶",
			PowerDelta:     "道心无暇",
			CharacterMutations: []domain.CharacterMutation{
				{Name: "王长老", StatusDelta: "和睦相处，收为义子", RelationDelta: "至交密友"},
			},
		},
		CreatedAt: time.Now(),
	}
	_, _ = s.CommitChapter(ctx, projID, ch2)

	// GetActivePlan detects keyword overlap is below 20% threshold -> sets status diverged
	plan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Status != domain.PlanStatusDiverged {
		t.Fatalf("expected plan status to be 'diverged', got '%s'", plan.Status)
	}
	if plan.WarningMessage == "" {
		t.Error("expected non-empty warning message for author when diverged")
	}
}

// Case ⑤: ListForecasts 不再与单连接池发生自死锁（死锁修复与状态刷新验证）
func TestListForecasts_NoDeadlockAfterChapterCommit(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_list_forecasts_deadlock.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_deadlock_test"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "死锁修复验证"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_dl_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "初始正史",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	fc := &domain.Forecast{
		ID:          "fc_dl_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧测试",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	// 追加一章（按计划执行——v2 指纹只覆盖框架+钩子身份，不再变化）
	ch2 := &domain.Chapter{
		ID:           "ch_dl_2",
		ProjectID:    projID,
		ChapterIndex: 2,
		Title:        "第二章正史归档",
		Content:      "正史已推进",
		CreatedAt:    time.Now(),
	}
	if _, err := s.CommitChapter(ctx, projID, ch2); err != nil {
		t.Fatalf("CommitChapter 2 failed: %v", err)
	}

	// 以 context.WithTimeout(3*time.Second) 调 ListForecasts
	listCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	list, err := s.ListForecasts(listCtx, projID)
	if err != nil {
		t.Fatalf("ListForecasts failed: %v", err)
	}
	if listCtx.Err() != nil {
		t.Fatalf("ListForecasts exceeded timeout deadline: %v", listCtx.Err())
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(list))
	}
	if list[0].Status != domain.ForecastStatusActive {
		t.Fatalf("expected forecast status to stay 'active' after in-plan chapter commit (v2), got '%s'", list[0].Status)
	}

	// 确认数据库中 project_active_plan 仍为 active（执行不 stale）
	var dbPlanStatus string
	if err := s.db.QueryRow(`SELECT status FROM project_active_plan WHERE project_id = ?`, projID).Scan(&dbPlanStatus); err != nil {
		t.Fatalf("query db plan status failed: %v", err)
	}
	if dbPlanStatus != string(domain.PlanStatusActive) {
		t.Fatalf("expected project_active_plan in db to stay 'active', got '%s'", dbPlanStatus)
	}

	// 断言关联 active plan 仍为 active
	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusActive {
		t.Fatalf("expected active plan status to stay 'active', got '%s'", activePlan.Status)
	}
}

// Case ⑥: CleanProject 无 stale 场景正常快速返回
func TestListForecasts_CleanProject(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_list_forecasts_clean.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_clean_test"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "干净项目测试"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. 无 forecast 项目
	listCtx1, cancel1 := context.WithTimeout(ctx, 3*time.Second)
	defer cancel1()
	list1, err := s.ListForecasts(listCtx1, projID)
	if err != nil {
		t.Fatalf("ListForecasts on empty project failed: %v", err)
	}
	if len(list1) != 0 {
		t.Fatalf("expected 0 forecasts, got %d", len(list1))
	}

	// 2. 有 active forecast 且指纹匹配（无 stale）
	ch1 := &domain.Chapter{
		ID:           "ch_clean_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "干净正史内容",
		CreatedAt:    time.Now(),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	fc := &domain.Forecast{
		ID:          "fc_clean_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "无偏离",
		BranchCount: 1,
		Horizon:     1,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "干净节拍"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	listCtx2, cancel2 := context.WithTimeout(ctx, 3*time.Second)
	defer cancel2()
	list2, err := s.ListForecasts(listCtx2, projID)
	if err != nil {
		t.Fatalf("ListForecasts with matching fingerprint failed: %v", err)
	}
	if len(list2) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(list2))
	}
	if list2[0].Status != domain.ForecastStatusActive {
		t.Fatalf("expected forecast status to be 'active', got '%s'", list2[0].Status)
	}
}

// TestSelectBranch_ArchivedForecastRejected verifies that selecting a branch on an archived
// forecast is rejected fail-closed, keeping the archived forecast archived and existing active forecast intact.
func TestSelectBranch_ArchivedForecastRejected(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_select_archived_rej.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_archived_rej_test"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "归档前瞻拒绝测试"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. Forecast A (active initially)
	fcA := &domain.Forecast{
		ID:          "fc_A",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧方案A",
		BranchCount: 2,
		Horizon:     2,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "branch_a1", BeatsSummary: "方案A节拍1"},
			{ID: "branch_a2", BeatsSummary: "方案A节拍2"},
		},
		CreatedAt: time.Now().Add(-10 * time.Minute),
	}
	if err := s.SaveForecast(ctx, fcA); err != nil {
		t.Fatalf("SaveForecast A failed: %v", err)
	}

	// 2. Forecast B (active, which archives A via SaveForecast single-active invariant)
	fcB := &domain.Forecast{
		ID:          "fc_B",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧方案B",
		BranchCount: 2,
		Horizon:     2,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "branch_b1", BeatsSummary: "方案B节拍1"},
			{ID: "branch_b2", BeatsSummary: "方案B节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fcB); err != nil {
		t.Fatalf("SaveForecast B failed: %v", err)
	}

	// Verify A is archived and B is active
	var statusA, statusB string
	if err := s.db.QueryRow(`SELECT status FROM narrative_forecasts WHERE id = ?`, fcA.ID).Scan(&statusA); err != nil {
		t.Fatalf("query status A failed: %v", err)
	}
	if err := s.db.QueryRow(`SELECT status FROM narrative_forecasts WHERE id = ?`, fcB.ID).Scan(&statusB); err != nil {
		t.Fatalf("query status B failed: %v", err)
	}
	if statusA != domain.ForecastStatusArchived || statusB != domain.ForecastStatusActive {
		t.Fatalf("precondition failed: statusA=%s (want %s), statusB=%s (want %s)",
			statusA, domain.ForecastStatusArchived, statusB, domain.ForecastStatusActive)
	}

	// 3. Attempt SelectBranch on archived forecast A -> MUST return error
	plan, err := s.SelectBranch(ctx, projID, fcA.ID, "branch_a1")
	if err == nil {
		t.Fatalf("expected error selecting branch on archived forecast, but got nil; plan=%+v", plan)
	}

	// 4. Assert A is STILL archived, B is STILL active
	if err := s.db.QueryRow(`SELECT status FROM narrative_forecasts WHERE id = ?`, fcA.ID).Scan(&statusA); err != nil {
		t.Fatalf("re-query status A failed: %v", err)
	}
	if err := s.db.QueryRow(`SELECT status FROM narrative_forecasts WHERE id = ?`, fcB.ID).Scan(&statusB); err != nil {
		t.Fatalf("re-query status B failed: %v", err)
	}
	if statusA != domain.ForecastStatusArchived {
		t.Errorf("status A corrupted: expected %s, got %s", domain.ForecastStatusArchived, statusA)
	}
	if statusB != domain.ForecastStatusActive {
		t.Errorf("status B corrupted: expected %s, got %s", domain.ForecastStatusActive, statusB)
	}

	// 5. Assert active plan was not set to A
	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan != nil && activePlan.ForecastID == fcA.ID {
		t.Errorf("active plan erroneously set to archived forecast A: %+v", activePlan)
	}
}

// TestSelectBranch_StaleForecastRejected verifies that selecting a branch on a stale forecast
// (due to canon fingerprint mismatch) is rejected fail-closed.
func TestSelectBranch_StaleForecastRejected(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_select_stale_rej.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_stale_rej_test"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "失效前瞻拒绝测试"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. Commit chapter 1 into canon
	ch1 := &domain.Chapter{
		ID:           "ch_stale_rej_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "初始正史内容",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	// 1b. Seed a hook BEFORE the forecast so the v2 fingerprint covers its identity
	hook := &domain.PlotHook{
		ID:             "hook_stale_rej_1",
		ProjectID:      projID,
		Title:          "玉佩之谜",
		Details:        "母亲遗物",
		CreatedChapter: 1,
		TargetChapter:  6,
		Status:         domain.HookStatusOpen,
		CreatedAt:      time.Now(),
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// 2. Create forecast matching chapter 1 fingerprint
	fc := &domain.Forecast{
		ID:          "fc_stale_rej",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧测试",
		BranchCount: 2,
		Horizon:     2,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "branch_s1", BeatsSummary: "节拍1"},
			{ID: "branch_s2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	// 3. Upstream rewrite: surgically delete the hook (hook-set surgery, NOT status flow)
	// to cause a v2 fingerprint mismatch. (v2: plain chapter commits are execution and stay active.)
	if err := s.DeletePlotHook(ctx, hook.ID); err != nil {
		t.Fatalf("DeletePlotHook failed: %v", err)
	}

	// 4. Attempt SelectBranch -> MUST return error because forecast is stale
	plan, err := s.SelectBranch(ctx, projID, fc.ID, "branch_s1")
	if err == nil {
		t.Fatalf("expected error selecting branch on stale forecast, but got nil; plan=%+v", plan)
	}

	// 5. Assert forecast status is marked stale in DB and not resurrected to active
	var dbStatus string
	if err := s.db.QueryRow(`SELECT status FROM narrative_forecasts WHERE id = ?`, fc.ID).Scan(&dbStatus); err != nil {
		t.Fatalf("query status failed: %v", err)
	}
	if dbStatus != domain.ForecastStatusStale {
		t.Errorf("expected forecast status to be 'stale', got '%s'", dbStatus)
	}

	// 6. Assert no active plan exists pointing to this forecast
	var planCount int
	if err := s.db.QueryRow(`SELECT count(*) FROM project_active_plan WHERE project_id = ? AND forecast_id = ? AND status = ?`,
		projID, fc.ID, domain.PlanStatusActive).Scan(&planCount); err != nil {
		t.Fatalf("query active plan count failed: %v", err)
	}
	if planCount != 0 {
		t.Errorf("expected 0 active plans for stale forecast, got %d", planCount)
	}
}

// TestSelectBranch_ActiveMaintainsSingleActive verifies that selecting a branch on an active forecast
// succeeds and strictly maintains the single-active invariant across the project.
func TestSelectBranch_ActiveMaintainsSingleActive(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_select_single_active.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_active_single_test"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "单一活跃前瞻验证"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. Commit chapter 1
	ch1 := &domain.Chapter{
		ID:           "ch_sa_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "正史内容",
		CreatedAt:    time.Now(),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// 2. Create forecast A
	fcA := &domain.Forecast{
		ID:          "fc_single_A",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "分歧方案A",
		BranchCount: 2,
		Horizon:     2,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "branch_a1", BeatsSummary: "节拍A1", ExpectedDeltas: "+神兵"},
			{ID: "branch_a2", BeatsSummary: "节拍A2", ExpectedDeltas: "+灵丹"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fcA); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	// 3. Select branch A1
	plan, err := s.SelectBranch(ctx, projID, fcA.ID, "branch_a1")
	if err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.ForecastID != fcA.ID || plan.BranchID != "branch_a1" {
		t.Errorf("unexpected plan: forecast=%s, branch=%s", plan.ForecastID, plan.BranchID)
	}
	if plan.Status != domain.PlanStatusActive {
		t.Errorf("expected plan status %s, got %s", domain.PlanStatusActive, plan.Status)
	}

	// 4. Assert exactly ONE active forecast exists in narrative_forecasts for this project
	var activeCount int
	if err := s.db.QueryRow(`
		SELECT count(*) 
		FROM narrative_forecasts 
		WHERE project_id = ? AND status = ?`,
		projID, domain.ForecastStatusActive,
	).Scan(&activeCount); err != nil {
		t.Fatalf("query active forecast count failed: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active forecast row for project, got %d", activeCount)
	}

	// 5. Assert active plan points to A
	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.ForecastID != fcA.ID || activePlan.BranchID != "branch_a1" {
		t.Errorf("expected active plan pointing to forecast %s / branch branch_a1, got forecast %s / branch %s",
			fcA.ID, activePlan.ForecastID, activePlan.BranchID)
	}
}

// v2: framework premise change is an upstream rewrite → forecast goes stale.
func TestSQLiteStore_FrameworkChange_MarksForecastStale(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_fw_stale.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_fw_stale"
	proj := &domain.Project{
		ID:         projID,
		Title:      "框架变更测试",
		WorldRules: "天道因果不可违逆",
		Framework:  &domain.ProjectFramework{ThemePremise: "天道因果循环"},
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_fw_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "初始正史内容",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	fc := &domain.Forecast{
		ID:          "fc_fw_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "框架分歧",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}
	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	loaded1, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast failed: %v", err)
	}
	if loaded1.Status != domain.ForecastStatusActive {
		t.Fatalf("expected active before framework change, got %s", loaded1.Status)
	}

	// Upstream rewrite: change one char of framework_json via the Project update path
	updated, err := s.GetProject(ctx, projID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	updated.Framework.ThemePremise = "天道因果循环改"
	if err := s.SaveProject(ctx, updated); err != nil {
		t.Fatalf("SaveProject (framework update) failed: %v", err)
	}

	loaded2, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast after framework change failed: %v", err)
	}
	if loaded2.Status != domain.ForecastStatusStale {
		t.Fatalf("expected forecast 'stale' after framework premise change, got '%s'", loaded2.Status)
	}

	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusStale {
		t.Fatalf("expected active plan 'stale', got '%s'", activePlan.Status)
	}
}

// v2: history rewritten below base (uncommit down past base) → stale via maxChapter < baseChapter.
func TestSQLiteStore_RewriteBelowBase_MarksForecastStale(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_belowbase_stale.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_belowbase_stale"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "基下改写测试"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	for i, id := range []string{"ch_bb_1", "ch_bb_2"} {
		ch := &domain.Chapter{
			ID:           id,
			ProjectID:    projID,
			ChapterIndex: i + 1,
			Title:        "正史章",
			Content:      "正史内容",
			CreatedAt:    time.Now().Add(time.Duration(i-2) * time.Hour),
		}
		if _, err := s.CommitChapter(ctx, projID, ch); err != nil {
			t.Fatalf("CommitChapter %d failed: %v", i+1, err)
		}
	}

	fc := &domain.Forecast{
		ID:          "fc_bb_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "基下分歧",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}
	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	// Uncommit only allows the latest chapter: retract ch2 first, then ch1.
	// base=1 and max drops to 0 < base → stale.
	if _, _, err := s.UncommitChapter(ctx, projID, 2); err != nil {
		t.Fatalf("UncommitChapter 2 failed: %v", err)
	}
	if _, _, err := s.UncommitChapter(ctx, projID, 1); err != nil {
		t.Fatalf("UncommitChapter 1 failed: %v", err)
	}

	loaded, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast after below-base rewrite failed: %v", err)
	}
	if loaded.Status != domain.ForecastStatusStale {
		t.Fatalf("expected forecast 'stale' after history rewritten below base, got '%s'", loaded.Status)
	}

	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusStale {
		t.Fatalf("expected active plan 'stale', got '%s'", activePlan.Status)
	}
}

// v2: hook-set surgery (delete) changes hook identities → stale.
func TestSQLiteStore_HookDeletion_MarksForecastStale(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_hookdel_stale.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_hookdel_stale"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "钩子删除测试"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_hd_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "初始正史内容",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	hook := &domain.PlotHook{
		ID:             "hook_hd_1",
		ProjectID:      projID,
		Title:          "玉佩之谜",
		Details:        "母亲遗物",
		CreatedChapter: 1,
		TargetChapter:  6,
		Status:         domain.HookStatusOpen,
		CreatedAt:      time.Now(),
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	fc := &domain.Forecast{
		ID:          "fc_hd_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "钩子分歧",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}
	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	// Hook-set surgery (distinct from status flow): delete the hook.
	if err := s.DeletePlotHook(ctx, hook.ID); err != nil {
		t.Fatalf("DeletePlotHook failed: %v", err)
	}

	loaded, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast after hook deletion failed: %v", err)
	}
	if loaded.Status != domain.ForecastStatusStale {
		t.Fatalf("expected forecast 'stale' after hook deletion, got '%s'", loaded.Status)
	}

	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusStale {
		t.Fatalf("expected active plan 'stale', got '%s'", activePlan.Status)
	}
}

// v2: hook STATUS flow (OPEN→RESOLVED) is execution, not surgery → stays active.
func TestSQLiteStore_HookStatusTransition_KeepsForecastActive(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_hookstatus_active.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_hookstatus_active"
	if err := s.SaveProject(ctx, &domain.Project{ID: projID, Title: "钩子流转测试"}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch_hs_1",
		ProjectID:    projID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "初始正史内容",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if _, err := s.CommitChapter(ctx, projID, ch1); err != nil {
		t.Fatalf("CommitChapter 1 failed: %v", err)
	}

	hook := &domain.PlotHook{
		ID:             "hook_hs_1",
		ProjectID:      projID,
		Title:          "玉佩之谜",
		Details:        "母亲遗物",
		CreatedChapter: 1,
		TargetChapter:  6,
		Status:         domain.HookStatusOpen,
		CreatedAt:      time.Now(),
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	fc := &domain.Forecast{
		ID:          "fc_hs_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "钩子分歧",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "节拍1"},
			{ID: "b2", BeatsSummary: "节拍2"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}
	if _, err := s.SelectBranch(ctx, projID, fc.ID, "b1"); err != nil {
		t.Fatalf("SelectBranch failed: %v", err)
	}

	// Status flow only: resolve the hook in place (identity unchanged).
	hook.Status = domain.HookStatusResolved
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook (resolve) failed: %v", err)
	}

	loaded, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast after hook resolve failed: %v", err)
	}
	if loaded.Status != domain.ForecastStatusActive {
		t.Fatalf("expected forecast to stay 'active' after hook status transition, got '%s'", loaded.Status)
	}

	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusActive {
		t.Fatalf("expected active plan to stay 'active', got '%s'", activePlan.Status)
	}
}

func TestSQLiteStore_ComputeFingerprint_FailsClosedOnQueryError(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_fp_query_err.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_fp_err"
	if err := s.SaveProject(ctx, &domain.Project{
		ID:        projID,
		Title:     "指纹报错测试",
		Framework: &domain.ProjectFramework{ThemePremise: "天道运行"},
	}); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	hook := &domain.PlotHook{
		ID:             "hook_fp_1",
		ProjectID:      projID,
		Title:          "神秘线索",
		CreatedChapter: 1,
		TargetChapter:  5,
		Status:         domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, hook); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}

	// 正常建库后取 fp1（两值形式）→ 无错且非空
	fp1, err := s.ComputeFingerprint(ctx, projID)
	if err != nil {
		t.Fatalf("expected initial ComputeFingerprint to succeed, got %v", err)
	}
	if fp1 == "" || !strings.HasPrefix(fp1, "v2:") {
		t.Fatalf("expected non-empty v2 fingerprint, got %q", fp1)
	}

	// DROP TABLE plot_hooks → ComputeFingerprint 返回 error
	if _, err := s.db.Exec(`DROP TABLE plot_hooks`); err != nil {
		t.Fatalf("DROP TABLE plot_hooks failed: %v", err)
	}
	fp2, err := s.ComputeFingerprint(ctx, projID)
	if err == nil {
		t.Fatalf("expected ComputeFingerprint to fail after DROP TABLE plot_hooks, got fp=%q", fp2)
	}

	// DROP TABLE projects → ComputeFingerprint 返回 error
	if _, err := s.db.Exec(`DROP TABLE projects`); err != nil {
		t.Fatalf("DROP TABLE projects failed: %v", err)
	}
	fp3, err := s.ComputeFingerprint(ctx, projID)
	if err == nil {
		t.Fatalf("expected ComputeFingerprint to fail after DROP TABLE projects, got fp=%q", fp3)
	}
}

func TestSQLiteStore_ForecastStaleWhenHookQueryBroken(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_hook_query_broken.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projID := "proj_hook_broken"
	proj := &domain.Project{
		ID:        projID,
		Title:     "钩子查询失败Fail-Closed测试",
		Framework: &domain.ProjectFramework{ThemePremise: "因果循环"},
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 零钩子 → 创建并 SaveForecast 一个 active forecast（存量指纹 = 同框架+零钩子的合法值）
	fc := &domain.Forecast{
		ID:          "fc_broken_1",
		ProjectID:   projID,
		BaseChapter: "0",
		Divergence:  "因果分歧",
		BranchCount: 1,
		Horizon:     2,
		Fingerprint: mustComputeFingerprint(t, s, ctx, projID),
		Status:      domain.ForecastStatusActive,
		Branches: []domain.ForecastBranch{
			{ID: "b1", BeatsSummary: "主干推进"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveForecast(ctx, fc); err != nil {
		t.Fatalf("SaveForecast failed: %v", err)
	}

	// 先断言：正常情况下 LoadForecast 不 stale（premise 未变）——这步是测试前提
	loadedBefore, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast before drop failed: %v", err)
	}
	if loadedBefore.Status != domain.ForecastStatusActive {
		t.Fatalf("expected forecast to be active before drop, got %s", loadedBefore.Status)
	}

	listBefore, err := s.ListForecasts(ctx, projID)
	if err != nil {
		t.Fatalf("ListForecasts before drop failed: %v", err)
	}
	if len(listBefore) != 1 || listBefore[0].Status != domain.ForecastStatusActive {
		t.Fatalf("expected list to have 1 active forecast before drop, got %+v", listBefore)
	}

	// DROP TABLE plot_hooks → LoadForecast 必须返回 status=STALE（查询失败 Fail-Closed），且不是 error；ListForecasts 同断言
	if _, err := s.db.Exec(`DROP TABLE plot_hooks`); err != nil {
		t.Fatalf("DROP TABLE plot_hooks failed: %v", err)
	}

	loadedAfter, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("expected LoadForecast not to error on broken hooks table, got %v", err)
	}
	if loadedAfter.Status != domain.ForecastStatusStale {
		t.Fatalf("expected LoadForecast status STALE, got %s", loadedAfter.Status)
	}

	// 重置回 active，断言 ListForecasts 同样能触发 Fail-Closed 判 stale
	if _, err := s.db.Exec(`UPDATE narrative_forecasts SET status = ? WHERE id = ?`, domain.ForecastStatusActive, fc.ID); err != nil {
		t.Fatalf("reset status failed: %v", err)
	}
	listAfter, err := s.ListForecasts(ctx, projID)
	if err != nil {
		t.Fatalf("expected ListForecasts not to error on broken hooks table, got %v", err)
	}
	if len(listAfter) != 1 {
		t.Fatalf("expected 1 forecast in list, got %d", len(listAfter))
	}
	if listAfter[0].Status != domain.ForecastStatusStale {
		t.Fatalf("expected ListForecasts status STALE, got %s", listAfter[0].Status)
	}
}
