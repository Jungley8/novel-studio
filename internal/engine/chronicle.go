package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// CanonHorizon encapsulates the synthesized rolling context window for a target chapter,
// ensuring strict continuity across previous sealed canon and active plot hooks.
// CanonHorizon encapsulates the synthesized three-tier context window for a target chapter:
// Layer 1: Global world axioms & book progress
// Layer 2: Rolling 3-chapter high-fidelity sealed canon
// Layer 3: Cross-volume callbacks & active/urgent plot hooks
type CanonHorizon struct {
	Project             *domain.Project    `json:"project"`
	TargetChapter       int                `json:"target_chapter"`
	GlobalSummary       string             `json:"global_summary"`
	RollingCanonText    string             `json:"rolling_canon_text"`
	TailAnchor          string             `json:"tail_anchor,omitempty"`
	HistoricalCallbacks string             `json:"historical_callbacks,omitempty"`
	RecentChapters      []*domain.Chapter  `json:"recent_chapters"`
	UrgentHooks         []*domain.PlotHook `json:"urgent_hooks"`
	AllActiveHooks      []*domain.PlotHook `json:"all_active_hooks"`
	ProtagonistState    domain.Protagonist `json:"protagonist_state"`
	WorldRules          string             `json:"world_rules"`
}

// CanonChronicle acts as the deep context assembler and horizon keeper.
type CanonChronicle struct {
	store store.Store
}

func NewCanonChronicle(s store.Store) *CanonChronicle {
	return &CanonChronicle{store: s}
}

// AssembleHorizon deterministically synthesizes the three-tier context horizon:
// 1. Layer 1: Global Summary & Macro Arc
// 2. Layer 2: Rolling 3-chapter sealed canon window
// 3. Layer 3: Cross-volume callbacks for active hooks & urgent plot resolution
func (c *CanonChronicle) AssembleHorizon(ctx context.Context, projectID string, targetChapter int) (*CanonHorizon, error) {
	if c.store == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	project, err := c.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project failed: %w", err)
	}

	chapters, err := c.store.ListChapters(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters failed: %w", err)
	}

	hooks, err := c.store.ListPlotHooks(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list plot hooks failed: %w", err)
	}

	// Separate preceding chapters
	var preceding []*domain.Chapter
	for _, ch := range chapters {
		if ch.ChapterIndex < targetChapter {
			preceding = append(preceding, ch)
		}
	}

	// Layer 1: Global Summary
	globalSummary := fmt.Sprintf("书名：《%s》 | 目标平台：%s | 全书当前进度：已归档 %d 章，正向第 %d 章推进。\n核心世界法则：%s",
		project.Title, project.TargetPlatform, len(preceding), targetChapter, project.WorldRules)

	// Layer 2: Rolling 3-Chapter Window
	windowSize := 3
	var recent []*domain.Chapter
	var older []*domain.Chapter
	if len(preceding) <= windowSize {
		recent = preceding
	} else {
		older = preceding[:len(preceding)-windowSize]
		recent = preceding[len(preceding)-windowSize:]
	}

	var sb strings.Builder
	if len(recent) == 0 {
		sb.WriteString("(前序章节：本书开篇第一章，无前序历史)")
	} else {
		for _, ch := range recent {
			sb.WriteString(fmt.Sprintf("【第 %d 章：%s】\n", ch.ChapterIndex, ch.Title))
			if ch.CoreConflict != "" {
				sb.WriteString(fmt.Sprintf("• 核心冲突：%s\n", ch.CoreConflict))
			}
			if len(ch.Beats) > 0 {
				var bActions []string
				for _, b := range ch.Beats {
					if b.Action != "" {
						bActions = append(bActions, b.Action)
					}
				}
				if len(bActions) > 0 {
					sb.WriteString(fmt.Sprintf("• 履约节拍：%s\n", strings.Join(bActions, " ➔ ")))
				}
			}
			if ch.StateMutation.InventoryDelta != "" || ch.StateMutation.PowerDelta != "" {
				sb.WriteString(fmt.Sprintf("• 状态结算：%s %s\n", ch.StateMutation.InventoryDelta, ch.StateMutation.PowerDelta))
			}
			runes := []rune(ch.Content)
			if len(runes) > 0 {
				start := 0
				if len(runes) > 200 {
					start = len(runes) - 200
				}
				anchor := string(runes[start:])
				sb.WriteString(fmt.Sprintf("• 尾段风格锚定（最后%d字）：%s\n", len([]rune(anchor)), anchor))
			}
			sb.WriteString("\n")
		}
	}

	var tailAnchor string
	if len(recent) > 0 {
		lastCh := recent[len(recent)-1]
		lastRunes := []rune(lastCh.Content)
		if len(lastRunes) > 0 {
			start := 0
			if len(lastRunes) > 200 {
				start = len(lastRunes) - 200
			}
			tailAnchor = string(lastRunes[start:])
		}
	}

	// Identify active and urgent plot hooks
	var activeHooks []*domain.PlotHook
	var urgentHooks []*domain.PlotHook
	activeOriginChapters := make(map[int]bool)

	for _, h := range hooks {
		if h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting {
			activeHooks = append(activeHooks, h)
			activeOriginChapters[h.CreatedChapter] = true
			// Urgent if scheduled for resolution within 2 chapters or overdue
			if h.TargetChapter <= targetChapter+2 {
				urgentHooks = append(urgentHooks, h)
			}
		}
	}

	// Layer 3: Cross-volume Historical Callbacks for long-arc foreshadowing
	var cbSb strings.Builder
	if len(older) > 0 {
		var callbacks []string
		for _, oldCh := range older {
			if activeOriginChapters[oldCh.ChapterIndex] {
				callbacks = append(callbacks, fmt.Sprintf("• 【第 %d 章：%s】埋下的长线伏笔根源，核心冲突：%s",
					oldCh.ChapterIndex, oldCh.Title, oldCh.CoreConflict))
			}
		}
		if len(callbacks) > 0 {
			cbSb.WriteString("【跨卷历史因果线索 (Historical Callbacks)】\n")
			cbSb.WriteString(strings.Join(callbacks, "\n"))
		}
	}

	return &CanonHorizon{
		Project:             project,
		TargetChapter:       targetChapter,
		GlobalSummary:       globalSummary,
		RollingCanonText:    strings.TrimSpace(sb.String()),
		TailAnchor:          tailAnchor,
		HistoricalCallbacks: strings.TrimSpace(cbSb.String()),
		RecentChapters:      recent,
		UrgentHooks:         urgentHooks,
		AllActiveHooks:      activeHooks,
		ProtagonistState:    project.Protagonist,
		WorldRules:          project.WorldRules,
	}, nil
}
