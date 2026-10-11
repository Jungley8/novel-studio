package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// MatrixSceneGenerateRequest encapsulates parameters for generating scenes in a chapter.
type MatrixSceneGenerateRequest struct {
	VolumeIndex  int    `json:"volume_index"`
	ChapterIndex int    `json:"chapter_index"`
	ChapterTitle string `json:"chapter_title"`
	CoreConflict string `json:"core_conflict"`
	Prompt       string `json:"prompt"`
}

// GenerateMatrixScenes decomposes a chapter into atomic theatrical scenes with pacing and tension.
func (o *Orchestrator) GenerateMatrixScenes(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	req MatrixSceneGenerateRequest,
) ([]domain.Scene, TokenUsage, error) {
	systemPrompt := `你是一名网络小说分镜头编剧大师。
你的任务是将单章核心冲突拆解为 2 到 4 个紧凑的连续戏剧场次 (Scenes)。
每个场次必须具备清晰的戏剧目标、冲突阻碍与张力曲线。

必须且仅输出 JSON 数组，格式如下：
[
  {
    "scene_index": 1,
    "title": "场次标题（4-8字）",
    "dramatic_goal": "本场核心角色想要达成的目标",
    "conflict_barrier": "阻碍该目标的现实或对手阻力",
    "tension_level": 1到10的整数张力评分,
    "prose_content": "本场次的粗纲与看点扼要（50-100字）"
  }
]`

	volGoal := "推进主线"
	if project != nil && project.Framework != nil {
		for _, v := range project.Framework.VolumeArcs {
			if v.VolumeIndex == req.VolumeIndex {
				volGoal = fmt.Sprintf("第%d卷《%s》: 核心目标[%s], 卷终高潮[%s]", v.VolumeIndex, v.Title, v.CoreGoal, v.Climax)
				break
			}
		}
	}

	userPrompt := fmt.Sprintf(`【分卷任务】%s
【章节】第 %d 章 《%s》
【核心冲突】%s
【补充要求】%s

请将其拆解为戏剧场次并输出 JSON 数组。`,
		volGoal, req.ChapterIndex, req.ChapterTitle, req.CoreConflict, req.Prompt,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.6)
	if err != nil {
		return nil, usage, fmt.Errorf("generate matrix scenes failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean matrix scenes json failed: %w", err)
	}

	var scenes []domain.Scene
	if err := json.Unmarshal([]byte(cleanJSON), &scenes); err != nil {
		return nil, usage, fmt.Errorf("unmarshal matrix scenes failed: %w", err)
	}

	now := time.Now()
	for i := range scenes {
		if scenes[i].SceneIndex <= 0 {
			scenes[i].SceneIndex = i + 1
		}
		if scenes[i].TensionLevel <= 0 {
			scenes[i].TensionLevel = 5
		}
		if project != nil {
			scenes[i].ProjectID = project.ID
		}
		scenes[i].CreatedAt = now
		scenes[i].UpdatedAt = now
	}

	return scenes, usage, nil
}

