package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// errStreamIdleTimeout is the sentinel wrapped into TimeoutError for stream-idle violations.
var errStreamIdleTimeout = errors.New("stream-idle timeout")

// synthesizedStreamStatus is the Status we report on *APIError values that
// describe a malformed-but-200 streaming response. Keeping it at 200
// distinguishes these from genuine non-2xx responses, which carry the
// upstream status verbatim.
const synthesizedStreamStatus = http.StatusOK

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
	const chunkBufSize = 16
	r.Stream = true
	chunks := make(chan StreamChunk, chunkBufSize)
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
		defer func() { _ = resp.Body.Close() }()

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
//
// Returns an *APIError when the stream closes cleanly (EOF) without ever
// delivering a [DONE] marker AND without producing any chunk — that
// pattern indicates the upstream truncated the response before any
// payload reached us.
func (c *Client) drainSSE(ctx context.Context, body io.Reader, chunks chan<- StreamChunk) error {
	idle := time.NewTimer(c.StreamIdleTimeout)
	defer idle.Stop()

	readCtx, cancelReader := context.WithCancel(ctx)
	defer cancelReader()
	lineCh, errCh := startLineReader(readCtx, bufio.NewReader(body))

	var event bytes.Buffer
	var st sseState
	for {
		done, err := c.drainSSEStep(ctx, idle, lineCh, errCh, chunks, &event, &st)
		if err != nil {
			return err
		}
		if done {
			if st.sawDone || st.chunks > 0 {
				return nil
			}
			return &APIError{Status: synthesizedStreamStatus, Detail: "stream closed before any data was received"}
		}
	}
}

// sseState tracks loop-scoped counters used to decide whether a clean
// EOF should be treated as success or as an empty-stream API error.
type sseState struct {
	sawDone bool
	chunks  int
}

// drainSSEStep processes one select iteration of the SSE event loop.
// Returns (done=true, nil) when the stream loop should exit (either the
// [DONE] terminator is seen or the underlying reader hits EOF).
func (c *Client) drainSSEStep(
	ctx context.Context,
	idle *time.Timer,
	lineCh <-chan []byte,
	errCh <-chan error,
	chunks chan<- StreamChunk,
	event *bytes.Buffer,
	st *sseState,
) (bool, error) {
	select {
	case <-ctx.Done():
		return false, &TimeoutError{Which: "cancelled", Err: ctx.Err()}
	case <-idle.C:
		return false, &TimeoutError{Which: "stream-idle", Err: fmt.Errorf("no chunk within %s: %w", c.StreamIdleTimeout, errStreamIdleTimeout)}
	case err := <-errCh:
		// The line reader and the error path are independent channels,
		// so on EOF a line may still be buffered. Drain whatever's
		// pending and dispatch a half-buffered event before declaring
		// the stream done.
		if errors.Is(err, io.EOF) {
			if drainErr := drainPending(lineCh, event, chunks, st); drainErr != nil {
				return false, drainErr
			}
			return true, nil
		}
		return false, fmt.Errorf("read stream: %w", err)
	case line := <-lineCh:
		resetTimer(idle, c.StreamIdleTimeout)
		return processSSELine(string(line), event, chunks, st)
	}
}

// drainPending consumes any line still buffered in lineCh and flushes the
// remaining event buffer before the loop exits on EOF.
func drainPending(lineCh <-chan []byte, event *bytes.Buffer, chunks chan<- StreamChunk, st *sseState) error {
	for {
		select {
		case line := <-lineCh:
			if _, err := processSSELine(string(line), event, chunks, st); err != nil {
				return err
			}
		default:
			if event.Len() > 0 {
				if _, err := dispatchEvent(event.Bytes(), chunks, st); err != nil {
					event.Reset()
					return err
				}
				event.Reset()
			}
			return nil
		}
	}
}

// processSSELine handles one raw SSE line. Returns (done, err).
func processSSELine(line string, event *bytes.Buffer, chunks chan<- StreamChunk, st *sseState) (bool, error) {
	trimmed := strings.TrimRight(line, "\r\n")
	if strings.HasPrefix(trimmed, ":") {
		// Comment / heartbeat — ignore.
		return false, nil
	}
	if trimmed != "" {
		event.WriteString(trimmed)
		event.WriteByte('\n')
		return false, nil
	}
	// Empty line = event separator. Dispatch buffered data.
	if event.Len() == 0 {
		return false, nil
	}
	done, err := dispatchEvent(event.Bytes(), chunks, st)
	event.Reset()
	return done, err
}

// startLineReader launches a background goroutine that reads lines from r and
// sends them on lineCh. Any read error (including io.EOF) is sent on errCh.
func startLineReader(ctx context.Context, r *bufio.Reader) (<-chan []byte, <-chan error) {
	lineCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				select {
				case lineCh <- line:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}
		}
	}()
	return lineCh, errCh
}

// dispatchEvent decodes a single SSE event (potentially multi-line `data:`
// entries) and emits a StreamChunk. Returns done=true when the event is
// the upstream's `[DONE]` terminator. An error envelope or
// `finish_reason:"error"` in the payload is surfaced as *APIError.
func dispatchEvent(payload []byte, out chan<- StreamChunk, st *sseState) (bool, error) {
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
		st.sawDone = true
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
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return false, fmt.Errorf("decode sse chunk: %w", err)
	}
	if chunk.Error != nil {
		detail := chunk.Error.Message
		if detail == "" {
			detail = "upstream signaled an error in stream"
		}
		return false, &APIError{Status: synthesizedStreamStatus, Body: []byte(data), Detail: detail}
	}
	sc := StreamChunk{Usage: chunk.Usage}
	if len(chunk.Choices) > 0 {
		sc.Delta = chunk.Choices[0].Delta.Content
		sc.FinishReason = chunk.Choices[0].FinishReason
	}
	if sc.FinishReason == "error" {
		return false, &APIError{Status: synthesizedStreamStatus, Body: []byte(data), Detail: "upstream finished with finish_reason=error"}
	}
	st.chunks++
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
