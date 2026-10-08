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
	Project             *domain.Project         `json:"project"`
	TargetChapter       int                     `json:"target_chapter"`
	GlobalSummary       string                  `json:"global_summary"`
	CurrentVolume       *domain.VolumeArc       `json:"current_volume,omitempty"`
	ActivePowerTier     *domain.PowerLadderTier `json:"active_power_tier,omitempty"`
	RollingCanonText    string                  `json:"rolling_canon_text"`
	TailAnchor          string                  `json:"tail_anchor,omitempty"`
	HistoricalCallbacks string                  `json:"historical_callbacks,omitempty"`
	RecentChapters      []*domain.Chapter       `json:"recent_chapters"`
	UrgentHooks         []*domain.PlotHook      `json:"urgent_hooks"`
	AllActiveHooks      []*domain.PlotHook      `json:"all_active_hooks"`
	ProtagonistState    domain.Protagonist      `json:"protagonist_state"`
	WorldRules          string                  `json:"world_rules"`
	ActiveCodexEntries  []*domain.CodexEntry    `json:"active_codex_entries,omitempty"`
	CodexRelations      []domain.EntityRelation `json:"codex_relations,omitempty"`
	CodexContextText    string                  `json:"codex_context_text,omitempty"`
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

	var currentVolume *domain.VolumeArc
	var activeTier *domain.PowerLadderTier
	if project.Framework != nil {
		accChapters := 0
		for i := range project.Framework.VolumeArcs {
			arc := project.Framework.VolumeArcs[i]
			est := arc.EstimatedChapters
			if est <= 0 {
				est = 40
			}
			if targetChapter > accChapters && targetChapter <= accChapters+est {
				currentVolume = &arc
				break
			}
			accChapters += est
		}
		if currentVolume == nil && len(project.Framework.VolumeArcs) > 0 {
			currentVolume = &project.Framework.VolumeArcs[len(project.Framework.VolumeArcs)-1]
		}
		if currentVolume != nil {
			globalSummary += fmt.Sprintf("\n【当前分卷总纲】第 %d 卷：《%s》 | 卷主线：%s | 卷大高潮：%s",
				currentVolume.VolumeIndex, currentVolume.Title, currentVolume.CoreGoal, currentVolume.Climax)
		}

		for i := range project.Framework.PowerLadder {
			tier := project.Framework.PowerLadder[i]
			if strings.Contains(project.Protagonist.NameAndLevel, tier.Realm) {
				activeTier = &tier
				break
			}
		}
	}

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
	hooksByChapter := make(map[int][]*domain.PlotHook)

	for _, h := range hooks {
		if h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting {
			activeHooks = append(activeHooks, h)
			hooksByChapter[h.CreatedChapter] = append(hooksByChapter[h.CreatedChapter], h)
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
			chHooks := hooksByChapter[oldCh.ChapterIndex]
			if len(chHooks) == 0 {
				continue
			}
			for _, h := range chHooks {
				var hookDesc strings.Builder
				hookDesc.WriteString(fmt.Sprintf("• 【第 %d 章：%s】埋设伏笔《%s》（预定第 %d 章回收，当前状态：%s）",
					oldCh.ChapterIndex, oldCh.Title, h.Title, h.TargetChapter, h.Status))
				if strings.TrimSpace(h.Details) != "" {
					hookDesc.WriteString(fmt.Sprintf("\n  - 伏笔要点与设计：%s", strings.TrimSpace(h.Details)))
				}
				if strings.TrimSpace(oldCh.CoreConflict) != "" {
					hookDesc.WriteString(fmt.Sprintf("\n  - 原章核心冲突：%s", strings.TrimSpace(oldCh.CoreConflict)))
				}
				excerpt := extractHookSceneExcerpt(oldCh.Content, h.Title, h.Details, 150)
				if excerpt != "" {
					hookDesc.WriteString(fmt.Sprintf("\n  - 埋设时历史场景还原：“%s”", excerpt))
				}
				callbacks = append(callbacks, hookDesc.String())
			}
		}
		if len(callbacks) > 0 {
			cbSb.WriteString("【跨卷历史因果线索 (Historical Callbacks)】\n")
			cbSb.WriteString(strings.Join(callbacks, "\n\n"))
		}
	}

	// Layer 4: The Codex (动态世界观全域百科与时空切片)
	var activeCodex []*domain.CodexEntry
	var codexRels []domain.EntityRelation
	var codexSb strings.Builder

	codexEntries, err := c.store.ListCodexEntries(ctx, projectID, "")
	if err == nil && len(codexEntries) > 0 {
		scanner := NewMentionScanner()
		scanText := sb.String() + "\n" + tailAnchor
		matched, _ := scanner.ScanText(scanText, codexEntries)
		if len(matched) > 0 {
			activeCodex = matched
			codexSb.WriteString("【出场世界观实体与时空状态 (The Codex)】\n")
			matchedIDs := make(map[string]bool)
			for _, ent := range matched {
				matchedIDs[ent.ID] = true
				categoryLabel := "条目"
				switch ent.Category {
				case domain.CategoryCharacter:
					categoryLabel = "角色"
				case domain.CategoryLocation:
					categoryLabel = "地点"
				case domain.CategoryItem:
					categoryLabel = "物品/功法"
				case domain.CategoryLore:
					categoryLabel = "法则/设定"
				case domain.CategoryFaction:
					categoryLabel = "势力"
				}

				aliasStr := ""
				if len(ent.Aliases) > 0 {
					aliasStr = fmt.Sprintf(" (别名：%s)", strings.Join(ent.Aliases, ", "))
				}

				codexSb.WriteString(fmt.Sprintf("• [%s] %s%s", categoryLabel, ent.Name, aliasStr))
				if ent.Category == domain.CategoryCharacter {
					if ent.Archetype != "" {
						codexSb.WriteString(fmt.Sprintf(" 【定位: %s】", ent.Archetype))
					}
					if ent.CurrentDisposition != "" {
						codexSb.WriteString(fmt.Sprintf(" 【对主角立场: %s】", ent.CurrentDisposition))
					}
				}

				// Check active progression snapshot for target chapter
				prog := ent.ActiveProgression(targetChapter)
				if prog != nil {
					codexSb.WriteString(fmt.Sprintf(" - 当前时空切片(第%d章起)：%s", prog.ActiveFromChapter, prog.StatePayloadJSON))
					if prog.Notes != "" {
						codexSb.WriteString(fmt.Sprintf(" [%s]", prog.Notes))
					}
				} else if ent.Summary != "" {
					codexSb.WriteString(fmt.Sprintf(" - %s", ent.Summary))
				}
				codexSb.WriteString("\n")

				if ent.Category == domain.CategoryCharacter {
					if ent.VoiceTone != "" {
						codexSb.WriteString(fmt.Sprintf("  - 台词声口与语言风格：%s\n", ent.VoiceTone))
					}
					if ent.CoreMotivation != "" {
						codexSb.WriteString(fmt.Sprintf("  - 核心动机与底层执念：%s\n", ent.CoreMotivation))
					}
				}

				if ent.DetailsMarkdown != "" {
					codexSb.WriteString(fmt.Sprintf("  - 设定细节：%s\n", ent.DetailsMarkdown))
				}
			}

			// Query relations between co-occurring entities
			for _, ent := range matched {
				rels, relErr := c.store.ListCodexRelations(ctx, projectID, ent.ID)
				if relErr == nil {
					for _, r := range rels {
						if matchedIDs[r.TargetEntryID] {
							codexRels = append(codexRels, r)
						}
					}
				}
			}

			if len(codexRels) > 0 {
				codexSb.WriteString("\n【实体间羁绊与冲突事实 (Relations)】\n")
				for _, r := range codexRels {
					sourceName := entNameByID(matched, r.SourceEntryID)
					targetName := r.TargetName
					if targetName == "" {
						targetName = entNameByID(matched, r.TargetEntryID)
					}
					codexSb.WriteString(fmt.Sprintf("• [%s] ➔ [%s] (%s)", sourceName, targetName, r.RelationType))
					if r.Description != "" {
						codexSb.WriteString(fmt.Sprintf("：%s", r.Description))
					}
					codexSb.WriteString("\n")
				}
			}
		}
	}

	return &CanonHorizon{
		Project:             project,
		TargetChapter:       targetChapter,
		GlobalSummary:       globalSummary,
		CurrentVolume:       currentVolume,
		ActivePowerTier:     activeTier,
		RollingCanonText:    strings.TrimSpace(sb.String()),
		TailAnchor:          tailAnchor,
		HistoricalCallbacks: strings.TrimSpace(cbSb.String()),
		RecentChapters:      recent,
		UrgentHooks:         urgentHooks,
		AllActiveHooks:      activeHooks,
		ProtagonistState:    project.Protagonist,
		WorldRules:          project.WorldRules,
		ActiveCodexEntries:  activeCodex,
		CodexRelations:      codexRels,
		CodexContextText:    strings.TrimSpace(codexSb.String()),
	}, nil
}

