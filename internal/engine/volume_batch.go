package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

type VolumeBatch struct {
	workshop *ChapterWorkshop
	store    store.Store
}

func NewVolumeBatch(w *ChapterWorkshop, s store.Store) *VolumeBatch {
	return &VolumeBatch{
		workshop: w,
		store:    s,
	}
}

type VolumeProduceRequest struct {
	ProjectID       string
	VolumeIndex     int
	ChapterFrom     int // 0 → 卷首章；非 0 → 从 max(卷首章, ChapterFrom) 开始
	WordsTarget     int
	MaxRewriteLoops int
	NarrativeStyle  string
	ReasoningModel  string
	RunID           string // 空则生成一个
	OnProgress      func(VolumeEvent)
}

type VolumeEvent struct {
	Phase        string `json:"phase"` // VOLUME_START | CHAPTER_START | CHAPTER_DONE | VOLUME_STOPPED | VOLUME_COMPLETE
	ChapterIndex int    `json:"chapter_index,omitempty"`
	Message      string `json:"message"`
	Committed    bool   `json:"committed,omitempty"`
	NeedsHuman   bool   `json:"needs_human,omitempty"`
	Verdict      string `json:"verdict,omitempty"`
	Score        int    `json:"score,omitempty"`
}

type VolumeChapterSummary struct {
	ChapterIndex int        `json:"chapter_index"`
	Committed    bool       `json:"committed"`
	NeedsHuman   bool       `json:"needs_human"`
	Verdict      string     `json:"verdict"`
	Score        int        `json:"score"`
	TokenUsage   TokenUsage `json:"token_usage"`
}

type VolumeProduceResult struct {
	VolumeIndex      int                    `json:"volume_index"`
	StartChapter     int                    `json:"start_chapter"`
	EndChapter       int                    `json:"end_chapter"`
	Chapters         []VolumeChapterSummary `json:"chapters"`
	Stopped          bool                   `json:"stopped"`
	StoppedAtChapter int                    `json:"stopped_at_chapter,omitempty"`
	StopReason       string                 `json:"stop_reason,omitempty"` // "HALTED" | "ERROR"
	TotalUsage       TokenUsage             `json:"total_usage"`
	EstimatedCostUSD float64                `json:"estimated_cost_usd"`
}

// ProduceVolume executes autonomous volume batch production.
func (b *VolumeBatch) ProduceVolume(ctx context.Context, req VolumeProduceRequest) (*VolumeProduceResult, error) {
	return b.Produce(ctx, req)
}

