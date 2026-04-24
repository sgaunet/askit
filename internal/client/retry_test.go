package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/sgaunet/askit/internal/client"
)

func TestRetry_SucceedsOnFirstAttempt(t *testing.T) {
	t.Parallel()
	attempts := 0
	opts := client.DefaultRetryOptions()
	val, err := client.DoWithRetry(context.Background(), opts, func(_ context.Context) (int, error) {
		attempts++
		return 42, nil
	})
	if err != nil {
		t.Fatalf("DoWithRetry: %v", err)
	}
	if val != 42 {
		t.Errorf("val = %d; want 42", val)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d; want 1", attempts)
	}
}

func TestRetry_RetriesOn429(t *testing.T) {
	t.Parallel()
	attempts := 0
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 1 * time.Millisecond
	opts.MaxDelay = 2 * time.Millisecond
	opts.MaxAttempts = 3

	val, err := client.DoWithRetry(context.Background(), opts, func(_ context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", &client.APIError{
				Status: http.StatusTooManyRequests,
				Header: http.Header{},
			}
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("DoWithRetry: %v", err)
	}
	if val != "ok" {
		t.Errorf("val = %q; want ok", val)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d; want 3", attempts)
	}
}

func TestRetry_RetriesOn5xx(t *testing.T) {
	t.Parallel()
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 1 * time.Millisecond
	opts.MaxAttempts = 2

	_, err := client.DoWithRetry(context.Background(), opts, func(_ context.Context) (int, error) {
		return 0, &client.APIError{Status: http.StatusBadGateway, Header: http.Header{}}
	})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %T %v", err, err)
	}
}

func TestRetry_DoesNotRetryOn4xxOther(t *testing.T) {
	t.Parallel()
	attempts := 0
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 1 * time.Millisecond
	opts.MaxAttempts = 3

	_, err := client.DoWithRetry(context.Background(), opts, func(_ context.Context) (int, error) {
		attempts++
		return 0, &client.APIError{Status: http.StatusBadRequest, Header: http.Header{}}
	})
	if err == nil {
		t.Fatal("want error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d; want 1 (no retry on 400)", attempts)
	}
}

func TestRetry_RetriesNetworkError(t *testing.T) {
	t.Parallel()
	attempts := 0
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 1 * time.Millisecond
	opts.MaxAttempts = 2

	_, err := client.DoWithRetry(context.Background(), opts, func(_ context.Context) (int, error) {
		attempts++
		return 0, &client.NetworkError{Err: errors.New("connection refused")}
	})
	if err == nil {
		t.Fatal("want error")
	}
	if attempts != 2 {
		t.Errorf("attempts = %d; want 2", attempts)
	}
}

func TestRetry_UnexpectedEOF(t *testing.T) {
	t.Parallel()
	attempts := 0
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 1 * time.Millisecond
	opts.MaxAttempts = 2
	_, err := client.DoWithRetry(context.Background(), opts, func(_ context.Context) (int, error) {
		attempts++
		return 0, io.ErrUnexpectedEOF
	})
	if err == nil {
		t.Fatal("want error")
	}
	if attempts != 2 {
		t.Errorf("attempts = %d; want 2", attempts)
	}
}

func TestRetry_RespectsCtxCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 50 * time.Millisecond
	opts.MaxAttempts = 5

	done := make(chan error, 1)
	go func() {
		_, err := client.DoWithRetry(ctx, opts, func(_ context.Context) (int, error) {
			return 0, &client.APIError{Status: http.StatusServiceUnavailable, Header: http.Header{}}
		})
		done <- err
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("want error after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Error("retry did not honor cancellation")
	}
}

func TestRetry_HonorsRetryAfterSeconds(t *testing.T) {
	t.Parallel()
	attempts := 0
	opts := client.DefaultRetryOptions()
	opts.BaseDelay = 1 * time.Millisecond
	opts.MaxAttempts = 2
	start := time.Now()
	_, _ = client.DoWithRetry(context.Background(), opts, func(_ context.Context) (int, error) {
		attempts++
		h := http.Header{}
		h.Set("Retry-After", "0") // immediate but respected
		return 0, &client.APIError{Status: 429, Header: h}
	})
	if attempts != 2 {
		t.Errorf("attempts = %d; want 2", attempts)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("test took too long")
	}
}