// AnalyzeProtagonistState analyzes recent chapter text and deduces updated character state machine.
func (o *Orchestrator) AnalyzeProtagonistState(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	protagonist *domain.CodexEntry,
	chapters []*domain.Chapter,
) (*domain.CodexEntry, TokenUsage, error) {
	if project == nil {
		return nil, TokenUsage{}, errors.New("project cannot be nil")
	}
	if protagonist == nil {
		return nil, TokenUsage{}, errors.New("protagonist cannot be nil")
	}

	var recentExcerpts []string
	startIdx := 0
	if len(chapters) > 3 {
		startIdx = len(chapters) - 3
	}
	for _, c := range chapters[startIdx:] {
		contentSnippet := c.Content
		if len(contentSnippet) > 1200 {
			contentSnippet = contentSnippet[:1200] + "...(略)"
		}
		recentExcerpts = append(recentExcerpts, fmt.Sprintf("【第 %d 章 %s】\n%s", c.ChapterIndex, c.Title, contentSnippet))
	}

	systemPrompt := `你是一名网络小说严谨战力数值与角色状态精算师。
你的任务是根据主角当前已有状态和最新章节发生的事件，推演主角的最新实体状态机。
严禁凭空给主角添加未提及的逆天造化。

必须且仅输出 JSON 格式：
{
  "name_and_level": "主角姓名与当前最新境界（如：陆青 (练气六层巅峰)）",
  "inventory": "最新随身物品与道具清单（如：长剑x1, 残破古玉x1, 灵石x5）",
  "core_goal": "最新行事目标与紧迫危机",
  "health_status": "当前伤势、心境或体魄状态（如：经脉轻伤，神念微损）",
  "breakthrough_event": {
    "happened": true或false,
    "from_realm": "原境界",
    "to_realm": "新境界",
    "reason": "突破机缘契机"
  }
}`

	userPrompt := fmt.Sprintf(`【原主角状态】
姓名与境界：%s
随身物品：%s
当前目标：%s
健康状态：%s

【最新章节发生事件】
%s

请推演并输出更新后的主角状态 JSON。`,
		protNameLevel(protagonist),
		protInventory(protagonist),
		protGoal(protagonist),
		protHealth(protagonist),
		strings.Join(recentExcerpts, "\n\n"),
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, usage, fmt.Errorf("analyze protagonist state failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean protagonist state json failed: %w", err)
	}

	type stateDTO struct {
		NameAndLevel      string `json:"name_and_level"`
		Inventory         string `json:"inventory"`
		CoreGoal          string `json:"core_goal"`
		HealthStatus      string `json:"health_status"`
		BreakthroughEvent *struct {
			Happened  bool   `json:"happened"`
			FromRealm string `json:"from_realm"`
			ToRealm   string `json:"to_realm"`
			Reason    string `json:"reason"`
		} `json:"breakthrough_event"`
	}

	var dto stateDTO
	if err := json.Unmarshal([]byte(cleanJSON), &dto); err != nil {
		return nil, usage, fmt.Errorf("unmarshal protagonist state failed: %w", err)
	}

	updated := *protagonist
	if dto.NameAndLevel != "" {
		if idx := strings.Index(dto.NameAndLevel, "("); idx != -1 && strings.HasSuffix(dto.NameAndLevel, ")") {
			updated.Realm = strings.TrimSuffix(dto.NameAndLevel[idx+1:], ")")
		} else if idx := strings.Index(dto.NameAndLevel, "（"); idx != -1 && strings.HasSuffix(dto.NameAndLevel, "）") {
			updated.Realm = strings.TrimSuffix(dto.NameAndLevel[idx+len("（"):], "）")
		} else {
			updated.Realm = dto.NameAndLevel
		}
	}
	if dto.Inventory != "" {
		updated.Inventory = dto.Inventory
	}
	if dto.CoreGoal != "" {
		updated.CoreMotivation = dto.CoreGoal
		updated.Summary = dto.CoreGoal
	}
	if dto.HealthStatus != "" {
		updated.HealthStatus = dto.HealthStatus
	}

	if dto.BreakthroughEvent != nil && dto.BreakthroughEvent.Happened {
		if updated.StructuredLevel == nil {
			updated.StructuredLevel = &domain.PowerLevel{}
		}
		chapterNum := len(chapters)
		updated.StructuredLevel.History = append(updated.StructuredLevel.History, domain.LevelTransition{
			FromRealm: dto.BreakthroughEvent.FromRealm,
			ToRealm:   dto.BreakthroughEvent.ToRealm,
			Chapter:   chapterNum,
			Reason:    dto.BreakthroughEvent.Reason,
			Timestamp: time.Now(),
		})
		updated.StructuredLevel.Realm = dto.BreakthroughEvent.ToRealm
		updated.Realm = dto.BreakthroughEvent.ToRealm
	}
	updated.UpdatedAt = time.Now()

	return &updated, usage, nil
}

