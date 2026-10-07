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
	httpClient *http.Client
}

func NewHTTPLLMClient(baseURL, apiKey string) *HTTPLLMClient {
	c := &HTTPLLMClient{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
	c.UpdateCredentials(baseURL, apiKey)
	return c
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
	OutputText string `json:"output_text"`
	Output     []struct {
		Content string `json:"content"`
		Text    string `json:"text"`
	} `json:"output"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
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
	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
	}
	if strings.HasSuffix(endpoint, "/responses") {
		reqBody.Input = reqBody.Messages
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", zeroUsage, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", zeroUsage, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
	} else if len(chatResp.Output) > 0 {
		if chatResp.Output[0].Content != "" {
			content = chatResp.Output[0].Content
		} else {
			content = chatResp.Output[0].Text
		}
	} else {
		return "", zeroUsage, errors.New("LLM returned no choices or output content")
	}

	var usage TokenUsage
	if chatResp.Usage != nil {
		usage = TokenUsage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		}
	}

	return content, usage, nil
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
	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
		Stream:      true,
	}
	if strings.HasSuffix(endpoint, "/responses") {
		reqBody.Input = reqBody.Messages
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
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

				var streamResp struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
					Delta struct {
						Content string `json:"content"`
						Text    string `json:"text"`
					} `json:"delta"`
					OutputText string `json:"output_text"`
					Usage      *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					} `json:"usage"`
				}

				if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
					continue
				}

				var deltaText string
				if len(streamResp.Choices) > 0 && streamResp.Choices[0].Delta.Content != "" {
					deltaText = streamResp.Choices[0].Delta.Content
				} else if streamResp.Delta.Content != "" {
					deltaText = streamResp.Delta.Content
				} else if streamResp.Delta.Text != "" {
					deltaText = streamResp.Delta.Text
				} else if streamResp.OutputText != "" {
					deltaText = streamResp.OutputText
				}

				var usage *TokenUsage
				if streamResp.Usage != nil {
					usage = &TokenUsage{
						PromptTokens:     streamResp.Usage.PromptTokens,
						CompletionTokens: streamResp.Usage.CompletionTokens,
						TotalTokens:      streamResp.Usage.TotalTokens,
					}
				}

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
