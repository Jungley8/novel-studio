package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

// 生产段清单（施工前以读到的代码为准，本注释只记录结论，行号是路标不是真相）：
//
// 渲染提示词 assembly（internal/engine/orchestrator_render.go RenderSceneStream）：
//   - system：叙事口吻 buildNarrativeTonePrompt ← horizon.NarrativeTone；氛围
//     buildAtmosphereAnchorPrompt ← project.Framework；声口/五感/视角硬编码；
//     节奏 buildTensionCurvePrompt ← beats；平台 buildPlatformStylePrompt；
//     对话比 buildDialogueRatioPrompt ← beats；去AI味铁律硬编码。
//     LOCKED 意图卡可改写 POV 行。
//   - user：书名/章节/字数 + 世界法则 WorldRules（标题【世界法则与公理】
//     render.go:184）+ 主角【主角状态参考】`%s | 携带物品：%s`（render.go:187-188，
//     protNameLevel/protInventory，见 orchestrator_codex.go:226-238）
//     + anchorSection（StyleVaultAnchor 主锚 + TailAnchor 辅锚 +
//     BuildMotifConceptPrompt 母题）+ CodexContextText + wordBudget
//     buildBeatWordBudgetPrompt ← beats + beatsJSON；LOCKED 意图追加
//     buildRenderIntentDirective。
//   - 有母题块，无里程碑块，无 RollingCanon/HistoricalCallbacks/hooks，
//     无立意梗概/分卷/战力块。
//
// 节拍提示词 assembly（internal/engine/orchestrator_beats.go DeriveBeatsForChapterType）：
//   - system：章型定位 typeGuidance（5 章型之一）+ phaseGuidance + hookRule。
//   - user：作品信息 + premise buildStoryPremiseBlock + typeGuidance +
//     rhythmForesight（domain.PlanVolumeRhythm）+ milestone
//     BuildMilestoneContextBlock + 世界公理（标题【世界公理与不可违背法则】
//     beats.go:232；WorldRules + buildVolumeContextBlock
//     + ActivePowerTier 内联）+ 主角【主角当前状态机】三行（beats.go:235-238，
//     protNameLevel/protInventory/protGoal，见 orchestrator_codex.go:226-251）+
//     codex + RollingCanon + HistoricalCallbacks + hooksSummary（beats.go:138-154，
//     条目行 `- %s[%s] %s (目标回收: 第 %d 章)`半角括号 + 【🔥近期临期】前缀）+
//     coreConflict + plan buildPlanPromptBlock ← SelectedPlanText + intent
//     buildChapterIntentPromptBlock ← LOCKED ConsumedIntent。
//     内容源 fallback 链（beats.go:202-215）：title/platform/worldRules/protEntry
//     全部取自 horizon（horizon.Project.Title/TargetPlatform），无回退；
//     预览同样 horizon-first，p 仅作 horizon 缺失时的防御备份。
//   - 有里程碑块，无母题块/氛围块/文风锚。
//
// 预览复用原则：只嵌入生产侧已导出的块构造器（BuildMotifConceptPrompt、
// BuildMilestoneContextBlock、PreviewProtNameLevel/PreviewProtInventory/
// PreviewProtGoal 透传）或做只读展示拼接（raw 字段原文展示，不含任何
// 定位/清洗/匹配逻辑）；调不到生产块的只做原文展示并标注来源，绝不手写第
// 三份定位逻辑。为空则整块省略，不输出占位（主角块除外：生产 helper 自带
// 未知/无/暂无回退，恒有输出，预览逐字调用故同样恒有输出）。

// 预览双路受众标识。
const (
	PreviewAudienceRender = "render"
	PreviewAudienceBeats  = "beats"
)

type PromptPreviewRequest struct {
	ChapterIndex int      `json:"chapter_index"`
	SceneTitle   string   `json:"scene_title,omitempty"`
	POV          string   `json:"pov,omitempty"`
	DramaticGoal string   `json:"dramatic_goal,omitempty"`
	Conflict     string   `json:"conflict,omitempty"`
	CoreEvents   []string `json:"core_events,omitempty"`
	// 新调用方（PromptInspectorModal）透传的工作台输入：渲染路的节拍输入与
	// 篇幅、节拍路的核心冲突。老字段保留做向后兼容。
	CoreConflict string             `json:"core_conflict,omitempty"`
	Beats        []domain.SceneBeat `json:"beats,omitempty"`
	WordsTarget  int                `json:"words_target,omitempty"`
}

