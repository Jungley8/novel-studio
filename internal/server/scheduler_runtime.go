package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type schedulerRuntime struct {
	srv *Server
}

func (r *schedulerRuntime) resolveDefaults() (words int, loops int, style string, reasoningModel, writerModel, reviewerModel string) {
	words = 2000
	loops = 3
	style = ""
	if r.srv != nil && r.srv.cfg != nil {
		if r.srv.cfg.DefaultWordsTarget > 0 {
			words = r.srv.cfg.DefaultWordsTarget
		}
		if r.srv.cfg.DefaultMaxRewriteLoops > 0 {
			loops = r.srv.cfg.DefaultMaxRewriteLoops
		}
		style = r.srv.cfg.DefaultNarrativeStyle
		reasoningModel = r.srv.cfg.ReasoningModel
		writerModel = r.srv.cfg.WriterModel
		reviewerModel = r.srv.cfg.ReviewerModel
		if reviewerModel == "" {
			reviewerModel = reasoningModel
		}
	}
	return words, loops, style, reasoningModel, writerModel, reviewerModel
}

// nextChapterIndex 计算下一章序号：max(ChapterIndex)+1，空 → 1。
func nextChapterIndex(chapters []*domain.Chapter) int {
	maxIdx := 0
	for _, ch := range chapters {
		if ch != nil && ch.ChapterIndex > maxIdx {
			maxIdx = ch.ChapterIndex
		}
	}
	if maxIdx <= 0 {
		return 1
	}
	return maxIdx + 1
}

// findArcForChapter 查找包含 ch 的分卷（StartChapter <= ch <= EndChapter）；无 → nil。
func findArcForChapter(arcs []domain.VolumeArc, ch int) *domain.VolumeArc {
	for i := range arcs {
		if arcs[i].StartChapter <= ch && ch <= arcs[i].EndChapter {
			return &arcs[i]
		}
	}
	return nil
}

// RunContinuation 续产：kind=chapters → 连续生产 count 章；kind=volumes → 连续生产 count 卷。
func (r *schedulerRuntime) RunContinuation(
	ctx context.Context,
	projectID string,
	kind domain.AutomationQuotaKind,
	count int,
	progress func(string),
) (halted bool, message string, err error) {
	if r.srv == nil || r.srv.workshop == nil || r.srv.store == nil {
		return false, "", errors.New("workshop or store not configured")
	}

	// Fail-Closed：跨进程生产锁。存在有效编辑租约（守护 UI/CLI）或已有生产任务时
	// 返回 coord.ErrBusy；调度器据此跳过本轮并在下一趟自动重试。
	production, blocking, err := coord.AcquireProduction(ctx, r.srv.store, projectID, "排程自动续产", 0)
	if err != nil {
		if errors.Is(err, coord.ErrBusy) && blocking != nil {
			return false, "", fmt.Errorf("%w（%s）", err, blocking.Label)
		}
		return false, "", err
	}
	defer production.Release()

	words, loops, style, reasoningModel, writerModel, reviewerModel := r.resolveDefaults()

	switch kind {
	case domain.QuotaKindChapters:
		for i := 0; i < count; i++ {
			chapters, err := r.srv.store.ListChapters(ctx, projectID)
			if err != nil {
				return false, "", fmt.Errorf("list chapters for project %s: %w", projectID, err)
			}
			next := nextChapterIndex(chapters)
			production.SetChapter(next)

			var onProg engine.ProgressCallback
			if progress != nil {
				onProg = func(ev engine.WorkshopEvent) {
					progress(ev.Message)
				}
			}

			req := engine.WorkshopProduceRequest{
				ProjectID:        projectID,
				ChapterIndex:     next,
				WordsTarget:      words,
				MaxRewriteLoops:  loops,
				NarrativeStyle:   style,
				AutoCommit:       true,
				ResumeCheckpoint: true,
				ReasoningModel:   reasoningModel,
				WriterModel:      writerModel,
				ReviewerModel:    reviewerModel,
				OnProgress:       onProg,
			}

			res, err := r.srv.workshop.ProduceChapter(ctx, req)
			if err != nil {
				return false, "", err
			}
			if res != nil && res.Audit != nil && res.Audit.NeedsHuman {
				return true, fmt.Sprintf("第 %d 章停线待人工（%s/%d 分）", next, res.Audit.Verdict, res.Audit.Score), nil
			}
		}
		return false, fmt.Sprintf("已续产 %d 章", count), nil

	case domain.QuotaKindVolumes:
		produced := 0
		for i := 0; i < count; i++ {
			project, err := r.srv.store.GetProject(ctx, projectID)
			if err != nil {
				return false, "", fmt.Errorf("get project %s: %w", projectID, err)
			}
			if project == nil {
				return false, "", fmt.Errorf("project %s not found", projectID)
			}

			chapters, err := r.srv.store.ListChapters(ctx, projectID)
			if err != nil {
				return false, "", fmt.Errorf("list chapters for project %s: %w", projectID, err)
			}
			next := nextChapterIndex(chapters)

			arcs := engine.BackfillVolumeArcs(project.Framework.VolumeArcs)
			arc := findArcForChapter(arcs, next)
			if arc == nil {
				return false, fmt.Sprintf("全书已完结（共 %d 卷已产出）", produced), nil
			}

			var onProg func(engine.VolumeEvent)
			if progress != nil {
				onProg = func(ev engine.VolumeEvent) {
					production.SetChapter(ev.ChapterIndex)
					progress(ev.Message)
				}
			}

			batchReq := engine.VolumeProduceRequest{
				ProjectID:       projectID,
				VolumeIndex:     arc.VolumeIndex,
				ChapterFrom:     next,
				WordsTarget:     words,
				MaxRewriteLoops: loops,
				NarrativeStyle:  style,
				OnProgress:      onProg,
			}

			batch := engine.NewVolumeBatch(r.srv.workshop, r.srv.store)
			res, err := batch.ProduceVolume(ctx, batchReq)
			if err != nil {
				return false, "", err
			}
			if res != nil && res.Stopped {
				if res.StopReason == "HALTED" {
					return true, fmt.Sprintf("第 %d 章停线待人工", res.StoppedAtChapter), nil
				}
				return false, "", fmt.Errorf("第 %d 章生产失败（%s）", res.StoppedAtChapter, res.StopReason)
			}
			produced++
		}
		return false, fmt.Sprintf("已续产 %d 卷", count), nil

	default:
		return false, "", fmt.Errorf("unknown quota kind: %s", kind)
	}
}

