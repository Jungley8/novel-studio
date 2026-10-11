package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type StateValidationObservation struct {
	Code     string   `json:"code"`
	Summary  string   `json:"summary"`
	Evidence []string `json:"evidence,omitempty"`
}

type StateValidationResult struct {
	Consistent             bool                         `json:"consistent"`
	ReconciliationRequired bool                         `json:"reconciliation_required"`
	Observations           []StateValidationObservation `json:"observations"`
}

// ValidateSettledState 以 old 卡 / new 卡 / 正文独立校验 settler 输出。
// 空增量（mutation 全空）→ 直接 {Consistent:true}，不调 LLM。
func ValidateSettledState(
	ctx context.Context,
	client LLMClient,
	model string,
	baseline *domain.CodexEntry,
	mutation domain.StateMutation,
	draftText string,
	chapterIndex int,
) (StateValidationResult, TokenUsage, error) {
	if client == nil {
		return StateValidationResult{}, TokenUsage{}, errors.New("llm client cannot be nil")
	}
	if isStateMutationEmpty(mutation) {
		return StateValidationResult{Consistent: true}, TokenUsage{}, nil
	}

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
	_, _ = domain.ApplyStateMutation(&clone, mutation)

	oldCard := renderCodexStateCard(testBase)
	newCard := renderCodexStateCard(&clone)
	mutJSON, _ := json.Marshal(mutation)

	systemPrompt := `你是一名状态对账校验员。
你的任务是对照角色状态变更前后的状态卡（Old卡与New卡）以及最新章节正文内容，独立校验推演出的状态变迁是否准确无误。

检查重点：
(a) 与正文矛盾的状态：New卡中的状态、物品或境界是否与正文明确描写的发生冲突；
(b) 漏记的变迁：正文白纸黑字明确写出但New卡未体现的角色变动或道具变迁；
(c) 时间序不一致：状态发生的时间顺序与因果逻辑是否紊乱。

判决准则：
- consistent: 若完全一致且无矛盾无遗漏则为 true，否则为 false。
- reconciliation_required: 仅在“换一种状态投影可消除矛盾”时为 true；若正文本身自相矛盾则为 false（仅记录 observation）。
- observations: 发现的所有问题清单。
- observations 中每条的 evidence 必须包含至少一条完全来自正文的引用片段（不少于 8 字）；无法在正文中验证的观察将被证据门丢弃。

必须且仅输出严格的标准 JSON 格式：
{
  "consistent": true,
  "reconciliation_required": false,
  "observations": [
    {
      "code": "问题编码（如 item-conflict / missing-mutation / temporal-inconsistency / state-contradiction）",
      "summary": "问题简述",
      "evidence": ["引用的正文关键句"]
    }
  ]
}`

	userPrompt := fmt.Sprintf(`【第 %d 章状态对账】
【Old 状态卡（变更前）】
%s

【本次推演出的状态变迁（StateMutation JSON）】
%s

【New 状态卡（变迁应用后）】
%s

【第 %d 章正文内容】
%s

请严格对照 Old 状态卡、New 状态卡与正文内容，进行状态对账，并输出标准 JSON。`,
		chapterIndex,
		oldCard,
		string(mutJSON),
		newCard,
		chapterIndex,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, client, model, systemPrompt, userPrompt, 0.0, nil)
	if err != nil {
		return StateValidationResult{}, usage, fmt.Errorf("state validation LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var res StateValidationResult
	if err := json.Unmarshal([]byte(cleanJSON), &res); err != nil {
		return StateValidationResult{}, usage, fmt.Errorf("parse state validation JSON failed: %w (raw: %s)", err, resp)
	}

	kept, droppedCount := filterStateObservations(res.Observations, draftText)
	res.Observations = kept
	if droppedCount > 0 {
		res.Observations = append(res.Observations, StateValidationObservation{
			Code:    "unverifiable-observation",
			Summary: fmt.Sprintf("%d 条观察因引用无法在正文中验证已被丢弃", droppedCount),
		})
	}

	return res, usage, nil
}

// filterStateObservations 确定性过滤验证器观察：evidence 逐条要求
// 去首尾空白后 rune≥8 且为正文子串；观察的有效引用为空则整条丢弃。
// 返回保留的观察与丢弃条数。
func filterStateObservations(observations []StateValidationObservation, draftText string) (kept []StateValidationObservation, droppedCount int) {
	if len(observations) == 0 {
		return nil, 0
	}
	if draftText == "" {
		return nil, len(observations)
	}

	for _, obs := range observations {
		var validEvidence []string
		for _, ev := range obs.Evidence {
			trimmed := strings.TrimSpace(ev)
			if utf8.RuneCountInString(trimmed) >= 8 && strings.Contains(draftText, trimmed) {
				validEvidence = append(validEvidence, trimmed)
			}
		}
		if len(validEvidence) > 0 {
			obsCopy := obs
			obsCopy.Evidence = validEvidence
			kept = append(kept, obsCopy)
		} else {
			droppedCount++
		}
	}
	return kept, droppedCount
}

func renderCodexStateCard(e *domain.CodexEntry) string {
	if e == nil {
		return "【角色状态】无"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("角色: %s\n", baselineNameLevel(e)))
	b.WriteString(fmt.Sprintf("随身物品 (文本): %s\n", baselineInventory(e)))
	if len(e.StructuredItems) > 0 {
		var items []string
		for _, it := range e.StructuredItems {
			desc := fmt.Sprintf("%s x%d", it.Name, it.Quantity)
			if it.Quality != "" {
				desc += fmt.Sprintf(" (%s)", it.Quality)
			}
			items = append(items, desc)
		}
		b.WriteString(fmt.Sprintf("结构化物品: %s\n", strings.Join(items, "、")))
	}
	if e.CurrentDisposition != "" {
		b.WriteString(fmt.Sprintf("立场态势: %s\n", e.CurrentDisposition))
	}
	if e.HealthStatus != "" {
		b.WriteString(fmt.Sprintf("身心状态: %s\n", e.HealthStatus))
	}
	return strings.TrimSpace(b.String())
}
