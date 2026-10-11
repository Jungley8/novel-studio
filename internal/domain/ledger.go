package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	qtyMultiplierRegex  = regexp.MustCompile(`(?i)[*x×]\s*(\d+)$`)
	qtyChineseUnitRegex = regexp.MustCompile(`(\d+)\s*(个|颗|块|枚|把|瓶|张|本|支|道|条|粒|盒|株|坛|份|斤|件)$`)
	qtyTrailingNumRegex = regexp.MustCompile(`\s+(\d+)$`)
)

// ApplyStateMutation executes a structured transition on a character's state machine (such as the protagonist),
// validating invariants (e.g. attempting to consume an item not present in inventory)
// and producing a clean, canonical inventory and level snapshot.
func ApplyStateMutation(e *CodexEntry, mutation StateMutation) ([]string, error) {
	if e == nil {
		return nil, errors.New("codex entry cannot be nil")
	}
	var auditTrail []string

	// 1. Process Inventory Mutations
	items := initInventoryItems(e)
	rawDelta := strings.TrimSpace(mutation.InventoryDelta)

	if rawDelta != "" {
		deltaTokens := splitDeltaTokens(rawDelta)
		for _, token := range deltaTokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}

			// Check if this token represents an addition or a removal
			action, rawItem := detectItemAction(token)
			itemName, deltaQty := parseItemAndQuantity(rawItem)
			if itemName == "" {
				continue
			}
			if deltaQty <= 0 {
				deltaQty = 1
			}

			switch action {
			case "ADD":
				foundIndex := findInventoryItemIndex(items, itemName)
				if foundIndex >= 0 {
					items[foundIndex].Quantity += deltaQty
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 获得物品: %s (增加 %d, 当前保有: %d)", itemName, deltaQty, items[foundIndex].Quantity))
				} else {
					items = append(items, InventoryItem{
						Name:     itemName,
						Quantity: deltaQty,
					})
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 获得物品: %s (数量: %d)", itemName, deltaQty))
				}
			case "REMOVE":
				foundIndex := findInventoryItemIndex(items, itemName)
				if foundIndex >= 0 {
					if items[foundIndex].Quantity > deltaQty {
						items[foundIndex].Quantity -= deltaQty
						auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 消耗/移除物品: %s (扣除 %d, 剩余: %d)", itemName, deltaQty, items[foundIndex].Quantity))
					} else {
						consumedAll := items[foundIndex].Quantity
						items = append(items[:foundIndex], items[foundIndex+1:]...)
						auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 消耗/移除物品: %s (已全部用尽, 共消耗 %d)", itemName, consumedAll))
					}
				} else {
					// Invariant warning: consuming an item that doesn't exist
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Warning] 实体状态机不一致: 尝试移除未持有的物品 '%s'", itemName))
				}
			default:
				foundIndex := findInventoryItemIndex(items, itemName)
				if foundIndex >= 0 {
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 状态同步: %s (持有: %d)", itemName, items[foundIndex].Quantity))
				} else {
					items = append(items, InventoryItem{
						Name:     itemName,
						Quantity: deltaQty,
					})
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 状态同步: %s (数量: %d)", itemName, deltaQty))
				}
			}
		}

		var formatted []string
		for _, itm := range items {
			if itm.Quantity > 1 {
				formatted = append(formatted, fmt.Sprintf("%sx%d", itm.Name, itm.Quantity))
			} else {
				formatted = append(formatted, itm.Name)
			}
		}
		e.Inventory = strings.Join(formatted, ", ")
		e.StructuredItems = items
	} else if len(e.StructuredItems) == 0 && len(items) > 0 {
		e.StructuredItems = items
	}

	// 2. Process Power / Level Progression
	powerDelta := strings.TrimSpace(mutation.PowerDelta)
	if powerDelta != "" {
		currentLevel := e.Realm
		if currentLevel == "" {
			currentLevel = e.Name
		}
		_, oldRealm, newRealm := applyPowerDelta(currentLevel, powerDelta)
		if newRealm != "" && newRealm != oldRealm {
			auditTrail = append(auditTrail, fmt.Sprintf("[Ledger] 境界/战力演变: %s ➔ %s", currentLevel, newRealm))
			e.Realm = newRealm

			// Update structured level history
			if e.StructuredLevel == nil {
				e.StructuredLevel = &PowerLevel{
					Realm: newRealm,
				}
			}
			e.StructuredLevel.History = append(e.StructuredLevel.History, LevelTransition{
				FromRealm: oldRealm,
				ToRealm:   newRealm,
				Reason:    powerDelta,
				Timestamp: time.Now(),
			})
			e.StructuredLevel.Realm = newRealm
		}
	}

	return auditTrail, nil
}

