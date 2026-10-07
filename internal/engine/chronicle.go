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
type CanonHorizon struct {
	Project          *domain.Project    `json:"project"`
	TargetChapter    int                `json:"target_chapter"`
	RollingCanonText string             `json:"rolling_canon_text"`
	RecentChapters   []*domain.Chapter  `json:"recent_chapters"`
	UrgentHooks      []*domain.PlotHook `json:"urgent_hooks"`
	AllActiveHooks   []*domain.PlotHook `json:"all_active_hooks"`
	ProtagonistState domain.Protagonist `json:"protagonist_state"`
	WorldRules       string             `json:"world_rules"`
}

// CanonChronicle acts as the deep context assembler and horizon keeper.
type CanonChronicle struct {
	store store.Store
}

func NewCanonChronicle(s store.Store) *CanonChronicle {
	return &CanonChronicle{store: s}
}

// AssembleHorizon deterministically synthesizes the rolling 3-chapter canon window,
// urgent plot hooks nearing resolution, and verified entity states behind a single seam.
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

	// 1. Extract 3-chapter sealed rolling window prior to targetChapter
	var preceding []*domain.Chapter
	for _, ch := range chapters {
		if ch.ChapterIndex < targetChapter {
			preceding = append(preceding, ch)
		}
	}

	windowSize := 3
	var recent []*domain.Chapter
	if len(preceding) <= windowSize {
		recent = preceding
	} else {
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
			sb.WriteString("\n")
		}
	}

	// 2. Identify active and urgent plot hooks
	var activeHooks []*domain.PlotHook
	var urgentHooks []*domain.PlotHook

	for _, h := range hooks {
		if h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting {
			activeHooks = append(activeHooks, h)
			// Urgent if scheduled for resolution within 2 chapters or overdue
			if h.TargetChapter <= targetChapter+2 {
				urgentHooks = append(urgentHooks, h)
			}
		}
	}

	return &CanonHorizon{
		Project:          project,
		TargetChapter:    targetChapter,
		RollingCanonText: strings.TrimSpace(sb.String()),
		RecentChapters:   recent,
		UrgentHooks:      urgentHooks,
		AllActiveHooks:   activeHooks,
		ProtagonistState: project.Protagonist,
		WorldRules:       project.WorldRules,
	}, nil
}
