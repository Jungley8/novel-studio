package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// decideVerdict applies unified Fail-Closed gating logic across all review seams:
// Passing strictly requires: LLM verdict == ACCEPTED && score >= 80 && zero banned AI cliches && non-excessive exclamations.
func decideVerdict(llmVerdict domain.ReviewVerdict, score int, heuristic domain.LinterResult) domain.ReviewVerdict {
	exclExcessive := strings.Contains(heuristic.Message, "感叹号过密") || (heuristic.Message == "" && heuristic.ExclamationDensity > 4.0)
	if llmVerdict == domain.ReviewVerdictAccepted && score >= 80 && len(heuristic.HitBannedWords) == 0 && !exclExcessive {
		return domain.ReviewVerdictAccepted
	}
	return domain.ReviewVerdictRevision
}

// DivisionName represents one of the three specialized judicial review chambers.
type DivisionName string

const (
	DivisionLogic      DivisionName = "LogicDivision"
	DivisionStyle      DivisionName = "StyleDivision"
	DivisionContinuity DivisionName = "ContinuityDivision"
)

// DivisionResult stores the auditing outcome from a single judicial division.
type DivisionResult struct {
	DivisionName    DivisionName         `json:"division_name"`
	Verdict         domain.ReviewVerdict `json:"verdict"`
	Score           int                  `json:"score"`
	Issues          []string             `json:"issues"`
	Suggestions     string               `json:"suggestions"`
	ResolvedHookIDs []string             `json:"resolved_hook_ids,omitempty"`
}

// CourtAuditReport encapsulates the synthesized outcome of the three-division review court,
// extending domain.AuditReport with multi-agent debate and minority dissent trails.
type CourtAuditReport struct {
	*domain.AuditReport
	Dissent      []string                  `json:"dissent,omitempty"`
	DebateRounds int                       `json:"debate_rounds"`
	Divisions    map[string]DivisionResult `json:"divisions"`
}

// CourtRecord is an auditable log entry of a full court deliberation for playback.
type CourtRecord struct {
	DeliberatedAt time.Time                 `json:"deliberated_at"`
	ChapterIndex  int                       `json:"chapter_index"`
	ProjectTitle  string                    `json:"project_title"`
	Verdict       domain.ReviewVerdict      `json:"verdict"`
	FinalScore    int                       `json:"final_score"`
	Dissent       []string                  `json:"dissent,omitempty"`
	DebateRounds  int                       `json:"debate_rounds"`
	Divisions     map[string]DivisionResult `json:"divisions"`
	Report        *domain.AuditReport       `json:"report"`
}

// ReviewCourt orchestrates multi-agent three-division judicial review of novel drafts.
// TODO(frontend): Expose ReviewCourt divisions, deliberation playback trail, and dissent records to Prompt Inspector UI.
type ReviewCourt struct {
	client          LLMClient
	linter          *Linter
	divisionTimeout time.Duration

	mu      sync.RWMutex
	records []CourtRecord
}

// NewReviewCourt constructs a ReviewCourt reusing an LLMClient and Linter.
func NewReviewCourt(client LLMClient, linter *Linter) *ReviewCourt {
	if linter == nil {
		linter = NewLinter(nil)
	}
	return &ReviewCourt{
		client:          client,
		linter:          linter,
		divisionTimeout: 45 * time.Second,
		records:         make([]CourtRecord, 0),
	}
}

// SetDivisionTimeout configures per-division timeout (default 45s).
func (c *ReviewCourt) SetDivisionTimeout(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.divisionTimeout = d
}

// History returns a copy of historical court deliberations for review playback.
func (c *ReviewCourt) History() []CourtRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]CourtRecord, len(c.records))
	copy(res, c.records)
	return res
}

// LastRecord returns the most recent court deliberation entry, or nil if none.
func (c *ReviewCourt) LastRecord() *CourtRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.records) == 0 {
		return nil
	}
	rec := c.records[len(c.records)-1]
	return &rec
}

func (c *ReviewCourt) recordDeliberation(rec CourtRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, rec)
}

