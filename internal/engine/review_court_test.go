package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type divisionMockClient struct {
	logicResp      string
	styleResp      string
	continuityResp string
}

func (m *divisionMockClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	resp, _, err := m.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temperature)
	return resp, err
}

func (m *divisionMockClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	select {
	case <-ctx.Done():
		return "", TokenUsage{}, ctx.Err()
	default:
	}

	var resp string
	if strings.Contains(userPrompt, "LogicDivision") {
		resp = m.logicResp
	} else if strings.Contains(userPrompt, "StyleDivision") {
		resp = m.styleResp
	} else if strings.Contains(userPrompt, "ContinuityDivision") {
		resp = m.continuityResp
	} else {
		resp = m.logicResp
	}

	return resp, TokenUsage{PromptTokens: 50, CompletionTokens: 50, TotalTokens: 100}, nil
}

func (m *divisionMockClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	resp, _, err := m.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: resp}
	ch <- StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

// TestReviewCourt_ChiefJudge_FailClosedGuard is the critical guard test verifying Fail-Closed collegial veto:
// When 2 divisions vote ACCEPTED (scores 90, 88) and 1 division votes REVISION_NEEDED (score 70),
// the court MUST reject with REVISION_NEEDED, take min score (70), and record dissent from the passing divisions.
func TestReviewCourt_ChiefJudge_FailClosedGuard(t *testing.T) {
	mock := &divisionMockClient{
		logicResp: `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "逻辑因果严密，节拍履约良好",
			"resolved_hooks": [{"id": "hook-101", "quote": "长街风冷。林玄按住剑柄，剑光骤然暴起"}]
		}`,
		styleResp: `{
			"verdict": "REVISION_NEEDED",
			"score": 82,
			"issues": ["台词悬浮，角色声口千人一面"],
			"suggestions": "强化各角色不同口癖与动作对峙"
		}`,
		continuityResp: `{
			"verdict": "ACCEPTED",
			"score": 88,
			"issues": [],
			"suggestions": "承接前文顺畅"
		}`,
	}

	court := NewReviewCourt(mock, nil)
	proj := &domain.Project{Title: "剑噬诸天", WorldRules: "剑道独尊"}
	beats := []domain.SceneBeat{{Action: "主角拔剑破局"}}
	hooks := []*domain.PlotHook{{ID: "hook-101", Title: "残铁伏笔"}}
	draft := "长街风冷。林玄按住剑柄，剑光骤然暴起，撕裂长空！"

	report, usage, err := court.ChiefJudge(context.Background(), "mock-reviewer", proj, nil, 1, beats, hooks, draft)
	if err != nil {
		t.Fatalf("ChiefJudge failed: %v", err)
	}

	// 1. Guard check: Single division REVISION_NEEDED MUST cause total REVISION_NEEDED
	if report.Verdict != domain.ReviewVerdictRevision {
		t.Fatalf("GUARD FAILED: expected REVISION_NEEDED when StyleDivision rejected, but got %s", report.Verdict)
	}

	// 2. Score must be minimum of three divisions (82)
	if report.Score != 82 {
		t.Errorf("expected minimum score 82, got %d", report.Score)
	}

	// 3. Dissent must record the 2 passing divisions
	if len(report.Dissent) != 2 {
		t.Errorf("expected 2 dissenting approvals in minority, got %d: %v", len(report.Dissent), report.Dissent)
	}
	foundLogicDissent := false
	foundContDissent := false
	for _, d := range report.Dissent {
		if strings.Contains(d, "逻辑庭") {
			foundLogicDissent = true
		}
		if strings.Contains(d, "连续性庭") {
			foundContDissent = true
		}
	}
	if !foundLogicDissent || !foundContDissent {
		t.Errorf("expected dissent from Logic and Continuity divisions, got %v", report.Dissent)
	}

	// 4. Token usage must be positive and internally consistent (prompt+completion==total).
	// Divisions go through SafeChatCompletionWithStreamFallback, which estimates
	// tokens from text length when the stream carries no usage payload.
	if usage.TotalTokens <= 0 || usage.PromptTokens+usage.CompletionTokens != usage.TotalTokens {
		t.Errorf("inconsistent token accounting: %+v", usage)
	}

	// 5. Hook resolution preserved from LogicDivision
	if len(report.ResolvedHookIDs) != 1 || report.ResolvedHookIDs[0] != "hook-101" {
		t.Errorf("expected resolved hook-101, got %v", report.ResolvedHookIDs)
	}
	if len(report.ResolvedHookClaims) != 1 || report.ResolvedHookClaims[0].ID != "hook-101" {
		t.Errorf("expected resolved hook claim hook-101, got %v", report.ResolvedHookClaims)
	}
}

