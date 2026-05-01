package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// TestE2E_MultiConfig switches backends purely via -c, verifying each
// invocation targets the correct fake server (US3 acceptance scenario 1).
func TestE2E_MultiConfig(t *testing.T) {
	// Not parallel — the E2E helper mutates os.Stdin which races with
	// other tests that do the same.
	var hitsA, hitsB atomic.Int64
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hitsA.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"A"}}]}`)
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hitsB.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"B"}}]}`)
	}))
	defer srvB.Close()

	dir := t.TempDir()
	cfgA := filepath.Join(dir, "a.yml")
	cfgB := filepath.Join(dir, "b.yml")
	must := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	base := func(endpoint string) string {
		return fmt.Sprintf("endpoint: %s\nmodel: m\napi_key: k\ndefaults:\n  stream: false\n  retries: 0\n", endpoint)
	}
	must(cfgA, base(srvA.URL+"/v1"))
	must(cfgB, base(srvB.URL+"/v1"))

	// Drive twice with different -c.
	stdoutA, _, codeA := runRootWithStdin(t, []string{"-c", cfgA, "query", "hi"}, "")
	stdoutB, _, codeB := runRootWithStdin(t, []string{"-c", cfgB, "query", "hi"}, "")

	if codeA != ExitOK || !strings.Contains(stdoutA, "A") {
		t.Errorf("A run failed: code=%d stdout=%q", codeA, stdoutA)
	}
	if codeB != ExitOK || !strings.Contains(stdoutB, "B") {
		t.Errorf("B run failed: code=%d stdout=%q", codeB, stdoutB)
	}
	if hitsA.Load() != 1 || hitsB.Load() != 1 {
		t.Errorf("unexpected hits: A=%d B=%d (want 1/1)", hitsA.Load(), hitsB.Load())
	}
}

// TestE2E_MissingRequiredExitsThree verifies FR-013 aggregated error when
// nothing supplies endpoint/model.
func TestE2E_MissingRequiredExitsThree(t *testing.T) {
	t.Setenv("ASKIT_ENDPOINT", "")
	t.Setenv("ASKIT_MODEL", "")
	t.Setenv("ASKIT_API_KEY", "")
	t.Setenv("ASKIT_CONFIG", "")
	// Use a non-default HOME so the default config doesn't kick in.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, stderr, code := runRoot(t, "query", "hello")
	if code != ExitConfig {
		t.Errorf("exit = %d; want %d; stderr=%s", code, ExitConfig, stderr)
	}
	if !strings.Contains(stderr, "endpoint: required") {
		t.Errorf("stderr missing endpoint error: %s", stderr)
	}
	if !strings.Contains(stderr, "model: required") {
		t.Errorf("stderr missing model error: %s", stderr)
	}
}

// TestE2E_UnknownPreset verifies FR-033: error lists available preset names.
func TestE2E_UnknownPreset(t *testing.T) {
	// Not parallel — mutates os.Stdin via runRootWithStdin.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "c.yml")
	body := `
endpoint: http://x/v1
api_key: k
model: m
presets:
  ocr:
    system: "OCR"
  md:
    system: "MD"
`
	_ = os.WriteFile(cfg, []byte(body), 0o600)

	_, stderr, code := runRootWithStdin(t, []string{"-c", cfg, "query", "-p", "nope", "hello"}, "")
	if code != ExitConfig {
		t.Errorf("exit = %d; want %d", code, ExitConfig)
	}
	for _, want := range []string{"nope", "md", "ocr"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q in %q", want, stderr)
		}
	}
}
