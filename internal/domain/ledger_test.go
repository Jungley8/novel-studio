package domain

import (
	"strings"
	"testing"
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
