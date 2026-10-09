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
			Timeout: 300 * time.Second, // 300s timeout to support DeepSeek R1 and long-token reasoning
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
	if apiKey == "mock" || apiKey == "offline" || strings.HasPrefix(model, "mock") {
		return c.generateMockResponse(model, systemPrompt, userPrompt)
	}
	if apiKey == "" {
		return "", zeroUsage, errors.New("missing API key: please configure your API key in settings (or use 'mock' for offline testing)")
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

	var bodyBytes []byte
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return "", zeroUsage, fmt.Errorf("create request failed: %w", err)
		}

		c.applyHeaders(req, apiKey, endpoint)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", zeroUsage, ctx.Err()
			}
			if attempt == maxRetries-1 {
				return "", zeroUsage, fmt.Errorf("do HTTP request failed after %d attempts: %w", maxRetries, err)
			}
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
			continue
		}

		bodyBytes, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			if attempt == maxRetries-1 {
				return "", zeroUsage, fmt.Errorf("read response body failed: %w", err)
			}
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}

		// Retry on 429 Rate Limit or 5xx Server Error
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < maxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * 750 * time.Millisecond)
			continue
		}

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

	if apiKey == "mock" || apiKey == "offline" || strings.HasPrefix(model, "mock") {
		out := make(chan StreamChunk, 4)
		go func() {
			defer close(out)
			resp, usage, _ := c.generateMockResponse(model, systemPrompt, userPrompt)
			out <- StreamChunk{Delta: resp, Usage: &usage}
		}()
		return out, nil
	}
	if apiKey == "" {
		return nil, errors.New("missing API key: please configure your API key in settings (or use 'mock' for offline testing)")
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

				if deltaText != "" || usage != nil {
					if !sendChunk(StreamChunk{
						Delta: deltaText,
						Usage: usage,
					}) {
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			sendChunk(StreamChunk{Err: err})
		}
	}()

	return out, nil
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

	// 1. If explicit role is attached to the context, route to that role's client
	if role, ok := RoleFromContext(ctx); ok {
		if cl, ok := r.clients[role]; ok && cl != nil {
			targetModel := requestedModel
			if targetModel == "" {
				targetModel = r.models[role]
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