type PromptComponentPreview struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Content       string `json:"content"`
	TokenEstimate int    `json:"token_estimate"`
	Audience      string `json:"audience"`
}

// PromptAssemblyPreview 单路提示词装配镜像：render 或 beats 之一。
type PromptAssemblyPreview struct {
	Audience            string                   `json:"audience"`
	SystemPrompt        string                   `json:"system_prompt"`
	UserPrompt          string                   `json:"user_prompt"`
	Components          []PromptComponentPreview `json:"components"`
	TotalTokensEstimate int                      `json:"total_tokens_estimate"`
}

// PromptPreviewDiagnostics 预览诊断（非提示词）：启发式预检与评审记录。
// 评审门禁不属于任一路生产提示词，故独立于 render/beats 之外，避免误导排障。
type PromptPreviewDiagnostics struct {
	ReviewGate       PromptComponentPreview `json:"review_gate"`
	HeuristicSummary string                 `json:"heuristic_summary"`
	LastAuditSummary string                 `json:"last_audit_summary"`
}

type PromptPreviewResponse struct {
	Render      PromptAssemblyPreview    `json:"render"`
	Beats       PromptAssemblyPreview    `json:"beats"`
	Diagnostics PromptPreviewDiagnostics `json:"diagnostics"`
}

func estimateTokens(text string) int {
	return domain.EstimateTokens(text)
}

// previewMotifBlock 预览用母题块：直接嵌入生产注入块
// engine.BuildMotifConceptPrompt 的输出，与生产提示词逐字同源。
// 母题入库时已过 domain.SanitizeMotifs 清洗门（handlers_matrix.go），此处即生产注入原文。
// 无有效母题 → ""（整块省略，不输出占位）。
func previewMotifBlock(p *domain.Project) string {
	return engine.BuildMotifConceptPrompt(p)
}

// previewMilestoneBlock 预览用节拍定位块：直接嵌入生产定位块
// engine.BuildMilestoneContextBlock 的输出（逐字同源），另附全部里程碑一览
// （预览补充的 raw 数据展示，不含任何定位逻辑，便于肉眼对齐生产提示词）。
// 无里程碑 → ""（整块省略，不输出占位）。
func previewMilestoneBlock(p *domain.Project, chapterIndex int) string {
	block := engine.BuildMilestoneContextBlock(p, chapterIndex)
	if block == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(block)
	b.WriteString("\n【全部里程碑一览（预览补充）】")
	for _, m := range p.Framework.StoryMilestones {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = strings.TrimSpace(m.Key)
		}
		fmt.Fprintf(&b, "\n- 第%d章·%s·%s", m.TargetChapter, strings.TrimSpace(m.Key), name)
	}
	return b.String()
}

// previewAssemblyPart 装配段：content 为空则整块省略（不输出占位）。
type previewAssemblyPart struct {
	name    string
	desc    string
	content string
}

func newPromptAssembly(audience, systemPrompt string, parts []previewAssemblyPart) PromptAssemblyPreview {
	components := make([]PromptComponentPreview, 0, len(parts))
	var userParts []string
	total := 0
	for _, pt := range parts {
		if strings.TrimSpace(pt.content) == "" {
			continue
		}
		c := PromptComponentPreview{
			Name:          pt.name,
			Description:   pt.desc,
			Content:       pt.content,
			TokenEstimate: estimateTokens(pt.content),
			Audience:      audience,
		}
		components = append(components, c)
		userParts = append(userParts, pt.content)
		total += c.TokenEstimate
	}
	if components == nil {
		components = []PromptComponentPreview{}
	}
	return PromptAssemblyPreview{
		Audience:            audience,
		SystemPrompt:        systemPrompt,
		UserPrompt:          strings.Join(userParts, "\n\n---\n\n"),
		Components:          components,
		TotalTokensEstimate: total,
	}
}

// ---- 分路世界法则块（标题各从生产，内容源不变：horizon.WorldRules） ----

// previewRenderWorldRulesBlock 渲染路世界法则块，标题逐字取自生产
// orchestrator_render.go:184【世界法则与公理】。源空则整块省略。
func previewRenderWorldRulesBlock(worldRules string) string {
	if strings.TrimSpace(worldRules) == "" {
		return ""
	}
	return fmt.Sprintf("【世界法则与公理】\n%s", strings.TrimSpace(worldRules))
}

