package engine

import (
	"context"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// QualityGate unifies algorithmic statistical linting and semantic LLM review
// into a single cohesive auditing seam, powered by the multi-agent ReviewCourt.
type QualityGate struct {
	linter *Linter
	client LLMClient
	court  *ReviewCourt
}

func NewQualityGate(client LLMClient, customBanned []string) *QualityGate {
	linter := NewLinter(customBanned)
	return &QualityGate{
		linter: linter,
		client: client,
		court:  NewReviewCourt(client, linter),
	}
}

// ReviewCourt returns the underlying multi-agent judicial review court.
func (q *QualityGate) ReviewCourt() *ReviewCourt {
	return q.court
}

// FastAnalyze provides 0ms algorithmic quality analysis (burstiness & cliché detection).
func (q *QualityGate) FastAnalyze(text string) domain.LinterResult {
	return q.linter.Analyze(text)
}

// Audit runs the fast algorithmic heuristic pre-pass followed by deep semantic review,
// combining statistical scores and semantic critique into a single actionable AuditReport.
func (q *QualityGate) Audit(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
	protagonist *domain.CodexEntry,
	chapterIndex int,
	beats []domain.SceneBeat,
	draftText string,
	opts ...ChiefJudgeOptions,
) (*domain.AuditReport, TokenUsage, error) {
	return q.AuditWithHooks(ctx, reviewerModel, project, protagonist, chapterIndex, beats, nil, draftText, opts...)
}

// AuditWithHooks extends Audit with open plot hook awareness to verify hook payoffs and resolutions,
// delegating to the three-division ReviewCourt under Fail-Closed ChiefJudge.
func (q *QualityGate) AuditWithHooks(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
	protagonist *domain.CodexEntry,
	chapterIndex int,
	beats []domain.SceneBeat,
	activeHooks []*domain.PlotHook,
	draftText string,
	opts ...ChiefJudgeOptions,
) (*domain.AuditReport, TokenUsage, error) {
	courtReport, usage, err := q.court.ChiefJudge(ctx, reviewerModel, project, protagonist, chapterIndex, beats, activeHooks, draftText, opts...)
	if err != nil {
		return nil, usage, err
	}
	return courtReport.AuditReport, usage, nil
}
