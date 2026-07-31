package nvidia

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

type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func New(apiKey, model string, timeout time.Duration) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://integrate.api.nvidia.com/v1",
		http:    &http.Client{Timeout: timeout * time.Second},
	}
}

func (c *Client) Chat(ctx context.Context, messages []conversation.Message) (string, error) {
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var nvMessages []chatMessage
	for _, m := range messages {
		nvMessages = append(nvMessages, chatMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body := map[string]any{
		"model":       c.model,
		"messages":    nvMessages,
		"temperature": 1,
		"top_p":       0.95,
		"max_tokens":  4096,
		"seed":        42,
		"stream":      false,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("nvidia: %w", err)
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
		return "", fmt.Errorf("nvidia: decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("nvidia: no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}
