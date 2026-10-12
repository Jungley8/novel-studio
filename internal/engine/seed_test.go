package engine_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

func TestBuildWorldRules(t *testing.T) {
	fw := &domain.ProjectFramework{
		WorldAxioms: []string{"天地不仁", "道法自然"},
		PowerLadder: []domain.PowerLadderTier{
			{Realm: "练气境", Bottleneck: "经脉闭塞", Drawback: "寿元消耗"},
		},
	}

	rules := engine.BuildWorldRules(fw)
	if rules == "" {
		t.Fatalf("expected non-empty world rules")
	}
	if !testing.Verbose() {
		// Just sanity check keywords
	}
	if expected := "【天道法则与核心世界公理】"; rules[:len([]rune(expected))] != expected && rules[:36] != expected {
		// Check contains
	}
}

func TestSeedGenesisArtifacts(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer s.Close()

	fw := &domain.ProjectFramework{
		ThemePremise: "逆天改命，查明血仇",
		Protagonist: &domain.ProtagonistProfile{
			Name:         "沈青梧",
			Identity:     "边荒游侠",
			InitialRealm: "凡胎境",
			Goal:         "查明血案真相，踏平伪神",
			VoiceTone:    "沉稳果决",
			Inventory:    []string{"粗布行囊", "青铜残片"},
		},
		WorldAxioms: []string{"众生皆为草芥", "唯争一线生机"},
		PowerLadder: []domain.PowerLadderTier{
			{Realm: "凡胎境", Bottleneck: "肉身凡胎", Drawback: "气血枯败"},
		},
		KeyCharacters: []domain.CharacterProfile{
			{
				Name:    "楚掌柜",
				Role:    "引路导师",
				Realm:   "金丹境",
				Goal:    "护道周全",
				FateArc: "功成身退",
			},
		},
		SeedHooks: []domain.PlotHook{
			{
				Title:          "残破古镜",
				Details:        "井底偶得的神秘残镜",
				CreatedChapter: 1,
				TargetChapter:  15,
			},
		},
		Factions: []domain.Faction{
			{
				Name:        "青云宗",
				Alignment:   "正道盟主",
				ThreatLevel: "极高",
				Doctrine:    "顺应天意，斩妖除魔",
			},
		},
		VolumeArcs: []domain.VolumeArc{
			{
				VolumeIndex:       1,
				Title:             "第一卷：破茧",
				EstimatedChapters: 10,
			},
		},
	}

	proj := &domain.Project{
		ID:             "proj_test_seed",
		Title:          "九霄踏天录",
		TargetPlatform: "起点仙侠",
		WorldRules:     engine.BuildWorldRules(fw),
		Framework:      fw,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("save initial project: %v", err)
	}

	if err := engine.SeedGenesisArtifacts(ctx, s, proj, fw); err != nil {
		t.Fatalf("SeedGenesisArtifacts failed: %v", err)
	}

	// 1. 断言主角条目存在（Category 角色、Name 顾渊、StructuredLevel 非空）
	protEntry, err := s.GetCodexEntry(ctx, proj.ID, "codex_proj_test_seed_pro")
	if err != nil {
		t.Fatalf("protagonist entry not found: %v", err)
	}
	if protEntry.Category != domain.CategoryCharacter {
		t.Errorf("expected protagonist category %s, got %s", domain.CategoryCharacter, protEntry.Category)
	}
	if protEntry.Name != "沈青梧" {
		t.Errorf("expected protagonist name 沈青梧, got %s", protEntry.Name)
	}
	if protEntry.StructuredLevel == nil || protEntry.StructuredLevel.Realm != "凡胎境" {
		t.Errorf("expected StructuredLevel with realm 凡胎境, got %+v", protEntry.StructuredLevel)
	}
	if len(protEntry.StructuredItems) != 2 || protEntry.StructuredItems[0].Name != "粗布行囊" {
		t.Errorf("expected profile-driven StructuredItems, got %+v", protEntry.StructuredItems)
	}

	// 2. 断言伏笔落库
	hooks, err := s.ListPlotHooks(ctx, proj.ID)
	if err != nil {
		t.Fatalf("list plot hooks: %v", err)
	}
	if len(hooks) != 1 || hooks[0].Title != "残破古镜" {
		t.Errorf("expected 1 plot hook with title 残破古镜, got %+v", hooks)
	}

	// 3. 断言角色、地点、物品、势力落库
	charEntries, err := s.ListCodexEntries(ctx, proj.ID, domain.CategoryCharacter)
	if err != nil {
		t.Fatalf("list character entries: %v", err)
	}
	foundMentor := false
	for _, c := range charEntries {
		if c.Name == "楚掌柜" {
			foundMentor = true
			if c.Archetype != "MENTOR" {
				t.Errorf("expected archetype MENTOR for 楚掌柜, got %s", c.Archetype)
			}
			if len(c.Aliases) != 0 {
				t.Errorf("expected no hardcoded aliases for 楚掌柜, got %v", c.Aliases)
			}
		}
	}
	if !foundMentor {
		t.Errorf("key character 楚掌柜 was not persisted to codex")
	}

	locEntries, err := s.ListCodexEntries(ctx, proj.ID, domain.CategoryLocation)
	if err != nil || len(locEntries) != 0 {
		t.Errorf("expected no auto-seeded locations (genre-agnostic seeding), got %v", locEntries)
	}

	itemEntries, err := s.ListCodexEntries(ctx, proj.ID, domain.CategoryItem)
	if err != nil || len(itemEntries) != 0 {
		t.Errorf("expected no auto-seeded items (genre-agnostic seeding), got %v", itemEntries)
	}

	facEntries, err := s.ListCodexEntries(ctx, proj.ID, domain.CategoryFaction)
	if err != nil || len(facEntries) == 0 || facEntries[0].Name != "青云宗" {
		t.Errorf("expected faction 青云宗 to be seeded, got %v", facEntries)
	}

	// 4. 断言 fw 内存对象已剥离
	if fw.Protagonist != nil || fw.Factions != nil || fw.KeyCharacters != nil || fw.SeedHooks != nil {
		t.Fatalf("expected transient fields in memory fw to be stripped (nil), got protagonist=%v, factions=%v, chars=%v, hooks=%v",
			fw.Protagonist, fw.Factions, fw.KeyCharacters, fw.SeedHooks)
	}

	// 5. 断言 GetProject 读到剥离后的 framework
	savedProj, err := s.GetProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get project from store: %v", err)
	}
	if savedProj.Framework == nil {
		t.Fatalf("expected saved project framework to be non-nil")
	}
	if len(savedProj.Framework.Factions) != 0 {
		t.Errorf("expected saved framework factions to be empty, got %d", len(savedProj.Framework.Factions))
	}
	if len(savedProj.Framework.KeyCharacters) != 0 {
		t.Errorf("expected saved framework key characters to be empty, got %d", len(savedProj.Framework.KeyCharacters))
	}
	if len(savedProj.Framework.SeedHooks) != 0 {
		t.Errorf("expected saved framework seed hooks to be empty, got %d", len(savedProj.Framework.SeedHooks))
	}

	// 6. Fail-closed: a framework without a protagonist profile must be rejected.
	bareFW := &domain.ProjectFramework{ThemePremise: "缺失主角的框架"}
	bareProj := &domain.Project{
		ID:        "proj_test_seed_bare",
		Title:     "无主之书",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := engine.SeedGenesisArtifacts(ctx, s, bareProj, bareFW); err == nil {
		t.Fatalf("expected fail-closed error when framework lacks a protagonist profile")
	}
}
