package domain

import (
	"time"
)

// ChapterVersionMeta represents lightweight chapter version metadata (without full content).
type ChapterVersionMeta struct { // 列表用，不带正文
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	ChapterIndex int       `json:"chapter_index"`
	Source       string    `json:"source"` // "commit" | "edit"
	WordCount    int       `json:"word_count"`
	ContentHash  string    `json:"content_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

// ChapterVersion represents a full chapter version entry with prose content.
type ChapterVersion struct { // 单条用，带正文
	ChapterVersionMeta
	Content string `json:"content"`
}

// ChapterStateSnapshot captures the world/character and hook state after a chapter is committed.
type ChapterStateSnapshot struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	ChapterIndex    int       `json:"chapter_index"`
	ProtagonistJSON string    `json:"protagonist_json"`
	HooksJSON       string    `json:"hooks_json"`
	CreatedAt       time.Time `json:"created_at"`
}
