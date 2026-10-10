package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// StyleVaultStore defines the persistence operations required for style vault management.
type StyleVaultStore interface {
	GetStyleVault(ctx context.Context, projectID string) (*domain.StyleVault, error)
	SaveStyleVault(ctx context.Context, vault *domain.StyleVault) error
	SetGoldenExcerpts(ctx context.Context, projectID string, excerpts []string) error
	ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error)
}

// EnsureSeededVault checks for an existing StyleVault.
// If the vault is already present and seeded in the store, it returns it directly.
// If empty/missing and at least 3 chapters are archived, it automatically seeds the vault using
// the trailing 300 runes from chapters 1 to 3 and persists it to the store.
// If fewer than 3 chapters are archived, it returns an empty vault without error (no anchor).
//
// Constraint: The seeded style vault is constant and never automatically updated or overwritten.
// Overwriting or updating excerpts requires an explicit call to SetGoldenExcerpts.
func EnsureSeededVault(ctx context.Context, store StyleVaultStore, projectID string) (*domain.StyleVault, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id cannot be empty")
	}

	vault, err := store.GetStyleVault(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get style vault failed: %w", err)
	}
	if vault != nil && len(vault.GoldenExcerpts) > 0 {
		return vault, nil
	}

	chapters, err := store.ListChapters(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters for style vault seeding failed: %w", err)
	}

	// Sort archived chapters deterministically by chapter index ascending
	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].ChapterIndex < chapters[j].ChapterIndex
	})

	if len(chapters) < 3 {
		return &domain.StyleVault{
			ProjectID:      projectID,
			GoldenExcerpts: []string{},
		}, nil
	}

	excerpts := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		ch := chapters[i]
		runes := []rune(ch.Content)
		var excerpt string
		if len(runes) > 300 {
			excerpt = string(runes[len(runes)-300:])
		} else {
			excerpt = string(runes)
		}
		excerpts = append(excerpts, excerpt)
	}

	newVault := &domain.StyleVault{
		ProjectID:      projectID,
		GoldenExcerpts: excerpts,
		UpdatedAt:      time.Now().UTC(),
	}

	if err := store.SaveStyleVault(ctx, newVault); err != nil {
		return nil, fmt.Errorf("seed style vault failed: %w", err)
	}

	return newVault, nil
}

// BuildStyleAnchor builds the project-level constant style anchor prompt section.
// Returns an empty string if vault is nil or has no golden excerpts.
// Pure function.
func BuildStyleAnchor(vault *domain.StyleVault) string {
	if vault == nil || len(vault.GoldenExcerpts) == 0 {
		return ""
	}
	hasContent := false
	for _, e := range vault.GoldenExcerpts {
		if strings.TrimSpace(e) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【文风契约·稳定锚（全书恒定，严禁漂移）】")
	for i, excerpt := range vault.GoldenExcerpts {
		trimmed := strings.TrimSpace(excerpt)
		if trimmed == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n• 样本 %d：\n%s", i+1, trimmed))
	}
	return sb.String()
}

// DetectEchoPhrases detects repetitive echo phrases across chapters.
// Rules:
// 1. Rune-level 4-grams: extracts all 4-rune sequences appearing >= 1 time in draft.
// 2. Cross-chapter occurrence: counts distinct chapters in recent where the 4-gram appears.
// 3. Hit condition:
//   - Appears in draft >= 1 time;
//   - Appears in >= 3 distinct recent chapters;
//   - Occurrence ratio among recent chapters is <= 50% (phrases appearing in > 50% are treated as common style/penmanship and filtered out).
//
// 4. Output: sorted by hit chapter count descending, then lexicographically ascending; top 10 returned.
// 5. Does not panic on empty inputs. Pure function.
func DetectEchoPhrases(recent []string, draft string) []string {
	if len(recent) < 3 {
		return nil
	}
	draftRunes := []rune(draft)
	if len(draftRunes) < 4 {
		return nil
	}

	// 1. Collect candidate 4-grams from draft
	draftGrams := make(map[string]struct{})
	for i := 0; i <= len(draftRunes)-4; i++ {
		gram := string(draftRunes[i : i+4])
		draftGrams[gram] = struct{}{}
	}
	if len(draftGrams) == 0 {
		return nil
	}

	// 2. Pre-index recent chapters into distinct 4-gram sets
	recentGramSets := make([]map[string]struct{}, len(recent))
	for idx, ch := range recent {
		chRunes := []rune(ch)
		m := make(map[string]struct{})
		for i := 0; i <= len(chRunes)-4; i++ {
			m[string(chRunes[i:i+4])] = struct{}{}
		}
		recentGramSets[idx] = m
	}

	totalRecent := len(recent)

	type hitEntry struct {
		gram  string
		count int
	}

	var hits []hitEntry
	for gram := range draftGrams {
		count := 0
		for _, gSet := range recentGramSets {
			if _, ok := gSet[gram]; ok {
				count++
			}
		}

		// Hit condition: in >= 3 chapters, and <= 50% of recent chapters (exclude > 50%)
		if count >= 3 && float64(count)/float64(totalRecent) <= 0.5 {
			hits = append(hits, hitEntry{
				gram:  gram,
				count: count,
			})
		}
	}

	if len(hits) == 0 {
		return nil
	}

	// 3. Sort by hit count descending, ties broken by lexicographical order
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].count != hits[j].count {
			return hits[i].count > hits[j].count
		}
		return hits[i].gram < hits[j].gram
	})

	limit := 10
	if len(hits) < limit {
		limit = len(hits)
	}

	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = hits[i].gram
	}
	return result
}
