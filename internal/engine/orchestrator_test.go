package engine_test

import (
	"context"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockLLMClient) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, engine.TokenUsage, error) {
	if m.err != nil {
		return "", engine.TokenUsage{}, m.err
	}
	return m.response, engine.TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}, nil
}

func (m *mockLLMClient) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (<-chan engine.StreamChunk, error) {
	ch := make(chan engine.StreamChunk, 2)
	ch <- engine.StreamChunk{Delta: m.response}
	ch <- engine.StreamChunk{Done: true, Usage: &engine.TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}}
	close(ch)
	return ch, nil
}

func TestOrchestrator_DeriveBeats(t *testing.T) {
	mockJSON := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 5, "action": "反派拦路叫嚣", "expectation_broken": "主角面色平静", "reader_emotion": "紧张压抑", "info_gap": "反派不知主角已突破"},
			{"phase": "绝地反转", "tension": 9, "action": "主角一剑封喉", "expectation_broken": "反派不可置信", "reader_emotion": "大呼解气", "hook_type": "CLIFFHANGER"}
		],
		"state_mutation": {
			"inventory_delta": "消耗长剑",
			"power_delta": "略有领悟"
		}
	}`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{
		Title:          "剑道独尊",
		TargetPlatform: "番茄脑洞",
		Protagonist: domain.Protagonist{
			NameAndLevel: "陆青 (练气五层)",
		},
	}

	out, err := orch.DeriveBeats(context.Background(), "deepseek-reasoner", p, 1, "同门挑衅", nil)
	if err != nil {
		t.Fatalf("DeriveBeats failed: %v", err)
	}

	if len(out.Beats) != 2 {
		t.Errorf("expected 2 beats, got %d", len(out.Beats))
	}
	if out.Beats[0].ReaderEmotion != "紧张压抑" {
		t.Errorf("expected reader emotion 紧张压抑, got %s", out.Beats[0].ReaderEmotion)
	}
	if out.Beats[1].HookType != "CLIFFHANGER" {
		t.Errorf("expected hook type CLIFFHANGER, got %s", out.Beats[1].HookType)
	}
	if out.StateMutation.InventoryDelta != "消耗长剑" {
		t.Errorf("unexpected state mutation: %v", out.StateMutation)
	}
}

func TestOrchestrator_SuggestChapterConflict(t *testing.T) {
	mockResponse := "核心冲突：青云宗执法堂突然夜袭药园，欲以私藏妖丹之罪废黜主角修为，主角必须在三炷香内借助护山残阵逆向反杀。"
	mock := &mockLLMClient{response: mockResponse}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{
		Title:          "万古剑尊",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "天道有缺，弱肉强食",
		Protagonist: domain.Protagonist{
			NameAndLevel: "叶枫 (练气三层)",
			CoreGoal:     "查明家族血洗真相",
		},
	}
	horizon := &engine.CanonHorizon{
		Project:       p,
		TargetChapter: 1,
	}

	conflict, usage, err := orch.SuggestChapterConflict(context.Background(), "deepseek-reasoner", horizon)
	if err != nil {
		t.Fatalf("SuggestChapterConflict failed: %v", err)
	}
	expected := "青云宗执法堂突然夜袭药园，欲以私藏妖丹之罪废黜主角修为，主角必须在三炷香内借助护山残阵逆向反杀。"
	if conflict != expected {
		t.Errorf("expected cleaned conflict %q, got %q", expected, conflict)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected usage 300, got %d", usage.TotalTokens)
	}
}

func TestOrchestrator_RenderScene(t *testing.T) {
	mock := &mockLLMClient{response: "青云峰上，寒风如刀。陆青抬起眼皮，指尖微屈。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "剑道独尊", TargetPlatform: "知乎盐言"}
	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 8, Action: "风雪封山"},
	}

	content, usage, err := orch.RenderScene(context.Background(), "deepseek-chat", p, 1, beats, 1500)
	if err != nil {
		t.Fatalf("RenderScene failed: %v", err)
	}

	if content != "青云峰上，寒风如刀。陆青抬起眼皮，指尖微屈。" {
		t.Errorf("unexpected content: %s", content)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected usage 300, got %d", usage.TotalTokens)
	}
}

func TestOrchestrator_RenderSceneWithHorizon(t *testing.T) {
	mock := &mockLLMClient{response: "漫天风雪呼啸。陆青迎风而立。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "剑道独尊", TargetPlatform: "番茄脑洞"}
	horizon := &engine.CanonHorizon{
		Project:       p,
		TargetChapter: 2,
		TailAnchor:    "寒风卷起漫天飞雪，山门前悄然立着一道冷冽黑影。",
	}
	beats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 6, Action: "风雪封山"},
		{Phase: "章末留钩", Tension: 9, Action: "剑气破空", HookType: "CLIFFHANGER"},
	}

	content, usage, err := orch.RenderSceneWithHorizon(context.Background(), "deepseek-chat", horizon, beats, 2000)
	if err != nil {
		t.Fatalf("RenderSceneWithHorizon failed: %v", err)
	}

	if content != "漫天风雪呼啸。陆青迎风而立。" {
		t.Errorf("unexpected content: %s", content)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected usage 300, got %d", usage.TotalTokens)
	}
}

func TestOrchestrator_ReviewDraft(t *testing.T) {
	mockJSON := `{
		"verdict": "REVISION_NEEDED",
		"score": 72,
		"issues": ["主角未携带玄重尺却施展了该兵器技能"],
		"suggestions": "改为徒手拳法"
	}`
	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "斗破苍穹"}
	rev, usage, err := orch.ReviewDraft(context.Background(), "deepseek-reasoner", p, 1, nil, "草稿正文...")
	if err != nil {
		t.Fatalf("ReviewDraft failed: %v", err)
	}

	if rev.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected REVISION_NEEDED, got %s", rev.Verdict)
	}
	if rev.Score != 72 {
		t.Errorf("expected score 72, got %d", rev.Score)
	}
	if len(rev.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(rev.Issues))
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected usage 300, got %d", usage.TotalTokens)
	}
}

func TestOrchestrator_RewriteDraft(t *testing.T) {
	mock := &mockLLMClient{response: "重修后的正文：陆青翻掌成印，呼啸破风。"}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{Title: "斗破苍穹"}
	rev := &domain.ReviewResult{
		Verdict:     domain.ReviewVerdictRevision,
		Score:       70,
		Issues:      []string{"需增加拳法动作"},
		Suggestions: "改为八极崩",
	}

	rewritten, usage, err := orch.RewriteDraft(context.Background(), "deepseek-chat", p, 1, "旧草稿", rev)
	if err != nil {
		t.Fatalf("RewriteDraft failed: %v", err)
	}

	if rewritten != "重修后的正文：陆青翻掌成印，呼啸破风。" {
		t.Errorf("unexpected rewritten text: %s", rewritten)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected usage 300, got %d", usage.TotalTokens)
	}
}

func TestOrchestrator_BootstrapFramework(t *testing.T) {
	mockJSON := `{
		"theme_premise": "万古三千年修真界沉浮，凡人逆修弑神，天道实为寄生真魔",
		"world_axioms": ["天地以众生为鼎炉", "飞升实为献祭"],
		"power_ladder": [
			{"realm": "练气期", "description": "引气入体", "bottleneck": "开辟经脉", "drawback": "灵气微毒"},
			{"realm": "筑基期", "description": "铸造道基", "bottleneck": "筑基丹真伪", "drawback": "寿元绑定天道"}
		],
		"factions": [
			{"name": "青云正宗", "alignment": "天道走狗", "doctrine": "顺天承命", "threat_level": "极高"}
		],
		"key_characters": [
			{"name": "白发剑尊", "role": "引路残魂", "realm": "大乘残魂", "goal": "寻觅传人弑天", "fate_arc": "魂飞魄散"}
		],
		"volume_arcs": [
			{
				"volume_index": 1,
				"title": "第一卷：边陲残灵与破局逃亡",
				"theme": "生存与觉醒",
				"core_goal": "打破宗门死劫并筑基逃生",
				"climax": "斩杀外门执事血祭破界",
				"estimated_chapters": 30,
				"key_payoffs": ["丹田黑铁觉醒"]
			}
		],
		"seed_hooks": [
			{
				"title": "丹田深处的锈迹黑铁",
				"details": "无法炼化的古朴铁片，遇血会散发灼热",
				"created_chapter": 1,
				"target_chapter": 20,
				"status": "OPEN"
			}
		]
	}`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	req := engine.FrameworkBootstrapRequest{
		Title:          "凡人弑神录",
		TargetPlatform: "起点仙侠",
		CoreConcept:    "三千年修真沉浮，天道是寄生真魔",
	}

	fw, usage, err := orch.BootstrapFramework(context.Background(), "deepseek-reasoner", req)
	if err != nil {
		t.Fatalf("BootstrapFramework failed: %v", err)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected usage 300, got %d", usage.TotalTokens)
	}

	if fw.ThemePremise != "万古三千年修真界沉浮，凡人逆修弑神，天道实为寄生真魔" {
		t.Errorf("unexpected theme premise: %s", fw.ThemePremise)
	}
	if len(fw.WorldAxioms) != 2 {
		t.Errorf("expected 2 axioms, got %d", len(fw.WorldAxioms))
	}
	if len(fw.PowerLadder) != 2 {
		t.Errorf("expected 2 power tiers, got %d", len(fw.PowerLadder))
	} else if fw.PowerLadder[0].Tier != 1 || fw.PowerLadder[1].Tier != 2 {
		t.Errorf("expected tiers 1 and 2, got %d and %d", fw.PowerLadder[0].Tier, fw.PowerLadder[1].Tier)
	}
	if len(fw.VolumeArcs) != 1 {
		t.Errorf("expected 1 volume arc, got %d", len(fw.VolumeArcs))
	} else if fw.VolumeArcs[0].VolumeIndex != 1 {
		t.Errorf("expected volume_index 1, got %d", fw.VolumeArcs[0].VolumeIndex)
	}
	if len(fw.SeedHooks) != 1 {
		t.Errorf("expected 1 seed hook, got %d", len(fw.SeedHooks))
	}
}

func TestOrchestrator_GenerateCodexEntry(t *testing.T) {
	mockJSON := `{
		"name": "天残老祖",
		"aliases": ["残魔", "幽冥尊者"],
		"category": "CHARACTER",
		"summary": "深藏古墓的邪道大能，主角的残酷导师",
		"details_markdown": "曾横压东荒三百年，如今神魂衰朽，欲夺舍重修。",
		"tracking_mode": "AUTO_MENTION",
		"archetype": "MENTOR",
		"voice_tone": "沙哑阴冷，暗藏杀机",
		"core_motivation": "寻找完美肉身延寿",
		"current_disposition": "WARY"
	}`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{ID: "proj-1", Title: "逆天魔尊"}
	entry, _, err := orch.GenerateCodexEntry(context.Background(), "deepseek-reasoner", p, engine.CodexGenerateRequest{
		Name:     "天残老祖",
		Category: domain.CategoryCharacter,
		Prompt:   "神秘老魔头",
	})
	if err != nil {
		t.Fatalf("GenerateCodexEntry failed: %v", err)
	}
	if entry.Name != "天残老祖" {
		t.Errorf("expected name 天残老祖, got %s", entry.Name)
	}
	if entry.ProjectID != "proj-1" {
		t.Errorf("expected project ID proj-1, got %s", entry.ProjectID)
	}
	if len(entry.Aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(entry.Aliases))
	}
}

func TestOrchestrator_ExtractCodexRelations(t *testing.T) {
	mockJSON := `[
		{
			"source_name": "楚风",
			"target_name": "残骨铁印",
			"relation_type": "POSSESSES",
			"description": "主角的核心本命法宝"
		},
		{
			"source_name": "楚风",
			"target_name": "厉魔尊",
			"relation_type": "NEMESIS",
			"description": "灭门宿敌，不死不休"
		}
	]`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{ID: "proj-1"}
	entries := []*domain.CodexEntry{
		{ID: "e1", Name: "楚风", Category: domain.CategoryCharacter},
		{ID: "e2", Name: "残骨铁印", Category: domain.CategoryItem},
		{ID: "e3", Name: "厉魔尊", Category: domain.CategoryCharacter},
	}

	rels, _, err := orch.ExtractCodexRelations(context.Background(), "deepseek-reasoner", p, entries, "楚风手持残骨铁印，死战厉魔尊。")
	if err != nil {
		t.Fatalf("ExtractCodexRelations failed: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(rels))
	}
	if rels[0].SourceEntryID != "e1" || rels[0].TargetEntryID != "e2" {
		t.Errorf("expected e1 -> e2, got %s -> %s", rels[0].SourceEntryID, rels[0].TargetEntryID)
	}
	if rels[1].RelationType != "NEMESIS" {
		t.Errorf("expected relation NEMESIS, got %s", rels[1].RelationType)
	}
}

func TestOrchestrator_GenerateMatrixScenes(t *testing.T) {
	mockJSON := `[
		{
			"scene_index": 1,
			"title": "暗市夺印",
			"dramatic_goal": "拍下残骨铁印",
			"conflict_barrier": "黑煞门横插一脚抬价",
			"tension_level": 7,
			"prose_content": "拍卖会暗流涌动，主角沉着以幻影符脱身。"
		},
		{
			"scene_index": 2,
			"title": "雨夜袭杀",
			"dramatic_goal": "突出重围，反杀追兵",
			"conflict_barrier": "黑煞门三名筑基长老伏击",
			"tension_level": 9,
			"prose_content": "雨夜惊雷，主角初现剑意一剑荡平。"
		}
	]`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{ID: "proj-1"}
	scenes, _, err := orch.GenerateMatrixScenes(context.Background(), "deepseek-reasoner", p, engine.MatrixSceneGenerateRequest{
		VolumeIndex:  1,
		ChapterIndex: 5,
		ChapterTitle: "雨夜断魂",
		CoreConflict: "夺宝后被追杀伏击",
	})
	if err != nil {
		t.Fatalf("GenerateMatrixScenes failed: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}
	if scenes[0].TensionLevel != 7 {
		t.Errorf("expected tension 7, got %d", scenes[0].TensionLevel)
	}
	if scenes[1].Title != "雨夜袭杀" {
		t.Errorf("expected scene title 雨夜袭杀, got %s", scenes[1].Title)
	}
}

func TestOrchestrator_AnalyzeProtagonistState(t *testing.T) {
	mockJSON := `{
		"name_and_level": "楚风 (练气六层)",
		"inventory": "残骨铁印x1, 灵石x20",
		"core_goal": "前往万剑宗拜山",
		"health_status": "内息平稳，肉身增强",
		"breakthrough_event": {
			"happened": true,
			"from_realm": "练气五层",
			"to_realm": "练气六层",
			"reason": "融合残印精血顿悟"
		}
	}`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{
		ID: "proj-1",
		Protagonist: domain.Protagonist{
			NameAndLevel: "楚风 (练气五层)",
			Inventory:    "青铜残片x1",
		},
	}
	chapters := []*domain.Chapter{
		{ChapterIndex: 1, Title: "第一章", Content: "楚风融合精血，突破练气六层。"},
	}

	updated, _, err := orch.AnalyzeProtagonistState(context.Background(), "deepseek-reasoner", p, chapters)
	if err != nil {
		t.Fatalf("AnalyzeProtagonistState failed: %v", err)
	}
	if updated.NameAndLevel != "楚风 (练气六层)" {
		t.Errorf("expected level 练气六层, got %s", updated.NameAndLevel)
	}
	if updated.StructuredLevel == nil || len(updated.StructuredLevel.History) != 1 {
		t.Fatalf("expected 1 history breakthrough event")
	}
	if updated.StructuredLevel.History[0].ToRealm != "练气六层" {
		t.Errorf("expected to_realm 练气六层, got %s", updated.StructuredLevel.History[0].ToRealm)
	}
}

func TestOrchestrator_ExtractPlotHooks(t *testing.T) {
	mockJSON := `[
		{
			"title": "残骨铁印的第二道器灵",
			"details": "铁印内部隐隐传来远古封印哀鸣",
			"created_chapter": 2,
			"target_chapter": 15
		}
	]`

	mock := &mockLLMClient{response: mockJSON}
	orch := engine.NewOrchestrator(mock)

	p := &domain.Project{ID: "proj-1"}
	chapters := []*domain.Chapter{
		{ChapterIndex: 2, Title: "异变", CoreConflict: "铁印共鸣", Content: "残印内部传来阵阵异响。"},
	}

	hooks, _, err := orch.ExtractPlotHooks(context.Background(), "deepseek-reasoner", p, chapters, nil)
	if err != nil {
		t.Fatalf("ExtractPlotHooks failed: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].Title != "残骨铁印的第二道器灵" {
		t.Errorf("expected hook title 残骨铁印的第二道器灵, got %s", hooks[0].Title)
	}
	if hooks[0].TargetChapter != 15 {
		t.Errorf("expected target chapter 15, got %d", hooks[0].TargetChapter)
	}
}
