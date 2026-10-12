package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRepairUnescapedQuotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "valid json is identity",
			in:   `{"a":"he said \"hi\", ok","b":["x","y"]}`,
			want: `{"a":"he said \"hi\", ok","b":["x","y"]}`,
		},
		{
			name: "interior quoted phrase",
			in:   `{"issues":["老冯口中的两处（"您猜怎么着——"）属口语强调"]}`,
			want: `{"issues":["老冯口中的两处（\"您猜怎么着——\"）属口语强调"]}`,
		},
		{
			name: "adjacent interior quotes",
			in:   `{"issues":["（"您猜怎么着——""奇就奇在这儿——"）"]}`,
			want: `{"issues":["（\"您猜怎么着——\"\"奇就奇在这儿——\"）"]}`,
		},
		{
			name: "closing quote followed by comma",
			in:   `{"issues":["（"测试"）","next"]}`,
			want: `{"issues":["（\"测试\"）","next"]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := repairUnescapedQuotes(tc.in)
			if got != tc.want {
				t.Fatalf("repairUnescapedQuotes()\n got: %s\nwant: %s", got, tc.want)
			}
			if !json.Valid([]byte(got)) {
				t.Fatalf("repaired output must be valid JSON: %s", got)
			}
		})
	}
}

// TestExtractAndCleanJSON_UnescapedInteriorQuotes reproduces the real payload
// that halted chapter 1: a fenced review JSON whose issues array quotes the
// draft's dialogue with half-width double quotes.
func TestExtractAndCleanJSON_UnescapedInteriorQuotes(t *testing.T) {
	raw := "```json\n" +
		`{"verdict":"ACCEPTED","score":85,"issues":["揭晓式破折号4处需分类处理：老冯口中的两处（"您猜怎么着——""奇就奇在这儿——"）属口语强调，声口自然可保留","其余为叙述层问题"],"suggestions":"拆叙述层揭晓破折号，改用句号斩断"}` +
		"\n```"
	clean, err := ExtractAndCleanJSON(raw)
	if err != nil {
		t.Fatalf("fenced payload with unescaped quotes must be cleaned: %v", err)
	}
	var out struct {
		Verdict string   `json:"verdict"`
		Score   int      `json:"score"`
		Issues  []string `json:"issues"`
	}
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		t.Fatalf("cleaned payload must unmarshal: %v", err)
	}
	if out.Verdict != "ACCEPTED" || len(out.Issues) != 2 {
		t.Fatalf("unexpected decode: %+v", out)
	}
	if !strings.Contains(out.Issues[0], "您猜怎么着——") || !strings.Contains(out.Issues[0], "奇就奇在这儿——") {
		t.Fatalf("quoted phrases must survive repair: %q", out.Issues[0])
	}
}

// TestExtractAndCleanJSON_UnescapedQuotesPlusTrailingComma verifies the quote
// repair composes with the pre-existing trailing-comma repair.
func TestExtractAndCleanJSON_UnescapedQuotesPlusTrailingComma(t *testing.T) {
	raw := `{"verdict":"REVISION_NEEDED","issues":["叫他"老冯"即可"],}`
	clean, err := ExtractAndCleanJSON(raw)
	if err != nil {
		t.Fatalf("payload must be cleaned: %v", err)
	}
	var out struct {
		Issues []string `json:"issues"`
	}
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		t.Fatalf("cleaned payload must unmarshal: %v", err)
	}
	if len(out.Issues) != 1 || !strings.Contains(out.Issues[0], `"老冯"`) {
		t.Fatalf("unexpected decode: %+v", out)
	}
}
