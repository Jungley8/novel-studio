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

func (o *Orchestrator) SetClient(client LLMClient) {
	o.client = client
}

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
    "power_delta": "战力或修为变动描述",
    "character_mutations": [
      {
        "name": "配角或反派姓名",
        "status_delta": "该角色状态/心境/伤势/修为变迁，如：左臂被斩断，道心动摇",
        "relation_delta": "对主角或全局的关系变化，如：由轻蔑转为刻骨仇恨"
      }
    ]
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

【本章必须打破的核心冲突】
%s

请推演输出严格合法的 JSON。`,
		project.Title, project.TargetPlatform, horizon.TargetChapter,
		horizon.WorldRules, volumeContext, powerContext,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory, horizon.ProtagonistState.CoreGoal, codexContext,
		rollingCanon, callbacksText,
		hooksSummary,
		coreConflict,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.4)
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

func (o *Orchestrator) RenderScene(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	beats []domain.SceneBeat,
	wordsTarget int,
) (string, TokenUsage, error) {
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
) (string, TokenUsage, error) {
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

【去 AI 味与像人一样写作铁律】
1. 状语与副词克制法则（去AI味但必须保留自然健全语法）：
   - 中文能不用状语就不用状语，能不用副词就不用副词。严禁使用“修饰性副词+动词”（如：极其、猛然、冷冷地、迅速地、慢条斯理地、几不可察地、前所未有地）。彻底禁用“...地”修饰动词。
   - 画面感来自强动词和名词质感，绝不用副词给动词当拐杖（用“冲撞”代替“快速地跑”，用“抠案板”代替“紧张地抓着”）。
   - 【语法健全底线】：去AI味绝对不是剥离语法！严禁为了去状语而丢弃谓语或宾语，严禁写成残缺不全的电报式碎词短语。句子必须具备自然中文的主谓宾完整结构，读起来要像有血有肉的人类母语作家写出的小说，通顺、自然、有力，绝不能像电报机发出的短促字块。
2. 禁绝一切模板化书面比喻（喻词清零）：
   - 严禁出现“像...一样”、“如同...”、“宛若...”、“...般的”比喻套路（如：像干枯鹰爪、像钝刀刮铁、夜枭般的狞笑、如泥牛入海、如丧家之犬、目光如古井无波）。
   - 所有事物用客观物理材质与骨肉感直接呈现，严禁添加第二层廉价修辞。
3. 拒绝慢动作分解与微表情解剖：
   - 严禁描写“嘴角扯出弧度”、“眼角没有半分笑意”、“瞳孔缩成针尖”。
   - 严禁摄像头式机械分解动作（摘下斗笠 -> 搁在桌上 -> 桌面积灰 -> 指腹擦过 -> 留下白痕）。用人物主观焦点的粗粝动作直接交代。
4. 句长节奏（突发度）与自然呼吸（严禁电报式残疾断句）：
   - 维持长短句交错，平均句长 15-25 字。高潮与对峙可以用紧凑短句，但必须是主谓宾完整自然的人话（如：“顾渊没退。他身子前倾，脚下踏裂泥地，迎着恶臭撞进对方怀里。”）。
   - 【严禁电报残句】：严禁单字、单个短语独立成行成段（如严禁写成：“一滴。”、“砸桌。”、“响。”、“又一滴。”、“砸灰。”、“响。”、“咔。”、“烫。”）。
   - 【严禁游戏技能战报】：严禁在对话或正文中像游戏战斗日志一样生硬念出技能数值（如：“土神压顶咒。炼气三层，耗香火三缕。”、“荒火点灯式。残火一豆，耗香灰一捧。”）。功法耗损与天地法则必须化入身体受创、经脉剧痛或法力枯竭的沉浸式物理描写中。
   - 事件推进自然收束，严禁段尾空洞说教与总结升华。
5. 多人物独立声口与粗粝对话：
   - 人物对话必须像真人说话，带口语惯性与市井质感（“打听这干啥？”、“算哪门子账”），严禁打乒乓球式的书面文雅对答。
6. 严禁出现以下AI模式化废词：
   不由得、仿佛、宛若、嘴角勾起、眼神复杂、一时间、殊不知、与此同时、冷哼一声、倒吸一口凉气、暗自思忖、死寂一片、冷汗涔涔。`,
		tensionDirective,
		platformDirective,
		dialogueRatioDirective,
	)

	var anchorSection string
	if horizon.TailAnchor != "" {
		anchorSection = fmt.Sprintf("\n【上章收尾文风锚定 (最后200字，请严格承接此腔调与视角)】\n%s\n", horizon.TailAnchor)
	}

	var codexSection string
	if strings.TrimSpace(horizon.CodexContextText) != "" {
		codexSection = fmt.Sprintf("\n%s\n", horizon.CodexContextText)
	}

	beatsJSON, _ := json.MarshalIndent(beats, "", "  ")

	userPrompt := fmt.Sprintf(`【场景输入】
书名：《%s》
第 %d 章
目标字数：%d 字左右

【世界法则与公理】
%s

【主角状态参考】
%s | 携带物品：%s%s%s

【必须执行的场景节拍 (请严格按节拍顺序与因果展开)】
%s

请直接输出小说正文内容，无需任何开场白或寒暄。`,
		project.Title, chapterIndex, wordsTarget,
		horizon.WorldRules,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory,
		anchorSection, codexSection,
		string(beatsJSON),
	)

	ctxRole := ContextWithRole(ctx, RoleWriter)
	return o.client.ChatCompletionWithUsage(ctxRole, writerModel, systemPrompt, userPrompt, 0.75)
}

