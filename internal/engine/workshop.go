package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// WorkshopProduceRequest defines parameters for autonomous chapter production.
type WorkshopProduceRequest struct {
	ProjectID       string `json:"project_id"`
	ChapterIndex    int    `json:"chapter_index"`
	CoreConflict    string `json:"core_conflict"`
	ReasoningModel  string `json:"reasoning_model,omitempty"`
	WriterModel     string `json:"writer_model,omitempty"`
	ReviewerModel   string `json:"reviewer_model,omitempty"`
	WordsTarget     int    `json:"words_target,omitempty"`
	AutoCommit      bool   `json:"auto_commit,omitempty"`
	MaxRewriteLoops int    `json:"max_rewrite_loops,omitempty"`
}

// WorkshopProduceResult represents the end-to-end outcome of a chapter workshop run.
type WorkshopProduceResult struct {
	ChapterIndex   int                  `json:"chapter_index"`
	Title          string               `json:"title"`
	Beats          []domain.SceneBeat   `json:"beats"`
	StateMutation  domain.StateMutation `json:"state_mutation"`
	Content        string               `json:"content"`
	Audit          *domain.AuditReport  `json:"audit"`
	RewriteLoops   int                  `json:"rewrite_loops"`
	Committed      bool                 `json:"committed"`
	UpdatedProject *domain.Project      `json:"updated_project,omitempty"`
}

// ChapterWorkshop is the deep orchestration module that encapsulates the entire
// novel drafting, quality gate auditing, targeted rewrite loop, and atomic state commitment.
type ChapterWorkshop struct {
	orch        *Orchestrator
	chronicle   *CanonChronicle
	qualityGate *QualityGate
	store       store.Store
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
	}
}

// ProduceChapter executes the complete autonomous chapter production pipeline:
// 1. Assembles rolling 3-chapter canon horizon and active plot hooks.
// 2. Derives structured scene beats & predicted entity state mutations.
// 3. Renders high-sensory literary chapter draft.
// 4. Audits draft against quality gate (heuristic clichés/burstiness + semantic consistency).
// 5. Autonomously triggers targeted rewrite loops if revisions are needed.
// 6. Optionally commits chapter atomically into SQLite storage.
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

	// 1. Synthesize Canon Horizon
	horizon, err := w.chronicle.AssembleHorizon(ctx, req.ProjectID, req.ChapterIndex)
	if err != nil {
		return nil, fmt.Errorf("assemble canon horizon failed: %w", err)
	}

	// 2. Derive Scene Beats
	beatsOut, err := w.orch.DeriveBeatsWithHorizon(ctx, req.ReasoningModel, horizon, req.CoreConflict)
	if err != nil {
		return nil, fmt.Errorf("derive beats failed: %w", err)
	}

	// 3. Render Literary Scene Draft
	draftText, err := w.orch.RenderScene(ctx, req.WriterModel, horizon.Project, req.ChapterIndex, beatsOut.Beats, req.WordsTarget)
	if err != nil {
		return nil, fmt.Errorf("render scene draft failed: %w", err)
	}

	// 4. Audit via Quality Gate
	auditReport, err := w.qualityGate.Audit(ctx, req.ReviewerModel, horizon.Project, req.ChapterIndex, beatsOut.Beats, draftText)
	if err != nil {
		return nil, fmt.Errorf("quality gate audit failed: %w", err)
	}

	// 5. Targeted Rewrite Loop if revision needed
	rewriteLoops := 0
	currentDraft := draftText
	for auditReport.Verdict == domain.ReviewVerdictRevision && rewriteLoops < req.MaxRewriteLoops {
		rewriteLoops++
		revised, rerr := w.orch.RewriteDraft(ctx, req.WriterModel, horizon.Project, req.ChapterIndex, currentDraft, auditReport.ToReviewResult())
		if rerr != nil {
			break // graceful fallback to current draft if rewrite fails
		}
		currentDraft = revised

		// Re-audit revised draft
		newAudit, aerr := w.qualityGate.Audit(ctx, req.ReviewerModel, horizon.Project, req.ChapterIndex, beatsOut.Beats, currentDraft)
		if aerr == nil {
			auditReport = newAudit
		}
	}

	chapterTitle := fmt.Sprintf("第 %d 章", req.ChapterIndex)
	result := &WorkshopProduceResult{
		ChapterIndex:  req.ChapterIndex,
		Title:         chapterTitle,
		Beats:         beatsOut.Beats,
		StateMutation: beatsOut.StateMutation,
		Content:       currentDraft,
		Audit:         auditReport,
		RewriteLoops:  rewriteLoops,
		Committed:     false,
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
	}

	return result, nil
}
