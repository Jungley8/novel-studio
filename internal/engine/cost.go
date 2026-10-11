package engine

// EstimateCostUSD calculates estimated cost in USD for a given token usage.
// Rate: ~$0.00014 per 1k prompt tokens, ~$0.00028 per 1k completion tokens.
func EstimateCostUSD(usage TokenUsage) float64 {
	return (float64(usage.PromptTokens)*0.00014 + float64(usage.CompletionTokens)*0.00028) / 1000.0
}
