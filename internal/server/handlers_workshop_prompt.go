package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type PromptPreviewRequest struct {
	ChapterIndex int      `json:"chapter_index"`
	SceneTitle   string   `json:"scene_title,omitempty"`
	POV          string   `json:"pov,omitempty"`
	DramaticGoal string   `json:"dramatic_goal,omitempty"`
	Conflict     string   `json:"conflict,omitempty"`
	CoreEvents   []string `json:"core_events,omitempty"`
}

type PromptComponentPreview struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Content       string `json:"content"`
	TokenEstimate int    `json:"token_estimate"`
}

type PromptPreviewResponse struct {
	SystemPrompt        string                   `json:"system_prompt"`
	UserPrompt          string                   `json:"user_prompt"`
	Components          []PromptComponentPreview `json:"components"`
	TotalTokensEstimate int                      `json:"total_tokens_estimate"`
}

func estimateTokens(text string) int {
	return domain.EstimateTokens(text)
}

// previewMotifBlock 预览用母题展示块：只读 p.Framework.Motifs 做展示拼接，
// 不调 engine、不改生产注入逻辑、不动 store。
// 生产注入见 internal/engine/orchestrator_render.go:168（调用点）与 :225
// buildMotifConceptPrompt（表头【有意母题与符号（RC 精修锚点）】与规则行见该文件 :238-239）。
// 写入时母题先经 domain/motif.go:13 SanitizeMotifs 清洗（trim/去重/上限8，
// 调用点 internal/server/handlers_matrix.go:249）。
// 无有效母题 → ""（整块省略，不输出占位）。
func previewMotifBlock(p *domain.Project) string {
	if p == nil || p.Framework == nil {
		return ""
	}
	var lines []string
	for _, m := range p.Framework.Motifs {
		if trimmed := strings.TrimSpace(m); trimmed != "" {
			lines = append(lines, "- "+trimmed)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "【核心母题（生产提示词经清洗门注入）】\n" + strings.Join(lines, "\n") +
		"\n注：生产侧以 orchestrator_render.go:225 buildMotifConceptPrompt 清洗后版本为准（行文与本块同源）。"
}

// previewMilestoneBlock 预览用节拍定位块：只读 p.Framework.StoryMilestones /
// TotalPlannedChapters 做展示拼接，不调 engine、不改生产注入逻辑、不动 store。
// 生产块见 internal/engine/orchestrator_beats.go:354 buildMilestoneContextBlock
// （调用点 :221，表头【全书宏观节拍定位】见该文件 :386）。
// 定位行（计划/本章/越过/下一）与生产块逐行同文，另逐条列出全部里程碑
// （第{TargetChapter}章·{Key}·{Name}；StoryMilestone 无 Type/Summary 字段，
// 以 Key/Name 对应），便于与生产提示词肉眼对齐。
// 无里程碑 → ""（整块省略，不输出占位）。
func previewMilestoneBlock(p *domain.Project, chapterIndex int) string {
	if p == nil || p.Framework == nil || len(p.Framework.StoryMilestones) == 0 {
		return ""
	}
	var curr, overdue, next *domain.StoryMilestone
	for i := range p.Framework.StoryMilestones {
		m := &p.Framework.StoryMilestones[i]
		if m.TargetChapter == chapterIndex && curr == nil {
			curr = m
		}
		if m.TargetChapter < chapterIndex && (m.Status == domain.MilestoneStatusPending || strings.TrimSpace(m.Status) == "") {
			if overdue == nil || m.TargetChapter > overdue.TargetChapter {
				overdue = m
			}
		}
		if m.TargetChapter > chapterIndex {
			if next == nil || m.TargetChapter < next.TargetChapter {
				next = m
			}
		}
	}
	milestoneName := func(m domain.StoryMilestone) string {
		if strings.TrimSpace(m.Name) != "" {
			return strings.TrimSpace(m.Name)
		}
		return strings.TrimSpace(m.Key)
	}
	var b strings.Builder
	b.WriteString("【全书节拍定位】")
	if total := p.Framework.TotalPlannedChapters; total > 0 {
		fmt.Fprintf(&b, "\n- 全书计划：%d 章（本章为第 %d 章，约 %d%%）", total, chapterIndex, int(float64(chapterIndex)/float64(total)*100))
	} else {
		fmt.Fprintf(&b, "\n- 全书计划：未定（本章为第 %d 章）", chapterIndex)
	}
	if curr != nil {
		fmt.Fprintf(&b, "\n- 本章里程碑：『%s』@第 %d 章 —— 正文必须在本章兑现该里程碑事件与情绪高潮", milestoneName(*curr), chapterIndex)
	} else {
		b.WriteString("\n- 本章无里程碑锚点")
	}
	if overdue != nil {
		fmt.Fprintf(&b, "\n- 最近未兑现里程碑：『%s』@第 %d 章（PENDING，已越过）", milestoneName(*overdue), overdue.TargetChapter)
	}
	if next != nil {
		fmt.Fprintf(&b, "\n- 下一里程碑：『%s』@第 %d 章（间隔 %d 章）", milestoneName(*next), next.TargetChapter, next.TargetChapter-chapterIndex)
	}
	for _, m := range p.Framework.StoryMilestones {
		fmt.Fprintf(&b, "\n- 第%d章·%s·%s", m.TargetChapter, strings.TrimSpace(m.Key), milestoneName(m))
	}
	return b.String()
}

func (s *Server) handlePromptPreview(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PromptPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.ChapterIndex <= 0 {
		req.ChapterIndex = 1
	}

	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil || p == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	horizon, err := s.chronicle.AssembleHorizon(ctx, projectID, req.ChapterIndex)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "assemble horizon: "+err.Error())
		return
	}

	frameworkContent := fmt.Sprintf("【世界观与公理】\n%s\n\n【目标平台调性】\n%s", p.WorldRules, p.TargetPlatform)
	if p.Framework != nil {
		if strings.TrimSpace(p.Framework.ThemePremise) != "" {
			frameworkContent += fmt.Sprintf("\n\n【核心立意与主线】\n%s", strings.TrimSpace(p.Framework.ThemePremise))
		}
		if strings.TrimSpace(p.Framework.Logline) != "" {
			frameworkContent += fmt.Sprintf("\n\n【一句话梗概】\n%s", strings.TrimSpace(p.Framework.Logline))
		}
		if strings.TrimSpace(p.Framework.AtmosphereAnchor) != "" {
			frameworkContent += fmt.Sprintf("\n\n【视觉与氛围锚点】\n%s", strings.TrimSpace(p.Framework.AtmosphereAnchor))
		}
		if motifBlock := previewMotifBlock(p); motifBlock != "" {
			frameworkContent += "\n\n" + motifBlock
		}
		if milestoneBlock := previewMilestoneBlock(p, req.ChapterIndex); milestoneBlock != "" {
			frameworkContent += "\n\n" + milestoneBlock
		}
	}

	canonContent := fmt.Sprintf("【前序章节剧情记忆】\n%s", horizon.RollingCanonText)
	if horizon.TailAnchor != "" {
		canonContent += fmt.Sprintf("\n\n【上章末尾腔调锚定】\n%s", horizon.TailAnchor)
	}

	codexContent := horizon.CodexContextText
	if codexContent == "" {
		codexContent = "(当前章节未匹配到需强制注入的百科实体)"
	}

	var sceneSpec strings.Builder
	sceneSpec.WriteString(fmt.Sprintf("【目标章节】第 %d 章\n", req.ChapterIndex))
	if req.SceneTitle != "" {
		sceneSpec.WriteString(fmt.Sprintf("【场景标题】%s\n", req.SceneTitle))
	}
	if req.POV != "" {
		sceneSpec.WriteString(fmt.Sprintf("【视角人物(POV)】%s\n", req.POV))
	}
	if req.DramaticGoal != "" {
		sceneSpec.WriteString(fmt.Sprintf("【戏剧目标】%s\n", req.DramaticGoal))
	}
	if req.Conflict != "" {
		sceneSpec.WriteString(fmt.Sprintf("【阻碍与冲突】%s\n", req.Conflict))
	}
	if len(req.CoreEvents) > 0 {
		sceneSpec.WriteString("【核心事件节拍】\n")
		for i, ev := range req.CoreEvents {
			sceneSpec.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ev))
		}
	}

	components := []PromptComponentPreview{
		{
			Name:          "世界观与风格公理 (Global Framework)",
			Description:   "包含世界设定、平台调性约束与核心爽点规则",
			Content:       frameworkContent,
			TokenEstimate: estimateTokens(frameworkContent),
		},
		{
			Name:          "因果故事线记忆 (Canon Horizon)",
			Description:   "包含前序章节滚雪球摘要与上章尾部文风锚点",
			Content:       canonContent,
			TokenEstimate: estimateTokens(canonContent),
		},
		{
			Name:          "全域百科与关联 (The Codex Injected)",
			Description:   "动态扫描注入的角色阶段属性、别名、物品状态及两两关系",
			Content:       codexContent,
			TokenEstimate: estimateTokens(codexContent),
		},
		{
			Name:          "场景戏剧规格 (Scene Dramaturgy)",
			Description:   "当前场次的 POV、戏剧目标、冲突阻碍与核心节拍事实",
			Content:       sceneSpec.String(),
			TokenEstimate: estimateTokens(sceneSpec.String()),
		},
	}

	horizonContext := horizon.RollingCanonText
	if horizon.TailAnchor != "" {
		if horizonContext != "" {
			horizonContext += "\n" + horizon.TailAnchor
		} else {
			horizonContext = horizon.TailAnchor
		}
	}

	var linterRes domain.LinterResult
	if s.linter != nil {
		linterRes = s.linter.Analyze(horizonContext)
	} else if s.qualityGate != nil {
		linterRes = s.qualityGate.FastAnalyze(horizonContext)
	}

	var heuristicSummary string
	if strings.TrimSpace(horizonContext) == "" {
		heuristicSummary = "前序上下文为空，本章暂无启发式预检数据"
	} else {
		var details []string
		details = append(details, fmt.Sprintf("突发度: %d/100", linterRes.BurstinessScore))
		if len(linterRes.HitBannedWords) > 0 {
			details = append(details, fmt.Sprintf("命中毒词套词(%d处): %s", len(linterRes.HitBannedWords), strings.Join(linterRes.HitBannedWords, ", ")))
		} else {
			details = append(details, "零违禁套词")
		}
		details = append(details, fmt.Sprintf("感叹号密度: %.1f/千字", linterRes.ExclamationDensity))
		if linterRes.DialogueRatio > 0 {
			details = append(details, fmt.Sprintf("对话占比: %.1f%%", linterRes.DialogueRatio*100))
		}
		if len(linterRes.TopRepeatedNgrams) > 0 {
			details = append(details, fmt.Sprintf("高频重复短语: %s", strings.Join(linterRes.TopRepeatedNgrams, ", ")))
		}
		if len(linterRes.EmpiricalTells) > 0 {
			details = append(details, fmt.Sprintf("AI腔调特征: %s", strings.Join(linterRes.EmpiricalTells, ", ")))
		}
		status := "PASS"
		if !linterRes.Passed {
			status = "WARN"
		}
		heuristicSummary = fmt.Sprintf("[%s] %s", status, strings.Join(details, "；"))
	}

	var lastAuditSummary string
	cp, err := s.store.GetCheckpoint(ctx, projectID, req.ChapterIndex)
	if err == nil && cp != nil && cp.AuditReport != nil {
		rep := cp.AuditReport
		var issuesText string
		if len(rep.Issues) > 0 {
			top := rep.Issues
			if len(top) > 3 {
				top = top[:3]
			}
			var topLines []string
			for i, iss := range top {
				topLines = append(topLines, fmt.Sprintf("%d. %s", i+1, iss))
			}
			issuesText = fmt.Sprintf("\nTop 3 待改问题:\n  %s", strings.Join(topLines, "\n  "))
		} else {
			issuesText = "\nTop 3 待改问题: 无"
		}
		lastAuditSummary = fmt.Sprintf("评级: %s | 评分: %d%s", rep.Verdict, rep.Score, issuesText)
	} else {
		lastAuditSummary = "本章暂无评审记录"
	}

	var reviewGateContent strings.Builder
	reviewGateContent.WriteString("【Fail-Closed 放行规则】\n")
	reviewGateContent.WriteString("ACCEPTED && ≥80 && 零套词 && 感叹号不超标；任一分庭 REVISION 即驳回；耗尽返工停线待人工\n\n")
	reviewGateContent.WriteString("【本章启发式预检摘要】\n")
	reviewGateContent.WriteString(heuristicSummary)
	reviewGateContent.WriteString("\n\n")
	reviewGateContent.WriteString("【历史评审记录】\n")
	reviewGateContent.WriteString(lastAuditSummary)

	components = append(components, PromptComponentPreview{
		Name:          "评审门禁 (Review Gate)",
		Description:   "Fail-Closed 放行规则、本章启发式预检与历史评审记录",
		Content:       reviewGateContent.String(),
		TokenEstimate: estimateTokens(reviewGateContent.String()),
	})

	totalTokens := 0
	for _, c := range components {
		totalTokens += c.TokenEstimate
	}

	systemPrompt := `你是一名冷峻、极具电影镜头感的小说名家。严格遵循因果设定与声口约束，杜绝AI套话与伪高潮。`
	var userPrompt strings.Builder
	userPrompt.WriteString(frameworkContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(canonContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(codexContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(sceneSpec.String())

	jsonResponse(w, http.StatusOK, PromptPreviewResponse{
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt.String(),
		Components:          components,
		TotalTokensEstimate: totalTokens,
	})
}
