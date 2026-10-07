package engine

import (
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
)

// LLMClient represents a standard chat completion provider.
type LLMClient interface {
	ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error)
}

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

	cleanURL := strings.TrimRight(baseURL, "/")
	if cleanURL != "" && !strings.HasSuffix(cleanURL, "/v1") && !strings.Contains(cleanURL, "/v1/") {
		cleanURL += "/v1"
	}
	c.baseURL = cleanURL
	c.apiKey = apiKey
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *HTTPLLMClient) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temperature float64) (string, error) {
	c.mu.RLock()
	apiKey := c.apiKey
	baseURL := c.baseURL
	c.mu.RUnlock()

	if apiKey == "" {
		return "", errors.New("missing API key: please configure your API key in settings")
	}

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	endpoint := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal LLM response failed: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("LLM API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", errors.New("LLM returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}