// previewBeatsWorldRulesBlock 节拍路世界公理块，标题逐字取自生产
// orchestrator_beats.go:232【世界公理与不可违背法则】。源空则整块省略。
// 生产同段另有 volumeContext + powerContext（本预览拆为独立组件）。
func previewBeatsWorldRulesBlock(worldRules string) string {
	if strings.TrimSpace(worldRules) == "" {
		return ""
	}
	return fmt.Sprintf("【世界公理与不可违背法则】\n%s", strings.TrimSpace(worldRules))
}

// ---- 分路主角块（逐字调用生产 helper，删掉三项并集手写实现） ----

// previewRenderProtagonistBlock 渲染路主角块，行文逐字对齐生产
// orchestrator_render.go:187-188：【主角状态参考】\n%s | 携带物品：%s。
// 两项均经 engine.PreviewProt* 透传生产 helper（含未知/无回退），故恒有输出。
func previewRenderProtagonistBlock(entry *domain.CodexEntry) string {
	return fmt.Sprintf("【主角状态参考】\n%s | 携带物品：%s",
		engine.PreviewProtNameLevel(entry), engine.PreviewProtInventory(entry))
}

// previewBeatsProtagonistBlock 节拍路主角块，行文逐字对齐生产
// orchestrator_beats.go:235-238：【主角当前状态机】\n姓名与等级：%s\n
// 随身物品栏：%s\n当前隐秘目标：%s。三项均经 engine.PreviewProt* 透传生产
// helper（含未知/无/暂无回退），故恒有输出。生产同段另有 codexContext
// （本预览拆为独立组件）。
func previewBeatsProtagonistBlock(entry *domain.CodexEntry) string {
	return fmt.Sprintf("【主角当前状态机】\n姓名与等级：%s\n随身物品栏：%s\n当前隐秘目标：%s",
		engine.PreviewProtNameLevel(entry), engine.PreviewProtInventory(entry), engine.PreviewProtGoal(entry))
}

// ---- 唯一共有块：全域百科（同一字符串两路复用，保证一致） ----

// previewSharedCodexBlock 全域百科原文展示（horizon.CodexContextText 自带
// 【出场世界观实体与时空状态】表头，预览不另加包装）。为空则整块省略。
func previewSharedCodexBlock(codexText string) string {
	if strings.TrimSpace(codexText) == "" {
		return ""
	}
	return strings.TrimSpace(codexText)
}

// ---- 渲染路独有块（只读展示；生产包装句见 orchestrator_render.go，不复写） ----

func previewRenderAtmosphereBlock(p *domain.Project) string {
	if p == nil || p.Framework == nil {
		return ""
	}
	anchor := strings.TrimSpace(p.Framework.AtmosphereAnchor)
	if anchor == "" {
		return ""
	}
	return "【视觉与氛围锚点】\n" + anchor +
		"\n注：生产渲染 system 另有固定包装句（见 orchestrator_render.go buildAtmosphereAnchorPrompt），预览仅展示锚点原文。"
}

// previewRenderStyleAnchors 文风锚定原文展示，顺序与生产 anchorSection 一致：
// 主锚 StyleVaultAnchor 在前，辅锚 TailAnchor 在后。为展示拼接，无定位逻辑。
func previewRenderStyleAnchors(styleVaultAnchor, tailAnchor string) string {
	var b strings.Builder
	if strings.TrimSpace(styleVaultAnchor) != "" {
		fmt.Fprintf(&b, "【主文风锚定】\n%s\n", strings.TrimSpace(styleVaultAnchor))
	}
	if strings.TrimSpace(tailAnchor) == "" {
		return strings.TrimSpace(b.String())
	}
	if b.Len() > 0 {
		b.WriteString("\n【辅·上章收尾文风锚定（最后200字，请严格承接此腔调与视角）】\n")
	} else {
		b.WriteString("【上章收尾文风锚定（最后200字，请严格承接此腔调与视角）】\n")
	}
	b.WriteString(strings.TrimSpace(tailAnchor))
	return b.String()
}

func previewRenderPlatformBlock(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return ""
	}
	return fmt.Sprintf("【目标平台调性】\n%s", strings.TrimSpace(platform))
}

