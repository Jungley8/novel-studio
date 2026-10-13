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
	noteSuffixRegex     = regexp.MustCompile(`[\(（【\[]([^\)）】\]]+)[\)）】\]]\s*$`)
	bookTitleRegex      = regexp.MustCompile(`《([^》]+)》`)
	chineseQuoteRegex   = regexp.MustCompile(`[“「]([^”」]+)[”」]`)
)

var narrativeItemPrefixes = []string{
	"系统录入", "系统提示",
	"随身携带的", "随身携带", "随身的", "随身",
	"怀中的", "怀中",
	"袖中的", "袖中",
	"从储物袋取出的", "储物袋中的",
	"作为物证提交天道销毁的", "作为物证提交天道销毁",
	"作为物证的", "作为物证",
	"暗藏的", "暗中备好的", "私藏的",
}

func cleanItemNote(s string) string {
	s = strings.TrimSpace(s)
	if m := noteSuffixRegex.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(s[:len(s)-len(m[0])])
	}
	return s
}

func cleanNarrativeItemName(s string) string {
	s = cleanItemNote(strings.TrimSpace(s))
	for _, p := range narrativeItemPrefixes {
		s = strings.TrimPrefix(s, p)
		s = strings.TrimSpace(s)
	}
	// Strip trailing particles like "的" or "之" when preceded by substantive text
	runes := []rune(s)
	if len(runes) > 2 && (runes[len(runes)-1] == '的' || runes[len(runes)-1] == '之') {
		s = string(runes[:len(runes)-1])
	}
	return strings.TrimSpace(s)
}

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
	replacer := strings.NewReplacer("，", ",", ";", ",", "；", ",", "\n", ",", "、", ",")
	raw := replacer.Replace(delta)
	var rawTokens []string
	for _, t := range strings.Split(raw, ",") {
		ct := strings.TrimSpace(t)
		if ct != "" {
			rawTokens = append(rawTokens, ct)
		}
	}
	// Stitch modifier phrases broken by punctuation (e.g. "林天傲画押的" + "《弟子引资入局协议》")
	var tokens []string
	for i := 0; i < len(rawTokens); i++ {
		curr := rawTokens[i]
		if (strings.HasSuffix(curr, "的") || strings.HasSuffix(curr, "之")) && i+1 < len(rawTokens) {
			next := rawTokens[i+1]
			nextAction, _ := detectItemAction(next)
			if nextAction == "DEFAULT" {
				curr = curr + next
				i++
			}
		}
		tokens = append(tokens, curr)
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

	// 0. Pre-clean trailing parenthetical/bracket narrative notes and system prefixes
	s = cleanNarrativeItemName(s)

	// 1. Check multiplier suffix like "洗髓丹x3", "灵石*10"
	if m := qtyMultiplierRegex.FindStringSubmatch(s); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cleanName := cleanNarrativeItemName(strings.TrimSpace(s[:len(s)-len(m[0])]))
			if cleanName != "" {
				return cleanName, n
			}
		}
	}

	// 2. Check Chinese unit suffix like "洗髓丹3颗", "灵石 100块"
	if m := qtyChineseUnitRegex.FindStringSubmatch(s); len(m) == 3 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cleanName := cleanNarrativeItemName(strings.TrimSpace(s[:len(s)-len(m[0])]))
			if cleanName != "" {
				return cleanName, n
			}
		}
	}

	// 3. Check trailing number after whitespace like "灵石 10"
	if m := qtyTrailingNumRegex.FindStringSubmatch(s); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			cleanName := cleanNarrativeItemName(strings.TrimSpace(s[:len(s)-len(m[0])]))
			if cleanName != "" {
				return cleanName, n
			}
		}
	}

	return cleanNarrativeItemName(s), 1
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

func isPlaceholderItem(name string) bool {
	clean := strings.TrimSpace(name)
	return clean == "" || clean == "无" || clean == "暂无" || clean == "待定" || clean == "随身物品待起草" || strings.Contains(clean, "待起草")
}

func extractNarrativeKeywords(text string) []string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '，', '。', '、', '！', '？', '：', '；', '（', '）', '【', '】', '“', '”', '《', '》', '「', '」', ',', '.', '!', '?', ':', ';', '(', ')', '[', ']', '+', '-', '*', '/', '\n', '\r', '\t', ' ':
			return ' '
		default:
			return r
		}
	}, text)

	fields := strings.Fields(clean)
	var keywords []string
	seen := make(map[string]bool)
	stopWords := map[string]bool{
		"系统录入": true, "系统提示": true, "随身携带": true, "作为物证": true, "提交天道": true,
	}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if len([]rune(f)) >= 2 && !seen[f] && !stopWords[f] {
			seen[f] = true
			keywords = append(keywords, f)
		}
	}
	return keywords
}

