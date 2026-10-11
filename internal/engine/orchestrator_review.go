package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

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
