package engine

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestBuildCharacterMotivationContext_SelectsMentionedCharacters(t *testing.T) {
	entries := []*domain.CodexEntry{
		{
			ID:       "char-1",
			Category: domain.CategoryCharacter,
			Name:     "林尘",
			Realm:    "练气九层",
			Want:     "夺取宗门秘传剑典，以报当年受辱之仇",
			Need:     "放下执念，接纳同道",
			Ghost:    "家族一夜被屠，唯独自己苟活",
		},
		{
			ID:       "char-2",
			Category: domain.CategoryCharacter,
			Name:     "楚峰",
			Realm:    "金丹初期",
			Want:     "登临绝顶，剑斩诸天神魔",
		},
	}

	draft := "林尘拔出长剑，冷冷地注视着走来的楚峰。两人之间杀意弥漫。"

	result := BuildCharacterMotivationContext(entries, draft)
	if result == "" {
		t.Fatalf("expected non-empty motivation context, got empty")
	}

	if !strings.Contains(result, "· 林尘（练气九层）") {
		t.Errorf("expected header for 林尘 with realm, got:\n%s", result)
	}
	if !strings.Contains(result, "Want 外在欲望：夺取宗门秘传剑典，以报当年受辱之仇") {
		t.Errorf("expected Want for 林尘, got:\n%s", result)
	}
	if !strings.Contains(result, "Need 真实需求：放下执念，接纳同道") {
		t.Errorf("expected Need for 林尘, got:\n%s", result)
	}
	if !strings.Contains(result, "Ghost 核心创伤：家族一夜被屠，唯独自己苟活") {
		t.Errorf("expected Ghost for 林尘, got:\n%s", result)
	}

	if !strings.Contains(result, "· 楚峰（金丹初期）") {
		t.Errorf("expected header for 楚峰 with realm, got:\n%s", result)
	}
	if !strings.Contains(result, "Want 外在欲望：登临绝顶，剑斩诸天神魔") {
		t.Errorf("expected Want for 楚峰, got:\n%s", result)
	}
	// Need and Ghost are empty for 楚峰, verify those lines are omitted
	if strings.Contains(result, "楚峰") && strings.Contains(result, "楚峰（金丹初期）\n  Need") {
		t.Errorf("expected empty Need line to be skipped for 楚峰, got:\n%s", result)
	}
}

func TestBuildCharacterMotivationContext_ExcludesUnmentionedCharacters(t *testing.T) {
	entries := []*domain.CodexEntry{
		{
			ID:       "char-1",
			Category: domain.CategoryCharacter,
			Name:     "林尘",
			Realm:    "练气九层",
			Want:     "夺取剑典",
		},
		{
			ID:       "char-2",
			Category: domain.CategoryCharacter,
			Name:     "赵霸",
			Realm:    "筑基中期",
			Want:     "横行宗门",
			Need:     "获得认可",
			Ghost:    "自幼被轻视",
		},
	}

	// Draft mentions ONLY 林尘, not 赵霸
	draft := "林尘一步步走上青云石阶，周身剑气呼啸。"

	result := BuildCharacterMotivationContext(entries, draft)
	if !strings.Contains(result, "林尘") {
		t.Errorf("expected mentioned character 林尘 in result, got:\n%s", result)
	}
	if strings.Contains(result, "赵霸") {
		t.Errorf("expected unmentioned character 赵霸 to be excluded, but found in result:\n%s", result)
	}
	if strings.Contains(result, "横行宗门") {
		t.Errorf("expected unmentioned character's Want to be excluded, but found in result:\n%s", result)
	}
}

