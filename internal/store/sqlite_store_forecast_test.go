package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

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
		Fingerprint: s.ComputeFingerprint(ctx, projID),
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

// Case ③: 新章归档后旧 forecast 读出 stale
func TestSQLiteStore_FingerprintMismatch_MarksForecastStale(t *testing.T) {
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
		Fingerprint: s.ComputeFingerprint(ctx, projID),
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

	// Reading forecast now MUST evaluate fingerprint mismatch and read out as stale
	loaded2, err := s.LoadForecast(ctx, projID, fc.ID)
	if err != nil {
		t.Fatalf("LoadForecast after chapter archive failed: %v", err)
	}
	if loaded2.Status != domain.ForecastStatusStale {
		t.Fatalf("expected forecast status to be 'stale' after new chapter archive, got '%s'", loaded2.Status)
	}

	// Active plan must also be updated to stale
	activePlan, err := s.GetActivePlan(ctx, projID)
	if err != nil {
		t.Fatalf("GetActivePlan failed: %v", err)
	}
	if activePlan == nil {
		t.Fatal("expected non-nil active plan")
	}
	if activePlan.Status != domain.PlanStatusStale {
		t.Fatalf("expected active plan status to be 'stale', got '%s'", activePlan.Status)
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

	// Forecast covering chapter 2 and chapter 3
	fc := &domain.Forecast{
		ID:          "fc_consume_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "下山历练路线",
		BranchCount: 2,
		Horizon:     2, // Covers chapters 2 and 3
		Fingerprint: s.ComputeFingerprint(ctx, projID),
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

	fc := &domain.Forecast{
		ID:          "fc_div_1",
		ProjectID:   projID,
		BaseChapter: "1",
		Divergence:  "斩杀强敌还是结盟",
		BranchCount: 2,
		Horizon:     3,
		Fingerprint: s.ComputeFingerprint(ctx, projID),
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
