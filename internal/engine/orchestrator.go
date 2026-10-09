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
	if strings.TrimSpace(coreConflict) == "" && horizon != nil && horizon.Project != nil {
		if suggested, _, err := o.SuggestChapterConflict(ctx, reasoningModel, horizon); err == nil && strings.TrimSpace(suggested) != "" {
			coreConflict = strings.TrimSpace(suggested)
		} else {
			coreConflict = fmt.Sprintf("第 %d 章核心矛盾爆发与命运转折", horizon.TargetChapter)
		}
	}

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

	toneDirective := buildNarrativeTonePrompt(horizon.NarrativeTone)
	tensionDirective := buildTensionCurvePrompt(beats)
	if tensionDirective == "" {
		tensionDirective = "- 节奏平稳，长短交替推进"
	}
	platformDirective := buildPlatformStylePrompt(project.TargetPlatform)
	dialogueRatioDirective := buildDialogueRatioPrompt(beats)
	wordBudgetDirective := buildBeatWordBudgetPrompt(beats, wordsTarget)

	systemPrompt := fmt.Sprintf(`你是一名专业成熟、文笔深厚扎实的小说名家。

%s

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

【去 AI 味与自然中文写作铁律】
1. 生理肉身锚点与重力阻力（Somatic Grounding，彻底破除纯概念推演）：
   - 彻底拒绝心理学名词与抽象情绪总结（严禁写“心中泛起警惕”、“感到一阵绝望”、“生出一丝敬畏”）。
   - 情绪必须通过生理阻力传递：肌肉紧绷、重力沉降、冷汗浸湿衣领的黏腻感、喉头吞咽的酸苦、指关节发白的压迫。
2. 严禁翻案腔与假想敌（Anti-Contrarian / 严禁“不是A而是B”套路）：
   - 严禁出现“不是……而是……”、“看似……实则……”、“与其说……不如说……”、“你以为……其实……”、“不在于……而在于……”等虚立假靶子的翻案句式。
   - 直接从正面下判断或展开动作，不与虚构的假想敌缠斗。
3. 拒绝上帝视角动机交代（Anti-Explanatory Motives）：
   - 人物采取行动后，严禁紧跟“这是为了……”、“因为他深知……”等解释性补丁。让动作本身和环境后果说话。
4. 冰山非对称对话（Asymmetric Subtext）：
   - 对话严禁打乒乓球式的书面文雅顺承对答，人物心怀鬼胎，带打岔、口风、停顿、市井粗粝感与适度留白。
5. 动词优势法则，彻底拔除副词、名词化与翻译腔壳子：
   - 能不用状语就不用状语，能不用副词就不用副词。彻底禁用“...地”修饰动词。画面感来自强动词和名词质感，绝不用副词当拐杖。
   - 严禁公文式动名词膨胀（如“完成了对……的……”、“实现了……的提升”）。
   - 严禁机械翻译腔壳子（如前置“当……时，”、前置“对于……来说/而言，”、复述句“这意味着/这表明”）。
   - 【语法健全底线】：去AI味绝对不是剥离语法！严禁为了去状语而丢弃谓语或宾语，严禁写成残缺不全的电报式碎词短语。句子必须具备自然中文的主谓宾完整结构，读起来要像有血有肉的人类母语作家写出的小说，通顺、自然、有力，绝不能像电报机发出的短促字块。
6. 禁绝一切模板化书面比喻（喻词清零与拟人化工具喻体消除）：
   - 严禁出现“像...一样”、“如同...”、“宛若...”、“...般的”比喻套路（如：像干枯鹰爪、像钝刀刮铁、夜枭般的狞笑、如泥牛入海、如丧家之犬、目光如古井无波）。
   - 严禁把功法、工具、灵宝比作“像一位智慧的导师/管家/全能的助手”，所有事物用客观物理材质与骨肉感直接呈现。
7. 拒绝慢动作分解与微表情解剖：
   - 严禁描写“嘴角扯出弧度”、“眼角没有半分笑意”、“瞳孔缩成针尖”。
   - 严禁摄像头式机械分解动作（摘下斗笠 -> 搁在桌上 -> 桌面积灰 -> 指腹擦过 -> 留下白痕）。用人物主观焦点的粗粝动作直接交代。
8. 句长节奏（突发度）与自然呼吸（严禁电报式残疾断句）：
   - 维持长短句交错，平均句长 15-25 字。高潮与对峙可以用紧凑短句，但必须是主谓宾完整自然的人话（如：“顾渊没退。他身子前倾，脚下踏裂泥地，迎着恶臭撞进对方怀里。”）。
   - 【严禁电报残句】：严禁单字、单个短语独立成行成段（如严禁写成：“一滴。”、“砸桌。”、“响。”、“又一滴。”、“砸灰。”、“响。”、“咔。”、“烫。”）。
   - 【严禁游戏技能战报】：严禁在对话或正文中像游戏战斗日志一样生硬念出技能数值（如：“土神压顶咒。炼气三层，耗香火三缕。”）。功法耗损与天地法则必须化入身体受创、经脉剧痛或法力枯竭的沉浸式物理描写中。
   - 事件推进自然收束，严禁段尾空洞说教与总结升华。
9. 严禁出现以下AI模式化废词：
   不由得、仿佛、宛若、嘴角勾起、眼神复杂、一时间、殊不知、与此同时、冷哼一声、倒吸一口凉气、暗自思忖、死寂一片、冷汗涔涔、说白了、说穿了。`,
		toneDirective,
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

%s

【必须执行的场景节拍 (请严格按节拍顺序与因果展开)】
%s

请直接输出小说正文内容，无需任何开场白或寒暄。`,
		project.Title, chapterIndex, wordsTarget,
		horizon.WorldRules,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory,
		anchorSection, codexSection,
		wordBudgetDirective,
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
	return o.client.ChatCompletionWithUsage(ctxRole, writerModel, systemPrompt, userPrompt, 0.7)
}

// buildNarrativeTonePrompt returns tailored tone instructions for 5 common web novel narrative voices.
func buildNarrativeTonePrompt(tone string) string {
	switch tone {
	case "hardboiled", "冷峻白描":
		return `【叙事口吻：冷峻白描】
- 语言硬朗克制，以纯粹的物理动作、物体质感和冷冽的感官细节推进，绝不滥情；
- 拒绝任何无谓的感叹与悬浮修饰，字句像冰刀刻石，直击因果本质；
- 语法自然严整，主谓宾完整自然，杜绝矫揉造作。`
	case "high_tension", "热血张力":
		return `【叙事口吻：热血张力】
- 冲突爆发力极强，动作如暴雨倾泻，骨肉碰撞与气机逆乱的体感描写极度饱满；
- 节奏紧绷，在危机与逆袭间形成强烈压迫感，长句铺陈危机，短句雷霆反击；
- 严禁生硬念招式报数值，用实打实的肉身崩解与意志对决引爆爽点。`
	case "classical", "古典志怪":
		return `【叙事口吻：古典志怪】
- 浸润中式民俗与诡异志怪氛围，文字古朴苍凉、微带阴冷与宿命感；
- 渲染香火、阴司、符箓、残庙、泥胎木雕等真实民俗物态，笔调如青灯夜话；
- 句式洗练严谨，富有古典中文特有的气韵与张力。`
	case "vernacular", "市井烟火":
		return `【叙事口吻：市井烟火】
- 粗粝鲜活、地气充盈，充满底层江湖的生存智慧与人情世故；
- 人物对白粗粝带劲，夹杂方言与江湖俚语，动作麻利，市井百态跃然纸上；
- 叙述生动饱满，既有小人物的苟且机变，又有拔刀时的果决狠戾。`
	case "cinematic", "电影全景":
		return `【叙事口吻：电影全景】
- 极具景深与镜头调度感，全景扫视与特写微距无缝切换，画面感与光影质感极其丰富；
- 强调声效、空间景深与人物走位，群像交错有致，气象恢弘；
- 兼顾宏观史诗感与微观骨肉细节，如一部高水准中式史诗电影。`
	default:
		return `【叙事口吻：冷峻写实】
- 冷峻克制，富有电影镜头质感与物理物态张力；
- 拒绝浮夸情绪与AI套路，长短句错落起伏，语法健全自然。`
	}
}

// buildBeatWordBudgetPrompt enforces beat-level word budgets ensuring substantial chapter length.
func buildBeatWordBudgetPrompt(beats []domain.SceneBeat, wordsTarget int) string {
	if wordsTarget <= 0 {
		wordsTarget = 2000
	}
	numBeats := len(beats)
	if numBeats == 0 {
		return fmt.Sprintf("【全章字数与篇幅要求】\n- 本章总篇幅必须达到 %d 字以上，细节充实饱满，严禁草草带过或压缩梗概！", wordsTarget)
	}

	avgPerBeat := wordsTarget / numBeats
	if avgPerBeat < 400 {
		avgPerBeat = 400
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【全章字数与节拍篇幅预算 (至关重要！总字数必须达到 %d 字以上)】\n", wordsTarget))
	sb.WriteString("- 严禁将情节写成粗略的大纲梗概！每个节拍必须充分展开环境五感、动作推拉、心理机锋与细节交锋。\n")
	sb.WriteString(fmt.Sprintf("- 平均每个节拍预算：约 %d~%d 字，本章共 %d 个节拍，整章正文必须充实饱满，坚决达到 %d 字以上：\n", avgPerBeat, avgPerBeat+150, numBeats, wordsTarget))
	for i, b := range beats {
		phaseDesc := b.Phase
		if phaseDesc == "" {
			phaseDesc = fmt.Sprintf("第%d拍", i+1)
		}
		sb.WriteString(fmt.Sprintf("  * 节拍 %d（%s）：篇幅预算约 %d 字。深入展开细节，严禁两三句话匆忙了事。\n", i+1, phaseDesc, avgPerBeat))
	}
	return sb.String()
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
1. 核心立意与世界规则 (World Axioms)：提炼 3-5 条底层不可逆的世界运转公理与核心铁律（依据小说题材自适应：修仙之天道反噬/科幻之物理公理/悬疑之因果闭环/奇幻之魔法代价/都市之社会法则）。
2. 战力阶梯 (Power Ladder)：设计 6-9 个严密境界或实力层级，包含阶位序号(tier: 1..N)、破坏力表征、突破门槛与代价副作用（拒绝廉价数值堆砌）。
3. 核心势力 (Factions)：设计 3-4 个主要势力宗门或组织财阀，明确其立场、核心主张、独门手段与威胁级别。
4. 关键人物谱系 (Key Characters)：设计 3-5 位与主角命运交织的关键角色（引路导师、宿敌死仇、亦正亦邪同盟、远古残魂）。
5. 分卷宏观大纲 (Volume Arcs)：设计前 3-4 卷的大纲，每卷包含：卷序号(volume_index: 1..N)、卷名、卷主题、核心主线目标、终极大高潮情节、预估章数与核心回收伏笔。
6. 开局种子伏笔 (Seed Hooks)：设计 3-5 个开局前 3 章埋下的长线伏笔（包含目标回收章节 10-60 章）。

输出格式：必须且仅输出标准合法的纯 JSON 格式：
{
  "theme_premise": "核心主旨一句话描述",
  "world_axioms": ["世界规则1", "核心公理2", "底层铁律3"],
  "power_ladder": [
    {
      "tier": 1,
      "realm": "境界名或能力阶段名",
      "description": "破坏力与能力表征",
      "bottleneck": "突破门槛与关卡",
      "drawback": "突破代价或能力副作用"
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

	for i := range fw.PowerLadder {
		if fw.PowerLadder[i].Tier == 0 {
			fw.PowerLadder[i].Tier = i + 1
		}
	}
	for i := range fw.VolumeArcs {
		if fw.VolumeArcs[i].VolumeIndex == 0 {
			fw.VolumeArcs[i].VolumeIndex = i + 1
		}
	}

	return &fw, usage, nil
}

// CodexGenerateRequest encapsulates inputs for generating or expanding a codex entry.
type CodexGenerateRequest struct {
	Name     string               `json:"name"`
	Category domain.CodexCategory `json:"category"`
	Prompt   string               `json:"prompt"`
}

// GenerateCodexEntry prompts the reasoning engine to draft a rich, worldbuilding-consistent Codex entry.
func (o *Orchestrator) GenerateCodexEntry(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	req CodexGenerateRequest,
) (*domain.CodexEntry, TokenUsage, error) {
	category := req.Category
	if category == "" {
		category = domain.CategoryCharacter
	}

	systemPrompt := `你是一名网络小说白金级百科世界观架构师。
你的任务是根据世界观法则与作者灵感，为小说设定集构筑一个立体、严密、极具戏剧张力与辨识度的实体档案。
实体类别包括：CHARACTER(人物), LOCATION(地点), ITEM(法宝/道具), LORE(功法/规则), FACTION(门派/势力)。

必须以纯 JSON 格式输出，字段如下：
{
  "name": "实体名称",
  "aliases": ["别名或外号1", "外号2"],
  "category": "CHARACTER | LOCATION | ITEM | LORE | FACTION",
  "summary": "一句话核心定义（30字以内）",
  "details_markdown": "详细生平/渊源/物理表征/能力弱点的Markdown文本",
  "tracking_mode": "AUTO_MENTION",
  "archetype": "PROTAGONIST | ANTAGONIST | DEUTERAGONIST | MENTOR | SUPPORTING（仅人物有效，其他留空）",
  "voice_tone": "台词声口与语言特征（如：冷峻寡言，习惯以反问施压）",
  "core_motivation": "核心底层动机与死穴弱点",
  "current_disposition": "HOSTILE | WARY | NEUTRAL | FRIENDLY | DEVOTED（当前对主角立场）"
}`

	worldInfo := "无特定规则"
	if project != nil {
		worldInfo = fmt.Sprintf("书名：《%s》\n世界公理：%s", project.Title, project.WorldRules)
		if project.Framework != nil && project.Framework.ThemePremise != "" {
			worldInfo += fmt.Sprintf("\n全书主旨：%s", project.Framework.ThemePremise)
		}
	}

	userPrompt := fmt.Sprintf(`【设定背景】
%s

【生成需求】
目标分类：%s
实体名称：%s
设定灵感/要求：%s

请输出该实体的标准纯 JSON 档案。`,
		worldInfo, category, req.Name, req.Prompt,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.7)
	if err != nil {
		return nil, usage, fmt.Errorf("generate codex entry failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean codex json failed: %w", err)
	}

	var entry domain.CodexEntry
	if err := json.Unmarshal([]byte(cleanJSON), &entry); err != nil {
		return nil, usage, fmt.Errorf("unmarshal codex entry failed: %w", err)
	}

	if project != nil {
		entry.ProjectID = project.ID
	}
	if entry.Category == "" {
		entry.Category = category
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("codex-%d", time.Now().UnixNano())
	}
	if entry.TrackingMode == "" {
		entry.TrackingMode = domain.TrackingModeAutoMention
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	return &entry, usage, nil
}

// ExtractCodexRelations prompts the reasoning engine to deduce relationships between entities.
func (o *Orchestrator) ExtractCodexRelations(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	entries []*domain.CodexEntry,
	recentContext string,
) ([]domain.EntityRelation, TokenUsage, error) {
	if len(entries) < 2 {
		return nil, TokenUsage{}, errors.New("need at least 2 codex entries to infer relationships")
	}

	var entNames []string
	nameToID := make(map[string]string)
	for _, e := range entries {
		entNames = append(entNames, fmt.Sprintf("- %s (分类: %s, 概述: %s)", e.Name, e.Category, e.Summary))
		nameToID[e.Name] = e.ID
		for _, a := range e.Aliases {
			nameToID[a] = e.ID
		}
	}

	systemPrompt := `你是一名网络小说复杂关系网络分析专家。
你的任务是根据已有实体档案及剧情上下文，提炼实体之间的单向或双向关联关系。
关系类型包含但不限于：
NEMESIS(宿敌死仇), ALLY(生死同盟), MASTER_DISCIPLE(师徒教导), KINSHIP(血脉亲缘), CRUSH(爱慕道侣),
BELONGS_TO(宗门从属), POSSESSES(本命持有), LOCATED_IN(驻扎身处), OPPOSES(暗中对抗)。

必须且仅输出 JSON 数组，格式如下：
[
  {
    "source_name": "实体A名称",
    "target_name": "实体B名称",
    "relation_type": "NEMESIS | ALLY | ...",
    "description": "简要关系描述（20字以内）"
  }
]`

	userPrompt := fmt.Sprintf(`【实体池】
%s

【剧情参考上下文】
%s

请推演实体间切实存在的关联网络，并输出关系数组 JSON。`,
		strings.Join(entNames, "\n"), recentContext,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.4)
	if err != nil {
		return nil, usage, fmt.Errorf("extract codex relations failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean relations json failed: %w", err)
	}

	type rawRel struct {
		SourceName   string `json:"source_name"`
		TargetName   string `json:"target_name"`
		RelationType string `json:"relation_type"`
		Description  string `json:"description"`
	}

	var rawList []rawRel
	if err := json.Unmarshal([]byte(cleanJSON), &rawList); err != nil {
		return nil, usage, fmt.Errorf("unmarshal relations json failed: %w", err)
	}

	var rels []domain.EntityRelation
	projectID := ""
	if project != nil {
		projectID = project.ID
	}

	resolveID := func(name string) string {
		name = strings.TrimSpace(name)
		if name == "" {
			return ""
		}
		if id, ok := nameToID[name]; ok {
			return id
		}
		for _, e := range entries {
			if strings.Contains(e.Name, name) || strings.Contains(name, e.Name) {
				return e.ID
			}
			for _, a := range e.Aliases {
				if strings.Contains(a, name) || strings.Contains(name, a) {
					return e.ID
				}
			}
		}
		return ""
	}

	for i, r := range rawList {
		srcID := resolveID(r.SourceName)
		tgtID := resolveID(r.TargetName)
		if srcID == "" || tgtID == "" || srcID == tgtID {
			continue
		}
		rels = append(rels, domain.EntityRelation{
			ID:            fmt.Sprintf("rel-%d-%d", time.Now().UnixNano(), i+1),
			ProjectID:     projectID,
			SourceEntryID: srcID,
			TargetEntryID: tgtID,
			TargetName:    r.TargetName,
			RelationType:  r.RelationType,
			Description:   r.Description,
			CreatedAt:     time.Now(),
		})
	}

	return rels, usage, nil
}

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
	chapters []*domain.Chapter,
) (*domain.Protagonist, TokenUsage, error) {
	if project == nil {
		return nil, TokenUsage{}, errors.New("project cannot be nil")
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
		project.Protagonist.NameAndLevel,
		project.Protagonist.Inventory,
		project.Protagonist.CoreGoal,
		project.Protagonist.HealthStatus,
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

	updated := project.Protagonist
	if dto.NameAndLevel != "" {
		updated.NameAndLevel = dto.NameAndLevel
	}
	if dto.Inventory != "" {
		updated.Inventory = dto.Inventory
	}
	if dto.CoreGoal != "" {
		updated.CoreGoal = dto.CoreGoal
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
	}

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

请推演第 %d 章的核心冲突与事件爆点（纯文本直接输出）：`,
		project.Title, project.TargetPlatform, horizon.TargetChapter,
		horizon.WorldRules, volumeContext, powerContext,
		horizon.ProtagonistState.NameAndLevel, horizon.ProtagonistState.Inventory, horizon.ProtagonistState.CoreGoal, codexContext,
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
