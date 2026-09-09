package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type restClient struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

func newRESTClient(baseURL, apiKey string) *restClient {
	return &restClient{
		http:    &http.Client{},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

func (c *restClient) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("qdrant: marshal: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, fmt.Errorf("qdrant: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("qdrant: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return resp.StatusCode, fmt.Errorf("qdrant: status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if out != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return resp.StatusCode, fmt.Errorf("qdrant: decode: %w", err)
		}
	}

	return resp.StatusCode, nil
}

type vectorParams struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type collectionInfo struct {
	Result struct {
		Config struct {
			Params struct {
				Vectors vectorParams `json:"vectors"`
			} `json:"params"`
		} `json:"config"`
	} `json:"result"`
}

func (c *restClient) getCollection(ctx context.Context, name string) (*collectionInfo, error) {
	var info collectionInfo
	status, err := c.do(ctx, http.MethodGet, "/collections/"+name, nil, &info)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	return &info, nil
}

func (c *restClient) createCollection(ctx context.Context, name string, size int) error {
	body := map[string]any{
		"vectors": vectorParams{Size: size, Distance: "Cosine"},
	}
	_, err := c.do(ctx, http.MethodPut, "/collections/"+name, body, nil)
	return err
}

type point struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
	Score   float64        `json:"score,omitempty"`
}

func (c *restClient) upsertPoint(ctx context.Context, collection string, p point) error {
	body := map[string]any{
		"points": []point{p},
	}
	_, err := c.do(ctx, http.MethodPut, "/collections/"+collection+"/points", body, nil)
	return err
}

func (c *restClient) retrievePoints(ctx context.Context, collection string, ids []string) ([]point, error) {
	body := map[string]any{
		"ids":          ids,
		"with_payload": true,
		"with_vector":  false,
	}

	var result struct {
		Result []point `json:"result"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points", body, &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

func (c *restClient) searchPoints(ctx context.Context, collection string, vector []float32, limit int) ([]point, error) {
	body := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}

	var result struct {
		Result []point `json:"result"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/search", body, &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

func (c *restClient) scrollPoints(ctx context.Context, collection string, filter map[string]any) ([]point, error) {
	var all []point
	var offset any

	for {
		body := map[string]any{
			"filter":       filter,
			"limit":        256,
			"with_vector":  true,
			"with_payload": true,
		}
		if offset != nil {
			body["offset"] = offset
		}

		var result struct {
			Result struct {
				Points         []point `json:"points"`
				NextPageOffset any     `json:"next_page_offset"`
			} `json:"result"`
		}
		if _, err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/scroll", body, &result); err != nil {
			return nil, err
		}

		all = append(all, result.Result.Points...)

		if result.Result.NextPageOffset == nil || len(result.Result.Points) == 0 {
			break
		}
		offset = result.Result.NextPageOffset
	}

	return all, nil
}

func (c *restClient) countPoints(ctx context.Context, collection string) (int, error) {
	body := map[string]any{"exact": true}

	var result struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/count", body, &result); err != nil {
		return 0, err
	}
	return result.Result.Count, nil
}
