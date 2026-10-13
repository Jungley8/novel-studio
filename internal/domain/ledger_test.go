package domain

import (
	"strings"
	"testing"
	"time"
)

func TestApplyStateMutation(t *testing.T) {
	entry := &CodexEntry{
		Name:         "叶辰",
		Realm:        "练气一层",
		Inventory:    "粗布衣袍, 基础功法, 青钢剑",
		Archetype:    "PROTAGONIST",
		HealthStatus: "健康",
	}

	mutation := StateMutation{
		InventoryDelta: "+残破古玉; 消耗青钢剑; 获得洗髓丹",
		PowerDelta:     "心境突破，练气二层",
	}

	audit, err := ApplyStateMutation(entry, mutation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Check inventory items
	if !strings.Contains(entry.Inventory, "残破古玉") {
		t.Errorf("expected 残破古玉 in inventory, got %s", entry.Inventory)
	}
	if !strings.Contains(entry.Inventory, "洗髓丹") {
		t.Errorf("expected 洗髓丹 in inventory, got %s", entry.Inventory)
	}
	if strings.Contains(entry.Inventory, "青钢剑") {
		t.Errorf("expected 青钢剑 to be removed, got %s", entry.Inventory)
	}

	// 2. Check power progression
	if !strings.Contains(entry.Realm, "练气二层") {
		t.Errorf("expected updated level, got %s", entry.Realm)
	}

	// 3. Check audit trail
	if len(audit) < 3 {
		t.Errorf("expected at least 3 audit entries, got %d", len(audit))
	}

	// 4. Test Invariant: Consuming item not owned
	mutationGhost := StateMutation{
		InventoryDelta: "-诛仙剑",
	}
	auditGhost, _ := ApplyStateMutation(entry, mutationGhost)
	foundWarning := false
	for _, a := range auditGhost {
		if strings.Contains(a, "实体状态机不一致") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected invariant warning when consuming unheld item, got %v", auditGhost)
	}

	// 5. Test Structured Level & Items
	if entry.StructuredLevel == nil || len(entry.StructuredLevel.History) == 0 {
		t.Fatalf("expected structured level history to be populated")
	}
	if len(entry.StructuredItems) != 4 {
		t.Errorf("expected 4 structured items, got %d", len(entry.StructuredItems))
	}

	// 6. Test multiple level progressions without nesting parentheses
	mutation2 := StateMutation{
		PowerDelta: "突破筑基初期",
	}
	_, err = ApplyStateMutation(entry, mutation2)
	if err != nil {
		t.Fatalf("ApplyStateMutation 2 failed: %v", err)
	}
	if strings.Contains(entry.NameAndLevel(), "((") || strings.Contains(entry.NameAndLevel(), "))") {
		t.Errorf("nested parentheses detected: %s", entry.NameAndLevel())
	}
	if !strings.Contains(entry.NameAndLevel(), "筑基初期") {
		t.Errorf("expected 筑基初期, got %s", entry.NameAndLevel())
	}
}

func TestApplyStateMutation_MathematicalQuantity(t *testing.T) {
	entry := &CodexEntry{
		Name:      "楚枫",
		Realm:     "炼气一层",
		Inventory: "粗布衣, 基础飞剑",
		Archetype: "PROTAGONIST",
	}

	// 1. Gain multiple items: 洗髓丹x3, 灵石 100块
	mutation1 := StateMutation{
		InventoryDelta: "获得 洗髓丹x3; +灵石 100块",
	}
	audit1, err := ApplyStateMutation(entry, mutation1)
	if err != nil {
		t.Fatalf("mutation1 failed: %v", err)
	}

	var xisui *InventoryItem
	var lingshi *InventoryItem
	for i := range entry.StructuredItems {
		itm := &entry.StructuredItems[i]
		if itm.Name == "洗髓丹" {
			xisui = itm
		}
		if itm.Name == "灵石" {
			lingshi = itm
		}
	}
	if xisui == nil || xisui.Quantity != 3 {
		t.Fatalf("expected 洗髓丹 quantity 3, got %+v (audit: %v)", xisui, audit1)
	}
	if lingshi == nil || lingshi.Quantity != 100 {
		t.Fatalf("expected 灵石 quantity 100, got %+v", lingshi)
	}
	if !strings.Contains(entry.Inventory, "洗髓丹x3") {
		t.Errorf("expected 洗髓丹x3 in Inventory string, got: %s", entry.Inventory)
	}

	// 2. Consume partial items: 消耗 洗髓丹x1, 消耗 灵石 20块
	mutation2 := StateMutation{
		InventoryDelta: "消耗 洗髓丹x1; -灵石 20块",
	}
	_, err = ApplyStateMutation(entry, mutation2)
	if err != nil {
		t.Fatalf("mutation2 failed: %v", err)
	}

	var xisui2 *InventoryItem
	var lingshi2 *InventoryItem
	for i := range entry.StructuredItems {
		itm := &entry.StructuredItems[i]
		if itm.Name == "洗髓丹" {
			xisui2 = itm
		}
		if itm.Name == "灵石" {
			lingshi2 = itm
		}
	}
	if xisui2 == nil || xisui2.Quantity != 2 {
		t.Fatalf("expected 洗髓丹 quantity 2 after consuming 1, got %+v", xisui2)
	}
	if lingshi2 == nil || lingshi2.Quantity != 80 {
		t.Fatalf("expected 灵石 quantity 80 after consuming 20, got %+v", lingshi2)
	}
	if !strings.Contains(entry.Inventory, "洗髓丹x2") {
		t.Errorf("expected 洗髓丹x2 in inventory, got: %s", entry.Inventory)
	}

	// 3. Fully consume remaining items: 消耗 洗髓丹x2
	mutation3 := StateMutation{
		InventoryDelta: "消耗 洗髓丹x2",
	}
	_, err = ApplyStateMutation(entry, mutation3)
	if err != nil {
		t.Fatalf("mutation3 failed: %v", err)
	}
	for _, itm := range entry.StructuredItems {
		if itm.Name == "洗髓丹" {
			t.Fatalf("expected 洗髓丹 to be completely removed, but still present: %+v", itm)
		}
	}
	if strings.Contains(entry.Inventory, "洗髓丹") {
		t.Errorf("expected 洗髓丹 to be removed from inventory string, got: %s", entry.Inventory)
	}
}

func TestRollbackStateMutation_PowerDeltaGuard(t *testing.T) {
	entry := &CodexEntry{
		Name:      "韩立",
		Realm:     "筑基初期",
		Inventory: "青竹蜂云剑, 养魂木",
		Archetype: "PROTAGONIST",
		StructuredLevel: &PowerLevel{
			Realm: "筑基初期",
			History: []LevelTransition{
				{
					FromRealm: "练气十三层",
					ToRealm:   "筑基初期",
					Reason:    "服下筑基丹突破",
					Timestamp: time.Now(),
				},
			},
		},
		StructuredItems: []InventoryItem{
			{Name: "青竹蜂云剑", Quantity: 1},
			{Name: "养魂木", Quantity: 1},
		},
	}

	// 1. PowerDelta is empty: inventory rolled back, but realm and history remain completely untouched
	mutationEmptyPower := StateMutation{
		InventoryDelta: "+降尘丹",
		PowerDelta:     "",
	}

	audit, err := RollbackStateMutation(entry, mutationEmptyPower)
	if err != nil {
		t.Fatalf("RollbackStateMutation failed: %v", err)
	}

	if entry.Realm != "筑基初期" {
		t.Errorf("expected Realm to remain 筑基初期 when PowerDelta is empty, got %s", entry.Realm)
	}
	if entry.StructuredLevel == nil || len(entry.StructuredLevel.History) != 1 {
		t.Fatalf("expected StructuredLevel.History length to remain 1, got %v", entry.StructuredLevel)
	}
	if entry.StructuredLevel.Realm != "筑基初期" {
		t.Errorf("expected StructuredLevel.Realm to remain 筑基初期, got %s", entry.StructuredLevel.Realm)
	}
	if entry.StructuredLevel.History[0].FromRealm != "练气十三层" || entry.StructuredLevel.History[0].ToRealm != "筑基初期" {
		t.Errorf("unexpected history entry: %+v", entry.StructuredLevel.History[0])
	}
	for _, a := range audit {
		if strings.Contains(a, "境界回滚") {
			t.Errorf("unexpected power rollback audit log when PowerDelta is empty: %s", a)
		}
	}

	// 2. Control case: PowerDelta is non-empty -> realm and history popped normally
	mutationWithPower := StateMutation{
		PowerDelta: "突破筑基初期",
	}

	auditPower, err := RollbackStateMutation(entry, mutationWithPower)
	if err != nil {
		t.Fatalf("RollbackStateMutation with power failed: %v", err)
	}

	if entry.Realm != "练气十三层" {
		t.Errorf("expected Realm to revert to 练气十三层, got %s", entry.Realm)
	}
	if entry.StructuredLevel == nil || len(entry.StructuredLevel.History) != 0 {
		t.Fatalf("expected StructuredLevel.History to be empty after rollback, got %v", entry.StructuredLevel)
	}
	if entry.StructuredLevel.Realm != "练气十三层" {
		t.Errorf("expected StructuredLevel.Realm to revert to 练气十三层, got %s", entry.StructuredLevel.Realm)
	}
	foundPowerAudit := false
	for _, a := range auditPower {
		if strings.Contains(a, "境界回滚") {
			foundPowerAudit = true
			break
		}
	}
	if !foundPowerAudit {
		t.Errorf("expected power rollback audit log, got %v", auditPower)
	}
}

func TestRollbackStateMutation_RemoveMissingItemNoConjure(t *testing.T) {
	entry := &CodexEntry{
		Name:      "林尘",
		Realm:     "筑基初期",
		Inventory: "",
		Archetype: "PROTAGONIST",
	}

	mutation := StateMutation{
		InventoryDelta: "-诛仙剑; -灵石x5",
	}
	audit, err := RollbackStateMutation(entry, mutation)
	if err != nil {
		t.Fatalf("RollbackStateMutation failed: %v", err)
	}

	for _, itm := range entry.StructuredItems {
		if itm.Name == "诛仙剑" || itm.Name == "灵石" {
			t.Fatalf("phantom item conjured by rollback: %+v (inventory: %q)", itm, entry.Inventory)
		}
	}
	if strings.Contains(entry.Inventory, "诛仙剑") || strings.Contains(entry.Inventory, "灵石") {
		t.Fatalf("phantom item conjured in inventory string: %q", entry.Inventory)
	}

	warnings := 0
	for _, a := range audit {
		if strings.Contains(a, "[Ledger Warning]") {
			warnings++
		}
	}
	if warnings != 2 {
		t.Errorf("expected 2 [Ledger Warning] trail entries, got %d: %v", warnings, audit)
	}
}

func TestRollbackStateMutation_RemoveHeldItemRestoresQuantity(t *testing.T) {
	entry := &CodexEntry{
		Name:      "楚枫",
		Realm:     "炼气一层",
		Inventory: "洗髓丹x3",
		Archetype: "PROTAGONIST",
		StructuredItems: []InventoryItem{
			{Name: "洗髓丹", Quantity: 3},
		},
	}

	mutation := StateMutation{
		InventoryDelta: "消耗 洗髓丹x1",
	}
	if _, err := ApplyStateMutation(entry, mutation); err != nil {
		t.Fatalf("ApplyStateMutation failed: %v", err)
	}
	if len(entry.StructuredItems) != 1 || entry.StructuredItems[0].Quantity != 2 {
		t.Fatalf("expected 洗髓丹 quantity 2 after consume, got %+v", entry.StructuredItems)
	}

	audit, err := RollbackStateMutation(entry, mutation)
	if err != nil {
		t.Fatalf("RollbackStateMutation failed: %v", err)
	}
	if len(entry.StructuredItems) != 1 || entry.StructuredItems[0].Quantity != 3 {
		t.Fatalf("expected 洗髓丹 quantity 3 after rollback, got %+v", entry.StructuredItems)
	}
	if !strings.Contains(entry.Inventory, "洗髓丹x3") {
		t.Errorf("expected 洗髓丹x3 in inventory string, got: %s", entry.Inventory)
	}
	foundRestore := false
	for _, a := range audit {
		if strings.Contains(a, "回滚归还") {
			foundRestore = true
			break
		}
	}
	if !foundRestore {
		t.Errorf("expected rollback restore audit log, got %v", audit)
	}
}

func TestParseItemAndQuantity_NarrativeNotes(t *testing.T) {
	cases := []struct {
		input    string
		wantName string
		wantQty  int
	}{
		{"劣质的定情玉佩（作为物证提交天道销毁）", "劣质的定情玉佩", 1},
		{"劣质的定情玉佩(作为物证提交天道销毁)", "劣质的定情玉佩", 1},
		{"劣质的定情玉佩【作为物证提交天道销毁】", "劣质的定情玉佩", 1},
		{"洗髓丹x3（服用）", "洗髓丹", 3},
		{"洗髓丹（服用）x3", "洗髓丹", 3},
		{"灵石 100块 (贿赂长老)", "灵石", 100},
		{"纯阳真经", "纯阳真经", 1},
	}

	for _, tc := range cases {
		name, qty := parseItemAndQuantity(tc.input)
		if name != tc.wantName || qty != tc.wantQty {
			t.Errorf("parseItemAndQuantity(%q) = (%q, %d), want (%q, %d)", tc.input, name, qty, tc.wantName, tc.wantQty)
		}
	}
}

func TestReconcileNarrativeInventoryDelta(t *testing.T) {
	prot := &CodexEntry{
		Name:      "林尘",
		Inventory: "随身物品待起草",
	}

	// 1. 正文有提及物品，自动追溯补全
	content := "林尘咬牙取出那一枚劣质的定情玉佩，猛然捏碎作为物证提交天道销毁。"
	mutation := StateMutation{
		InventoryDelta: "-劣质的定情玉佩（作为物证提交天道销毁）",
	}

	reconciled := ReconcileNarrativeInventoryDelta(prot, content, 1, mutation)
	if !strings.Contains(reconciled.InventoryDelta, "+劣质的定情玉佩") {
		t.Fatalf("expected reconciled delta to contain '+劣质的定情玉佩', got: %s", reconciled.InventoryDelta)
	}

	// 演练结算应该零告警成功
	trail, err := DryRunStateMutation(prot, reconciled)
	if err != nil {
		t.Fatalf("DryRunStateMutation failed: %v", err)
	}
	for _, tr := range trail {
		if strings.Contains(tr, "[Ledger Warning]") {
			t.Fatalf("unexpected warning in reconciled trail: %s", tr)
		}
	}

	// 2. 无正文证据且非占位初始库存时拒绝补全
	protOwned := &CodexEntry{
		Name:      "林尘",
		Inventory: "新手木剑",
	}
	mutationGhost := StateMutation{
		InventoryDelta: "-辟毒珠",
	}
	reconciledGhost := ReconcileNarrativeInventoryDelta(protOwned, "平平无奇的叙事...", 2, mutationGhost)
	if strings.Contains(reconciledGhost.InventoryDelta, "+辟毒珠") {
		t.Fatalf("un-evidenced ghost item must not be auto-reconciled, got: %s", reconciledGhost.InventoryDelta)
	}
	trailGhost, _ := DryRunStateMutation(protOwned, reconciledGhost)
	foundWarning := false
	for _, tr := range trailGhost {
		if strings.Contains(tr, "[Ledger Warning]") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected warning for unowned unevidenced consume")
	}
}
