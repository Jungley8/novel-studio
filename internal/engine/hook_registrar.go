package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Jungley8/novel-studio/internal/domain"
)

const (
	RejectionMissingTitle   = "missing-title"
	RejectionDuplicateTitle = "duplicate-title"
)

// ExtractPlantedHooks 从刚完成的章节正文中提取新埋下的伏笔候选（带正文证据引用）。
// 与手动提取（Orchestrator.ExtractPlotHooks）分离：单章、强制 quote 证据、由登记证据门二次校验。
func ExtractPlantedHooks(
	ctx context.Context,
	client LLMClient,
	model string,
	project *domain.Project,
	chapterIndex int,
	chapterText string,
	existingHooks []*domain.PlotHook,
) ([]domain.PlantedHookCandidate, TokenUsage, error) {
	if client == nil {
		return nil, TokenUsage{}, errors.New("llm client cannot be nil")
	}
	if project == nil {
		return nil, TokenUsage{}, errors.New("project cannot be nil")
	}
	if strings.TrimSpace(chapterText) == "" {
		return nil, TokenUsage{}, errors.New("chapter text cannot be empty")
	}

	var existingTitles []string
	for _, h := range existingHooks {
		if h == nil {
			continue
		}
		chap := h.TargetChapter
		if chap <= 0 {
			chap = h.CreatedChapter
		}
		existingTitles = append(existingTitles, fmt.Sprintf("- [%s] %s (第%d章)", h.Status, h.Title, chap))
	}
	existingStr := "暂无"
	if len(existingTitles) > 0 {
		existingStr = strings.Join(existingTitles, "\n")
	}

	systemPrompt := `你是一名网络小说伏笔登记员。
你的任务是从刚完成的章节正文中提取本章新埋下的长线伏笔候选（带正文证据引用）。

硬性要求：
1. 只登记不在【已有伏笔】列表中的新伏笔；若本章未埋下新的长线伏笔，必须返回空数组 []。
2. 每条伏笔必须包含 quote（正文原句引用），必须直接从正文中逐字复制且长度不少于 8 个字符。
3. target_chapter（建议回收章节号）必须大于当前章节号。
4. 最多提取 3 条新伏笔。

必须且仅输出 JSON 数组，格式如下：
[
  {
    "title": "伏笔标题（4-10字，简明概括）",
    "details": "具体线索细节与后续可能的引爆走向",
    "quote": "正文中的原句摘录（必须逐字匹配正文且≥8字）",
    "target_chapter": 建议回收章节号
  }
]`

	userPrompt := fmt.Sprintf(`【已有伏笔】
%s

【当前进行至】第 %d 章

【本章正文全文】
%s

请从本章正文中提取新埋下的伏笔候选（带正文逐字引用 quote），无则输出 []。输出 JSON 数组。`,
		existingStr,
		chapterIndex,
		chapterText,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := client.ChatCompletionWithUsage(ctxRole, model, systemPrompt, userPrompt, 0.4)
	if err != nil {
		return nil, usage, fmt.Errorf("extract planted hooks failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean hooks json failed: %w", err)
	}

	candidates, err := UnmarshalJSONArray[domain.PlantedHookCandidate](cleanJSON)
	if err != nil {
		return nil, usage, fmt.Errorf("unmarshal planted hooks failed: %w", err)
	}

	for i := range candidates {
		if candidates[i].TargetChapter <= chapterIndex {
			candidates[i].TargetChapter = chapterIndex + 5
		}
	}

	return candidates, usage, nil
}

// PlantedHookRejection 记录被登记证据门拒绝的伏笔候选。
type PlantedHookRejection struct {
	Candidate domain.PlantedHookCandidate
	Reason    string
}

// FilterPlantedHookCandidates 登记证据门（确定性、无 LLM 调用）：
// (a) title trim 后非空；(b) title 与已有伏笔（trim 精确匹配）及批内已通过者不重复；
// (c) quote trim 后 rune 数 ≥8 且为正文子串（与回收证据门同口径）。
// 通过者回写 trim 后的 Title/Quote。
func FilterPlantedHookCandidates(
	candidates []domain.PlantedHookCandidate,
	chapterText string,
	existingHooks []*domain.PlotHook,
) (valid []domain.PlantedHookCandidate, rejected []PlantedHookRejection) {
	if len(candidates) == 0 {
		return nil, nil
	}

	existingTitles := make(map[string]bool)
	for _, h := range existingHooks {
		if h != nil {
			t := strings.TrimSpace(h.Title)
			if t != "" {
				existingTitles[t] = true
			}
		}
	}

	passedTitles := make(map[string]bool)
	for _, c := range candidates {
		title := strings.TrimSpace(c.Title)
		if title == "" {
			rejected = append(rejected, PlantedHookRejection{
				Candidate: c,
				Reason:    RejectionMissingTitle,
			})
			continue
		}

		if existingTitles[title] || passedTitles[title] {
			rejected = append(rejected, PlantedHookRejection{
				Candidate: c,
				Reason:    RejectionDuplicateTitle,
			})
			continue
		}

		quote := strings.TrimSpace(c.Quote)
		if utf8.RuneCountInString(quote) < 8 {
			rejected = append(rejected, PlantedHookRejection{
				Candidate: c,
				Reason:    RejectionInsufficientEvidence,
			})
			continue
		}

		if !strings.Contains(chapterText, quote) {
			rejected = append(rejected, PlantedHookRejection{
				Candidate: c,
				Reason:    RejectionEvidenceNotInDraft,
			})
			continue
		}

		passedTitles[title] = true
		c.Title = title
		c.Quote = quote
		valid = append(valid, c)
	}

	return valid, rejected
}