func TestBuildCharacterMotivationContext_EmptyInput(t *testing.T) {
	entry := &domain.CodexEntry{
		ID:       "char-1",
		Category: domain.CategoryCharacter,
		Name:     "林尘",
		Want:     "追求长生",
	}

	tests := []struct {
		name    string
		entries []*domain.CodexEntry
		draft   string
	}{
		{"nil entries", nil, "林尘走来"},
		{"empty entries", []*domain.CodexEntry{}, "林尘走来"},
		{"empty draft", []*domain.CodexEntry{entry}, ""},
		{"whitespace draft", []*domain.CodexEntry{entry}, "   \n\t  "},
		{"no match draft", []*domain.CodexEntry{entry}, "天地不仁，万物为刍狗。"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildCharacterMotivationContext(tc.entries, tc.draft)
			if got != "" {
				t.Errorf("expected empty string for %s, got: %q", tc.name, got)
			}
		})
	}
}

func TestBuildCharacterMotivationContext_CapConstraints(t *testing.T) {
	// 1. Cap on character count (max 6)
	var manyEntries []*domain.CodexEntry
	var names []string
	for i := 1; i <= 10; i++ {
		name := string(rune('甲' + i - 1))
		names = append(names, name)
		manyEntries = append(manyEntries, &domain.CodexEntry{
			ID:       name,
			Category: domain.CategoryCharacter,
			Name:     name,
			Realm:    "凡人",
			Want:     "愿望" + name,
		})
	}

	draftWithAll := strings.Join(names, "与") + "在客栈相遇。"
	result := BuildCharacterMotivationContext(manyEntries, draftWithAll)
	count := strings.Count(result, "· ")
	if count != 6 {
		t.Errorf("expected at most 6 character cards capped, got count=%d, result:\n%s", count, result)
	}

	// 2. Cap on runes (max 2000)
	longText := strings.Repeat("长生之路漫漫其修远兮吾将上下而求索", 100) // ~1700 runes
	hugeEntries := []*domain.CodexEntry{
		{
			ID:       "char-huge-1",
			Category: domain.CategoryCharacter,
			Name:     "苍天尊者",
			Want:     longText,
			Need:     longText,
			Ghost:    longText,
		},
		{
			ID:       "char-huge-2",
			Category: domain.CategoryCharacter,
			Name:     "幽冥狂神",
			Want:     longText,
			Need:     longText,
			Ghost:    longText,
		},
	}

	hugeDraft := "苍天尊者与幽冥狂神在虚空中对峙。"
	hugeResult := BuildCharacterMotivationContext(hugeEntries, hugeDraft)
	runeLen := len([]rune(hugeResult))
	if runeLen > 2000 {
		t.Errorf("expected result runes capped at 2000, got %d", runeLen)
	}
	if runeLen == 0 {
		t.Fatalf("expected non-empty result for huge entries")
	}
}

func TestBuildCharacterMotivationContext_WithoutRealm(t *testing.T) {
	entries := []*domain.CodexEntry{
		{
			ID:       "char-no-realm",
			Category: domain.CategoryCharacter,
			Name:     "无名客",
			Realm:    "",
			Want:     "寻一把好刀",
		},
	}
	draft := "无名客缓步前行。"
	result := BuildCharacterMotivationContext(entries, draft)
	if !strings.Contains(result, "· 无名客\n") {
		t.Errorf("expected header without realm parentheses, got:\n%s", result)
	}
	if strings.Contains(result, "· 无名客（）") {
		t.Errorf("unexpected empty parentheses in header, got:\n%s", result)
	}
}

type capturedCall struct {
	systemPrompt string
	userPrompt   string
}

type promptCaptureClient struct {
	mu    sync.Mutex
	calls []capturedCall
}

func (m *promptCaptureClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	m.mu.Lock()
	m.calls = append(m.calls, capturedCall{systemPrompt: systemPrompt, userPrompt: userPrompt})
	m.mu.Unlock()
	return `{"verdict": "ACCEPTED", "score": 85, "issues": [], "suggestions": "合格"}`, nil
}

func (m *promptCaptureClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	resp, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return resp, TokenUsage{TotalTokens: 100}, err
}

func (m *promptCaptureClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	resp, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: resp}
	ch <- StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *promptCaptureClient) findCall(keyword string) *capturedCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if strings.Contains(c.userPrompt, keyword) {
			return &c
		}
	}
	return nil
}

