package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type apiClient struct {
	http    *http.Client
	baseURL string
	fileURL string
}

func newAPIClient(token string) *apiClient {
	return &apiClient{
		http:    &http.Client{Timeout: 40 * time.Second},
		baseURL: "https://api.telegram.org/bot" + token,
		fileURL: "https://api.telegram.org/file/bot" + token,
	}
}

func (c *apiClient) call(ctx context.Context, method string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("telegram: marshal: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, reader)
	if err != nil {
		return fmt.Errorf("telegram: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram: read response: %w", err)
	}

	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return fmt.Errorf("telegram: decode: %w: %s", err, strings.TrimSpace(string(bodyBytes)))
	}
	if !envelope.OK {
		return fmt.Errorf("telegram: %s: %s", method, envelope.Description)
	}

	if out != nil {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("telegram: decode result: %w", err)
		}
	}

	return nil
}

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type chat struct {
	ID int64 `json:"id"`
}

type mediaFile struct {
	FileID string `json:"file_id"`
}

type message struct {
	MessageID int64      `json:"message_id"`
	From      user       `json:"from"`
	Chat      chat       `json:"chat"`
	Text      string     `json:"text"`
	Voice     *mediaFile `json:"voice"`
	Audio     *mediaFile `json:"audio"`
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type file struct {
	FilePath string `json:"file_path"`
}

func (c *apiClient) getMe(ctx context.Context) (user, error) {
	var u user
	err := c.call(ctx, "getMe", nil, &u)
	return u, err
}

func (c *apiClient) getUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]update, error) {
	body := map[string]any{
		"offset":  offset,
		"timeout": timeoutSeconds,
	}
	var updates []update
	err := c.call(ctx, "getUpdates", body, &updates)
	return updates, err
}

func (c *apiClient) sendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, nil)
}

func (c *apiClient) sendChatAction(ctx context.Context, chatID int64, action string) error {
	return c.call(ctx, "sendChatAction", map[string]any{
		"chat_id": chatID,
		"action":  action,
	}, nil)
}

func (c *apiClient) getFile(ctx context.Context, fileID string) (file, error) {
	var f file
	err := c.call(ctx, "getFile", map[string]any{"file_id": fileID}, &f)
	return f, err
}

func (c *apiClient) downloadFile(ctx context.Context, filePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.fileURL+"/"+filePath, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram: file download status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
