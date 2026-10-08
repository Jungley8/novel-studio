package domain

import (
	"testing"
)

func TestCodexEntry_Validate(t *testing.T) {
	entry := &CodexEntry{
		ProjectID: "proj-1",
		Category:  CategoryCharacter,
		Name:      "楚枫",
	}
	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid entry, got: %v", err)
	}
	if entry.TrackingMode != TrackingModeAutoMention {
		t.Errorf("expected default tracking mode AUTO_MENTION, got: %s", entry.TrackingMode)
	}

	invalid := &CodexEntry{
		ProjectID: "",
		Name:      "楚枫",
		Category:  CategoryCharacter,
	}
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for empty project_id")
	}

	invalidCategory := &CodexEntry{
		ProjectID: "proj-1",
		Name:      "楚枫",
		Category:  "UNKNOWN",
	}
	if err := invalidCategory.Validate(); err == nil {
		t.Errorf("expected error for unknown category")
	}
}

func TestCodexEntry_ActiveProgression(t *testing.T) {
	entry := &CodexEntry{
		ProjectID: "proj-1",
		Category:  CategoryCharacter,
		Name:      "楚枫",
		Progressions: []Progression{
			{ActiveFromChapter: 1, StatePayloadJSON: `{"realm":"炼气三层"}`},
			{ActiveFromChapter: 10, StatePayloadJSON: `{"realm":"筑基初期"}`},
			{ActiveFromChapter: 30, StatePayloadJSON: `{"realm":"金丹境"}`},
		},
	}

	// At chapter 5 -> should match chapter 1 progression
	p5 := entry.ActiveProgression(5)
	if p5 == nil || p5.ActiveFromChapter != 1 {
		t.Fatalf("expected active from chapter 1, got %+v", p5)
	}

	// At chapter 10 -> should match chapter 10 progression
	p10 := entry.ActiveProgression(10)
	if p10 == nil || p10.ActiveFromChapter != 10 {
		t.Fatalf("expected active from chapter 10, got %+v", p10)
	}

	// At chapter 25 -> should still match chapter 10 progression
	p25 := entry.ActiveProgression(25)
	if p25 == nil || p25.ActiveFromChapter != 10 {
		t.Fatalf("expected active from chapter 10, got %+v", p25)
	}

	// At chapter 50 -> should match chapter 30 progression
	p50 := entry.ActiveProgression(50)
	if p50 == nil || p50.ActiveFromChapter != 30 {
		t.Fatalf("expected active from chapter 30, got %+v", p50)
	}

	// At chapter 0 -> no progression active
	p0 := entry.ActiveProgression(0)
	if p0 != nil {
		t.Errorf("expected nil progression before chapter 1, got %+v", p0)
	}
}
