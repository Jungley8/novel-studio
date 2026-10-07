package engine_test

import (
	"context"
	"testing"

	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestHTTPLLMClient_UpdateCredentials(t *testing.T) {
	client := engine.NewHTTPLLMClient("https://api.initial.com", "key-1")

	// Calling with missing key error scenario
	client.UpdateCredentials("https://api.updated.com/v1/", "")
	_, err := client.ChatCompletion(context.Background(), "gpt-4", "sys", "user", 0.7)
	if err == nil {
		t.Fatal("expected error on empty API key")
	}

	// Update with valid key
	client.UpdateCredentials("https://api.updated.com", "key-2")
}
