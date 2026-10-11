package domain

import (
	"strings"
	"unicode"
)

// TokenizeForSearch 把文本转为确定性检索记号串（写入与查询必须同源调用）：
// - 连续汉字段（unicode.Is(unicode.Han, r)）：长度 1 → 该字本身；长度 ≥2 → 全部重叠 2-gram（如 "伏笔甲" → "伏笔 笔甲"）
// - ASCII 字母数字段：整体小写为词
// - 其余：分隔符
// 返回空格分隔的记号串；空输入返回 ""。
func TokenizeForSearch(s string) string {
	var tokens []string
	var hanRun []rune
	var asciiRun []rune

	flushHan := func() {
		if len(hanRun) == 0 {
			return
		}
		if len(hanRun) == 1 {
			tokens = append(tokens, string(hanRun[0]))
		} else {
			for i := 0; i < len(hanRun)-1; i++ {
				tokens = append(tokens, string(hanRun[i:i+2]))
			}
		}
		hanRun = hanRun[:0]
	}

	flushASCII := func() {
		if len(asciiRun) == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(string(asciiRun)))
		asciiRun = asciiRun[:0]
	}

	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			flushASCII()
			hanRun = append(hanRun, r)
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			flushHan()
			asciiRun = append(asciiRun, r)
		} else {
			flushHan()
			flushASCII()
		}
	}
	flushHan()
	flushASCII()

	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " ")
}

// MemoryHit 一条检索命中（只读投影）。
type MemoryHit struct {
	Kind         string // "summary" | "hook" | "retro"
	RefID        string
	ChapterIndex int
	Raw          string  // 原始文本（供后续注入）
	Score        float64 // bm25（越小越相关）
}
