package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// SettleFromDraft audits draft text strictly according to literal facts, deriving post-chapter
// StateMutation along with textual evidence citations (quotes >= 8 characters).
// It verifies evidence presence and performs dry-run invariant pre-checks against baseline.
func SettleFromDraft(
	ctx context.Context,
	client LLMClient,
	model string,
	project *domain.Project,
	baseline *domain.CodexEntry,
	draftText string,
	chapterIndex int,
) (domain.StateMutation, []domain.SettlementEvidence, TokenUsage, error) {
	if client == nil {
		return domain.StateMutation{}, nil, TokenUsage{}, errors.New("llm client cannot be nil")
	}
	if strings.TrimSpace(draftText) == "" {
		return domain.StateMutation{}, nil, TokenUsage{}, errors.New("draft text cannot be empty")
	}

	projectTitle := "小说项目"
	if project != nil && strings.TrimSpace(project.Title) != "" {
		projectTitle = strings.TrimSpace(project.Title)
	}

	baseNameLevel := baselineNameLevel(baseline)
	baseInventory := baselineInventory(baseline)

	systemPrompt := `你是一名网络小说严谨战力数值与实体状态终审精算师。
你的任务是根据主角基线状态与最新章节的正文内容，严格依据【正文白纸黑字写出的事实】，推演出本章发生的状态变迁（StateMutation）与原文证据（Evidence）。
严禁凭空为主角或配角添加正文未提及的道具、造化或状态。

铁律原则：
1. 唯正文主义：正文没有明确写出的道具获得/消耗、修为突破或配角变动，一律严禁凭空添加！
2. 证据链严密：每一个非空的变动（inventory_delta / power_delta / character_mutations），都必须在 evidence 中提供至少一条来自正文的字面引用片段（quote，必须完全来自正文，且长度不少于8个字符）！无证据的变迁将被严格拒绝！
3. 消耗道具必须在基线状态中持有；未持有的道具严禁出现消耗！
4. 必须且仅输出标准 JSON 格式：
{
  "inventory_delta": "物品变动描述（获得写'+道具名'，消耗写'-道具名'，多个用逗号隔开；若无变动填空字符串\"\"）",
  "power_delta": "境界或战力变动描述（若无变动填空字符串\"\"）",
  "character_mutations": [
    {
      "name": "配角或反派姓名",
      "status_delta": "该角色状态/心境/伤势/修为变迁",
      "relation_delta": "对主角或全局的关系变化"
    }
  ],
  "evidence": [
    {
      "field": "inventory | power | character:<角色名>",
      "quote": "正文中的原文精准引用片段（≥8字）",
      "chapter_index": 章节序号
    }
  ]
}`

	userPrompt := fmt.Sprintf(`【书名】《%s》 第 %d 章
【主角基线状态】%s | 随身物品: %s
【正文草稿】
%s

请严格依据正文白纸黑字推演本章状态变迁，并给出每一项变动的原文引用证据（quote ≥ 8字），输出标准 JSON。`,
		projectTitle,
		chapterIndex,
		baseNameLevel,
		baseInventory,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, client, model, systemPrompt, userPrompt, 0.2, nil)
	if err != nil {
		return domain.StateMutation{}, nil, usage, fmt.Errorf("settle LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return domain.StateMutation{}, nil, usage, fmt.Errorf("settle clean json failed: %w", err)
	}

	var delta domain.SettlementDelta
	if err := json.Unmarshal([]byte(cleanJSON), &delta); err != nil {
		return domain.StateMutation{}, nil, usage, fmt.Errorf("settle unmarshal json failed: %w", err)
	}

	for i := range delta.Evidence {
		if delta.Evidence[i].ChapterIndex <= 0 {
			delta.Evidence[i].ChapterIndex = chapterIndex
		}
	}

	// 1. Evidence validation: all non-empty deltas must be backed by quotes from draft
	if err := delta.Validate(draftText); err != nil {
		return domain.StateMutation{}, nil, usage, fmt.Errorf("settle evidence validation failed: %w", err)
	}

	// 2. State mutation invariant pre-check on baseline
	mutation := delta.ToStateMutation()
	if err := ValidateStateMutationInvariants(baseline, mutation); err != nil {
		return domain.StateMutation{}, nil, usage, fmt.Errorf("settle state mutation pre-check rejected: %w", err)
	}

	return mutation, delta.Evidence, usage, nil
}

// ValidateStateMutationInvariants performs a dry-run execution of ApplyStateMutation on a copy
// of the baseline entity to detect invariant violations (e.g. consuming unowned items) before committing.
func ValidateStateMutationInvariants(baseline *domain.CodexEntry, mutation domain.StateMutation) error {
	testBase := baseline
	if testBase == nil {
		testBase = &domain.CodexEntry{
			Name:     "主角",
			Category: domain.CategoryCharacter,
		}
	}
	clone := *testBase
	if testBase.StructuredLevel != nil {
		lvlCopy := *testBase.StructuredLevel
		clone.StructuredLevel = &lvlCopy
	}
	if len(testBase.StructuredItems) > 0 {
		clone.StructuredItems = make([]domain.InventoryItem, len(testBase.StructuredItems))
		copy(clone.StructuredItems, testBase.StructuredItems)
	}

	auditTrail, err := domain.ApplyStateMutation(&clone, mutation)
	if err != nil {
		return fmt.Errorf("state mutation application error: %w", err)
	}
	for _, t := range auditTrail {
		if strings.Contains(t, "[Ledger Warning]") || strings.Contains(t, "尝试移除未持有的物品") {
			return fmt.Errorf("state mutation pre-check rejected invariant violation: %s", t)
		}
	}
	return nil
}

func baselineNameLevel(e *domain.CodexEntry) string {
	if e == nil {
		return "主角 (未知)"
	}
	return e.NameAndLevel()
}

func baselineInventory(e *domain.CodexEntry) string {
	if e == nil || strings.TrimSpace(e.Inventory) == "" {
		return "无"
	}
	return e.Inventory
}
