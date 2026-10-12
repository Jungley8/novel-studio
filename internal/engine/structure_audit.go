package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type rawStructureFinding struct {
	Type         string `json:"type"`
	Severity     string `json:"severity"`
	ChapterStart int    `json:"chapter_start"`
	ChapterEnd   int    `json:"chapter_end"`
	Summary      string `json:"summary"`
	Evidence     string `json:"evidence"`
	Suggestion   string `json:"suggestion"`
}

// RunStructureAudit conducts a whole-book structural review evaluating plot holes, pacing,
// redundancy, dropped hooks, and character/theme arc drift across chapter summaries, hooks, framework, and bench stats.
func (o *Orchestrator) RunStructureAudit(
	ctx context.Context,
	model string,
	project *domain.Project,
	summaries []*domain.ChapterSummary,
	hooks []*domain.PlotHook,
	benchSummary string,
) ([]domain.StructureFinding, TokenUsage, error) {
	if o == nil || o.client == nil {
		return nil, TokenUsage{}, errors.New("orchestrator or llm client is nil (fail-closed)")
	}
	if project == nil {
		return nil, TokenUsage{}, errors.New("project cannot be nil")
	}

	// 无摘要输入返回空切片 nil error
	if len(summaries) == 0 {
		return []domain.StructureFinding{}, TokenUsage{}, nil
	}

	// 1. Framework 组装 (ThemePremise/Logline/里程碑目标章节+类型+概要)
	var fwParts []string
	if project.Framework != nil {
		if premise := strings.TrimSpace(project.Framework.ThemePremise); premise != "" {
			fwParts = append(fwParts, "立意："+premise)
		}
		if logline := strings.TrimSpace(project.Framework.Logline); logline != "" {
			fwParts = append(fwParts, "梗概："+logline)
		}
		if len(project.Framework.StoryMilestones) > 0 {
			var msList []string
			for _, m := range project.Framework.StoryMilestones {
				name := m.Name
				if name == "" {
					name = m.Key
				}
				msDesc := fmt.Sprintf("第%d章 [%s - %s]", m.TargetChapter, m.Key, name)
				if strings.TrimSpace(m.Note) != "" {
					msDesc += " (" + strings.TrimSpace(m.Note) + ")"
				}
				msList = append(msList, msDesc)
			}
			fwParts = append(fwParts, "故事里程碑规划：\n  "+strings.Join(msList, "\n  "))
		}
	}
	fwSection := "暂无明确大纲框架"
	if len(fwParts) > 0 {
		fwSection = strings.Join(fwParts, "\n")
	}

	// 2. 章摘要组装 (第N章：text，每章截断 300 rune，总 cap 12000 rune)
	sortedSummaries := make([]*domain.ChapterSummary, 0, len(summaries))
	for _, s := range summaries {
		if s != nil {
			sortedSummaries = append(sortedSummaries, s)
		}
	}
	sort.Slice(sortedSummaries, func(i, j int) bool {
		return sortedSummaries[i].ChapterIndex < sortedSummaries[j].ChapterIndex
	})

	var summaryBuilder strings.Builder
	currentRunes := 0
	const maxTotalRunes = 12000

	for _, s := range sortedSummaries {
		runes := []rune(strings.TrimSpace(s.Text))
		if len(runes) > 300 {
			runes = runes[:300]
		}
		line := fmt.Sprintf("第%d章：%s\n", s.ChapterIndex, string(runes))
		lineRunes := []rune(line)
		if currentRunes+len(lineRunes) > maxTotalRunes {
			remaining := maxTotalRunes - currentRunes
			if remaining > 0 {
				summaryBuilder.WriteString(string(lineRunes[:remaining]))
			}
			break
		}
		summaryBuilder.WriteString(line)
		currentRunes += len(lineRunes)
	}

	// 3. open/fermenting 伏笔
	var hookLines []string
	for _, h := range hooks {
		if h == nil {
			continue
		}
		if h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting {
			hookLines = append(hookLines, fmt.Sprintf("- [%s] 伏笔: %s (埋设于第%d章, 目标第%d章) 详情: %s",
				h.Status, h.Title, h.CreatedChapter, h.TargetChapter, strings.TrimSpace(h.Details)))
		}
	}
	hooksSection := "暂无活跃/发酵中伏笔"
	if len(hookLines) > 0 {
		hooksSection = strings.Join(hookLines, "\n")
	}

	// 4. bench 摘要文本
	benchSection := "暂无 Bench 数据"
	if strings.TrimSpace(benchSummary) != "" {
		benchSection = strings.TrimSpace(benchSummary)
	}

	systemPrompt := `你是一名资深网络小说剧情总监与全书结构审阅专家。
你的任务是对全书的故事结构进行全局结构性体检，发现跨章节的宏观结构性问题，包括：
- PLOT_HOLE: 设定冲突、前后矛盾、逻辑硬伤
- PACING: 节奏失衡（长期平淡拖沓、高潮缺位或转折突兀）
- REDUNDANT: 支线注水、无效情节、重复循环套路
- DROPPED_HOOK: 伏笔遗忘、关键线索断线未收、双线写丢
- ARC_DRIFT: 角色弧光漂移、核心立意偏离、里程碑未按计划兑现

必须且仅输出严格的 JSON 数组，严禁包含任何 Markdown 格式前缀或额外说明文本。
数组中每一项必须包含以下字段：
[
  {
    "type": "PLOT_HOLE | PACING | REDUNDANT | DROPPED_HOOK | ARC_DRIFT",
    "severity": "HIGH | MEDIUM | LOW",
    "chapter_start": 3,
    "chapter_end": 5,
    "summary": "问题一句话概述",
    "evidence": "正文或摘要中的具体事实证据引用",
    "suggestion": "具体可落地的调整或修改建议"
  }
]
若全书结构严谨无明显缺陷，返回空数组 []。最多输出 20 条。`

	userPrompt := fmt.Sprintf(`【作品基本信息】
- 书名：%s
- 目标平台：%s
- 世界观/规则摘要：%s

【宏观大纲与框架】
%s

【伏笔台账 (OPEN / FERMENTING)】
%s

【质量体检 (Bench) 诊断参考】
%s

【全书章节摘要】
%s

请基于以上全书材料完成结构体检，指出伏笔/节奏/冗余/弧光漂移问题。输出 JSON 数组：`,
		project.Title,
		project.TargetPlatform,
		project.WorldRules,
		fwSection,
		hooksSection,
		benchSection,
		summaryBuilder.String(),
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, model, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, usage, fmt.Errorf("run structure audit LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean structure audit json failed: %w", err)
	}

	rawFindings, err := UnmarshalJSONArray[rawStructureFinding](cleanJSON)
	if err != nil {
		return nil, usage, fmt.Errorf("unmarshal structure audit json failed: %w", err)
	}

	findings := make([]domain.StructureFinding, 0, len(rawFindings))
	now := time.Now().UTC()
	for _, raw := range rawFindings {
		f := domain.StructureFinding{
			ProjectID:    project.ID,
			Type:         domain.StructureFindingType(strings.TrimSpace(raw.Type)),
			Severity:     domain.StructureFindingSeverity(strings.TrimSpace(raw.Severity)),
			ChapterStart: raw.ChapterStart,
			ChapterEnd:   raw.ChapterEnd,
			Summary:      strings.TrimSpace(raw.Summary),
			Evidence:     strings.TrimSpace(raw.Evidence),
			Suggestion:   strings.TrimSpace(raw.Suggestion),
			Status:       domain.StructureFindingStatusOpen,
			CreatedAt:    now,
		}

		// 逐项 Validate，非法项丢弃
		if err := f.Validate(); err != nil {
			continue
		}

		findings = append(findings, f)
		if len(findings) >= 20 {
			break
		}
	}

	return findings, usage, nil
}
