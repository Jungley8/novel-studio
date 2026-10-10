package activity

import (
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

// Excerpt returns the head of s capped at n runes, with an ellipsis marker
// when truncated. Timeline lists use this so they never touch compressed blobs.
func Excerpt(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n]) + "…"
}

// CharsDelta reports rune-count movement between snapshots as
// (added, removed). It is a size signal for the timeline, not a real diff:
// growth counts as added, shrinkage as removed.
func CharsDelta(before, after string) (added, removed int) {
	d := utf8.RuneCountInString(after) - utf8.RuneCountInString(before)
	if d >= 0 {
		return d, 0
	}
	return 0, -d
}

// ChangedFields compares two flat maps and returns the sorted top-level keys
// whose values differ (including added/removed keys). Callers marshal
// structured snapshots to map[string]any first.
func ChangedFields(before, after map[string]any) []string {
	if before == nil {
		before = map[string]any{}
	}
	if after == nil {
		after = map[string]any{}
	}
	seen := map[string]bool{}
	var out []string
	for k, bv := range before {
		seen[k] = true
		av, ok := after[k]
		if !ok || !reflect.DeepEqual(bv, av) {
			out = append(out, k)
		}
	}
	for k := range after {
		if !seen[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