// ExtractPlotHooks reads story text and extracts unaddressed clues or suspense hooks.
func (o *Orchestrator) ExtractPlotHooks(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	chapters []*domain.Chapter,
	existingHooks []*domain.PlotHook,
) ([]domain.PlotHook, TokenUsage, error) {
	if project == nil {
		return nil, TokenUsage{}, errors.New("project cannot be nil")
	}

	var existingTitles []string
	for _, h := range existingHooks {
		existingTitles = append(existingTitles, fmt.Sprintf("- [%s] %s (第%d章)", h.Status, h.Title, h.TargetChapter))
	}

	var recentChapters []string
	startIdx := 0
	if len(chapters) > 3 {
		startIdx = len(chapters) - 3
	}
	for _, c := range chapters[startIdx:] {
		contentSnippet := c.Content
		if len(contentSnippet) > 1000 {
			contentSnippet = contentSnippet[:1000] + "...(略)"
		}
		recentChapters = append(recentChapters, fmt.Sprintf("【第 %d 章 %s】\n核心冲突: %s\n正文片段: %s", c.ChapterIndex, c.Title, c.CoreConflict, contentSnippet))
	}

	systemPrompt := `你是一名网络小说长线伏笔挖掘大师。
你的任务是根据最新剧情与全书设定，挖掘正文中自然留下的未解悬念、可疑细节、敌对暗流或未兑现承诺，形成高质量的长线伏笔。
避免提取已被记录的重复伏笔。

必须且仅输出 JSON 数组：
[
  {
    "title": "伏笔标题（4-10字，如：神秘青铜残印的器灵）",
    "details": "具体线索细节与可能引发的后续危机或反转",
    "created_chapter": 埋下的章节号,
    "target_chapter": 建议回收或爆发的章节号 (通常为当前章+3至+20章)
  }
]`

	currentChapter := len(chapters)
	if currentChapter == 0 {
		currentChapter = 1
	}

	userPrompt := fmt.Sprintf(`【当前已有伏笔】
%s

【最新正文脉络】
%s

【当前进行至】第 %d 章

请挖掘出 2 到 4 条有价值的新伏笔，输出 JSON 数组。`,
		strings.Join(existingTitles, "\n"),
		strings.Join(recentChapters, "\n\n"),
		currentChapter,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.6)
	if err != nil {
		return nil, usage, fmt.Errorf("extract plot hooks failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean hooks json failed: %w", err)
	}

	type rawHook struct {
		Title          string `json:"title"`
		Details        string `json:"details"`
		CreatedChapter int    `json:"created_chapter"`
		TargetChapter  int    `json:"target_chapter"`
	}

	var rawHooks []rawHook
	if err := json.Unmarshal([]byte(cleanJSON), &rawHooks); err != nil {
		return nil, usage, fmt.Errorf("unmarshal hooks failed: %w", err)
	}

	var hooks []domain.PlotHook
	now := time.Now()
	for i, rh := range rawHooks {
		if strings.TrimSpace(rh.Title) == "" {
			continue
		}
		cChap := rh.CreatedChapter
		if cChap <= 0 {
			cChap = currentChapter
		}
		tChap := rh.TargetChapter
		if tChap <= cChap {
			tChap = cChap + 5
		}
		hooks = append(hooks, domain.PlotHook{
			ID:             fmt.Sprintf("hook_ai_%d_%d", now.UnixNano(), i+1),
			ProjectID:      project.ID,
			Title:          rh.Title,
			Details:        rh.Details,
			CreatedChapter: cChap,
			TargetChapter:  tChap,
			Status:         domain.HookStatusOpen,
			CreatedAt:      now,
		})
	}

	return hooks, usage, nil
}