// RollbackStateMutation reverts the effects of a StateMutation from a withdrawn chapter,
// restoring consumed inventory items, removing acquired items, and rewinding power level transitions.
func RollbackStateMutation(e *CodexEntry, mutation StateMutation) ([]string, error) {
	if e == nil {
		return nil, errors.New("codex entry cannot be nil")
	}
	var auditTrail []string

	// 1. Invert inventory delta
	items := initInventoryItems(e)
	rawDelta := strings.TrimSpace(mutation.InventoryDelta)

	if rawDelta != "" {
		deltaTokens := splitDeltaTokens(rawDelta)
		for _, token := range deltaTokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}

			action, rawItem := detectItemAction(token)
			itemName, deltaQty := parseItemAndQuantity(rawItem)
			if itemName == "" {
				continue
			}
			if deltaQty <= 0 {
				deltaQty = 1
			}

			switch action {
			case "ADD":
				// Reversing an addition means removing it
				foundIndex := findInventoryItemIndex(items, itemName)
				if foundIndex >= 0 {
					if items[foundIndex].Quantity > deltaQty {
						items[foundIndex].Quantity -= deltaQty
						auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Rollback] 回滚移除物品: %s (扣减 %d, 剩余: %d)", itemName, deltaQty, items[foundIndex].Quantity))
					} else {
						items = append(items[:foundIndex], items[foundIndex+1:]...)
						auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Rollback] 回滚完全移除物品: %s", itemName))
					}
				}
			case "REMOVE":
				// Reversing a consumption means re-adding it
				foundIndex := findInventoryItemIndex(items, itemName)
				if foundIndex >= 0 {
					items[foundIndex].Quantity += deltaQty
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Rollback] 回滚归还已消耗物品: %s (增加 %d, 当前: %d)", itemName, deltaQty, items[foundIndex].Quantity))
				} else {
					// Fail-closed guard (P2-2): reversing a consumption of an item the
					// entity never held must not conjure it out of thin air. A phantom
					// REMOVE can land in the store via the commit path (ApplyStateMutation
					// only logs a warning for unheld removals), and blindly re-adding it
					// here would turn consumed "air" into real inventory on uncommit.
					// Record a warning trail instead and leave the inventory untouched.
					auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Warning] 回滚跳过: 尝试归还未持有的物品 '%s'，拒绝凭空补发", itemName))
				}
			}
		}

		var formatted []string
		for _, itm := range items {
			if itm.Quantity > 1 {
				formatted = append(formatted, fmt.Sprintf("%sx%d", itm.Name, itm.Quantity))
			} else {
				formatted = append(formatted, itm.Name)
			}
		}
		e.Inventory = strings.Join(formatted, ", ")
		e.StructuredItems = items
	}

	// 2. Rollback Power progression
	if strings.TrimSpace(mutation.PowerDelta) != "" && e.StructuredLevel != nil && len(e.StructuredLevel.History) > 0 {
		lastIdx := len(e.StructuredLevel.History) - 1
		lastHist := e.StructuredLevel.History[lastIdx]
		e.StructuredLevel.Realm = lastHist.FromRealm
		e.StructuredLevel.History = e.StructuredLevel.History[:lastIdx]
		e.Realm = lastHist.FromRealm
		auditTrail = append(auditTrail, fmt.Sprintf("[Ledger Rollback] 境界回滚: %s ➔ %s", lastHist.ToRealm, lastHist.FromRealm))
	}

	return auditTrail, nil
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

func parseItemAndQuantity(raw string) (name string, qty int) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", 1
	}

	// 1. Check multiplier suffix like "洗髓丹x3", "灵石*10"
	if m := qtyMultiplierRegex.FindStringSubmatch(s); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cleanName := strings.TrimSpace(s[:len(s)-len(m[0])])
			if cleanName != "" {
				return cleanName, n
			}
		}
	}

	// 2. Check Chinese unit suffix like "洗髓丹3颗", "灵石 100块"
	if m := qtyChineseUnitRegex.FindStringSubmatch(s); len(m) == 3 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cleanName := strings.TrimSpace(s[:len(s)-len(m[0])])
			if cleanName != "" {
				return cleanName, n
			}
		}
	}

	// 3. Check trailing number after whitespace like "灵石 10"
	if m := qtyTrailingNumRegex.FindStringSubmatch(s); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cleanName := strings.TrimSpace(s[:len(s)-len(m[0])])
			if cleanName != "" {
				return cleanName, n
			}
		}
	}

	return s, 1
}

func initInventoryItems(e *CodexEntry) []InventoryItem {
	if e == nil {
		return nil
	}
	if len(e.StructuredItems) > 0 {
		var copyItems []InventoryItem
		for _, itm := range e.StructuredItems {
			name := strings.TrimSpace(itm.Name)
			qty := itm.Quantity
			if qty <= 0 {
				qty = 1
			}
			copyItems = append(copyItems, InventoryItem{
				Name:       name,
				Quantity:   qty,
				Quality:    itm.Quality,
				AcquiredAt: itm.AcquiredAt,
			})
		}
		return copyItems
	}

	rawItems := parseInventoryItems(e.Inventory)
	var items []InventoryItem
	for _, raw := range rawItems {
		name, qty := parseItemAndQuantity(raw)
		if name != "" {
			items = append(items, InventoryItem{
				Name:     name,
				Quantity: qty,
			})
		}
	}
	return items
}

func findInventoryItemIndex(items []InventoryItem, targetName string) int {
	cleanTarget := strings.TrimSpace(targetName)
	for i, itm := range items {
		if itm.Name == cleanTarget {
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

	return cleanDelta, current, cleanDelta
}

// DryRunStateMutation 在副本上执行状态结算，只返回审计轨迹，不改原对象。
func DryRunStateMutation(e *CodexEntry, mutation StateMutation) ([]string, error) {
	if e == nil {
		return nil, nil
	}
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal codex entry for dry run: %w", err)
	}
	var clone CodexEntry
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("unmarshal codex entry for dry run: %w", err)
	}
	return ApplyStateMutation(&clone, mutation)
}