// RunBook 开书：执行队列项完整 pipeline（创世→意图→自动锁→批产）。
func (r *schedulerRuntime) RunBook(
	ctx context.Context,
	item *domain.BookQueueItem,
	progress func(string),
) (halted bool, projectID string, message string, err error) {
	if r.srv == nil || r.srv.orch == nil || r.srv.workshop == nil || r.srv.store == nil {
		return false, "", "", errors.New("pipeline dependencies not configured (orch, workshop, or store is nil)")
	}
	if item == nil {
		return false, "", "", errors.New("book queue item cannot be nil")
	}

	words, loops, style, reasoningModel, _, _ := r.resolveDefaults()

	// Fail-Closed：开书流水线在项目创世入库时点（ProjectID 首次可知）登记生产锁；
	// 锁在位期间：调度器续产跳过、UI 正史写入口 409、CLI produce 拒绝。
	bookHook, trackChapter, releaseProduction := coord.BindProduction(r.srv.store, "排程开书")
	defer releaseProduction()

	onProg := func(ev engine.PipelineEvent) {
		trackChapter(ev.ChapterIndex)
		if progress != nil {
			progress(ev.Message)
		}
	}

	pipeline := engine.NewStudioPipeline(r.srv.orch, engine.NewVolumeBatch(r.srv.workshop, r.srv.store), r.srv.store)
	req := engine.PipelineRequest{
		Title:            item.Title,
		Concept:          item.Concept,
		TargetPlatform:   item.Platform,
		ReasoningModel:   reasoningModel,
		Volumes:          item.Volumes,
		AutoLockIntents:  item.AutoLock,
		WordsTarget:      words,
		MaxRewriteLoops:  loops,
		NarrativeStyle:   style,
		OnProgress:       onProg,
		OnProjectCreated: bookHook,
	}

	res, err := pipeline.Run(ctx, req)
	if err != nil {
		return false, "", "", err
	}
	if res.Halted {
		return true, res.ProjectID, res.HaltReason, nil
	}
	return false, res.ProjectID, fmt.Sprintf("开书完成：已批产 %d 卷", len(res.Volumes)), nil
}