// SuggestChapterConflict prompts the reasoning model to invent an intense, dramatic core conflict
// for the target chapter based on canon history, current volume arc, urgent hooks, and protagonist goals.
func (o *Orchestrator) SuggestChapterConflict(
	ctx context.Context,
	reasoningModel string,
	horizon *CanonHorizon,
) (string, TokenUsage, error) {
	if horizon == nil || horizon.Project == nil {
		return "", TokenUsage{}, errors.New("horizon and project are required to suggest chapter conflict")
	}

	systemPrompt := fmt.Sprintf(`你是一名网络小说白金级剧情架构师与高潮推演专家。
你的任务是根据当前作品设定、世界公理、当前卷主线目标、主角当前境遇与开放伏笔，为即将展开的第 %d 章推演一个张力饱满、打破预期的【核心剧情冲突与关键转折】。

要求：
1. 冲突必须紧扣当前卷主线，并强力推动主角核心目标；
2. 包含明确的外部压迫力量、生死/利益对峙或意料之外的规则危机；
3. 禁止空洞抽象的大道理或概念叙述，必须具备具体的具象动作与事件爆点（如：某势力登门索要信物、护道禁制突遭反噬、拍卖会上截胡宿敌、绝境中不得不以身试险等）；
4. 语言紧凑有力，纯文本输出 50~150 字，严禁包含任何前缀（如“本章核心冲突：”、“冲突：”等）或引号，直接输出冲突陈述本身。`, horizon.TargetChapter)

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

	callbacksText := ""
	if strings.TrimSpace(horizon.HistoricalCallbacks) != "" {
		callbacksText = "\n\n" + horizon.HistoricalCallbacks
	}

	var volumeContext string
	if horizon != nil {
		volumeContext = buildVolumeContextBlock(horizon.CurrentVolume, horizon.TargetChapter)
	}

	var powerContext string
	if horizon.ActivePowerTier != nil {
		powerContext = fmt.Sprintf("\n【当前战力境界法则】\n- 境界：%s (%s)\n- 升级瓶颈：%s\n- 天道代价：%s\n",
			horizon.ActivePowerTier.Realm, horizon.ActivePowerTier.Description,
			horizon.ActivePowerTier.Bottleneck, horizon.ActivePowerTier.Drawback)
	}

	codexContext := ""
	if strings.TrimSpace(horizon.CodexContextText) != "" {
		codexContext = "\n\n" + horizon.CodexContextText
	}

	project := horizon.Project
	userPrompt := fmt.Sprintf(`【作品信息】
书名：《%s》
目标平台：%s
当前章节序号：第 %d 章

【世界公理与不可违背法则】
%s%s%s

【主角当前状态机】
姓名与等级：%s
随身物品栏：%s
当前隐秘目标：%s%s

【前序正史视界 (最近 3 章密封剧情)】
%s%s

【当前开放状态的伏笔】
%s

请推演第 %d 章的核心冲突与事件爆点（纯文本直接输出）：`,
		project.Title, project.TargetPlatform, horizon.TargetChapter,
		horizon.WorldRules, volumeContext, powerContext,
		protNameLevel(horizon.ProtagonistEntry), protInventory(horizon.ProtagonistEntry), protGoal(horizon.ProtagonistEntry), codexContext,
		rollingCanon, callbacksText,
		hooksSummary, horizon.TargetChapter,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.7)
	if err != nil {
		return "", usage, fmt.Errorf("suggest chapter conflict failed: %w", err)
	}

	// Clean any markdown formatting or prefix
	cleaned := strings.TrimSpace(resp)
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	for _, prefix := range []string{"本章冲突：", "核心冲突：", "剧情冲突：", "冲突："} {
		if strings.HasPrefix(cleaned, prefix) {
			cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, prefix))
		}
	}
	cleaned = strings.Trim(cleaned, `"'“”`)

	return cleaned, usage, nil
}
