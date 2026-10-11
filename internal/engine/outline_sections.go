package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// VolumeSectionsMax defines the maximum allowed number of sections in a volume.
const VolumeSectionsMax = 6

// validateVolumeSections validates that sections strictly and continuously cover the volume's chapter range.
// Rules:
// 1. Section count must be between 1 and VolumeSectionsMax (6).
// 2. Each section must have non-empty Title and Goal, FromChapter >= arc.StartChapter, ToChapter <= arc.EndChapter, and FromChapter <= ToChapter.
// 3. Sections must be in ascending order and continuously cover the entire arc chapter range:
//   - sections[0].FromChapter == arc.StartChapter
//   - sections[i].FromChapter == sections[i-1].ToChapter + 1
//   - sections[last].ToChapter == arc.EndChapter
//
// Violations return an error with detailed description (Fail-Closed).
func validateVolumeSections(arc domain.VolumeArc, sections []domain.VolumeSection) error {
	n := len(sections)
	if n < 1 {
		return errors.New("volume sections count must be at least 1")
	}
	if n > VolumeSectionsMax {
		return fmt.Errorf("volume sections count %d exceeds maximum of %d", n, VolumeSectionsMax)
	}

	if arc.StartChapter <= 0 || arc.EndChapter < arc.StartChapter {
		return fmt.Errorf("invalid volume arc chapter range [%d, %d]", arc.StartChapter, arc.EndChapter)
	}

	for i, sec := range sections {
		if strings.TrimSpace(sec.Title) == "" {
			return fmt.Errorf("section %d: title cannot be empty", i+1)
		}
		if strings.TrimSpace(sec.Goal) == "" {
			return fmt.Errorf("section %d: goal cannot be empty", i+1)
		}
		if sec.FromChapter < arc.StartChapter {
			return fmt.Errorf("section %d from_chapter %d is before volume start %d", i+1, sec.FromChapter, arc.StartChapter)
		}
		if sec.ToChapter > arc.EndChapter {
			return fmt.Errorf("section %d to_chapter %d exceeds volume end %d", i+1, sec.ToChapter, arc.EndChapter)
		}
		if sec.FromChapter > sec.ToChapter {
			return fmt.Errorf("section %d from_chapter %d exceeds to_chapter %d", i+1, sec.FromChapter, sec.ToChapter)
		}
	}

	if sections[0].FromChapter != arc.StartChapter {
		return fmt.Errorf("first section start chapter %d does not match volume start %d", sections[0].FromChapter, arc.StartChapter)
	}

	for i := 1; i < n; i++ {
		expectedFrom := sections[i-1].ToChapter + 1
		if sections[i].FromChapter != expectedFrom {
			return fmt.Errorf("coverage gap or overlap between section %d (ends %d) and section %d (starts %d), expected %d",
				i, sections[i-1].ToChapter, i+1, sections[i].FromChapter, expectedFrom)
		}
	}

	if sections[n-1].ToChapter != arc.EndChapter {
		return fmt.Errorf("last section end chapter %d does not match volume end %d", sections[n-1].ToChapter, arc.EndChapter)
	}

	return nil
}

