package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestLLMRouter_ContextAndModelRouting(t *testing.T) {
	calledRole := ""
	tsWriter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledRole = "writer"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"writer output"}}],"usage":{"total_tokens":10}}`))
	}))
	defer tsWriter.Close()

	tsReviewer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledRole = "reviewer"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"reviewer output"}}],"usage":{"total_tokens":25}}`))
	}))
	defer tsReviewer.Close()

	defaultCl := engine.NewHTTPLLMClient("https://api.default.com", "def-key")
	router := engine.NewLLMRouter(defaultCl)

	writerCl := engine.NewHTTPLLMClient(tsWriter.URL, "writer-key")
	reviewerCl := engine.NewHTTPLLMClient(tsReviewer.URL, "reviewer-key")

	router.ConfigureRole(engine.RoleWriter, writerCl, "writer-model")
	router.ConfigureRole(engine.RoleReviewer, reviewerCl, "reviewer-model")

	// 1. ContextWithRole routes to writer
	ctxWriter := engine.ContextWithRole(context.Background(), engine.RoleWriter)
	resp, usage, err := router.ChatCompletionWithUsage(ctxWriter, "", "sys", "user", 0.7)
	if err != nil {
		t.Fatalf("writer ChatCompletionWithUsage failed: %v", err)
	}
	if calledRole != "writer" || resp != "writer output" || usage.TotalTokens != 10 {
		t.Fatalf("expected writer response, got role=%s resp=%s tokens=%d", calledRole, resp, usage.TotalTokens)
	}

	// 2. ContextWithRole routes to reviewer
	ctxReviewer := engine.ContextWithRole(context.Background(), engine.RoleReviewer)
	respRev, usageRev, err := router.ChatCompletionWithUsage(ctxReviewer, "", "sys", "user", 0.3)
	if err != nil {
		t.Fatalf("reviewer ChatCompletionWithUsage failed: %v", err)
	}
	if calledRole != "reviewer" || respRev != "reviewer output" || usageRev.TotalTokens != 25 {
		t.Fatalf("expected reviewer response, got role=%s resp=%s tokens=%d", calledRole, respRev, usageRev.TotalTokens)
	}

	// 3. Model match routes to reviewer without role in context
	calledRole = ""
	respModel, err := router.ChatCompletion(context.Background(), "reviewer-model", "sys", "user", 0.3)
	if err != nil {
		t.Fatalf("model match failed: %v", err)
	}
	if calledRole != "reviewer" || respModel != "reviewer output" {
		t.Fatalf("expected model match to route to reviewer, got %s", calledRole)
	}
}

func TestHTTPLLMClient_StreamCancellationLeakPrevention(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// Continuously write chunks until connection closes
		for i := 0; i < 200; i++ {
			_, err := w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"chunk\"}}]}\n\n"))
			if err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	ctx, cancel := context.WithCancel(context.Background())

	streamCh, err := client.ChatCompletionStream(ctx, "model", "sys", "user", 0.7)
	if err != nil {
		t.Fatalf("stream creation failed: %v", err)
	}

	// Read only 1 chunk then immediately cancel context without consuming the rest of streamCh
	<-streamCh
	cancel()

	// Wait briefly: the sender goroutine must not block or leak
	// Closing channel indicates goroutine has terminated cleanly
	done := make(chan bool)
	go func() {
		for range streamCh {
			// drain until closed
		}
		done <- true
	}()

	select {
	case <-done:
		// success: stream channel was closed cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine blocked on sending to unconsumed stream channel: leak detected")
	}
}

func TestHTTPLLMClient_BoundedRetryThenFailClosedOn429(t *testing.T) {
	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	_, _, err := client.ChatCompletionWithUsage(context.Background(), "test-model", "sys", "user", 0.7)
	if err == nil {
		t.Fatalf("expected error after exhausting bounded retries on a persistent 429, got nil")
	}
	// 1 initial attempt + 2 bounded, backoff-gated retries — not a retry storm.
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (1 initial + 2 bounded retries) on 429, got %d", got)
	}
}

