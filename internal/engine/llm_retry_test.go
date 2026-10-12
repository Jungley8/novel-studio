package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIsRetryableLLMError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"io.EOF", io.EOF, true},
		{"unexpected EOF", io.ErrUnexpectedEOF, true},
		{"wrapped EOF", fmt.Errorf("do stream HTTP request: %w", io.EOF), true},
		{"eof text", errors.New(`Post "https://x": EOF`), true},
		{"connection reset", errors.New("read: connection reset by peer"), true},
		{"broken pipe", errors.New("write: broken pipe"), true},
		{"http 500", errors.New("LLM API returned HTTP 500: boom"), true},
		{"http 429", errors.New("LLM API returned HTTP 429: slow down"), true},
		{"stream http 502", errors.New("LLM Stream API returned HTTP 502: bad gateway"), true},
		{"http 401", errors.New("LLM API returned HTTP 401: unauthorized"), false},
		{"http 400", errors.New("LLM API returned HTTP 400: bad request"), false},
		{"parse error", errors.New("unmarshal LLM response failed: invalid character 'x'"), false},
		{"business error", errors.New("LLM API error: FreeTierError: nope"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableLLMError(tc.err); got != tc.want {
				t.Fatalf("isRetryableLLMError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// closeConnectionOnceThenServe aborts the first request at the transport level
// (simulating the upstream EOF that halted chapter 1) and serves a valid
// response for every later attempt.
func newFlakyServer(t *testing.T, okBody string, contentType string) (*httptest.Server, *int32) {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Errorf("hijacking unsupported")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = io.WriteString(w, okBody)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

func TestChatCompletionRetriesTransientEOF(t *testing.T) {
	srv, count := newFlakyServer(t, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`, "application/json")

	c := NewHTTPLLMClient(srv.URL, "test-key")
	content, usage, err := c.ChatCompletionWithUsage(context.Background(), "test-model", "sys", "user", 0.5)
	if err != nil {
		t.Fatalf("transient EOF must be retried, got %v", err)
	}
	if content != "ok" || usage.TotalTokens != 4 {
		t.Fatalf("unexpected result content=%q usage=%+v", content, usage)
	}
	if atomic.LoadInt32(count) != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", atomic.LoadInt32(count))
	}
}

func TestChatCompletionDoesNotRetryBusinessError(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"unauthorized"}}`)
	}))
	defer srv.Close()

	c := NewHTTPLLMClient(srv.URL, "test-key")
	if _, _, err := c.ChatCompletionWithUsage(context.Background(), "test-model", "sys", "user", 0.5); err == nil {
		t.Fatal("401 must fail")
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("401 must not be retried, got %d attempts", atomic.LoadInt32(&count))
	}
}

func TestResponsesRequestIncludesMaxOutputTokens(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"output_text":"好","usage":{"input_tokens":3,"output_tokens":1,"total_tokens":4}}`)
	}))
	defer srv.Close()

	c := NewHTTPLLMClient(srv.URL+"/responses", "test-key")
	content, _, err := c.ChatCompletionWithUsage(context.Background(), "muse-like", "sys", "user", 0.5)
	if err != nil {
		t.Fatalf("/responses call failed: %v", err)
	}
	if content != "好" {
		t.Fatalf("unexpected content %q", content)
	}
	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("decode captured body: %v (body=%s)", err, gotBody)
	}
	if body["max_output_tokens"] != float64(responsesMaxOutputTokens) {
		t.Fatalf("expected max_output_tokens=%d in /responses body, got %v (body=%s)", responsesMaxOutputTokens, body["max_output_tokens"], gotBody)
	}
}

func TestStreamFallbackRetriesTransientEOF(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"好\"},\"finish_reason\":null}]}\n\n" +
		"data: [DONE]\n\n"
	srv, count := newFlakyServer(t, sse, "text/event-stream")

	c := NewHTTPLLMClient(srv.URL, "test-key")
	content, _, err := SafeChatCompletionWithStreamFallback(context.Background(), c, "test-model", "sys", "user", 0.5, nil)
	if err != nil {
		t.Fatalf("transient EOF on stream must be retried, got %v", err)
	}
	if !strings.Contains(content, "好") {
		t.Fatalf("unexpected content %q", content)
	}
	if atomic.LoadInt32(count) != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", atomic.LoadInt32(count))
	}
}
