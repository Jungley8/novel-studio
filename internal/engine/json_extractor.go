package engine

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var trailingCommaRegex = regexp.MustCompile(`,(\s*[}\]])`)

// ExtractAndCleanJSON extracts a valid JSON object or array from LLM responses,
// cleanly handling markdown code blocks, balanced braces, conversational text,
// trailing commas, and partial truncation.
func ExtractAndCleanJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("empty LLM response")
	}

	// 1. Try markdown code block ```json ... ``` or ``` ... ```
	if candidate := extractFromMarkdownFence(trimmed); candidate != "" {
		if valid := attemptValidateAndSanitize(candidate); valid != "" {
			return valid, nil
		}
	}

	firstBrace := strings.Index(trimmed, "{")
	firstBracket := strings.Index(trimmed, "[")

	// If bracket appears before brace, prioritize array extraction
	if firstBracket != -1 && (firstBrace == -1 || firstBracket < firstBrace) {
		if candidate := extractBalanced(trimmed, '[', ']'); candidate != "" {
			if valid := attemptValidateAndSanitize(candidate); valid != "" {
				return valid, nil
			}
		}
		if candidate := extractBalanced(trimmed, '{', '}'); candidate != "" {
			if valid := attemptValidateAndSanitize(candidate); valid != "" {
				return valid, nil
			}
		}
		lastBracket := strings.LastIndex(trimmed, "]")
		if lastBracket != -1 && lastBracket > firstBracket {
			candidate := trimmed[firstBracket : lastBracket+1]
			if valid := attemptValidateAndSanitize(candidate); valid != "" {
				return valid, nil
			}
		}
	} else {
		if candidate := extractBalanced(trimmed, '{', '}'); candidate != "" {
			if valid := attemptValidateAndSanitize(candidate); valid != "" {
				return valid, nil
			}
		}
		if candidate := extractBalanced(trimmed, '[', ']'); candidate != "" {
			if valid := attemptValidateAndSanitize(candidate); valid != "" {
				return valid, nil
			}
		}
		lastBrace := strings.LastIndex(trimmed, "}")
		if firstBrace != -1 && lastBrace > firstBrace {
			candidate := trimmed[firstBrace : lastBrace+1]
			if valid := attemptValidateAndSanitize(candidate); valid != "" {
				return valid, nil
			}
		}
	}

	// One last attempt on the entire trimmed string
	if valid := attemptValidateAndSanitize(trimmed); valid != "" {
		return valid, nil
	}

	return "", errors.New("failed to extract valid JSON from LLM output")
}

func extractFromMarkdownFence(s string) string {
	idx := strings.Index(s, "```")
	if idx == -1 {
		return ""
	}
	sub := s[idx+3:]
	// Strip optional json or other language identifier
	if newline := strings.Index(sub, "\n"); newline != -1 {
		lang := strings.TrimSpace(sub[:newline])
		if lang == "json" || lang == "" {
			sub = sub[newline+1:]
		}
	}
	endIdx := strings.LastIndex(sub, "```")
	if endIdx != -1 {
		return strings.TrimSpace(sub[:endIdx])
	}
	return strings.TrimSpace(sub)
}

func extractBalanced(s string, open, close byte) string {
	start := -1
	depth := 0
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			continue
		}

		if c == open {
			if depth == 0 {
				start = i
			}
			depth++
		} else if c == close && depth > 0 {
			depth--
			if depth == 0 && start != -1 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

func attemptValidateAndSanitize(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}

	// Check if already valid
	if json.Valid([]byte(candidate)) {
		return candidate
	}

	// 0. Repair half-width quotes that ended up inside string literals without
	// escaping (the common LLM failure mode when it quotes prose, e.g.
	// "老冯口中的两处（"您猜怎么着——"）"). Every pass below tracks string
	// state, which under-escaped input breaks, so repair quotes first.
	quoted := repairUnescapedQuotes(candidate)
	if json.Valid([]byte(quoted)) {
		return quoted
	}

	// 1. Remove trailing commas before } or ] strictly outside string literals
	sanitized := removeTrailingCommasAware(quoted)
	if json.Valid([]byte(sanitized)) {
		return sanitized
	}

	// 2. Try closing missing curly braces or brackets strictly counted outside string literals
	openBraces, openBrackets := countUnclosedTokens(sanitized)
	if openBraces > 0 || openBrackets > 0 {
		repaired := sanitized + strings.Repeat("]", openBrackets) + strings.Repeat("}", openBraces)
		if json.Valid([]byte(repaired)) {
			return repaired
		}
		repairedAlt := sanitized + strings.Repeat("}", openBraces) + strings.Repeat("]", openBrackets)
		if json.Valid([]byte(repairedAlt)) {
			return repairedAlt
		}
	}

	return ""
}

// repairUnescapedQuotes escapes half-width double quotes that appear inside a
// JSON string literal without a backslash. A quote is treated as closing the
// string only when the next non-whitespace byte is ',', '}', ']', ':' or
// end-of-input; anything else (typically CJK text or punctuation following a
// quoted phrase) means the quote was literal content and gets escaped. For
// structurally valid JSON the function returns the input unchanged.
func repairUnescapedQuotes(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 16)
	inString := false
	escaped := false
	n := len(s)

	for i := 0; i < n; i++ {
		c := s[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			sb.WriteByte(c)
			continue
		}

		if escaped {
			escaped = false
			sb.WriteByte(c)
			continue
		}

		switch c {
		case '\\':
			escaped = true
			sb.WriteByte(c)
		case '"':
			j := i + 1
			for j < n {
				b := s[j]
				if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
					j++
					continue
				}
				break
			}
			if j >= n || s[j] == ',' || s[j] == '}' || s[j] == ']' || s[j] == ':' {
				inString = false
				sb.WriteByte(c)
			} else {
				sb.WriteString(`\"`)
			}
		default:
			sb.WriteByte(c)
		}
	}

	return sb.String()
}

// removeTrailingCommasAware removes trailing commas directly preceding '}' or ']'
// while strictly preserving commas and brackets inside quoted JSON string literals.
func removeTrailingCommasAware(s string) string {
	var sb strings.Builder
	inString := false
	escaped := false
	n := len(s)

	for i := 0; i < n; i++ {
		c := s[i]
		if inString {
			sb.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			sb.WriteByte(c)
			continue
		}

		if c == ',' {
			// Lookahead for closing brace or bracket outside string literal
			peekIdx := i + 1
			for peekIdx < n && (s[peekIdx] == ' ' || s[peekIdx] == '\t' || s[peekIdx] == '\r' || s[peekIdx] == '\n') {
				peekIdx++
			}
			if peekIdx < n && (s[peekIdx] == '}' || s[peekIdx] == ']') {
				// Trailing comma found outside string: skip writing it!
				continue
			}
		}

		sb.WriteByte(c)
	}

	return sb.String()
}

// countUnclosedTokens counts unbalanced '{' and '[' exclusively outside string literals.
func countUnclosedTokens(s string) (unclosedBraces, unclosedBrackets int) {
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			continue
		}

		if c == '{' {
			unclosedBraces++
		} else if c == '}' && unclosedBraces > 0 {
			unclosedBraces--
		} else if c == '[' {
			unclosedBrackets++
		} else if c == ']' && unclosedBrackets > 0 {
			unclosedBrackets--
		}
	}

	return unclosedBraces, unclosedBrackets
}
