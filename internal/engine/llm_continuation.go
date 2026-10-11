package engine

import (
	"strings"
)

const maxOutputLimitContinuations = 3
const continuationOverlapMax = 4096

// isOutputLimitFinishReason: 大小写/空白容错；"length" 与 "max_tokens" 视为 output-limit。
func isOutputLimitFinishReason(r string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(r))
	return trimmed == "length" || trimmed == "max_tokens"
}

// mergeExactContinuation: 最长后缀/前缀重叠去重（≤continuationOverlapMax），
// 无重叠时按需补 "\n" 连接；任一侧为空返回另一侧。
func mergeExactContinuation(prefix, continuation string) string {
	if prefix == "" {
		return continuation
	}
	if continuation == "" {
		return prefix
	}

	maxK := len(prefix)
	if len(continuation) < maxK {
		maxK = len(continuation)
	}
	if maxK > continuationOverlapMax {
		maxK = continuationOverlapMax
	}

	for k := maxK; k >= 1; k-- {
		if prefix[len(prefix)-k:] == continuation[:k] {
			return prefix + continuation[k:]
		}
	}

	if strings.HasSuffix(prefix, "\n") || strings.HasPrefix(continuation, "\n") {
		return prefix + continuation
	}
	return prefix + "\n" + continuation
}

// continuationInstruction: 固定中文指令——
// "从上一段停止处继续完成同一份正文。不要重写开头、不要概括、不要重复已完成内容、不要解释过程；只输出缺失的后续内容，并把内容写完整。"
func continuationInstruction() string {
	return "从上一段停止处继续完成同一份正文。不要重写开头、不要概括、不要重复已完成内容、不要解释过程；只输出缺失的后续内容，并把内容写完整。"
}

// MergeExactContinuation exports mergeExactContinuation for testing across packages.
func MergeExactContinuation(prefix, continuation string) string {
	return mergeExactContinuation(prefix, continuation)
}

// IsOutputLimitFinishReason exports isOutputLimitFinishReason for testing across packages.
func IsOutputLimitFinishReason(r string) bool {
	return isOutputLimitFinishReason(r)
}

// ContinuationInstruction exports continuationInstruction for testing across packages.
func ContinuationInstruction() string {
	return continuationInstruction()
}
