package engine

import (
	"strings"
	"testing"
)

func TestMergeExactContinuation(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		continuation string
		want         string
	}{
		{
			name:         "完全重叠去重",
			prefix:       "第一章正文内容完全一致",
			continuation: "第一章正文内容完全一致",
			want:         "第一章正文内容完全一致",
		},
		{
			name:         "完全相同英文字符",
			prefix:       "The quick brown fox",
			continuation: "The quick brown fox",
			want:         "The quick brown fox",
		},
		{
			name:         "部分重叠",
			prefix:       "主角推开大门，狂风呼啸",
			continuation: "狂风呼啸，带来刺骨寒意。",
			want:         "主角推开大门，狂风呼啸，带来刺骨寒意。",
		},
		{
			name:         "部分重叠英文",
			prefix:       "Chapter 1: The journey begins at dawn.",
			continuation: "at dawn. Shadows lengthened across the valley.",
			want:         "Chapter 1: The journey begins at dawn. Shadows lengthened across the valley.",
		},
		{
			name:         "无重叠补换行",
			prefix:       "第一段内容结束。",
			continuation: "第二段内容开始。",
			want:         "第一段内容结束。\n第二段内容开始。",
		},
		{
			name:         "无重叠补换行-prefix已有换行",
			prefix:       "第一段内容结束。\n",
			continuation: "第二段内容开始。",
			want:         "第一段内容结束。\n第二段内容开始。",
		},
		{
			name:         "无重叠补换行-continuation已有换行",
			prefix:       "第一段内容结束。",
			continuation: "\n第二段内容开始。",
			want:         "第一段内容结束。\n第二段内容开始。",
		},
		{
			name:         "无重叠补换行-两侧均有换行去重",
			prefix:       "第一段内容结束。\n",
			continuation: "\n第二段内容开始。",
			want:         "第一段内容结束。\n第二段内容开始。",
		},
		{
			name:         "一侧为空-prefix为空",
			prefix:       "",
			continuation: "只有续写内容",
			want:         "只有续写内容",
		},
		{
			name:         "一侧为空-continuation为空",
			prefix:       "只有前置内容",
			continuation: "",
			want:         "只有前置内容",
		},
		{
			name:         "一侧为空-双侧均为空",
			prefix:       "",
			continuation: "",
			want:         "",
		},
		{
			name:         "超长重叠上限限制",
			prefix:       "PREFIX-" + strings.Repeat("X", 5000),
			continuation: strings.Repeat("X", 5000) + "-SUFFIX",
			want:         "PREFIX-" + strings.Repeat("X", 5000) + strings.Repeat("X", 5000-continuationOverlapMax) + "-SUFFIX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeExactContinuation(tt.prefix, tt.continuation)
			if got != tt.want {
				t.Errorf("mergeExactContinuation() =\n%q, want\n%q", got, tt.want)
			}
		})
	}
}

func TestIsOutputLimitFinishReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   bool
	}{
		{name: "length", reason: "length", want: true},
		{name: "max_tokens", reason: "max_tokens", want: true},
		{name: "LENGTH大写", reason: "LENGTH", want: true},
		{name: "Length首字母大写", reason: "Length", want: true},
		{name: "MAX_TOKENS大写", reason: "MAX_TOKENS", want: true},
		{name: "Max_Tokens混合大小写", reason: "Max_Tokens", want: true},
		{name: "前后空格容错", reason: "  length  ", want: true},
		{name: "前后换行与空格容错", reason: " \n max_tokens \t ", want: true},
		{name: "空字符串", reason: "", want: false},
		{name: "纯空白字符", reason: "   \t\n", want: false},
		{name: "stop常规结束", reason: "stop", want: false},
		{name: "STOP常规结束大写", reason: "STOP", want: false},
		{name: "content_filter审核拦截", reason: "content_filter", want: false},
		{name: "tool_calls工具调用", reason: "tool_calls", want: false},
		{name: "null字符串", reason: "null", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOutputLimitFinishReason(tt.reason)
			if got != tt.want {
				t.Errorf("isOutputLimitFinishReason(%q) = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

func TestContinuationInstruction(t *testing.T) {
	want := "从上一段停止处继续完成同一份正文。不要重写开头、不要概括、不要重复已完成内容、不要解释过程；只输出缺失的后续内容，并把内容写完整。"
	if got := continuationInstruction(); got != want {
		t.Errorf("continuationInstruction() = %q, want %q", got, want)
	}
}