// TestReviewCourt_ChiefJudge_UnanimousAccept verifies that when all three divisions agree on ACCEPTED >= 80,
// and algorithmic metrics pass, the draft is ACCEPTED with empty Dissent.
func TestReviewCourt_ChiefJudge_UnanimousAccept(t *testing.T) {
	mock := &divisionMockClient{
		logicResp: `{
			"verdict": "ACCEPTED",
			"score": 85,
			"issues": [],
			"suggestions": "逻辑顺畅",
			"resolved_hooks": []
		}`,
		styleResp: `{
			"verdict": "ACCEPTED",
			"score": 92,
			"issues": [],
			"suggestions": "反AI声口优良"
		}`,
		continuityResp: `{
			"verdict": "ACCEPTED",
			"score": 88,
			"issues": [],
			"suggestions": "无文风漂移"
		}`,
	}

	court := NewReviewCourt(mock, nil)
	proj := &domain.Project{Title: "万古天帝"}
	draft := "风雷阵阵。陆青立于山巅之上，俯瞰万丈深渊，指尖雷光游走。"

	report, usage, err := court.ChiefJudge(context.Background(), "mock-reviewer", proj, nil, 2, nil, nil, draft)
	if err != nil {
		t.Fatalf("ChiefJudge failed: %v", err)
	}

	if report.Verdict != domain.ReviewVerdictAccepted {
		t.Errorf("expected ACCEPTED, got %s", report.Verdict)
	}
	if report.Score != 85 { // min(85, 92, 88)
		t.Errorf("expected minimum score 85, got %d", report.Score)
	}
	if len(report.Dissent) != 0 {
		t.Errorf("expected empty dissent on unanimous accept, got %v", report.Dissent)
	}
	if usage.TotalTokens <= 0 || usage.PromptTokens+usage.CompletionTokens != usage.TotalTokens {
		t.Errorf("inconsistent token accounting: %+v", usage)
	}
}

// TestReviewCourt_ChiefJudge_HeuristicBannedWordsVeto verifies that even if all LLM divisions vote ACCEPTED,
// AI cliches detected by the linter strictly trigger Fail-Closed REVISION_NEEDED.
func TestReviewCourt_ChiefJudge_HeuristicBannedWordsVeto(t *testing.T) {
	mock := &divisionMockClient{
		logicResp:      `{"verdict": "ACCEPTED", "score": 95, "issues": [], "suggestions": "完美"}`,
		styleResp:      `{"verdict": "ACCEPTED", "score": 95, "issues": [], "suggestions": "完美"}`,
		continuityResp: `{"verdict": "ACCEPTED", "score": 95, "issues": [], "suggestions": "完美"}`,
	}

	court := NewReviewCourt(mock, nil)
	proj := &domain.Project{Title: "测试"}
	// Text containing banned cliches
	clicheDraft := "他不由得冷笑了一声，嘴角勾起一丝嘲讽的笑意。"

	report, _, err := court.ChiefJudge(context.Background(), "mock-reviewer", proj, nil, 1, nil, nil, clicheDraft)
	if err != nil {
		t.Fatalf("ChiefJudge failed: %v", err)
	}

	if report.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected Fail-Closed REVISION_NEEDED due to banned clichés, got %s", report.Verdict)
	}
	if len(report.HitBannedWords) == 0 {
		t.Errorf("expected HitBannedWords to be populated")
	}
}

// TestReviewCourt_ChiefJudge_ScoreUnder80Veto verifies that if the minimum score drops below 80,
// the verdict is strictly REVISION_NEEDED even if LLM stated ACCEPTED.
func TestReviewCourt_ChiefJudge_ScoreUnder80Veto(t *testing.T) {
	mock := &divisionMockClient{
		logicResp:      `{"verdict": "ACCEPTED", "score": 85, "issues": []}`,
		styleResp:      `{"verdict": "ACCEPTED", "score": 75, "issues": ["略显单调"]}`,
		continuityResp: `{"verdict": "ACCEPTED", "score": 88, "issues": []}`,
	}

	court := NewReviewCourt(mock, nil)
	draft := "长夜漫漫，寒星孤悬。道人闭目盘坐，静候天明。"

	report, _, err := court.ChiefJudge(context.Background(), "mock-reviewer", nil, nil, 1, nil, nil, draft)
	if err != nil {
		t.Fatalf("ChiefJudge failed: %v", err)
	}

	if report.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected REVISION_NEEDED when min score is 75 < 80, got %s", report.Verdict)
	}
	if report.Score != 75 {
		t.Errorf("expected score 75, got %d", report.Score)
	}
}

