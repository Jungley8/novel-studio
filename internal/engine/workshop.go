package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// WorkshopPhase indicates the pipeline stage in progress.
type WorkshopPhase string

const (
	PhaseInit      WorkshopPhase = "INIT"
	PhaseHorizon   WorkshopPhase = "HORIZON_ASSEMBLED"
	PhaseBeats     WorkshopPhase = "BEATS_DERIVED"
	PhaseRendering WorkshopPhase = "RENDERING"
	PhaseDrafted   WorkshopPhase = "DRAFTED"
	PhaseAuditing  WorkshopPhase = "AUDITING"
	PhaseRewriting WorkshopPhase = "REWRITING"
	PhaseAudited   WorkshopPhase = "AUDITED"
	PhaseCommitted WorkshopPhase = "COMMITTED"
	PhaseComplete  WorkshopPhase = "COMPLETE"
)

// WorkshopEvent carries real-time streaming progress to SSE or CLI consumers.
type WorkshopEvent struct {
	Phase        WorkshopPhase       `json:"phase"`
	Message      string              `json:"message"`
	Delta        string              `json:"delta,omitempty"`
	CoreConflict string              `json:"core_conflict,omitempty"`
	Beats        []domain.SceneBeat  `json:"beats,omitempty"`
	DraftText    string              `json:"draft_text,omitempty"`
	AuditReport  *domain.AuditReport `json:"audit_report,omitempty"`
	RewriteLoop  int                 `json:"rewrite_loop,omitempty"`
	TotalTokens  int                 `json:"total_tokens,omitempty"`
}

// ProgressCallback receives real-time workshop pipeline events.
type ProgressCallback func(event WorkshopEvent)

// WorkshopProduceRequest defines parameters for autonomous chapter production.
type WorkshopProduceRequest struct {
	ProjectID        string             `json:"project_id"`
	ChapterIndex     int                `json:"chapter_index"`
	CoreConflict     string             `json:"core_conflict"`
	Beats            []domain.SceneBeat `json:"beats,omitempty"`
	InitialDraft     string             `json:"initial_draft,omitempty"`
	ReasoningModel   string             `json:"reasoning_model,omitempty"`
	WriterModel      string             `json:"writer_model,omitempty"`
	ReviewerModel    string             `json:"reviewer_model,omitempty"`
	WordsTarget      int                `json:"words_target,omitempty"`
	NarrativeStyle   string             `json:"narrative_style,omitempty"`
	AutoCommit       bool               `json:"auto_commit,omitempty"`
	MaxRewriteLoops  int                `json:"max_rewrite_loops,omitempty"`
	ResumeCheckpoint bool               `json:"resume_checkpoint,omitempty"`
	EnableHarmonize  bool               `json:"enable_harmonize,omitempty"`
	PerturbIntensity float64            `json:"perturb_intensity,omitempty"`
	RunID            string             `json:"run_id,omitempty"`
	OnProgress       ProgressCallback   `json:"-"`
}

// WorkshopProduceResult represents the end-to-end outcome of a chapter workshop run.
type WorkshopProduceResult struct {
	ChapterIndex         int                         `json:"chapter_index"`
	Title                string                      `json:"title"`
	CoreConflict         string                      `json:"core_conflict,omitempty"`
	Beats                []domain.SceneBeat          `json:"beats"`
	StateMutation        domain.StateMutation        `json:"state_mutation"`
	Content              string                      `json:"content"`
	Audit                *domain.AuditReport         `json:"audit"`
	RewriteLoops         int                         `json:"rewrite_loops"`
	Committed            bool                        `json:"committed"`
	TotalUsage           TokenUsage                  `json:"total_usage"`
	EstimatedCostUSD     float64                     `json:"estimated_cost_usd"`
	ResumedPhase         string                      `json:"resumed_phase,omitempty"`
	UpdatedProject       *domain.Project             `json:"updated_project,omitempty"`
	HarmonizeReport      *HarmonizeReport            `json:"harmonize_report,omitempty"`
	TechnicalBlocked     bool                        `json:"technical_blocked,omitempty"`
	TechnicalBlockReason string                      `json:"technical_block_reason,omitempty"`
	UnmatchedNames       []string                    `json:"unmatched_names,omitempty"`
	SettlementEvidence   []domain.SettlementEvidence `json:"settlement_evidence,omitempty"`
}

// ChapterWorkshop is the deep orchestration module that encapsulates the entire
// novel drafting, quality gate auditing, targeted rewrite loop, and atomic state commitment.
type ChapterWorkshop struct {
	orch        *Orchestrator
	chronicle   *CanonChronicle
	qualityGate *QualityGate
	store       store.Store
	harmonizer  *Harmonizer
}

func NewChapterWorkshop(
	orch *Orchestrator,
	chronicle *CanonChronicle,
	qualityGate *QualityGate,
	store store.Store,
) *ChapterWorkshop {
	return &ChapterWorkshop{
		orch:        orch,
		chronicle:   chronicle,
		qualityGate: qualityGate,
		store:       store,
		harmonizer:  NewHarmonizer(),
	}
}

func (w *ChapterWorkshop) Harmonizer() *Harmonizer {
	return w.harmonizer
}