// LogicDivision audits world axioms, scene beat causal fulfillment, and plot hook payoffs.
func (c *ReviewCourt) LogicDivision(
	ctx context.Context,
	model string,
	beats []domain.SceneBeat,
	hooks []*domain.PlotHook,
	draftText string,
) (*DivisionResult, TokenUsage, error) {
	systemPrompt := `你是一名极其严苛的网络小说逻辑分庭总审编（ReviewCourt LogicDivision）。
你的专职审查重点：
1. 世界法则与实体状态不变量：主角是否违背了世界法则公理？战力体系与境界是否出现崩坏矛盾？是否使用了未持有的物品？
2. 节拍因果履约：给定的剧情节拍核心动作和打破预期是否全部落实？情节因果链条是否完整，有无逻辑跳跃或机械降神？
3. 伏笔回收判定：正文是否回收或实质推进了给定的开放伏笔？若有，在 resolved_hook_ids 中给出已回收的伏笔 ID 列表。

必须且仅输出标准 JSON 格式：
{
  "verdict": "ACCEPTED" 或 "REVISION_NEEDED",
  "score": 1到100的整数 (80分及以上方可通过),
  "issues": ["逻辑硬伤或节拍履约问题1", "问题2"],
  "suggestions": "针对逻辑因果与伏笔回收的具体修改建议",
  "resolved_hook_ids": ["已在此章成功回收的伏笔ID列表，若无则为空数组 []"]
}`

	beatsJSON, _ := json.Marshal(beats)
	hooksContext := "暂无开放伏笔"
	if len(hooks) > 0 {
		var hList []string
		for _, h := range hooks {
			hList = append(hList, fmt.Sprintf("- [%s] ID:%s | 标题:《%s》 | 目标章节:第%d章 | 详情:%s", h.Status, h.ID, h.Title, h.TargetChapter, h.Details))
		}
		hooksContext = strings.Join(hList, "\n")
	}

	userPrompt := fmt.Sprintf(`【审查分庭】逻辑庭 (LogicDivision)
【要求履行的节拍】%s
【参考开放伏笔】
%s

【待审正文草稿】
%s

请给出逻辑庭审阅判决。`,
		string(beatsJSON),
		hooksContext,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := c.client.ChatCompletionWithUsage(ctxRole, model, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, usage, fmt.Errorf("logic division LLM review failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var out struct {
		Verdict         domain.ReviewVerdict `json:"verdict"`
		Score           int                  `json:"score"`
		Issues          []string             `json:"issues"`
		Suggestions     string               `json:"suggestions"`
		ResolvedHookIDs []string             `json:"resolved_hook_ids"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, usage, fmt.Errorf("parse logic division JSON failed (raw: %s): %w", resp, err)
	}

	return &DivisionResult{
		DivisionName:    DivisionLogic,
		Verdict:         out.Verdict,
		Score:           out.Score,
		Issues:          out.Issues,
		Suggestions:     out.Suggestions,
		ResolvedHookIDs: out.ResolvedHookIDs,
	}, usage, nil
}

// StyleDivision audits anti-AI patterns, character vocal tone, and algorithmic statistical indicators.
func (c *ReviewCourt) StyleDivision(
	ctx context.Context,
	model string,
	linterResult domain.LinterResult,
	draftText string,
) (*DivisionResult, TokenUsage, error) {
	systemPrompt := `你是一名极其严苛的网络小说风格与声口分庭总审编（ReviewCourt StyleDivision）。
你的专职审查重点：
1. 反AI味声口：句式是否有起伏？是否存在“不由得”、“嘴角勾起”等AI模式化套词？是否有空洞浮夸的悬浮情绪或模式化转折？
2. 角色声口与对峙质感：各角色台词风格是否鲜活、符合身份与性格？动作对峙是否真实有张力？
3. 算法指标复核：结合底层算法质检预警（突发度、对话比、感叹号密度、高频短语等），复核行文节奏与文学质感。

必须且仅输出标准 JSON 格式：
{
  "verdict": "ACCEPTED" 或 "REVISION_NEEDED",
  "score": 1到100的整数 (80分及以上方可通过),
  "issues": ["风格问题1", "问题2"],
  "suggestions": "针对反AI味、句长节奏与角色声口修改建议"
}`

	var heuristicList []string
	if len(linterResult.HitBannedWords) > 0 {
		heuristicList = append(heuristicList, fmt.Sprintf("检出AI套词: %s", strings.Join(linterResult.HitBannedWords, ", ")))
	}
	if linterResult.BurstinessScore < 40 {
		heuristicList = append(heuristicList, fmt.Sprintf("句长节奏过于平缓 (突发度: %d/100)", linterResult.BurstinessScore))
	}
	if linterResult.ExclamationDensity > 4.0 || strings.Contains(linterResult.Message, "感叹号") {
		heuristicList = append(heuristicList, fmt.Sprintf("感叹号过密 (每千字 %.1f 个)", linterResult.ExclamationDensity))
	}
	if len(linterResult.TopRepeatedNgrams) > 0 {
		heuristicList = append(heuristicList, fmt.Sprintf("高频重复短语: %s", strings.Join(linterResult.TopRepeatedNgrams, ", ")))
	}
	if linterResult.DialogueRatio > 0 && (linterResult.DialogueRatio < 0.10 || linterResult.DialogueRatio > 0.65) {
		heuristicList = append(heuristicList, fmt.Sprintf("对话占比异常 (%.1f%%)", linterResult.DialogueRatio*100))
	}
	heuristicNotes := "各项算法指标优良"
	if len(heuristicList) > 0 {
		heuristicNotes = strings.Join(heuristicList, "; ")
	}

	userPrompt := fmt.Sprintf(`【审查分庭】风格庭 (StyleDivision)
【底层算法质检预警与指标】
%s

【待审正文草稿】
%s

请给出风格庭审阅判决。`,
		heuristicNotes,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := c.client.ChatCompletionWithUsage(ctxRole, model, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, usage, fmt.Errorf("style division LLM review failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var out struct {
		Verdict     domain.ReviewVerdict `json:"verdict"`
		Score       int                  `json:"score"`
		Issues      []string             `json:"issues"`
		Suggestions string               `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, usage, fmt.Errorf("parse style division JSON failed (raw: %s): %w", resp, err)
	}

	return &DivisionResult{
		DivisionName: DivisionStyle,
		Verdict:      out.Verdict,
		Score:        out.Score,
		Issues:       out.Issues,
		Suggestions:  out.Suggestions,
	}, usage, nil
}

// ContinuityDivision audits cross-volume callback consistency, TailAnchor prose drift, and state invariants.
func (c *ReviewCourt) ContinuityDivision(
	ctx context.Context,
	model string,
	horizonSummary string,
	draftText string,
) (*DivisionResult, TokenUsage, error) {
	systemPrompt := `你是一名极其严苛的网络小说连续性与状态机分庭总审编（ReviewCourt ContinuityDivision）。
你的专职审查重点：
1. 跨卷回调一致性：正文与前序历史脉络、宏观大纲、人物命运走向、主线目标是否保持一致？有无背离核心设定？
2. TailAnchor 文风与腔调漂移：若提供了上一章末尾文风锚定（TailAnchor），正文开头与视角是否顺畅承接？文风与叙事腔调是否发生突兀断裂或漂移？
3. 状态机不变量：主角状态、势力关系、重要信物与因果线是否维持合法转移，无空降物品或凭空遗忘？

必须且仅输出标准 JSON 格式：
{
  "verdict": "ACCEPTED" 或 "REVISION_NEEDED",
  "score": 1到100的整数 (80分及以上方可通过),
  "issues": ["连续性或文风漂移问题1", "问题2"],
  "suggestions": "针对前后文承接、状态转移与文风锚定的修改建议"
}`

	if strings.TrimSpace(horizonSummary) == "" {
		horizonSummary = "无特定正史视界摘要"
	}

	userPrompt := fmt.Sprintf(`【审查分庭】连续性庭 (ContinuityDivision)
【正史视界摘要与锚定设定】
%s

【待审正文草稿】
%s

请给出连续性庭审阅判决。`,
		horizonSummary,
		draftText,
	)

	ctxRole := ContextWithRole(ctx, RoleReviewer)
	resp, usage, err := c.client.ChatCompletionWithUsage(ctxRole, model, systemPrompt, userPrompt, 0.3)
	if err != nil {
		return nil, usage, fmt.Errorf("continuity division LLM review failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var out struct {
		Verdict     domain.ReviewVerdict `json:"verdict"`
		Score       int                  `json:"score"`
		Issues      []string             `json:"issues"`
		Suggestions string               `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &out); err != nil {
		return nil, usage, fmt.Errorf("parse continuity division JSON failed (raw: %s): %w", resp, err)
	}

	return &DivisionResult{
		DivisionName: DivisionContinuity,
		Verdict:      out.Verdict,
		Score:        out.Score,
		Issues:       out.Issues,
		Suggestions:  out.Suggestions,
	}, usage, nil
}

// ChiefJudgeOptions provides optional contextual parameters for ChiefJudge deliberation.
type ChiefJudgeOptions struct {
	HorizonSummary string
	TailAnchor     string
}

// ChiefJudge executes concurrent three-division review under an errgroup with per-division timeouts.
// Any division voting REVISION_NEEDED enforces an overall REVISION_NEEDED verdict (Fail-Closed collegial deliberation).
// The overall score is the minimum of the three divisions (penalized by heuristic anomalies).
// Dissenting approvals are recorded in Dissent.
func (c *ReviewCourt) ChiefJudge(
	ctx context.Context,
	reviewerModel string,
	project *domain.Project,
	chapterIndex int,
	beats []domain.SceneBeat,
	activeHooks []*domain.PlotHook,
	draftText string,
	opts ...ChiefJudgeOptions,
) (*CourtAuditReport, TokenUsage, error) {
	// 1. Algorithmic pre-pass
	heuristic := c.linter.Analyze(draftText)

	// 2. Synthesize horizon summary for ContinuityDivision
	var horizonSummary string
	if len(opts) > 0 && opts[0].HorizonSummary != "" {
		horizonSummary = opts[0].HorizonSummary
	} else {
		horizonSummary = buildDefaultHorizonSummary(project, chapterIndex, activeHooks)
	}
	if len(opts) > 0 && opts[0].TailAnchor != "" {
		horizonSummary += fmt.Sprintf("\n【上章末尾文风锚定 (TailAnchor)】\n%s", opts[0].TailAnchor)
	}

	// 3. Concurrently invoke three divisions via errGroup with independent timeouts
	c.mu.RLock()
	timeout := c.divisionTimeout
	c.mu.RUnlock()
	if timeout <= 0 {
		timeout = 45 * time.Second
	}

	g, gCtx := newCourtErrGroup(ctx)

	var (
		mu              sync.Mutex
		logicRes        *DivisionResult
		logicUsage      TokenUsage
		styleRes        *DivisionResult
		styleUsage      TokenUsage
		continuityRes   *DivisionResult
		continuityUsage TokenUsage
	)

	// Division 1: LogicDivision
	g.Go(func() error {
		divCtx, cancel := context.WithTimeout(gCtx, timeout)
		defer cancel()
		res, usage, err := c.LogicDivision(divCtx, reviewerModel, beats, activeHooks, draftText)
		if err != nil {
			return fmt.Errorf("logic division failed: %w", err)
		}
		mu.Lock()
		logicRes = res
		logicUsage = usage
		mu.Unlock()
		return nil
	})

	// Division 2: StyleDivision
	g.Go(func() error {
		divCtx, cancel := context.WithTimeout(gCtx, timeout)
		defer cancel()
		res, usage, err := c.StyleDivision(divCtx, reviewerModel, heuristic, draftText)
		if err != nil {
			return fmt.Errorf("style division failed: %w", err)
		}
		mu.Lock()
		styleRes = res
		styleUsage = usage
		mu.Unlock()
		return nil
	})

	// Division 3: ContinuityDivision
	g.Go(func() error {
		divCtx, cancel := context.WithTimeout(gCtx, timeout)
		defer cancel()
		res, usage, err := c.ContinuityDivision(divCtx, reviewerModel, horizonSummary, draftText)
		if err != nil {
			return fmt.Errorf("continuity division failed: %w", err)
		}
		mu.Lock()
		continuityRes = res
		continuityUsage = usage
		mu.Unlock()
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, TokenUsage{}, fmt.Errorf("review court deliberation error: %w", err)
	}

	totalUsage := TokenUsage{
		PromptTokens:     logicUsage.PromptTokens + styleUsage.PromptTokens + continuityUsage.PromptTokens,
		CompletionTokens: logicUsage.CompletionTokens + styleUsage.CompletionTokens + continuityUsage.CompletionTokens,
		TotalTokens:      logicUsage.TotalTokens + styleUsage.TotalTokens + continuityUsage.TotalTokens,
	}

	// 4. Fail-Closed Collegial Consensus:
	// Minimum of three scores is taken as terminal score
	minScore := logicRes.Score
	if styleRes.Score < minScore {
		minScore = styleRes.Score
	}
	if continuityRes.Score < minScore {
		minScore = continuityRes.Score
	}

	// Fail-Closed collegial veto: Any single division voting REVISION_NEEDED enforces REVISION_NEEDED
	var courtLLMVerdict domain.ReviewVerdict
	if logicRes.Verdict == domain.ReviewVerdictRevision ||
		styleRes.Verdict == domain.ReviewVerdictRevision ||
		continuityRes.Verdict == domain.ReviewVerdictRevision {
		courtLLMVerdict = domain.ReviewVerdictRevision
	} else {
		courtLLMVerdict = domain.ReviewVerdictAccepted
	}

	// Record minority dissenting approvals when court rejects
	var dissent []string
	if courtLLMVerdict == domain.ReviewVerdictRevision {
		if logicRes.Verdict == domain.ReviewVerdictAccepted {
			dissent = append(dissent, fmt.Sprintf("[逻辑庭] 主张放行 (评分 %d): %s", logicRes.Score, logicRes.Suggestions))
		}
		if styleRes.Verdict == domain.ReviewVerdictAccepted {
			dissent = append(dissent, fmt.Sprintf("[风格庭] 主张放行 (评分 %d): %s", styleRes.Score, styleRes.Suggestions))
		}
		if continuityRes.Verdict == domain.ReviewVerdictAccepted {
			dissent = append(dissent, fmt.Sprintf("[连续性庭] 主张放行 (评分 %d): %s", continuityRes.Score, continuityRes.Suggestions))
		}
	}

	// 5. Apply heuristic score penalties & run decideVerdict
	finalScore := minScore
	if len(heuristic.HitBannedWords) > 0 {
		finalScore -= len(heuristic.HitBannedWords) * 4
	}
	exclExcessive := IsExclamationExcessive(draftText, heuristic.ExclamationDensity)
	if exclExcessive {
		finalScore -= 6
	}
	if len(heuristic.TopRepeatedNgrams) >= 3 {
		finalScore -= 4
	}
	if finalScore < 0 {
		finalScore = 0
	}

	finalVerdict := decideVerdict(courtLLMVerdict, finalScore, heuristic)

	// 6. Synthesize comprehensive issues
	var allIssues []string
	if len(heuristic.HitBannedWords) > 0 {
		allIssues = append(allIssues, fmt.Sprintf("命中AI套词: %s", strings.Join(heuristic.HitBannedWords, ", ")))
	}
	if heuristic.BurstinessScore < 35 {
		allIssues = append(allIssues, fmt.Sprintf("句式过于单一平缓 (突发度 %d 分)", heuristic.BurstinessScore))
	}
	if exclExcessive {
		allIssues = append(allIssues, fmt.Sprintf("感叹号严重超标 (每千字 %.1f 个)", heuristic.ExclamationDensity))
	}
	if len(heuristic.TopRepeatedNgrams) >= 3 {
		allIssues = append(allIssues, fmt.Sprintf("机械性重复短语: %s", strings.Join(heuristic.TopRepeatedNgrams, ", ")))
	}

	for _, issue := range logicRes.Issues {
		allIssues = append(allIssues, fmt.Sprintf("[逻辑庭] %s", issue))
	}
	for _, issue := range styleRes.Issues {
		allIssues = append(allIssues, fmt.Sprintf("[风格庭] %s", issue))
	}
	for _, issue := range continuityRes.Issues {
		allIssues = append(allIssues, fmt.Sprintf("[连续性庭] %s", issue))
	}

	// Synthesize suggestions
	var suggestionParts []string
	if logicRes.Suggestions != "" {
		suggestionParts = append(suggestionParts, fmt.Sprintf("【逻辑庭建议】%s", logicRes.Suggestions))
	}
	if styleRes.Suggestions != "" {
		suggestionParts = append(suggestionParts, fmt.Sprintf("【风格庭建议】%s", styleRes.Suggestions))
	}
	if continuityRes.Suggestions != "" {
		suggestionParts = append(suggestionParts, fmt.Sprintf("【连续性庭建议】%s", continuityRes.Suggestions))
	}

	// Collect resolved hooks
	resolvedHookIDs := logicRes.ResolvedHookIDs

	report := &domain.AuditReport{
		Verdict:            finalVerdict,
		Score:              finalScore,
		BurstinessScore:    heuristic.BurstinessScore,
		HitBannedWords:     heuristic.HitBannedWords,
		DialogueRatio:      heuristic.DialogueRatio,
		ParagraphVariance:  heuristic.ParagraphVariance,
		TopRepeatedNgrams:  heuristic.TopRepeatedNgrams,
		ExclamationDensity: heuristic.ExclamationDensity,
		Issues:             allIssues,
		Suggestions:        strings.Join(suggestionParts, "\n"),
		ResolvedHookIDs:    resolvedHookIDs,
		ReviewedAt:         time.Now(),
	}

	divisionsMap := map[string]DivisionResult{
		string(DivisionLogic):      *logicRes,
		string(DivisionStyle):      *styleRes,
		string(DivisionContinuity): *continuityRes,
	}

	courtReport := &CourtAuditReport{
		AuditReport:  report,
		Dissent:      dissent,
		DebateRounds: 1,
		Divisions:    divisionsMap,
	}

	// 7. Record deliberation for playback
	var projTitle string
	if project != nil {
		projTitle = project.Title
	}
	c.recordDeliberation(CourtRecord{
		DeliberatedAt: time.Now(),
		ChapterIndex:  chapterIndex,
		ProjectTitle:  projTitle,
		Verdict:       finalVerdict,
		FinalScore:    finalScore,
		Dissent:       dissent,
		DebateRounds:  1,
		Divisions:     divisionsMap,
		Report:        report,
	})

	return courtReport, totalUsage, nil
}

func buildDefaultHorizonSummary(project *domain.Project, chapterIndex int, hooks []*domain.PlotHook) string {
	if project == nil {
		return "无正史视界摘要"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("书名: 《%s》 | 目标章节: 第 %d 章\n", project.Title, chapterIndex))
	if project.TargetPlatform != "" {
		sb.WriteString(fmt.Sprintf("目标平台: %s\n", project.TargetPlatform))
	}
	if project.WorldRules != "" {
		sb.WriteString(fmt.Sprintf("世界法则公理: %s\n", project.WorldRules))
	}
	if project.Protagonist.NameAndLevel != "" {
		sb.WriteString(fmt.Sprintf("主角当前状态: %s | 物品栏: %s\n", project.Protagonist.NameAndLevel, project.Protagonist.Inventory))
	}
	if len(hooks) > 0 {
		sb.WriteString("当前活跃伏笔:\n")
		for _, h := range hooks {
			sb.WriteString(fmt.Sprintf("- [%s] ID:%s | 《%s》| 目标:第%d章 | %s\n", h.Status, h.ID, h.Title, h.TargetChapter, h.Details))
		}
	}
	return sb.String()
}

// courtErrGroup coordinates goroutines and cancels their shared context on first error.
type courtErrGroup struct {
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

func newCourtErrGroup(ctx context.Context) (*courtErrGroup, context.Context) {
	cCtx, cancel := context.WithCancel(ctx)
	return &courtErrGroup{cancel: cancel}, cCtx
}

func (g *courtErrGroup) Go(f func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}

func (g *courtErrGroup) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}