// previewRenderSceneSpec 老请求字段的场景规格展示（SceneTitle/POV/DramaticGoal/
// Conflict/CoreEvents）。全空则省略。
func previewRenderSceneSpec(req PromptPreviewRequest) string {
	var sceneSpec strings.Builder
	sceneSpec.WriteString(fmt.Sprintf("【目标章节】第 %d 章\n", req.ChapterIndex))
	hasExtra := false
	if req.SceneTitle != "" {
		fmt.Fprintf(&sceneSpec, "【场景标题】%s\n", req.SceneTitle)
		hasExtra = true
	}
	if req.POV != "" {
		fmt.Fprintf(&sceneSpec, "【视角人物(POV)】%s\n", req.POV)
		hasExtra = true
	}
	if req.DramaticGoal != "" {
		fmt.Fprintf(&sceneSpec, "【戏剧目标】%s\n", req.DramaticGoal)
		hasExtra = true
	}
	if req.Conflict != "" {
		fmt.Fprintf(&sceneSpec, "【阻碍与冲突】%s\n", req.Conflict)
		hasExtra = true
	}
	if len(req.CoreEvents) > 0 {
		sceneSpec.WriteString("【核心事件节拍】\n")
		for i, ev := range req.CoreEvents {
			fmt.Fprintf(&sceneSpec, "  %d. %s\n", i+1, ev)
		}
		hasExtra = true
	}
	if !hasExtra {
		return ""
	}
	return strings.TrimRight(sceneSpec.String(), "\n")
}

// previewRenderBeatsInput 节拍输入原文展示（生产渲染消费 beats 推导节奏/对话比/
// 篇幅预算；预算推导为生产逻辑，预览仅展示输入原文与目标字数）。
func previewRenderBeatsInput(beats []domain.SceneBeat, wordsTarget int) string {
	if len(beats) == 0 && wordsTarget <= 0 {
		return ""
	}
	var b strings.Builder
	if wordsTarget > 0 {
		fmt.Fprintf(&b, "【目标字数】%d 字左右\n", wordsTarget)
	}
	if len(beats) == 0 {
		return strings.TrimRight(b.String(), "\n")
	}
	b.WriteString("【必须执行的场景节拍（输入原文，生产按此顺序与因果展开）】\n")
	beatsJSON, _ := json.MarshalIndent(beats, "", "  ")
	b.WriteString(string(beatsJSON))
	return b.String()
}

// ---- 节拍路独有块（只读展示；生产包装见 orchestrator_beats.go，不复写） ----

func previewBeatsWorkInfoBlock(title, platform string, chapterIndex int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "【作品信息】\n书名：《%s》\n目标平台：%s\n当前章节序号：第 %d 章",
		strings.TrimSpace(title), strings.TrimSpace(platform), chapterIndex)
	return b.String()
}

// previewBeatsPremiseBlock 立意梗概原文展示，行文格式与生产
// buildStoryPremiseBlock 一致（【全书立意与梗概】/立意：/梗概：），内容为字段
// 原文，无清洗逻辑。两项皆空则整块省略。
func previewBeatsPremiseBlock(p *domain.Project) string {
	if p == nil || p.Framework == nil {
		return ""
	}
	premise := strings.TrimSpace(p.Framework.ThemePremise)
	logline := strings.TrimSpace(p.Framework.Logline)
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

// previewBeatsVolumeBlock 分卷原文展示。所在节判定为生产逻辑
// （buildVolumeContextBlock），预览仅罗列分卷与全部分节原文，不做匹配判定。
func previewBeatsVolumeBlock(volume *domain.VolumeArc) string {
	if volume == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "【当前分卷主线任务】\n第 %d 卷：《%s》\n- 卷核心主线目标：%s\n- 卷终极大高潮：%s",
		volume.VolumeIndex, strings.TrimSpace(volume.Title),
		strings.TrimSpace(volume.CoreGoal), strings.TrimSpace(volume.Climax))
	if len(volume.Sections) > 0 {
		b.WriteString("\n【本卷分节一览（预览罗列原文，本章所在节判定见生产）】")
		for _, sec := range volume.Sections {
			fmt.Fprintf(&b, "\n- 第 %d-%d 章·%s·节目标：%s",
				sec.FromChapter, sec.ToChapter, strings.TrimSpace(sec.Title), strings.TrimSpace(sec.Goal))
			if len(sec.KeyEvents) > 0 {
				fmt.Fprintf(&b, "\n  关键事件：%s", strings.Join(sec.KeyEvents, "；"))
			}
		}
	}
	return b.String()
}

