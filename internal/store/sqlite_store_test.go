package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
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
	if updatedProj.Protagonist.Inventory != "粗布衣, 洗髓丹" {
		t.Errorf("unexpected inventory: %s", updatedProj.Protagonist.Inventory)
	}
	if len(updatedProj.Protagonist.StructuredItems) != 2 || updatedProj.Protagonist.StructuredItems[1].Quantity != 1 {
		t.Errorf("unexpected structured items: %+v", updatedProj.Protagonist.StructuredItems)
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

func TestSQLiteStore_CodexCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "novel_codex.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// 1. Create Project
	proj := &domain.Project{
		ID:             "p-codex-test",
		Title:          "修仙世界",
		TargetPlatform: "起点仙侠",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 2. Save Character Entry with Aliases
	chara := &domain.CodexEntry{
		ID:              "c-chufeng",
		ProjectID:       proj.ID,
		Category:        domain.CategoryCharacter,
		Name:            "楚枫",
		Aliases:         []string{"疯子楚", "白衣修罗", "楚长老"},
		ColorTag:        "#f59e0b",
		Summary:         "青云门外门弟子，性格坚毅果决",
		DetailsMarkdown: "身怀修罗血脉，擅长弑神刀法。",
		TrackingMode:    domain.TrackingModeAutoMention,
	}
	if err := s.SaveCodexEntry(ctx, chara); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}

	// 3. Save Enemy Character
	enemy := &domain.CodexEntry{
		ID:           "c-zhao",
		ProjectID:    proj.ID,
		Category:     domain.CategoryCharacter,
		Name:         "赵天霸",
		Aliases:      []string{"赵爷", "霸刀"},
		Summary:      "黑风寨大当家",
		TrackingMode: domain.TrackingModeAutoMention,
	}
	if err := s.SaveCodexEntry(ctx, enemy); err != nil {
		t.Fatalf("SaveCodexEntry enemy failed: %v", err)
	}

	// 4. Save Location Entry
	loc := &domain.CodexEntry{
		ID:           "l-danfang",
		ProjectID:    proj.ID,
		Category:     domain.CategoryLocation,
		Name:         "废丹房",
		Summary:      "炼丹峰地脉深处的废弃库房",
		TrackingMode: domain.TrackingModeAutoMention,
	}
	if err := s.SaveCodexEntry(ctx, loc); err != nil {
		t.Fatalf("SaveCodexEntry loc failed: %v", err)
	}

	// 5. List Codex Entries
	allEntries, err := s.ListCodexEntries(ctx, proj.ID, "")
	if err != nil {
		t.Fatalf("ListCodexEntries failed: %v", err)
	}
	if len(allEntries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(allEntries))
	}

	charEntries, err := s.ListCodexEntries(ctx, proj.ID, domain.CategoryCharacter)
	if err != nil {
		t.Fatalf("ListCodexEntries category failed: %v", err)
	}
	if len(charEntries) != 2 {
		t.Fatalf("expected 2 character entries, got %d", len(charEntries))
	}

	// 6. Get Entry & check aliases
	gotChara, err := s.GetCodexEntry(ctx, proj.ID, "c-chufeng")
	if err != nil {
		t.Fatalf("GetCodexEntry failed: %v", err)
	}
	if gotChara == nil || gotChara.Name != "楚枫" {
		t.Fatalf("expected character 楚枫, got %+v", gotChara)
	}
	if len(gotChara.Aliases) != 3 {
		t.Errorf("expected 3 aliases, got %d: %v", len(gotChara.Aliases), gotChara.Aliases)
	}

	// 7. Progressions
	p1 := &domain.Progression{
		ID:                "prog-1",
		ActiveFromChapter: 1,
		StatePayloadJSON:  `{"realm":"炼气三层","status":"外门废柴"}`,
		Notes:             "开局弱小",
	}
	p2 := &domain.Progression{
		ID:                "prog-2",
		ActiveFromChapter: 10,
		StatePayloadJSON:  `{"realm":"筑基初期","status":"内门真传"}`,
		Notes:             "宗门大比夺魁",
	}
	if err := s.SaveCodexProgression(ctx, "c-chufeng", p1); err != nil {
		t.Fatalf("SaveCodexProgression 1 failed: %v", err)
	}
	if err := s.SaveCodexProgression(ctx, "c-chufeng", p2); err != nil {
		t.Fatalf("SaveCodexProgression 2 failed: %v", err)
	}

	progs, err := s.ListCodexProgressions(ctx, "c-chufeng")
	if err != nil {
		t.Fatalf("ListCodexProgressions failed: %v", err)
	}
	if len(progs) != 2 {
		t.Fatalf("expected 2 progressions, got %d", len(progs))
	}

	// 8. Relations
	rel := &domain.EntityRelation{
		ID:            "rel-chufeng-zhao",
		ProjectID:     proj.ID,
		SourceEntryID: "c-chufeng",
		TargetEntryID: "c-zhao",
		RelationType:  "NEMESIS",
		Description:   "灭族之仇，不死不休",
	}
	if err := s.SaveCodexRelation(ctx, proj.ID, rel); err != nil {
		t.Fatalf("SaveCodexRelation failed: %v", err)
	}

	rels, err := s.ListCodexRelations(ctx, proj.ID, "c-chufeng")
	if err != nil {
		t.Fatalf("ListCodexRelations failed: %v", err)
	}
	if len(rels) != 1 || rels[0].TargetName != "赵天霸" {
		t.Fatalf("expected relation with target 赵天霸, got %+v", rels)
	}

	// 9. Re-fetch character and verify all populated
	fullChara, err := s.GetCodexEntry(ctx, proj.ID, "c-chufeng")
	if err != nil {
		t.Fatalf("GetCodexEntry full failed: %v", err)
	}
	if len(fullChara.Progressions) != 2 {
		t.Errorf("expected 2 progressions on entry, got %d", len(fullChara.Progressions))
	}
	if len(fullChara.Relations) != 1 {
		t.Errorf("expected 1 relation on entry, got %d", len(fullChara.Relations))
	}

	// 10. Delete relation & entry
	if err := s.DeleteCodexRelation(ctx, proj.ID, "rel-chufeng-zhao"); err != nil {
		t.Fatalf("DeleteCodexRelation failed: %v", err)
	}
	if err := s.DeleteCodexEntry(ctx, proj.ID, "c-chufeng"); err != nil {
		t.Fatalf("DeleteCodexEntry failed: %v", err)
	}
	delCheck, err := s.GetCodexEntry(ctx, proj.ID, "c-chufeng")
	if err != nil || delCheck != nil {
		t.Errorf("expected nil after delete, got %v, err: %v", delCheck, err)
	}
}

