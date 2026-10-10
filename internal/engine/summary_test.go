package engine

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type fakeSummaryLLMClient struct {
	response string
	err      error
}

func (f *fakeSummaryLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

func (f *fakeSummaryLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 80, CompletionTokens: 120, TotalTokens: 200}, err
}

func (f *fakeSummaryLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 80, CompletionTokens: 120, TotalTokens: 200}}
	close(ch)
	return ch, nil
}

func TestSummarizeChapter_Case1_FourElementsAndLength(t *testing.T) {
	ctx := context.Background()
	project := &domain.Project{
		ID:    "proj-test-1",
		Title: "太玄问道录",
	}
	draft := "风雷激荡。林凡深吸一口气，持残剑迎战青云门执事赵无极。两人大战数十回合，林凡强行燃血催动无极剑意斩灭敌首，然而自身亦力竭重伤跌入葬仙渊。寒雾弥漫中，深渊底部忽然传来古剑幽咽共鸣。"
	mutation := domain.StateMutation{
		PowerDelta: "+无极剑意小成",
		CharacterMutations: []domain.CharacterMutation{
			{Name: "赵无极", StatusDelta: "身死道消", RelationDelta: "仇敌伏诛"},
		},
	}

	t.Run("valid summary within 100 runes with four elements", func(t *testing.T) {
		validJSON := "```json\n" + `{
			"characters": "林凡，青云门执事赵无极",
			"conflict": "黑煞谷遭青云门执事拦截截杀",
			"result": "催动残剑斩灭敌首后重伤坠渊",
			"suspense": "深渊底部传来古剑异鸣与魔气波动",
			"summary": "林凡在黑煞谷遭青云门执事赵无极拦截截杀，强行催动残剑斩灭强敌，力竭坠入葬仙渊，深渊底传来古剑异鸣。"
		}` + "\n```"

		client := &fakeSummaryLLMClient{response: validJSON}
		summary, usage, err := SummarizeChapter(ctx, client, "fake-reasoner", project, 1, draft, mutation)
		if err != nil {
			t.Fatalf("SummarizeChapter failed: %v", err)
		}

		runes := utf8.RuneCountInString(summary.Text)
		if runes > 100 {
			t.Errorf("summary length exceeded 100 runes: got %d", runes)
		}
		if runes == 0 {
			t.Errorf("summary text was empty")
		}

		// Ensure four narrative elements are represented in summary
		for _, elem := range []string{"林凡", "赵无极", "拦截", "斩灭", "坠入葬仙渊", "古剑异鸣"} {
			if !strings.Contains(summary.Text, elem) {
				t.Errorf("expected summary to contain narrative element %q, got: %s", elem, summary.Text)
			}
		}

		// Ensure no meta-narrative words
		for _, meta := range []string{"本章", "笔者", "本回", "故事中"} {
			if strings.Contains(summary.Text, meta) {
				t.Errorf("forbidden meta word %q found in summary: %s", meta, summary.Text)
			}
		}

		if summary.ProjectID != "proj-test-1" {
			t.Errorf("expected project ID 'proj-test-1', got %q", summary.ProjectID)
		}
		if summary.ChapterIndex != 1 {
			t.Errorf("expected chapter index 1, got %d", summary.ChapterIndex)
		}
		if summary.Model != "fake-reasoner" {
			t.Errorf("expected model 'fake-reasoner', got %q", summary.Model)
		}
		if usage.TotalTokens != 200 {
			t.Errorf("expected 200 total tokens, got %d", usage.TotalTokens)
		}
	})

	t.Run("overlength fake summary returns error without truncation", func(t *testing.T) {
		overlengthJSON := "```json\n" + `{
			"summary": "在极其浩瀚苍茫的无尽修仙世界之中，林凡孤身一人行走在苍莽古老的无尽大山深处，忽然遭遇了来自青云圣地的无数强者倾巢而出的重重包围与围追堵截。双方展开了惊天动地的大战，虚空寸寸碎裂，天地变色，日月无光。林凡拼尽全身最后一丝法力，终于成功斩杀了带头的数位元婴期大能长老，随后负伤极重，倒地昏迷不醒，生死未卜。"
		}` + "\n```"

		client := &fakeSummaryLLMClient{response: overlengthJSON}
		_, _, err := SummarizeChapter(ctx, client, "fake-reasoner", project, 1, draft, mutation)
		if err == nil {
			t.Fatalf("expected error for overlength fake summary (>100 runes), got nil")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("expected length exceed error message, got: %v", err)
		}
	})

	t.Run("empty draft text returns error", func(t *testing.T) {
		client := &fakeSummaryLLMClient{response: `{"summary": "有效摘要"}`}
		_, _, err := SummarizeChapter(ctx, client, "fake-reasoner", project, 1, "   ", mutation)
		if err == nil {
			t.Fatalf("expected error for empty draft text, got nil")
		}
	})

	t.Run("nil client returns error", func(t *testing.T) {
		_, _, err := SummarizeChapter(ctx, nil, "fake-reasoner", project, 1, draft, mutation)
		if err == nil {
			t.Fatalf("expected error for nil client, got nil")
		}
	})

	t.Run("meta narrative word rejected", func(t *testing.T) {
		metaJSON := `{"summary": "本章中林凡在黑煞谷大展神威斩杀敌首，笔者在此埋下伏笔。"}`
		client := &fakeSummaryLLMClient{response: metaJSON}
		_, _, err := SummarizeChapter(ctx, client, "fake-reasoner", project, 1, draft, mutation)
		if err == nil {
			t.Fatalf("expected error for meta narrative words, got nil")
		}
	})
}

