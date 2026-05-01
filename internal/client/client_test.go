package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/prompt"
)

func buildReq() *client.Request {
	return &client.Request{
		Model: "test-model",
		Prompt: &prompt.Prompt{
			System: "you are helpful",
			Messages: []prompt.Message{{
				Role: "user",
				Content: []prompt.ContentPart{{Type: prompt.PartTypeText, Text: "hi"}},
			}},
		},
		Temperature: 0.2,
		MaxTokens:   100,
	}
}

func TestComplete_HappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %q; want Bearer test-key", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"finish_reason": "stop",
					"message":       map[string]any{"content": "hello back"},
				},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, "test-key", 5*time.Second, 5*time.Second)
	resp, err := c.Complete(context.Background(), buildReq())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "hello back" {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("finish = %q", resp.FinishReason)
	}
	if resp.Usage.TotalTokens != 7 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestComplete_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key","type":"auth"}}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "bad", time.Second, time.Second)
	_, err := c.Complete(context.Background(), buildReq())
	if err == nil {
		t.Fatal("want error")
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusUnauthorized {
		t.Errorf("status = %d", apiErr.Status)
	}
	if apiErr.Error() == "" || apiErr.APIResponseBody() == "" {
		t.Errorf("APIError content missing")
	}
}

func TestComplete_NetworkError(t *testing.T) {
	t.Parallel()
	// Point at a socket we expect to refuse.
	c := client.New("http://127.0.0.1:1", "", time.Second, time.Second)
	_, err := c.Complete(context.Background(), buildReq())
	if err == nil {
		t.Fatal("want network error")
	}
	var ne *client.NetworkError
	if !errors.As(err, &ne) {
		t.Errorf("want *NetworkError, got %T: %v", err, err)
	}
}
