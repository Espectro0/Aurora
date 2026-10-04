package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func NewClient(baseURL, apiKey, endUserAccountID string) *Client {
	return &Client{
		http:             &http.Client{Timeout: 30 * time.Second},
		baseURL:          strings.TrimRight(baseURL, "/"),
		apiKey:           apiKey,
		endUserAccountID: endUserAccountID,
	}
}

func (c *Client) do(ctx context.Context, method, endpoint string, body []byte, opName string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("apiroc: request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apiroc: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("apiroc: %s: status %d: %s", opName, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return respBody, nil
}

func (c *Client) ListCalendars(ctx context.Context) ([]Calendar, error) {
	endpoint := fmt.Sprintf("%s/calendars/%s", c.baseURL, c.endUserAccountID)

	body, err := c.do(ctx, http.MethodGet, endpoint, nil, "list calendars")
	if err != nil {
		return nil, err
	}

	var out listCalendarsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("apiroc: decode: %w", err)
	}

	return out.Data, nil
}

func (c *Client) ListEvents(ctx context.Context, calendarID string, opts ListEventsOptions) (*ListEventsResult, error) {
	endpoint := fmt.Sprintf("%s/events/%s/%s", c.baseURL, c.endUserAccountID, url.PathEscape(calendarID))

	q := url.Values{}
	if opts.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(opts.PageSize))
	}
	if opts.PageToken != "" {
		q.Set("pageToken", opts.PageToken)
	}
	if opts.SyncToken != "" {
		q.Set("syncToken", opts.SyncToken)
	}
	if opts.StartDateTime != "" {
		q.Set("startDateTime", opts.StartDateTime)
	}
	if opts.EndDateTime != "" {
		q.Set("endDateTime", opts.EndDateTime)
	}
	if opts.TimeZone != "" {
		q.Set("timeZone", opts.TimeZone)
	}
	if opts.MetadataFilters != "" {
		q.Set("metadataFilters", opts.MetadataFilters)
	}
	if opts.ExpandRecurrences {
		q.Set("expandRecurrences", "true")
	}
	if opts.Search != "" {
		q.Set("search", opts.Search)
	}
	if opts.OrderBy != "" {
		q.Set("orderBy", opts.OrderBy)
	}
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	body, err := c.do(ctx, http.MethodGet, endpoint, nil, "list events")
	if err != nil {
		return nil, err
	}

	var out ListEventsResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("apiroc: decode: %w", err)
	}

	return &out, nil
}

func (c *Client) GetEvent(ctx context.Context, calendarID, eventID string) (*Event, error) {
	endpoint := fmt.Sprintf("%s/events/%s/%s/%s", c.baseURL, c.endUserAccountID, url.PathEscape(calendarID), url.PathEscape(eventID))

	body, err := c.do(ctx, http.MethodGet, endpoint, nil, "get event")
	if err != nil {
		return nil, err
	}

	var out Event
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("apiroc: decode: %w", err)
	}

	return &out, nil
}

func (c *Client) CreateEvent(ctx context.Context, calendarID string, event CreateEventRequest) (*Event, error) {
	endpoint := fmt.Sprintf("%s/events/%s/%s", c.baseURL, c.endUserAccountID, url.PathEscape(calendarID))

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("apiroc: encode: %w", err)
	}

	body, err := c.do(ctx, http.MethodPost, endpoint, payload, "create event")
	if err != nil {
		return nil, err
	}

	var out Event
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("apiroc: decode: %w", err)
	}

	return &out, nil
}
