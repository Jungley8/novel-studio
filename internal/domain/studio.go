package domain

import "time"

type StudioThread struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	ProjectID          string    `json:"project_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	MessageCount       int       `json:"message_count"`
	LastMessagePreview string    `json:"last_message_preview,omitempty"`
}

type StudioMessage struct {
	ID         string    `json:"id"`
	ThreadID   string    `json:"thread_id"`
	Role       string    `json:"role"` // user | assistant
	Content    string    `json:"content"`
	Mode       string    `json:"mode,omitempty"`
	ProjectID  string    `json:"project_id,omitempty"`
	TokenUsage string    `json:"token_usage,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
