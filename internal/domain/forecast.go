package domain

import "time"

// Forecast status lifecycle constants.
const (
	ForecastStatusActive   = "active"
	ForecastStatusStale    = "stale"
	ForecastStatusArchived = "archived"
)

// ActivePlan status lifecycle constants.
const (
	PlanStatusActive   = "active"
	PlanStatusConsumed = "consumed"
	PlanStatusDiverged = "diverged"
	PlanStatusStale    = "stale"
	PlanStatusDropped  = "dropped"
)

// ForecastBranch represents a candidate narrative branch / timeline option.
type ForecastBranch struct {
	ID             string `json:"id"`
	BeatsSummary   string `json:"beats_summary"`
	Decisions      string `json:"decisions"`
	ExpectedDeltas string `json:"expected_deltas"`
	Risks          string `json:"risks"`
	IntentMatch    string `json:"intent_match"`
}

// Forecast encapsulates the narrative tree forecast with multiple branches.
type Forecast struct {
	ID          string           `json:"id"`
	ProjectID   string           `json:"project_id"`
	BaseChapter string           `json:"base_chapter"`
	Divergence  string           `json:"divergence"`
	BranchCount int              `json:"branch_count"`
	Horizon     int              `json:"horizon"`
	Fingerprint string           `json:"fingerprint"`
	Status      string           `json:"status"` // active | stale | archived
	Branches    []ForecastBranch `json:"branches"`
	CreatedAt   time.Time        `json:"created_at"`
}

// PlanChapter represents an individual chapter's consumption state within an active plan.
type PlanChapter struct {
	ChapterIndex int    `json:"chapter_index"`
	Title        string `json:"title,omitempty"`
	Summary      string `json:"summary,omitempty"`
	Status       string `json:"status"` // active | consumed
}

// ActivePlan represents the chosen forecast branch and its execution state.
type ActivePlan struct {
	ProjectID      string        `json:"project_id"`
	ForecastID     string        `json:"forecast_id"`
	BranchID       string        `json:"branch_id"`
	PlanText       string        `json:"plan_text"`
	Status         string        `json:"status"` // active | consumed | diverged | stale | dropped
	PlanChapters   []PlanChapter `json:"plan_chapters,omitempty"`
	ExpectedDeltas string        `json:"expected_deltas,omitempty"`
	WarningMessage string        `json:"warning_message,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
