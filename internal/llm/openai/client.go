package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/llm"
)

var _ llm.Provider = (*Client)(nil)

type baseURLProvider func(ctx context.Context) (string, error)

type Client struct {
	model        string
	baseURL      string
	baseProvider baseURLProvider
	apiKey       string
	maxTokens    int
	http         *http.Client
}

func New(model string, timeout time.Duration) *Client {
	return &Client{
		model:     model,
		maxTokens: 512,
		http:      &http.Client{Timeout: timeout},
	}
}

func (c *Client) SetMaxTokens(n int) {
	c.maxTokens = n
}

func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

func (c *Client) SetBaseURLProvider(fn func(ctx context.Context) (string, error)) {
	c.baseProvider = fn
}

func (c *Client) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *Client) resolveBaseURL(ctx context.Context) (string, error) {
	if c.baseProvider != nil {
		return c.baseProvider(ctx)
	}
	if c.baseURL != "" {
		return c.baseURL, nil
	}
	return "", fmt.Errorf("openai: no base URL configured")
}

func (c *Client) Chat(ctx context.Context, messages []conversation.Message) (string, error) {
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	baseURL, err := c.resolveBaseURL(ctx)
	if err != nil {
		return "", err
	}

	reqMessages := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, chatMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body := map[string]any{
		"model":       c.model,
		"messages":    reqMessages,
		"temperature": 1,
		"top_p":       0.8,
		"max_tokens":  c.maxTokens,
		"stream":      false,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}

	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("openai: decode: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("openai: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

func (c *Client) ChatWithTools(ctx context.Context, messages []conversation.Message, tools []llm.ToolDefinition) (llm.ChatResult, error) {
	type functionCall struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}

	type toolCallOut struct {
		ID       string       `json:"id"`
		Type     string       `json:"type"`
		Function functionCall `json:"function"`
	}

	type chatMessage struct {
		Role       string        `json:"role"`
		Content    string        `json:"content,omitempty"`
		ToolCallID string        `json:"tool_call_id,omitempty"`
		ToolCalls  []toolCallOut `json:"tool_calls,omitempty"`
	}

	type functionDef struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Parameters  map[string]any `json:"parameters,omitempty"`
	}

	type toolDef struct {
		Type     string      `json:"type"`
		Function functionDef `json:"function"`
	}

	baseURL, err := c.resolveBaseURL(ctx)
	if err != nil {
		return llm.ChatResult{}, err
	}

	reqMessages := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		cm := chatMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, toolCallOut{
				ID:   tc.ID,
				Type: "function",
				Function: functionCall{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		reqMessages = append(reqMessages, cm)
	}

	reqTools := make([]toolDef, 0, len(tools))
	for _, t := range tools {
		reqTools = append(reqTools, toolDef{
			Type: "function",
			Function: functionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	body := map[string]any{
		"model":       c.model,
		"messages":    reqMessages,
		"temperature": 1,
		"top_p":       0.8,
		"max_tokens":  c.maxTokens,
		"stream":      false,
	}
	if len(reqTools) > 0 {
		body["tools"] = reqTools
	}

	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return llm.ChatResult{}, fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content   string        `json:"content"`
				ToolCalls []toolCallOut `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return llm.ChatResult{}, fmt.Errorf("openai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return llm.ChatResult{}, fmt.Errorf("openai: decode: %w", err)
	}
	if result.Error != nil {
		return llm.ChatResult{}, fmt.Errorf("openai: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return llm.ChatResult{}, fmt.Errorf("openai: no choices in response")
	}

	msg := result.Choices[0].Message

	chatResult := llm.ChatResult{Content: msg.Content}
	for _, tc := range msg.ToolCalls {
		chatResult.ToolCalls = append(chatResult.ToolCalls, conversation.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return chatResult, nil
}
