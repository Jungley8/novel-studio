package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func (o *Orchestrator) ReviewDraft(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
	protagonist *domain.CodexEntry,
	chapterIndex int,
	beats []domain.SceneBeat,
	draftText string,
) (*domain.ReviewResult, TokenUsage, error) {
	systemPrompt := `你是一名极其挑剔、拥有十余年网文编辑经验的总编审（Reviewer）。
你的任务是对送审的裸正文草稿进行严格审查，寻找：
1. 战力崩坏与设定矛盾：主角是否使用了物品栏中没有的道具？是否违反了世界法则？
2. 节拍偏差：是否漏掉了规定的核心动作或反转？
3. 声口与AI套路：是否有AI模式化空洞叙述或情绪悬浮？

输出格式：必须且仅输出标准 JSON：
{
  "verdict": "ACCEPTED" 或 "REVISION_NEEDED",
  "score": 1到100的整数 (80分及以上方可通过),
  "issues": ["具体问题1", "具体问题2"],
  "suggestions": "具体的修改整改指令"
}`

	beatsJSON, _ := json.Marshal(beats)

	userPrompt := fmt.Sprintf(`【送审章节】《%s》 第 %d 章
【世界法则公理】%s
【主角当前状态】%s | 物品栏: %s
【要求履行的节拍】%s

【待审正文草稿】
%s

请给出你的审阅判决。`,
		project.Title, chapterIndex,
		project.WorldRules,
		protNameLevel(protagonist), protInventory(protagonist),
		string(beatsJSON),
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, o.client, reviewerModel, systemPrompt, userPrompt, 0.3, nil)
	if err != nil {
		return nil, usage, fmt.Errorf("review draft LLM call failed: %w", err)
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
		return nil, usage, fmt.Errorf("parse review JSON failed (raw: %s): %w", resp, err)
	}

	heuristic := NewLinter(nil).Analyze(draftText)
	finalScore := out.Score
	if len(heuristic.HitBannedWords) > 0 {
		finalScore -= len(heuristic.HitBannedWords) * 4
	}
	if IsExclamationExcessive(draftText, heuristic.ExclamationDensity) {
		finalScore -= 6
	}
	if len(heuristic.TopRepeatedNgrams) >= 3 {
		finalScore -= 4
	}
	if finalScore < 0 {
		finalScore = 0
	}
	out.Verdict = decideVerdict(out.Verdict, finalScore, draftText, heuristic)

	return &domain.ReviewResult{
		Verdict:         out.Verdict,
		Score:           finalScore,
		Issues:          out.Issues,
		Suggestions:     out.Suggestions,
		ResolvedHookIDs: out.ResolvedHookIDs,
		ReviewedAt:      time.Now(),
	}, usage, nil
}

// RewriteOptions configures targeted rewrite criteria.
type RewriteOptions struct {
	WordsTarget    int    `json:"words_target,omitempty"`
	NarrativeStyle string `json:"narrative_style,omitempty"`
}

func (o *Orchestrator) RewriteDraft(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	originalDraft string,
	review *domain.ReviewResult,
) (string, TokenUsage, error) {
	return o.RewriteDraftWithOptions(ctx, writerModel, project, chapterIndex, originalDraft, review, RewriteOptions{})
}

func (o *Orchestrator) RewriteDraftWithOptions(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	originalDraft string,
	review *domain.ReviewResult,
	opts RewriteOptions,
) (string, TokenUsage, error) {
	return o.RewriteDraftStream(ctx, writerModel, project, chapterIndex, originalDraft, review, opts, nil)
}

func (o *Orchestrator) RewriteDraftStream(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	originalDraft string,
	review *domain.ReviewResult,
	opts RewriteOptions,
	onDelta func(string),
) (string, TokenUsage, error) {
	toneDirective := buildNarrativeTonePrompt(opts.NarrativeStyle)

	systemPrompt := fmt.Sprintf(`你是一名顶级网文精修专家。你的任务是根据主审的具体驳回意见，对原草稿进行定向精修与差分重构。

%s

精修与去AI味铁律：
1. 严格修复主审指出的所有问题，逐条落实整改意见。
2. 保持自然中文语法健全：严禁写成残缺不全的电报式碎词短语。主语、谓语、宾语要完整自然，严禁单字单词独行（如“响。”、“烫。”、“骨响。咔。”）。
3. 翻案腔外科手术：彻底斩断“不是……而是……”、“看似……实则……”、“与其说……不如说……”的虚立假靶子套路，正面直击主线断言与动作。
4. 动名词消肿：拔除公文式名词化膨胀（如“完成了对……的……”），恢复原始强动词推进。
5. 拔除书面比喻与拟人工具喻体：删掉所有“像...”、“如...”、“宛若...”式书面比喻，严禁将法宝阵法比作“像一位智慧的导师/管家”，用真实物理机理与骨肉破坏直接呈现。
6. 状语与副词克制：能不用状语就不用状语，能不用副词就不用副词。彻底禁用“...地”修饰动词，用强动词和名词质感直接呈现动作，绝不用副词当拐杖。
7. 拒绝微表情与慢动作：删掉“嘴角弧度”、“瞳孔针尖”、“眼角笑意”，动作保持粗粝主观聚焦。
8. 严禁游戏技能战报：绝不能像网游战报一样生硬报出技能名或念出消耗数值。
9. 【差分保护与篇幅充实】集中火力解决病灶，直接输出精修重构后的完整正文，无需任何客套寒暄。`, toneDirective)

	issuesText := "无明显硬伤"
	if len(review.Issues) > 0 {
		issuesText = "- " + strings.Join(review.Issues, "\n- ")
	}

	var budgetNotice string
	if opts.WordsTarget > 0 {
		budgetNotice = fmt.Sprintf("\n【篇幅与字数底线要求】\n精修重构后全章篇幅必须达到 %d 字以上，细节充实饱满，严禁因删减修辞而导致字数缩水！\n", opts.WordsTarget)
	}

	userPrompt := fmt.Sprintf(`【重修任务】《%s》 第 %d 章
【主编评分】%d 分 | 判决: %s
【审查指出的硬伤】
%s
【整改建议】%s%s

【原始草稿】
%s

请输出精修重构后的完整正文。`,
		project.Title, chapterIndex,
		review.Score, review.Verdict,
		issuesText, review.Suggestions,
		budgetNotice,
		originalDraft,
	)

	ctxRole := ContextWithRole(ctx, RoleWriter)
	return SafeChatCompletionWithStreamFallback(ctxRole, o.client, writerModel, systemPrompt, userPrompt, 0.7, onDelta)
}
