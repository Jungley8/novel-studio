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
      "expectation_broken": "谁的心理预期被打破了",
      "reader_emotion": "期望读者此刻的情绪（如：紧张压抑、好奇期待、大呼解气、心疼揪心）",
      "info_gap": "信息差（角色知/读者不知，或读者知/角色不知）",
      "hook_type": "最后一拍必填钩子类型（CLIFFHANGER | REVERSAL | MYSTERY | POWER_UP，前3拍留空）"
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

	callbacksText := ""
	if strings.TrimSpace(horizon.HistoricalCallbacks) != "" {
		callbacksText = "\n\n" + horizon.HistoricalCallbacks
	}

	var volumeContext string
	if horizon.CurrentVolume != nil {
		volumeContext = fmt.Sprintf("\n【当前分卷主线任务】\n第 %d 卷：《%s》\n- 卷核心主线目标：%s\n- 卷终极大高潮：%s\n本章必须严格服务于本卷主线因果推进。\n",
			horizon.CurrentVolume.VolumeIndex, horizon.CurrentVolume.Title,
			horizon.CurrentVolume.CoreGoal, horizon.CurrentVolume.Climax)
	}

	var powerContext string
	if horizon.ActivePowerTier != nil {
		powerContext = fmt.Sprintf("\n【当前战力境界法则】\n- 境界：%s (%s)\n- 升级瓶颈：%s\n- 天道代价：%s\n",
			horizon.ActivePowerTier.Realm, horizon.ActivePowerTier.Description,
			horizon.ActivePowerTier.Bottleneck, horizon.ActivePowerTier.Drawback)
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
当前隐秘目标：%s

【前序正史视界 (最近 3 章密封剧情)】
%s%s

【当前开放状态的伏笔】
%s

【本章必须打破的核心冲突】
%s

请推演输出严格合法的 JSON。`,
		project.Title, project.TargetPlatform, horizon.TargetChapter,
		horizon.WorldRules, volumeContext, powerContext,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory, horizon.ProtagonistState.CoreGoal,
		rollingCanon, callbacksText,
		hooksSummary,
		coreConflict,
	)

	resp, err := o.client.ChatCompletion(ctx, reasoningModel, systemPrompt, userPrompt, 0.4)
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
	horizon := &CanonHorizon{
		Project:          project,
		TargetChapter:    chapterIndex,
		ProtagonistState: project.Protagonist,
		WorldRules:       project.WorldRules,
	}
	return o.RenderSceneWithHorizon(ctx, writerModel, horizon, beats, wordsTarget)
}

func (o *Orchestrator) RenderSceneWithHorizon(
	ctx context.Context,
	writerModel string,
	horizon *CanonHorizon,
	beats []domain.SceneBeat,
	wordsTarget int,
) (string, error) {
	if wordsTarget <= 0 {
		wordsTarget = 2000
	}
	project := horizon.Project
	chapterIndex := horizon.TargetChapter

	tensionDirective := buildTensionCurvePrompt(beats)
	if tensionDirective == "" {
		tensionDirective = "- 节奏平稳，长短交替推进"
	}
	platformDirective := buildPlatformStylePrompt(project.TargetPlatform)
	dialogueRatioDirective := buildDialogueRatioPrompt(beats)

	systemPrompt := fmt.Sprintf(`你是一名冷峻、极具电影镜头感的网络小说名家。

【声口约束】
- 每个角色的对话必须有独特口头禅或语言习惯，区分出年龄、阶层、修炼体系差异。
- 反派不许"哈哈哈"空洞大笑，必须有个性化的得意、阴鸷或蔑视表达。

【五感权重】
- 战斗场景：听觉 40%% + 触觉 30%% + 视觉 30%%，强调骨骼碎裂声、灵力震荡的皮肤刺痛与腥甜气息。
- 阴谋场景：嗅觉 30%% + 视觉 40%% + 听觉 30%%，强调汗液腥味、瞳孔微变与衣料摩挲声。
- 突破场景：触觉 50%% + 视觉 30%% + 听觉 20%%，强调经脉灼烧、丹田翻涌与脏腑雷鸣的体感。

【叙事视角与承前风格】
- 锁定第三人称限制视角，聚焦主角即时感知。
- 维持与全书一贯的叙述腔调、文字密度，严禁突然词藻华丽或过于口水化。

【节奏曲线指令 (基于当前场景节拍张力)】
%s

【目标平台调性】
%s

【对话与叙述比例】
%s

【写作铁律】
1. 严禁出现以下AI模式化废词：不由得、仿佛、宛若、嘴角勾起、眼神复杂、一时间、殊不知、与此同时、冷哼一声、倒吸一口凉气、暗自思忖。
2. 句长节奏（突发度）：战斗与对峙必须多用 3-6 字短句（可单句成段）；氛围烘托多用感官长句，长短句剧烈交替。
3. Show, don't tell：严禁直抒胸臆“他很愤怒”，必须通过微动作、肌肉紧绷、环境反馈体现。
4. 严格按照提供的节拍事实展开，不得擅自修改因果大纲。`,
		tensionDirective,
		platformDirective,
		dialogueRatioDirective,
	)

	var anchorSection string
	if horizon.TailAnchor != "" {
		anchorSection = fmt.Sprintf("\n【上章收尾文风锚定 (最后200字，请严格承接此腔调与视角)】\n%s\n", horizon.TailAnchor)
	}

	beatsJSON, _ := json.MarshalIndent(beats, "", "  ")

	userPrompt := fmt.Sprintf(`【场景输入】
书名：《%s》
第 %d 章
目标字数：%d 字左右

【世界法则与公理】
%s

【主角状态参考】
%s | 携带物品：%s%s

【必须执行的场景节拍 (请严格按节拍顺序与因果展开)】
%s

请直接输出小说正文内容，无需任何开场白或寒暄。`,
		project.Title, chapterIndex, wordsTarget,
		horizon.WorldRules,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory,
		anchorSection,
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
		Verdict:         out.Verdict,
		Score:           out.Score,
		Issues:          out.Issues,
		Suggestions:     out.Suggestions,
		ResolvedHookIDs: out.ResolvedHookIDs,
		ReviewedAt:      time.Now(),
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
	systemPrompt := `你是一名顶级网文精修专家。你的任务是根据主编审（Reviewer）的具体驳回意见，对原草稿进行定向精修与差分重构。
精修与写作铁律：
1. 严格修复主编指出的所有问题，逐条落实整改意见。
2. 【差分保护】严禁全盘推翻重写！对于主编没有提出异议的优秀段落和精彩描写，必须原样保留。
3. 修改比例严格控制在有问题的局部段落（修改内容原则上不得超过全文的 35%），集中火力解决病灶。
4. 严格遵循 Show, don't tell，杜绝AI套话，严禁出现不由得、仿佛、嘴角勾起等模式化废词。
5. 直接输出精修重构后的完整正文，无需任何客套寒暄。`

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

// buildTensionCurvePrompt dynamically generates rhythm density directives from beat tension values.
func buildTensionCurvePrompt(beats []domain.SceneBeat) string {
	if len(beats) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, b := range beats {
		switch {
		case b.Tension >= 8:
			sb.WriteString(fmt.Sprintf("- 第%d拍(张力%d): 极密节奏，3-6字短句连射，可单句成段，单段不超2行\n", i+1, b.Tension))
		case b.Tension >= 5:
			sb.WriteString(fmt.Sprintf("- 第%d拍(张力%d): 中速推进，长短交替，单段3-5行\n", i+1, b.Tension))
		default:
			sb.WriteString(fmt.Sprintf("- 第%d拍(张力%d): 舒缓蓄力，允许感官长句铺陈，单段5-8行\n", i+1, b.Tension))
		}
	}
	return strings.TrimSpace(sb.String())
}

// buildPlatformStylePrompt injects platform-specific writing style guidance.
func buildPlatformStylePrompt(platform string) string {
	switch {
	case strings.Contains(platform, "番茄"):
		return "番茄小说读者偏好：\n- 爽感优先，每 800 字至少一个小爽点（打脸/装逼/升级/认输）\n- 章末必须留一个\"明天一定要看\"级别的钩子\n- 段落极短（手机阅读），每段不超 3 行"
	case strings.Contains(platform, "起点"):
		return "起点仙侠读者偏好：\n- 战力体系严密，描写战斗时必须报出招式名与灵力层级消耗\n- 允许适度世界观展开描写（不超过本章 15%）\n- 章末可留悬念也可做小闭环"
	case strings.Contains(platform, "知乎"), strings.Contains(platform, "盐言"):
		return "知乎盐言读者偏好：\n- 文学性优先，句式高级感，拒绝网文口水话\n- 心理描写细腻深沉，意象隐喻取代直白叙述\n- 段落可长，允许散文化长句"
	default:
		return "通用网文风格：爽感与文学性兼顾，长短句交替，段落适中。"
	}
}

// buildDialogueRatioPrompt generates dialogue proportion guidance based on scene phases.
func buildDialogueRatioPrompt(beats []domain.SceneBeat) string {
	hasFight := false
	hasIntrigue := false
	for _, b := range beats {
		if b.Tension >= 8 {
			hasFight = true
		}
		if strings.Contains(b.Phase, "试探") || strings.Contains(b.Phase, "下套") || strings.Contains(b.Phase, "密谋") {
			hasIntrigue = true
		}
	}
	switch {
	case hasFight:
		return "本章以动作为主，对话占 15-25%，每段对话不超过 2 句。"
	case hasIntrigue:
		return "本章对话密集，占 40-55%，通过对话推进信息差和心理博弈。"
	default:
		return "本章对话占 30-40%，叙述与对话交替穿插。"
	}
}

func extractJSON(s string) string {
	if valid, err := ExtractAndCleanJSON(s); err == nil {
		return valid
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}

// FrameworkBootstrapRequest encapsulates inputs required to deduce a comprehensive macro framework.
type FrameworkBootstrapRequest struct {
	Title          string `json:"title"`
	TargetPlatform string `json:"target_platform"`
	CoreConcept    string `json:"core_concept"`
}

// BootstrapFramework prompts the reasoning engine to deduce the entire world bible,
// power ladder, factions, dramatis personae, volume arcs, and seed plot hooks.
func (o *Orchestrator) BootstrapFramework(
	ctx context.Context,
	reasoningModel string,
	req FrameworkBootstrapRequest,
) (*domain.ProjectFramework, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("title cannot be empty")
	}
	platform := req.TargetPlatform
	if strings.TrimSpace(platform) == "" {
		platform = "通用网文"
	}

	systemPrompt := `你是一名网络小说白金级架构总策划兼世界观架构大师。
你的任务是根据作者提供的书名、目标平台与核心灵感，推演并构建出一套宏大、自洽、严密且极具商业与文学张力的全书顶层架构总纲（Project Framework）。

推演法则：
1. 核心立意与世界公理 (World Axioms)：提炼 3-5 条底层不可逆的世界运转公理与天道真相（反乌托邦/克苏鲁/假仙真魔/因果宿命）。
2. 战力阶梯 (Power Ladder)：设计 6-9 个严密境界，详细定义每个境界的能力、突破瓶颈与反噬代价（拒绝廉价数值堆砌）。
3. 核心势力 (Factions)：设计 3-4 个主要势力宗门，明确其立场、核心主张、独门手段与威胁级别。
4. 关键人物谱系 (Key Characters)：设计 3-5 位与主角命运交织的关键角色（引路导师、宿敌死仇、亦正亦邪同盟、远古残魂）。
5. 分卷宏观大纲 (Volume Arcs)：设计前 3-4 卷的大纲，每卷包含：卷序号、卷名、卷主题、核心主线目标、终极大高潮情节、预估章数与核心回收伏笔。
6. 开局种子伏笔 (Seed Hooks)：设计 3-5 个开局前 3 章埋下的长线伏笔（包含目标回收章节 10-60 章）。

输出格式：必须且仅输出标准合法的纯 JSON 格式：
{
  "theme_premise": "核心主旨一句话描述",
  "world_axioms": ["世界公理1", "世界公理2", "天道残酷真相3"],
  "power_ladder": [
    {
      "realm": "境界名，如：练气期",
      "description": "境界能力特征",
      "bottleneck": "突破门槛与关卡",
      "drawback": "突破代价或天道反噬"
    }
  ],
  "factions": [
    {
      "name": "势力名称",
      "alignment": "阵营立场",
      "doctrine": "核心功法主张与手段",
      "threat_level": "威胁级别：中等/极高/灭顶之灾"
    }
  ],
  "key_characters": [
    {
      "name": "姓名",
      "role": "领路人/宿敌/同盟",
      "realm": "初始境界",
      "goal": "核心动机",
      "fate_arc": "宿命悲剧或终局"
    }
  ],
  "volume_arcs": [
    {
      "volume_index": 1,
      "title": "卷名",
      "theme": "卷主题",
      "core_goal": "本卷必须达成的核心目标",
      "climax": "本卷终极大高潮情节",
      "estimated_chapters": 40,
      "key_payoffs": ["本卷回收的伏笔1"]
    }
  ],
  "seed_hooks": [
    {
      "title": "伏笔标题",
      "details": "伏笔具体细节与暗线线索",
      "created_chapter": 1,
      "target_chapter": 20,
      "status": "OPEN"
    }
  ]
}`

	userPrompt := fmt.Sprintf(`【创世输入】
书名：《%s》
目标平台：%s
核心脑洞与题材灵感：%s

请推演并输出全书宏观创世总纲 JSON。`,
		req.Title, platform, req.CoreConcept,
	)

	resp, err := o.client.ChatCompletion(ctx, reasoningModel, systemPrompt, userPrompt, 0.5)
	if err != nil {
		return nil, fmt.Errorf("bootstrap framework LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var fw domain.ProjectFramework
	if err := json.Unmarshal([]byte(cleanJSON), &fw); err != nil {
		return nil, fmt.Errorf("parse framework JSON failed (raw: %s): %w", resp, err)
	}

	return &fw, nil
}