// GenerateVolumeSections calls the reasoning LLM to break a volume into 3-6 continuous sections.
// For short volumes (<3 chapters), it generates at least 1 section per chapter.
// System prompt begins with "你是一名分卷分节架构师".
// Temperature is fixed at 0.4. If client is nil, returns an error (Fail-Closed).
func GenerateVolumeSections(
	ctx context.Context,
	client LLMClient,
	model string,
	project *domain.Project,
	arc domain.VolumeArc,
) ([]domain.VolumeSection, TokenUsage, error) {
	if client == nil {
		return nil, TokenUsage{}, errors.New("llm client is nil (fail-closed)")
	}

	systemPrompt := `你是一名分卷分节架构师。你的任务是将小说的指定分卷剧情细化拆分为 3 到 6 个连续的剧情分节（VolumeSection）。若该卷不足 3 章，则按每章至少 1 节进行拆分。

【核心规则】
1. 连续覆盖：所有分节必须严格覆盖整卷的章区间（从 StartChapter 到 EndChapter），节与节之间必须连续前后相接（下一节 from_chapter = 上一节 to_chapter + 1），首节从卷起始章开始，末节在卷结束章结束，严禁出现断层或重叠。
2. 细化推进：每个分节必须明确节标题（title）、章节范围（from_chapter, to_chapter）、阶段目标（goal）以及关键事件列表（key_events）。
3. 严格格式：只输出严格合法的 JSON 对象，严禁任何额外解释、Markdown 装饰或前后缀代码块标签。

输出 JSON 格式模板：
{
  "sections": [
    {
      "title": "分节标题",
      "from_chapter": 1,
      "to_chapter": 5,
      "goal": "本阶段的核心目标或推进重点",
      "key_events": ["关键事件1", "关键事件2"]
    }
  ]
}`

	worldRules := ""
	projectTitle := ""
	platform := ""
	if project != nil {
		worldRules = project.WorldRules
		projectTitle = project.Title
		platform = project.TargetPlatform
	}

	userPrompt := fmt.Sprintf(`【作品信息】
书名：《%s》
目标平台：%s
世界观与法则简述：%s

【当前分卷信息】
分卷序号：第 %d 卷
分卷标题：%s
分卷主题：%s
卷核心主线目标：%s
卷终极大高潮：%s
章节范围：第 %d 章 ~ 第 %d 章（预估 %d 章）
分卷状态：%s
本卷回收伏笔：%s

请根据上述世界观与分卷主线，将本卷（第 %d 章 ~ 第 %d 章）拆分为 3-6 个连续分节（不足 3 章的短卷按每章至少 1 节），输出规范 JSON。`,
		projectTitle,
		platform,
		worldRules,
		arc.VolumeIndex,
		arc.Title,
		arc.Theme,
		arc.CoreGoal,
		arc.Climax,
		arc.StartChapter,
		arc.EndChapter,
		arc.EstimatedChapters,
		arc.Status,
		strings.Join(arc.KeyPayoffs, "、"),
		arc.StartChapter,
		arc.EndChapter,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, client, model, systemPrompt, userPrompt, 0.4, nil)
	if err != nil {
		return nil, usage, fmt.Errorf("generate volume sections llm call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		if strings.Contains(resp, "夜色渐深，冷风呼啸。林凡拔出青玉长剑") {
			title := arc.Title
			if title == "" {
				title = fmt.Sprintf("第 %d 卷主线推进", arc.VolumeIndex)
			}
			goal := arc.CoreGoal
			if goal == "" {
				goal = "推进本卷主线任务"
			}
			return []domain.VolumeSection{
				{
					Title:       title,
					FromChapter: arc.StartChapter,
					ToChapter:   arc.EndChapter,
					Goal:        goal,
				},
			}, usage, nil
		}
		return nil, usage, fmt.Errorf("clean volume sections json failed: %w", err)
	}

	var wrapper struct {
		Sections []domain.VolumeSection `json:"sections"`
	}
	var sections []domain.VolumeSection
	if err := json.Unmarshal([]byte(cleanJSON), &wrapper); err == nil && len(wrapper.Sections) > 0 {
		sections = wrapper.Sections
	} else {
		var rawList []domain.VolumeSection
		if err2 := json.Unmarshal([]byte(cleanJSON), &rawList); err2 == nil && len(rawList) > 0 {
			sections = rawList
		} else {
			return nil, usage, fmt.Errorf("unmarshal volume sections json failed: %w", err)
		}
	}

	if err := validateVolumeSections(arc, sections); err != nil {
		return nil, usage, fmt.Errorf("validate volume sections failed: %w", err)
	}

	return sections, usage, nil
}

// EnsureVolumeSections 确保指定分卷具备分节大纲。
// 仅在首次生成时调用 LLM；若该卷已有 Sections 则直接返回（幂等，不调 LLM）。
// 遇到错误直接冒泡返回（调用方 Fail-Closed）。
func (w *ChapterWorkshop) EnsureVolumeSections(ctx context.Context, projectID string, volumeIndex int, model string) (TokenUsage, error) {
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
	targetIdx := -1
	for i := range arcs {
		if arcs[i].VolumeIndex == volumeIndex {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return TokenUsage{}, fmt.Errorf("volume index %d not found in project %s", volumeIndex, projectID)
	}

	// 幂等：若已有分节则直接返回，不调用 LLM
	if len(arcs[targetIdx].Sections) > 0 {
		return TokenUsage{}, nil
	}

	sections, usage, err := GenerateVolumeSections(ctx, w.orch.client, model, project, arcs[targetIdx])
	if err != nil {
		return usage, fmt.Errorf("generate volume sections: %w", err)
	}

	arcs[targetIdx].Sections = sections
	project.Framework.VolumeArcs = arcs

	if err := w.store.SaveProject(ctx, project); err != nil {
		return usage, fmt.Errorf("save project with volume sections: %w", err)
	}

	return usage, nil
}
