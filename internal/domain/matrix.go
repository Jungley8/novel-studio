package domain

import (
	"errors"
	"strings"
	"time"
)

// MarkerType represents category of in-line editorial annotations.
type MarkerType string

const (
	MarkerTypeTodo       MarkerType = "TODO"
	MarkerTypePlotHole   MarkerType = "PLOT_HOLE"
	MarkerTypeHookAnchor MarkerType = "HOOK_ANCHOR"
	MarkerTypeNote       MarkerType = "NOTE"
)

// SceneMarker represents an editorial highlight or annotation in a scene manuscript.
type SceneMarker struct {
	ID             string     `json:"id"`
	SceneID        string     `json:"scene_id"`
	MarkerType     MarkerType `json:"marker_type"`
	Color          string     `json:"color"`
	TextRangeStart int        `json:"text_range_start"`
	TextRangeEnd   int        `json:"text_range_end"`
	Content        string     `json:"content"`
	Resolved       bool       `json:"resolved"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Scene represents an atomic narrative beat / scene block within a chapter.
type Scene struct {
	ID              string        `json:"id"`
	ChapterID       string        `json:"chapter_id"`
	ProjectID       string        `json:"project_id"`
	SceneIndex      int           `json:"scene_index"`
	Title           string        `json:"title"`
	LocationEntryID string        `json:"location_entry_id,omitempty"`
	DramaticGoal    string        `json:"dramatic_goal"`
	ConflictBarrier string        `json:"conflict_barrier"`
	TensionLevel    int           `json:"tension_level"` // 1-10
	Beats           []SceneBeat   `json:"beats,omitempty"`
	StateMutation   StateMutation `json:"state_mutation,omitempty"`
	ProseContent    string        `json:"prose_content"`
	WordCount       int           `json:"word_count"`
	ExcludeFromAI   bool          `json:"exclude_from_ai"`
	Markers         []SceneMarker `json:"markers,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

func (s *Scene) Validate() error {
	if s.ChapterID == "" {
		return errors.New("scene chapter_id cannot be empty")
	}
	if s.SceneIndex <= 0 {
		s.SceneIndex = 1
	}
	if s.TensionLevel <= 0 {
		s.TensionLevel = 5
	}
	if s.TensionLevel > 10 {
		s.TensionLevel = 10
	}
	s.Title = strings.TrimSpace(s.Title)
	return nil
}

// MatrixChapterRow represents a chapter row with its nested scenes in the Matrix overview.
type MatrixChapterRow struct {
	Chapter *Chapter `json:"chapter"`
	Scenes  []*Scene `json:"scenes"`
}

// MatrixVolumeGroup represents a volume group containing chapters in the Matrix.
type MatrixVolumeGroup struct {
	VolumeIndex int                 `json:"volume_index"`
	Title       string              `json:"title"`
	Theme       string              `json:"theme"`
	CoreGoal    string              `json:"core_goal"`
	Climax      string              `json:"climax"`
	Chapters    []*MatrixChapterRow `json:"chapters"`
	TotalWords  int                 `json:"total_words"`
	AvgTension  float64             `json:"avg_tension"`
}

// MatrixOverview encapsulates the entire book's multi-dimensional matrix planning grid.
type MatrixOverview struct {
	ProjectID   string               `json:"project_id"`
	Volumes     []*MatrixVolumeGroup `json:"volumes"`
	OrphanRows  []*MatrixChapterRow  `json:"orphan_rows,omitempty"` // Chapters not assigned to a volume
	TotalScenes int                  `json:"total_scenes"`
	TotalWords  int                  `json:"total_words"`
}
