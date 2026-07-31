package nvidia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Embedder struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func New(apiKey, model, baseURL string, timeout time.Duration) *Embedder {
	return &Embedder{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout * time.Second},
	}
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body := map[string]any{
		"input":           []string{text},
		"model":           e.model,
		"input_type":      "query",
		"encoding_format": "float",
		"truncate":        "NONE",
	}

	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nvidia-embed: %w", err)
	}

	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("nvidia-embed: decode: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("nvidia-embed: no data in response")
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
