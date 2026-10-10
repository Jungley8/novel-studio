package domain

import "time"

// StyleVault encapsulates the project-level stylistic contract and golden excerpts.
// Each golden excerpt is guaranteed to be <= 500 runes by the store layer.
type StyleVault struct {
	ProjectID      string    `json:"project_id"`
	GoldenExcerpts []string  `json:"golden_excerpts"`
	UpdatedAt      time.Time `json:"updated_at"`
}
