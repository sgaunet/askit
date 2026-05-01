package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Model is one entry from /v1/models.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by,omitempty"`
	Created int64  `json:"created,omitempty"`
}

// ModelsResponse is the full upstream response body, preserved verbatim
// for the `--json` output form of `askit models`.
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
	Raw    []byte  `json:"-"` // for --json passthrough
}

// ListModels performs GET {endpoint}/models. Respects the connect+TTFB
// timeout configured on the Client. Callers wanting retry semantics should
// wrap via DoWithRetry.
func (c *Client) ListModels(ctx context.Context) (*ModelsResponse, error) {
	u, err := url.JoinPath(c.Endpoint, "models")
	if err != nil {
		return nil, fmt.Errorf("build URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, wrapTransport(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			Status: resp.StatusCode,
			Body:   body,
			Header: resp.Header.Clone(),
		}
	}
	var out ModelsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	out.Raw = body
	return &out, nil
}

// Unused but kept to keep time imported for future streaming-models paths.
var _ = time.Second