// Produce executes autonomous volume batch production.
func (b *VolumeBatch) Produce(ctx context.Context, req VolumeProduceRequest) (*VolumeProduceResult, error) {
	if b == nil || b.workshop == nil || b.store == nil {
		return nil, fmt.Errorf("volume batch dependencies are not fully configured")
	}

	// 1. Fetch project and locate target volume arc (Fail-Closed)
	project, err := b.store.GetProject(ctx, req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", req.ProjectID, err)
	}
	if project == nil {
		return nil, fmt.Errorf("project %s not found", req.ProjectID)
	}
	if project.Framework == nil {
		return nil, fmt.Errorf("project framework is nil for project %s", req.ProjectID)
	}

	arcs := BackfillVolumeArcs(project.Framework.VolumeArcs)
	var targetArc *domain.VolumeArc
	for i := range arcs {
		if arcs[i].VolumeIndex == req.VolumeIndex {
			targetArc = &arcs[i]
			break
		}
	}
	if targetArc == nil {
		return nil, fmt.Errorf("volume %d not found in project %s", req.VolumeIndex, req.ProjectID)
	}

	// 2. Validate and calculate chapter range [max(arc.StartChapter, ChapterFrom), arc.EndChapter]
	startChapter := targetArc.StartChapter
	if req.ChapterFrom > 0 {
		if req.ChapterFrom > targetArc.EndChapter {
			return nil, fmt.Errorf("chapter_from %d exceeds volume end chapter %d", req.ChapterFrom, targetArc.EndChapter)
		}
		if req.ChapterFrom > startChapter {
			startChapter = req.ChapterFrom
		}
	}
	endChapter := targetArc.EndChapter
	if endChapter < startChapter {
		return nil, fmt.Errorf("invalid volume chapter range [%d, %d]", startChapter, endChapter)
	}
	if endChapter-startChapter+1 > 200 {
		return nil, fmt.Errorf("volume chapter span %d exceeds limit of 200", endChapter-startChapter+1)
	}

	// 3. Shared RunID across the entire volume batch
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = activity.NewRunID()
	}
	ctx = activity.WithRunID(ctx, runID)

	result := &VolumeProduceResult{
		VolumeIndex:  req.VolumeIndex,
		StartChapter: startChapter,
		EndChapter:   endChapter,
		Chapters:     make([]VolumeChapterSummary, 0, endChapter-startChapter+1),
		Stopped:      false,
	}

	emit := func(ev VolumeEvent) {
		if req.OnProgress != nil {
			req.OnProgress(ev)
		}
	}

	emit(VolumeEvent{
		Phase:   "VOLUME_START",
		Message: fmt.Sprintf("开始批量生产第 %d 卷 (第 %d-%d 章)...", req.VolumeIndex, startChapter, endChapter),
	})

	sectionsUsage, sErr := b.workshop.EnsureVolumeSections(ctx, req.ProjectID, req.VolumeIndex, req.ReasoningModel)
	if sErr != nil {
		return nil, fmt.Errorf("ensure volume sections for volume %d: %w", req.VolumeIndex, sErr)
	}
	addTokenUsage(&result.TotalUsage, sectionsUsage)

	if p, err := b.store.GetProject(ctx, req.ProjectID); err == nil && p != nil && p.Framework != nil {
		for i := range p.Framework.VolumeArcs {
			if p.Framework.VolumeArcs[i].VolumeIndex == req.VolumeIndex {
				targetArc.Sections = p.Framework.VolumeArcs[i].Sections
				break
			}
		}
	}

	emit(VolumeEvent{Phase: "VOLUME_SECTIONS", Message: fmt.Sprintf("本卷分节大纲就绪（%d 节）", len(targetArc.Sections))})

	for chIdx := startChapter; chIdx <= endChapter; chIdx++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			result.Stopped = true
			result.StopReason = "ERROR"
			result.StoppedAtChapter = chIdx
			emit(VolumeEvent{
				Phase:        "VOLUME_STOPPED",
				ChapterIndex: chIdx,
				Message:      fmt.Sprintf("生产被中断: %v", ctxErr),
			})
			break
		}

		emit(VolumeEvent{
			Phase:        "CHAPTER_START",
			ChapterIndex: chIdx,
			Message:      fmt.Sprintf("开始生产第 %d 章...", chIdx),
		})

		chReq := WorkshopProduceRequest{
			ProjectID:        req.ProjectID,
			ChapterIndex:     chIdx,
			ReasoningModel:   req.ReasoningModel,
			WordsTarget:      req.WordsTarget,
			MaxRewriteLoops:  req.MaxRewriteLoops,
			NarrativeStyle:   req.NarrativeStyle,
			AutoCommit:       true,
			ResumeCheckpoint: true,
			RunID:            runID,
			OnProgress: func(ev WorkshopEvent) {
				emit(VolumeEvent{
					Phase:        string(ev.Phase),
					ChapterIndex: chIdx,
					Message:      ev.Message,
				})
			},
		}

		chRes, chErr := b.workshop.ProduceChapter(ctx, chReq)
		if chErr != nil {
			result.Stopped = true
			result.StopReason = "ERROR"
			result.StoppedAtChapter = chIdx
			emit(VolumeEvent{
				Phase:        "VOLUME_STOPPED",
				ChapterIndex: chIdx,
				Message:      fmt.Sprintf("第 %d 章生产失败: %v", chIdx, chErr),
			})
			break
		}

		var (
			verdict    string
			score      int
			needsHuman bool
			committed  bool
		)
		if chRes != nil {
			committed = chRes.Committed
			if chRes.Audit != nil {
				verdict = string(chRes.Audit.Verdict)
				score = chRes.Audit.Score
				needsHuman = chRes.Audit.NeedsHuman
			}
			result.TotalUsage.PromptTokens += chRes.TotalUsage.PromptTokens
			result.TotalUsage.CompletionTokens += chRes.TotalUsage.CompletionTokens
			result.TotalUsage.TotalTokens += chRes.TotalUsage.TotalTokens
			result.EstimatedCostUSD += chRes.EstimatedCostUSD

			result.Chapters = append(result.Chapters, VolumeChapterSummary{
				ChapterIndex: chIdx,
				Committed:    committed,
				NeedsHuman:   needsHuman,
				Verdict:      verdict,
				Score:        score,
				TokenUsage:   chRes.TotalUsage,
			})
		}

		emit(VolumeEvent{
			Phase:        "CHAPTER_DONE",
			ChapterIndex: chIdx,
			Message:      fmt.Sprintf("第 %d 章生产完成: %s (得分: %d)", chIdx, verdict, score),
			Committed:    committed,
			NeedsHuman:   needsHuman,
			Verdict:      verdict,
			Score:        score,
		})

		if chRes != nil && chRes.Audit != nil && chRes.Audit.NeedsHuman {
			result.Stopped = true
			result.StopReason = "HALTED"
			result.StoppedAtChapter = chIdx
			emit(VolumeEvent{
				Phase:        "VOLUME_STOPPED",
				ChapterIndex: chIdx,
				Message:      fmt.Sprintf("第 %d 章触发停线待人工，整卷生产已终止", chIdx),
				Committed:    committed,
				NeedsHuman:   true,
				Verdict:      verdict,
				Score:        score,
			})
			break
		}
	}

	if !result.Stopped {
		emit(VolumeEvent{
			Phase:   "VOLUME_COMPLETE",
			Message: fmt.Sprintf("第 %d 卷全卷生产完成 (共 %d 章)", req.VolumeIndex, len(result.Chapters)),
		})
	}

	return result, nil
}
