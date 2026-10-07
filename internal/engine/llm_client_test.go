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

func TestLLMRouter_RoleDispatch(t *testing.T) {
	defaultClient := engine.NewHTTPLLMClient("https://api.default.com", "default-key")
	router := engine.NewLLMRouter(defaultClient)

	// Custom configuration with independent Reviewer (Claude/OpenAI) and Reasoner (R1)
	writerClient := engine.NewHTTPLLMClient("https://api.deepseek.com", "ds-key")
	reviewerClient := engine.NewHTTPLLMClient("https://api.anthropic.com", "claude-key")

	router.ConfigureRole(engine.RoleWriter, writerClient, "deepseek-v3")
	router.ConfigureRole(engine.RoleReviewer, reviewerClient, "claude-3-5-sonnet")

	wClient, wModel := router.ClientForRole(engine.RoleWriter)
	if wModel != "deepseek-v3" {
		t.Errorf("expected writer model deepseek-v3, got %s", wModel)
	}
	if wClient != writerClient {
		t.Errorf("expected custom writerClient")
	}

	rClient, rModel := router.ClientForRole(engine.RoleReviewer)
	if rModel != "claude-3-5-sonnet" {
		t.Errorf("expected reviewer model claude-3-5-sonnet, got %s", rModel)
	}
	if rClient != reviewerClient {
		t.Errorf("expected custom reviewerClient")
	}

	// Reasoner was not explicitly configured -> falls back safely to default client
	rsClient, _ := router.ClientForRole(engine.RoleReasoner)
	if rsClient != defaultClient {
		t.Errorf("expected defaultClient for unconfigured reasoner")
	}
}
