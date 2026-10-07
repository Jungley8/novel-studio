package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// Orchestrator coordinates the reasoning engine and writing engine.
type Orchestrator struct {
	client LLMClient
}

func NewOrchestrator(client LLMClient) *Orchestrator {
	return &Orchestrator{client: client}
}

type DeriveBeatsOutput struct {
	Beats         []domain.SceneBeat   `json:"beats"`
	StateMutation domain.StateMutation `json:"state_mutation"`
}

func (o *Orchestrator) DeriveBeats(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	chapterIndex int,
	coreConflict string,
	activeHooks []*domain.PlotHook,
) (*DeriveBeatsOutput, error) {
	horizon := &CanonHorizon{
		Project:          project,
		TargetChapter:    chapterIndex,
		RollingCanonText: "(无前序历史)",
		AllActiveHooks:   activeHooks,
		ProtagonistState: project.Protagonist,
		WorldRules:       project.WorldRules,
	}
	return o.DeriveBeatsWithHorizon(ctx, reasoningModel, horizon, coreConflict)
}

func (o *Orchestrator) DeriveBeatsWithHorizon(
	ctx context.Context,
	reasoningModel string,
	horizon *CanonHorizon,
	coreConflict string,
) (*DeriveBeatsOutput, error) {
	systemPrompt := `你是一名网络小说架构与读者心理学总设计师。你严禁输出抒情散文。
你的任务是根据给定的主角实体状态、世界公理与核心冲突，推演下一章严密的 4 个剧情节拍 (Beats)。
必须以纯 JSON 格式返回，包含：
{
  "beats": [
    {
      "phase": "蓄力压迫 | 试探下套 | 绝地反转 | 章末留钩",
      "tension": 1到10的整数,
      "action": "具体的物理动作事实",
      "expectation_broken": "谁的心理预期被打破了"
    }
  ],
  "state_mutation": {
    "inventory_delta": "物品变动描述，如：消耗青钢剑，获得残破古玉",
    "power_delta": "战力或修为变动描述"
  }
}`

	hooksSummary := "无"
	if len(horizon.AllActiveHooks) > 0 {
		var hList []string
		for _, h := range horizon.AllActiveHooks {
			prefix := ""
			if len(horizon.UrgentHooks) > 0 {
				for _, uh := range horizon.UrgentHooks {
					if uh.ID == h.ID {
						prefix = "【🔥近期临期】"
						break
					}
				}
			}
			hList = append(hList, fmt.Sprintf("- %s[%s] %s (目标回收: 第 %d 章)", prefix, h.Status, h.Title, h.TargetChapter))
		}
		hooksSummary = strings.Join(hList, "\n")
	}

	rollingCanon := "(开篇第一章，无前序历史)"
	if strings.TrimSpace(horizon.RollingCanonText) != "" {
		rollingCanon = horizon.RollingCanonText
	}

	project := horizon.Project
	userPrompt := fmt.Sprintf(`【作品信息】
书名：《%s》
目标平台：%s
当前章节序号：第 %d 章

【世界公理与不可违背法则】
%s

【主角当前状态机】
姓名与等级：%s
随身物品栏：%s
当前隐秘目标：%s

【前序正史视界 (最近 3 章密封剧情)】
%s

【当前开放状态的伏笔】
%s

【本章必须打破的核心冲突】
%s

请推演输出严格合法的 JSON。`,
		project.Title, project.TargetPlatform, horizon.TargetChapter,
		horizon.WorldRules,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory, horizon.ProtagonistState.CoreGoal,
		rollingCanon,
		hooksSummary,
		coreConflict,
	)

	resp, err := o.client.ChatCompletion(ctx, reasoningModel, systemPrompt, userPrompt, 0.4)
	if err != nil {
		return nil, fmt.Errorf("derive beats LLM call failed: %w", err)
	}

	cleanJSON := extractJSON(resp)
	var out DeriveBeatsOutput
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, fmt.Errorf("parse beats JSON failed (raw: %s): %w", resp, err)
	}

	return &out, nil
}