// TestReviewCourt_PlaybackHistory verifies that every deliberation is recorded with divisions,
// dissent, verdict, and final score for full replayability.
func TestReviewCourt_PlaybackHistory(t *testing.T) {
	mock := &divisionMockClient{
		logicResp:      `{"verdict": "ACCEPTED", "score": 90, "issues": []}`,
		styleResp:      `{"verdict": "REVISION_NEEDED", "score": 70, "issues": ["风格问题"]}`,
		continuityResp: `{"verdict": "ACCEPTED", "score": 85, "issues": []}`,
	}

	court := NewReviewCourt(mock, nil)
	proj := &domain.Project{Title: "遮天蔽日"}
	draft := "风云激荡。林凡傲立长空。"

	_, _, err := court.ChiefJudge(context.Background(), "mock-reviewer", proj, nil, 5, nil, nil, draft)
	if err != nil {
		t.Fatalf("ChiefJudge failed: %v", err)
	}

	history := court.History()
	if len(history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history))
	}
	last := court.LastRecord()
	if last == nil {
		t.Fatalf("expected last record to be non-nil")
	}

	if last.ProjectTitle != "遮天蔽日" {
		t.Errorf("expected project title '遮天蔽日', got %s", last.ProjectTitle)
	}
	if last.ChapterIndex != 5 {
		t.Errorf("expected chapter index 5, got %d", last.ChapterIndex)
	}
	if last.Verdict != domain.ReviewVerdictRevision {
		t.Errorf("expected recorded verdict REVISION_NEEDED, got %s", last.Verdict)
	}
	if len(last.Divisions) != 3 {
		t.Errorf("expected 3 division records, got %d", len(last.Divisions))
	}
	if len(last.Dissent) != 2 {
		t.Errorf("expected 2 dissent records in playback, got %d", len(last.Dissent))
	}
}

// TestReviewCourt_IndividualDivisions tests each division method independently.
func TestReviewCourt_IndividualDivisions(t *testing.T) {
	mock := &divisionMockClient{
		logicResp: `{
			"verdict": "ACCEPTED",
			"score": 88,
			"issues": ["伏笔暂未闭环"],
			"suggestions": "可在下一章展开",
			"resolved_hooks": [{"id": "h1", "quote": "草稿文本内容足够八个字以上"}]
		}`,
		styleResp: `{
			"verdict": "ACCEPTED",
			"score": 85,
			"issues": ["句式平实"],
			"suggestions": "丰富动词表现力"
		}`,
		continuityResp: `{
			"verdict": "ACCEPTED",
			"score": 90,
			"issues": [],
			"suggestions": "文风承接自然"
		}`,
	}

	court := NewReviewCourt(mock, nil)
	ctx := context.Background()

	// LogicDivision
	lRes, lUsage, err := court.LogicDivision(ctx, "mock", nil, nil, nil, "草稿文本...")
	if err != nil {
		t.Fatalf("LogicDivision failed: %v", err)
	}
	if lRes.DivisionName != DivisionLogic || lRes.Verdict != domain.ReviewVerdictAccepted || lRes.Score != 88 {
		t.Errorf("unexpected LogicDivision result: %+v", lRes)
	}
	if len(lRes.ResolvedHooks) != 1 || lRes.ResolvedHooks[0].ID != "h1" {
		t.Errorf("unexpected resolved hooks: %v", lRes.ResolvedHooks)
	}
	if lUsage.TotalTokens <= 0 || lUsage.PromptTokens+lUsage.CompletionTokens != lUsage.TotalTokens {
		t.Errorf("inconsistent logic division token accounting: %+v", lUsage)
	}

	// StyleDivision
	sRes, sUsage, err := court.StyleDivision(ctx, "mock", domain.LinterResult{BurstinessScore: 50}, "草稿文本...")
	if err != nil {
		t.Fatalf("StyleDivision failed: %v", err)
	}
	if sRes.DivisionName != DivisionStyle || sRes.Verdict != domain.ReviewVerdictAccepted || sRes.Score != 85 {
		t.Errorf("unexpected StyleDivision result: %+v", sRes)
	}
	if sUsage.TotalTokens <= 0 || sUsage.PromptTokens+sUsage.CompletionTokens != sUsage.TotalTokens {
		t.Errorf("inconsistent style division token accounting: %+v", sUsage)
	}

	// ContinuityDivision
	cRes, cUsage, err := court.ContinuityDivision(ctx, "mock", "前文摘要", "", "草稿文本...")
	if err != nil {
		t.Fatalf("ContinuityDivision failed: %v", err)
	}
	if cRes.DivisionName != DivisionContinuity || cRes.Verdict != domain.ReviewVerdictAccepted || cRes.Score != 90 {
		t.Errorf("unexpected ContinuityDivision result: %+v", cRes)
	}
	if cUsage.TotalTokens <= 0 || cUsage.PromptTokens+cUsage.CompletionTokens != cUsage.TotalTokens {
		t.Errorf("inconsistent continuity division token accounting: %+v", cUsage)
	}
}

