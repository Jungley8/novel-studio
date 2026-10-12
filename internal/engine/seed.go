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

	proto := fw.Protagonist
	if proto == nil || strings.TrimSpace(proto.Name) == "" {
		return fmt.Errorf("seed genesis: framework missing protagonist profile; regenerate the framework bootstrap")
	}

	initialRealm := strings.TrimSpace(proto.InitialRealm)
	if initialRealm == "" && len(fw.PowerLadder) > 0 {
		initialRealm = fw.PowerLadder[0].Realm
	}
	if initialRealm == "" {
		initialRealm = "初始"
	}

	protagonistName := strings.TrimSpace(proto.Name)
	protagonistWant := strings.TrimSpace(proto.Want)
	if protagonistWant == "" {
		protagonistWant = fw.ThemePremise
	}
	protagonistNeed := strings.TrimSpace(proto.Need)
	protagonistGhost := strings.TrimSpace(proto.Ghost)

	var inventoryItems []string
	for _, it := range proto.Inventory {
		if trimmed := strings.TrimSpace(it); trimmed != "" {
			inventoryItems = append(inventoryItems, trimmed)
		}
	}
	inventoryText := strings.Join(inventoryItems, ", ")
	if inventoryText == "" {
		inventoryText = "无"
	}
	structuredItems := make([]domain.InventoryItem, 0, len(inventoryItems))
	for _, it := range inventoryItems {
		structuredItems = append(structuredItems, domain.InventoryItem{Name: it, Quantity: 1, Quality: "初始随身物", AcquiredAt: 0})
	}

	identityText := strings.TrimSpace(proto.Identity)
	if identityText == "" {
		identityText = "未设定"
	}

	projectID := proj.ID

	protDetailsSb := strings.Builder{}
	protDetailsSb.WriteString(fmt.Sprintf("### %s (全书核心主角)\n- **身份**: %s\n- **初始境界/实力**: %s\n- **外在欲望**: %s\n", protagonistName, identityText, initialRealm, protagonistWant))
	if protagonistNeed != "" {
		protDetailsSb.WriteString(fmt.Sprintf("- **真实需求**: %s\n", protagonistNeed))
	}
	if protagonistGhost != "" {
		protDetailsSb.WriteString(fmt.Sprintf("- **核心创伤**: %s\n", protagonistGhost))
	}
	protDetailsSb.WriteString(fmt.Sprintf("- **随身物品**: %s\n- **初始状态**: 良好\n", inventoryText))

	// 0. Seed Protagonist into The Codex
	protEntry := &domain.CodexEntry{
		ID:                 fmt.Sprintf("codex_%s_pro", projectID),
		ProjectID:          projectID,
		Category:           domain.CategoryCharacter,
		Name:               protagonistName,
		ColorTag:           "#f59e0b",
		Summary:            fmt.Sprintf("全书核心主角 | 身份: %s | 境界: %s | 欲望: %s", identityText, initialRealm, protagonistWant),
		DetailsMarkdown:    protDetailsSb.String(),
		Archetype:          "PROTAGONIST",
		VoiceTone:          strings.TrimSpace(proto.VoiceTone),
		Want:               protagonistWant,
		Need:               protagonistNeed,
		Ghost:              protagonistGhost,
		CurrentDisposition: "FRIENDLY",
		TrackingMode:       domain.TrackingModeAlwaysInject,
		Aliases:            []string{"主角", protagonistName, fmt.Sprintf("%s (%s)", protagonistName, initialRealm)},
		Realm:              initialRealm,
		Inventory:          inventoryText,
		HealthStatus:       "良好",
		StructuredLevel: &domain.PowerLevel{
			Realm: initialRealm,
			History: []domain.LevelTransition{
				{
					FromRealm: "初始",
					ToRealm:   initialRealm,
					Chapter:   0,
					Reason:    "创世开局设定",
					Timestamp: time.Now(),
				},
			},
		},
		StructuredItems: structuredItems,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
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

	// 2. Seed key cast characters into The Codex (protagonist excluded by contract)
	for i, kc := range fw.KeyCharacters {
		if strings.TrimSpace(kc.Name) == "" || strings.TrimSpace(kc.Name) == protagonistName {
			continue
		}
		archetype := "SUPPORTING"
		disposition := "NEUTRAL"
		voiceTone := "言简意赅，语带锋芒"
		colorTag := "#10b981"

		if strings.Contains(kc.Role, "宿敌") || strings.Contains(kc.Role, "魔") || strings.Contains(kc.Role, "反派") {
			archetype = "ANTAGONIST"
			disposition = "HOSTILE"
			voiceTone = "表面客气，话里藏刀，真正发狠时语气反而更轻"
			colorTag = "#f43f5e"
		} else if strings.Contains(kc.Role, "搭档") || strings.Contains(kc.Role, "同盟") || strings.Contains(kc.Role, "女配") {
			archetype = "DEUTERAGONIST"
			disposition = "WARY"
			voiceTone = "机敏直接，试探与真心并存"
			colorTag = "#8b5cf6"
		} else if strings.Contains(kc.Role, "导师") || strings.Contains(kc.Role, "领路") {
			archetype = "MENTOR"
			disposition = "FRIENDLY"
			voiceTone = "沉稳克制，言简意深"
			colorTag = "#3b82f6"
		}

		kcWant := strings.TrimSpace(kc.Want)
		kcNeed := strings.TrimSpace(kc.Need)
		kcGhost := strings.TrimSpace(kc.Ghost)

		kcDetailsSb := strings.Builder{}
		kcDetailsSb.WriteString(fmt.Sprintf("### %s\n- **戏剧定位**: %s\n- **修为境界**: %s\n", kc.Name, kc.Role, kc.Realm))
		if kcWant != "" {
			kcDetailsSb.WriteString(fmt.Sprintf("- **外在欲望**: %s\n", kcWant))
		}
		if kcNeed != "" {
			kcDetailsSb.WriteString(fmt.Sprintf("- **真实需求**: %s\n", kcNeed))
		}
		if kcGhost != "" {
			kcDetailsSb.WriteString(fmt.Sprintf("- **核心创伤**: %s\n", kcGhost))
		}
		kcDetailsSb.WriteString(fmt.Sprintf("- **宿命轨迹**: %s\n", kc.FateArc))

		entry := &domain.CodexEntry{
			ID:                 fmt.Sprintf("codex_%s_char_%d", projectID, i+1),
			ProjectID:          projectID,
			Category:           domain.CategoryCharacter,
			Name:               kc.Name,
			ColorTag:           colorTag,
			Summary:            fmt.Sprintf("%s (%s), 境界: %s", kc.Role, kc.FateArc, kc.Realm),
			DetailsMarkdown:    kcDetailsSb.String(),
			Archetype:          archetype,
			VoiceTone:          voiceTone,
			Want:               kcWant,
			Need:               kcNeed,
			Ghost:              kcGhost,
			CurrentDisposition: disposition,
			TrackingMode:       domain.TrackingModeAutoMention,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		_ = store.SaveCodexEntry(ctx, entry)
	}

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

	fw.Protagonist = nil
	fw.Factions = nil
	fw.KeyCharacters = nil
	fw.SeedHooks = nil
	if err := store.SaveProject(ctx, proj); err != nil {
		return fmt.Errorf("seed genesis: save stripped project: %w", err)
	}
	return nil
}
