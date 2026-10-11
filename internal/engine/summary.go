package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// TODO(WIRE): Integration wire point for workshop.go (to be connected after rhythm merges).
// In ProduceChapter: after settler successfully finishes (SettleFromDraft) and before commit (CommitChapter):
// Call SummarizeChapter(ctx, client, model, project, chapterIndex, draftText, settledMutation)
// Save the resulting ChapterSummary into store via store.SaveChapterSummary.
// If the current chapter marks the end of a volume arc, trigger BuildVolumeRetrospective and persist via store.SaveVolumeRetrospective.

type ChapterSummary = domain.ChapterSummary
type VolumeRetrospective = domain.VolumeRetrospective
type VolumeArc = domain.VolumeArc

const (
	// MaxChapterSummaryRunes defines the strict hard limit for chapter summaries.
	MaxChapterSummaryRunes = 100
	// MaxVolumeRetrospectiveRunes defines the strict hard limit for volume retrospectives.
	MaxVolumeRetrospectiveRunes = 300
)

// SummarizeChapter synthesizes an ultra-dense chapter summary (<= 100 Chinese runes)
// with the 4 essential narrative elements (characters, conflict, outcome, unresolved suspense),
// rejecting any output that exceeds length or contains forbidden meta-narrative words.
func SummarizeChapter(
	ctx context.Context,
	client LLMClient,
	model string,
	project *domain.Project,
	chapterIndex int,
	draftText string,
	settledMutation domain.StateMutation,
) (ChapterSummary, TokenUsage, error) {
	if client == nil {
		return ChapterSummary{}, TokenUsage{}, errors.New("llm client cannot be nil")
	}
	if strings.TrimSpace(draftText) == "" {
		return ChapterSummary{}, TokenUsage{}, errors.New("draft text cannot be empty")
	}

	projectTitle := "小说项目"
	if project != nil && strings.TrimSpace(project.Title) != "" {
		projectTitle = strings.TrimSpace(project.Title)
	}

	inventoryDelta := settledMutation.InventoryDelta
	if strings.TrimSpace(inventoryDelta) == "" {
		inventoryDelta = "无"
	}
	powerDelta := settledMutation.PowerDelta
	if strings.TrimSpace(powerDelta) == "" {
		powerDelta = "无"
	}
	var charMutations []string
	for _, cm := range settledMutation.CharacterMutations {
		charMutations = append(charMutations, fmt.Sprintf("%s: %s (%s)", cm.Name, cm.StatusDelta, cm.RelationDelta))
	}
	charMutationsStr := "无"
	if len(charMutations) > 0 {
		charMutationsStr = strings.Join(charMutations, "; ")
	}

	systemPrompt := `你是一名网络小说严谨连载编剧与长期记忆架构师。
你的任务是根据章节正文内容以及已结算的状态变迁（StateMutation），提炼出严格在100字以内的极简章节剧情摘要。

核心铁律：
1. 压缩四要素：摘要必须完整涵盖四要素：【人物】（核心当事人）、【冲突】（主要矛盾/交锋）、【结果】（胜负/破局/代价）、【遗留悬念】（未决危机/伏笔悬念）。
2. 字数严格限制：摘要文本必须在 100 个汉字以内（包含标点），禁止超过100字！
3. 严禁元叙述词：禁止出现“本章”、“笔者”、“本回”、“故事中”等打破第四面墙的元叙述词，直接以客观叙事语言陈述剧情事实。
4. 唯正文主义：必须严格依据正文白纸黑字及状态变迁，严禁凭空脑补未发生的剧情。
5. 必须输出标准 JSON 格式：
{
  "characters": "核心人物",
  "conflict": "主要冲突",
  "result": "冲突结果",
  "suspense": "遗留悬念",
  "summary": "100字内包含人物、冲突、结果与遗留悬念四要素的连贯中文摘要"
}`

	userPrompt := fmt.Sprintf(`【书名】《%s》 第 %d 章
【状态变迁】道具变动: %s | 战力变动: %s | 角色变动: %s
【正文内容】
%s

请提炼严格在100字以内的章节摘要（含人物/冲突/结果/遗留悬念四要素，禁止出现“本章”“笔者”等词），输出标准 JSON。`,
		projectTitle,
		chapterIndex,
		inventoryDelta,
		powerDelta,
		charMutationsStr,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, client, model, systemPrompt, userPrompt, 0.2, nil)
	if err != nil {
		return ChapterSummary{}, usage, fmt.Errorf("summarize chapter LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return ChapterSummary{}, usage, fmt.Errorf("summarize chapter clean json failed: %w", err)
	}

	type chapterSummaryPayload struct {
		Summary    string `json:"summary"`
		Text       string `json:"text"`
		Characters string `json:"characters"`
		Conflict   string `json:"conflict"`
		Result     string `json:"result"`
		Suspense   string `json:"suspense"`
	}

	var payload chapterSummaryPayload
	if err := json.Unmarshal([]byte(cleanJSON), &payload); err != nil {
		return ChapterSummary{}, usage, fmt.Errorf("summarize chapter unmarshal failed: %w", err)
	}

	summaryText := strings.TrimSpace(payload.Summary)
	if summaryText == "" {
		summaryText = strings.TrimSpace(payload.Text)
	}
	if summaryText == "" && (payload.Characters != "" || payload.Conflict != "" || payload.Result != "" || payload.Suspense != "") {
		summaryText = strings.TrimSpace(fmt.Sprintf("%s遭遇%s，最终%s，%s", payload.Characters, payload.Conflict, payload.Result, payload.Suspense))
	}

	runes := utf8.RuneCountInString(summaryText)
	if runes == 0 {
		return ChapterSummary{}, usage, errors.New("empty chapter summary text")
	}
	if runes > MaxChapterSummaryRunes {
		return ChapterSummary{}, usage, fmt.Errorf("chapter summary exceeds %d runes (got %d): %s", MaxChapterSummaryRunes, runes, summaryText)
	}

	for _, banned := range []string{"本章", "笔者"} {
		if strings.Contains(summaryText, banned) {
			return ChapterSummary{}, usage, fmt.Errorf("chapter summary contains forbidden meta-narrative word %q", banned)
		}
	}

	projectID := ""
	if project != nil {
		projectID = project.ID
	}

	cs := ChapterSummary{
		ProjectID:    projectID,
		ChapterIndex: chapterIndex,
		Text:         summaryText,
		Model:        model,
		Tokens:       usage.TotalTokens,
		CreatedAt:    time.Now(),
	}
	return cs, usage, nil
}

// BuildVolumeRetrospective synthesizes a volume-level narrative review (<= 300 Chinese runes)
// covering what happened, what was left behind, and what unresolved hooks remain.
func BuildVolumeRetrospective(
	ctx context.Context,
	client LLMClient,
	model string,
	project *domain.Project,
	volume VolumeArc,
	span domain.VolumeSpan,
	summaries []ChapterSummary,
) (VolumeRetrospective, TokenUsage, error) {
	if client == nil {
		return VolumeRetrospective{}, TokenUsage{}, errors.New("llm client cannot be nil")
	}
	if len(summaries) == 0 {
		return VolumeRetrospective{}, TokenUsage{}, errors.New("summaries cannot be empty")
	}

	fromChapter, toChapter := span.StartChapter, span.EndChapter
	if fromChapter <= 0 || toChapter < fromChapter {
		if volume.StartChapter > 0 && volume.EndChapter >= volume.StartChapter {
			fromChapter = volume.StartChapter
			toChapter = volume.EndChapter
		} else if project != nil && project.Framework != nil && len(project.Framework.VolumeArcs) > 0 {
			normalized := BackfillVolumeArcs(project.Framework.VolumeArcs)
			spans, _ := deriveVolumeSpans(normalized)
			for _, s := range spans {
				if s.VolumeIndex == volume.VolumeIndex {
					fromChapter = s.StartChapter
					toChapter = s.EndChapter
					break
				}
			}
		}
	}
	if fromChapter <= 0 || toChapter < fromChapter {
		if len(summaries) > 0 {
			fromChapter = summaries[0].ChapterIndex
			toChapter = summaries[0].ChapterIndex
			for _, s := range summaries {
				if s.ChapterIndex < fromChapter {
					fromChapter = s.ChapterIndex
				}
				if s.ChapterIndex > toChapter {
					toChapter = s.ChapterIndex
				}
			}
		} else {
			est := volume.EstimatedChapters
			if est <= 0 {
				est = 40
			}
			idx := volume.VolumeIndex
			if idx <= 0 {
				idx = 1
			}
			fromChapter = (idx-1)*est + 1
			toChapter = fromChapter + est - 1
		}
	}

	var sumLines strings.Builder
	for _, s := range summaries {
		sumLines.WriteString(fmt.Sprintf("第 %d 章：%s\n", s.ChapterIndex, s.Text))
	}

	systemPrompt := `你是一名网络小说宏观剧情架构师与大卷剧情复盘专家。
你的任务是根据给定的本卷各章摘要与分卷设定，生成严格在300字以内的卷级剧情回顾（Volume Retrospective）。

核心要求：
1. 三段式结构：回顾内容必须清晰包含以下三段核心内容：
   - 【发生】：本卷发生了什么（主线发展、核心危机与高潮解决过程）；
   - 【留下】：给主角、阵营与世界格局留下了什么（战力突破、新盟友/宿敌、代价）；
   - 【未回收】：遗留或未回收的线索（未决伏笔、暗流危机、下一卷引子）。
2. 字数严格限制：总字数必须严格在 300 个汉字以内（包含标点），禁止超过 300 字！
3. 严禁元叙述词：禁止出现“本卷”、“本章”、“笔者”等元叙述词，直接以客观叙事语言陈述。
4. 必须输出标准 JSON 格式：
{
  "happened": "发生了什么描述",
  "remaining": "留下什么描述",
  "unresolved": "未回收什么描述",
  "text": "【发生】...\\n【留下】...\\n【未回收】..."
}`

	projectTitle := "小说项目"
	if project != nil && strings.TrimSpace(project.Title) != "" {
		projectTitle = strings.TrimSpace(project.Title)
	}

	userPrompt := fmt.Sprintf(`【书名】《%s》
【分卷】第 %d 卷：《%s》 | 卷主线：%s | 卷大高潮：%s
【章节区间】第 %d 章 ~ 第 %d 章
【各章摘要】
%s

请提炼本卷回顾（必须包含发生/留下/未回收三段，总字数300字内，禁止出现“本卷”“本章”“笔者”等词），输出标准 JSON。`,
		projectTitle,
		volume.VolumeIndex,
		volume.Title,
		volume.CoreGoal,
		volume.Climax,
		fromChapter,
		toChapter,
		sumLines.String(),
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := SafeChatCompletionWithStreamFallback(ctxRole, client, model, systemPrompt, userPrompt, 0.3, nil)
	if err != nil {
		return VolumeRetrospective{}, usage, fmt.Errorf("volume retrospective LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return VolumeRetrospective{}, usage, fmt.Errorf("volume retrospective clean json failed: %w", err)
	}

	type volumeRetrospectivePayload struct {
		Happened   string `json:"happened"`
		Remaining  string `json:"remaining"`
		Unresolved string `json:"unresolved"`
		Text       string `json:"text"`
	}

	var payload volumeRetrospectivePayload
	if err := json.Unmarshal([]byte(cleanJSON), &payload); err != nil {
		return VolumeRetrospective{}, usage, fmt.Errorf("volume retrospective unmarshal failed: %w", err)
	}

	retroText := strings.TrimSpace(payload.Text)
	if retroText != "" && strings.Contains(retroText, "发生") && strings.Contains(retroText, "留下") && strings.Contains(retroText, "未回收") {
		// Valid complete text from LLM
	} else if payload.Happened != "" || payload.Remaining != "" || payload.Unresolved != "" {
		retroText = fmt.Sprintf("【发生】%s\n【留下】%s\n【未回收】%s",
			strings.TrimSpace(payload.Happened),
			strings.TrimSpace(payload.Remaining),
			strings.TrimSpace(payload.Unresolved),
		)
	}

	runes := utf8.RuneCountInString(retroText)
	if runes == 0 {
		return VolumeRetrospective{}, usage, errors.New("empty volume retrospective text")
	}
	if runes > MaxVolumeRetrospectiveRunes {
		return VolumeRetrospective{}, usage, fmt.Errorf("volume retrospective exceeds %d runes (got %d): %s", MaxVolumeRetrospectiveRunes, runes, retroText)
	}

	if !strings.Contains(retroText, "发生") || !strings.Contains(retroText, "留下") || !strings.Contains(retroText, "未回收") {
		return VolumeRetrospective{}, usage, errors.New("volume retrospective must contain all three sections: 发生, 留下, 未回收")
	}

	for _, banned := range []string{"本卷", "本章", "笔者"} {
		if strings.Contains(retroText, banned) {
			return VolumeRetrospective{}, usage, fmt.Errorf("volume retrospective contains forbidden meta-narrative word %q", banned)
		}
	}

	projectID := ""
	if project != nil {
		projectID = project.ID
	}

	vr := VolumeRetrospective{
		ProjectID:   projectID,
		VolumeIndex: volume.VolumeIndex,
		Text:        retroText,
		FromChapter: fromChapter,
		ToChapter:   toChapter,
		CreatedAt:   time.Now(),
	}
	return vr, usage, nil
}
