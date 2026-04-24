package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client issues HTTP requests against an OpenAI-compatible endpoint.
// Safe for concurrent use: the underlying *http.Client has its own
// connection pooling.
type Client struct {
	Endpoint   string // base URL including /v1
	APIKey     string
	HTTPClient *http.Client
	// ConnectTimeout is the wall-clock deadline for establishing the
	// connection and receiving the first response byte (FR-070 item 1).
	ConnectTimeout time.Duration
	// StreamIdleTimeout is the max silence between chunks once streaming
	// has begun (FR-070 item 2).
	StreamIdleTimeout time.Duration
}

// New constructs a Client with a per-instance *http.Transport whose
// ResponseHeaderTimeout enforces the connect+TTFB deadline without
// cancelling the body-read phase. Body-read idle is enforced by the
// streaming logic in stream.go (stream-idle timeout).
func New(endpoint, apiKey string, connectTimeout, streamIdle time.Duration) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: connectTimeout,
		TLSHandshakeTimeout:   connectTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &Client{
		Endpoint:          strings.TrimRight(endpoint, "/"),
		APIKey:            apiKey,
		HTTPClient:        &http.Client{Transport: transport},
		ConnectTimeout:    connectTimeout,
		StreamIdleTimeout: streamIdle,
	}
}

// newRequest builds the HTTP request for a chat completion. Returned request
// already has the Authorization header set (if the API key is non-empty).
func (c *Client) newRequest(ctx context.Context, r *Request) (*http.Request, error) {
	body, err := buildRequestBody(r)
	if err != nil {
		return nil, err
	}
	u, err := url.JoinPath(c.Endpoint, "chat/completions")
	if err != nil {
		return nil, fmt.Errorf("build URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if r.Stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	return req, nil
}

// Complete issues a buffered (non-streaming) chat-completion request.
// Caller is responsible for wrapping with retry semantics via DoWithRetry.
func (c *Client) Complete(ctx context.Context, r *Request) (*Response, error) {
	r.Stream = false
	req, err := c.newRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, wrapTransport(err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			Status: resp.StatusCode,
			Body:   raw,
			Header: resp.Header.Clone(),
		}
	}

	var parsed struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	var text, finish string
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
		finish = parsed.Choices[0].FinishReason
	}
	return &Response{
		Text:         text,
		FinishReason: finish,
		Usage:        parsed.Usage,
		Raw:          raw,
		Duration:     time.Since(started),
	}, nil
}

// APIError is a non-2xx upstream response. The CLI layer maps it to exit
// code 6 via [cli.NewAPIErr].
type APIError struct {
	Status int
	Body   []byte
	Header http.Header
}

// Error renders the status line plus a one-line summary extracted from the
// body (when parseable as OpenAI's error envelope).
func (a *APIError) Error() string {
	msg := summarizeBody(a.Body)
	if msg != "" {
		return fmt.Sprintf("%d %s: %s", a.Status, http.StatusText(a.Status), msg)
	}
	return fmt.Sprintf("%d %s", a.Status, http.StatusText(a.Status))
}

// APIResponseBody returns the truncated upstream body for -vv output.
func (a *APIError) APIResponseBody() string {
	return string(a.Body)
}

// summarizeBody best-effort extracts `{error.message}` or `{error}` from the
// OpenAI error shape. Returns empty string if the body isn't JSON or has no
// recognizable shape.
func summarizeBody(body []byte) string {
	var env struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		return env.Error.Message
	}
	// Some endpoints put a plain string under .error.
	var flat struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &flat); err == nil && flat.Error != "" {
		return flat.Error
	}
	return ""
}

// wrapTransport turns low-level transport errors into a typed error the
// retry engine can inspect. The returned error is either a *NetworkError
// or a *TimeoutError depending on the underlying cause.
func wrapTransport(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Which: "connect+ttfb", Err: err}
	}
	if errors.Is(err, context.Canceled) {
		return &TimeoutError{Which: "cancelled", Err: err}
	}
	return &NetworkError{Err: err}
}

// NetworkError is a transport-layer failure (DNS, connection refused,
// TLS, connection reset). Retryable when the retry engine sees it before
// streaming has begun.
type NetworkError struct{ Err error }

func (n *NetworkError) Error() string { return n.Err.Error() }
func (n *NetworkError) Unwrap() error { return n.Err }

// TimeoutError is a deadline or cancellation event. Not retryable; exits
// with code 7.
type TimeoutError struct {
	Which string // "connect+ttfb" | "stream-idle" | "cancelled"
	Err   error
}

func (t *TimeoutError) Error() string { return t.Which + " timeout: " + t.Err.Error() }
func (t *TimeoutError) Unwrap() error { return t.Err }
