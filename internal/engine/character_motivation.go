package engine

import (
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

const (
	maxCharacterMotivationCards = 6
	maxCharacterMotivationRunes = 2000
)

// BuildCharacterMotivationContext extracts mentioned characters from draftText and renders
// their inner motivation engine (Want / Need / Ghost) for continuity court review.
// Empty input or unmentioned entities return an empty string. Capped at 6 characters and 2000 runes.
func BuildCharacterMotivationContext(entries []*domain.CodexEntry, draftText string) string {
	if len(entries) == 0 || strings.TrimSpace(draftText) == "" {
		return ""
	}

	matched, _ := NewMentionScanner().ScanText(draftText, entries)
	if len(matched) == 0 {
		return ""
	}

	var charEntries []*domain.CodexEntry
	for _, ent := range matched {
		if ent == nil {
			continue
		}
		if ent.Category != "" && ent.Category != domain.CategoryCharacter {
			continue
		}
		charEntries = append(charEntries, ent)
		if len(charEntries) >= maxCharacterMotivationCards {
			break
		}
	}

	if len(charEntries) == 0 {
		return ""
	}

	var blocks []string
	for _, ent := range charEntries {
		name := strings.TrimSpace(ent.Name)
		if name == "" {
			continue
		}
		var lines []string
		realm := strings.TrimSpace(ent.Realm)
		if realm != "" {
			lines = append(lines, fmt.Sprintf("· %s（%s）", name, realm))
		} else {
			lines = append(lines, fmt.Sprintf("· %s", name))
		}

		if want := strings.TrimSpace(ent.Want); want != "" {
			lines = append(lines, fmt.Sprintf("  Want 外在欲望：%s", want))
		}
		if need := strings.TrimSpace(ent.Need); need != "" {
			lines = append(lines, fmt.Sprintf("  Need 真实需求：%s", need))
		}
		if ghost := strings.TrimSpace(ent.Ghost); ghost != "" {
			lines = append(lines, fmt.Sprintf("  Ghost 核心创伤：%s", ghost))
		}

		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	if len(blocks) == 0 {
		return ""
	}

	result := strings.Join(blocks, "\n")
	runes := []rune(result)
	if len(runes) > maxCharacterMotivationRunes {
		result = string(runes[:maxCharacterMotivationRunes])
	}
	return result
}
