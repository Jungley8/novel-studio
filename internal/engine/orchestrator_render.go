package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// buildRenderIntentDirective 构建 render userPrompt 插入的意图指令段（视角+情绪曲线）。
// nil / 非 LOCKED / 无内容 → 返回 ""。
func buildRenderIntentDirective(intent *domain.ChapterIntent) string {
	if intent == nil || intent.Status != domain.IntentStatusLocked {
		return ""
	}
	pov := strings.TrimSpace(intent.POV)
	var curves []string
	for _, c := range intent.EmotionCurve {
		if s := strings.TrimSpace(c); s != "" {
			curves = append(curves, s)
		}
	}
	if pov == "" && len(curves) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "【本章意图卡指令（作者已锁定）】")
	if pov != "" {
		lines = append(lines, fmt.Sprintf("视角：%s", pov))
	}
	if len(curves) > 0 {
		lines = append(lines, fmt.Sprintf("情绪曲线：%s", strings.Join(curves, " → ")))
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func isStateMutationEmpty(m domain.StateMutation) bool {
	return strings.TrimSpace(m.InventoryDelta) == "" &&
		strings.TrimSpace(m.PowerDelta) == "" &&
		len(m.CharacterMutations) == 0
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
		Project:       project,
		TargetChapter: chapterIndex,
		WorldRules:    project.WorldRules,
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
	return o.RenderSceneStream(ctx, writerModel, horizon, beats, wordsTarget, nil)
}

func (o *Orchestrator) RenderSceneStream(
	ctx context.Context,
	writerModel string,
	horizon *CanonHorizon,
	beats []domain.SceneBeat,
	wordsTarget int,
	onDelta func(string),
) (string, TokenUsage, error) {
	if wordsTarget <= 0 {
		wordsTarget = 2000
	}
	project := horizon.Project
	chapterIndex := horizon.TargetChapter

	toneDirective := buildNarrativeTonePrompt(horizon.NarrativeTone)
	atmosphereDirective := buildAtmosphereAnchorPrompt(project)
	tensionDirective := buildTensionCurvePrompt(beats)
	if tensionDirective == "" {
		tensionDirective = "- 节奏平稳，长短交替推进"
	}
	platformDirective := buildPlatformStylePrompt(project.TargetPlatform)
	dialogueRatioDirective := buildDialogueRatioPrompt(beats)
	wordBudgetDirective := buildBeatWordBudgetPrompt(beats, wordsTarget)

	systemPrompt := fmt.Sprintf(`你是一名专业成熟、文笔深厚扎实的小说名家。

%s

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
   - 维持长短句交错，平均句长 15-25 字。高潮与对峙可以用紧凑短句，但必须是主谓宾完整自然的人话（如：“他没退。身子前倾，脚下踏裂泥地，迎着恶臭撞进对方怀里。”）。
   - 【严禁电报残句】：严禁单字、单个短语独立成行成段（如严禁写成：“一滴。”、“砸桌。”、“响。”、“又一滴。”、“砸灰。”、“响。”、“咔。”、“烫。”）。
   - 【严禁游戏技能战报】：严禁在对话或正文中像游戏战斗日志一样生硬念出技能数值（如：“土神压顶咒。炼气三层，耗香火三缕。”）。功法耗损与天地法则必须化入身体受创、经脉剧痛或法力枯竭的沉浸式物理描写中。
   - 事件推进自然收束，严禁段尾空洞说教与总结升华。
9. 严禁出现以下AI模式化废词：
   不由得、仿佛、宛若、嘴角勾起、眼神复杂、一时间、殊不知、与此同时、冷哼一声、倒吸一口凉气、暗自思忖、死寂一片、冷汗涔涔、说白了、说穿了。`,
		toneDirective,
		atmosphereDirective,
		tensionDirective,
		platformDirective,
		dialogueRatioDirective,
	)

	var anchorSection string
	if horizon.StyleVaultAnchor != "" {
		anchorSection = fmt.Sprintf("\n【主文风锚定】\n%s\n", horizon.StyleVaultAnchor)
		if horizon.TailAnchor != "" {
			anchorSection += fmt.Sprintf("\n【辅·上章收尾文风锚定 (最后200字，请严格承接此腔调与视角)】\n%s\n", horizon.TailAnchor)
		}
	} else if horizon.TailAnchor != "" {
		anchorSection = fmt.Sprintf("\n【上章收尾文风锚定 (最后200字，请严格承接此腔调与视角)】\n%s\n", horizon.TailAnchor)
	}
	if motifSection := BuildMotifConceptPrompt(project); motifSection != "" {
		anchorSection += "\n" + motifSection + "\n"
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
		protNameLevel(horizon.ProtagonistEntry), protInventory(horizon.ProtagonistEntry),
		anchorSection, codexSection,
		wordBudgetDirective,
		string(beatsJSON),
	)

	if horizon != nil && horizon.ConsumedIntent != nil && horizon.ConsumedIntent.Status == domain.IntentStatusLocked {
		intent := horizon.ConsumedIntent
		pov := strings.TrimSpace(intent.POV)
		if pov != "" && pov != "第三人称限制视角" {
			povLine := fmt.Sprintf("锁定「%s」（作者意图卡指定），聚焦视角角色的即时感知。", pov)
			systemPrompt = strings.Replace(systemPrompt, "锁定第三人称限制视角，聚焦主角即时感知。", povLine, 1)
		}
	}

	if horizon != nil && horizon.ConsumedIntent != nil {
		if directive := buildRenderIntentDirective(horizon.ConsumedIntent); directive != "" {
			userPrompt = strings.Replace(userPrompt, "【必须执行的场景节拍", directive+"【必须执行的场景节拍", 1)
		}
	}

	ctxRole := ContextWithRole(ctx, RoleWriter)
	return SafeChatCompletionWithStreamFallback(ctxRole, o.client, writerModel, systemPrompt, userPrompt, 0.75, onDelta)
}

// BuildMotifConceptPrompt 有意母题与符号清单注入块（RC 精修锚点）。
// project/framework 为空或无有效母题 → 返回 ""（纯函数）。
// 导出供提示词预览直接嵌入：预览与生产提示词逐字同源，禁止在预览侧手写第二份实现。
func BuildMotifConceptPrompt(project *domain.Project) string {
	if project == nil || project.Framework == nil {
		return ""
	}
	var lines []string
	for _, m := range project.Framework.Motifs {
		if trimmed := strings.TrimSpace(m); trimmed != "" {
			lines = append(lines, "- "+trimmed)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "【有意母题与符号（RC 精修锚点）】\n" + strings.Join(lines, "\n") +
		"\n规则：在恰当时机自然复现以上母题，强化主题回响；严禁机械复读、标签化重复，严禁为复现母题破坏场景因果。"
}

// buildAtmosphereAnchorPrompt 全书氛围锚点指令。project/framework/锚点为空 → ""。
// 输出：【视觉与氛围锚点（全书恒定基调，严禁漂移）】\n<anchor>\n所有场景的环境、光影、色彩与声音渲染必须贴合此基调，严禁氛围漂移。
func buildAtmosphereAnchorPrompt(project *domain.Project) string {
	if project == nil || project.Framework == nil {
		return ""
	}
	anchor := strings.TrimSpace(project.Framework.AtmosphereAnchor)
	if anchor == "" {
		return ""
	}
	return fmt.Sprintf("【视觉与氛围锚点（全书恒定基调，严禁漂移）】\n%s\n所有场景的环境、光影、色彩与声音渲染必须贴合此基调，严禁氛围漂移。", anchor)
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
