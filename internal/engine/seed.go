package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// GenesisSeedStore represents the storage capabilities needed to persist genesis seed artifacts.
type GenesisSeedStore interface {
	SaveCodexEntry(ctx context.Context, entry *domain.CodexEntry) error
	SavePlotHook(ctx context.Context, h *domain.PlotHook) error
	SaveProject(ctx context.Context, p *domain.Project) error
}

// BuildWorldRules formats world axioms and power ladder tiers into a consolidated markdown string.
func BuildWorldRules(fw *domain.ProjectFramework) string {
	if fw == nil {
		return ""
	}
	var worldRules strings.Builder
	worldRules.WriteString("【天道法则与核心世界公理】\n")
	for i, axiom := range fw.WorldAxioms {
		worldRules.WriteString(fmt.Sprintf("%d. %s\n", i+1, axiom))
	}
	if len(fw.PowerLadder) > 0 {
		worldRules.WriteString("\n【战力阶梯与突破反噬】\n")
		for _, tier := range fw.PowerLadder {
			worldRules.WriteString(fmt.Sprintf("- %s: 关隘[%s] | 代价[%s]\n", tier.Realm, tier.Bottleneck, tier.Drawback))
		}
	}
	return strings.TrimSpace(worldRules.String())
}

// SeedGenesisArtifacts writes initial codex entries (protagonist, cast, locations, items, factions)
// and plot hooks into the store, then strips transient seed fields from the framework to maintain
// a single source of truth in the store.
func SeedGenesisArtifacts(ctx context.Context, store GenesisSeedStore, proj *domain.Project, fw *domain.ProjectFramework) error {
	if proj == nil || fw == nil {
		return nil
	}

	initialRealm := "凡胎境"
	if len(fw.PowerLadder) > 0 {
		initialRealm = fw.PowerLadder[0].Realm
	}

	protagonistName := "顾渊"
	protagonistGoal := fw.ThemePremise
	if protagonistGoal == "" {
		protagonistGoal = "查明宗门覆灭真相，逆伐伪神"
	}

	projectID := proj.ID

	// 0. Seed Protagonist into The Codex
	protEntry := &domain.CodexEntry{
		ID:                 fmt.Sprintf("codex_%s_pro", projectID),
		ProjectID:          projectID,
		Category:           domain.CategoryCharacter,
		Name:               protagonistName,
		ColorTag:           "#f59e0b",
		Summary:            fmt.Sprintf("全书核心主角 | 境界: %s | 动机: %s", initialRealm, protagonistGoal),
		DetailsMarkdown:    fmt.Sprintf("### %s (全书核心主角)\n- **初始境界**: %s\n- **核心动机**: %s\n- **随身底牌**: 凡骨铁印x1, 粗布短褐x1, 引路符x1\n- **初始状态**: 良好\n", protagonistName, initialRealm, protagonistGoal),
		Archetype:          "PROTAGONIST",
		VoiceTone:          "内敛隐忍，遇死地杀伐果决",
		CoreMotivation:     protagonistGoal,
		CurrentDisposition: "FRIENDLY",
		TrackingMode:       domain.TrackingModeAlwaysInject,
		Aliases:            []string{"主角", protagonistName, fmt.Sprintf("%s (%s)", protagonistName, initialRealm)},
		Realm:              initialRealm,
		Inventory:          "凡骨铁印x1, 粗布短褐x1, 引路符x1",
		HealthStatus:       "良好",
		StructuredLevel: &domain.PowerLevel{
			Realm: initialRealm,
			History: []domain.LevelTransition{
				{
					FromRealm: "凡俗白身",
					ToRealm:   initialRealm,
					Chapter:   0,
					Reason:    "创世开局设定",
					Timestamp: time.Now(),
				},
			},
		},
		StructuredItems: []domain.InventoryItem{
			{Name: "凡骨铁印", Quantity: 1, Quality: "上古不灭残印", AcquiredAt: 0},
			{Name: "粗布短褐", Quantity: 1, Quality: "随身布衣", AcquiredAt: 0},
			{Name: "引路符", Quantity: 1, Quality: "低阶符箓", AcquiredAt: 0},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = store.SaveCodexEntry(ctx, protEntry)

	// 1. Seed plot hooks
	for i, sh := range fw.SeedHooks {
		hook := &domain.PlotHook{
			ID:             fmt.Sprintf("hook_%s_%d", projectID, i+1),
			ProjectID:      projectID,
			Title:          sh.Title,
			Details:        sh.Details,
			CreatedChapter: sh.CreatedChapter,
			TargetChapter:  sh.TargetChapter,
			Status:         domain.HookStatusOpen,
			CreatedAt:      time.Now(),
		}
		_ = store.SavePlotHook(ctx, hook)
	}

	// 2. Seed key cast characters into The Codex
	for i, kc := range fw.KeyCharacters {
		archetype := "SUPPORTING"
		disposition := "NEUTRAL"
		voiceTone := "言简意赅，语带锋芒"
		colorTag := "#10b981"

		if strings.Contains(kc.Role, "宿敌") || strings.Contains(kc.Role, "魔") || strings.Contains(kc.Role, "反派") {
			archetype = "ANTAGONIST"
			disposition = "HOSTILE"
			voiceTone = "言语极轻尖细，像钝刀刮生锈铁皮，伪善转为狰狞"
			colorTag = "#f43f5e"
		} else if strings.Contains(kc.Role, "搭档") || strings.Contains(kc.Role, "同盟") || strings.Contains(kc.Role, "女配") {
			archetype = "DEUTERAGONIST"
			disposition = "WARY"
			voiceTone = "灵动中藏着敏锐算计，惊骇时语带轻颤"
			colorTag = "#8b5cf6"
		} else if strings.Contains(kc.Role, "导师") || strings.Contains(kc.Role, "领路") {
			archetype = "MENTOR"
			disposition = "FRIENDLY"
			voiceTone = "沧桑威严，如老僧入定"
			colorTag = "#3b82f6"
		}

		aliases := []string{}
		if kc.Name == "楚掌柜" {
			aliases = []string{"疯子楚", "白衣修罗"}
		} else if kc.Name == "柳依依" {
			aliases = []string{"小师妹", "柳姑娘"}
		}

		entry := &domain.CodexEntry{
			ID:                 fmt.Sprintf("codex_%s_char_%d", projectID, i+1),
			ProjectID:          projectID,
			Category:           domain.CategoryCharacter,
			Name:               kc.Name,
			ColorTag:           colorTag,
			Summary:            fmt.Sprintf("%s (%s), 境界: %s", kc.Role, kc.FateArc, kc.Realm),
			DetailsMarkdown:    fmt.Sprintf("### %s\n- **戏剧定位**: %s\n- **修为境界**: %s\n- **核心动机**: %s\n- **宿命轨迹**: %s\n", kc.Name, kc.Role, kc.Realm, kc.Goal, kc.FateArc),
			Archetype:          archetype,
			VoiceTone:          voiceTone,
			CoreMotivation:     kc.Goal,
			CurrentDisposition: disposition,
			TrackingMode:       domain.TrackingModeAutoMention,
			Aliases:            aliases,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		_ = store.SaveCodexEntry(ctx, entry)
	}

	// 3. Seed Location & Item into The Codex
	_ = store.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:              fmt.Sprintf("codex_%s_loc_1", projectID),
		ProjectID:       projectID,
		Category:        domain.CategoryLocation,
		Name:            "聚仙楼",
		ColorTag:        "#eab308",
		Summary:         "风雪隘口残破茶肆客栈，黄绫帘子破损，暗伏化龙暗哨与机关地道",
		DetailsMarkdown: "北地官道上唯一的歇脚茶棚客栈，内设通天阁暗桩地道，是第一卷杀局起点。",
		TrackingMode:    domain.TrackingModeAutoMention,
		Aliases:         []string{"风雪茶肆", "聚仙客栈"},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	})
	_ = store.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:              fmt.Sprintf("codex_%s_item_1", projectID),
		ProjectID:       projectID,
		Category:        domain.CategoryItem,
		Name:            "凡骨残印",
		ColorTag:        "#06b6d4",
		Summary:         "主角胸口深处的上古神纹残印，可化解魔毒，吞噬神明诅咒",
		DetailsMarkdown: "上古宗门覆灭遗留的不灭凡铁残印，唯有凡人无灵根胎骨可驭，遇魔毒神威自显。",
		TrackingMode:    domain.TrackingModeAutoMention,
		Aliases:         []string{"残骨铁印", "凡骨铁印"},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	})

	// 4. Seed Factions into The Codex
	for i, f := range fw.Factions {
		if strings.TrimSpace(f.Name) == "" {
			continue
		}
		entry := &domain.CodexEntry{
			ID:              fmt.Sprintf("codex_%s_fac_%d", projectID, i+1),
			ProjectID:       projectID,
			Category:        domain.CategoryFaction,
			Name:            f.Name,
			Alignment:       f.Alignment,
			ThreatLevel:     f.ThreatLevel,
			ColorTag:        "#6366f1",
			Summary:         fmt.Sprintf("立场: %s | 威胁度: %s", f.Alignment, f.ThreatLevel),
			DetailsMarkdown: f.Doctrine,
			TrackingMode:    domain.TrackingModeAutoMention,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = store.SaveCodexEntry(ctx, entry)
	}

	fw.Factions = nil
	fw.KeyCharacters = nil
	fw.SeedHooks = nil
	if err := store.SaveProject(ctx, proj); err != nil {
		return fmt.Errorf("seed genesis: save stripped project: %w", err)
	}
	return nil
}