func previewBeatsPowerBlock(tier *domain.PowerLadderTier) string {
	if tier == nil {
		return ""
	}
	return fmt.Sprintf("【当前战力境界法则】\n- 境界：%s (%s)\n- 升级瓶颈：%s\n- 天道代价：%s",
		strings.TrimSpace(tier.Realm), strings.TrimSpace(tier.Description),
		strings.TrimSpace(tier.Bottleneck), strings.TrimSpace(tier.Drawback))
}

// previewBeatsCanonBlock 前序正史原文展示。渲染生产不消费 RollingCanon 与
// HistoricalCallbacks，故该块只属于节拍路。
func previewBeatsCanonBlock(rollingCanon, historicalCallbacks string) string {
	var b strings.Builder
	if strings.TrimSpace(rollingCanon) != "" {
		fmt.Fprintf(&b, "【前序正史视界】\n%s", strings.TrimSpace(rollingCanon))
	}
	if strings.TrimSpace(historicalCallbacks) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s", strings.TrimSpace(historicalCallbacks))
	}
	return b.String()
}

// previewBeatsHooksBlock 开放伏笔原文罗列，条目行格式逐字取自生产
// orchestrator_beats.go:151：`- %s[%s] %s (目标回收: 第 %d 章)`（半角括号）。
// 生产另有【🔥近期临期】前缀（beats.go:142-151，以 UrgentHooks ID 匹配判定，
// 内联于 DeriveBeatsForChapterType，预览不复写该判定逻辑，仅逐字镜像非临期
// 行；临期条目生产行首含前缀，预览行首无前缀，特此标注）。无伏笔则整块省略
// （生产缺省为 "无"，见 beats.go:138；预览循 C3 定式省略空块）。
func previewBeatsHooksBlock(hooks []*domain.PlotHook) string {
	var lines []string
	for _, h := range hooks {
		if h == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s (目标回收: 第 %d 章)",
			strings.TrimSpace(string(h.Status)), strings.TrimSpace(h.Title), h.TargetChapter))
	}
	if len(lines) == 0 {
		return ""
	}
	return "【当前开放状态的伏笔】\n" + strings.Join(lines, "\n")
}

// previewBeatsConflictBlock 核心冲突输入原文展示。生产在空冲突时有
// SuggestChapterConflict 回退（LLM 推演），预览绝不复写该回退，仅展示输入。
func previewBeatsConflictBlock(coreConflict, legacyConflict string) string {
	conflict := strings.TrimSpace(coreConflict)
	if conflict == "" {
		conflict = strings.TrimSpace(legacyConflict)
	}
	if conflict == "" {
		return ""
	}
	return "【本章必须打破的核心冲突】\n" + conflict
}

// previewBeatsPlanIntentBlock 作者路线与意图卡原文展示（store 只读）。
// 生产包装（buildPlanPromptBlock / buildChapterIntentPromptBlock 的固定行文）
// 为生产常量，预览仅展示原文。两项皆空则整块省略。
func previewBeatsPlanIntentBlock(planText string, intent *domain.ChapterIntent) string {
	var b strings.Builder
	if strings.TrimSpace(planText) != "" {
		fmt.Fprintf(&b, "【作者选定的发展路线（剧情树候选计划原文）】\n%s", strings.TrimSpace(planText))
	}
	if intent != nil && intent.Status == domain.IntentStatusLocked {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("【本章意图卡（作者已锁定，原文）】")
		if strings.TrimSpace(intent.Conflict) != "" {
			fmt.Fprintf(&b, "\n核心冲突：%s", strings.TrimSpace(intent.Conflict))
		}
		if strings.TrimSpace(intent.POV) != "" {
			fmt.Fprintf(&b, "\nPOV 视角：%s", strings.TrimSpace(intent.POV))
		}
		var curves []string
		for _, c := range intent.EmotionCurve {
			if s := strings.TrimSpace(c); s != "" {
				curves = append(curves, s)
			}
		}
		if len(curves) > 0 {
			fmt.Fprintf(&b, "\n情绪曲线：%s", strings.Join(curves, " → "))
		}
		if intent.TensionTarget > 0 {
			fmt.Fprintf(&b, "\n张力目标：%d/10", intent.TensionTarget)
		}
	}
	return b.String()
}