func hasNarrativeEvidence(content, rawItem, parsedItem string) bool {
	if strings.TrimSpace(content) == "" {
		return false
	}

	// 1. Direct exact match
	if strings.Contains(content, rawItem) {
		return true
	}

	// 2. Bracket-cleaned note match
	cleanNote := cleanItemNote(rawItem)
	if cleanNote != "" && strings.Contains(content, cleanNote) {
		return true
	}

	// 3. Parsed / normalized item match
	if parsedItem != "" && strings.Contains(content, parsedItem) {
		return true
	}

	// 4. Quoted entity or book title match
	for _, raw := range []string{rawItem, parsedItem} {
		if m := bookTitleRegex.FindStringSubmatch(raw); len(m) == 2 {
			if strings.Contains(content, m[0]) || strings.Contains(content, m[1]) {
				return true
			}
		}
		if m := chineseQuoteRegex.FindStringSubmatch(raw); len(m) == 2 {
			if strings.Contains(content, m[0]) || strings.Contains(content, m[1]) {
				return true
			}
		}
	}

	// 5. Keyword decomposition & narrative presence
	target := parsedItem
	if target == "" {
		target = cleanNote
	}
	if target == "" {
		target = rawItem
	}

	keywords := extractNarrativeKeywords(target)
	if len(keywords) > 0 {
		// A. If a significant keyword of length >= 3 appears in content
		for _, kw := range keywords {
			if len([]rune(kw)) >= 3 && strings.Contains(content, kw) {
				return true
			}
		}

		// B. If multiple keywords exist and all appear in content
		if len(keywords) >= 2 {
			allFound := true
			for _, kw := range keywords {
				if !strings.Contains(content, kw) {
					allFound = false
					break
				}
			}
			if allFound {
				return true
			}
		}
	}

	// C. Sub-string checks for composite phrases like "林天傲画押"
	runes := []rune(target)
	if len(runes) >= 4 {
		prefixPart := string(runes[:3])
		suffixPart := string(runes[3:])
		if strings.Contains(content, prefixPart) && strings.Contains(content, suffixPart) {
			return true
		}
	}

	return false
}

func findInventoryItemIndex(items []InventoryItem, targetName string) int {
	cleanTarget := strings.TrimSpace(targetName)
	if cleanTarget == "" {
		return -1
	}

	// 1. Exact match
	for i, itm := range items {
		if itm.Name == cleanTarget {
			return i
		}
	}

	// 2. Stripped note match
	targetCleanNote := cleanItemNote(cleanTarget)
	for i, itm := range items {
		if !isPlaceholderItem(itm.Name) && cleanItemNote(itm.Name) == targetCleanNote {
			return i
		}
	}

	// 3. Normalized narrative name match (strip prefixes, trailing '的')
	targetNorm := cleanNarrativeItemName(cleanTarget)
	if targetNorm != "" {
		for i, itm := range items {
			if !isPlaceholderItem(itm.Name) && cleanNarrativeItemName(itm.Name) == targetNorm {
				return i
			}
		}
	}

	// 4. Quoted entity / book title match
	if m := bookTitleRegex.FindStringSubmatch(cleanTarget); len(m) == 2 {
		targetBook := m[1]
		for i, itm := range items {
			if isPlaceholderItem(itm.Name) {
				continue
			}
			if bm := bookTitleRegex.FindStringSubmatch(itm.Name); len(bm) == 2 && bm[1] == targetBook {
				return i
			}
			if strings.Contains(itm.Name, targetBook) {
				return i
			}
		}
	}

	// 5. Containment match (for non-placeholder substantive names with >= 2 runes)
	targetRunes := []rune(cleanTarget)
	if len(targetRunes) >= 2 {
		for i, itm := range items {
			if isPlaceholderItem(itm.Name) {
				continue
			}
			itemRunes := []rune(itm.Name)
			if len(itemRunes) >= 2 {
				if strings.Contains(cleanTarget, itm.Name) || strings.Contains(itm.Name, cleanTarget) {
					return i
				}
				if targetNorm != "" && strings.Contains(itm.Name, targetNorm) {
					return i
				}
			}
		}
	}

	return -1
}

// ContainsInventoryItem 检查实体当前库存（包含结构化条目与文本）是否持有目标物品
func ContainsInventoryItem(e *CodexEntry, targetName string) bool {
	if e == nil {
		return false
	}
	items := initInventoryItems(e)
	return findInventoryItemIndex(items, targetName) >= 0
}

// ReconcileNarrativeInventoryDelta 针对章节定稿中合乎正文叙事证据的未登记背景物品扣减，
// 执行自动调解与追溯登记（Auto-Reconcile），避免因开篇背景物品未预先录入而阻断定稿。
func ReconcileNarrativeInventoryDelta(prot *CodexEntry, content string, chapterIndex int, mutation StateMutation) StateMutation {
	if prot == nil {
		return mutation
	}
	rawDelta := strings.TrimSpace(mutation.InventoryDelta)
	if rawDelta == "" {
		return mutation
	}

	tokens := splitDeltaTokens(rawDelta)
	var addReconciled []string
	reconciledSet := make(map[string]bool)

	for _, token := range tokens {
		action, rawItem := detectItemAction(token)
		if action != "REMOVE" {
			continue
		}
		itemName, deltaQty := parseItemAndQuantity(rawItem)
		if itemName == "" || reconciledSet[itemName] {
			continue
		}

		// 检查主角当前是否已持有该物品
		if ContainsInventoryItem(prot, itemName) || ContainsInventoryItem(prot, rawItem) {
			continue
		}

		// 证据门判定：正文内容显式提及该物品或有明确正文叙事证据支撑
		hasEvidence := hasNarrativeEvidence(content, rawItem, itemName)

		if hasEvidence {
			reconciledSet[itemName] = true
			reconciledSet[rawItem] = true
			qtyStr := ""
			if deltaQty > 1 {
				qtyStr = fmt.Sprintf("x%d", deltaQty)
			}
			addReconciled = append(addReconciled, fmt.Sprintf("+%s%s", itemName, qtyStr))
		}
	}

	if len(addReconciled) > 0 {
		// 将追溯补全的前置获得指令注入到当前变更链条头部
		mutation.InventoryDelta = strings.Join(addReconciled, ", ") + ", " + mutation.InventoryDelta
	}

	return mutation
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
