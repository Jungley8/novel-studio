package engine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

const (
	// DefaultContextBudgetTokens defines the fallback token limit when no configuration is provided (12000).
	DefaultContextBudgetTokens = 12000
	// MaxCodexDetailsRunes defines the rune limit for individual Codex entry DetailsMarkdown before in-place truncation.
	MaxCodexDetailsRunes = 300
	// MaxRetrievedMemories defines the max number of historical summary memories retrieved via BM25.
	MaxRetrievedMemories = 8
)

type budgetContextKey struct{}

// WithContextBudget attaches an explicit token budget to the context.
func WithContextBudget(ctx context.Context, budgetTokens int) context.Context {
	return context.WithValue(ctx, budgetContextKey{}, budgetTokens)
}

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
	ProtagonistEntry    *domain.CodexEntry      `json:"protagonist_entry,omitempty"`
	WorldRules          string                  `json:"world_rules"`
	ActiveCodexEntries  []*domain.CodexEntry    `json:"active_codex_entries,omitempty"`
	CodexRelations      []domain.EntityRelation `json:"codex_relations,omitempty"`
	CodexContextText    string                  `json:"codex_context_text,omitempty"`
	NarrativeTone       string                  `json:"narrative_tone,omitempty"`
	StyleVaultAnchor    string                  `json:"style_vault_anchor,omitempty"`
	SelectedPlanText    string                  `json:"selected_plan_text,omitempty"`
	ConsumedIntent      *domain.ChapterIntent   `json:"consumed_intent,omitempty"`
}

// VolumeRetrospectiveStore defines the query interface for volume retrospectives.
type VolumeRetrospectiveStore interface {
	GetVolumeRetrospective(ctx context.Context, projectID string, volumeIndex int) (*domain.VolumeRetrospective, error)
}

// CanonChronicle acts as the deep context assembler and horizon keeper.
type CanonChronicle struct {
	store        store.Store
	cfg          *config.Config
	budgetTokens int
	compactor    CompressibleCompactor
}

func NewCanonChronicle(s store.Store) *CanonChronicle {
	return &CanonChronicle{store: s}
}

// SetBudgetTokens overrides the token budget for AssembleHorizon.
func (c *CanonChronicle) SetBudgetTokens(tokens int) {
	c.budgetTokens = tokens
}

// SetConfig sets the system config reference on CanonChronicle.
func (c *CanonChronicle) SetConfig(cfg *config.Config) {
	c.cfg = cfg
}

// SetCompactor sets the semantic memory compactor for dropped compressible context.
func (c *CanonChronicle) SetCompactor(fn CompressibleCompactor) {
	c.compactor = fn
}

func (c *CanonChronicle) resolveBudget(ctx context.Context) int {
	if ctx != nil {
		if val, ok := ctx.Value(budgetContextKey{}).(int); ok && val > 0 {
			return val
		}
	}
	if c.budgetTokens > 0 {
		return c.budgetTokens
	}
	if c.cfg != nil && c.cfg.ContextBudgetTokens > 0 {
		return c.cfg.ContextBudgetTokens
	}
	return DefaultContextBudgetTokens
}

// AccountedExtra represents an out-of-canon injected prompt block accounted as protected context.
type AccountedExtra struct {
	ID   string
	Text string
}

// AssembleHorizon deterministically synthesizes the three-tier context horizon:
// 1. Layer 1: Global Summary & Macro Arc
// 2. Layer 2: Rolling 3-chapter sealed canon window
// 3. Layer 3: Cross-volume callbacks for active hooks & urgent plot resolution
// Zero-conflict constraint: preserved for external callers, delegates to AssembleHorizonTraced.
func (c *CanonChronicle) AssembleHorizon(ctx context.Context, projectID string, targetChapter int) (*CanonHorizon, error) {
	horizon, _, err := c.AssembleHorizonTraced(ctx, projectID, targetChapter)
	return horizon, err
}

