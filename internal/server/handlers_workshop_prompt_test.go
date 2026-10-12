package server

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

// 预览块必须逐字嵌入生产块：preview*Block 若被改回手写拼接（表头/行文与生产块
// 不一致），本测试变红。定位逻辑的唯一事实来源是 engine 包，server 侧禁止第二份实现。
func TestPreviewBlocksEmbedProductionBlocks(t *testing.T) {
	p := &domain.Project{Framework: &domain.ProjectFramework{
		Motifs: []string{"  血色月亮  ", "", "雾中低语"},
		StoryMilestones: []domain.StoryMilestone{
			{Key: "OPENING_HOOK", Name: "  开局钩子  ", TargetChapter: 1, Status: domain.MilestoneStatusDone},
			{Key: "MIDPOINT", Name: "中点", TargetChapter: 15, Status: domain.MilestoneStatusPending},
			{Key: "FINALE", Name: "", TargetChapter: 30, Status: domain.MilestoneStatusPending},
		},
		TotalPlannedChapters: 30,
	}}

	motifBlock := previewMotifBlock(p)
	if !strings.Contains(motifBlock, "【有意母题与符号（RC 精修锚点）】") {
		t.Errorf("motif preview lost production header, got:\n%s", motifBlock)
	}
	if !strings.Contains(motifBlock, "- 血色月亮") || !strings.Contains(motifBlock, "- 雾中低语") {
		t.Errorf("motif preview lost production entries, got:\n%s", motifBlock)
	}

	msBlock := previewMilestoneBlock(p, 10)
	if !strings.Contains(msBlock, "【全书宏观节拍定位】") {
		t.Errorf("milestone preview lost production header, got:\n%s", msBlock)
	}
	if !strings.Contains(msBlock, "『中点』@第 15 章") {
		t.Errorf("milestone preview lost production next-milestone line, got:\n%s", msBlock)
	}
	if !strings.Contains(msBlock, "【全部里程碑一览（预览补充）】") {
		t.Errorf("milestone preview lost appendix list, got:\n%s", msBlock)
	}

	if got := previewMotifBlock(nil); got != "" {
		t.Errorf("nil project motif preview should be empty, got %q", got)
	}
	if got := previewMilestoneBlock(nil, 1); got != "" {
		t.Errorf("nil project milestone preview should be empty, got %q", got)
	}
}

