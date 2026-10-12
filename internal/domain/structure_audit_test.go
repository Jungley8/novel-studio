package domain

import (
	"strings"
	"testing"
	"time"
)

func validTestFinding() *StructureFinding {
	return &StructureFinding{
		ID:           "find_1",
		ProjectID:    "proj_1",
		Type:         StructureFindingTypePlotHole,
		Severity:     StructureFindingSeverityHigh,
		ChapterStart: 3,
		ChapterEnd:   5,
		Summary:      "第3章主角获得的玉佩在第5章突然失踪",
		Evidence:     "第3章明确提及贴身存放，第5章遇险未被触发或提及",
		Suggestion:   "在第5章补充玉佩去向说明或调整情节触发",
		Status:       StructureFindingStatusOpen,
		CreatedAt:    time.Now(),
	}
}

func TestStructureFinding_Validate(t *testing.T) {
	t.Run("valid finding passes validation", func(t *testing.T) {
		f := validTestFinding()
		if err := f.Validate(); err != nil {
			t.Fatalf("expected valid finding to pass, got: %v", err)
		}
	})

	t.Run("nil finding fails", func(t *testing.T) {
		var f *StructureFinding
		if err := f.Validate(); err == nil {
			t.Fatal("expected error for nil finding")
		}
	})

	t.Run("all valid types pass", func(t *testing.T) {
		types := []StructureFindingType{
			StructureFindingTypePlotHole,
			StructureFindingTypePacing,
			StructureFindingTypeRedundant,
			StructureFindingTypeDroppedHook,
			StructureFindingTypeArcDrift,
		}
		for _, typ := range types {
			f := validTestFinding()
			f.Type = typ
			if err := f.Validate(); err != nil {
				t.Errorf("expected type %s to be valid, got: %v", typ, err)
			}
		}
	})

	t.Run("invalid type fails", func(t *testing.T) {
		f := validTestFinding()
		f.Type = "UNKNOWN_TYPE"
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "invalid finding type") {
			t.Fatalf("expected invalid type error, got: %v", err)
		}
	})

	t.Run("all valid severities pass", func(t *testing.T) {
		severities := []StructureFindingSeverity{
			StructureFindingSeverityHigh,
			StructureFindingSeverityMedium,
			StructureFindingSeverityLow,
		}
		for _, sev := range severities {
			f := validTestFinding()
			f.Severity = sev
			if err := f.Validate(); err != nil {
				t.Errorf("expected severity %s to be valid, got: %v", sev, err)
			}
		}
	})

	t.Run("invalid severity fails", func(t *testing.T) {
		f := validTestFinding()
		f.Severity = "CRITICAL"
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "invalid finding severity") {
			t.Fatalf("expected invalid severity error, got: %v", err)
		}
	})

	t.Run("all valid statuses pass", func(t *testing.T) {
		statuses := []StructureFindingStatus{
			StructureFindingStatusOpen,
			StructureFindingStatusResolved,
			StructureFindingStatusIgnored,
		}
		for _, st := range statuses {
			f := validTestFinding()
			f.Status = st
			if err := f.Validate(); err != nil {
				t.Errorf("expected status %s to be valid, got: %v", st, err)
			}
		}
	})

	t.Run("invalid status fails", func(t *testing.T) {
		f := validTestFinding()
		f.Status = "PENDING"
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "invalid finding status") {
			t.Fatalf("expected invalid status error, got: %v", err)
		}
	})

	t.Run("chapter start < 1 fails", func(t *testing.T) {
		f := validTestFinding()
		f.ChapterStart = 0
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "chapter start must be >= 1") {
			t.Fatalf("expected chapter start error, got: %v", err)
		}
	})

	t.Run("chapter end < 1 fails", func(t *testing.T) {
		f := validTestFinding()
		f.ChapterEnd = 0
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "chapter end must be >= 1") {
			t.Fatalf("expected chapter end error, got: %v", err)
		}
	})

	t.Run("chapter start > chapter end fails (inverted)", func(t *testing.T) {
		f := validTestFinding()
		f.ChapterStart = 10
		f.ChapterEnd = 5
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "chapter start cannot be greater than chapter end") {
			t.Fatalf("expected inverted chapter error, got: %v", err)
		}
	})

	t.Run("single chapter start == end passes", func(t *testing.T) {
		f := validTestFinding()
		f.ChapterStart = 7
		f.ChapterEnd = 7
		if err := f.Validate(); err != nil {
			t.Fatalf("expected single chapter finding to pass, got: %v", err)
		}
	})

	t.Run("empty summary fails", func(t *testing.T) {
		f := validTestFinding()
		f.Summary = "   "
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "summary cannot be empty") {
			t.Fatalf("expected empty summary error, got: %v", err)
		}
	})

	t.Run("empty evidence fails", func(t *testing.T) {
		f := validTestFinding()
		f.Evidence = "\t\n "
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "evidence cannot be empty") {
			t.Fatalf("expected empty evidence error, got: %v", err)
		}
	})

	t.Run("empty suggestion fails", func(t *testing.T) {
		f := validTestFinding()
		f.Suggestion = ""
		if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "suggestion cannot be empty") {
			t.Fatalf("expected empty suggestion error, got: %v", err)
		}
	})
}
