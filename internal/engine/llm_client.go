package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Delta string      `json:"delta"`
	Usage *TokenUsage `json:"usage,omitempty"`
	Done  bool        `json:"done"`
	Err   error       `json:"err,omitempty"`
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
	mu         sync.RWMutex
	baseURL    string
	apiKey     string
	sessionID  string
	httpClient *http.Client
}

func NewHTTPLLMClient(baseURL, apiKey string) *HTTPLLMClient {
	c := &HTTPLLMClient{
		sessionID: fmt.Sprintf("novel-studio-%x", time.Now().UnixNano()),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
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
	} `json:"choices"`
	OutputText string          `json:"output_text"`
	Output     json.RawMessage `json:"output"`
	Usage      *struct {
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

func (c *HTTPLLMClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error) {
	content, _, err := c.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temperature)
	return content, err
}

func (c *HTTPLLMClient) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	c.mu.RLock()
	apiKey := c.apiKey
	baseURL := c.baseURL
	c.mu.RUnlock()

	var zeroUsage TokenUsage
	if apiKey == "" {
		return "", zeroUsage, errors.New("missing API key: please configure your API key in settings")
	}

	endpoint := c.endpointFor(baseURL)
	var reqBody chatRequest
	if strings.HasSuffix(endpoint, "/responses") {
		reqBody = chatRequest{
			Model:        model,
			Instructions: systemPrompt,
			Input:        userPrompt,
			Temperature:  temperature,
		}
	} else {
		reqBody = chatRequest{
			Model: model,
			Messages: []chatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			Temperature: temperature,
		}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", zeroUsage, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", zeroUsage, fmt.Errorf("create request failed: %w", err)
	}

	c.applyHeaders(req, apiKey, endpoint)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", zeroUsage, fmt.Errorf("do HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", zeroUsage, fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", zeroUsage, fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", zeroUsage, fmt.Errorf("unmarshal LLM response failed: %w", err)
	}

	if chatResp.Error != nil {
		return "", zeroUsage, fmt.Errorf("LLM API error: %s", chatResp.Error.Message)
	}

	var content string
	if len(chatResp.Choices) > 0 && chatResp.Choices[0].Message.Content != "" {
		content = chatResp.Choices[0].Message.Content
	} else if chatResp.OutputText != "" {
		content = chatResp.OutputText
	} else if extracted := extractOutputText(chatResp.Output); extracted != "" {
		content = extracted
	} else {
		return "", zeroUsage, errors.New("LLM returned no choices or output content")
	}

	usage := extractTokenUsage(chatResp.Usage)
	return content, usage, nil
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

func (c *HTTPLLMClient) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	c.mu.RLock()
	apiKey := c.apiKey
	baseURL := c.baseURL
	c.mu.RUnlock()

	if apiKey == "" {
		return nil, errors.New("missing API key: please configure your API key in settings")
	}

	endpoint := c.endpointFor(baseURL)
	var reqBody chatRequest
	if strings.HasSuffix(endpoint, "/responses") {
		reqBody = chatRequest{
			Model:        model,
			Instructions: systemPrompt,
			Input:        userPrompt,
			Temperature:  temperature,
			Stream:       true,
		}
	} else {
		reqBody = chatRequest{
			Model: model,
			Messages: []chatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
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

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				out <- StreamChunk{Err: ctx.Err()}
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
					out <- StreamChunk{Done: true}
					return
				}

				var rawMap map[string]interface{}
				if err := json.Unmarshal([]byte(data), &rawMap); err != nil {
					continue
				}

				deltaText := extractStreamDelta(rawMap)
				usage := extractStreamUsage(rawMap)

				if deltaText != "" || usage != nil {
					out <- StreamChunk{
						Delta: deltaText,
						Usage: usage,
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			out <- StreamChunk{Err: err}
		}
	}()

	return out, nil
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

	// Reasoner
	if cfg.ReasonerProvider != nil && cfg.ReasonerProvider.APIBase != "" {
		r.clients[RoleReasoner] = NewHTTPLLMClient(cfg.ReasonerProvider.APIBase, cfg.ReasonerProvider.APIKey)
		r.models[RoleReasoner] = cfg.ReasonerProvider.Model
	} else {
		r.clients[RoleReasoner] = r.defaultCl
		r.models[RoleReasoner] = cfg.ReasoningModel
	}

	// Writer
	if cfg.WriterProvider != nil && cfg.WriterProvider.APIBase != "" {
		r.clients[RoleWriter] = NewHTTPLLMClient(cfg.WriterProvider.APIBase, cfg.WriterProvider.APIKey)
		r.models[RoleWriter] = cfg.WriterProvider.Model
	} else {
		r.clients[RoleWriter] = r.defaultCl
		r.models[RoleWriter] = cfg.WriterModel
	}

	// Reviewer (P0: Independent cross-provider Reviewer)
	if cfg.ReviewerProvider != nil && cfg.ReviewerProvider.APIBase != "" {
		r.clients[RoleReviewer] = NewHTTPLLMClient(cfg.ReviewerProvider.APIBase, cfg.ReviewerProvider.APIKey)
		r.models[RoleReviewer] = cfg.ReviewerProvider.Model
	} else {
		r.clients[RoleReviewer] = r.defaultCl
		model := cfg.ReviewerModel
		if model == "" {
			model = cfg.ReasoningModel
		}
		r.models[RoleReviewer] = model
	}
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

func (r *LLMRouter) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error) {
	return r.defaultCl.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
}

func (r *LLMRouter) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	return r.defaultCl.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temperature)
}

func (r *LLMRouter) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	return r.defaultCl.ChatCompletionStream(ctx, model, systemPrompt, userPrompt, temperature)
}
