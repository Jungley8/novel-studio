package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestExtractAndCleanJSON(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		expectErr bool
		verifyKey string
		expectVal string
	}{
		{
			name: "Standard pure JSON",
			raw:  `{"title": "剑道独尊", "chapters": 10}`,
		},
		{
			name: "Markdown code fence with json language tag",
			raw:  "这是为您生成的节拍推演：\n```json\n{\n  \"action\": \"一剑封喉\"\n}\n```\n祝您写作顺利！",
		},
		{
			name: "Nested braces inside strings",
			raw:  `{"dialogue": "他大喊了一声：{小心背后！}", "action": "反手拔剑"}`,
		},
		{
			name: "Trailing commas before closing brace",
			raw:  `{"score": 85, "verdict": "ACCEPTED",}`,
		},
		{
			name: "Conversational text preamble without code fences",
			raw:  `好的，以下是剧情大纲：{"phase": "绝地反转", "tension": 9} 希望对你有帮助。`,
		},
		{
			name: "Truncated closing brace recovery",
			raw:  `{"status": "IN_PROGRESS", "data": {"nested": "value"`,
		},
		{
			name:      "Empty input",
			raw:       "   ",
			expectErr: true,
		},
		{
			name:      "Completely invalid text",
			raw:       "这是一段普通的散文文本，没有任何大括号或结构化数据。",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := engine.ExtractAndCleanJSON(tt.raw)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for %s, got none (res: %s)", tt.name, res)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.name, err)
			}

			if !json.Valid([]byte(res)) {
				t.Errorf("extracted JSON is invalid: %s", res)
			}
		})
	}
}
