package domain

// ContextTrace records the token budget allocation, itemized accounting,
// and truncation history of CanonChronicle horizon assembly.
type ContextTrace struct {
	ProtectedIDs     []string `json:"protected_ids"`
	CompressibleIDs  []string `json:"compressible_ids"`
	DroppedIDs       []string `json:"dropped_ids"`
	BudgetTokens     int      `json:"budget_tokens"`
	EstimatedTokens  int      `json:"estimated_tokens"`
	Truncated        bool     `json:"truncated"`
	CompactionFailed bool     `json:"compaction_failed,omitempty"`
	CompactionNote   string   `json:"compaction_note,omitempty"`
}

// EstimateTokens provides a lightweight heuristic token estimation for context pieces:
// len([]rune(s))*3/4.
// Note: This is an empirical coarse estimation (typically ~0.75 tokens per rune,
// shared with the prompt-preview estimator so budget and preview stay consistent).
// Full accurate tokenization (via tiktoken/BPE) is scheduled for P1.
func EstimateTokens(s string) int {
	return len([]rune(s)) * 3 / 4
}