// ProduceChapter executes the complete autonomous chapter production pipeline:
// 1. Checks and restores checkpoint if previous crash/restart occurred.
// 2. Assembles rolling 3-chapter canon horizon, active plot hooks, and macro progress.
// 3. Derives structured scene beats & predicted entity state mutations.
// 4. Renders high-sensory literary chapter draft.
// 5. Audits draft against quality gate (heuristic clichés/burstiness + semantic consistency).
// 6. Autonomously triggers targeted rewrite loops if revisions are needed.
// 7. Optionally commits chapter atomically into SQLite storage and clears checkpoint.
func (w *ChapterWorkshop) ProduceChapter(ctx context.Context, req WorkshopProduceRequest) (*WorkshopProduceResult, error) {
	if w.orch == nil || w.chronicle == nil || w.qualityGate == nil || w.store == nil {
		return nil, fmt.Errorf("chapter workshop dependencies are not fully configured")
	}

	if req.ChapterIndex <= 0 {
		req.ChapterIndex = 1
	}
	if req.WordsTarget <= 0 {
		req.WordsTarget = 2000
	}
	if req.MaxRewriteLoops <= 0 {
		req.MaxRewriteLoops = 3
	}

	// Ledger correlation: every event of this run shares one run id so the
	// timeline can fold beats/drafts/audits/rewrites into a single group.
	if strings.TrimSpace(req.RunID) == "" {
		req.RunID = activity.NewRunID()
	}
	ctx = activity.WithRunID(ctx, req.RunID)
	if activity.SourceFromCtx(ctx) == activity.SourceDirect {
		ctx = activity.WithSource(ctx, activity.SourceAutoPipe)
	}

	emit := func(ev WorkshopEvent) {
		if req.OnProgress != nil {
			req.OnProgress(ev)
		}
	}

	emit(WorkshopEvent{Phase: PhaseInit, Message: fmt.Sprintf("初始化第 %d 章生产流水线...", req.ChapterIndex)})

	var totalUsage TokenUsage
	addUsage := func(u TokenUsage) {
		totalUsage.PromptTokens += u.PromptTokens
		totalUsage.CompletionTokens += u.CompletionTokens
		totalUsage.TotalTokens += u.TotalTokens
	}

	// 0. Checkpoint Inspection & Resumption
	var (
		resumedPhase      string
		beatsOut          *DeriveBeatsOutput
		draftText         string
		auditReport       *domain.AuditReport
		checkpointHit     bool
		rewriteLoops      int
		rhythmWarning     string
		planInjectWarning string
	)

	if req.ResumeCheckpoint {
		cp, err := w.store.GetCheckpoint(ctx, req.ProjectID, req.ChapterIndex)
		if err == nil && cp != nil {
			checkpointHit = true
			resumedPhase = string(cp.Phase)
			rewriteLoops = cp.RewriteLoops
			if len(cp.Beats) > 0 {
				beatsOut = &DeriveBeatsOutput{
					Beats:         cp.Beats,
					StateMutation: cp.StateMutation,
				}
			}
			if cp.DraftText != "" {
				draftText = cp.DraftText
			}
			if cp.AuditReport != nil {
				auditReport = cp.AuditReport
			}
			emit(WorkshopEvent{
				Phase:   WorkshopPhase(cp.Phase),
				Message: fmt.Sprintf("已从断点恢复 (阶段: %s, 历史返工轮次: %d)", cp.Phase, cp.RewriteLoops),
				Beats:   cp.Beats,
			})
			// Ledger annotation (best-effort: must not break the resumed run).
			_ = w.store.AppendEvent(ctx, &activity.ActivityEvent{
				ProjectID:    req.ProjectID,
				RunID:        req.RunID,
				EntityType:   activity.EntityCheckpoint,
				EntityID:     fmt.Sprintf("cp_%s_%d", req.ProjectID, req.ChapterIndex),
				ChapterIndex: req.ChapterIndex,
				EventType:    activity.EventCheckpointResumed,
				Actor:        activity.ActorFromCtx(ctx),
				Source:       activity.SourceFromCtx(ctx),
				Title:        fmt.Sprintf("第 %d 章从断点恢复（%s）", req.ChapterIndex, cp.Phase),
				Summary:      fmt.Sprintf("历史返工 %d 轮", cp.RewriteLoops),
				CreatedAt:    time.Now(),
			})
		}
	} else {
		// Clean run explicitly requested: wipe out any prior stale checkpoint
		_ = w.store.ClearCheckpoint(ctx, req.ProjectID, req.ChapterIndex)
	}

	// Override beats from request if explicitly provided and not recovered from checkpoint
	if beatsOut == nil && len(req.Beats) > 0 {
		beatsOut = &DeriveBeatsOutput{
			Beats: req.Beats,
		}
	}

	// Caller/UI explicit draft ALWAYS takes precedence over checkpoint!
	if strings.TrimSpace(req.InitialDraft) != "" {
		draftText = strings.TrimSpace(req.InitialDraft)
		// Explicit draft submitted by caller requires fresh quality audit & rewrite loop budget
		auditReport = nil
		rewriteLoops = 0
		emit(WorkshopEvent{
			Phase:       PhaseDrafted,
			Message:     fmt.Sprintf("已载入最新手稿草稿 (共 %d 字)，将在当前手稿基础上执行质检与闭环推演", len([]rune(draftText))),
			DraftText:   draftText,
			TotalTokens: totalUsage.TotalTokens,
		})
	}

	// 1. Synthesize Canon Horizon
	horizon, err := w.chronicle.AssembleHorizon(ctx, req.ProjectID, req.ChapterIndex)
	if err != nil {
		return nil, fmt.Errorf("assemble canon horizon failed: %w", err)
	}
	if req.NarrativeStyle != "" {
		horizon.NarrativeTone = req.NarrativeStyle
	}

	// 1.1 Inject Active Narrative Tree Plan (if available)
	// 取活跃剧情树计划（若从 checkpoint 恢复且已有 beats，虽重取计划注入 horizon，但因 beats 已存在会跳过 DeriveBeats 推演，自然跳过再次影响）
	if ps, ok := w.store.(interface {
		GetActivePlan(ctx context.Context, projectID string) (*domain.ActivePlan, error)
	}); ok {
		plan, pErr := ps.GetActivePlan(ctx, req.ProjectID)
		if pErr != nil {
			planInjectWarning = fmt.Sprintf("【计划注入警告】获取活跃计划失败: %v", pErr)
		} else if plan != nil && plan.Status == domain.PlanStatusActive && strings.TrimSpace(plan.PlanText) != "" {
			horizon.SelectedPlanText = plan.PlanText
			emit(WorkshopEvent{
				Phase:   PhaseHorizon,
				Message: "已注入作者选定的发展路线（剧情树计划），本章节拍将向该方向对齐",
			})
		}
	}

	emit(WorkshopEvent{Phase: PhaseHorizon, Message: "正史视界组装完成 (3章密封正史与开放伏笔已就绪)"})

	// 1.5 Establish High-Tension Chapter Conflict
	if strings.TrimSpace(req.CoreConflict) == "" || (strings.HasPrefix(strings.TrimSpace(req.CoreConflict), "第 ") && strings.Contains(req.CoreConflict, "剧情冲突与关键转折")) {
		emit(WorkshopEvent{Phase: PhaseHorizon, Message: "正在结合前史脉络与分卷大纲，智能推演本章黄金剧情冲突..."})
		suggested, sUsage, sErr := w.orch.SuggestChapterConflict(ctx, req.ReasoningModel, horizon)
		if sErr == nil && strings.TrimSpace(suggested) != "" {
			req.CoreConflict = strings.TrimSpace(suggested)
			addUsage(sUsage)
			emit(WorkshopEvent{
				Phase:        PhaseHorizon,
				Message:      fmt.Sprintf("本章剧情冲突已确立：%s", req.CoreConflict),
				CoreConflict: req.CoreConflict,
			})
		} else if strings.TrimSpace(req.CoreConflict) == "" {
			req.CoreConflict = fmt.Sprintf("第 %d 章核心矛盾爆发与命运转折", req.ChapterIndex)
		}
	}

	// 2. Derive Scene Beats (Skip if recovered from checkpoint or provided in request)
	if beatsOut == nil {
		// 2.1 Retrieve recent chapters and classify recent types
		var recentTypes []domain.ChapterType
		var lastChapterHook string
		if chapters, lErr := w.store.ListChapters(ctx, req.ProjectID); lErr == nil && len(chapters) > 0 {
			var preceding []*domain.Chapter
			for _, ch := range chapters {
				if ch != nil && ch.ChapterIndex < req.ChapterIndex {
					preceding = append(preceding, ch)
				}
			}
			sort.Slice(preceding, func(i, j int) bool {
				return preceding[i].ChapterIndex < preceding[j].ChapterIndex
			})
			startIdx := 0
			if len(preceding) > 5 {
				startIdx = len(preceding) - 5
			}
			recentChs := preceding[startIdx:]
			for _, ch := range recentChs {
				if len(ch.Beats) > 0 {
					recentTypes = append(recentTypes, domain.ClassifyChapterType(ch.Beats))
				}
			}
			if len(preceding) > 0 {
				lastCh := preceding[len(preceding)-1]
				if len(lastCh.Beats) > 0 {
					lastChapterHook = strings.TrimSpace(lastCh.Beats[len(lastCh.Beats)-1].HookType)
				}
			}
		}

		// 2.2 Determine planned chapter type for this chapter
		var planned []domain.ChapterType
		if horizon != nil && horizon.CurrentVolume != nil {
			planned = domain.PlanVolumeRhythm(horizon.CurrentVolume, recentTypes, 8)
		}
		targetType := domain.ChapterTypeEscalation
		foundNonConflict := false
		for _, pt := range planned {
			consec := 1
			for i := len(recentTypes) - 1; i >= 0; i-- {
				if recentTypes[i] == pt {
					consec++
				} else {
					break
				}
			}
			if consec < 4 {
				targetType = pt
				foundNonConflict = true
				break
			}
		}
		if !foundNonConflict {
			candidates := []domain.ChapterType{
				domain.ChapterTypeEscalation,
				domain.ChapterTypeClimax,
				domain.ChapterTypeAftermath,
				domain.ChapterTypeSetup,
				domain.ChapterTypeInterlude,
			}
			for _, c := range candidates {
				consec := 1
				for i := len(recentTypes) - 1; i >= 0; i-- {
					if recentTypes[i] == c {
						consec++
					} else {
						break
					}
				}
				if consec < 4 {
					targetType = c
					break
				}
			}
		}

		// 2.3 Derive beats with chosen chapter type
		bOut, bErr := w.orch.DeriveBeatsForChapterType(ctx, req.ReasoningModel, horizon, req.CoreConflict, targetType)
		if bErr != nil {
			return nil, fmt.Errorf("derive beats failed: %w", bErr)
		}
		beatsOut = bOut
		addUsage(bOut.Usage)

		// 2.4 Check rhythm repetition: consecutive same type >= 4 OR same HookType as previous chapter
		actualType := domain.ClassifyChapterType(beatsOut.Beats)
		consecCount := 1
		for i := len(recentTypes) - 1; i >= 0; i-- {
			if recentTypes[i] == actualType {
				consecCount++
			} else {
				break
			}
		}
		actualHook := ""
		if len(beatsOut.Beats) > 0 {
			actualHook = strings.TrimSpace(beatsOut.Beats[len(beatsOut.Beats)-1].HookType)
		}
		isSameHook := lastChapterHook != "" && actualHook != "" && strings.EqualFold(lastChapterHook, actualHook)
		isConsec4 := consecCount >= 4

		// Retry at most once if repetition detected
		if isConsec4 || isSameHook {
			altCandidates := []domain.ChapterType{
				domain.ChapterTypeClimax,
				domain.ChapterTypeAftermath,
				domain.ChapterTypeSetup,
				domain.ChapterTypeInterlude,
				domain.ChapterTypeEscalation,
			}
			altType := domain.ChapterTypeEscalation
			for _, c := range altCandidates {
				if c != actualType && c != targetType {
					consec := 1
					for i := len(recentTypes) - 1; i >= 0; i-- {
						if recentTypes[i] == c {
							consec++
						} else {
							break
						}
					}
					if consec < 4 {
						altType = c
						break
					}
				}
			}
			if altType == "" {
				if actualType == domain.ChapterTypeEscalation {
					altType = domain.ChapterTypeSetup
				} else {
					altType = domain.ChapterTypeEscalation
				}
			}

			altOut, altErr := w.orch.DeriveBeatsForChapterType(ctx, req.ReasoningModel, horizon, req.CoreConflict, altType)
			if altErr == nil {
				beatsOut = altOut
				addUsage(altOut.Usage)
				actualType = domain.ClassifyChapterType(beatsOut.Beats)
				consecCount = 1
				for i := len(recentTypes) - 1; i >= 0; i-- {
					if recentTypes[i] == actualType {
						consecCount++
					} else {
						break
					}
				}
			}
		}

		if consecCount >= 4 {
			rhythmWarning = fmt.Sprintf("【节奏提示】连续同型%d章", consecCount)
		}

		// Save Checkpoint after beats derivation (must persist final beats)
		_ = w.store.SaveCheckpoint(activity.WithActor(ctx, activity.ActorAIReasoner), &domain.ChapterCheckpoint{
			ProjectID:     req.ProjectID,
			ChapterIndex:  req.ChapterIndex,
			Phase:         domain.CheckpointPhaseBeats,
			CoreConflict:  req.CoreConflict,
			Beats:         beatsOut.Beats,
			StateMutation: beatsOut.StateMutation,
		})
		msg := fmt.Sprintf("节拍推演完成 (%d个张力节拍与状态变迁生成)", len(beatsOut.Beats))
		if rhythmWarning != "" {
			msg += " " + rhythmWarning
		}
		emit(WorkshopEvent{
			Phase:       PhaseBeats,
			Message:     msg,
			Beats:       beatsOut.Beats,
			TotalTokens: totalUsage.TotalTokens,
		})
	}

	// 3. Render Literary Scene Draft (Skip if already recovered from checkpoint or explicit draft provided)
	if draftText == "" {
		emit(WorkshopEvent{Phase: PhaseRendering, Message: "正在进行文学高张力渲染..."})
		rendered, usage, rErr := w.orch.RenderSceneWithHorizon(ctx, req.WriterModel, horizon, beatsOut.Beats, req.WordsTarget)
		if rErr != nil {
			return nil, fmt.Errorf("render scene draft failed: %w", rErr)
		}
		draftText = rendered
		addUsage(usage)

		// Optional: Apply Censor Harmonization & Adversarial Perturbation
		if req.EnableHarmonize || req.PerturbIntensity > 0 {
			intensity := req.PerturbIntensity
			if intensity <= 0 {
				intensity = 0.6
			}
			harmonized, hReport := w.harmonizer.FullProcess(draftText, intensity)
			if len(hReport.HarmonizedItems) > 0 {
				emit(WorkshopEvent{
					Phase:   PhaseDrafted,
					Message: fmt.Sprintf("已完成国内平台合规脱敏和谐 (%d 处高危敏感词平滑替换)", len(hReport.HarmonizedItems)),
				})
			}
			draftText = harmonized
		}

		// Save Checkpoint after initial rendering
		_ = w.store.SaveCheckpoint(activity.WithActor(ctx, activity.ActorAIWriter), &domain.ChapterCheckpoint{
			ProjectID:     req.ProjectID,
			ChapterIndex:  req.ChapterIndex,
			Phase:         domain.CheckpointPhaseDrafted,
			CoreConflict:  req.CoreConflict,
			Beats:         beatsOut.Beats,
			StateMutation: beatsOut.StateMutation,
			DraftText:     draftText,
		})
		emit(WorkshopEvent{
			Phase:       PhaseDrafted,
			Message:     fmt.Sprintf("正文初稿渲染完成 (共 %d 字)", len([]rune(draftText))),
			DraftText:   draftText,
			TotalTokens: totalUsage.TotalTokens,
		})
	}

	// 3.5 Settle Entity State Mutations from Draft Text
	emit(WorkshopEvent{Phase: PhaseDrafted, Message: "正在严格依据正文白纸黑字结算角色状态变迁与证据链..."})
	var (
		settledMutation      domain.StateMutation
		settleEvidences      []domain.SettlementEvidence
		technicalBlocked     bool
		technicalBlockReason string
	)
	sMut, sEvs, sUsage, sErr := SettleFromDraft(ctx, w.orch.client, req.ReasoningModel, horizon.Project, horizon.ProtagonistEntry, draftText, req.ChapterIndex)
	addUsage(sUsage)
	if sErr != nil {
		technicalBlocked = true
		technicalBlockReason = sErr.Error()
		emit(WorkshopEvent{
			Phase:   PhaseDrafted,
			Message: fmt.Sprintf("状态结算技术预检未通过 (Fail-Closed): %v", sErr),
		})
	} else {
		settledMutation = sMut
		settleEvidences = sEvs
	}

	// 4. Audit via Quality Gate (with plot hooks resolution awareness)
	if auditReport == nil {
		emit(WorkshopEvent{Phase: PhaseAuditing, Message: "正在执行 QualityGate 算法质检与主审综合审校..."})
		audit, usage, aErr := w.qualityGate.AuditWithHooks(ctx, req.ReviewerModel, horizon.Project, horizon.ProtagonistEntry, req.ChapterIndex, beatsOut.Beats, horizon.AllActiveHooks, draftText)
		if aErr != nil {
			return nil, fmt.Errorf("quality gate audit failed: %w", aErr)
		}
		auditReport = audit
		addUsage(usage)

		// Save Checkpoint after audit
		_ = w.store.SaveCheckpoint(activity.WithActor(ctx, activity.ActorAIReviewer), &domain.ChapterCheckpoint{
			ProjectID:     req.ProjectID,
			ChapterIndex:  req.ChapterIndex,
			Phase:         domain.CheckpointPhaseAudited,
			CoreConflict:  req.CoreConflict,
			Beats:         beatsOut.Beats,
			StateMutation: beatsOut.StateMutation,
			DraftText:     draftText,
			AuditReport:   auditReport,
		})
		emit(WorkshopEvent{
			Phase:       PhaseAudited,
			Message:     fmt.Sprintf("综合质检完成: 评级 %s | 得分 %d", auditReport.Verdict, auditReport.Score),
			AuditReport: auditReport,
			TotalTokens: totalUsage.TotalTokens,
		})
	}
	if auditReport != nil && technicalBlocked {
		auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【TECHNICAL BLOCKED】状态结算失败: %s，已阻断归档入库", technicalBlockReason))
	}
	if auditReport != nil && rhythmWarning != "" {
		auditReport.Issues = append(auditReport.Issues, rhythmWarning)
	}
	if auditReport != nil && planInjectWarning != "" {
		auditReport.Issues = append(auditReport.Issues, planInjectWarning)
	}

	// 5. Targeted Rewrite Loop if revision needed
	currentDraft := draftText
	for auditReport.Verdict == domain.ReviewVerdictRevision && rewriteLoops < req.MaxRewriteLoops {
		rewriteLoops++
		emit(WorkshopEvent{
			Phase:       PhaseRewriting,
			Message:     fmt.Sprintf("触发展开第 %d/%d 轮定向精修...", rewriteLoops, req.MaxRewriteLoops),
			RewriteLoop: rewriteLoops,
		})

		revised, usage, rerr := w.orch.RewriteDraftWithOptions(ctx, req.WriterModel, horizon.Project, req.ChapterIndex, currentDraft, auditReport.ToReviewResult(), RewriteOptions{
			WordsTarget:    req.WordsTarget,
			NarrativeStyle: req.NarrativeStyle,
		})
		if rerr != nil {
			break // fallback gracefully to current draft
		}
		currentDraft = revised
		addUsage(usage)

		// Re-audit revised draft
		newAudit, auditUsage, aerr := w.qualityGate.AuditWithHooks(ctx, req.ReviewerModel, horizon.Project, horizon.ProtagonistEntry, req.ChapterIndex, beatsOut.Beats, horizon.AllActiveHooks, currentDraft)
		if aerr == nil {
			auditReport = newAudit
			addUsage(auditUsage)
		}

		// Save checkpoint for rewrite loop
		_ = w.store.SaveCheckpoint(activity.WithActor(ctx, activity.ActorAIRewriter), &domain.ChapterCheckpoint{
			ProjectID:     req.ProjectID,
			ChapterIndex:  req.ChapterIndex,
			Phase:         domain.CheckpointPhaseRewriting,
			CoreConflict:  req.CoreConflict,
			Beats:         beatsOut.Beats,
			StateMutation: beatsOut.StateMutation,
			DraftText:     currentDraft,
			AuditReport:   auditReport,
			RewriteLoops:  rewriteLoops,
		})

		emit(WorkshopEvent{
			Phase:       PhaseAudited,
			Message:     fmt.Sprintf("第 %d 轮精修后质检: 评级 %s | 得分 %d", rewriteLoops, auditReport.Verdict, auditReport.Score),
			DraftText:   currentDraft,
			AuditReport: auditReport,
			RewriteLoop: rewriteLoops,
			TotalTokens: totalUsage.TotalTokens,
		})
	}

	// 5.5 Re-settle after rewrite loop if draft text has changed (重写后必须重新结算)
	if currentDraft != draftText {
		emit(WorkshopEvent{
			Phase:   PhaseDrafted,
			Message: "正文已发生重写变动，重新执行终审状态结算...",
		})
		reMut, reEvs, reUsage, reErr := SettleFromDraft(ctx, w.orch.client, req.ReasoningModel, horizon.Project, horizon.ProtagonistEntry, currentDraft, req.ChapterIndex)
		addUsage(reUsage)
		if reErr != nil {
			technicalBlocked = true
			technicalBlockReason = reErr.Error()
			settledMutation = domain.StateMutation{}
			settleEvidences = nil
			if auditReport != nil {
				auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【TECHNICAL BLOCKED】重写后状态结算失败: %s，已阻断归档入库", technicalBlockReason))
			}
			emit(WorkshopEvent{
				Phase:   PhaseDrafted,
				Message: fmt.Sprintf("重写后状态结算未通过 (Fail-Closed): %v", reErr),
			})
		} else {
			technicalBlocked = false
			technicalBlockReason = ""
			settledMutation = reMut
			settleEvidences = reEvs
		}
	}

	// 5.55 Cross-chapter echo detection (non-blocking audit issues)
	allChapters, listErr := w.store.ListChapters(ctx, req.ProjectID)
	if listErr != nil {
		if auditReport != nil {
			auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【回声警告】检测失败: %v", listErr))
		}
	} else {
		var pastChapters []*domain.Chapter
		for _, ch := range allChapters {
			if ch != nil && ch.ChapterIndex < req.ChapterIndex {
				pastChapters = append(pastChapters, ch)
			}
		}
		sort.Slice(pastChapters, func(i, j int) bool {
			return pastChapters[i].ChapterIndex < pastChapters[j].ChapterIndex
		})
		if len(pastChapters) > 20 {
			pastChapters = pastChapters[len(pastChapters)-20:]
		}
		recentTexts := make([]string, 0, len(pastChapters))
		for _, ch := range pastChapters {
			recentTexts = append(recentTexts, ch.Content)
		}
		echoPhrases := DetectEchoPhrases(recentTexts, currentDraft)
		if auditReport != nil {
			for _, ph := range echoPhrases {
				auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【跨章重复警告】短语「%s」在近 20 章中反复出现，建议换写。", ph))
			}
		}
	}

	// 5.6 Summarize chapter if not technically blocked.
	// 摘要失败永不阻塞归档（注释写明理由：摘要可由正文重建，与状态真相不同级）。
	if !technicalBlocked {
		cs, csUsage, csErr := SummarizeChapter(ctx, w.orch.client, req.ReasoningModel, horizon.Project, req.ChapterIndex, currentDraft, settledMutation)
		if csErr != nil {
			if auditReport != nil {
				auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【摘要警告】生成章节摘要失败: %v", csErr))
			}
		} else {
			addUsage(csUsage)
			if saver, ok := w.store.(interface {
				SaveChapterSummary(ctx context.Context, cs *domain.ChapterSummary) error
			}); ok {
				if cs.ProjectID == "" {
					cs.ProjectID = req.ProjectID
				}
				if err := saver.SaveChapterSummary(ctx, &cs); err != nil {
					if auditReport != nil {
						auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【摘要警告】保存章节摘要失败: %v", err))
					}
				}
			} else {
				if auditReport != nil {
					auditReport.Issues = append(auditReport.Issues, "【摘要警告】存储层未实现 SaveChapterSummary 接口")
				}
			}
		}
	}

	// Collect unmatched character mutation names against Codex entries
	unmatchedNames := w.findUnmatchedCharacterNames(ctx, req.ProjectID, settledMutation.CharacterMutations)
	if len(unmatchedNames) > 0 {
		for _, name := range unmatchedNames {
			if auditReport != nil {
				auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【配角连续性警告】正文结算出现未登记在 Codex 的配角变迁: %q (unmatched_name)，请确认是否笔误写漂", name))
			}
		}
	}
	if auditReport != nil && rhythmWarning != "" {
		hasWarn := false
		for _, iss := range auditReport.Issues {
			if iss == rhythmWarning {
				hasWarn = true
				break
			}
		}
		if !hasWarn {
			auditReport.Issues = append(auditReport.Issues, rhythmWarning)
		}
	}
	if auditReport != nil && planInjectWarning != "" {
		hasWarn := false
		for _, iss := range auditReport.Issues {
			if iss == planInjectWarning {
				hasWarn = true
				break
			}
		}
		if !hasWarn {
			auditReport.Issues = append(auditReport.Issues, planInjectWarning)
		}
	}

	// Cost estimation: ~$0.00014 per 1k prompt tokens, ~$0.00028 per 1k completion tokens
	estimatedCostUSD := (float64(totalUsage.PromptTokens)*0.00014 + float64(totalUsage.CompletionTokens)*0.00028) / 1000.0

	chapterTitle := fmt.Sprintf("第 %d 章", req.ChapterIndex)
	result := &WorkshopProduceResult{
		ChapterIndex:         req.ChapterIndex,
		Title:                chapterTitle,
		CoreConflict:         req.CoreConflict,
		Beats:                beatsOut.Beats,
		StateMutation:        settledMutation,
		Content:              currentDraft,
		Audit:                auditReport,
		RewriteLoops:         rewriteLoops,
		Committed:            false,
		TotalUsage:           totalUsage,
		EstimatedCostUSD:     estimatedCostUSD,
		TechnicalBlocked:     technicalBlocked,
		TechnicalBlockReason: technicalBlockReason,
		UnmatchedNames:       unmatchedNames,
		SettlementEvidence:   settleEvidences,
	}
	if checkpointHit {
		result.ResumedPhase = resumedPhase
	}
	if req.EnableHarmonize || req.PerturbIntensity > 0 {
		_, rpt := w.harmonizer.FullProcess(currentDraft, 0)
		result.HarmonizeReport = &rpt
	}

	// 6. Optional Atomic Commitment (Strict Fail-Closed QualityGate)
	shouldCommit := false
	if req.AutoCommit {
		if technicalBlocked {
			// Technical blockage takes absolute precedence over semantic verdict (Fail-Closed)
			shouldCommit = false
			emit(WorkshopEvent{
				Phase:       PhaseComplete,
				Message:     fmt.Sprintf("技术阻断停线: 状态结算未通过 (%s)，拒绝自动入库", technicalBlockReason),
				TotalTokens: totalUsage.TotalTokens,
			})
		} else if auditReport.Verdict == domain.ReviewVerdictAccepted {
			shouldCommit = true
		} else {
			// TODO: Downstream callers (Frontend, CLI, batch production orchestrator) consume NeedsHuman directly instead of matching Issues text.
			auditReport.NeedsHuman = true
			auditReport.Issues = append(auditReport.Issues, "【ESCALATED】已达最大重写轮次仍未通过，已停线待人工，拒绝自动入库")
			emit(WorkshopEvent{
				Phase:       PhaseComplete,
				Message:     fmt.Sprintf("停线待人工: 章节质检未通过 (评级 %s | 得分 %d)，已达最大重写轮次，拒绝自动入库", auditReport.Verdict, auditReport.Score),
				TotalTokens: totalUsage.TotalTokens,
			})
		}
	}

	if technicalBlocked {
		// When technical blocked, ensure checkpoint remains strictly at PhaseDrafted (do not advance or archive)
		_ = w.store.SaveCheckpoint(activity.WithActor(ctx, activity.ActorAIWriter), &domain.ChapterCheckpoint{
			ProjectID:     req.ProjectID,
			ChapterIndex:  req.ChapterIndex,
			Phase:         domain.CheckpointPhaseDrafted,
			CoreConflict:  req.CoreConflict,
			Beats:         beatsOut.Beats,
			StateMutation: beatsOut.StateMutation,
			DraftText:     currentDraft,
		})
	}

	if shouldCommit {
		chapter := &domain.Chapter{
			ID:              fmt.Sprintf("ch_%s_%d", req.ProjectID, req.ChapterIndex),
			ProjectID:       req.ProjectID,
			ChapterIndex:    req.ChapterIndex,
			Title:           chapterTitle,
			CoreConflict:    req.CoreConflict,
			Beats:           beatsOut.Beats,
			StateMutation:   settledMutation,
			Content:         currentDraft,
			WordCount:       len([]rune(currentDraft)),
			BurstinessScore: auditReport.BurstinessScore,
			LinterPassed:    true,
			Review:          auditReport.ToReviewResult(),
			CreatedAt:       time.Now(),
		}

		updatedProj, cerr := w.store.CommitChapter(activity.WithActor(ctx, activity.ActorSystem), req.ProjectID, chapter)
		if cerr != nil {
			return nil, fmt.Errorf("atomic chapter commitment failed: %w", cerr)
		}
		result.Committed = true
		result.UpdatedProject = updatedProj

		// Clear checkpoint after successful commit
		_ = w.store.ClearCheckpoint(ctx, req.ProjectID, req.ChapterIndex)

		emit(WorkshopEvent{
			Phase:   PhaseCommitted,
			Message: fmt.Sprintf("第 %d 章已成功原子结算并归档入库！", req.ChapterIndex),
		})

		// Volume retrospective on volume arc completion.
		// 无 Framework 或算不出卷边界时跳过（不报错）。
		activeProj := result.UpdatedProject
		if activeProj == nil {
			activeProj = horizon.Project
		}
		if activeProj != nil && activeProj.Framework != nil && len(activeProj.Framework.VolumeArcs) > 0 {
			spans, arcMap := deriveVolumeSpans(activeProj.Framework.VolumeArcs)
			volIdx := domain.ResolveVolume(req.ChapterIndex, spans)
			if volIdx != -1 {
				var currentSpan *domain.VolumeSpan
				for i := range spans {
					if spans[i].VolumeIndex == volIdx {
						currentSpan = &spans[i]
						break
					}
				}
				if currentSpan != nil && req.ChapterIndex == currentSpan.EndChapter {
					arc := arcMap[volIdx]
					if arc != nil {
						if vrStore, ok := w.store.(interface {
							ListChapterSummaries(ctx context.Context, projectID string, from, to int) ([]*domain.ChapterSummary, error)
							SaveVolumeRetrospective(ctx context.Context, retro *domain.VolumeRetrospective) error
						}); ok {
							projID := req.ProjectID
							if activeProj.ID != "" {
								projID = activeProj.ID
							}
							summaries, lErr := vrStore.ListChapterSummaries(ctx, projID, currentSpan.StartChapter, currentSpan.EndChapter)
							if lErr != nil {
								if auditReport != nil {
									auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【摘要警告】获取分卷章节摘要失败: %v", lErr))
								}
							} else {
								var chSummaries []ChapterSummary
								for _, s := range summaries {
									if s != nil {
										chSummaries = append(chSummaries, *s)
									}
								}
								if len(chSummaries) == 0 {
									if auditReport != nil {
										auditReport.Issues = append(auditReport.Issues, "【摘要警告】未找到本卷任何章节摘要，跳过分卷回顾")
									}
								} else {
									vr, vrUsage, vrErr := BuildVolumeRetrospective(ctx, w.orch.client, req.ReasoningModel, activeProj, *arc, chSummaries)
									if vrErr != nil {
										if auditReport != nil {
											auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【摘要警告】生成分卷回顾失败: %v", vrErr))
										}
									} else {
										addUsage(vrUsage)
										estimatedCostUSD = (float64(totalUsage.PromptTokens)*0.00014 + float64(totalUsage.CompletionTokens)*0.00028) / 1000.0
										result.TotalUsage = totalUsage
										result.EstimatedCostUSD = estimatedCostUSD
										if vr.ProjectID == "" {
											vr.ProjectID = projID
										}
										if sErr := vrStore.SaveVolumeRetrospective(ctx, &vr); sErr != nil {
											if auditReport != nil {
												auditReport.Issues = append(auditReport.Issues, fmt.Sprintf("【摘要警告】保存分卷回顾失败: %v", sErr))
											}
										}
									}
								}
							}
						} else {
							if auditReport != nil {
								auditReport.Issues = append(auditReport.Issues, "【摘要警告】存储层未实现分卷回顾持久化接口")
							}
						}
					}
				}
			}
		}
	}

	emit(WorkshopEvent{
		Phase:       PhaseComplete,
		Message:     fmt.Sprintf("全流程生产完成！总Token消耗: %d", totalUsage.TotalTokens),
		TotalTokens: totalUsage.TotalTokens,
	})

	// Ledger aggregate marker (best-effort annotation; per-phase state events
	// are already recorded transactionally by the store layer).
	usageBytes, _ := json.Marshal(totalUsage)
	pipeSummary := fmt.Sprintf("终审 %s %d 分 · 返工 %d 轮 · %d tokens（$%.4f）· 归档=%v",
		auditReport.Verdict, auditReport.Score, rewriteLoops,
		totalUsage.TotalTokens, estimatedCostUSD, result.Committed)
	_ = w.store.AppendEvent(ctx, &activity.ActivityEvent{
		ProjectID:    req.ProjectID,
		RunID:        req.RunID,
		EntityType:   activity.EntityWorkshop,
		EntityID:     req.RunID,
		ChapterIndex: req.ChapterIndex,
		EventType:    activity.EventWorkshopCompleted,
		Actor:        activity.ActorSystem,
		Source:       activity.SourceFromCtx(ctx),
		Title:        fmt.Sprintf("第 %d 章流水线完成", req.ChapterIndex),
		Summary:      pipeSummary,
		TokenUsage:   string(usageBytes),
		CreatedAt:    time.Now(),
	})

	return result, nil
}

func (w *ChapterWorkshop) findUnmatchedCharacterNames(ctx context.Context, projectID string, cms []domain.CharacterMutation) []string {
	if len(cms) == 0 {
		return nil
	}
	entries, err := w.store.ListCodexEntries(ctx, projectID, "")
	if err != nil {
		return nil
	}
	knownNames := make(map[string]bool)
	for _, e := range entries {
		knownNames[strings.TrimSpace(e.Name)] = true
		for _, a := range e.Aliases {
			knownNames[strings.TrimSpace(a)] = true
		}
	}
	var unmatched []string
	seen := make(map[string]bool)
	for _, cm := range cms {
		name := strings.TrimSpace(cm.Name)
		if name == "" || knownNames[name] || seen[name] {
			continue
		}
		seen[name] = true
		unmatched = append(unmatched, name)
	}
	return unmatched
}
