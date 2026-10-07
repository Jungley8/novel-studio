package engine_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestHTTPLLMClient_OpenCodeAndResponsesCompatibility(t *testing.T) {
	calledPath := ""
	receivedSession := ""
	var lastRequestBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path
		receivedSession = r.Header.Get("x-opencode-session")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		lastRequestBody = body

		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"流式\"}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"推演\"}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":5,\"output_tokens\":10,\"total_tokens\":15}}}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/opencode/zen/go/v1/responses" {
			// Real OpenCode Responses API style response with nested content and input_tokens/output_tokens
			_, _ = w.Write([]byte(`{
				"output": [
					{"type": "reasoning", "encrypted_content": "abc"},
					{
						"type": "message",
						"role": "assistant",
						"content": [{"type": "output_text", "text": "来自 OpenCode Responses API 的推演正文"}]
					}
				],
				"usage": {"input_tokens": 10, "output_tokens": 20, "total_tokens": 30}
			}`))
		} else {
			// Standard Chat Completions style response
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"来自 OpenCode Chat Completions 的正文"}}],"usage":{"prompt_tokens":5,"completion_tokens":15,"total_tokens":20}}`))
		}
	}))
	defer ts.Close()

	ctx := context.Background()

	// Test A: User configures baseURL with /responses and opencode domain
	clientA := engine.NewHTTPLLMClient(ts.URL+"/opencode/zen/go/v1/responses", "test-key")
	gotA, usageA, errA := clientA.ChatCompletionWithUsage(ctx, "muse-spark-1.3-contributor", "sys-instructions", "user-prompt", 0.7)
	if errA != nil {
		t.Fatalf("ChatCompletionWithUsage with /responses failed: %v", errA)
	}
	if calledPath != "/opencode/zen/go/v1/responses" {
		t.Errorf("expected path /opencode/zen/go/v1/responses, got %s", calledPath)
	}
	if receivedSession == "" {
		t.Errorf("expected x-opencode-session header to be sent for opencode endpoint")
	}
	// Verify that /responses strictly does NOT have "messages" parameter
	if _, hasMessages := lastRequestBody["messages"]; hasMessages {
		t.Errorf("Responses API payload MUST NOT include 'messages' parameter, got %v", lastRequestBody["messages"])
	}
	if lastRequestBody["instructions"] != "sys-instructions" {
		t.Errorf("expected instructions 'sys-instructions', got %v", lastRequestBody["instructions"])
	}
	if lastRequestBody["input"] != "user-prompt" {
		t.Errorf("expected input 'user-prompt', got %v", lastRequestBody["input"])
	}
	if gotA != "来自 OpenCode Responses API 的推演正文" {
		t.Errorf("unexpected output: %s", gotA)
	}
	if usageA.PromptTokens != 10 || usageA.CompletionTokens != 20 || usageA.TotalTokens != 30 {
		t.Errorf("expected 10/20/30 tokens, got %d/%d/%d", usageA.PromptTokens, usageA.CompletionTokens, usageA.TotalTokens)
	}

	// Test B: Stream with /responses
	streamCh, errStream := clientA.ChatCompletionStream(ctx, "muse-spark-1.3-contributor", "sys-instructions", "user-prompt", 0.7)
	if errStream != nil {
		t.Fatalf("ChatCompletionStream failed: %v", errStream)
	}
	var streamText string
	var streamUsage *engine.TokenUsage
	for chunk := range streamCh {
		if chunk.Err != nil {
			t.Fatalf("stream chunk error: %v", chunk.Err)
		}
		streamText += chunk.Delta
		if chunk.Usage != nil {
			streamUsage = chunk.Usage
		}
	}
	if streamText != "流式推演" {
		t.Errorf("expected streamText '流式推演', got %q", streamText)
	}
	if streamUsage == nil || streamUsage.TotalTokens != 15 {
		t.Errorf("expected stream usage total tokens 15, got %+v", streamUsage)
	}

	// Test C: Standard BaseURL uses /chat/completions and messages payload
	clientB := engine.NewHTTPLLMClient(ts.URL+"/opencode/zen/go/v1", "test-key")
	gotB, usageB, errB := clientB.ChatCompletionWithUsage(ctx, "muse-spark-1.3-contributor", "sys", "user", 0.7)
	if errB != nil {
		t.Fatalf("ChatCompletionWithUsage with base URL failed: %v", errB)
	}
	if calledPath != "/opencode/zen/go/v1/chat/completions" {
		t.Errorf("expected path /opencode/zen/go/v1/chat/completions, got %s", calledPath)
	}
	if _, hasMessages := lastRequestBody["messages"]; !hasMessages {
		t.Errorf("Chat Completions payload MUST include 'messages' parameter")
	}
	if _, hasInput := lastRequestBody["input"]; hasInput {
		t.Errorf("Chat Completions payload MUST NOT include 'input' parameter")
	}
	if gotB != "来自 OpenCode Chat Completions 的正文" {
		t.Errorf("unexpected output: %s", gotB)
	}
	if usageB.TotalTokens != 20 {
		t.Errorf("expected 20 total tokens, got %d", usageB.TotalTokens)
	}
}

