package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests cover scenarios where the upstream returns HTTP 200 but the
// payload is semantically broken (empty choices, an error envelope, an
// error finish reason, an error event in stream, or a stream that closed
// without delivering any data). All of them must exit with ExitAPI so a
// caller scripting against askit can tell the call did not produce a
// usable answer.

func TestE2E_QueryEmptyChoices(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusOK, `{"choices":[]}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "hi"}, "")
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "no choices") {
		t.Errorf("stderr should mention empty choices, got:\n%s", stderr)
	}
}

func TestE2E_QueryErrorEnvelopeOn200(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusOK, `{"error":{"message":"upstream boom","type":"server_error"}}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "hi"}, "")
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "upstream boom") {
		t.Errorf("stderr should surface upstream message, got:\n%s", stderr)
	}
}

func TestE2E_QueryFinishReasonError(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusOK, `{
		"choices":[{"message":{"content":"partial"},"finish_reason":"error"}]
	}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "hi"}, "")
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "finish_reason=error") {
		t.Errorf("stderr should name finish_reason=error, got:\n%s", stderr)
	}
}

func TestE2E_QueryStreamErrorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		_, _ = fmt.Fprintln(w, `data: {"error":{"message":"mid-stream boom","type":"server_error"}}`)
		_, _ = fmt.Fprintln(w, "")
		if fl != nil {
			fl.Flush()
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "--stream", "hi"}, "")
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "mid-stream boom") {
		t.Errorf("stderr should surface mid-stream error, got:\n%s", stderr)
	}
}

func TestE2E_QueryStreamEOFNoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Server closes immediately with no SSE events and no [DONE].
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "--stream", "hi"}, "")
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "stream closed before any data") {
		t.Errorf("stderr should explain empty stream, got:\n%s", stderr)
	}
}
