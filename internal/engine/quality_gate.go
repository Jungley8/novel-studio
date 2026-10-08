package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// QualityGate unifies algorithmic statistical linting and semantic LLM review
// into a single cohesive auditing seam.
type QualityGate struct {
	linter *Linter
	client LLMClient
}

func NewQualityGate(client LLMClient, customBanned []string) *QualityGate {
	return &QualityGate{
		linter: NewLinter(customBanned),
		client: client,
	}
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
	chapterIndex int,
	beats []domain.SceneBeat,
	draftText string,
) (*domain.AuditReport, TokenUsage, error) {
	return q.AuditWithHooks(ctx, reviewerModel, project, chapterIndex, beats, nil, draftText)
}

// AuditWithHooks extends Audit with open plot hook awareness to verify hook payoffs and resolutions.
func (q *QualityGate) AuditWithHooks(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
	chapterIndex int,
	beats []domain.SceneBeat,
	activeHooks []*domain.PlotHook,
	draftText string,
) (*domain.AuditReport, TokenUsage, error) {
	// 1. Fast algorithmic pre-pass
	heuristic := q.linter.Analyze(draftText)

	// 2. Prepare LLM reviewer prompt with heuristic findings injected
	var hList []string
	if len(heuristic.HitBannedWords) > 0 {
		hList = append(hList, fmt.Sprintf("已检出AI模式化套词: %s", strings.Join(heuristic.HitBannedWords, ", ")))
	}
	if heuristic.BurstinessScore < 40 {
		hList = append(hList, fmt.Sprintf("句长节奏过于平缓 (突发度仅 %d/100，易被平台反AI检测识别)", heuristic.BurstinessScore))
	}
	if IsExclamationExcessive(draftText, heuristic.ExclamationDensity) {
		hList = append(hList, fmt.Sprintf("感叹号过密 (每千字 %.1f 个，文字显浮夸)", heuristic.ExclamationDensity))
	}
	if len(heuristic.TopRepeatedNgrams) > 0 {
		hList = append(hList, fmt.Sprintf("检出高频重复短语: %s", strings.Join(heuristic.TopRepeatedNgrams, ", ")))
	}
	runesCount := len([]rune(draftText))
	if runesCount >= 300 && (heuristic.DialogueRatio < 0.10 || heuristic.DialogueRatio > 0.65) {
		hList = append(hList, fmt.Sprintf("对话占比异常 (%.1f%%，建议保持在 15%%-55%% 之间)", heuristic.DialogueRatio*100))
	}

	heuristicNotes := "各项算法指标优良"
	if len(hList) > 0 {
		heuristicNotes = strings.Join(hList, "; ")
	}

	hooksContext := "暂无开放伏笔"
	if len(activeHooks) > 0 {
		var hList []string
		for _, h := range activeHooks {
			hList = append(hList, fmt.Sprintf("- [%s] ID:%s | 标题:《%s》 | 目标章节:第%d章 | 详情:%s", h.Status, h.ID, h.Title, h.TargetChapter, h.Details))
		}
		hooksContext = strings.Join(hList, "\n")
	}

	systemPrompt := `你是一名极其严苛、深谙网络小说工业标准的总审编（QualityGate Reviewer）。
你的任务是对正文草稿进行严格的综合审校。
审查重点：
1. 世界法则与实体状态不变量：主角是否违背了世界法则？是否使用了未持有的物品？
2. 节拍因果履约：4 个节拍的核心动作和打破预期是否全部落实？
3. 反AI味声口：句长是否有起伏？是否存在情绪悬浮或空洞废话？
4. 伏笔回收判定：若正文成功回收或推进了提供的开放伏笔，在 resolved_hook_ids 中列出其 ID。

必须且仅输出标准 JSON 格式：
{
  "verdict": "ACCEPTED" 或 "REVISION_NEEDED",
  "score": 1到100的整数 (80分及以上方可通过),
  "issues": ["具体问题1", "具体问题2"],
  "suggestions": "具体的定向修改整改指令",
  "resolved_hook_ids": ["已在此章成功回收的伏笔ID列表，若无则为空数组 []"]
}`

	beatsJSON, _ := json.Marshal(beats)

	userPrompt := fmt.Sprintf(`【送审章节】《%s》 第 %d 章
【世界法则公理】%s
【主角当前状态】%s | 物品栏: %s
【要求履行的节拍】%s
【底层算法质检预警】%s
【参考开放伏笔】
%s

【待审正文草稿】
%s

请给出你的综合审阅判决。`,
		project.Title, chapterIndex,
		project.WorldRules,
		project.Protagonist.NameAndLevel, project.Protagonist.Inventory,
		string(beatsJSON),
		heuristicNotes,
		hooksContext,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := q.client.ChatCompletionWithUsage(ctxRole, reviewerModel, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, usage, fmt.Errorf("quality gate LLM review failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var out struct {
		Verdict         domain.ReviewVerdict `json:"verdict"`
		Score           int                  `json:"score"`
		Issues          []string             `json:"issues"`
		Suggestions     string               `json:"suggestions"`
		ResolvedHookIDs []string             `json:"resolved_hook_ids"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, usage, fmt.Errorf("parse quality gate JSON failed (raw: %s): %w", resp, err)
	}

	// 3. Integrate heuristic findings with semantic findings
	var allIssues []string
	if len(heuristic.HitBannedWords) > 0 {
		allIssues = append(allIssues, fmt.Sprintf("命中AI套词: %s", strings.Join(heuristic.HitBannedWords, ", ")))
	}
	if heuristic.BurstinessScore < 35 {
		allIssues = append(allIssues, fmt.Sprintf("句式过于单一平缓 (突发度 %d 分)", heuristic.BurstinessScore))
	}
	exclExcessive := IsExclamationExcessive(draftText, heuristic.ExclamationDensity)
	if exclExcessive {
		allIssues = append(allIssues, fmt.Sprintf("感叹号严重超标 (每千字 %.1f 个)", heuristic.ExclamationDensity))
	}
	if len(heuristic.TopRepeatedNgrams) >= 3 {
		allIssues = append(allIssues, fmt.Sprintf("机械性重复短语: %s", strings.Join(heuristic.TopRepeatedNgrams, ", ")))
	}
	allIssues = append(allIssues, out.Issues...)

	// Heuristic penalty on score if cliches or structural anomalies exist
	finalScore := out.Score
	if len(heuristic.HitBannedWords) > 0 {
		finalScore -= len(heuristic.HitBannedWords) * 4
	}
	if exclExcessive {
		finalScore -= 6
	}
	if len(heuristic.TopRepeatedNgrams) >= 3 {
		finalScore -= 4
	}
	if finalScore < 0 {
		finalScore = 0
	}

	// Verdict check: if LLM rejected or score < 80 or cliches/exclamation exist, enforce REVISION_NEEDED.
	// Semantic reviewer veto is fail-closed and cannot be overridden by score.
	verdict := out.Verdict
	if out.Verdict == domain.ReviewVerdictAccepted && finalScore >= 80 && len(heuristic.HitBannedWords) == 0 && !exclExcessive {
		verdict = domain.ReviewVerdictAccepted
	} else {
		verdict = domain.ReviewVerdictRevision
	}

	return &domain.AuditReport{
		Verdict:            verdict,
		Score:              finalScore,
		BurstinessScore:    heuristic.BurstinessScore,
		HitBannedWords:     heuristic.HitBannedWords,
		DialogueRatio:      heuristic.DialogueRatio,
		ParagraphVariance:  heuristic.ParagraphVariance,
		TopRepeatedNgrams:  heuristic.TopRepeatedNgrams,
		ExclamationDensity: heuristic.ExclamationDensity,
		Issues:             allIssues,
		Suggestions:        out.Suggestions,
		ResolvedHookIDs:    out.ResolvedHookIDs,
		ReviewedAt:         time.Now(),
	}, usage, nil
}