func TestBuildVolumeRetrospective_Case2_ThreeSections(t *testing.T) {
	ctx := context.Background()
	project := &domain.Project{
		ID:    "proj-test-1",
		Title: "太玄问道录",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:       1,
					Title:             "潜龙出渊",
					Theme:             "底层修士崛起",
					CoreGoal:          "夺取筑基丹并脱离宗门追杀",
					Climax:            "黑煞谷斩执事，坠葬仙渊破局",
					EstimatedChapters: 40,
				},
				{
					VolumeIndex:       2,
					Title:             "魔渊争锋",
					EstimatedChapters: 50,
				},
			},
		},
	}

	volume := project.Framework.VolumeArcs[0]
	summaries := []domain.ChapterSummary{
		{
			ProjectID:    "proj-test-1",
			ChapterIndex: 1,
			Text:         "林凡黑煞谷破伏击斩敌首，坠入深渊获古剑。",
		},
		{
			ProjectID:    "proj-test-1",
			ChapterIndex: 2,
			Text:         "林凡炼化古剑残魂修成无极剑意，引雷劫破关。",
		},
	}

	t.Run("valid retrospective containing three sections within 300 runes", func(t *testing.T) {
		validJSON := "```json\n" + `{
			"happened": "林凡历经黑煞谷伏击、深渊悟剑与雷劫破关，彻底荡平青云门北域追杀分部并夺得太虚遗迹秘宝。",
			"remaining": "太玄宗名震北域，林凡成就无极剑体但道基暗藏煞气，急需九阳圣水调和方可安稳筑基。",
			"unresolved": "葬仙渊深处封印的大荒魔尊真身已然苏醒，宗门高层潜伏的影阁内奸仍未浮出水面。"
		}` + "\n```"

		client := &fakeSummaryLLMClient{response: validJSON}
		retro, usage, err := BuildVolumeRetrospective(ctx, client, "fake-reasoner", project, volume, summaries)
		if err != nil {
			t.Fatalf("BuildVolumeRetrospective failed: %v", err)
		}

		// Ensure three sections are present
		if !strings.Contains(retro.Text, "发生") {
			t.Errorf("retrospective missing '发生' section: %s", retro.Text)
		}
		if !strings.Contains(retro.Text, "留下") {
			t.Errorf("retrospective missing '留下' section: %s", retro.Text)
		}
		if !strings.Contains(retro.Text, "未回收") {
			t.Errorf("retrospective missing '未回收' section: %s", retro.Text)
		}

		runes := utf8.RuneCountInString(retro.Text)
		if runes > 300 {
			t.Errorf("retrospective exceeded 300 runes: got %d", runes)
		}

		// Ensure no meta-narrative words
		for _, meta := range []string{"本卷", "本章", "笔者"} {
			if strings.Contains(retro.Text, meta) {
				t.Errorf("forbidden meta word %q found in retrospective: %s", meta, retro.Text)
			}
		}

		if retro.ProjectID != "proj-test-1" {
			t.Errorf("expected project ID 'proj-test-1', got %q", retro.ProjectID)
		}
		if retro.VolumeIndex != 1 {
			t.Errorf("expected volume index 1, got %d", retro.VolumeIndex)
		}
		if retro.FromChapter != 1 || retro.ToChapter != 40 {
			t.Errorf("expected chapter range 1..40, got %d..%d", retro.FromChapter, retro.ToChapter)
		}
		if usage.TotalTokens != 200 {
			t.Errorf("expected 200 total tokens, got %d", usage.TotalTokens)
		}
	})

	t.Run("missing sections returns error", func(t *testing.T) {
		incompleteJSON := `{"text": "【发生】林凡破关。【留下】战力大进。"}`
		client := &fakeSummaryLLMClient{response: incompleteJSON}
		_, _, err := BuildVolumeRetrospective(ctx, client, "fake-reasoner", project, volume, summaries)
		if err == nil {
			t.Fatalf("expected error for missing 未回收 section, got nil")
		}
	})

	t.Run("overlength retrospective returns error", func(t *testing.T) {
		longText := "【发生】" + strings.Repeat("浩瀚苍茫的世界发生了大战，天地碎裂，众生喋血。", 8) + "\n【留下】道基受损。\n【未回收】魔尊未死。"
		longJSON := `{"text": "` + longText + `"}`
		client := &fakeSummaryLLMClient{response: longJSON}
		_, _, err := BuildVolumeRetrospective(ctx, client, "fake-reasoner", project, volume, summaries)
		if err == nil {
			t.Fatalf("expected error for overlength retrospective, got nil")
		}
	})

	t.Run("nil framework calculates chapter range from summaries", func(t *testing.T) {
		projectNoFw := &domain.Project{
			ID:    "proj-no-fw",
			Title: "散修传",
		}
		volumeStandalone := domain.VolumeArc{
			VolumeIndex: 1,
			Title:       "第一卷",
		}
		rawJSON := `{"happened": "破关斩敌", "remaining": "道基稳固", "unresolved": "宿敌潜逃"}`
		client := &fakeSummaryLLMClient{response: rawJSON}
		retro, _, err := BuildVolumeRetrospective(ctx, client, "fake-reasoner", projectNoFw, volumeStandalone, summaries)
		if err != nil {
			t.Fatalf("BuildVolumeRetrospective with nil framework failed: %v", err)
		}
		if retro.FromChapter != 1 || retro.ToChapter != 2 {
			t.Errorf("expected range derived from summaries 1..2, got %d..%d", retro.FromChapter, retro.ToChapter)
		}
	})

	t.Run("empty summaries returns error", func(t *testing.T) {
		client := &fakeSummaryLLMClient{response: `{"happened": "发生", "remaining": "留下", "unresolved": "未回收"}`}
		_, _, err := BuildVolumeRetrospective(ctx, client, "fake-reasoner", project, volume, nil)
		if err == nil {
			t.Fatalf("expected error for empty summaries, got nil")
		}
	})
}
