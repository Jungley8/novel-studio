package domain

// ContextTrace records the token budget allocation, itemized accounting,
// and truncation history of CanonChronicle horizon assembly.
type ContextTrace struct {
	ProtectedIDs    []string `json:"protected_ids"`
	CompressibleIDs []string `json:"compressible_ids"`
	DroppedIDs      []string `json:"dropped_ids"`
	BudgetTokens    int      `json:"budget_tokens"`
	EstimatedTokens int      `json:"estimated_tokens"`
	Truncated       bool     `json:"truncated"`
}

// EstimateTokens provides a lightweight heuristic token estimation for context pieces:
// len([]rune(s))/2.
// Note: This is an empirical coarse estimation (typically ~1 token per 1.5-2 Chinese runes or words).
// Full accurate tokenization (via tiktoken/BPE) is scheduled for P1.
func EstimateTokens(s string) int {
	return len([]rune(s)) / 2
}
