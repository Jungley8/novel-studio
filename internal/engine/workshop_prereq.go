package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// VolumeArcForChapter 返回覆盖指定章号的卷弧（BackfillVolumeArcs 后按 [Start,End] 匹配）；无 → nil。
func VolumeArcForChapter(framework *domain.ProjectFramework, chapterIndex int) *domain.VolumeArc {
	if framework == nil || len(framework.VolumeArcs) == 0 || chapterIndex <= 0 {
		return nil
	}
	arcs := BackfillVolumeArcs(framework.VolumeArcs)
	for i := range arcs {
		if chapterIndex >= arcs[i].StartChapter && chapterIndex <= arcs[i].EndChapter {
			return &arcs[i]
		}
	}
	return nil
}

// EnsureVolumeIntents 幂等确保指定分卷意图卡齐备（缺章才生成；全齐 → 零 LLM 调用）。
// 超过单次上限 60 章时按 ≤60 分块多次调用 GenerateVolumeIntents（existingMap = 实际已存在 ∪ 弧内除本块外全部章号），usage 累加。
func (w *ChapterWorkshop) EnsureVolumeIntents(ctx context.Context, projectID string, volumeIndex int, model string) (TokenUsage, error) {
	if w == nil || w.orch == nil || w.orch.client == nil {
		return TokenUsage{}, errors.New("workshop orchestrator or llm client is nil (fail-closed)")
	}
	if w.store == nil {
		return TokenUsage{}, errors.New("workshop store is nil (fail-closed)")
	}

	project, err := w.store.GetProject(ctx, projectID)
	if err != nil {
		return TokenUsage{}, fmt.Errorf("get project %s: %w", projectID, err)
	}
	if project == nil {
		return TokenUsage{}, fmt.Errorf("project %s not found", projectID)
	}
	if project.Framework == nil {
		return TokenUsage{}, fmt.Errorf("project framework is nil for project %s", projectID)
	}

	arcs := BackfillVolumeArcs(project.Framework.VolumeArcs)
	var targetArc *domain.VolumeArc
	for i := range arcs {
		if arcs[i].VolumeIndex == volumeIndex {
			targetArc = &arcs[i]
			break
		}
	}
	if targetArc == nil {
		return TokenUsage{}, fmt.Errorf("volume index %d not found in project %s", volumeIndex, projectID)
	}

	existingIntents, err := w.store.ListChapterIntents(ctx, projectID, volumeIndex)
	if err != nil {
		return TokenUsage{}, fmt.Errorf("list chapter intents for volume %d: %w", volumeIndex, err)
	}
	existingMap := make(map[int]bool)
	for _, it := range existingIntents {
		if it != nil {
			existingMap[it.ChapterIndex] = true
		}
	}

	var missing []int
	if targetArc.StartChapter > 0 && targetArc.EndChapter >= targetArc.StartChapter {
		for ch := targetArc.StartChapter; ch <= targetArc.EndChapter; ch++ {
			if !existingMap[ch] {
				missing = append(missing, ch)
			}
		}
	}

	if len(missing) == 0 {
		return TokenUsage{}, nil
	}

	const maxIntentsBatch = 60
	var totalUsage TokenUsage

	for i := 0; i < len(missing); i += maxIntentsBatch {
		end := i + maxIntentsBatch
		if end > len(missing) {
			end = len(missing)
		}
		chunk := missing[i:end]
		chunkSet := make(map[int]bool, len(chunk))
		for _, ch := range chunk {
			chunkSet[ch] = true
		}

		// existingMap = 实际已存在 ∪ 弧内除本块外全部章号
		existingForCall := make(map[int]bool)
		for ch := targetArc.StartChapter; ch <= targetArc.EndChapter; ch++ {
			if !chunkSet[ch] {
				existingForCall[ch] = true
			}
		}

		newIntents, usage, err := w.orch.GenerateVolumeIntents(ctx, model, project, *targetArc, existingForCall)
		addTokenUsage(&totalUsage, usage)
		if err != nil {
			return totalUsage, fmt.Errorf("generate volume intents for volume %d: %w", volumeIndex, err)
		}

		for j := range newIntents {
			intent := newIntents[j]
			if err := w.store.SaveChapterIntent(ctx, &intent); err != nil {
				return totalUsage, fmt.Errorf("save chapter intent ch%d: %w", intent.ChapterIndex, err)
			}
		}
	}

	return totalUsage, nil
}

// EnsureProductionPrerequisites 统一生产前置：意图补齐 → 分节确保（全入口唯一路径，fail-closed）。
func (w *ChapterWorkshop) EnsureProductionPrerequisites(ctx context.Context, projectID string, chapterIndex int, model string) (TokenUsage, error) {
	if w == nil || w.store == nil {
		return TokenUsage{}, errors.New("workshop or store is nil (fail-closed)")
	}

	project, err := w.store.GetProject(ctx, projectID)
	if err != nil {
		return TokenUsage{}, fmt.Errorf("get project %s: %w", projectID, err)
	}
	if project == nil {
		return TokenUsage{}, fmt.Errorf("project %s not found", projectID)
	}
	if project.Framework == nil {
		return TokenUsage{}, nil
	}

	arc := VolumeArcForChapter(project.Framework, chapterIndex)
	if arc == nil {
		return TokenUsage{}, nil
	}

	var totalUsage TokenUsage

	intentUsage, err := w.EnsureVolumeIntents(ctx, projectID, arc.VolumeIndex, model)
	addTokenUsage(&totalUsage, intentUsage)
	if err != nil {
		return totalUsage, fmt.Errorf("ensure volume intents for volume %d: %w", arc.VolumeIndex, err)
	}

	_, secUsage, err := w.EnsureVolumeSections(ctx, projectID, arc.VolumeIndex, model)
	addTokenUsage(&totalUsage, secUsage)
	if err != nil {
		return totalUsage, fmt.Errorf("ensure volume sections for volume %d: %w", arc.VolumeIndex, err)
	}

	return totalUsage, nil
}