func entNameByID(entries []*domain.CodexEntry, id string) string {
	for _, e := range entries {
		if e.ID == id {
			return e.Name
		}
	}
	return id
}

// extractHookSceneExcerpt extracts a high-relevance narrative excerpt around the moment
// a foreshadowing hook was planted in an older canon chapter.
func extractHookSceneExcerpt(content string, hookTitle string, hookDetails string, maxRunes int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = 150
	}
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return cleanExcerpt(string(runes))
	}

	searchTerms := buildHookSearchTerms(hookTitle, hookDetails)
	bestIdx := -1
	for _, term := range searchTerms {
		if term == "" {
			continue
		}
		idx := strings.Index(content, term)
		if idx != -1 {
			bestIdx = len([]rune(content[:idx]))
			break
		}
	}

	if bestIdx == -1 {
		// Fallback to chapter tail where plot hooks / cliffhangers are traditionally placed
		start := len(runes) - maxRunes
		if start < 0 {
			start = 0
		}
		excerpt := string(runes[start:])
		clean := cleanExcerpt(excerpt)
		if start > 0 && !strings.HasPrefix(clean, "...") {
			clean = "..." + clean
		}
		return clean
	}

	halfWindow := maxRunes / 2
	start := bestIdx - halfWindow
	if start < 0 {
		start = 0
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}

	excerpt := string(runes[start:end])
	clean := cleanExcerpt(excerpt)
	if start > 0 && !strings.HasPrefix(clean, "...") {
		clean = "..." + clean
	}
	if end < len(runes) && !strings.HasSuffix(clean, "...") {
		clean = clean + "..."
	}
	return clean
}

func buildHookSearchTerms(title string, details string) []string {
	var terms []string
	cleanTitle := strings.Trim(title, "《》“”\"'【】 ")
	if cleanTitle != "" {
		terms = append(terms, cleanTitle)
		tRunes := []rune(cleanTitle)
		if len(tRunes) >= 4 {
			mid := len(tRunes) / 2
			terms = append(terms, string(tRunes[:mid]))
			terms = append(terms, string(tRunes[mid:]))
		}
	}

	if details != "" {
		splitFn := func(c rune) bool {
			return c == '，' || c == '。' || c == '；' || c == '！' || c == '？' ||
				c == ',' || c == '.' || c == ';' || c == '!' || c == '?' || c == '\n'
		}
		phrases := strings.FieldsFunc(details, splitFn)
		for _, p := range phrases {
			p = strings.TrimSpace(p)
			pRunes := []rune(p)
			if len(pRunes) >= 2 && len(pRunes) <= 15 {
				terms = append(terms, p)
			}
		}
	}
	return terms
}

func cleanExcerpt(s string) string {
	lines := strings.Split(s, "\n")
	var cleanedLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}
	return strings.Join(cleanedLines, " ")
}