const renderSystemPrompt = `你是一名专业成熟、文笔深厚扎实的小说名家。

（生产渲染 system 全文另含声口约束、五感权重、节奏曲线、平台调性、对话比例与去AI味铁律等生产常量段，均为生产硬编码，预览不复写；叙事口吻与氛围锚点见 render 组件原文。详见 orchestrator_render.go RenderSceneStream）`

const beatsSystemPrompt = `你是一名网络小说架构与读者心理学总设计师。你严禁输出抒情散文。

（生产节拍 system 另含本章节奏定位 typeGuidance 与节拍 JSON schema；章型判定为生产运行时逻辑，预览不预判。详见 orchestrator_beats.go DeriveBeatsForChapterType）`

func buildRenderAssembly(p *domain.Project, horizon *engine.CanonHorizon, req PromptPreviewRequest) PromptAssemblyPreview {
	// 内容源 fallback 链：渲染生产 project := horizon.Project（render.go:82），
	// 预览同样 horizon-first，p 仅作 horizon 缺失时的防御备份。
	proj := p
	var worldRules, styleVault, tailAnchor, codexText string
	var protEntry *domain.CodexEntry
	if horizon != nil {
		worldRules = horizon.WorldRules
		styleVault = horizon.StyleVaultAnchor
		tailAnchor = horizon.TailAnchor
		codexText = horizon.CodexContextText
		protEntry = horizon.ProtagonistEntry
		if horizon.Project != nil {
			proj = horizon.Project
		}
	}
	if strings.TrimSpace(worldRules) == "" && proj != nil {
		worldRules = proj.WorldRules
	}
	var platform string
	if proj != nil {
		platform = proj.TargetPlatform
	}
	parts := []previewAssemblyPart{
		{"世界法则与公理 (Render)", "渲染提示词 user 段：世界法则原文，标题取自生产 render.go:184", previewRenderWorldRulesBlock(worldRules)},
		{"目标平台调性 (Render)", "渲染提示词 system 段来源：平台原文（生产包装见 buildPlatformStylePrompt）", previewRenderPlatformBlock(platform)},
		{"视觉与氛围锚点 (Render)", "渲染提示词 system 段来源：氛围锚点原文，渲染路独有", previewRenderAtmosphereBlock(proj)},
		{"有意母题与符号 (Render)", "渲染提示词注入块：生产块逐字同源，渲染路独有", previewMotifBlock(proj)},
		{"文风锚定 (Render)", "渲染提示词 user 段：主文风锚 + 上章收尾腔调锚原文，渲染路独有", previewRenderStyleAnchors(styleVault, tailAnchor)},
		{"主角状态参考 (Render)", "渲染提示词 user 段：逐字对齐生产 render.go:187-188（透传生产 helper）", previewRenderProtagonistBlock(protEntry)},
		{"全域百科与关联 (Render)", "渲染提示词 user 段：Codex 原文（与节拍路共有块一致），为空整块省略", previewSharedCodexBlock(codexText)},
		{"场景戏剧规格 (Render)", "渲染场景输入：老请求字段展示，全空则省略", previewRenderSceneSpec(req)},
		{"节拍输入与篇幅 (Render)", "渲染提示词 user 段输入：节拍 JSON 原文与目标字数", previewRenderBeatsInput(req.Beats, req.WordsTarget)},
	}
	return newPromptAssembly(PreviewAudienceRender, renderSystemPrompt, parts)
}

