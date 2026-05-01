package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func modelsCfg(t *testing.T, endpoint string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yml")
	body := fmt.Sprintf("endpoint: %s\napi_key: k\nmodel: m\ndefaults:\n  timeout: 2s\n  stream_idle_timeout: 2s\n  retries: 0\n", endpoint)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestModels_PlainSortedOneIDPerLine(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[{"id":"zebra"},{"id":"apple"},{"id":"mango"}]}`)
	}))
	defer srv.Close()

	stdout, _, code := runRoot(t, "-c", modelsCfg(t, srv.URL+"/v1"), "models")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	want := []string{"apple", "mango", "zebra"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v; want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("lines[%d] = %q; want %q", i, lines[i], want[i])
		}
	}
}

func TestModels_JSONPassthrough(t *testing.T) {
	t.Parallel()
	raw := `{"object":"list","data":[{"id":"m1","object":"model"}],"extra_vendor_field":"preserved"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, raw)
	}))
	defer srv.Close()

	stdout, _, code := runRoot(t, "-c", modelsCfg(t, srv.URL+"/v1"), "models", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	// Output should contain the verbatim body (plus a trailing newline).
	if !strings.Contains(stdout, raw) {
		t.Errorf("stdout missing verbatim body; got %q", stdout)
	}
	if !strings.Contains(stdout, "extra_vendor_field") {
		t.Errorf("vendor-specific field was stripped; got %q", stdout)
	}
}

func TestModels_UnreachableExits5(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runRoot(t, "-c", modelsCfg(t, "http://127.0.0.1:1/v1"), "models")
	_ = stdout
	if code != ExitNetwork {
		t.Errorf("exit = %d; want %d", code, ExitNetwork)
	}
	if !strings.Contains(stderr, "askit: endpoint:") {
		t.Errorf("stderr not endpoint-categorized: %s", stderr)
	}
}

func TestModels_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"bad key"}}`)
	}))
	defer srv.Close()
	_, stderr, code := runRoot(t, "-c", modelsCfg(t, srv.URL+"/v1"), "models")
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "401") {
		t.Errorf("stderr missing status: %s", stderr)
	}
}