func TestSQLiteStore_MatrixAndScenes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "novel_matrix.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// 1. Create Project with Framework VolumeArcs
	proj := &domain.Project{
		ID:             "p-matrix-test",
		Title:          "大周仙吏",
		TargetPlatform: "起点仙侠",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{VolumeIndex: 1, Title: "初入京城", EstimatedChapters: 10, CoreGoal: "立足"},
				{VolumeIndex: 2, Title: "妖市风云", EstimatedChapters: 10, CoreGoal: "破案"},
			},
		},
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 2. Commit chapter 1
	ch1 := &domain.Chapter{
		ID:           "ch-1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章 异界醒来",
		WordCount:    2000,
	}
	if err := s.SaveChapter(ctx, ch1); err != nil {
		t.Fatalf("SaveChapter failed: %v", err)
	}

	// 3. Save Scene 1.1 & 1.2
	sc1 := &domain.Scene{
		ID:              "sc-1-1",
		ChapterID:       ch1.ID,
		ProjectID:       proj.ID,
		SceneIndex:      1,
		Title:           "破庙避雨",
		DramaticGoal:    "寻找食物并确认身份",
		ConflictBarrier: "暴雨封路且有山匪出没",
		TensionLevel:    4,
		ProseContent:    "暴雨如注，破庙漏水...",
		WordCount:       1200,
	}
	sc2 := &domain.Scene{
		ID:              "sc-1-2",
		ChapterID:       ch1.ID,
		ProjectID:       proj.ID,
		SceneIndex:      2,
		Title:           "遭遇捕快",
		DramaticGoal:    "隐瞒来历并借机进城",
		ConflictBarrier: "被盘问路引户籍",
		TensionLevel:    7,
		ProseContent:    "脚步声由远及近...",
		WordCount:       800,
	}
	if err := s.SaveScene(ctx, sc1); err != nil {
		t.Fatalf("SaveScene 1 failed: %v", err)
	}
	if err := s.SaveScene(ctx, sc2); err != nil {
		t.Fatalf("SaveScene 2 failed: %v", err)
	}

	// 4. Save Marker
	marker := &domain.SceneMarker{
		ID:             "m-1",
		SceneID:        sc1.ID,
		MarkerType:     domain.MarkerTypeTodo,
		Color:          "#f59e0b",
		TextRangeStart: 5,
		TextRangeEnd:   15,
		Content:        "补充泥泞地面的气味描写",
	}
	if err := s.SaveSceneMarker(ctx, marker); err != nil {
		t.Fatalf("SaveSceneMarker failed: %v", err)
	}

	// 5. Get Scene & Markers
	gotScene, err := s.GetScene(ctx, sc1.ID)
	if err != nil || gotScene == nil {
		t.Fatalf("GetScene failed: %v", err)
	}
	if len(gotScene.Markers) != 1 {
		t.Fatalf("expected 1 marker on scene, got %d", len(gotScene.Markers))
	}

	// 6. List Scenes for chapter
	scenes, err := s.ListScenes(ctx, ch1.ID)
	if err != nil {
		t.Fatalf("ListScenes failed: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	// 7. Get Matrix Overview
	overview, err := s.GetMatrixOverview(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetMatrixOverview failed: %v", err)
	}
	if overview.TotalScenes != 2 {
		t.Errorf("expected 2 total scenes in matrix, got %d", overview.TotalScenes)
	}
	if len(overview.Volumes) != 2 {
		t.Errorf("expected 2 volume groups, got %d", len(overview.Volumes))
	}
	if len(overview.Volumes[0].Chapters) != 1 {
		t.Errorf("expected chapter 1 in volume 1, got %d", len(overview.Volumes[0].Chapters))
	}
	if overview.Volumes[0].AvgTension != 5.5 { // (4 + 7)/2 = 5.5
		t.Errorf("expected avg tension 5.5, got %f", overview.Volumes[0].AvgTension)
	}
}

func TestSQLiteStore_MultiCharacterMutationCommit(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_multi_char.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	// 1. Create project with protagonist
	proj := &domain.Project{
		ID:             "proj_cast",
		Title:          "凡人弑神录",
		TargetPlatform: "起点仙侠",
		WorldRules:     "神明不可直视，凡骨亦可弑神",
		Protagonist: domain.Protagonist{
			NameAndLevel: "顾渊 (凡人胎骨)",
			Inventory:    "凡骨残印x1",
			CoreGoal:     "隐忍查明三千年前宗门覆灭真相",
		},
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 2. Create Cast Characters in Codex
	char1 := &domain.CodexEntry{
		ID:                 "char_chu",
		ProjectID:          proj.ID,
		Category:           domain.CategoryCharacter,
		Name:               "楚掌柜",
		Aliases:            []string{"白衣修罗"},
		Summary:            "聚仙楼暗桩掌柜，实为魔修潜伏者",
		Archetype:          "ANTAGONIST",
		VoiceTone:          "阴鸷寡言，常带讥讽冷笑",
		CoreMotivation:     "搜集修士精血向魔尊邀功",
		CurrentDisposition: "WARY",
		TrackingMode:       domain.TrackingModeAlwaysInject,
	}
	if err := s.SaveCodexEntry(ctx, char1); err != nil {
		t.Fatalf("SaveCodexEntry char1 failed: %v", err)
	}

	char2 := &domain.CodexEntry{
		ID:                 "char_liu",
		ProjectID:          proj.ID,
		Category:           domain.CategoryCharacter,
		Name:               "柳依依",
		Aliases:            []string{"依依师妹"},
		Summary:            "同门暗哨，精通符箓易容",
		Archetype:          "DEUTERAGONIST",
		VoiceTone:          "娇憨机敏，语速极快",
		CoreMotivation:     "寻找失踪的亲兄",
		CurrentDisposition: "FRIENDLY",
		TrackingMode:       domain.TrackingModeAutoMention,
	}
	if err := s.SaveCodexEntry(ctx, char2); err != nil {
		t.Fatalf("SaveCodexEntry char2 failed: %v", err)
	}

	// 3. Commit Chapter with both Protagonist mutation and Cast mutations
	ch := &domain.Chapter{
		ID:           "ch_cast_1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "风雪聚仙楼",
		CoreConflict: "主角暗中破译接头密信，识破楚掌柜伪装并反手设伏",
		Beats: []domain.SceneBeat{
			{Phase: "蓄力压迫", Tension: 5, Action: "楚掌柜斟酒逼迫主角喝下毒酒", ExpectationBroken: "主角不动声色以残印吸纳剧毒"},
			{Phase: "绝地反转", Tension: 9, Action: "主角暴起斩断楚掌柜右臂", ExpectationBroken: "楚掌柜惊恐发现主角并非凡人"},
		},
		StateMutation: domain.StateMutation{
			InventoryDelta: "+百年寒铁令x1",
			PowerDelta:     "觉醒凡骨神纹",
			CharacterMutations: []domain.CharacterMutation{
				{
					Name:          "白衣修罗", // Matched via Alias!
					StatusDelta:   "右臂被斩断，狼狈遁入密道",
					RelationDelta: "HOSTILE",
				},
				{
					Name:          "柳依依", // Matched via Name!
					StatusDelta:   "伏击暗处目睹主角神力，震撼归心",
					RelationDelta: "DEVOTED",
				},
			},
		},
		Content: "寒夜风雪呼啸。聚仙楼二楼雅间内，楚掌柜端着酒盏，眼神冰冷...",
	}

	updatedProj, err := s.CommitChapter(ctx, proj.ID, ch)
	if err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// 4. Verify Protagonist update
	if !strings.Contains(updatedProj.Protagonist.Inventory, "百年寒铁令") {
		t.Errorf("expected protagonist inventory to have 百年寒铁令, got %s", updatedProj.Protagonist.Inventory)
	}
	if !strings.Contains(updatedProj.Protagonist.NameAndLevel, "觉醒凡骨神纹") {
		t.Errorf("expected protagonist power update, got %s", updatedProj.Protagonist.NameAndLevel)
	}

	// 5. Verify Character 1 (楚掌柜) Progression & Disposition via alias match
	gotChar1, err := s.GetCodexEntry(ctx, proj.ID, char1.ID)
	if err != nil {
		t.Fatalf("GetCodexEntry char1 failed: %v", err)
	}
	if gotChar1.CurrentDisposition != "HOSTILE" {
		t.Errorf("expected 楚掌柜 disposition HOSTILE, got %s", gotChar1.CurrentDisposition)
	}
	if len(gotChar1.Progressions) != 1 {
		t.Fatalf("expected 1 progression for 楚掌柜, got %d", len(gotChar1.Progressions))
	}
	if !strings.Contains(gotChar1.Progressions[0].Notes, "右臂被斩断") {
		t.Errorf("expected progression notes to contain 右臂被斩断, got %s", gotChar1.Progressions[0].Notes)
	}

	// 6. Verify Character 2 (柳依依) Progression & Disposition via name match
	gotChar2, err := s.GetCodexEntry(ctx, proj.ID, char2.ID)
	if err != nil {
		t.Fatalf("GetCodexEntry char2 failed: %v", err)
	}
	if gotChar2.CurrentDisposition != "DEVOTED" {
		t.Errorf("expected 柳依依 disposition DEVOTED, got %s", gotChar2.CurrentDisposition)
	}
	if len(gotChar2.Progressions) != 1 {
		t.Fatalf("expected 1 progression for 柳依依, got %d", len(gotChar2.Progressions))
	}
	if !strings.Contains(gotChar2.Progressions[0].Notes, "震撼归心") {
		t.Errorf("expected progression notes to contain 震撼归心, got %s", gotChar2.Progressions[0].Notes)
	}
}