// AssembleHorizonWithExtras synthesizes the context horizon with extra accounted protected prompt blocks.
func (c *CanonChronicle) AssembleHorizonWithExtras(ctx context.Context, projectID string, targetChapter int, extras ...AccountedExtra) (*CanonHorizon, error) {
	horizon, _, err := c.AssembleHorizonTracedWithExtras(ctx, projectID, targetChapter, extras...)
	return horizon, err
}

// AssembleHorizonTraced deterministically synthesizes the three-tier context horizon with itemized token budgeting and tracing.
func (c *CanonChronicle) AssembleHorizonTraced(ctx context.Context, projectID string, targetChapter int) (*CanonHorizon, *domain.ContextTrace, error) {
	return c.AssembleHorizonTracedWithExtras(ctx, projectID, targetChapter)
}

// AssembleHorizonTracedWithExtras deterministically synthesizes the three-tier context horizon with itemized token budgeting,
// tracing, and injected extras accounted as protected context items.
func (c *CanonChronicle) AssembleHorizonTracedWithExtras(ctx context.Context, projectID string, targetChapter int, extras ...AccountedExtra) (*CanonHorizon, *domain.ContextTrace, error) {
	if c.store == nil {
		return nil, nil, fmt.Errorf("store is not initialized")
	}

	project, err := c.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("get project failed: %w", err)
	}

	chapters, err := c.store.ListChapters(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("list chapters failed: %w", err)
	}

	hooks, err := c.store.ListPlotHooks(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("list plot hooks failed: %w", err)
	}

	// Separate preceding chapters
	var preceding []*domain.Chapter
	for _, ch := range chapters {
		if ch.ChapterIndex < targetChapter {
			preceding = append(preceding, ch)
		}
	}

	protagonist, _ := c.store.GetProtagonistEntry(ctx, projectID)
	if protagonist == nil {
		protagonist = &domain.CodexEntry{
			ProjectID: projectID,
			Category:  domain.CategoryCharacter,
			Name:      "主角",
			Archetype: "PROTAGONIST",
		}
	}

	budget := c.resolveBudget(ctx)
	trace := &domain.ContextTrace{
		ProtectedIDs:    make([]string, 0),
		CompressibleIDs: make([]string, 0),
		DroppedIDs:      make([]string, 0),
		BudgetTokens:    budget,
		EstimatedTokens: 0,
		Truncated:       false,
	}

	// Layer 1: Global Summary
	globalSummary := fmt.Sprintf("书名：《%s》 | 目标平台：%s | 全书当前进度：已归档 %d 章，正向第 %d 章推进。\n核心世界法则：%s",
		project.Title, project.TargetPlatform, len(preceding), targetChapter, project.WorldRules)

	var currentVolume *domain.VolumeArc
	var activeTier *domain.PowerLadderTier
	if project.Framework != nil {
		project.Framework.VolumeArcs = BackfillVolumeArcs(project.Framework.VolumeArcs)
		spans, arcMap := deriveVolumeSpans(project.Framework.VolumeArcs)
		currentVolume = resolveVolumeArc(targetChapter, spans, arcMap)
		if currentVolume != nil {
			globalSummary += fmt.Sprintf("\n【当前分卷总纲】第 %d 卷：《%s》 | 卷主线：%s | 卷大高潮：%s",
				currentVolume.VolumeIndex, currentVolume.Title, currentVolume.CoreGoal, currentVolume.Climax)
		} else {
			currentVolume = nil
			globalSummary += "\n【当前分卷总纲】目标章节超出已规划分卷，请先扩展卷纲"
		}

		for i := range project.Framework.PowerLadder {
			tier := project.Framework.PowerLadder[i]
			if strings.Contains(protagonist.NameAndLevel(), tier.Realm) || (protagonist.Realm != "" && strings.Contains(protagonist.Realm, tier.Realm)) {
				activeTier = &tier
				break
			}
		}
	}

	var retroList []*domain.VolumeRetrospective
	if currentVolume != nil {
		if rStore, ok := c.store.(VolumeRetrospectiveStore); ok {
			// 加载卷号小于 currentVolume.VolumeIndex 的最后 2 卷（从 currentVolume.VolumeIndex-1 往下取，最多 2 个）
			// 回顾是可选记忆层，缺卷数据是正常态，nil 或 error 均跳过，不报错中断
			for v := currentVolume.VolumeIndex - 1; v >= 1 && v >= currentVolume.VolumeIndex-2; v-- {
				vr, err := rStore.GetVolumeRetrospective(ctx, projectID, v)
				if err != nil || vr == nil {
					continue
				}
				retroList = append(retroList, vr)
			}
		}
	}

	// Compressible candidate definition
	type compressibleCandidate struct {
		id         string
		category   string
		desc       string
		tokens     int
		isUrgent   bool
		distance   int
		codexEntry *domain.CodexEntry
	}

	var retrievedCandidates []compressibleCandidate
	intents, err := c.store.ListChapterIntents(ctx, projectID, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("list chapter intents for retrieval: %w", err)
	}
	var targetIntent *domain.ChapterIntent
	for _, it := range intents {
		if it != nil && it.ChapterIndex == targetChapter {
			targetIntent = it
			break
		}
	}
	if targetIntent != nil {
		query := strings.TrimSpace(targetIntent.Conflict + " " + strings.Join(targetIntent.HookPayoffs, " "))
		if query != "" {
			hits, err := c.store.SearchMemory(ctx, projectID, query, MaxRetrievedMemories*3)
			if err != nil {
				return nil, nil, fmt.Errorf("search memory failed: %w", err)
			}
			seenRef := make(map[string]bool)
			for _, hit := range hits {
				if hit == nil {
					continue
				}
				if hit.Kind != "summary" {
					continue
				}
				if hit.ChapterIndex <= 0 || hit.ChapterIndex >= targetChapter-3 {
					continue
				}
				if seenRef[hit.RefID] {
					continue
				}
				seenRef[hit.RefID] = true

				desc := fmt.Sprintf("【历史检索记忆·第%d章】%s", hit.ChapterIndex, strings.TrimSpace(hit.Raw))
				retrievedCandidates = append(retrievedCandidates, compressibleCandidate{
					id:       "retrieved/chapter:" + strconv.Itoa(hit.ChapterIndex),
					category: "retrieved",
					desc:     desc,
					tokens:   domain.EstimateTokens(desc),
					isUrgent: false,
					distance: targetChapter - hit.ChapterIndex,
				})
				if len(retrievedCandidates) >= MaxRetrievedMemories {
					break
				}
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

	type rollingItem struct {
		id   string
		text string
	}
	var rollingItems []rollingItem
	var rollingSb strings.Builder

	if len(recent) == 0 {
		rollingSb.WriteString("(前序章节：本书开篇第一章，无前序历史)")
	} else {
		for _, ch := range recent {
			var chSb strings.Builder
			chSb.WriteString(fmt.Sprintf("【第 %d 章：%s】\n", ch.ChapterIndex, ch.Title))
			if ch.CoreConflict != "" {
				chSb.WriteString(fmt.Sprintf("• 核心冲突：%s\n", ch.CoreConflict))
			}
			if len(ch.Beats) > 0 {
				var bActions []string
				for _, b := range ch.Beats {
					if b.Action != "" {
						bActions = append(bActions, b.Action)
					}
				}
				if len(bActions) > 0 {
					chSb.WriteString(fmt.Sprintf("• 履约节拍：%s\n", strings.Join(bActions, " ➔ ")))
				}
			}
			if ch.StateMutation.InventoryDelta != "" || ch.StateMutation.PowerDelta != "" {
				chSb.WriteString(fmt.Sprintf("• 状态结算：%s %s\n", ch.StateMutation.InventoryDelta, ch.StateMutation.PowerDelta))
			}
			runes := []rune(ch.Content)
			if len(runes) > 0 {
				start := 0
				if len(runes) > 200 {
					start = len(runes) - 200
				}
				anchor := string(runes[start:])
				chSb.WriteString(fmt.Sprintf("• 尾段风格锚定（最后%d字）：%s\n", len([]rune(anchor)), anchor))
			}
			chSb.WriteString("\n")
			chText := chSb.String()
			rollingSb.WriteString(chText)
			rollingItems = append(rollingItems, rollingItem{
				id:   fmt.Sprintf("rolling/ch%d", ch.ChapterIndex),
				text: chText,
			})
		}
	}

	// TailAnchor 已降级为衔接语气锚（辅锚），主锚为 StyleVaultAnchor（全书恒定文风契约）。
	var tailAnchor string
	if len(recent) > 0 {
		lastCh := recent[len(recent)-1]
		lastRunes := []rune(lastCh.Content)
		if len(lastRunes) > 200 {
			tailAnchor = string(lastRunes[len(lastRunes)-200:])
		} else if len(lastRunes) > 0 {
			tailAnchor = string(lastRunes)
		}
	}

	// Protected context items:
	// 1. global/rules: World rules and macro framework
	// 2. global/protagonist: Protagonist current persona and realm
	// 3. rolling/ch<N>: Sealed canon of recent 3 chapters
	// 4. style/tail_anchor: Stylistic bridge for prose continuity
	// TailAnchor is classified as protected context because it serves as the critical
	// stylistic continuity bridge across the chapter boundary (to be managed by Style Vault in future).
	// It is capped at 200 runes (~100 tokens), so protecting it avoids style drift without blowing budget.
	type protectedItem struct {
		id     string
		tokens int
	}
	var protectedList []protectedItem

	rulesTokens := domain.EstimateTokens(globalSummary)
	protectedList = append(protectedList, protectedItem{id: "global/rules", tokens: rulesTokens})

	// protagText covers the union of accounting-side fields (NameAndLevel, Archetype,
	// Summary) and prompt-render-side fields (protNameLevel=NameAndLevel,
	// protInventory=Inventory, protGoal=CoreMotivation else Summary). Inventory has no
	// upper bound, so omitting it would let unbounded bytes slip past the budget gate.
	// Superset estimation (safe direction): counted bytes always cover rendered bytes.
	protagText := protagonist.NameAndLevel() + " " + protagonist.Archetype + " " + protagonist.Summary + " " + protagonist.Inventory + " " + protagonist.CoreMotivation
	protagTokens := domain.EstimateTokens(protagText)
	protectedList = append(protectedList, protectedItem{id: "global/protagonist", tokens: protagTokens})

	for _, it := range rollingItems {
		protectedList = append(protectedList, protectedItem{id: it.id, tokens: domain.EstimateTokens(it.text)})
	}

	if tailAnchor != "" {
		protectedList = append(protectedList, protectedItem{id: "style/tail_anchor", tokens: domain.EstimateTokens(tailAnchor)})
	}

	for _, extra := range extras {
		if extra.Text != "" {
			protectedList = append(protectedList, protectedItem{
				id:     extra.ID,
				tokens: domain.EstimateTokens(extra.Text),
			})
		}
	}

	totalProtectedTokens := 0
	for _, p := range protectedList {
		totalProtectedTokens += p.tokens
		trace.ProtectedIDs = append(trace.ProtectedIDs, p.id)
	}

	// Protected overflow check: fail-closed with actionable diagnostics
	if totalProtectedTokens > budget {
		var breakdown []string
		for _, p := range protectedList {
			breakdown = append(breakdown, fmt.Sprintf("%s: %d tokens", p.id, p.tokens))
		}
		trace.EstimatedTokens = totalProtectedTokens
		trace.Truncated = true
		return nil, trace, fmt.Errorf("context budget exceeded by protected items: estimated %d tokens exceeds budget %d tokens. 超限项清单: [%s]. 建议操作：请在系统设置中调大 context_budget_tokens (当前配置: %d)，或精简世界法则设定 (如 global/rules 中过长的公理内容)",
			totalProtectedTokens, budget, strings.Join(breakdown, ", "), budget)
	}

	// Identify active and urgent plot hooks
	var activeHooks []*domain.PlotHook
	var urgentHooks []*domain.PlotHook
	hooksByChapter := make(map[int][]*domain.PlotHook)

	for _, h := range hooks {
		if h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting {
			activeHooks = append(activeHooks, h)
			hooksByChapter[h.CreatedChapter] = append(hooksByChapter[h.CreatedChapter], h)
			if h.TargetChapter <= targetChapter+2 {
				urgentHooks = append(urgentHooks, h)
			}
		}
	}

	// Compressible items collection:
	// - Layer 3: Cross-volume callbacks for active hooks & foreshadowing
	// - Layer 4: The Codex entries mentioned in rolling window
	var callbackCandidates []compressibleCandidate
	if len(older) > 0 {
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

				isUrgent := h.TargetChapter <= targetChapter+2
				dist := h.TargetChapter - targetChapter
				if dist < 0 {
					dist = -dist
				}
				if h.TargetChapter <= 0 {
					dist = targetChapter - oldCh.ChapterIndex
					if dist < 0 {
						dist = -dist
					}
				}

				hID := h.ID
				if hID == "" {
					hID = h.Title
				}
				descStr := hookDesc.String()
				callbackCandidates = append(callbackCandidates, compressibleCandidate{
					id:       fmt.Sprintf("callback/%s", hID),
					category: "callback",
					desc:     descStr,
					tokens:   domain.EstimateTokens(descStr),
					isUrgent: isUrgent,
					distance: dist,
				})
			}
		}
	}

	var codexCandidates []compressibleCandidate
	relLinesBySource := make(map[string][]string)
	relStructsBySource := make(map[string][]domain.EntityRelation)

	codexEntries, err := c.store.ListCodexEntries(ctx, projectID, "")
	if err == nil && len(codexEntries) > 0 {
		scanner := NewMentionScanner()
		scanText := rollingSb.String() + "\n" + tailAnchor
		matched, _ := scanner.ScanText(scanText, codexEntries)

		// Relations pre-query (superset over the full matched set, not the retained
		// subset which is only known after the budget loop): every relation byte that
		// can appear in CodexContextText is folded into its source candidate's desc
		// BEFORE EstimateTokens, otherwise trace.EstimatedTokens<=budget would not
		// imply actual prompt<=budget. Guard style (append only when relErr == nil)
		// mirrors the previous post-loop block, which is deleted to avoid double
		// counting; CodexContextText is assembled from retained descs that already
		// carry their relations segments.
		matchedIDs := make(map[string]bool, len(matched))
		for _, ent := range matched {
			if ent != nil {
				matchedIDs[ent.ID] = true
			}
		}
		for _, ent := range matched {
			if ent == nil {
				continue
			}
			rels, relErr := c.store.ListCodexRelations(ctx, projectID, ent.ID)
			if relErr == nil {
				for _, r := range rels {
					if r.SourceEntryID != ent.ID {
						continue
					}
					if !matchedIDs[r.TargetEntryID] {
						continue
					}
					dup := false
					for _, prev := range relStructsBySource[ent.ID] {
						if prev.ID != "" && prev.ID == r.ID {
							dup = true
							break
						}
					}
					if dup {
						continue
					}
					sourceName := entNameByID(matched, r.SourceEntryID)
					targetName := r.TargetName
					if targetName == "" {
						targetName = entNameByID(matched, r.TargetEntryID)
					}
					var relSb strings.Builder
					relSb.WriteString(fmt.Sprintf("• [%s] ➔ [%s] (%s)", sourceName, targetName, r.RelationType))
					if r.Description != "" {
						relSb.WriteString(fmt.Sprintf("：%s", r.Description))
					}
					relSb.WriteString("\n")
					relLinesBySource[ent.ID] = append(relLinesBySource[ent.ID], relSb.String())
					relStructsBySource[ent.ID] = append(relStructsBySource[ent.ID], r)
				}
			}
		}

		for _, ent := range matched {
			var codexSb strings.Builder
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
				detailsRunes := []rune(ent.DetailsMarkdown)
				detailsText := ent.DetailsMarkdown
				if len(detailsRunes) > MaxCodexDetailsRunes {
					// Requirement 4: Codex single-entry DetailsMarkdown exceeding 300 runes
					// is truncated with …[truncated]. Truncation is counted in estimated tokens,
					// but does NOT enter DroppedIDs per specification.
					detailsText = string(detailsRunes[:MaxCodexDetailsRunes]) + "…[truncated]"
				}
				codexSb.WriteString(fmt.Sprintf("  - 设定细节：%s\n", detailsText))
			}

			entID := ent.ID
			if entID == "" {
				entID = ent.Name
			}
			if lines := relLinesBySource[ent.ID]; len(lines) > 0 {
				codexSb.WriteString("\n【实体间羁绊与冲突事实 (Relations)】\n")
				for _, ln := range lines {
					codexSb.WriteString(ln)
				}
			}
			descStr := codexSb.String()
			codexCandidates = append(codexCandidates, compressibleCandidate{
				id:         fmt.Sprintf("codex/%s", entID),
				category:   "codex",
				desc:       descStr,
				tokens:     domain.EstimateTokens(descStr),
				isUrgent:   false,
				distance:   0, // Active entities appearing in current rolling window have distance 0
				codexEntry: ent,
			})
		}
	}

	var retroCandidates []compressibleCandidate
	for _, vr := range retroList {
		desc := fmt.Sprintf("【往卷回顾·第%d卷（第%d-%d章）】%s",
			vr.VolumeIndex, vr.FromChapter, vr.ToChapter, strings.TrimSpace(vr.Text))
		dist := targetChapter - vr.ToChapter
		if dist < 0 {
			dist = -dist
		}
		retroCandidates = append(retroCandidates, compressibleCandidate{
			id:       fmt.Sprintf("history/retro_vol_%d", vr.VolumeIndex),
			category: "retro",
			desc:     desc,
			tokens:   domain.EstimateTokens(desc),
			isUrgent: false,
			distance: dist,
		})
	}

	var allCompressible []compressibleCandidate
	allCompressible = append(allCompressible, callbackCandidates...)
	allCompressible = append(allCompressible, codexCandidates...)
	allCompressible = append(allCompressible, retroCandidates...)
	allCompressible = append(allCompressible, retrievedCandidates...)

	// Deterministic sorting: urgent items first, then closer distance to target chapter, then item ID tie-breaker
	sort.SliceStable(allCompressible, func(i, j int) bool {
		if allCompressible[i].isUrgent != allCompressible[j].isUrgent {
			return allCompressible[i].isUrgent
		}
		if allCompressible[i].distance != allCompressible[j].distance {
			return allCompressible[i].distance < allCompressible[j].distance
		}
		return allCompressible[i].id < allCompressible[j].id
	})

	remainingBudget := budget - totalProtectedTokens
	retainedMap := make(map[string]bool)
	currentRemaining := remainingBudget
	totalEstimatedTokens := totalProtectedTokens
	var droppedItems []CompactionItem

	for _, item := range allCompressible {
		if item.tokens <= currentRemaining {
			currentRemaining -= item.tokens
			totalEstimatedTokens += item.tokens
			retainedMap[item.id] = true
			trace.CompressibleIDs = append(trace.CompressibleIDs, item.id)
		} else {
			trace.DroppedIDs = append(trace.DroppedIDs, item.id)
			trace.Truncated = true
			kind := item.category
			if strings.HasPrefix(item.id, "codex/") {
				kind = "codex"
			} else if strings.HasPrefix(item.id, "callback/") {
				kind = "callback"
			} else if strings.HasPrefix(item.id, "history/retro") || strings.HasPrefix(item.id, "retro/") {
				kind = "retro"
			}
			droppedItems = append(droppedItems, CompactionItem{
				ID:   item.id,
				Kind: kind,
				Text: item.desc,
			})
		}
	}
	trace.EstimatedTokens = totalEstimatedTokens

	var compiledText string
	if len(droppedItems) > 0 && c.compactor != nil {
		remaining := budget - totalEstimatedTokens
		if remaining >= MinCompactionBudgetTokens {
			compiled, cErr := c.compactor(ctx, CompactionInput{
				ChapterIndex: targetChapter,
				Dropped:      droppedItems,
				BudgetTokens: remaining,
			})
			if cErr != nil {
				trace.CompactionFailed = true
				trace.CompactionNote = fmt.Sprintf("语义压缩执行失败: %v", cErr)
			} else if strings.TrimSpace(compiled) == "" {
				trace.CompactionFailed = true
				trace.CompactionNote = "语义压缩输出为空"
			} else if domain.EstimateTokens(compiled) > remaining {
				trace.CompactionFailed = true
				trace.CompactionNote = fmt.Sprintf("语义压缩输出超出剩余预算: 预估 %d tokens 超过剩余预算 %d tokens", domain.EstimateTokens(compiled), remaining)
			} else {
				tokens := domain.EstimateTokens(compiled)
				totalEstimatedTokens += tokens
				trace.EstimatedTokens = totalEstimatedTokens
				trace.CompressibleIDs = append(trace.CompressibleIDs, "memory/compiled-compressible")
				compiledText = compiled
			}
		} else {
			trace.CompactionNote = "剩余预算不足，跳过压缩"
		}
	}

	// Synthesize HistoricalCallbacks text from retained items
	var cbSb strings.Builder
	var retainedCallbackDescs []string
	omittedCallbackCount := 0

	for _, it := range callbackCandidates {
		if retainedMap[it.id] {
			retainedCallbackDescs = append(retainedCallbackDescs, it.desc)
		} else {
			omittedCallbackCount++
		}
	}

	if len(retainedCallbackDescs) > 0 {
		cbSb.WriteString("【跨卷历史因果线索 (Historical Callbacks)】\n")
		cbSb.WriteString(strings.Join(retainedCallbackDescs, "\n\n"))
		if omittedCallbackCount > 0 {
			cbSb.WriteString(fmt.Sprintf("\n\n（另有 %d 处较远期伏笔持续发酵中，已精简现场摘录以保障模型核心注意力）", omittedCallbackCount))
		}
	}

	var retainedRetroDescs []string
	for _, it := range retroCandidates {
		if retainedMap[it.id] {
			retainedRetroDescs = append(retainedRetroDescs, it.desc)
		}
	}

	if len(retainedRetroDescs) > 0 {
		if cbSb.Len() > 0 {
			cbSb.WriteString("\n\n")
		}
		cbSb.WriteString("【往卷全局记忆 (Volume Retrospectives)】\n")
		cbSb.WriteString(strings.Join(retainedRetroDescs, "\n\n"))
	}

	var retainedRetrievedDescs []string
	for _, item := range allCompressible {
		if retainedMap[item.id] && strings.HasPrefix(item.id, "retrieved/") {
			retainedRetrievedDescs = append(retainedRetrievedDescs, item.desc)
			trace.RetrievedIDs = append(trace.RetrievedIDs, strings.TrimPrefix(item.id, "retrieved/"))
		}
	}

	if len(retainedRetrievedDescs) > 0 {
		if cbSb.Len() > 0 {
			cbSb.WriteString("\n\n")
		}
		cbSb.WriteString("【历史检索记忆 (Retrieved Memory)】\n")
		cbSb.WriteString(strings.Join(retainedRetrievedDescs, "\n\n"))
	}

	if compiledText != "" {
		if cbSb.Len() > 0 {
			cbSb.WriteString("\n\n")
		}
		cbSb.WriteString("【压缩记忆 (Compiled Memory)】\n")
		cbSb.WriteString(compiledText)
	}

	// Synthesize Codex text and relations from retained items.
	// Relations segments are already embedded in the retained descs (counted
	// pre-loop), so CodexContextText is assembled purely by joining retained descs;
	// the old post-loop relations query/append block is deleted to avoid double
	// counting. The CodexRelations field is restricted to the retained subset from
	// the pre-queried relations.
	var activeCodex []*domain.CodexEntry
	var codexRels []domain.EntityRelation
	var codexSb strings.Builder
	retainedCodexIDs := make(map[string]bool)

	for _, it := range codexCandidates {
		if retainedMap[it.id] {
			activeCodex = append(activeCodex, it.codexEntry)
			retainedCodexIDs[it.codexEntry.ID] = true
			if codexSb.Len() == 0 {
				codexSb.WriteString("【出场世界观实体与时空状态 (The Codex)】\n")
			}
			codexSb.WriteString(it.desc)
		}
	}

	for _, it := range codexCandidates {
		if !retainedMap[it.id] || it.codexEntry == nil {
			continue
		}
		for _, r := range relStructsBySource[it.codexEntry.ID] {
			if retainedCodexIDs[r.TargetEntryID] {
				codexRels = append(codexRels, r)
			}
		}
	}

	var styleVaultAnchor string
	if vs, ok := c.store.(StyleVaultStore); ok {
		vault, err := EnsureSeededVault(ctx, vs, projectID)
		if err != nil {
			return nil, nil, fmt.Errorf("ensure seeded style vault failed: %w", err)
		}
		if vault != nil {
			styleVaultAnchor = BuildStyleAnchor(vault)
		}
	}

	// Anchor secondary gate: StyleVaultAnchor is generated after the budget loop but
	// is always injected into the prose prompt (anchorSection), so its bytes must be
	// counted here, otherwise trace.EstimatedTokens<=budget would not imply actual
	// prompt<=budget. style/vault_anchor is the full-book constant style contract
	// (主锚); style/tail_anchor above is only the 200-rune auxiliary bridge (辅锚)
	// carrying the previous chapter's closing tone. Protected-overflow style error
	// keeps the tokfix semantics (trace.Truncated=true).
	if styleVaultAnchor != "" {
		trace.ProtectedIDs = append(trace.ProtectedIDs, "style/vault_anchor")
	}
	anchorTokens := domain.EstimateTokens(styleVaultAnchor)
	totalEstimatedTokens += anchorTokens
	trace.EstimatedTokens = totalEstimatedTokens
	if totalEstimatedTokens > budget {
		trace.Truncated = true
		return nil, trace, fmt.Errorf("context budget exceeded after including style vault anchor: estimated %d tokens exceeds budget %d tokens. 超限项清单: [style/vault_anchor: %d tokens]. 建议操作：请在系统设置中调大 context_budget_tokens (当前配置: %d)，或精简文风锚定样本",
			totalEstimatedTokens, budget, anchorTokens, budget)
	}

	return &CanonHorizon{
		Project:             project,
		TargetChapter:       targetChapter,
		GlobalSummary:       globalSummary,
		CurrentVolume:       currentVolume,
		ActivePowerTier:     activeTier,
		RollingCanonText:    strings.TrimSpace(rollingSb.String()),
		TailAnchor:          tailAnchor,
		HistoricalCallbacks: strings.TrimSpace(cbSb.String()),
		RecentChapters:      recent,
		UrgentHooks:         urgentHooks,
		AllActiveHooks:      activeHooks,
		ProtagonistEntry:    protagonist,
		WorldRules:          project.WorldRules,
		ActiveCodexEntries:  activeCodex,
		CodexRelations:      codexRels,
		CodexContextText:    strings.TrimSpace(codexSb.String()),
		StyleVaultAnchor:    styleVaultAnchor,
	}, trace, nil
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

