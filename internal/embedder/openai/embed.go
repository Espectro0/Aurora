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
)

type baseURLProvider func(ctx context.Context) (string, error)

type Embedder struct {
	model        string
	baseURL      string
	baseProvider baseURLProvider
	apiKey       string
	http         *http.Client
}

func New(model string, timeout time.Duration) *Embedder {
	return &Embedder{
		model: model,
		http:  &http.Client{Timeout: timeout},
	}
}

func (e *Embedder) SetBaseURL(baseURL string) {
	e.baseURL = baseURL
}

func (e *Embedder) SetBaseURLProvider(fn func(ctx context.Context) (string, error)) {
	e.baseProvider = fn
}

func (e *Embedder) SetAPIKey(key string) {
	e.apiKey = key
}

func (e *Embedder) resolveBaseURL(ctx context.Context) (string, error) {
	if e.baseProvider != nil {
		return e.baseProvider(ctx)
	}
	if e.baseURL != "" {
		return e.baseURL, nil
	}
	return "", fmt.Errorf("openai-embed: no base URL configured")
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	baseURL, err := e.resolveBaseURL(ctx)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"input": []string{text},
		"model": e.model,
	}

	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/embeddings", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai-embed: %w", err)
	}

	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai-embed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("openai-embed: decode: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("openai-embed: %s", result.Error.Message)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("openai-embed: no data in response")
	}

	return float64To32(result.Data[0].Embedding), nil
}

func float64To32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}
