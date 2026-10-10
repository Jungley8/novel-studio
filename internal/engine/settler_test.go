package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type fakeSettlerLLMClient struct {
	settleJSON string
	err        error
}

func (f *fakeSettlerLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.settleJSON, nil
}

func (f *fakeSettlerLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 100, CompletionTokens: 150, TotalTokens: 250}, err
}

func (f *fakeSettlerLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := f.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 100, CompletionTokens: 150, TotalTokens: 250}}
	close(ch)
	return ch, nil
}

func TestSettler_ValidEvidencePasses(t *testing.T) {
	draft := "风雷激荡。林凡深吸一口气，仰头服下一枚清灵丹，顿感通体舒泰。随后他体内法力奔涌，赫然突破至练气七层！一旁的楚掌柜见状面色惨白，道心动摇。"
	baseline := &domain.CodexEntry{
		Name:      "林凡",
		Realm:     "练气六层巅峰",
		Inventory: "清灵丹x3, 青钢剑x1",
	}

	validJSON := "```json\n" + `{
		"inventory_delta": "-清灵丹",
		"power_delta": "突破至练气七层",
		"character_mutations": [
			{
				"name": "楚掌柜",
				"status_delta": "道心动摇，面色惨白",
				"relation_delta": "生出畏惧"
			}
		],
		"evidence": [
			{
				"field": "inventory",
				"quote": "仰头服下一枚清灵丹，顿感通体舒泰",
				"chapter_index": 1
			},
			{
				"field": "power",
				"quote": "体内法力奔涌，赫然突破至练气七层",
				"chapter_index": 1
			},
			{
				"field": "character:楚掌柜",
				"quote": "一旁的楚掌柜见状面色惨白，道心动摇",
				"chapter_index": 1
			}
		]
	}` + "\n```"

	client := &fakeSettlerLLMClient{settleJSON: validJSON}
	mutation, evidences, usage, err := SettleFromDraft(context.Background(), client, "mock-model", nil, baseline, draft, 1)
	if err != nil {
		t.Fatalf("expected settlement to succeed, got error: %v", err)
	}

	if mutation.InventoryDelta != "-清灵丹" {
		t.Errorf("expected InventoryDelta '-清灵丹', got %q", mutation.InventoryDelta)
	}
	if mutation.PowerDelta != "突破至练气七层" {
		t.Errorf("expected PowerDelta '突破至练气七层', got %q", mutation.PowerDelta)
	}
	if len(mutation.CharacterMutations) != 1 || mutation.CharacterMutations[0].Name != "楚掌柜" {
		t.Errorf("unexpected CharacterMutations: %+v", mutation.CharacterMutations)
	}
	if len(evidences) != 3 {
		t.Errorf("expected 3 evidences, got %d", len(evidences))
	}
	if usage.TotalTokens != 250 {
		t.Errorf("expected 250 tokens, got %d", usage.TotalTokens)
	}
}

func TestSettler_MissingEvidenceRejected(t *testing.T) {
	draft := "林凡获得一枚太虚古镜，随后转身离开山谷。"
	baseline := &domain.CodexEntry{Name: "林凡", Realm: "练气期"}

	// Missing evidence array completely
	noEvidenceJSON := `{
		"inventory_delta": "+太虚古镜",
		"power_delta": "",
		"character_mutations": [],
		"evidence": []
	}`

	client := &fakeSettlerLLMClient{settleJSON: noEvidenceJSON}
	_, _, _, err := SettleFromDraft(context.Background(), client, "mock-model", nil, baseline, draft, 1)
	if err == nil {
		t.Fatal("expected settlement to fail due to missing evidence, but got nil error")
	}
	if !strings.Contains(err.Error(), "lacks required evidence") {
		t.Errorf("expected error to mention lack of evidence, got: %v", err)
	}
}

func TestSettler_QuoteTooShortRejected(t *testing.T) {
	draft := "林凡拔出青钢剑，一剑断浪。"
	baseline := &domain.CodexEntry{Name: "林凡", Realm: "练气期"}

	// Quote is only 4 characters (< 8)
	shortQuoteJSON := `{
		"inventory_delta": "+绝世神兵",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{
				"field": "inventory",
				"quote": "拔出青钢",
				"chapter_index": 1
			}
		]
	}`

	client := &fakeSettlerLLMClient{settleJSON: shortQuoteJSON}
	_, _, _, err := SettleFromDraft(context.Background(), client, "mock-model", nil, baseline, draft, 1)
	if err == nil {
		t.Fatal("expected settlement to fail due to short quote, got nil error")
	}
	if !strings.Contains(err.Error(), "quote too short") {
		t.Errorf("expected error to mention quote too short, got: %v", err)
	}
}

func TestSettler_QuoteNotInDraftRejected(t *testing.T) {
	draft := "林凡盘膝打坐，吐纳天地初生的一缕紫气。"
	baseline := &domain.CodexEntry{Name: "林凡", Realm: "练气期"}

	// Quote does not appear in draft
	fabricatedQuoteJSON := `{
		"inventory_delta": "+九转还魂草",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{
				"field": "inventory",
				"quote": "采得一株通体莹白的九转还魂草",
				"chapter_index": 1
			}
		]
	}`

	client := &fakeSettlerLLMClient{settleJSON: fabricatedQuoteJSON}
	_, _, _, err := SettleFromDraft(context.Background(), client, "mock-model", nil, baseline, draft, 1)
	if err == nil {
		t.Fatal("expected settlement to fail due to fabricated quote, got nil error")
	}
	if !strings.Contains(err.Error(), "not found in draft text") {
		t.Errorf("expected error to mention quote not found in draft text, got: %v", err)
	}
}

func TestSettler_UnownedItemRemovalRejected(t *testing.T) {
	draft := "生死危机之下，林凡毫不犹豫咬破舌尖，吞服下救命的万年续命金丹，强行稳住心脉。"
	baseline := &domain.CodexEntry{
		Name:      "林凡",
		Realm:     "练气三层",
		Inventory: "粗布道袍x1, 铁木剑x1", // Does NOT possess 万年续命金丹
	}

	unownedRemovalJSON := `{
		"inventory_delta": "-万年续命金丹",
		"power_delta": "",
		"character_mutations": [],
		"evidence": [
			{
				"field": "inventory",
				"quote": "吞服下救命的万年续命金丹，强行稳住心脉",
				"chapter_index": 1
			}
		]
	}`

	client := &fakeSettlerLLMClient{settleJSON: unownedRemovalJSON}
	_, _, _, err := SettleFromDraft(context.Background(), client, "mock-model", nil, baseline, draft, 1)
	if err == nil {
		t.Fatal("expected settlement to fail invariant pre-check for unowned item, got nil error")
	}
	if !strings.Contains(err.Error(), "pre-check rejected") {
		t.Errorf("expected error to mention invariant pre-check rejection, got: %v", err)
	}
}