// deriveVolumeSpans builds []domain.VolumeSpan and mapping directly from VolumeArcs' explicit StartChapter/EndChapter.
func deriveVolumeSpans(arcs []domain.VolumeArc) ([]domain.VolumeSpan, map[int]*domain.VolumeArc) {
	spans := make([]domain.VolumeSpan, 0, len(arcs))
	arcMap := make(map[int]*domain.VolumeArc, len(arcs))
	for i := range arcs {
		arc := &arcs[i]
		volIdx := arc.VolumeIndex
		if volIdx == 0 {
			volIdx = i + 1
		}
		spans = append(spans, domain.VolumeSpan{
			VolumeIndex:  volIdx,
			StartChapter: arc.StartChapter,
			EndChapter:   arc.EndChapter,
		})
		arcMap[volIdx] = arc
	}
	return spans, arcMap
}

// resolveVolumeArc locates the VolumeArc for targetChapter using explicit VolumeSpans.
// Returns nil if targetChapter misses (gap, out-of-bounds, or empty spans).
func resolveVolumeArc(targetChapter int, spans []domain.VolumeSpan, arcMap map[int]*domain.VolumeArc) *domain.VolumeArc {
	volIdx := domain.ResolveVolume(targetChapter, spans)
	if volIdx == -1 {
		return nil
	}
	return arcMap[volIdx]
}
