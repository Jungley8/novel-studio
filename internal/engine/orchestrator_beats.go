package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type DeriveBeatsOutput struct {
	Beats         []domain.SceneBeat   `json:"beats"`
	StateMutation domain.StateMutation `json:"state_mutation"`
	Usage         TokenUsage           `json:"usage,omitempty"`
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
	return o.DeriveBeatsForChapterType(ctx, reasoningModel, horizon, coreConflict, domain.ChapterTypeEscalation)
}

func (o *Orchestrator) DeriveBeatsForChapterType(
	ctx context.Context,
	reasoningModel string,
	horizon *CanonHorizon,
	coreConflict string,
	ct domain.ChapterType,
) (*DeriveBeatsOutput, error) {
	if ct == "" {
		ct = domain.ChapterTypeEscalation
	}

	if strings.TrimSpace(coreConflict) == "" && horizon != nil && horizon.Project != nil {
		if suggested, _, err := o.SuggestChapterConflict(ctx, reasoningModel, horizon); err == nil && strings.TrimSpace(suggested) != "" {
			coreConflict = strings.TrimSpace(suggested)
		} else {
			coreConflict = fmt.Sprintf("第 %d 章核心矛盾爆发与命运转折", horizon.TargetChapter)
		}
	}

	var typeGuidance string
	var phaseGuidance string
	var hookRule string
	switch ct {
	case domain.ChapterTypeClimax:
		typeGuidance = `【本章节奏定位：CLIMAX 爆发章】
- 拍数要求：推演 4 到 6 个紧凑激烈的剧情节拍 (Beats)。
- 张力指令：全章维持高张力持续爆发（张力 8-10），核心冲突迎来最强对抗、生死反转或高潮爆发。
- 章末留钩：最后一拍必须挂强钩子（hook_type 必填 CLIFFHANGER | REVERSAL | MYSTERY | POWER_UP 之一）。`
		phaseGuidance = "危机激化 | 生死搏杀 | 绝地反扑 | 巅峰爆发 | 命运转折 | 章末留钩"
		hookRule = "最后一拍必须填写强钩子类型（CLIFFHANGER | REVERSAL | MYSTERY | POWER_UP，前序节拍留空）"
	case domain.ChapterTypeAftermath:
		typeGuidance = `【本章节奏定位：AFTERMATH 余波章】
- 拍数要求：推演 3 到 4 个剧情节拍 (Beats)。
- 张力指令：高潮后的降温收束与情绪沉淀（张力 2-5），聚焦战后清点、抚平创伤与格局重塑。
- 章末收束：允许末拍无钩子收束（hook_type 可留空，平稳收尾）。`
		phaseGuidance = "战后清点 | 抚平创伤 | 格局重构 | 沉淀修整"
		hookRule = "最后一拍允许无钩子收束（hook_type 留空或选填）"
	case domain.ChapterTypeSetup:
		typeGuidance = `【本章节奏定位：SETUP 铺垫章】
- 拍数要求：推演 3 到 4 个剧情节拍 (Beats)。
- 张力指令：低中张力暗流涌动（张力 2-5），重在展示信息差、暗线埋设与下一轮危机酝酿。
- 章末收束：允许末拍无钩子收束或留有悬念微钩（hook_type 可留空或填写）。`
		phaseGuidance = "暗流涌动 | 试探盘查 | 伏笔埋设 | 局势隐变"
		hookRule = "最后一拍可留空或填写悬念微钩类型"
	case domain.ChapterTypeInterlude:
		typeGuidance = `【本章节奏定位：INTERLUDE 转场章】
- 拍数要求：推演 3 到 4 个剧情节拍 (Beats)。
- 张力指令：舒缓平稳过渡（张力 1-4），重在旅途行进、日常调剂、世界观展示或修整盘点。
- 章末收束：允许末拍无钩子收束（hook_type 留空，张弛有度）。`
		phaseGuidance = "旅途行进 | 坊市见闻 | 心境调剂 | 势力更迭"
		hookRule = "最后一拍留空，平缓过渡"
	case domain.ChapterTypeEscalation:
		fallthrough
	default:
		typeGuidance = `【本章节奏定位：ESCALATION 蓄力章】
- 拍数要求：严密的 4 个剧情节拍 (Beats)。
- 张力指令：张力爬坡递增（由 4-5 逐步爬升至 7-8），危机步步紧逼，矛盾层层升级。
- 章末收束：最后一拍建议挂钩强化期待（hook_type 填写 CLIFFHANGER | REVERSAL | MYSTERY | POWER_UP 之一）。`
		phaseGuidance = "蓄力压迫 | 试探下套 | 矛盾激化 | 章末留钩"
		hookRule = "最后一拍必填或建议填写钩子类型（CLIFFHANGER | REVERSAL | MYSTERY | POWER_UP，前3拍留空）"
	}

	systemPrompt := fmt.Sprintf(`你是一名网络小说架构与读者心理学总设计师。你严禁输出抒情散文。
你的任务是根据给定的主角实体状态、世界公理与核心冲突，结合本章节奏定位推演剧情节拍 (Beats)。

%s

必须以纯 JSON 格式返回，包含：
{
  "beats": [
    {
      "phase": "%s",
      "tension": 1到10的整数,
      "action": "具体的物理动作事实",
      "expectation_broken": "谁的心理预期被打破了",
      "reader_emotion": "期望读者此刻的情绪（如：紧张压抑、好奇期待、大呼解气、心疼揪心）",
      "info_gap": "信息差（角色知/读者不知，或读者知/角色不知）",
      "hook_type": "%s"
    }
  ],
  "state_mutation": {
    "inventory_delta": "物品变动描述，如：消耗青钢剑，获得残破古玉",
    "power_delta": "战力或修为变动描述",
    "character_mutations": [
      {
        "name": "配角或反派姓名",
        "status_delta": "该角色状态/心境/伤势/修为变迁，如：左臂被斩断，道心动摇",
        "relation_delta": "对主角或全局的关系变化，如：由轻蔑转为刻骨仇恨"
      }
    ]
  }
}`, typeGuidance, phaseGuidance, hookRule)

	hooksSummary := "无"
	if horizon != nil && len(horizon.AllActiveHooks) > 0 {
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
	if horizon != nil && strings.TrimSpace(horizon.RollingCanonText) != "" {
		rollingCanon = horizon.RollingCanonText
	}

	callbacksText := ""
	if horizon != nil && strings.TrimSpace(horizon.HistoricalCallbacks) != "" {
		callbacksText = "\n\n" + horizon.HistoricalCallbacks
	}

	var volumeContext string
	if horizon != nil {
		volumeContext = buildVolumeContextBlock(horizon.CurrentVolume, horizon.TargetChapter)
	}

	var powerContext string
	if horizon != nil && horizon.ActivePowerTier != nil {
		powerContext = fmt.Sprintf("\n【当前战力境界法则】\n- 境界：%s (%s)\n- 升级瓶颈：%s\n- 天道代价：%s\n",
			horizon.ActivePowerTier.Realm, horizon.ActivePowerTier.Description,
			horizon.ActivePowerTier.Bottleneck, horizon.ActivePowerTier.Drawback)
	}

	codexContext := ""
	if horizon != nil && strings.TrimSpace(horizon.CodexContextText) != "" {
		codexContext = "\n\n" + horizon.CodexContextText
	}

	var rhythmForesight string
	if horizon != nil && horizon.CurrentVolume != nil {
		var recentTypes []domain.ChapterType
		for _, ch := range horizon.RecentChapters {
			if ch != nil && len(ch.Beats) > 0 {
				recentTypes = append(recentTypes, domain.ClassifyChapterType(ch.Beats))
			}
		}
		rhythmPlan := domain.PlanVolumeRhythm(horizon.CurrentVolume, recentTypes, 8)
		if len(rhythmPlan) > 0 {
			var planStrs []string
			for i, t := range rhythmPlan {
				planStrs = append(planStrs, fmt.Sprintf("+%d章:%s(%s)", i+1, t, t.ChineseName()))
			}
			rhythmForesight = fmt.Sprintf("\n【卷内节奏前瞻 (未来%d章规划)】\n%s\n本章规划定位：%s (%s)\n",
				len(rhythmPlan), strings.Join(planStrs, " -> "), ct, ct.ChineseName())
		}
	}

	projectTitle := ""
	targetPlatform := ""
	targetChapter := 1
	worldRules := ""
	var protEntry *domain.CodexEntry
	if horizon != nil {
		targetChapter = horizon.TargetChapter
		worldRules = horizon.WorldRules
		protEntry = horizon.ProtagonistEntry
		if horizon.Project != nil {
			projectTitle = horizon.Project.Title
			targetPlatform = horizon.Project.TargetPlatform
		}
	}

	var premiseBlock string
	var milestoneBlock string
	if horizon != nil && horizon.Project != nil {
		premiseBlock = buildStoryPremiseBlock(horizon.Project)
		milestoneBlock = BuildMilestoneContextBlock(horizon.Project, targetChapter)
	}

	userPrompt := fmt.Sprintf(`【作品信息】
书名：《%s》
目标平台：%s
当前章节序号：第 %d 章
%s
%s
%s
%s
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

【本章必须打破的核心冲突】
%s

请推演输出严格合法的 JSON。`,
		projectTitle, targetPlatform, targetChapter,
		premiseBlock,
		typeGuidance,
		rhythmForesight,
		milestoneBlock,
		worldRules, volumeContext, powerContext,
		protNameLevel(protEntry), protInventory(protEntry), protGoal(protEntry), codexContext,
		rollingCanon, callbacksText,
		hooksSummary,
		coreConflict,
	)

	if horizon != nil && strings.TrimSpace(horizon.SelectedPlanText) != "" {
		planBlock := buildPlanPromptBlock(horizon.SelectedPlanText)
		userPrompt = strings.Replace(userPrompt, "请推演输出严格合法的 JSON。", planBlock+"请推演输出严格合法的 JSON。", 1)
	}

	if horizon != nil && horizon.ConsumedIntent != nil && horizon.ConsumedIntent.Status == domain.IntentStatusLocked {
		intentBlock := buildChapterIntentPromptBlock(horizon.ConsumedIntent)
		userPrompt = strings.Replace(userPrompt, "请推演输出严格合法的 JSON。", intentBlock+"请推演输出严格合法的 JSON。", 1)
	}

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, o.client, reasoningModel, systemPrompt, userPrompt, 0.4, nil)
	if err != nil {
		return nil, fmt.Errorf("derive beats LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}
	var out DeriveBeatsOutput
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, fmt.Errorf("parse beats JSON failed (raw: %s): %w", resp, err)
	}
	out.Usage = usage

	return &out, nil
}

// buildVolumeContextBlock 构建分卷卷纲注入块；arc 为 nil → ""。
// 若 targetChapter 落在某个分节内，追加该节（节标题/节目标/关键事件）。
func buildVolumeContextBlock(arc *domain.VolumeArc, targetChapter int) string {
	if arc == nil {
		return ""
	}

	var matched *domain.VolumeSection
	for i := range arc.Sections {
		sec := &arc.Sections[i]
		if targetChapter >= sec.FromChapter && targetChapter <= sec.ToChapter {
			matched = sec
			break
		}
	}

	if matched == nil {
		return fmt.Sprintf("\n【当前分卷主线任务】\n第 %d 卷：《%s》\n- 卷核心主线目标：%s\n- 卷终极大高潮：%s\n本章必须严格服务于本卷主线因果推进。\n",
			arc.VolumeIndex, arc.Title, arc.CoreGoal, arc.Climax)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n【当前分卷主线任务】\n第 %d 卷：《%s》\n- 卷核心主线目标：%s\n- 卷终极大高潮：%s\n",
		arc.VolumeIndex, arc.Title, arc.CoreGoal, arc.Climax)
	fmt.Fprintf(&b, "【本卷分节·本章所在节（第 %d-%d 章）：%s】\n- 节目标：%s\n",
		matched.FromChapter, matched.ToChapter, matched.Title, matched.Goal)
	if len(matched.KeyEvents) > 0 {
		fmt.Fprintf(&b, "- 关键事件：%s\n", strings.Join(matched.KeyEvents, "；"))
	}
	b.WriteString("本章必须严格服务于本卷主线因果推进与本节节目标。\n")
	return b.String()
}

// buildStoryPremiseBlock 立意+梗概注入块。project/framework 为空或无内容 → ""。
// 输出：【全书立意与梗概】\n立意：<theme_premise>\n梗概：<logline>（各自非空才输出该行）
func buildStoryPremiseBlock(project *domain.Project) string {
	if project == nil || project.Framework == nil {
		return ""
	}
	premise := strings.TrimSpace(project.Framework.ThemePremise)
	logline := strings.TrimSpace(project.Framework.Logline)
	if premise == "" && logline == "" {
		return ""
	}

	var lines []string
	lines = append(lines, "【全书立意与梗概】")
	if premise != "" {
		lines = append(lines, "立意："+premise)
	}
	if logline != "" {
		lines = append(lines, "梗概："+logline)
	}
	return strings.Join(lines, "\n")
}

// BuildMilestoneContextBlock 书级节拍定位块。无里程碑 → ""。
// 导出供提示词预览直接嵌入：预览与生产提示词逐字同源，禁止在预览侧手写第二份实现。
// 输出（多行）：
// 【全书宏观节拍定位】
// - 全书计划：M 章（本章为第 N 章，约 P%）
// - 本章里程碑：『名称』@第 N 章 —— 正文必须在本章兑现该里程碑事件与情绪高潮（仅 TargetChapter==N 时输出该行，否则输出：- 本章无里程碑锚点）
// - 最近未兑现里程碑：『名称』@第 T 章（PENDING，已越过）（target<N 且 PENDING 的最大者）
// - 下一里程碑：『名称』@第 T 章（间隔 K 章）（target>N 的最小者）
func BuildMilestoneContextBlock(project *domain.Project, chapterIndex int) string {
	if project == nil || project.Framework == nil || len(project.Framework.StoryMilestones) == 0 {
		return ""
	}

	var currMilestone *domain.StoryMilestone
	var recentOverdue *domain.StoryMilestone
	var nextMilestone *domain.StoryMilestone

	for i := range project.Framework.StoryMilestones {
		m := &project.Framework.StoryMilestones[i]
		if m.TargetChapter == chapterIndex {
			if currMilestone == nil {
				currMilestone = m
			}
		}
		if m.TargetChapter < chapterIndex {
			isPending := (m.Status == domain.MilestoneStatusPending || m.Status == "")
			if isPending {
				if recentOverdue == nil || m.TargetChapter > recentOverdue.TargetChapter {
					recentOverdue = m
				}
			}
		}
		if m.TargetChapter > chapterIndex {
			if nextMilestone == nil || m.TargetChapter < nextMilestone.TargetChapter {
				nextMilestone = m
			}
		}
	}

	var lines []string
	lines = append(lines, "【全书宏观节拍定位】")

	totalChapters := project.Framework.TotalPlannedChapters
	if totalChapters > 0 {
		pct := int(float64(chapterIndex) / float64(totalChapters) * 100)
		lines = append(lines, fmt.Sprintf("- 全书计划：%d 章（本章为第 %d 章，约 %d%%）", totalChapters, chapterIndex, pct))
	} else {
		lines = append(lines, fmt.Sprintf("- 全书计划：未定（本章为第 %d 章）", chapterIndex))
	}

	if currMilestone != nil {
		name := currMilestone.Name
		if strings.TrimSpace(name) == "" {
			name = currMilestone.Key
		}
		lines = append(lines, fmt.Sprintf("- 本章里程碑：『%s』@第 %d 章 —— 正文必须在本章兑现该里程碑事件与情绪高潮", name, chapterIndex))
	} else {
		lines = append(lines, "- 本章无里程碑锚点")
	}

	if recentOverdue != nil {
		name := recentOverdue.Name
		if strings.TrimSpace(name) == "" {
			name = recentOverdue.Key
		}
		lines = append(lines, fmt.Sprintf("- 最近未兑现里程碑：『%s』@第 %d 章（PENDING，已越过）", name, recentOverdue.TargetChapter))
	}

	if nextMilestone != nil {
		name := nextMilestone.Name
		if strings.TrimSpace(name) == "" {
			name = nextMilestone.Key
		}
		interval := nextMilestone.TargetChapter - chapterIndex
		lines = append(lines, fmt.Sprintf("- 下一里程碑：『%s』@第 %d 章（间隔 %d 章）", name, nextMilestone.TargetChapter, interval))
	}

	return strings.Join(lines, "\n")
}

func buildPlanPromptBlock(planText string) string {
	trimmed := strings.TrimSpace(planText)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("【作者选定的发展路线（剧情树候选计划，非正史）】\n以下路线仅用于对齐本章推进方向；严禁把其中内容当作既定事实直接落成正文事实；与正史、状态机、世界公理冲突时，一律以正史为准，并在其余节拍中体现你的调整与过渡：\n%s\n\n", trimmed)
}

func buildChapterIntentPromptBlock(intent *domain.ChapterIntent) string {
	if intent == nil {
		return ""
	}
	var lines []string
	lines = append(lines, "【本章意图卡（作者已锁定，节拍必须服务该意图）】")
	if strings.TrimSpace(intent.Conflict) != "" {
		lines = append(lines, fmt.Sprintf("核心冲突：%s", strings.TrimSpace(intent.Conflict)))
	}
	if strings.TrimSpace(intent.POV) != "" {
		lines = append(lines, fmt.Sprintf("POV 视角：%s", strings.TrimSpace(intent.POV)))
	}
	var curves []string
	for _, c := range intent.EmotionCurve {
		if s := strings.TrimSpace(c); s != "" {
			curves = append(curves, s)
		}
	}
	if len(curves) > 0 {
		lines = append(lines, fmt.Sprintf("情绪曲线：%s", strings.Join(curves, " → ")))
	}
	if intent.TensionTarget > 0 {
		lines = append(lines, fmt.Sprintf("张力目标：%d/10", intent.TensionTarget))
	}
	var plants []string
	for _, p := range intent.HookPlants {
		if s := strings.TrimSpace(p); s != "" {
			plants = append(plants, s)
		}
	}
	if len(plants) > 0 {
		lines = append(lines, fmt.Sprintf("埋设伏笔：%s", strings.Join(plants, "、")))
	}
	var payoffs []string
	for _, p := range intent.HookPayoffs {
		if s := strings.TrimSpace(p); s != "" {
			payoffs = append(payoffs, s)
		}
	}
	if len(payoffs) > 0 {
		lines = append(lines, fmt.Sprintf("回收伏笔：%s", strings.Join(payoffs, "、")))
	}
	if !isStateMutationEmpty(intent.ExpectedMutation) {
		if b, err := json.Marshal(intent.ExpectedMutation); err == nil {
			lines = append(lines, fmt.Sprintf("预期状态变迁（作者预期，仅供参考；严禁为迁就预期扭曲因果与人物行为，最终以写后结算为准）：%s", string(b)))
		}
	}
	lines = append(lines, "铁律：冲突不得偏离此意图；与正史、状态机、世界公理冲突时以正史为准，并在节拍中体现调整与过渡。")

	return strings.Join(lines, "\n") + "\n\n"
}