func buildBeatsAssembly(p *domain.Project, horizon *engine.CanonHorizon, req PromptPreviewRequest, planText string, intent *domain.ChapterIntent) PromptAssemblyPreview {
	// 内容源 fallback 链对齐生产 beats.go:202-215：title/platform/worldRules/
	// protEntry 全部取自 horizon（title/platform 只取 horizon.Project），生产无
	// 回退；预览同样 horizon-first，p 仅作 horizon 缺失时的防御备份。
	// premise/milestone 生产取 horizon.Project（beats.go:219-221），故同样用 proj。
	proj := p
	var worldRules, rollingCanon, callbacks, codexText string
	var volume *domain.VolumeArc
	var powerTier *domain.PowerLadderTier
	var hooks []*domain.PlotHook
	var protEntry *domain.CodexEntry
	if horizon != nil {
		worldRules = horizon.WorldRules
		volume = horizon.CurrentVolume
		powerTier = horizon.ActivePowerTier
		rollingCanon = horizon.RollingCanonText
		callbacks = horizon.HistoricalCallbacks
		hooks = horizon.AllActiveHooks
		codexText = horizon.CodexContextText
		protEntry = horizon.ProtagonistEntry
		if horizon.Project != nil {
			proj = horizon.Project
		}
	}
	if strings.TrimSpace(worldRules) == "" && proj != nil {
		worldRules = proj.WorldRules
	}
	var title, platform string
	if proj != nil {
		title = proj.Title
		platform = proj.TargetPlatform
	}
	parts := []previewAssemblyPart{
		{"作品信息 (Beats)", "节拍提示词 user 段：书名/平台/章节原文", previewBeatsWorkInfoBlock(title, platform, req.ChapterIndex)},
		{"全书立意与梗概 (Beats)", "节拍提示词 user 段：立意梗概原文，节拍路独有", previewBeatsPremiseBlock(proj)},
		{"全书宏观节拍定位 (Beats)", "节拍提示词 user 段：生产定位块逐字同源 + 里程碑一览补充，节拍路独有", previewMilestoneBlock(proj, req.ChapterIndex)},
		{"分卷主线任务 (Beats)", "节拍提示词 user 段：分卷原文罗列，节拍路独有", previewBeatsVolumeBlock(volume)},
		{"战力境界法则 (Beats)", "节拍提示词 user 段：当前境界原文，节拍路独有", previewBeatsPowerBlock(powerTier)},
		{"世界公理 (Beats)", "节拍提示词 user 段：世界公理原文，标题取自生产 beats.go:232", previewBeatsWorldRulesBlock(worldRules)},
		{"主角状态机 (Beats)", "节拍提示词 user 段：逐字对齐生产 beats.go:235-238（透传生产 helper）", previewBeatsProtagonistBlock(protEntry)},
		{"全域百科与关联 (Beats)", "节拍提示词 user 段：Codex 原文（与渲染路共有块一致），为空整块省略", previewSharedCodexBlock(codexText)},
		{"前序正史视界 (Beats)", "节拍提示词 user 段：滚动正史与历史回调原文，节拍路独有", previewBeatsCanonBlock(rollingCanon, callbacks)},
		{"开放伏笔 (Beats)", "节拍提示词 user 段：伏笔台账原文罗列，节拍路独有", previewBeatsHooksBlock(hooks)},
		{"核心冲突 (Beats)", "节拍提示词 user 段输入：冲突原文（生产空回退不复写）", previewBeatsConflictBlock(req.CoreConflict, req.Conflict)},
		{"作者路线与意图卡 (Beats)", "节拍提示词 user 段输入：计划与锁定意图卡原文", previewBeatsPlanIntentBlock(planText, intent)},
	}
	return newPromptAssembly(PreviewAudienceBeats, beatsSystemPrompt, parts)
}