// 双路预览互斥 guard：render 路不得含里程碑表头【全书宏观节拍定位】，
// beats 路不得含母题表头【有意母题与符号】。两路共有块（世界法则/主角/百科）
// 必须字符串一致。若生产 assembly 未来让某块两路共有，应改断言为共有块一致。
func TestPreviewAssembliesMutualExclusion(t *testing.T) {
	p := &domain.Project{
		ID:             "p-preview-split",
		Title:          "测试书",
		TargetPlatform: "番茄小说",
		WorldRules:     "世界法则第一条",
		Framework: &domain.ProjectFramework{
			ThemePremise:     "复仇与救赎",
			Logline:          "少年仗剑复仇",
			AtmosphereAnchor: "阴冷民俗废土",
			Motifs:           []string{"血色月亮"},
			StoryMilestones: []domain.StoryMilestone{
				{Key: "MIDPOINT", Name: "中点", TargetChapter: 15, Status: domain.MilestoneStatusPending},
			},
			TotalPlannedChapters: 30,
		},
	}
	horizon := &engine.CanonHorizon{
		Project:          p,
		TargetChapter:    10,
		WorldRules:       p.WorldRules,
		RollingCanonText: "前两章小结",
		TailAnchor:       "尾锚二百字",
		StyleVaultAnchor: "主锚文风契约",
		CodexContextText: "【出场世界观实体与时空状态】\n• [角色] 主角",
		ProtagonistEntry: &domain.CodexEntry{Name: "主角", Realm: "练气九层", Inventory: "古剑", Want: "复仇"},
		CurrentVolume:    &domain.VolumeArc{VolumeIndex: 1, Title: "开卷", CoreGoal: "入局", Climax: "反杀"},
		ActivePowerTier:  &domain.PowerLadderTier{Realm: "练气", Description: "引气入体", Bottleneck: "心魔", Drawback: "经脉灼痛"},
		AllActiveHooks:   []*domain.PlotHook{{ID: "h1", Title: "玉佩之谜", Status: domain.HookStatusOpen, TargetChapter: 12}},
	}
	req := PromptPreviewRequest{
		ChapterIndex: 10,
		CoreConflict: "本章核心冲突",
		Beats:        []domain.SceneBeat{{Phase: "蓄力压迫", Tension: 5, Action: "夜探祠堂"}},
		WordsTarget:  2000,
	}
	intent := &domain.ChapterIntent{
		ProjectID: "p-preview-split", ChapterIndex: 10,
		Conflict: "意图冲突", POV: "主角", EmotionCurve: []string{"压抑", "爆发"},
		TensionTarget: 7, Status: domain.IntentStatusLocked,
	}

	render := buildRenderAssembly(p, horizon, req)
	beats := buildBeatsAssembly(p, horizon, req, "作者路线原文", intent)

	joinContents := func(a PromptAssemblyPreview) string {
		var b strings.Builder
		b.WriteString(a.UserPrompt)
		for _, c := range a.Components {
			b.WriteString("\n" + c.Content)
			if c.Audience != a.Audience {
				t.Errorf("component %q audience %q mismatches assembly %q", c.Name, c.Audience, a.Audience)
			}
		}
		return b.String()
	}
	renderText := joinContents(render)
	beatsText := joinContents(beats)

	if render.Audience != PreviewAudienceRender || beats.Audience != PreviewAudienceBeats {
		t.Fatalf("assembly audience mismatch: render=%q beats=%q", render.Audience, beats.Audience)
	}
	// 互斥：render 无里程碑表头，beats 无母题表头。
	if strings.Contains(renderText, "【全书宏观节拍定位】") {
		t.Errorf("render assembly must not contain milestone header, got:\n%s", renderText)
	}
	if strings.Contains(beatsText, "【有意母题与符号") {
		t.Errorf("beats assembly must not contain motif header, got:\n%s", beatsText)
	}
	// 各自独有块存在。
	if !strings.Contains(renderText, "【有意母题与符号（RC 精修锚点）】") {
		t.Errorf("render assembly must contain motif block, got:\n%s", renderText)
	}
	if !strings.Contains(beatsText, "【全书宏观节拍定位】") {
		t.Errorf("beats assembly must contain milestone block, got:\n%s", beatsText)
	}
	if !strings.Contains(beatsText, "【前序正史视界】") {
		t.Errorf("beats assembly must contain canon block, got:\n%s", beatsText)
	}
	// 共有块一致：世界法则/主角/百科三块在两路字符串相等。
	findContent := func(a PromptAssemblyPreview, substr string) string {
		for _, c := range a.Components {
			if strings.Contains(c.Name, substr) {
				return c.Content
			}
		}
		return ""
	}
	if got := findContent(render, "世界法则与公理"); got == "" {
		t.Errorf("render assembly missing shared world-rules block")
	} else if other := findContent(beats, "世界公理"); got != other {
		t.Errorf("shared world-rules block diverged:\nrender:\n%s\nbeats:\n%s", got, other)
	}
	if got := findContent(render, "主角状态参考"); got == "" {
		t.Errorf("render assembly missing shared protagonist block")
	} else if other := findContent(beats, "主角状态机"); got != other {
		t.Errorf("shared protagonist block diverged:\nrender:\n%s\nbeats:\n%s", got, other)
	}
	if got := findContent(render, "全域百科"); got == "" {
		t.Errorf("render assembly missing shared codex block")
	} else if other := findContent(beats, "全域百科"); got != other {
		t.Errorf("shared codex block diverged:\nrender:\n%s\nbeats:\n%s", got, other)
	}
}

// 为空则整块省略：空框架/空 horizon 不得输出占位符。
func TestPreviewAssembliesOmitEmpty(t *testing.T) {
	p := &domain.Project{ID: "p-empty", Title: "空书"}
	horizon := &engine.CanonHorizon{Project: p, TargetChapter: 1}
	req := PromptPreviewRequest{ChapterIndex: 1}

	render := buildRenderAssembly(p, horizon, req)
	beats := buildBeatsAssembly(p, horizon, req, "", nil)

	check := func(a PromptAssemblyPreview) {
		for _, c := range a.Components {
			if strings.TrimSpace(c.Content) == "" {
				t.Errorf("assembly %q component %q must be omitted when empty, not emitted blank", a.Audience, c.Name)
			}
		}
		for _, placeholder := range []string{"(当前章节未匹配", "暂无评审", "占位"} {
			if strings.Contains(a.UserPrompt, placeholder) {
				t.Errorf("assembly %q user prompt must not contain placeholder %q", a.Audience, placeholder)
			}
		}
	}
	check(render)
	check(beats)

	for _, c := range render.Components {
		if strings.Contains(c.Content, "【有意母题与符号") || strings.Contains(c.Content, "【全书宏观节拍定位】") {
			t.Errorf("empty render assembly must not contain motif/milestone blocks, got %q", c.Name)
		}
	}
	for _, c := range beats.Components {
		if strings.Contains(c.Content, "【有意母题与符号") || strings.Contains(c.Content, "【全书宏观节拍定位】") {
			t.Errorf("empty beats assembly must not contain motif/milestone blocks, got %q", c.Name)
		}
	}
}
