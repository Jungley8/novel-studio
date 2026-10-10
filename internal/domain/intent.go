package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// IntentStatus defines the lifecycle status of a chapter intent card.
type IntentStatus string

const (
	IntentStatusDraft  IntentStatus = "DRAFT"
	IntentStatusLocked IntentStatus = "LOCKED"
	IntentStatusStale  IntentStatus = "STALE"

	// Convenient aliases
	StatusDraft  = IntentStatusDraft
	StatusLocked = IntentStatusLocked
	StatusStale  = IntentStatusStale
)

// ChapterIntent represents a structured chapter intent card bridging volume arcs and beats.
type ChapterIntent struct {
	ID               string        `json:"id,omitempty"`
	ProjectID        string        `json:"project_id"`
	ChapterIndex     int           `json:"chapter_index"`
	VolumeIndex      int           `json:"volume_index"`
	Conflict         string        `json:"conflict"`
	POV              string        `json:"pov"`
	EmotionCurve     []string      `json:"emotion_curve,omitempty"`
	HookPlants       []string      `json:"hook_plants,omitempty"`
	HookPayoffs      []string      `json:"hook_payoffs,omitempty"`
	ExpectedMutation StateMutation `json:"expected_mutation,omitempty"`
	TensionTarget    int           `json:"tension_target"`
	Status           IntentStatus  `json:"status"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// Validate validates chapter intent invariants (ChapterIndex > 0, TensionTarget 1-10, Conflict non-empty, Status valid).
func (ci *ChapterIntent) Validate() error {
	if ci.ChapterIndex <= 0 {
		return errors.New("chapter intent chapter_index must be greater than 0")
	}
	if ci.TensionTarget < 1 || ci.TensionTarget > 10 {
		return errors.New("chapter intent tension_target must be between 1 and 10")
	}
	if strings.TrimSpace(ci.Conflict) == "" {
		return errors.New("chapter intent conflict cannot be empty")
	}
	switch ci.Status {
	case IntentStatusDraft, IntentStatusLocked, IntentStatusStale:
	default:
		return fmt.Errorf("invalid chapter intent status: %q", ci.Status)
	}
	return nil
}

// ValidateStatusTransition enforces the lifecycle state machine:
// DRAFT -> LOCKED -> STALE -> LOCKED.
func ValidateStatusTransition(from, to IntentStatus) error {
	switch from {
	case IntentStatusDraft:
		if to == IntentStatusLocked {
			return nil
		}
	case IntentStatusLocked:
		if to == IntentStatusStale {
			return nil
		}
	case IntentStatusStale:
		if to == IntentStatusLocked {
			return nil
		}
	}
	return fmt.Errorf("disallowed intent status transition: %s -> %s", from, to)
}

// TransitionTo updates the ChapterIntent's status following valid lifecycle transitions.
// Fail-Closed: invalid intents are not permitted to transition to LOCKED.
func (ci *ChapterIntent) TransitionTo(target IntentStatus) error {
	if err := ValidateStatusTransition(ci.Status, target); err != nil {
		return err
	}
	if target == IntentStatusLocked {
		if err := ci.Validate(); err != nil {
			return fmt.Errorf("fail-closed: invalid intent cannot be locked: %w", err)
		}
	}
	ci.Status = target
	return nil
}

// VolumeSpan defines an explicit chapter range for a volume.
//
// TODO(P1): Merge VolumeSpan into VolumeArc (internal/domain/models.go) and migrate StartChapter/EndChapter
// once concurrent tasks on models.go are complete.
type VolumeSpan struct {
	VolumeIndex  int `json:"volume_index"`
	StartChapter int `json:"start_chapter"`
	EndChapter   int `json:"end_chapter"`
}

// ResolveVolume finds the volume index for a given chapter index.
// It returns the matched VolumeIndex if StartChapter <= chapter <= EndChapter.
// If not found in any span, it returns -1 (never guesses or falls back to the last volume).
func ResolveVolume(chapter int, spans []VolumeSpan) int {
	if chapter <= 0 || len(spans) == 0 {
		return -1
	}
	for _, span := range spans {
		if chapter >= span.StartChapter && chapter <= span.EndChapter {
			return span.VolumeIndex
		}
	}
	return -1
}

// MarkStaleOnVolumeChange marks LOCKED intents belonging to changedVolume as STALE.
// Pure function: returns a new slice without modifying unrelated intents or statuses.
func MarkStaleOnVolumeChange(intents []ChapterIntent, changedVolume int) []ChapterIntent {
	if intents == nil {
		return nil
	}
	result := make([]ChapterIntent, len(intents))
	copy(result, intents)
	for i := range result {
		if result[i].VolumeIndex == changedVolume && result[i].Status == IntentStatusLocked {
			result[i].Status = IntentStatusStale
		}
	}
	return result
}
