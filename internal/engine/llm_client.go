package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
)

// TokenUsage holds token usage metrics from an LLM call.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streamed token or final usage report.
type StreamChunk struct {
	Delta        string      `json:"delta"`
	Usage        *TokenUsage `json:"usage,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Done         bool        `json:"done"`
	Err          error       `json:"err,omitempty"`
}

// LLMClient represents a standard chat completion provider supporting sync, usage, and streaming.
type LLMClient interface {
	ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error)
	ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error)
	ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error)
}

// LLMRole distinguishes specific generation roles for multi-provider dispatch.
type LLMRole string

const (
	RoleReasoner LLMRole = "reasoner"
	RoleWriter   LLMRole = "writer"
	RoleReviewer LLMRole = "reviewer"
)

type HTTPLLMClient struct {
	mu           sync.RWMutex
	baseURL      string
	apiKey       string
	sessionID    string
	httpClient   *http.Client
	streamClient *http.Client
}

func newOptimizedTransport(responseHeaderTimeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
}

func NewHTTPLLMClient(baseURL, apiKey string) *HTTPLLMClient {
	c := &HTTPLLMClient{
		sessionID: fmt.Sprintf("novel-studio-%x", time.Now().UnixNano()),
		httpClient: &http.Client{
			Transport: newOptimizedTransport(180 * time.Second),
			Timeout:   300 * time.Second, // 300s total timeout for sync completion
		},
		streamClient: &http.Client{
			Transport: newOptimizedTransport(120 * time.Second),
			Timeout:   0, // Streaming connection lifetime is managed by ctx; no premature cutoff
		},
	}
	c.UpdateCredentials(baseURL, apiKey)
	return c
}

func (c *HTTPLLMClient) SetSessionID(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = sessionID
}

func (c *HTTPLLMClient) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

func (c *HTTPLLMClient) applyHeaders(req *http.Request, apiKey, endpoint string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	c.mu.RLock()
	sessionID := c.sessionID
	c.mu.RUnlock()
	if sessionID == "" {
		sessionID = fmt.Sprintf("novel-studio-%x", time.Now().UnixNano())
	}

	// OpenCode Go & Zen requires x-opencode-session for cache routing & session affinity
	// See: https://opencode.ai/docs/go/#where-can-i-use-it
	if strings.Contains(endpoint, "opencode") {
		req.Header.Set("x-opencode-session", sessionID)
	}
}

func (c *HTTPLLMClient) UpdateCredentials(baseURL, apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	c.baseURL = cleanURL
	c.apiKey = apiKey
}

func (c *HTTPLLMClient) endpointFor(baseURL string) string {
	clean := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if clean == "" {
		return ""
	}
	// Direct path match
	if strings.HasSuffix(clean, "/chat/completions") || strings.HasSuffix(clean, "/responses") {
		return clean
	}
	// Path with /v1 or /go/v1 or custom gateway version prefix
	if strings.HasSuffix(clean, "/v1") || strings.Contains(clean, "/v1/") || strings.Contains(clean, "/v1") {
		return clean + "/chat/completions"
	}
	return clean + "/v1/chat/completions"
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model         string                 `json:"model"`
	Messages      []chatMessage          `json:"messages,omitempty"`
	Input         interface{}            `json:"input,omitempty"`
	Instructions  string                 `json:"instructions,omitempty"`
	Temperature   float64                `json:"temperature"`
	Stream        bool                   `json:"stream,omitempty"`
	StreamOptions map[string]interface{} `json:"stream_options,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	OutputText   string          `json:"output_text"`
	Output       json.RawMessage `json:"output"`
	FinishReason string          `json:"finish_reason"`
	Usage        *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		InputTokens      int `json:"input_tokens"`
		OutputTokens     int `json:"output_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func extractFinishReason(resp *chatResponse) string {
	if resp == nil {
		return ""
	}
	if len(resp.Choices) > 0 && resp.Choices[0].FinishReason != "" {
		return resp.Choices[0].FinishReason
	}
	if resp.FinishReason != "" {
		return resp.FinishReason
	}
	return ""
}

func extractOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(raw, &items); err == nil {
		var sb strings.Builder
		for _, item := range items {
			if t, ok := item["text"].(string); ok && t != "" {
				sb.WriteString(t)
				continue
			}
			if c, ok := item["content"].(string); ok && c != "" {
				sb.WriteString(c)
				continue
			}
			if parts, ok := item["content"].([]interface{}); ok {
				for _, p := range parts {
					if pMap, ok := p.(map[string]interface{}); ok {
						if t, ok := pMap["text"].(string); ok && t != "" {
							sb.WriteString(t)
						}
					}
				}
			}
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}

	var single map[string]interface{}
	if err := json.Unmarshal(raw, &single); err == nil {
		if t, ok := single["text"].(string); ok && t != "" {
			return t
		}
		if c, ok := single["content"].(string); ok && c != "" {
			return c
		}
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}

	return ""
}

func extractTokenUsage(u *struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
}) TokenUsage {
	if u == nil {
		return TokenUsage{}
	}
	prompt := u.PromptTokens
	if prompt == 0 {
		prompt = u.InputTokens
	}
	comp := u.CompletionTokens
	if comp == 0 {
		comp = u.OutputTokens
	}
	total := u.TotalTokens
	if total == 0 {
		total = prompt + comp
	}
	return TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: comp,
		TotalTokens:      total,
	}
}

// isTimeoutError determines whether an error stems from connection or read deadline exceeded.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "awaiting headers") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "client.timeout") ||
		strings.Contains(msg, "timeout exceeded")
}

// maxLLMRetryAttempts is the number of EXTRA attempts made for transient
// transport-level failures (EOF, connection reset, timeouts, HTTP 5xx/429).
// Content-level failures (parse errors, 4xx) are never retried.
const maxLLMRetryAttempts = 2

// isRetryableLLMError reports whether err is a transient transport failure
// worth a bounded retry. Business errors (HTTP 4xx other than 429, malformed
// request, empty choices) are not retryable.
func isRetryableLLMError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if isTimeoutError(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{"connection reset", "broken pipe", "unexpected eof", " eof", "eof\"", "connection refused"} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	for _, needle := range []string{
		"llm api returned http 5", "llm api returned http 429",
		"stream api returned http 5", "stream api returned http 429",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// sleepRetryBackoff waits 2s, 4s, ... between attempts; returns false when ctx
// is cancelled first.
func sleepRetryBackoff(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt) * 2 * time.Second
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// chatCompletionOnceWithRetry adds bounded retries around chatCompletionOnce
// for transient transport failures; the caller's ctx gates every retry.
func (c *HTTPLLMClient) chatCompletionOnceWithRetry(ctx context.Context, model string, messages []chatMessage, temperature float64) (string, TokenUsage, string, error) {
	content, usage, finishReason, err := c.chatCompletionOnce(ctx, model, messages, temperature)
	for attempt := 1; err != nil && attempt <= maxLLMRetryAttempts && isRetryableLLMError(err) && ctx.Err() == nil; attempt++ {
		if !sleepRetryBackoff(ctx, attempt) {
			break
		}
		content, usage, finishReason, err = c.chatCompletionOnce(ctx, model, messages, temperature)
	}
	return content, usage, finishReason, err
}

// streamOnceWithRetry adds bounded retries around streamOnce for transient
// transport failures. A retried attempt restarts generation from scratch;
// consumers that received partial deltas (UI streaming) may observe a
// restart, which is strictly better than failing the whole chapter.
func streamOnceWithRetry(ctx context.Context, client LLMClient, model string, messages []chatMessage, temperature float64, onDelta func(string)) (string, TokenUsage, string, error) {
	content, usage, finishReason, err := streamOnce(ctx, client, model, messages, temperature, onDelta)
	for attempt := 1; err != nil && attempt <= maxLLMRetryAttempts && isRetryableLLMError(err) && ctx.Err() == nil; attempt++ {
		if !sleepRetryBackoff(ctx, attempt) {
			break
		}
		content, usage, finishReason, err = streamOnce(ctx, client, model, messages, temperature, onDelta)
	}
	return content, usage, finishReason, err
}

func (c *HTTPLLMClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error) {
	content, _, err := c.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temperature)
	return content, err
}

func (c *HTTPLLMClient) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	initialMessages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	content, usage, finishReason, err := c.chatCompletionOnceWithRetry(ctx, model, initialMessages, temperature)
	if err != nil {
		return "", usage, err
	}

	if !isOutputLimitFinishReason(finishReason) {
		return content, usage, nil
	}

	accumulated := content
	totalUsage := usage

	for continuationCount := 1; continuationCount <= maxOutputLimitContinuations; continuationCount++ {
		contMessages := []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
			{Role: "assistant", Content: accumulated},
			{Role: "user", Content: continuationInstruction()},
		}

		newText, roundUsage, roundFinishReason, roundErr := c.chatCompletionOnce(ctx, model, contMessages, temperature)
		if roundErr != nil {
			return "", totalUsage, roundErr
		}

		addTokenUsage(&totalUsage, roundUsage)
		accumulated = mergeExactContinuation(accumulated, newText)

		if !isOutputLimitFinishReason(roundFinishReason) {
			return accumulated, totalUsage, nil
		}
	}

	return "", totalUsage, fmt.Errorf("模型 [%s] 续写 %d 次后仍被截断", model, maxOutputLimitContinuations)
}

func (c *HTTPLLMClient) chatCompletionOnce(ctx context.Context, model string, messages []chatMessage, temperature float64) (string, TokenUsage, string, error) {
	c.mu.RLock()
	apiKey := c.apiKey
	baseURL := c.baseURL
	c.mu.RUnlock()

	var zeroUsage TokenUsage
	if apiKey == "mock" || apiKey == "offline" || strings.HasPrefix(model, "mock") {
		var sys, user string
		for _, m := range messages {
			if m.Role == "system" && sys == "" {
				sys = m.Content
			} else if m.Role == "user" {
				user = m.Content
			}
		}
		content, usage, err := c.generateMockResponse(model, sys, user)
		return content, usage, "stop", err
	}
	if apiKey == "" {
		return "", zeroUsage, "", errors.New("missing API key: please configure your API key in settings (or use 'mock' for offline testing)")
	}

	endpoint := c.endpointFor(baseURL)
	var reqBody chatRequest
	if strings.HasSuffix(endpoint, "/responses") {
		var instructions string
		var nonSystemMsgs []chatMessage
		for _, m := range messages {
			if m.Role == "system" && instructions == "" {
				instructions = m.Content
			} else {
				nonSystemMsgs = append(nonSystemMsgs, m)
			}
		}
		if len(nonSystemMsgs) == 1 && nonSystemMsgs[0].Role == "user" {
			reqBody = chatRequest{
				Model:        model,
				Instructions: instructions,
				Input:        nonSystemMsgs[0].Content,
				Temperature:  temperature,
			}
		} else {
			reqBody = chatRequest{
				Model:        model,
				Instructions: instructions,
				Input:        nonSystemMsgs,
				Temperature:  temperature,
			}
		}
	} else {
		reqBody = chatRequest{
			Model:       model,
			Messages:    messages,
			Temperature: temperature,
		}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", zeroUsage, "", fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", zeroUsage, "", fmt.Errorf("create request failed: %w", err)
	}

	c.applyHeaders(req, apiKey, endpoint)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", zeroUsage, "", ctx.Err()
		}
		if isTimeoutError(err) {
			return "", zeroUsage, "", fmt.Errorf("模型 [%s] 请求超时: 上游等待首字返回耗时过长 (通常由公用/免费模型并发队列拥堵或长文本推理算力不足引起，建议检查网络或切换为主力写作模型): %w", model, err)
		}
		return "", zeroUsage, "", fmt.Errorf("do HTTP request failed: %w", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		if isTimeoutError(err) {
			return "", zeroUsage, "", fmt.Errorf("模型 [%s] 读取响应超时 (连接过早中断): %w", model, err)
		}
		return "", zeroUsage, "", fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", zeroUsage, "", fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", zeroUsage, "", fmt.Errorf("unmarshal LLM response failed: %w", err)
	}

	if chatResp.Error != nil {
		return "", zeroUsage, "", fmt.Errorf("LLM API error: %s", chatResp.Error.Message)
	}

	var content string
	if len(chatResp.Choices) > 0 && chatResp.Choices[0].Message.Content != "" {
		content = chatResp.Choices[0].Message.Content
	} else if chatResp.OutputText != "" {
		content = chatResp.OutputText
	} else if extracted := extractOutputText(chatResp.Output); extracted != "" {
		content = extracted
	} else {
		return "", zeroUsage, "", errors.New("LLM returned no choices or output content")
	}

	usage := extractTokenUsage(chatResp.Usage)
	finishReason := extractFinishReason(&chatResp)
	return content, usage, finishReason, nil
}

func extractStreamDelta(rawMap map[string]interface{}) string {
	// 1. choices[0].delta.content or text (Standard Chat Completions)
	if choices, ok := rawMap["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok && content != "" {
					return content
				}
				if text, ok := delta["text"].(string); ok && text != "" {
					return text
				}
			}
		}
	}
	// 2. output_text directly
	if ot, ok := rawMap["output_text"].(string); ok && ot != "" {
		return ot
	}
	// 3. delta field (OpenAI Responses API emits string in response.output_text.delta or response.text.delta)
	if deltaRaw, exists := rawMap["delta"]; exists {
		if dStr, ok := deltaRaw.(string); ok && dStr != "" {
			return dStr
		}
		if dMap, ok := deltaRaw.(map[string]interface{}); ok {
			if c, ok := dMap["content"].(string); ok && c != "" {
				return c
			}
			if t, ok := dMap["text"].(string); ok && t != "" {
				return t
			}
		}
	}
	return ""
}

func extractStreamUsage(rawMap map[string]interface{}) *TokenUsage {
	usageRaw, ok := rawMap["usage"].(map[string]interface{})
	if !ok {
		if respMap, ok := rawMap["response"].(map[string]interface{}); ok {
			usageRaw, _ = respMap["usage"].(map[string]interface{})
		}
	}
	if usageRaw == nil {
		return nil
	}
	getInt := func(keys ...string) int {
		for _, k := range keys {
			if v, ok := usageRaw[k].(float64); ok {
				return int(v)
			}
		}
		return 0
	}
	prompt := getInt("prompt_tokens", "input_tokens")
	comp := getInt("completion_tokens", "output_tokens")
	total := getInt("total_tokens")
	if total == 0 {
		total = prompt + comp
	}
	if prompt == 0 && comp == 0 && total == 0 {
		return nil
	}
	return &TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: comp,
		TotalTokens:      total,
	}
}

func extractStreamFinishReason(rawMap map[string]interface{}) string {
	if choices, ok := rawMap["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
				return fr
			}
		}
	}
	if fr, ok := rawMap["finish_reason"].(string); ok && fr != "" {
		return fr
	}
	if respMap, ok := rawMap["response"].(map[string]interface{}); ok {
		if fr, ok := respMap["finish_reason"].(string); ok && fr != "" {
			return fr
		}
	}
	return ""
}

func (c *HTTPLLMClient) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return c.ChatCompletionStreamMessages(ctx, model, messages, temperature)
}

func (c *HTTPLLMClient) ChatCompletionStreamMessages(ctx context.Context, model string, messages []chatMessage, temperature float64) (<-chan StreamChunk, error) {
	c.mu.RLock()
	apiKey := c.apiKey
	baseURL := c.baseURL
	c.mu.RUnlock()

	if apiKey == "mock" || apiKey == "offline" || strings.HasPrefix(model, "mock") {
		out := make(chan StreamChunk, 4)
		go func() {
			defer close(out)
			var sys, user string
			for _, m := range messages {
				if m.Role == "system" && sys == "" {
					sys = m.Content
				} else if m.Role == "user" {
					user = m.Content
				}
			}
			resp, usage, _ := c.generateMockResponse(model, sys, user)
			out <- StreamChunk{Delta: resp, Usage: &usage, FinishReason: "stop"}
		}()
		return out, nil
	}
	if apiKey == "" {
		return nil, errors.New("missing API key: please configure your API key in settings (or use 'mock' for offline testing)")
	}

	endpoint := c.endpointFor(baseURL)
	var reqBody chatRequest
	if strings.HasSuffix(endpoint, "/responses") {
		var instructions string
		var nonSystemMsgs []chatMessage
		for _, m := range messages {
			if m.Role == "system" && instructions == "" {
				instructions = m.Content
			} else {
				nonSystemMsgs = append(nonSystemMsgs, m)
			}
		}
		if len(nonSystemMsgs) == 1 && nonSystemMsgs[0].Role == "user" {
			reqBody = chatRequest{
				Model:        model,
				Instructions: instructions,
				Input:        nonSystemMsgs[0].Content,
				Temperature:  temperature,
				Stream:       true,
			}
		} else {
			reqBody = chatRequest{
				Model:        model,
				Instructions: instructions,
				Input:        nonSystemMsgs,
				Temperature:  temperature,
				Stream:       true,
			}
		}
	} else {
		reqBody = chatRequest{
			Model:       model,
			Messages:    messages,
			Temperature: temperature,
			Stream:      true,
		}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}

	c.applyHeaders(req, apiKey, endpoint)
	req.Header.Set("Accept", "text/event-stream")

	client := c.streamClient
	if client == nil {
		client = c.httpClient
	}

	resp, err := client.Do(req)
	if err != nil {
		if isTimeoutError(err) {
			return nil, fmt.Errorf("模型 [%s] 流式请求等待首字超时 (未在时限内收到响应头): %w", model, err)
		}
		return nil, fmt.Errorf("do stream HTTP request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("LLM Stream API returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	out := make(chan StreamChunk, 64)

	go func() {
		defer resp.Body.Close()
		defer close(out)

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		sendChunk := func(chunk StreamChunk) bool {
			select {
			case out <- chunk:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				sendChunk(StreamChunk{Err: ctx.Err()})
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "[DONE]" {
					sendChunk(StreamChunk{Done: true})
					return
				}

				var rawMap map[string]interface{}
				if err := json.Unmarshal([]byte(data), &rawMap); err != nil {
					continue
				}

				deltaText := extractStreamDelta(rawMap)
				usage := extractStreamUsage(rawMap)
				finishReason := extractStreamFinishReason(rawMap)

				if deltaText != "" || usage != nil || finishReason != "" {
					if !sendChunk(StreamChunk{
						Delta:        deltaText,
						Usage:        usage,
						FinishReason: finishReason,
					}) {
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			if isTimeoutError(err) {
				sendChunk(StreamChunk{Err: fmt.Errorf("模型 [%s] 流式接收中断 (连接超时): %w", model, err)})
			} else {
				sendChunk(StreamChunk{Err: err})
			}
		}
	}()

	return out, nil
}

func streamOnce(
	ctx context.Context,
	client LLMClient,
	model string,
	messages []chatMessage,
	temperature float64,
	onDelta func(string),
) (string, TokenUsage, string, error) {
	var streamCh <-chan StreamChunk
	var err error

	type messageStreamer interface {
		ChatCompletionStreamMessages(ctx context.Context, model string, messages []chatMessage, temperature float64) (<-chan StreamChunk, error)
	}

	if ms, ok := client.(messageStreamer); ok {
		streamCh, err = ms.ChatCompletionStreamMessages(ctx, model, messages, temperature)
	} else {
		var sys, user string
		if len(messages) == 2 && messages[0].Role == "system" && messages[1].Role == "user" {
			sys = messages[0].Content
			user = messages[1].Content
		} else {
			for _, m := range messages {
				if m.Role == "system" && sys == "" {
					sys = m.Content
				} else if user == "" {
					user = m.Content
				} else {
					user += "\n\n" + m.Content
				}
			}
		}
		streamCh, err = client.ChatCompletionStream(ctx, model, sys, user, temperature)
	}

	if err != nil {
		if isTimeoutError(err) {
			return "", TokenUsage{}, "", fmt.Errorf("模型 [%s] 流式请求等待首字超时 (未在时限内收到响应头): %w", model, err)
		}
		return "", TokenUsage{}, "", err
	}

	var sb strings.Builder
	var finalUsage TokenUsage
	var finalFinishReason string
	hasUsage := false
	streamFailed := false
	var streamErr error

	for chunk := range streamCh {
		if chunk.Err != nil {
			streamFailed = true
			streamErr = chunk.Err
			break
		}
		if chunk.Delta != "" {
			sb.WriteString(chunk.Delta)
			if onDelta != nil {
				onDelta(chunk.Delta)
			}
		}
		if chunk.Usage != nil {
			finalUsage = *chunk.Usage
			hasUsage = true
		}
		if chunk.FinishReason != "" {
			finalFinishReason = chunk.FinishReason
		}
	}

	if streamFailed {
		if isTimeoutError(streamErr) {
			return "", finalUsage, "", fmt.Errorf("模型 [%s] 流式生成中断 (等待首字或长文本传输超时): %w", model, streamErr)
		}
		return "", finalUsage, "", streamErr
	}

	if !hasUsage {
		var promptRunes int
		for _, m := range messages {
			promptRunes += len([]rune(m.Content))
		}
		pTokens := promptRunes * 3 / 2
		cTokens := len([]rune(sb.String())) * 3 / 2
		finalUsage = TokenUsage{
			PromptTokens:     pTokens,
			CompletionTokens: cTokens,
			TotalTokens:      pTokens + cTokens,
		}
	}

	return sb.String(), finalUsage, finalFinishReason, nil
}

// SafeChatCompletionWithStreamFallback executes an LLM request using streaming first to eliminate
// 'awaiting headers' timeouts on gateways. If streaming succeeds, tokens are aggregated in memory;
// transient transport errors (EOF, connection resets, timeouts, HTTP 5xx/429) get bounded
// backoff-gated retries and still bubble up fail-closed once the budget is spent; non-retryable
// errors bubble up unchanged (no retry, no sync fallback).
// If streaming completes with empty content, it falls back to ChatCompletionWithUsage.
// Output-limit finishes trigger bounded continuation via mergeExactContinuation.
func SafeChatCompletionWithStreamFallback(
	ctx context.Context,
	client LLMClient,
	model string,
	systemPrompt, userPrompt string,
	temperature float64,
	onDelta func(string),
) (string, TokenUsage, error) {
	if client == nil {
		return "", TokenUsage{}, errors.New("nil llm client")
	}

	initialMessages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	content, usage, finishReason, err := streamOnceWithRetry(ctx, client, model, initialMessages, temperature, onDelta)
	if err == nil && content != "" {
		if !isOutputLimitFinishReason(finishReason) {
			return content, usage, nil
		}

		accumulated := content
		totalUsage := usage

		for continuationCount := 1; continuationCount <= maxOutputLimitContinuations; continuationCount++ {
			contMessages := []chatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
				{Role: "assistant", Content: accumulated},
				{Role: "user", Content: continuationInstruction()},
			}

			newText, roundUsage, roundFinishReason, roundErr := streamOnce(ctx, client, model, contMessages, temperature, onDelta)
			if roundErr != nil {
				return "", totalUsage, roundErr
			}

			addTokenUsage(&totalUsage, roundUsage)
			accumulated = mergeExactContinuation(accumulated, newText)

			if !isOutputLimitFinishReason(roundFinishReason) {
				return accumulated, totalUsage, nil
			}
		}

		return "", totalUsage, fmt.Errorf("模型 [%s] 续写 %d 次后仍被截断", model, maxOutputLimitContinuations)
	}

	if err != nil {
		return "", TokenUsage{}, err
	}

	// Fallback to sync ChatCompletionWithUsage
	return client.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temperature)
}

type roleContextKey struct{}

// ContextWithRole binds an LLMRole to the context for multi-provider dispatch.
func ContextWithRole(ctx context.Context, role LLMRole) context.Context {
	return context.WithValue(ctx, roleContextKey{}, role)
}

// RoleFromContext retrieves the LLMRole bound to the context.
func RoleFromContext(ctx context.Context) (LLMRole, bool) {
	v, ok := ctx.Value(roleContextKey{}).(LLMRole)
	return v, ok
}

// LLMRouter orchestrates multi-provider role dispatch (Writer, Reasoner, Reviewer)
// ensuring independent models can review drafts without self-judging bias.
type LLMRouter struct {
	mu        sync.RWMutex
	defaultCl *HTTPLLMClient
	clients   map[LLMRole]*HTTPLLMClient
	models    map[LLMRole]string
}

func NewLLMRouter(defaultCl *HTTPLLMClient) *LLMRouter {
	return &LLMRouter{
		defaultCl: defaultCl,
		clients:   make(map[LLMRole]*HTTPLLMClient),
		models:    make(map[LLMRole]string),
	}
}

func (r *LLMRouter) ConfigureRole(role LLMRole, client *HTTPLLMClient, model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[role] = client
	r.models[role] = model
}

func (r *LLMRouter) UpdateFromConfig(cfg *config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultCl.UpdateCredentials(cfg.APIBase, cfg.APIKey)

	resolveProvider := func(p *config.ProviderConfig, defaultModel string) (*HTTPLLMClient, string) {
		targetBase := strings.TrimSpace(cfg.APIBase)
		targetKey := strings.TrimSpace(cfg.APIKey)
		targetModel := strings.TrimSpace(defaultModel)

		if p != nil {
			if strings.TrimSpace(p.APIBase) != "" {
				targetBase = strings.TrimSpace(p.APIBase)
			}
			if strings.TrimSpace(p.APIKey) != "" {
				targetKey = strings.TrimSpace(p.APIKey)
			}
			if strings.TrimSpace(p.Model) != "" {
				targetModel = strings.TrimSpace(p.Model)
			}
		}

		if targetBase == strings.TrimSpace(cfg.APIBase) && targetKey == strings.TrimSpace(cfg.APIKey) {
			return r.defaultCl, targetModel
		}
		return NewHTTPLLMClient(targetBase, targetKey), targetModel
	}

	// Reasoner
	clReasoner, modelReasoner := resolveProvider(cfg.ReasonerProvider, cfg.ReasoningModel)
	r.clients[RoleReasoner] = clReasoner
	r.models[RoleReasoner] = modelReasoner

	// Writer
	clWriter, modelWriter := resolveProvider(cfg.WriterProvider, cfg.WriterModel)
	r.clients[RoleWriter] = clWriter
	r.models[RoleWriter] = modelWriter

	// Reviewer
	defaultRevModel := cfg.ReviewerModel
	if defaultRevModel == "" {
		defaultRevModel = cfg.ReasoningModel
	}
	clReviewer, modelReviewer := resolveProvider(cfg.ReviewerProvider, defaultRevModel)
	r.clients[RoleReviewer] = clReviewer
	r.models[RoleReviewer] = modelReviewer
}

func (r *LLMRouter) ClientForRole(role LLMRole) (LLMClient, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cl, ok := r.clients[role]
	if !ok || cl == nil {
		cl = r.defaultCl
	}
	model := r.models[role]
	return cl, model
}

func (r *LLMRouter) resolveClientAndModel(ctx context.Context, requestedModel string) (LLMClient, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. If explicit role is attached to the context, route to that role's client and configured model
	if role, ok := RoleFromContext(ctx); ok {
		if cl, ok := r.clients[role]; ok && cl != nil {
			targetModel := r.models[role]
			if targetModel == "" {
				targetModel = requestedModel
			}
			return cl, targetModel
		}
	}

	// 2. If requestedModel matches a specific role's configured model, route accordingly
	if requestedModel != "" {
		for role, configuredModel := range r.models {
			if configuredModel != "" && configuredModel == requestedModel {
				if cl, ok := r.clients[role]; ok && cl != nil {
					return cl, requestedModel
				}
			}
		}
	}

	return r.defaultCl, requestedModel
}

func (r *LLMRouter) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error) {
	cl, targetModel := r.resolveClientAndModel(ctx, model)
	return cl.ChatCompletion(ctx, targetModel, systemPrompt, userPrompt, temperature)
}

func (r *LLMRouter) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	cl, targetModel := r.resolveClientAndModel(ctx, model)
	return cl.ChatCompletionWithUsage(ctx, targetModel, systemPrompt, userPrompt, temperature)
}

func (r *LLMRouter) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	cl, targetModel := r.resolveClientAndModel(ctx, model)
	return cl.ChatCompletionStream(ctx, targetModel, systemPrompt, userPrompt, temperature)
}

func (r *LLMRouter) ChatCompletionStreamMessages(ctx context.Context, model string, messages []chatMessage, temperature float64) (<-chan StreamChunk, error) {
	cl, targetModel := r.resolveClientAndModel(ctx, model)
	if ms, ok := cl.(interface {
		ChatCompletionStreamMessages(context.Context, string, []chatMessage, float64) (<-chan StreamChunk, error)
	}); ok {
		return ms.ChatCompletionStreamMessages(ctx, targetModel, messages, temperature)
	}
	var sys, user string
	for _, m := range messages {
		if m.Role == "system" && sys == "" {
			sys = m.Content
		} else if user == "" {
			user = m.Content
		} else {
			user += "\n\n" + m.Content
		}
	}
	return cl.ChatCompletionStream(ctx, targetModel, sys, user, temperature)
}

func (r *LLMRouter) ChatCompletionForRole(ctx context.Context, role LLMRole, model string, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	r.mu.RLock()
	cl, ok := r.clients[role]
	targetModel := model
	if targetModel == "" {
		targetModel = r.models[role]
	}
	if !ok || cl == nil {
		cl = r.defaultCl
	}
	r.mu.RUnlock()

	return cl.ChatCompletionWithUsage(ctx, targetModel, systemPrompt, userPrompt, temperature)
}

func (c *HTTPLLMClient) generateMockResponse(model, systemPrompt, userPrompt string) (string, TokenUsage, error) {
	pTokens := len([]rune(systemPrompt+userPrompt))/3 + 120

	// 1. Beats derivation
	if strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
		cTokens := 480
		resp := `{
  "beats": [
    {
      "phase": "蓄力压迫",
      "tension": 5,
      "action": "风雪骤急，茶肆雅间内暗流涌动，酒杯见底之际异样浮现。",
      "expectation_broken": "本以为是寻常落脚，暗桩已无声布下绝杀之局。",
      "reader_emotion": "紧张压抑",
      "info_gap": "读者与主角察觉杀意，暗桩尚以为主角毫无戒心"
    },
    {
      "phase": "试探下套",
      "tension": 7,
      "action": "楚掌柜斟满烈酒逼近，言语间三度试探功法底细，杀机毕露。",
      "expectation_broken": "主角并未惊慌，顺水推舟将计就计引其入套。",
      "reader_emotion": "好奇期待",
      "info_gap": "主角已知酒中有蛊，楚掌柜以为得手"
    },
    {
      "phase": "绝地反转",
      "tension": 9,
      "action": "残印神芒骤亮，主角反手震碎酒盏，雷霆断其右臂，破窗突围。",
      "expectation_broken": "楚掌柜本以为主角是凡胎，却遭神纹反噬重创。",
      "reader_emotion": "大呼解气",
      "info_gap": "暗中观望的同门探子被主角真正实力彻底震慑"
    },
    {
      "phase": "章末留钩",
      "tension": 8,
      "action": "主角夺下染血密令疾驰而去，雪地中现出一排深不可测的青铜脚印。",
      "expectation_broken": "刚斩暗桩，更庞大的幕后巨擘已悄然抵近门外。",
      "reader_emotion": "心悬期待",
      "info_gap": "读者已知魔宗追兵已至，主角尚未察觉青铜脚印所指何人",
      "hook_type": "CLIFFHANGER"
    }
  ],
  "state_mutation": {
    "inventory_delta": "+百年寒铁令x1, -引路符x1",
    "power_delta": "凡骨神纹觉醒一成",
    "character_mutations": [
      {
        "name": "楚掌柜",
        "status_delta": "右臂被斩断，狼狈遁入暗道残喘",
        "relation_delta": "对主角由轻蔑转为刻骨仇恨"
      },
      {
        "name": "柳依依",
        "status_delta": "暗中目睹残印神威，心神剧震",
        "relation_delta": "由试探结盟转为彻底敬畏"
      }
    ]
  }
}`
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, nil
	}

	// 2. Reviewer audit (QualityGate / Orchestrator Review, must not be rewrite draft)
	combinedReview := systemPrompt + " " + userPrompt
	if !strings.Contains(systemPrompt, "精修") && !strings.Contains(systemPrompt, "定向精修") && (strings.Contains(combinedReview, "QualityGate") || strings.Contains(combinedReview, "审编") || strings.Contains(combinedReview, "编审") || strings.Contains(combinedReview, "综合审校") || strings.Contains(combinedReview, "审阅判决")) {
		cTokens := 160
		resp := `{
  "verdict": "ACCEPTED",
  "score": 92,
  "issues": [],
  "suggestions": "节奏明快，多角色声口与微动作对峙层次分明，突发度与张力达标。",
  "resolved_hook_ids": []
}`
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, nil
	}

	// 3. Scene prose draft
	if strings.Contains(systemPrompt, "文学渲染") || strings.Contains(systemPrompt, "小说正文") || strings.Contains(systemPrompt, "写作铁律") {
		prose := `外头风雪砸门，把聚仙楼的黄布帘子扯得啪啪响。

屋里就柜台上一盏油灯，火头晃得厉害，影子在泥墙上直爬。

案子后头坐着楚掌柜，身上那件旧袍子早磨脱了色。炭盆里煨着锡壶，劣酒咕嘟冒泡。那双手瘦得光剩一把骨头，青筋暴着直抠案板。

他斜过眼，眼珠子泛黄，盯死来人：“北边来的？”

嗓子又干又尖，直往耳朵眼钻。

顾渊没接茬。他摘下斗笠往桌上一掼，随手抹掉一层厚灰，拉开木凳坐下。

贴肉的粗布袄子底下，那块凡骨铁印滚烫，烙得皮肉发紧。

“小店荒僻，就这烧刀子驱寒。”楚掌柜提壶倒酒，土碗里溅起沫子。酒气刺鼻，底下压着一丝甜腻腥气。

七步散。

顾渊搭在膝头上的手扣紧了桌面。

“在这荒口守了多少年？”顾渊开口，嗓音沙哑，长途跑马磨出来的干瘪。

楚掌柜提壶的手顿了顿，咧嘴笑，笑意没沾上脸：“三十来年。打听这干啥？”

“三十年，官道地底下的死人够铺几层了？”

风撞烂了半扇木门。

冷风卷进来，油灯灭了。

楚掌柜袖口一扬，五指成抓，指风直抠顾渊喉骨，嘴里狞笑：“凡胎贱种，也配问通天阁的事！”

顾渊没退。

他腰身一沉，踏裂泥地，合身迎头撞进楚掌柜怀里。

左掌一翻，凡骨残印迎着掌心迎头压上。

铁印腾起白光，魔毒撞上来，全被神纹吞了进去。

楚掌柜喉咙里挤出一声闷哼，脸色煞白：“你不是凡——”

一声骨脆响。

顾渊反手铰住他的手腕，真力一拧，腕骨断成两截。

楚掌柜惨叫半声，整条右膀子齐肘撕断，血喷在窗纸上。他脚底一蹬，地板翻开暗道，连滚带爬跌进窟窿里，只留下一条断臂在泥水里抽搐。

风卷着雪沫子灌进窗缝，血腥气散开了。

顾渊弯腰，从断手上掰下一枚青铜铁令。上面刻着小篆：通天阁。

房梁上有轻响。

一道黄衫人影轻飘飘落下来。柳依依捏着三道追魂符，眼睛直勾勾盯着顾渊手里的铁印，胸口起伏。

“顾……顾师兄，”柳依依嘴唇打哆嗦，“那老头是化龙境暗桩，你手里那是啥物件？”

顾渊把铁令塞进怀里，斗笠扣回头顶。

“符收起来。”

他一把推开门，踩进没膝的雪窝子里。

雪地上除了拖出的血印子，还轧着两道深沟，泛着青铜黑光，不知是啥车撵留下的。`
		cTokens := len([]rune(prose))
		return prose, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, nil
	}

	// 4. Genesis Bootstrap Framework
	combined := systemPrompt + " " + userPrompt
	if strings.Contains(combined, "创世总纲") || strings.Contains(combined, "Genesis") || strings.Contains(combined, "Project Framework") || strings.Contains(combined, "顶层架构总纲") {
		cTokens := 600
		resp := `{
  "theme_premise": "凡人无灵根，天道被不可名状的神明寄生；修士飞升实为祭品；主角掌凡骨铁印隐忍弑神",
  "protagonist": {
    "name": "顾渊",
    "identity": "边荒无灵根凡人",
    "initial_realm": "凡胎境",
    "goal": "查明宗门覆灭真相，隐忍弑神",
    "voice_tone": "内敛隐忍，遇死地杀伐果决",
    "inventory": ["凡骨铁印", "粗布短褐", "引路符"]
  },
  "world_axioms": [
    "不可直视神明法相，凡人目睹必发狂化肉泥",
    "天地灵气皆附带神明蛊毒，修真境界越高异化越深",
    "凡骨无灵根者不受神明窥伺，凡铁残印唯凡骨可驭"
  ],
  "power_ladder": [
    {
      "realm": "凡胎境",
      "description": "气力胜过常人，肉身无灵气波动",
      "bottleneck": "熬炼骨髓，引凡火锻体",
      "drawback": "无灵力护体，寿元与凡夫无异"
    },
    {
      "realm": "锻骨境",
      "description": "骨骼化为青铜之质，徒手碎刃",
      "bottleneck": "承受凡骨铁印反噬之痛",
      "drawback": "骨髓剧痛，阴雨天如万蚁噬髓"
    }
  ],
  "factions": [
    {
      "name": "通天阁",
      "alignment": "魔道暗桩",
      "doctrine": "垄断边陲仙凡坊市暗庄，搜罗凡人精血",
      "threat_level": "极高"
    }
  ],
  "key_characters": [
    {
      "name": "楚掌柜",
      "role": "宿敌暗哨",
      "realm": "化龙境魔修",
      "goal": "截杀凡人异数向魔尊邀功",
      "fate_arc": "第一章被断右臂遁走，沦为通天阁追杀棋子"
    },
    {
      "name": "柳依依",
      "role": "关键搭档",
      "realm": "筑基境圆满",
      "goal": "借各方势力暗查灭门真相，求生自保",
      "fate_arc": "目睹凡骨神印，由试探结盟转为敬畏相随"
    }
  ],
  "volume_arcs": [
    {
      "volume_index": 1,
      "title": "风雪聚仙楼",
      "theme": "荒野客栈惊变与凡骨初醒",
      "core_goal": "破译通天阁潜伏密网，斩断暗桩爪牙",
      "climax": "聚仙楼血战，主角以凡人之躯斩魔门化龙暗哨",
      "estimated_chapters": 30,
      "key_payoffs": ["百年寒铁令真相"]
    }
  ],
  "seed_hooks": [
    {
      "title": "聚仙楼地底的青铜车辙",
      "details": "聚仙楼地道尽头连接着不可名状的青铜神车，车轮上沾染着三千年前的神血",
      "created_chapter": 1,
      "target_chapter": 15,
      "status": "OPEN"
    }
  ]
}`
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, nil
	}

	return "推演完成，因果闭环。", TokenUsage{PromptTokens: pTokens, CompletionTokens: 20, TotalTokens: pTokens + 20}, nil
}