// buildReviewGateContent 评审门禁诊断内容（预览诊断，非任一路生产提示词）。
// 原 handler 内联逻辑整体搬迁至此，行为不变。
func (s *Server) buildReviewGateContent(r *http.Request, projectID string, horizonContext string, chapterIndex int) (reviewGate, heuristicSummary, lastAuditSummary string) {
	var linterRes domain.LinterResult
	if s.linter != nil {
		linterRes = s.linter.Analyze(horizonContext)
	} else if s.qualityGate != nil {
		linterRes = s.qualityGate.FastAnalyze(horizonContext)
	}

	if strings.TrimSpace(horizonContext) == "" {
		heuristicSummary = "前序上下文为空，本章暂无启发式预检数据"
	} else {
		var details []string
		details = append(details, fmt.Sprintf("突发度: %d/100", linterRes.BurstinessScore))
		if len(linterRes.HitBannedWords) > 0 {
			details = append(details, fmt.Sprintf("命中毒词套词(%d处): %s", len(linterRes.HitBannedWords), strings.Join(linterRes.HitBannedWords, ", ")))
		} else {
			details = append(details, "零违禁套词")
		}
		details = append(details, fmt.Sprintf("感叹号密度: %.1f/千字", linterRes.ExclamationDensity))
		if linterRes.DialogueRatio > 0 {
			details = append(details, fmt.Sprintf("对话占比: %.1f%%", linterRes.DialogueRatio*100))
		}
		if len(linterRes.TopRepeatedNgrams) > 0 {
			details = append(details, fmt.Sprintf("高频重复短语: %s", strings.Join(linterRes.TopRepeatedNgrams, ", ")))
		}
		if len(linterRes.EmpiricalTells) > 0 {
			details = append(details, fmt.Sprintf("AI腔调特征: %s", strings.Join(linterRes.EmpiricalTells, ", ")))
		}
		status := "PASS"
		if !linterRes.Passed {
			status = "WARN"
		}
		heuristicSummary = fmt.Sprintf("[%s] %s", status, strings.Join(details, "；"))
	}

	ctx := r.Context()
	cp, err := s.store.GetCheckpoint(ctx, projectID, chapterIndex)
	if err == nil && cp != nil && cp.AuditReport != nil {
		rep := cp.AuditReport
		var issuesText string
		if len(rep.Issues) > 0 {
			top := rep.Issues
			if len(top) > 3 {
				top = top[:3]
			}
			var topLines []string
			for i, iss := range top {
				topLines = append(topLines, fmt.Sprintf("%d. %s", i+1, iss))
			}
			issuesText = fmt.Sprintf("\nTop 3 待改问题:\n  %s", strings.Join(topLines, "\n  "))
		} else {
			issuesText = "\nTop 3 待改问题: 无"
		}
		lastAuditSummary = fmt.Sprintf("评级: %s | 评分: %d%s", rep.Verdict, rep.Score, issuesText)
	} else {
		lastAuditSummary = "本章暂无评审记录"
	}

	var reviewGateContent strings.Builder
	reviewGateContent.WriteString("【Fail-Closed 放行规则】\n")
	reviewGateContent.WriteString("ACCEPTED && ≥80 && 零套词 && 感叹号不超标；任一分庭 REVISION 即驳回；耗尽返工停线待人工\n\n")
	reviewGateContent.WriteString("【本章启发式预检摘要】\n")
	reviewGateContent.WriteString(heuristicSummary)
	reviewGateContent.WriteString("\n\n")
	reviewGateContent.WriteString("【历史评审记录】\n")
	reviewGateContent.WriteString(lastAuditSummary)
	return reviewGateContent.String(), heuristicSummary, lastAuditSummary
}

func (s *Server) handlePromptPreview(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PromptPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.ChapterIndex <= 0 {
		req.ChapterIndex = 1
	}

	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil || p == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	horizon, err := s.chronicle.AssembleHorizon(ctx, projectID, req.ChapterIndex)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "assemble horizon: "+err.Error())
		return
	}

	// 只读装载作者路线与意图卡（store 直读；缺失/失败即省略对应块，不阻断预览）。
	var planText string
	if ps, ok := s.store.(interface {
		GetActivePlan(ctx context.Context, projectID string) (*domain.ActivePlan, error)
	}); ok {
		if plan, pErr := ps.GetActivePlan(ctx, projectID); pErr == nil && plan != nil &&
			plan.Status == domain.PlanStatusActive && strings.TrimSpace(plan.PlanText) != "" {
			planText = plan.PlanText
		}
	}
	var intent *domain.ChapterIntent
	if got, iErr := s.store.GetChapterIntent(ctx, projectID, req.ChapterIndex); iErr == nil && got != nil &&
		got.Status == domain.IntentStatusLocked {
		intent = got
	}

	renderAssembly := buildRenderAssembly(p, horizon, req)
	beatsAssembly := buildBeatsAssembly(p, horizon, req, planText, intent)

	horizonContext := horizon.RollingCanonText
	if horizon.TailAnchor != "" {
		if horizonContext != "" {
			horizonContext += "\n" + horizon.TailAnchor
		} else {
			horizonContext = horizon.TailAnchor
		}
	}
	reviewGate, heuristicSummary, lastAuditSummary := s.buildReviewGateContent(r, projectID, horizonContext, req.ChapterIndex)

	jsonResponse(w, http.StatusOK, PromptPreviewResponse{
		Render: renderAssembly,
		Beats:  beatsAssembly,
		Diagnostics: PromptPreviewDiagnostics{
			ReviewGate: PromptComponentPreview{
				Name:          "评审门禁 (预览诊断)",
				Description:   "Fail-Closed 放行规则、本章启发式预检与历史评审记录；非生产提示词",
				Content:       reviewGate,
				TokenEstimate: estimateTokens(reviewGate),
				Audience:      "",
			},
			HeuristicSummary: heuristicSummary,
			LastAuditSummary: lastAuditSummary,
		},
	})
}