func TestContinuityDivision_InjectsMotivationPrompt(t *testing.T) {
	mock := &promptCaptureClient{}
	court := NewReviewCourt(mock, NewLinter(nil))

	ctx := context.Background()

	// Case 1: With character context
	charCtx := "· 林尘（练气九层）\n  Want 外在欲望：夺取剑典"
	_, _, err := court.ContinuityDivision(ctx, "mock-model", "历史摘要", charCtx, "正文林尘拔剑...")
	if err != nil {
		t.Fatalf("ContinuityDivision failed: %v", err)
	}

	call1 := mock.findCall("ContinuityDivision")
	if call1 == nil {
		t.Fatalf("ContinuityDivision call not captured")
	}

	if !strings.Contains(call1.userPrompt, "【出场角色内在引擎（动机卡）】\n· 林尘（练气九层）\n  Want 外在欲望：夺取剑典") {
		t.Errorf("expected character context injected in userPrompt, got:\n%s", call1.userPrompt)
	}

	if !strings.Contains(call1.systemPrompt, "角色动机一致性") {
		t.Errorf("expected 角色动机一致性 in systemPrompt, got:\n%s", call1.systemPrompt)
	}

	if !strings.Contains(call1.systemPrompt, "反派必须按其自我正当逻辑行动（自认正义/自洽），不得沦为纯粹工具人") {
		t.Errorf("expected villain logic requirement in systemPrompt, got:\n%s", call1.systemPrompt)
	}

	// Case 2: Without character context (empty)
	mock2 := &promptCaptureClient{}
	court2 := NewReviewCourt(mock2, NewLinter(nil))
	_, _, err = court2.ContinuityDivision(ctx, "mock-model", "历史摘要", "", "正文草稿...")
	if err != nil {
		t.Fatalf("ContinuityDivision failed: %v", err)
	}

	call2 := mock2.findCall("ContinuityDivision")
	if call2 == nil {
		t.Fatalf("ContinuityDivision call 2 not captured")
	}

	if !strings.Contains(call2.userPrompt, "【出场角色内在引擎（动机卡）】\n（本章未匹配到角色动机卡）") {
		t.Errorf("expected fallback text for empty character context, got:\n%s", call2.userPrompt)
	}
}

func TestChiefJudge_PassesCharacterContextAndTailAnchor(t *testing.T) {
	mock := &promptCaptureClient{}
	court := NewReviewCourt(mock, NewLinter(nil))

	proj := &domain.Project{ID: "p1", Title: "测试天道"}
	charCtx := "· 林尘（练气九层）\n  Want 外在欲望：夺取剑典"
	tailAnchor := "残阳如血，古道西风。"

	report, _, err := court.ChiefJudge(
		context.Background(),
		"mock-reviewer",
		proj,
		nil,
		1,
		nil,
		nil,
		"正文草稿内容充足超过一百个字符测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试测试",
		ChiefJudgeOptions{
			TailAnchor:       tailAnchor,
			CharacterContext: charCtx,
		},
	)
	if err != nil {
		t.Fatalf("ChiefJudge failed: %v", err)
	}
	if report == nil {
		t.Fatalf("expected non-nil report")
	}

	contCall := mock.findCall("ContinuityDivision")
	if contCall == nil {
		t.Fatalf("expected ContinuityDivision call captured in ChiefJudge")
	}

	if !strings.Contains(contCall.userPrompt, "【出场角色内在引擎（动机卡）】\n"+charCtx) {
		t.Errorf("expected CharacterContext passed to ContinuityDivision, got:\n%s", contCall.userPrompt)
	}

	if !strings.Contains(contCall.userPrompt, "【上章末尾文风锚定 (TailAnchor)】\n"+tailAnchor) {
		t.Errorf("expected TailAnchor passed to ContinuityDivision, got:\n%s", contCall.userPrompt)
	}
}