func (o *Orchestrator) ReviewDraft(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
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
		project.Protagonist.NameAndLevel, project.Protagonist.Inventory,
		string(beatsJSON),
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reviewerModel, systemPrompt, userPrompt, 0.3)
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
	}, usage, nil
}

func (o *Orchestrator) RewriteDraft(
	ctx context.Context,
	writerModel string,
	project *domain.Project,
	chapterIndex int,
	originalDraft string,
	review *domain.ReviewResult,
) (string, TokenUsage, error) {
	systemPrompt := `你是一名顶级网文精修专家。你的任务是根据主审的具体驳回意见，对原草稿进行定向精修与差分重构。
精修与去AI味铁律：
1. 严格修复主审指出的所有问题，逐条落实整改意见。
2. 保持自然中文语法健全：严禁写成残缺不全的电报式碎词短语。主语、谓语、宾语要完整自然，严禁单字单词独行（如“响。”、“烫。”、“骨响。咔。”）。
3. 状语与副词克制：能不用状语就不用状语，能不用副词就不用副词。彻底禁用“...地”修饰动词，用强动词和名词质感直接呈现动作，绝不用副词当拐杖。
4. 拔除书面比喻：删掉所有“像...”、“如...”、“宛若...”式书面比喻，用真实骨肉感与物理破坏直接呈现。
5. 拒绝微表情与慢动作：删掉“嘴角弧度”、“瞳孔针尖”、“眼角笑意”，动作保持粗粝主观聚焦。
6. 严禁游戏技能战报：绝不能像网游战报一样生硬报出技能名或念出消耗数值。
7. 【差分保护】集中火力解决病灶，直接输出精修重构后的完整正文，无需任何客套寒暄。`

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

	ctxRole := ContextWithRole(ctx, RoleWriter)
	return o.client.ChatCompletionWithUsage(ctxRole, writerModel, systemPrompt, userPrompt, 0.7)
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
			sb.WriteString(fmt.Sprintf("- 第%d拍(张力%d): 极高张力，动作紧凑，多用8-15字短促有力复合句，主谓宾结构完整自然，严禁将单字单个短语拆散独行成段；\n", i+1, b.Tension))
		case b.Tension >= 5:
			sb.WriteString(fmt.Sprintf("- 第%d拍(张力%d): 中速推进，节奏张弛有度，15-25字长短句交替，段落3-5行为宜；\n", i+1, b.Tension))
		default:
			sb.WriteString(fmt.Sprintf("- 第%d拍(张力%d): 沉浸铺陈，细腻环境与感官长句，叙述流畅舒展；\n", i+1, b.Tension))
		}
	}
	return strings.TrimSpace(sb.String())
}

// buildPlatformStylePrompt injects platform-specific writing style guidance.
func buildPlatformStylePrompt(platform string) string {
	switch {
	case strings.Contains(platform, "番茄"):
		return "番茄小说读者偏好：\n- 爽感优先，每 800 字至少一个小爽点（打脸/装逼/升级/认输）\n- 章末必须留一个\"明天一定要看\"级别的钩子\n- 段落适中，叙述利落紧凑"
	case strings.Contains(platform, "起点"):
		return "起点仙侠读者偏好：\n- 战力体系严谨自洽，将境界差距、法力耗损与功法代价自然融入肉身体感与战局攻防中，绝不能像游戏战报一样生硬报出数值或刻板念出技能名\n- 允许适度世界观展开描写（不超过本章 15%）\n- 章末可留悬念也可做小闭环"
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
) (*domain.ProjectFramework, TokenUsage, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, TokenUsage{}, errors.New("title cannot be empty")
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

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.5)
	if err != nil {
		return nil, usage, fmt.Errorf("bootstrap framework LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var fw domain.ProjectFramework
	if err := json.Unmarshal([]byte(cleanJSON), &fw); err != nil {
		return nil, usage, fmt.Errorf("parse framework JSON failed (raw: %s): %w", resp, err)
	}

	return &fw, usage, nil
}
