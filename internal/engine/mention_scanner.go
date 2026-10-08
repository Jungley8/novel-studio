package engine

import (
	"sort"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// MatchedMention records the entity and specific name/alias that triggered a mention in text.
type MatchedMention struct {
	Entry        *domain.CodexEntry `json:"entry"`
	MatchedAlias string             `json:"matched_alias"`
	FirstIndex   int                `json:"first_index"`
}

// MentionScanner performs high-performance entity and alias scanning over novel prose and beats.
type MentionScanner struct{}

func NewMentionScanner() *MentionScanner {
	return &MentionScanner{}
}

// ScanText identifies all Codex entities mentioned in text by Name or any Alias,
// as well as entities configured with TrackingModeAlwaysInject.
func (s *MentionScanner) ScanText(text string, entries []*domain.CodexEntry) ([]*domain.CodexEntry, []MatchedMention) {
	if len(entries) == 0 {
		return nil, nil
	}

	var matches []MatchedMention
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry == nil {
			continue
		}

		// 1. If AlwaysInject, include unconditionally
		if entry.TrackingMode == domain.TrackingModeAlwaysInject {
			if !seen[entry.ID] {
				seen[entry.ID] = true
				matches = append(matches, MatchedMention{
					Entry:        entry,
					MatchedAlias: entry.Name,
					FirstIndex:   -1,
				})
			}
			continue
		}

		// 2. If Manual, skip auto-mention scanning
		if entry.TrackingMode == domain.TrackingModeManual {
			continue
		}

		// 3. Scan Name
		cleanName := strings.TrimSpace(entry.Name)
		matched := false
		firstIdx := -1
		if cleanName != "" {
			idx := strings.Index(text, cleanName)
			if idx != -1 {
				matched = true
				firstIdx = idx
				matches = append(matches, MatchedMention{
					Entry:        entry,
					MatchedAlias: cleanName,
					FirstIndex:   idx,
				})
				seen[entry.ID] = true
			}
		}

		// 4. Scan Aliases if name not already matched
		if !matched {
			for _, alias := range entry.Aliases {
				cleanAlias := strings.TrimSpace(alias)
				if cleanAlias == "" {
					continue
				}
				idx := strings.Index(text, cleanAlias)
				if idx != -1 {
					if firstIdx == -1 || idx < firstIdx {
						firstIdx = idx
					}
					matches = append(matches, MatchedMention{
						Entry:        entry,
						MatchedAlias: cleanAlias,
						FirstIndex:   idx,
					})
					seen[entry.ID] = true
					break
				}
			}
		}
	}

	// Sort matches by appearance in text (AlwaysInject first with index -1)
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].FirstIndex < matches[j].FirstIndex
	})

	var matchedEntries []*domain.CodexEntry
	entryDedup := make(map[string]bool)
	for _, m := range matches {
		if !entryDedup[m.Entry.ID] {
			entryDedup[m.Entry.ID] = true
			matchedEntries = append(matchedEntries, m.Entry)
		}
	}

	return matchedEntries, matches
}
