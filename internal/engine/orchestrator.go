package engine

import (
	"strings"
)

// Orchestrator coordinates the reasoning engine and writing engine.
type Orchestrator struct {
	client LLMClient
}

func NewOrchestrator(client LLMClient) *Orchestrator {
	return &Orchestrator{client: client}
}

func (o *Orchestrator) SetClient(client LLMClient) {
	o.client = client
}

func extractJSON(s string) string {
	if valid, err := ExtractAndCleanJSON(s); err == nil {
		return valid
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}
