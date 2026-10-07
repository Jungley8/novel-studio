package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	if len(activeHooks) > 0 {
		var hList []string
		for _, h := range activeHooks {
			hList = append(hList, fmt.Sprintf("- [%s] %s (目标回收章节: %d)", h.Status, h.Title, h.TargetChapter))
		}
		hooksSummary = strings.Join(hList, "\n")
	}

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

【当前开放状态的伏笔】
%s

【本章必须打破的核心冲突】
%s

请推演输出严格合法的 JSON。`,
		project.Title, project.TargetPlatform, chapterIndex,
		project.WorldRules,
		project.Protagonist.NameAndLevel, project.Protagonist.Inventory, project.Protagonist.CoreGoal,
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

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}
