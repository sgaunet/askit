package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Stream issues a streaming chat-completion request and returns a channel
// of [StreamChunk] plus an error channel that carries at most one failure
// before closing. Callers MUST drain both; closing the returned context
// aborts the stream.
//
// The method enforces two deadlines:
//
//  1. Connect+TTFB: if no response header arrives within ConnectTimeout,
//     the request is aborted with a TimeoutError{Which:"connect+ttfb"}.
//  2. Stream-idle: after the first chunk, each subsequent read has up to
//     StreamIdleTimeout before aborting with TimeoutError{Which:"stream-idle"}.
//
// A helper [DrainStream] aggregates chunks into a [Response] for callers
// that want buffered semantics anyway (e.g. `--output json` with streaming
// requested).
func (c *Client) Stream(ctx context.Context, r *Request) (<-chan StreamChunk, <-chan error) {
	r.Stream = true
	chunks := make(chan StreamChunk, 16)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		req, err := c.newRequest(ctx, r)
		if err != nil {
			errs <- err
			return
		}

		// Phase 1: connect + TTFB is enforced by the Transport's
		// ResponseHeaderTimeout. We let the request's context remain the
		// parent ctx so body-reads in phase 2 are bounded only by
		// cancellation or the explicit stream-idle timer.
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			errs <- wrapTransport(err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			errs <- &APIError{Status: resp.StatusCode, Body: body, Header: resp.Header.Clone()}
			return
		}

		// Phase 2: stream with per-chunk idle timeout.
		if err := c.drainSSE(ctx, resp.Body, chunks); err != nil {
			errs <- err
		}
	}()

	return chunks, errs
}

// drainSSE reads the server-sent events stream and pushes deltas on the
// chunks channel. The per-chunk idle timer is reset whenever a new byte
// arrives.
func (c *Client) drainSSE(ctx context.Context, body io.Reader, chunks chan<- StreamChunk) error {
	reader := bufio.NewReader(body)
	idle := time.NewTimer(c.StreamIdleTimeout)
	defer idle.Stop()

	lineCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	// Background reader.
	readCtx, cancelReader := context.WithCancel(ctx)
	defer cancelReader()
	go func() {
		for {
			if readCtx.Err() != nil {
				return
			}
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				select {
				case lineCh <- line:
				case <-readCtx.Done():
					return
				}
			}
			if err != nil {
				select {
				case errCh <- err:
				case <-readCtx.Done():
				}
				return
			}
		}
	}()

	var event bytes.Buffer
	for {
		select {
		case <-ctx.Done():
			return &TimeoutError{Which: "cancelled", Err: ctx.Err()}
		case <-idle.C:
			return &TimeoutError{Which: "stream-idle", Err: fmt.Errorf("no chunk within %s", c.StreamIdleTimeout)}
		case err := <-errCh:
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stream: %w", err)
		case line := <-lineCh:
			resetTimer(idle, c.StreamIdleTimeout)
			trimmed := strings.TrimRight(string(line), "\r\n")
			if trimmed == "" {
				// Event separator: dispatch buffered data.
				if event.Len() == 0 {
					continue
				}
				done, err := dispatchEvent(event.Bytes(), chunks)
				if err != nil {
					return err
				}
				event.Reset()
				if done {
					return nil
				}
				continue
			}
			if strings.HasPrefix(trimmed, ":") {
				// Comment / heartbeat — ignore.
				continue
			}
			event.WriteString(trimmed)
			event.WriteByte('\n')
		}
	}
}

// dispatchEvent decodes a single SSE event (potentially multi-line `data:`
// entries) and emits a StreamChunk. Returns done=true when the event is
// the upstream's `[DONE]` terminator.
func dispatchEvent(payload []byte, out chan<- StreamChunk) (done bool, err error) {
	var dataParts []string
	for line := range strings.SplitSeq(string(payload), "\n") {
		if rest, ok := strings.CutPrefix(line, "data:"); ok {
			dataParts = append(dataParts, strings.TrimPrefix(strings.TrimSpace(rest), " "))
		}
	}
	data := strings.Join(dataParts, "")
	if data == "" {
		return false, nil
	}
	if data == "[DONE]" {
		return true, nil
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return false, fmt.Errorf("decode sse chunk: %w", err)
	}
	sc := StreamChunk{Usage: chunk.Usage}
	if len(chunk.Choices) > 0 {
		sc.Delta = chunk.Choices[0].Delta.Content
		sc.FinishReason = chunk.Choices[0].FinishReason
	}
	out <- sc
	return false, nil
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

// DrainStream reads a stream to completion and returns an aggregated
// [Response]. Useful when the caller wanted streaming semantics for
// timing but needs the final buffered result.
func DrainStream(chunks <-chan StreamChunk, errs <-chan error) (*Response, error) {
	var (
		sb     strings.Builder
		finish string
		usage  Usage
	)
	for chunk := range chunks {
		sb.WriteString(chunk.Delta)
		if chunk.FinishReason != "" {
			finish = chunk.FinishReason
		}
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
	}
	if err, ok := <-errs; ok && err != nil {
		return nil, err
	}
	return &Response{
		Text:         sb.String(),
		FinishReason: finish,
		Usage:        usage,
	}, nil
}

// Ensure the http package remains used even when client.go is the only
// path that touches it in some builds.
var _ = http.MethodPost
