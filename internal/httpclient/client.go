package httpclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	http *http.Client
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{http: &http.Client{Timeout: timeout}}
}

func (c *Client) FetchJSON(url string, out any) error {
	return c.Fetch(url, func(resp *http.Response) error {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("httpclient: decoding json: %w", err)
		}
		return nil
	})
}

func (c *Client) Fetch(url string, handle func(resp *http.Response) error) error {
	resp, err := c.http.Get(url)
	if err != nil {
		return fmt.Errorf("httpclient: get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("httpclient: get %s: status %d", url, resp.StatusCode)
	}

	return handle(resp)
}
