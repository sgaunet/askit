package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_YAMLOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yml")
	body := `endpoint: http://x/v1
api_key: test-key
model: m
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runRoot(t, "-c", cfgPath, "config")
	if code != ExitOK {
		t.Errorf("exit = %d; want 0", code)
	}
	for _, want := range []string{"endpoint:", "http://x/v1", "model:", "presets:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("yaml missing %q in:\n%s", want, stdout)
		}
	}
}

func TestConfig_Path(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yml")
	_ = os.WriteFile(cfgPath, []byte("endpoint: http://x/v1\nmodel: m\n"), 0o600)
	stdout, _, code := runRoot(t, "-c", cfgPath, "config", "--path")
	if code != ExitOK {
		t.Errorf("exit = %d", code)
	}
	if strings.TrimSpace(stdout) != cfgPath {
		t.Errorf("path = %q; want %q", strings.TrimSpace(stdout), cfgPath)
	}
}

func TestConfig_PathWithoutFile(t *testing.T) {
	// Neither -c nor default file — path should be <builtins>.
	stdout, _, code := runRoot(t, "config", "--path")
	// If the user happens to have ~/.config/askit/config.yml we'd see its
	// path; this test treats either outcome as acceptable as long as the
	// exit code is OK.
	if code != ExitOK {
		t.Errorf("exit = %d", code)
	}
	_ = stdout
}

func TestConfig_ExplainShowsSources(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yml")
	body := `endpoint: http://from-file/v1
api_key: from-file-key
model: from-file-model
defaults:
  temperature: 0.7
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Set an env override and a flag override.
	t.Setenv("ASKIT_MODEL", "from-env")
	stdout, _, code := runRoot(t, "-c", cfgPath, "--endpoint", "http://from-flag/v1", "config", "--explain")
	if code != ExitOK {
		t.Fatalf("exit = %d; stdout=\n%s", code, stdout)
	}
	// endpoint came from --flag
	if !strings.Contains(stdout, "endpoint") || !strings.Contains(stdout, "flag") || !strings.Contains(stdout, "http://from-flag/v1") {
		t.Errorf("explain missing endpoint/flag/value in:\n%s", stdout)
	}
	// model came from env
	if !strings.Contains(stdout, "from-env") {
		t.Errorf("model env override not shown in:\n%s", stdout)
	}
	// api_key came from explicit-file
	if !strings.Contains(stdout, "explicit-file") {
		t.Errorf("explicit-file source label missing in:\n%s", stdout)
	}
	// defaults.temperature override shown
	if !strings.Contains(stdout, "defaults.temperature") || !strings.Contains(stdout, "0.7") {
		t.Errorf("temperature override missing in:\n%s", stdout)
	}
}

func TestConfig_ExplainAPIKeyShown(t *testing.T) {
	// FR-092 exception: askit config --explain MAY show the key (user-initiated).
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yml")
	secret := fmt.Sprintf("sk-local-%d", 12345)
	body := fmt.Sprintf("endpoint: http://x/v1\napi_key: %s\nmodel: m\n", secret)
	_ = os.WriteFile(cfgPath, []byte(body), 0o600)

	stdout, _, code := runRoot(t, "-c", cfgPath, "config", "--explain")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, secret) {
		t.Errorf("--explain should show api_key verbatim; got:\n%s", stdout)
	}
}

func TestConfig_PathAndExplainMutuallyExclusive(t *testing.T) {
	_, stderr, code := runRoot(t, "config", "--path", "--explain")
	if code != ExitUsage {
		t.Errorf("exit = %d; want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("stderr = %s", stderr)
	}
}
