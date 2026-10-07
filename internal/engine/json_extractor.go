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

	// 2. Try balanced brace extraction for objects { ... }
	if candidate := extractBalanced(trimmed, '{', '}'); candidate != "" {
		if valid := attemptValidateAndSanitize(candidate); valid != "" {
			return valid, nil
		}
	}

	// 3. Try balanced bracket extraction for arrays [ ... ]
	if candidate := extractBalanced(trimmed, '[', ']'); candidate != "" {
		if valid := attemptValidateAndSanitize(candidate); valid != "" {
			return valid, nil
		}
	}

	// 4. Fallback: slice from first { to last }
	firstBrace := strings.Index(trimmed, "{")
	lastBrace := strings.LastIndex(trimmed, "}")
	if firstBrace != -1 && lastBrace > firstBrace {
		candidate := trimmed[firstBrace : lastBrace+1]
		if valid := attemptValidateAndSanitize(candidate); valid != "" {
			return valid, nil
		}
	}

	// 5. One last attempt on the entire trimmed string
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

	// 1. Remove trailing commas before } or ]
	sanitized := trailingCommaRegex.ReplaceAllString(candidate, "$1")
	if json.Valid([]byte(sanitized)) {
		return sanitized
	}

	// 2. Try closing missing curly braces or brackets if output was cut off
	openBraces := strings.Count(sanitized, "{") - strings.Count(sanitized, "}")
	if openBraces > 0 {
		repaired := sanitized + strings.Repeat("}", openBraces)
		if json.Valid([]byte(repaired)) {
			return repaired
		}
	}

	openBrackets := strings.Count(sanitized, "[") - strings.Count(sanitized, "]")
	if openBrackets > 0 {
		repaired := sanitized + strings.Repeat("]", openBrackets)
		if json.Valid([]byte(repaired)) {
			return repaired
		}
	}

	return ""
}
