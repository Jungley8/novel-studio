package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type rawIntentCard struct {
	ChapterIndex     int                  `json:"chapter_index"`
	Conflict         string               `json:"conflict"`
	POV              string               `json:"pov"`
	EmotionCurve     []string             `json:"emotion_curve"`
	HookPlants       []string             `json:"hook_plants"`
	HookPayoffs      []string             `json:"hook_payoffs"`
	TensionTarget    int                  `json:"tension_target"`
	ExpectedMutation domain.StateMutation `json:"expected_mutation"`
}

// GenerateVolumeIntents 为一个分卷批量推演章节意图卡；existingChapters 中的章号跳过（只补缺）。
func (o *Orchestrator) GenerateVolumeIntents(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	arc domain.VolumeArc,
	existingChapters map[int]bool,
) ([]domain.ChapterIntent, TokenUsage, error) {
	if project == nil {
		return nil, TokenUsage{}, errors.New("project cannot be nil")
	}

	// 1. targets = [arc.StartChapter, arc.EndChapter] 中不在 existingChapters 的章号（升序）
	var targets []int
	if arc.StartChapter > 0 && arc.EndChapter >= arc.StartChapter {
		for ch := arc.StartChapter; ch <= arc.EndChapter; ch++ {
			if existingChapters == nil || !existingChapters[ch] {
				targets = append(targets, ch)
			}
		}
	}

	// 2. targets 空 → 直接返回空切片 + 零 Token（不发 LLM 请求）
	if len(targets) == 0 {
		return []domain.ChapterIntent{}, TokenUsage{}, nil
	}

	// 3. 超上限 Fail-Closed：len(targets) > 60 → error（文案含分段建议）
	if len(targets) > 60 {
		return nil, TokenUsage{}, fmt.Errorf("target chapters count (%d) exceeds limit of 60; please narrow chapter range or split into batches", len(targets))
	}

	// 4. systemPrompt: 网文剧情规划师
	systemPrompt := `你是一名资深网络小说剧情规划师与大纲构架师。
你的任务是为指定的章节批量推演章节意图卡（Chapter Intents）。
必须且仅输出严格的 JSON 数组，每个元素包含以下字段：
[
  {
    "chapter_index": 章节序号（正整数）,
    "conflict": "本章核心矛盾与戏剧冲突阻碍（必填，非空）",
    "pov": "本章主要视角人物",
    "emotion_curve": ["情绪词1", "情绪词2", "情绪词3"], // 读者情绪起伏曲线（包含3到5个词）
    "hook_plants": ["本章埋下的伏笔或悬念钩子"],
    "hook_payoffs": ["本章回收的前序伏笔或兑现承诺"],
    "tension_target": 1到10之间的整数张力目标,
    "expected_mutation": {
      "inventory_delta": "重要物品/法宝/资源变动",
      "power_delta": "战力/修为境界/伤势变动"
    }
  }
]

推演铁律：
1. 张力与情绪曲线必须契合分卷的整体节奏脉络：开卷蓄势铺垫、中段低谷蓄力或危机反转、卷末高潮爆发（结合卷终大高潮 Climax 设计）。
2. 每张卡必须严格对应目标章号列表中的章节，不得遗漏或错乱。
3. 必须输出纯净合法的 JSON 数组，严禁包含任何前置或后置对话。`

	// 5. userPrompt: 书名/平台/WorldRules 摘要 + arc.Title/CoreGoal/Climax/KeyPayoffs + 目标章号列表
	var targetsStr []string
	for _, t := range targets {
		targetsStr = append(targetsStr, fmt.Sprintf("%d", t))
	}

	userPrompt := fmt.Sprintf(`【作品信息】
- 书名：%s
- 目标平台：%s
- 世界观/规则摘要：%s

【分卷纲要 (卷 %d)】
- 卷名：%s
- 核心主线目标：%s
- 卷终大高潮：%s
- 核心伏笔回收：%s

【推演目标】
请为以下目标章号逐章推演意图卡：%s`,
		project.Title,
		project.TargetPlatform,
		project.WorldRules,
		arc.VolumeIndex,
		arc.Title,
		arc.CoreGoal,
		arc.Climax,
		strings.Join(arc.KeyPayoffs, "、"),
		strings.Join(targetsStr, ", "),
	)

	// 6. ContextWithRole(RoleReasoner) + SafeChatCompletionWithStreamFallback + ExtractAndCleanJSON
	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, o.client, reasoningModel, systemPrompt, userPrompt, 0.4, nil)
	if err != nil {
		return nil, usage, fmt.Errorf("generate volume intents LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean volume intents json failed: %w", err)
	}

	rawCards, err := UnmarshalJSONArray[rawIntentCard](cleanJSON)
	if err != nil {
		return nil, usage, fmt.Errorf("unmarshal volume intents json failed: %w", err)
	}

	if len(rawCards) == 0 {
		return nil, usage, errors.New("empty chapter intents returned from LLM")
	}

	// 7. 映射 domain：ProjectID/VolumeIndex/ChapterIndex/Status=IntentStatusDraft/UpdatedAt=now；逐张 Validate()
	intents := make([]domain.ChapterIntent, 0, len(rawCards))
	now := time.Now().UTC()
	for i, raw := range rawCards {
		chIdx := raw.ChapterIndex
		if chIdx <= 0 && i < len(targets) {
			chIdx = targets[i]
		}

		card := domain.ChapterIntent{
			ID:               fmt.Sprintf("intent_%s_%d", project.ID, chIdx),
			ProjectID:        project.ID,
			VolumeIndex:      arc.VolumeIndex,
			ChapterIndex:     chIdx,
			Conflict:         strings.TrimSpace(raw.Conflict),
			POV:              strings.TrimSpace(raw.POV),
			EmotionCurve:     raw.EmotionCurve,
			HookPlants:       raw.HookPlants,
			HookPayoffs:      raw.HookPayoffs,
			ExpectedMutation: raw.ExpectedMutation,
			TensionTarget:    raw.TensionTarget,
			Status:           domain.IntentStatusDraft,
			UpdatedAt:        now,
		}

		if err := card.Validate(); err != nil {
			return nil, usage, fmt.Errorf("validate chapter intent at index %d (chapter %d) failed: %w", i, chIdx, err)
		}

		intents = append(intents, card)
	}

	return intents, usage, nil
}
