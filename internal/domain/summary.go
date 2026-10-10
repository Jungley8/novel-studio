package domain

import "time"

// ChapterSummary represents a concise (<= 100 characters) natural language summary
// of a single chapter, capturing key characters, conflict, outcome, and unresolved suspense.
type ChapterSummary struct {
	ProjectID    string    `json:"project_id"`
	ChapterIndex int       `json:"chapter_index"`
	Text         string    `json:"text"`
	Model        string    `json:"model,omitempty"`
	Tokens       int       `json:"tokens,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// VolumeRetrospective encapsulates a macro-level review (<= 300 characters) of an entire book volume,
// covering what happened, what was left behind, and what unresolved hooks remain across the volume arc.
type VolumeRetrospective struct {
	ProjectID   string    `json:"project_id"`
	VolumeIndex int       `json:"volume_index"`
	Text        string    `json:"text"`
	FromChapter int       `json:"from_chapter"`
	ToChapter   int       `json:"to_chapter"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}
