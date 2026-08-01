package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("openai: decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}
