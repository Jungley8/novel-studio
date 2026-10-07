package domain

import (
	"strings"
	"testing"
)

func TestApplyStateMutation(t *testing.T) {
	initial := Protagonist{
		NameAndLevel: "叶辰 (练气一层)",
		Inventory:    "粗布衣袍, 基础功法, 青钢剑",
		CoreGoal:     "救出家族老祖",
		HealthStatus: "健康",
	}

	mutation := StateMutation{
		InventoryDelta: "+残破古玉; 消耗青钢剑; 获得洗髓丹",
		PowerDelta:     "心境突破，练气二层",
	}

	updated, audit, err := ApplyStateMutation(initial, mutation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Check inventory items
	if !strings.Contains(updated.Inventory, "残破古玉") {
		t.Errorf("expected 残破古玉 in inventory, got %s", updated.Inventory)
	}
	if !strings.Contains(updated.Inventory, "洗髓丹") {
		t.Errorf("expected 洗髓丹 in inventory, got %s", updated.Inventory)
	}
	if strings.Contains(updated.Inventory, "青钢剑") {
		t.Errorf("expected 青钢剑 to be removed, got %s", updated.Inventory)
	}

	// 2. Check power progression
	if !strings.Contains(updated.NameAndLevel, "心境突破，练气二层") {
		t.Errorf("expected updated level, got %s", updated.NameAndLevel)
	}

	// 3. Check audit trail
	if len(audit) < 3 {
		t.Errorf("expected at least 3 audit entries, got %d", len(audit))
	}

	// 4. Test Invariant: Consuming item not owned
	mutationGhost := StateMutation{
		InventoryDelta: "-诛仙剑",
	}
	_, auditGhost, _ := ApplyStateMutation(updated, mutationGhost)
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
	if updated.StructuredLevel == nil || len(updated.StructuredLevel.History) == 0 {
		t.Fatalf("expected structured level history to be populated")
	}
	if len(updated.StructuredItems) != 4 {
		t.Errorf("expected 4 structured items, got %d", len(updated.StructuredItems))
	}

	// 6. Test multiple level progressions without nesting parentheses
	mutation2 := StateMutation{
		PowerDelta: "突破筑基初期",
	}
	updated2, _, err := ApplyStateMutation(updated, mutation2)
	if err != nil {
		t.Fatalf("ApplyStateMutation 2 failed: %v", err)
	}
	if strings.Contains(updated2.NameAndLevel, "((") || strings.Contains(updated2.NameAndLevel, "))") {
		t.Errorf("nested parentheses detected: %s", updated2.NameAndLevel)
	}
	if !strings.Contains(updated2.NameAndLevel, "筑基初期") {
		t.Errorf("expected 筑基初期, got %s", updated2.NameAndLevel)
	}
}
