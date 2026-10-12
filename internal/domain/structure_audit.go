package domain

import (
	"errors"
	"strings"
	"time"
)

// StructureFindingType defines the categorization of structural narrative issues.
type StructureFindingType string

const (
	StructureFindingTypePlotHole    StructureFindingType = "PLOT_HOLE"
	StructureFindingTypePacing      StructureFindingType = "PACING"
	StructureFindingTypeRedundant   StructureFindingType = "REDUNDANT"
	StructureFindingTypeDroppedHook StructureFindingType = "DROPPED_HOOK"
	StructureFindingTypeArcDrift    StructureFindingType = "ARC_DRIFT"
)

// StructureFindingSeverity defines the priority or severity of the issue.
type StructureFindingSeverity string

const (
	StructureFindingSeverityHigh   StructureFindingSeverity = "HIGH"
	StructureFindingSeverityMedium StructureFindingSeverity = "MEDIUM"
	StructureFindingSeverityLow    StructureFindingSeverity = "LOW"
)

// StructureFindingStatus tracks the resolution workflow of a finding.
type StructureFindingStatus string

const (
	StructureFindingStatusOpen     StructureFindingStatus = "OPEN"
	StructureFindingStatusResolved StructureFindingStatus = "RESOLVED"
	StructureFindingStatusIgnored  StructureFindingStatus = "IGNORED"
)

// StructureFinding represents an issue identified during whole-book structural audit.
type StructureFinding struct {
	ID           string                   `json:"id"`
	ProjectID    string                   `json:"project_id"`
	Type         StructureFindingType     `json:"type"`
	Severity     StructureFindingSeverity `json:"severity"`
	ChapterStart int                      `json:"chapter_start"`
	ChapterEnd   int                      `json:"chapter_end"`
	Summary      string                   `json:"summary"`
	Evidence     string                   `json:"evidence"`
	Suggestion   string                   `json:"suggestion"`
	Status       StructureFindingStatus   `json:"status"`
	CreatedAt    time.Time                `json:"created_at"`
}

// Validate checks the integrity and correctness of a StructureFinding.
func (f *StructureFinding) Validate() error {
	if f == nil {
		return errors.New("structure finding cannot be nil")
	}

	switch f.Type {
	case StructureFindingTypePlotHole,
		StructureFindingTypePacing,
		StructureFindingTypeRedundant,
		StructureFindingTypeDroppedHook,
		StructureFindingTypeArcDrift:
	default:
		return errors.New("invalid finding type: " + string(f.Type))
	}

	switch f.Severity {
	case StructureFindingSeverityHigh,
		StructureFindingSeverityMedium,
		StructureFindingSeverityLow:
	default:
		return errors.New("invalid finding severity: " + string(f.Severity))
	}

	switch f.Status {
	case StructureFindingStatusOpen,
		StructureFindingStatusResolved,
		StructureFindingStatusIgnored:
	default:
		return errors.New("invalid finding status: " + string(f.Status))
	}

	if f.ChapterStart < 1 {
		return errors.New("chapter start must be >= 1")
	}
	if f.ChapterEnd < 1 {
		return errors.New("chapter end must be >= 1")
	}
	if f.ChapterStart > f.ChapterEnd {
		return errors.New("chapter start cannot be greater than chapter end")
	}

	if strings.TrimSpace(f.Summary) == "" {
		return errors.New("summary cannot be empty")
	}
	if strings.TrimSpace(f.Evidence) == "" {
		return errors.New("evidence cannot be empty")
	}
	if strings.TrimSpace(f.Suggestion) == "" {
		return errors.New("suggestion cannot be empty")
	}

	return nil
}