// TestReviewCourt_ContextCancel verifies clean early exit on context cancel without goroutine leaks.
func TestReviewCourt_ContextCancel(t *testing.T) {
	mock := &divisionMockClient{
		logicResp:      `{"verdict": "ACCEPTED", "score": 90}`,
		styleResp:      `{"verdict": "ACCEPTED", "score": 90}`,
		continuityResp: `{"verdict": "ACCEPTED", "score": 90}`,
	}

	court := NewReviewCourt(mock, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, _, err := court.ChiefJudge(ctx, "mock", nil, nil, 1, nil, nil, "正文...")
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}

// TestDecideVerdict_TableDriven exhaustively verifies all Fail-Closed gating branches.
func TestDecideVerdict_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		llmVerdict domain.ReviewVerdict
		score      int
		draftText  string
		heuristic  domain.LinterResult
		expected   domain.ReviewVerdict
	}{
		{
			name:       "All clean and passed",
			llmVerdict: domain.ReviewVerdictAccepted,
			score:      85,
			draftText:  "晨曦微露，青石小径蜿蜒入山。",
			heuristic:  domain.LinterResult{Message: "质检通过"},
			expected:   domain.ReviewVerdictAccepted,
		},
		{
			name:       "Score below 80 fails closed",
			llmVerdict: domain.ReviewVerdictAccepted,
			score:      79,
			draftText:  "晨曦微露，青石小径蜿蜒入山。",
			heuristic:  domain.LinterResult{Message: "质检通过"},
			expected:   domain.ReviewVerdictRevision,
		},
		{
			name:       "LLM rejected fails closed despite 95 score",
			llmVerdict: domain.ReviewVerdictRevision,
			score:      95,
			draftText:  "晨曦微露，青石小径蜿蜒入山。",
			heuristic:  domain.LinterResult{Message: "质检通过"},
			expected:   domain.ReviewVerdictRevision,
		},
		{
			name:       "Banned cliches fail closed",
			llmVerdict: domain.ReviewVerdictAccepted,
			score:      90,
			draftText:  "晨曦微露，嘴角勾起一抹冷笑。",
			heuristic:  domain.LinterResult{HitBannedWords: []string{"嘴角勾起"}},
			expected:   domain.ReviewVerdictRevision,
		},
		{
			name:       "Excessive exclamation fails closed",
			llmVerdict: domain.ReviewVerdictAccepted,
			score:      90,
			draftText:  "杀！斩！破！死！灭！",
			heuristic:  domain.LinterResult{Message: "质检预警: 感叹号过密(每千字8.5个)", ExclamationDensity: 12.0},
			expected:   domain.ReviewVerdictRevision,
		},
		{
			name:       "Excessive exclamation by density without message fails closed",
			llmVerdict: domain.ReviewVerdictAccepted,
			score:      90,
			draftText:  "杀！斩！破！死！灭！",
			heuristic:  domain.LinterResult{ExclamationDensity: 12.0},
			expected:   domain.ReviewVerdictRevision,
		},
		{
			name:       "Misleading exclamation message ignored; predicate governs",
			llmVerdict: domain.ReviewVerdictAccepted,
			score:      90,
			draftText:  "平静如水的叙事，没有任何感叹号。",
			heuristic:  domain.LinterResult{Message: "质检预警: 感叹号过密(伪报警)", ExclamationDensity: 0.0},
			expected:   domain.ReviewVerdictAccepted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideVerdict(tc.llmVerdict, tc.score, tc.draftText, tc.heuristic)
			if got != tc.expected {
				t.Errorf("decideVerdict(%s, %d, ...) = %s; want %s", tc.llmVerdict, tc.score, got, tc.expected)
			}
		})
	}
}
