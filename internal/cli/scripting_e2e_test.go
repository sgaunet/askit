package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestE2E_JSONOutput(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusOK, `{
		"choices":[{"message":{"content":"extracted"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}
	}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	stdout, _, code := runRootWithStdin(t, []string{"-c", cfg, "query", "--output", "json", "hi"}, "")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v\nstdout:%s", err, stdout)
	}
	resp := parsed["response"].(map[string]any)
	if resp["text"] != "extracted" {
		t.Errorf("text wrong: %+v", resp)
	}
	if !strings.Contains(stdout, `"total_tokens":6`) {
		t.Errorf("usage missing: %s", stdout)
	}
}

func TestE2E_RawOutput(t *testing.T) {
	raw := `{"id":"x","choices":[{"message":{"content":"raw!"}}]}`
	srv := fakeEndpoint(t, http.StatusOK, raw)
	defer srv.Close()
	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	stdout, _, code := runRootWithStdin(t, []string{"-c", cfg, "query", "--output", "raw", "hi"}, "")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, raw) {
		t.Errorf("raw body not passed through; got %q", stdout)
	}
}

func TestE2E_StreamIdleTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
		// First chunk arrives fast; then silence past the idle deadline.
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"start"}}]}`)
		_, _ = fmt.Fprintln(w, "")
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")

	_, stderr, code := runRootWithStdin(t, []string{
		"-c", cfg, "query", "--stream", "--stream-idle-timeout", "50ms", "hi",
	}, "")
	if code != ExitTimeout {
		t.Errorf("exit = %d; want %d", code, ExitTimeout)
	}
	if !strings.Contains(stderr, "stream-idle") {
		t.Errorf("stderr should name stream-idle: %s", stderr)
	}
}

func TestE2E_DryRunNeverLeaksKey(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, "http://unreachable/v1", "")
	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "--dry-run", "hello"}, "")
	if code != ExitOK {
		t.Errorf("exit = %d", code)
	}
	if strings.Contains(stderr, "test-key") {
		t.Errorf("stderr leaked API key: %s", stderr)
	}
	if !strings.Contains(stderr, `"Authorization": "***"`) {
		t.Errorf("Authorization not redacted: %s", stderr)
	}
}

func TestE2E_ShellCompletionWorks(t *testing.T) {
	t.Parallel()
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			stdout, _, code := runRoot(t, "completion", shell)
			if code != ExitOK {
				t.Errorf("completion %s exit = %d", shell, code)
			}
			if len(stdout) < 100 {
				t.Errorf("completion %s too short: %d bytes", shell, len(stdout))
			}
		})
	}
}
