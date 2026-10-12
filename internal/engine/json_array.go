package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// preferredJSONArrayKeys ranks wrapper keys that commonly carry the array a
// given prompt asked for, so that an unrelated array inside the same object is
// not mistaken for the payload.
var preferredJSONArrayKeys = []string{
	"intents", "cards", "chapter_intents", "scenes", "hooks", "relations",
	"candidates", "sections", "chapters", "items", "data", "list", "results", "array", "payload",
}

// UnmarshalJSONArray decodes an LLM JSON payload that is expected to be an
// array of T but may arrive — out of model formatting variance — as:
//
//   - a bare array:        [{"...": 1}, {"...": 2}]
//   - a wrapped object:    {"intents": [{...}]} (any key, preferred keys ranked first)
//   - a numeric-keyed map: {"1": {...}, "2": {...}} (ordered by numeric key)
//
// The bare array wins; otherwise the first non-empty []T found under a
// preferred key, then any key, then a numeric map, wins. An empty array under
// any key is a valid "no items" result (prompts like "无则输出 []").
func UnmarshalJSONArray[T any](cleanJSON string) ([]T, error) {
	var direct []T
	if err := json.Unmarshal([]byte(cleanJSON), &direct); err == nil {
		return direct, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleanJSON), &obj); err != nil {
		return nil, fmt.Errorf("payload is neither a JSON array nor a JSON object: %w", err)
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var emptyFallback []T
	haveEmpty := false
	tryKey := func(k string) ([]T, bool) {
		var items []T
		if err := json.Unmarshal(obj[k], &items); err != nil {
			return nil, false
		}
		if len(items) > 0 {
			return items, true
		}
		if !haveEmpty {
			emptyFallback = items
			haveEmpty = true
		}
		return nil, false
	}

	for _, pref := range preferredJSONArrayKeys {
		for _, k := range keys {
			if strings.EqualFold(k, pref) {
				if items, ok := tryKey(k); ok {
					return items, nil
				}
			}
		}
	}
	for _, k := range keys {
		if items, ok := tryKey(k); ok {
			return items, nil
		}
	}

	// Numeric-keyed map of T: {"1": {...}, "2": {...}} — order by numeric key.
	var byNum map[string]T
	if err := json.Unmarshal([]byte(cleanJSON), &byNum); err == nil && len(byNum) > 0 {
		nums := make([]int, 0, len(byNum))
		vals := make(map[int]T, len(byNum))
		ordered := true
		for k, v := range byNum {
			n, err := strconv.Atoi(strings.TrimSpace(k))
			if err != nil {
				ordered = false
				break
			}
			nums = append(nums, n)
			vals[n] = v
		}
		if ordered {
			sort.Ints(nums)
			out := make([]T, 0, len(nums))
			for _, n := range nums {
				out = append(out, vals[n])
			}
			return out, nil
		}
	}

	if haveEmpty {
		return emptyFallback, nil
	}
	return nil, errors.New("could not locate an array payload in the JSON object")
}
