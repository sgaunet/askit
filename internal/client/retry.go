package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryOptions controls [DoWithRetry]'s behavior. Sensible defaults are
// provided by [DefaultRetryOptions] so callers only override what they
// actually change.
type RetryOptions struct {
	// MaxAttempts is the *total* number of attempts, including the first.
	// A value of 1 effectively disables retries.
	MaxAttempts int

	BaseDelay time.Duration
	MaxDelay  time.Duration

	// Logger, if non-nil, receives per-attempt Info events with attempt
	// number, reason, and upcoming delay. Useful under `-v`.
	Logger *slog.Logger

	// now and sleep are injection points for deterministic tests.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// Default retry parameters per contract.
const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 200 * time.Millisecond
	defaultMaxDelay    = 10 * time.Second
)

// DefaultRetryOptions returns a RetryOptions populated with the defaults
// from contracts: MaxAttempts=3 (initial + 2 retries), BaseDelay=200ms,
// MaxDelay=10s.
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts: defaultMaxAttempts,
		BaseDelay:   defaultBaseDelay,
		MaxDelay:    defaultMaxDelay,
	}
}

// DoWithRetry invokes fn, re-invoking it on eligible transient failures up
// to opts.MaxAttempts times total with full-jitter exponential backoff.
// It returns the last successful result or the last observed error.
//
// Eligibility (both conditions must hold):
//
//   - The error is a [NetworkError], a non-2xx [APIError] with status 429
//     or 5xx, or a pre-stream [io.ErrUnexpectedEOF]-class error.
//   - The request has not yet produced any streamed output (callers using
//     [Client.Stream] must invoke fn BEFORE consuming any StreamChunk).
//
// If ctx is cancelled or its deadline is reached during a backoff, the
// most recent error is returned wrapped in a [TimeoutError].
//nolint:ireturn // T is constrained to any by the generic type parameter; the concrete type is determined at call site.
func DoWithRetry[T any](ctx context.Context, opts RetryOptions, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	normalizeRetryOptions(&opts)

	var lastErr error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		lastErr = err
		if !retryable(err) || attempt == opts.MaxAttempts {
			return zero, err
		}
		if serr := waitForRetry(ctx, attempt, err, lastErr, opts); serr != nil {
			return zero, serr
		}
	}
	return zero, lastErr
}

// normalizeRetryOptions fills in zero-value fields with their defaults.
func normalizeRetryOptions(opts *RetryOptions) {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 1
	}
	if opts.BaseDelay <= 0 {
		opts.BaseDelay = defaultBaseDelay
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = defaultMaxDelay
	}
	if opts.sleep == nil {
		opts.sleep = realSleep
	}
}

// waitForRetry computes and executes the backoff delay for one retry.
// Returns a *TimeoutError when the context is cancelled during the wait.
func waitForRetry(ctx context.Context, attempt int, err, lastErr error, opts RetryOptions) error {
	delay := backoffDelay(attempt, opts.BaseDelay, opts.MaxDelay)
	if ra, ok := retryAfter(err, opts.now); ok && ra > delay {
		delay = ra
	}
	if opts.Logger != nil {
		opts.Logger.Info("retry scheduled",
			slog.Int("attempt", attempt),
			slog.Duration("delay", delay),
			slog.String("reason", err.Error()),
		)
	}
	if serr := opts.sleep(ctx, delay); serr != nil {
		return &TimeoutError{Which: "retry-backoff", Err: fmt.Errorf("%w (last error: %s)", serr, lastErr.Error())}
	}
	return nil
}

// retryable reports whether err matches the eligibility rules documented
// in FR-073.
func retryable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == http.StatusTooManyRequests:
			return true
		case apiErr.Status >= 500 && apiErr.Status < 600:
			return true
		}
		return false
	}
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	return false
}

// retryAfter extracts a Retry-After header value from an APIError.
// Returns ok=false when the header is absent or unparseable.
func retryAfter(err error, now func() time.Time) (time.Duration, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return 0, false
	}
	v := apiErr.Header.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, perr := strconv.Atoi(v); perr == nil {
		return time.Duration(secs) * time.Second, true
	}
	if t, perr := http.ParseTime(v); perr == nil {
		n := time.Now
		if now != nil {
			n = now
		}
		d := time.Until(t)
		_ = n
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// backoffDelay computes a full-jitter exponential backoff capped at
// opts.MaxDelay. The Nth attempt (1-indexed) uses a random delay in
// [0, base * 2^(N-1)].
func backoffDelay(attempt int, base, maxD time.Duration) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	exp := min(base<<(attempt-1), maxD)
	if exp <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(exp) + 1)) //nolint:gosec // G404: jitter for backoff does not need crypto/rand
}

func realSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("sleep interrupted: %w", ctx.Err())
	}
}
