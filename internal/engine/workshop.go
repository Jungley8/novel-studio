package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	Phase       WorkshopPhase       `json:"phase"`
	Message     string              `json:"message"`
	Delta       string              `json:"delta,omitempty"`
	Beats       []domain.SceneBeat  `json:"beats,omitempty"`
	DraftText   string              `json:"draft_text,omitempty"`
	AuditReport *domain.AuditReport `json:"audit_report,omitempty"`
	RewriteLoop int                 `json:"rewrite_loop,omitempty"`
	TotalTokens int                 `json:"total_tokens,omitempty"`
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
	OnProgress       ProgressCallback   `json:"-"`
}

// WorkshopProduceResult represents the end-to-end outcome of a chapter workshop run.
type WorkshopProduceResult struct {
	ChapterIndex     int                  `json:"chapter_index"`
	Title            string               `json:"title"`
	Beats            []domain.SceneBeat   `json:"beats"`
	StateMutation    domain.StateMutation `json:"state_mutation"`
	Content          string               `json:"content"`
	Audit            *domain.AuditReport  `json:"audit"`
	RewriteLoops     int                  `json:"rewrite_loops"`
	Committed        bool                 `json:"committed"`
	TotalUsage       TokenUsage           `json:"total_usage"`
	EstimatedCostUSD float64              `json:"estimated_cost_usd"`
	ResumedPhase     string               `json:"resumed_phase,omitempty"`
	UpdatedProject   *domain.Project      `json:"updated_project,omitempty"`
	HarmonizeReport  *HarmonizeReport     `json:"harmonize_report,omitempty"`
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
		resumedPhase  string
		beatsOut      *DeriveBeatsOutput
		draftText     string
		auditReport   *domain.AuditReport
		checkpointHit bool
		rewriteLoops  int
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
	emit(WorkshopEvent{Phase: PhaseHorizon, Message: "正史视界组装完成 (3章密封正史与开放伏笔已就绪)"})

	// 2. Derive Scene Beats (Skip if recovered from checkpoint or provided in request)
	if beatsOut == nil {
		bOut, bErr := w.orch.DeriveBeatsWithHorizon(ctx, req.ReasoningModel, horizon, req.CoreConflict)
		if bErr != nil {
			return nil, fmt.Errorf("derive beats failed: %w", bErr)
		}
		beatsOut = bOut
		addUsage(bOut.Usage)

		// Save Checkpoint after beats derivation
		_ = w.store.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
			ProjectID:     req.ProjectID,
			ChapterIndex:  req.ChapterIndex,
			Phase:         domain.CheckpointPhaseBeats,
			CoreConflict:  req.CoreConflict,
			Beats:         beatsOut.Beats,
			StateMutation: beatsOut.StateMutation,
		})
		emit(WorkshopEvent{
			Phase:       PhaseBeats,
			Message:     "节拍推演完成 (4个张力节拍与状态变迁生成)",
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
			_ = w.store.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
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

	// 4. Audit via Quality Gate (with plot hooks resolution awareness)
	if auditReport == nil {
		emit(WorkshopEvent{Phase: PhaseAuditing, Message: "正在执行 QualityGate 算法质检与主审综合审校..."})
		audit, usage, aErr := w.qualityGate.AuditWithHooks(ctx, req.ReviewerModel, horizon.Project, req.ChapterIndex, beatsOut.Beats, horizon.AllActiveHooks, draftText)
		if aErr != nil {
			return nil, fmt.Errorf("quality gate audit failed: %w", aErr)
		}
		auditReport = audit
		addUsage(usage)

		// Save Checkpoint after audit
		_ = w.store.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
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
		newAudit, auditUsage, aerr := w.qualityGate.AuditWithHooks(ctx, req.ReviewerModel, horizon.Project, req.ChapterIndex, beatsOut.Beats, horizon.AllActiveHooks, currentDraft)
		if aerr == nil {
			auditReport = newAudit
			addUsage(auditUsage)
		}

		// Save checkpoint for rewrite loop
		_ = w.store.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
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

	// Cost estimation: ~$0.00014 per 1k prompt tokens, ~$0.00028 per 1k completion tokens
	estimatedCostUSD := (float64(totalUsage.PromptTokens)*0.00014 + float64(totalUsage.CompletionTokens)*0.00028) / 1000.0

	chapterTitle := fmt.Sprintf("第 %d 章", req.ChapterIndex)
	result := &WorkshopProduceResult{
		ChapterIndex:     req.ChapterIndex,
		Title:            chapterTitle,
		Beats:            beatsOut.Beats,
		StateMutation:    beatsOut.StateMutation,
		Content:          currentDraft,
		Audit:            auditReport,
		RewriteLoops:     rewriteLoops,
		Committed:        false,
		TotalUsage:       totalUsage,
		EstimatedCostUSD: estimatedCostUSD,
	}
	if checkpointHit {
		result.ResumedPhase = resumedPhase
	}
	if req.EnableHarmonize || req.PerturbIntensity > 0 {
		_, rpt := w.harmonizer.FullProcess(currentDraft, 0)
		result.HarmonizeReport = &rpt
	}

	// 6. Optional Atomic Commitment
	if req.AutoCommit && auditReport.Verdict == domain.ReviewVerdictAccepted {
		chapter := &domain.Chapter{
			ID:              fmt.Sprintf("ch_%s_%d", req.ProjectID, req.ChapterIndex),
			ProjectID:       req.ProjectID,
			ChapterIndex:    req.ChapterIndex,
			Title:           chapterTitle,
			CoreConflict:    req.CoreConflict,
			Beats:           beatsOut.Beats,
			StateMutation:   beatsOut.StateMutation,
			Content:         currentDraft,
			WordCount:       len([]rune(currentDraft)),
			BurstinessScore: auditReport.BurstinessScore,
			LinterPassed:    auditReport.Verdict == domain.ReviewVerdictAccepted,
			Review:          auditReport.ToReviewResult(),
			CreatedAt:       time.Now(),
		}

		updatedProj, cerr := w.store.CommitChapter(ctx, req.ProjectID, chapter)
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
	}

	emit(WorkshopEvent{
		Phase:       PhaseComplete,
		Message:     fmt.Sprintf("全流程生产完成！总Token消耗: %d", totalUsage.TotalTokens),
		TotalTokens: totalUsage.TotalTokens,
	})

	return result, nil
}