func (o *Orchestrator) RenderScene(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	beats []domain.SceneBeat,
	wordsTarget int,
) (string, error) {
	if wordsTarget <= 0 {
		wordsTarget = 2000
	}

	systemPrompt := `你是一名冷峻、极具电影镜头感的网络小说名家。
写作铁律：
1. 严禁出现以下AI模式化废词：不由得、仿佛、宛若、嘴角勾起、眼神复杂、一时间、殊不知、与此同时、冷哼一声。
2. 句长节奏（突发度）：战斗与对峙必须多用 3-6 字短句（可单句成段）；氛围烘托多用感官长句，长短句剧烈交替。
3. Show, don't tell：严禁直抒胸臆“他很愤怒”，必须通过瞳孔骤缩、指关节泛白、下意识屏住呼吸等微动作体现。
4. 严格按照提供的 4 个节拍事实展开，不得擅自修改因果大纲。`

	beatsJSON, _ := json.MarshalIndent(beats, "", "  ")

	userPrompt := fmt.Sprintf(`【场景输入】
书名：《%s》
第 %d 章
目标字数：%d 字左右

【主角状态参考】
%s | 携带物品：%s

【必须执行的 4 个场景节拍】
%s

请直接输出小说正文内容，无需任何开场白或寒暄。`,
		project.Title, chapterIndex, wordsTarget,
		project.Protagonist.NameAndLevel, project.Protagonist.Inventory,
		string(beatsJSON),
	)

	return o.client.ChatCompletion(ctx, writerModel, systemPrompt, userPrompt, 0.75)
}

func (o *Orchestrator) ReviewDraft(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
	chapterIndex int,
	beats []domain.SceneBeat,
	draftText string,
) (*domain.ReviewResult, error) {
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
		project.Protagonist.NameAndLevel, project.Protagonist.Inventory,
		string(beatsJSON),
		draftText,
	)

	resp, err := o.client.ChatCompletion(ctx, reviewerModel, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, fmt.Errorf("review draft LLM call failed: %w", err)
	}

	cleanJSON := extractJSON(resp)
	var out struct {
		Verdict     domain.ReviewVerdict `json:"verdict"`
		Score       int                  `json:"score"`
		Issues      []string             `json:"issues"`
		Suggestions string               `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, fmt.Errorf("parse review JSON failed (raw: %s): %w", resp, err)
	}

	if out.Verdict != domain.ReviewVerdictAccepted && out.Verdict != domain.ReviewVerdictRevision {
		if out.Score >= 80 {
			out.Verdict = domain.ReviewVerdictAccepted
		} else {
			out.Verdict = domain.ReviewVerdictRevision
		}
	}

	return &domain.ReviewResult{
		Verdict:     out.Verdict,
		Score:       out.Score,
		Issues:      out.Issues,
		Suggestions: out.Suggestions,
		ReviewedAt:  time.Now(),
	}, nil
}

func (o *Orchestrator) RewriteDraft(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	originalDraft string,
	review *domain.ReviewResult,
) (string, error) {
	systemPrompt := `你是一名顶级网文精修专家。你的任务是根据主编审（Reviewer）的具体驳回意见，对原草稿进行定向精修与重写。
规则：
1. 严格修复主编指出的所有问题，落实整改意见。
2. 保持优秀句段，重构被指出的机械或违规段落。
3. 严格遵循 Show, don't tell，杜绝AI套话。直接输出重修后的正文。`

	issuesText := "无明显硬伤"
	if len(review.Issues) > 0 {
		issuesText = "- " + strings.Join(review.Issues, "\n- ")
	}

	userPrompt := fmt.Sprintf(`【重修任务】《%s》 第 %d 章
【主编评分】%d 分 | 判决: %s
【审查指出的硬伤】
%s
【整改建议】%s

【原始草稿】
%s

请输出精修重构后的完整正文。`,
		project.Title, chapterIndex,
		review.Score, review.Verdict,
		issuesText, review.Suggestions,
		originalDraft,
	)

	return o.client.ChatCompletion(ctx, writerModel, systemPrompt, userPrompt, 0.7)
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}
