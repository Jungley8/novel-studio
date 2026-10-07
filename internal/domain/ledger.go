package domain

import (
	"fmt"
	"strings"
	"time"
)

// ApplyStateMutation executes a structured transition on the protagonist's state machine,
// validating invariants (e.g. attempting to consume an item not present in inventory)
// and producing a clean, canonical inventory and level snapshot.
func ApplyStateMutation(p Protagonist, mutation StateMutation) (Protagonist, []string, error) {
	updated := p
	var auditTrail []string

	// 1. Process Inventory Mutations
	items := parseInventoryItems(p.Inventory)
	rawDelta := strings.TrimSpace(mutation.InventoryDelta)

	if rawDelta != "" {
		deltaTokens := splitDeltaTokens(rawDelta)
		for _, token := range deltaTokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}

			// Check if this token represents an addition or a removal
			action, itemName := detectItemAction(token)
			switch action {
			case "ADD":
				if !containsItem(items, itemName) {
					items = append(items, itemName)
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 获得物品: %s", itemName))
				} else {
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 物品已存在(更新记录): %s", itemName))
				}
			case "REMOVE":
				foundIndex := findItemIndex(items, itemName)
				if foundIndex >= 0 {
					removed := items[foundIndex]
					items = append(items[:foundIndex], items[foundIndex+1:]...)
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 消耗/移除物品: %s", removed))
				} else {
					// Invariant warning: consuming an item that doesn't exist
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Warning] 实体状态机不一致: 尝试移除未持有的物品 '%s'", itemName))
				}
			default:
				// Generic delta without explicit +/- prefix
				if !containsItem(items, token) {
					items = append(items, token)
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 状态同步: %s", token))
				}
			}
		}
		updated.Inventory = strings.Join(items, ", ")
	}

	// 2. Process Power / Level Progression
	powerDelta := strings.TrimSpace(mutation.PowerDelta)
	if powerDelta != "" {
		updatedLevel, oldRealm, newRealm := applyPowerDelta(p.NameAndLevel, powerDelta)
		if updatedLevel != p.NameAndLevel {
			auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 境界/战力演变: %s ➔ %s", p.NameAndLevel, updatedLevel))
			updated.NameAndLevel = updatedLevel

			// Update structured level history
			if updated.StructuredLevel == nil {
				updated.StructuredLevel = &PowerLevel{
					Realm: newRealm,
				}
			}
			updated.StructuredLevel.History = append(updated.StructuredLevel.History, LevelTransition{
				FromRealm: oldRealm,
				ToRealm:   newRealm,
				Reason:    powerDelta,
				Timestamp: time.Now(),
			})
			updated.StructuredLevel.Realm = newRealm
		}
	}

	// Update structured items
	var structured []InventoryItem
	for _, itm := range items {
		structured = append(structured, InventoryItem{
			Name:     itm,
			Quantity: 1,
		})
	}
	updated.StructuredItems = structured

	return updated, auditTrail, nil
}

func parseInventoryItems(inv string) []string {
	if strings.TrimSpace(inv) == "" {
		return nil
	}
	// Split by commas, semicolons, or newlines
	replacer := strings.NewReplacer("，", ",", ";", ",", "；", ",", "\n", ",")
	parts := strings.Split(replacer.Replace(inv), ",")

	var items []string
	seen := make(map[string]bool)
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		// Strip leading + or - if lingering in raw text
		clean = strings.TrimPrefix(clean, "+")
		clean = strings.TrimSpace(clean)
		if clean != "" && !seen[clean] {
			items = append(items, clean)
			seen[clean] = true
		}
	}
	return items
}

func splitDeltaTokens(delta string) []string {
	replacer := strings.NewReplacer("，", ",", ";", ",", "；", ",", "\n", ",")
	raw := replacer.Replace(delta)
	var tokens []string
	for _, t := range strings.Split(raw, ",") {
		ct := strings.TrimSpace(t)
		if ct != "" {
			tokens = append(tokens, ct)
		}
	}
	return tokens
}

func detectItemAction(token string) (action, itemName string) {
	// Leading + / -
	if strings.HasPrefix(token, "+") {
		return "ADD", strings.TrimSpace(strings.TrimPrefix(token, "+"))
	}
	if strings.HasPrefix(token, "-") {
		return "REMOVE", strings.TrimSpace(strings.TrimPrefix(token, "-"))
	}

	// Chinese prefixes for add
	addPrefixes := []string{"获得", "得到", "拾取", "买下", "购得", "炼制成", "收服"}
	for _, p := range addPrefixes {
		if strings.HasPrefix(token, p) {
			return "ADD", strings.TrimSpace(strings.TrimPrefix(token, p))
		}
	}

	// Chinese prefixes for remove
	removePrefixes := []string{"消耗", "失去", "损毁", "碎裂", "卖出", "服下", "使用"}
	for _, p := range removePrefixes {
		if strings.HasPrefix(token, p) {
			return "REMOVE", strings.TrimSpace(strings.TrimPrefix(token, p))
		}
	}

	return "DEFAULT", token
}

func containsItem(items []string, target string) bool {
	return findItemIndex(items, target) >= 0
}

func findItemIndex(items []string, target string) int {
	cleanTarget := strings.TrimSpace(target)
	for i, item := range items {
		if item == cleanTarget || strings.Contains(item, cleanTarget) || strings.Contains(cleanTarget, item) {
			return i
		}
	}
	return -1
}

func applyPowerDelta(current, delta string) (updatedLevel, oldRealm, newRealm string) {
	current = strings.TrimSpace(current)
	delta = strings.TrimSpace(delta)
	if current == "" {
		return delta, "", delta
	}
	if strings.Contains(current, delta) {
		return current, current, delta
	}

	// Clean out prefixes like "突破", "进阶", "晋升"
	cleanDelta := delta
	cleanDelta = strings.TrimPrefix(cleanDelta, "突破")
	cleanDelta = strings.TrimPrefix(cleanDelta, "晋升")
	cleanDelta = strings.TrimPrefix(cleanDelta, "进阶")
	cleanDelta = strings.TrimSpace(cleanDelta)

	// If current has form "Name (OldRealm)", replace the realm inside parenthesis
	startIdx := strings.Index(current, "(")
	endIdx := strings.LastIndex(current, ")")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		name := strings.TrimSpace(current[:startIdx])
		oldRealm = strings.TrimSpace(current[startIdx+1 : endIdx])
		newRealm = cleanDelta
		return fmt.Sprintf("%s (%s)", name, newRealm), oldRealm, newRealm
	}

	// Chinese brackets （ ）
	startIdx = strings.Index(current, "（")
	endIdx = strings.LastIndex(current, "）")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		name := strings.TrimSpace(current[:startIdx])
		oldRealm = strings.TrimSpace(current[startIdx+len("（") : endIdx])
		newRealm = cleanDelta
		return fmt.Sprintf("%s (%s)", name, newRealm), oldRealm, newRealm
	}

	return fmt.Sprintf("%s (%s)", current, cleanDelta), current, cleanDelta
}
