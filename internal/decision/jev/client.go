package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/decision"
)

var _ decision.Provider = (*Client)(nil)

type Client struct {
	model   string
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(model string, timeout time.Duration) *Client {
	if model == "" {
		model = "jev-latest"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{model: model, http: &http.Client{Timeout: timeout}}
}

func (c *Client) SetBaseURL(baseURL string) { c.baseURL = baseURL }
func (c *Client) SetAPIKey(key string)      { c.apiKey = key }

func (c *Client) resolveBaseURL() string {
	base := c.baseURL
	if base == "" {
		base = "https://openrouter.ai/api"
	}
	base = strings.TrimRight(base, "/")
	return strings.TrimSuffix(base, "/v1")
}

type apiQuestion struct {
	kind         string
	instructions string
	criteria     map[string]string
	levels       []string
}

func (q apiQuestion) MarshalJSON() ([]byte, error) {
	switch q.kind {
	case string(decision.TypeChoice):
		return json.Marshal(struct {
			Type         string            `json:"type"`
			Instructions string            `json:"instructions"`
			Criteria     map[string]string `json:"criteria"`
		}{q.kind, q.instructions, q.criteria})

	case string(decision.TypeScore):
		return json.Marshal(struct {
			Type         string   `json:"type"`
			Instructions string   `json:"instructions"`
			Criteria     []string `json:"criteria"`
		}{q.kind, q.instructions, q.levels})

	default: // noul
		if len(q.criteria) == 0 {
			return json.Marshal(struct {
				Type         string `json:"type"`
				Instructions string `json:"instructions"`
			}{q.kind, q.instructions})
		}
		return json.Marshal(struct {
			Type         string            `json:"type"`
			Instructions string            `json:"instructions"`
			Criteria     map[string]string `json:"criteria"`
		}{q.kind, q.instructions, q.criteria})
	}
}

func toAPIQuestion(q decision.Question) apiQuestion {
	return apiQuestion{
		kind:         string(q.Type),
		instructions: q.Instructions,
		criteria:     q.Criteria,
		levels:       q.Levels,
	}
}

type apiAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

func (a apiAnswer) toDecision() decision.Answer {
	return decision.Answer{
		Type:          decision.QuestionType(a.Type),
		Choice:        a.Choice,
		Score:         a.Score,
		Noul:          a.Noul,
		Confidence:    a.Confidence,
		Probabilities: a.Probabilities,
		Legend:        a.Legend,
	}
}

func (c *Client) Judge(ctx context.Context, state string, questions map[string]decision.Question) (map[string]decision.Answer, error) {
	if len(questions) == 0 {
		return nil, nil
	}

	apiQuestions := make(map[string]apiQuestion, len(questions))
	for id, q := range questions {
		apiQuestions[id] = toAPIQuestion(q)
	}

	body := map[string]any{
		"state":     state,
		"model":     c.model,
		"questions": apiQuestions,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("jev: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.resolveBaseURL()+"/alpha/decisions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("jev: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jev: status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var result struct {
		Model   string               `json:"model"`
		Answers map[string]apiAnswer `json:"answers"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("jev: decode: %w", err)
	}

	answers := make(map[string]decision.Answer, len(result.Answers))
	for id, a := range result.Answers {
		answers[id] = a.toDecision()
	}

	return answers, nil
}