func TestHTTPLLMClient_TimeoutStopsRetryStorm(t *testing.T) {
	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		// Simulate severe hanging / queue congestion: sleep longer than timeout
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"late"}}]}`))
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	// Set very short timeout ctx to simulate deadline exceeded awaiting headers
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err := client.ChatCompletionWithUsage(ctx, "step-5-preview-free", "sys", "user", 0.7)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	// Fail-Closed & Fail-Fast: must NOT perform 3 attempts on timeout
	if attempts.Load() > 1 {
		t.Fatalf("expected retry storm to be stopped on timeout (attempts <= 1), got %d attempts", attempts.Load())
	}
}

func TestSafeChatCompletionWithStreamFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"流式\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"精修正文\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"total_tokens\":30}}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	var deltas []string
	content, usage, err := engine.SafeChatCompletionWithStreamFallback(
		context.Background(),
		client,
		"writer-model",
		"sys",
		"user",
		0.7,
		func(d string) {
			deltas = append(deltas, d)
		},
	)
	if err != nil {
		t.Fatalf("SafeChatCompletionWithStreamFallback failed: %v", err)
	}
	if content != "流式精修正文" {
		t.Fatalf("expected '流式精修正文', got '%s'", content)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d", len(deltas))
	}
	if usage.TotalTokens != 30 {
		t.Fatalf("expected 30 tokens, got %d", usage.TotalTokens)
	}
}

func TestStream_OutputLimitContinuation(t *testing.T) {
	var requestCount atomic.Int32
	contentA := "长篇故事第一回，风雪交加之时，主角在破庙中避寒待续"
	contentB := "避寒待续。忽闻庙门外传来一阵急促马蹄声，踏雪而来。"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqNum := requestCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		if reqNum == 1 {
			_, _ = w.Write([]byte(fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", contentA)))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"total_tokens\":30}}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			return
		}

		if reqNum == 2 {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			msgs, _ := body["messages"].([]interface{})
			if len(msgs) != 4 {
				t.Errorf("expected 4 messages on continuation, got %d", len(msgs))
			}

			_, _ = w.Write([]byte(fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", contentB)))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":30,\"completion_tokens\":40,\"total_tokens\":70}}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			return
		}

		t.Errorf("unexpected request count: %d", reqNum)
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	got, usage, err := engine.SafeChatCompletionWithStreamFallback(
		context.Background(),
		client,
		"writer-model",
		"sys",
		"user",
		0.7,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := engine.MergeExactContinuation(contentA, contentB)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if requestCount.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount.Load())
	}
	if usage.TotalTokens != 100 {
		t.Fatalf("expected usage.TotalTokens 100 (30+70), got %d", usage.TotalTokens)
	}
	if usage.PromptTokens != 40 || usage.CompletionTokens != 60 {
		t.Fatalf("expected prompt 40, comp 60, got prompt %d, comp %d", usage.PromptTokens, usage.CompletionTokens)
	}
}

func TestStream_OutputLimitExhausted(t *testing.T) {
	var requestCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"截断片段\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":10,\"total_tokens\":20}}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	modelName := "deepseek-writer"
	content, _, err := engine.SafeChatCompletionWithStreamFallback(
		context.Background(),
		client,
		modelName,
		"sys",
		"user",
		0.7,
		nil,
	)
	if err == nil {
		t.Fatalf("expected error when output limit is exhausted after 3 continuations, got content %q", content)
	}
	if !strings.Contains(err.Error(), "续写 3 次后仍被截断") {
		t.Errorf("expected error message to contain '续写 3 次后仍被截断', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), modelName) {
		t.Errorf("expected error message to contain model name %q, got %q", modelName, err.Error())
	}
	// Initial request (1) + 3 continuation requests (3) = 4 total requests
	if requestCount.Load() != 4 {
		t.Fatalf("expected strictly 4 requests (1 initial + 3 continuations), got %d", requestCount.Load())
	}
}

func TestStream_NetworkErrorRetriesBoundedThenFails(t *testing.T) {
	var requestCount, continuationShaped atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if msgs, ok := body["messages"].([]interface{}); ok && len(msgs) >= 4 {
				continuationShaped.Add(1)
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"开始生成一部分\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		// Abruptly close underlying connection
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	_, _, err := engine.SafeChatCompletionWithStreamFallback(
		context.Background(),
		client,
		"writer-model",
		"sys",
		"user",
		0.7,
		nil,
	)
	if err == nil {
		t.Fatalf("expected error after exhausting bounded retries on a persistent stream cut, got nil")
	}
	// 1 initial attempt + 2 bounded retries; the transport error must still
	// bubble up (fail-closed) once the retry budget is spent.
	if got := requestCount.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (1 initial + 2 bounded retries), got %d", got)
	}
	// Every attempt must be a fresh initial request (system+user), never an
	// output-limit continuation (4 messages) — network retries must not
	// masquerade as continuations.
	if got := continuationShaped.Load(); got != 0 {
		t.Fatalf("network retries sent %d continuation-shaped requests, want 0", got)
	}
}

func TestSync_OutputLimitContinuation(t *testing.T) {
	var requestCount atomic.Int32
	part1 := "非流式正文第一段，夜幕降临待续"
	part2 := "夜幕降临待续。烛火摇曳，杀机四伏。"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqNum := requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if reqNum == 1 {
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"choices": [
					{
						"message": {"content": %q},
						"finish_reason": "length"
					}
				],
				"usage": {"prompt_tokens": 15, "completion_tokens": 25, "total_tokens": 40}
			}`, part1)))
			return
		}

		if reqNum == 2 {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			msgs, _ := body["messages"].([]interface{})
			if len(msgs) != 4 {
				t.Errorf("expected 4 messages on continuation, got %d", len(msgs))
			}

			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"choices": [
					{
						"message": {"content": %q},
						"finish_reason": "stop"
					}
				],
				"usage": {"prompt_tokens": 35, "completion_tokens": 45, "total_tokens": 80}
			}`, part2)))
			return
		}

		t.Errorf("unexpected request count: %d", reqNum)
	}))
	defer ts.Close()

	client := engine.NewHTTPLLMClient(ts.URL, "test-key")
	got, usage, err := client.ChatCompletionWithUsage(
		context.Background(),
		"writer-model",
		"sys",
		"user",
		0.7,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := engine.MergeExactContinuation(part1, part2)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if requestCount.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount.Load())
	}
	if usage.TotalTokens != 120 {
		t.Fatalf("expected usage.TotalTokens 120 (40+80), got %d", usage.TotalTokens)
	}
	if usage.PromptTokens != 50 || usage.CompletionTokens != 70 {
		t.Fatalf("expected prompt 50, comp 70, got prompt %d, comp %d", usage.PromptTokens, usage.CompletionTokens)
	}
}

func TestStream_FinishReasonStopNoContinuation(t *testing.T) {
	var streamRequests atomic.Int32
	var syncRequests atomic.Int32

	tsStream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamRequests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"完整流式内容\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":15,\"total_tokens\":25}}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer tsStream.Close()

	tsSync := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		syncRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {"content": "完整非流式内容"},
					"finish_reason": "stop"
				}
			],
			"usage": {"prompt_tokens": 12, "completion_tokens": 18, "total_tokens": 30}
		}`))
	}))
	defer tsSync.Close()

	ctx := context.Background()

	// 1. Stream regression test
	clientStream := engine.NewHTTPLLMClient(tsStream.URL, "test-key")
	gotStream, usageStream, errStream := engine.SafeChatCompletionWithStreamFallback(
		ctx,
		clientStream,
		"model",
		"sys",
		"user",
		0.7,
		nil,
	)
	if errStream != nil {
		t.Fatalf("SafeChatCompletionWithStreamFallback failed: %v", errStream)
	}
	if gotStream != "完整流式内容" {
		t.Errorf("got %q, want '完整流式内容'", gotStream)
	}
	if streamRequests.Load() != 1 {
		t.Errorf("expected strictly 1 stream request on finish_reason:stop, got %d", streamRequests.Load())
	}
	if usageStream.TotalTokens != 25 {
		t.Errorf("expected 25 total tokens, got %d", usageStream.TotalTokens)
	}

	// 2. Sync regression test
	clientSync := engine.NewHTTPLLMClient(tsSync.URL, "test-key")
	gotSync, usageSync, errSync := clientSync.ChatCompletionWithUsage(
		ctx,
		"model",
		"sys",
		"user",
		0.7,
	)
	if errSync != nil {
		t.Fatalf("ChatCompletionWithUsage failed: %v", errSync)
	}
	if gotSync != "完整非流式内容" {
		t.Errorf("got %q, want '完整非流式内容'", gotSync)
	}
	if syncRequests.Load() != 1 {
		t.Errorf("expected strictly 1 sync request on finish_reason:stop, got %d", syncRequests.Load())
	}
	if usageSync.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", usageSync.TotalTokens)
	}
}
