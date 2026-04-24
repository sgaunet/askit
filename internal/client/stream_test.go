package client_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sgaunet/askit/internal/client"
)

func sseHandler(events []string, flushDelay time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		for _, e := range events {
			_, _ = fmt.Fprintln(w, e)
			_, _ = fmt.Fprintln(w, "")
			if fl != nil {
				fl.Flush()
			}
			if flushDelay > 0 {
				time.Sleep(flushDelay)
			}
		}
	})
}

func TestStream_HappyPath(t *testing.T) {
	t.Parallel()
	events := []string{
		`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
		`data: {"choices":[{"delta":{"content":" world"}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		`data: [DONE]`,
	}
	srv := httptest.NewServer(sseHandler(events, 0))
	defer srv.Close()

	c := client.New(srv.URL, "k", 5*time.Second, 5*time.Second)
	chunks, errs := c.Stream(context.Background(), buildReq())
	var combined strings.Builder
	var finish string
	for ch := range chunks {
		combined.WriteString(ch.Delta)
		if ch.FinishReason != "" {
			finish = ch.FinishReason
		}
	}
	if err, ok := <-errs; ok && err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if got := combined.String(); got != "Hello world" {
		t.Errorf("combined = %q", got)
	}
	if finish != "stop" {
		t.Errorf("finish = %q", finish)
	}
}

func TestStream_IdleTimeout(t *testing.T) {
	t.Parallel()
	events := []string{
		`data: {"choices":[{"delta":{"content":"start"}}]}`,
		// Long sleep after first chunk to blow past idle.
	}
	srv := httptest.NewServer(sseHandler(events, 200*time.Millisecond))
	defer srv.Close()

	c := client.New(srv.URL, "k", 5*time.Second, 50*time.Millisecond)
	chunks, errs := c.Stream(context.Background(), buildReq())
	for range chunks {
		// drain
	}
	err := <-errs
	if err == nil {
		t.Fatal("want idle timeout error")
	}
	var te *client.TimeoutError
	if !errors.As(err, &te) || te.Which != "stream-idle" {
		t.Errorf("want stream-idle timeout, got %T %v", err, err)
	}
}

func TestDrainStream(t *testing.T) {
	t.Parallel()
	events := []string{
		`data: {"choices":[{"delta":{"content":"a"}}]}`,
		`data: {"choices":[{"delta":{"content":"b"}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}
	srv := httptest.NewServer(sseHandler(events, 0))
	defer srv.Close()

	c := client.New(srv.URL, "k", 5*time.Second, 5*time.Second)
	chunks, errs := c.Stream(context.Background(), buildReq())
	resp, err := client.DrainStream(chunks, errs)
	if err != nil {
		t.Fatalf("DrainStream: %v", err)
	}
	if resp.Text != "ab" {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("finish = %q", resp.FinishReason)
	}
}
