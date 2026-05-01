package cli

import (
	"strings"
	"testing"
)

// TestHelp_APIKeyDiscouraged locks in the wording required by FR-016 so
// the "discouraged — visible in ps" nudge doesn't silently disappear in a
// future refactor.
func TestHelp_APIKeyDiscouraged(t *testing.T) {
	t.Parallel()
	stdout, _, code := runRoot(t, "--help")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "--api-key") {
		t.Error("help missing --api-key flag")
	}
	if !strings.Contains(stdout, "discouraged") {
		t.Errorf("help missing 'discouraged' nudge on --api-key:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ASKIT_API_KEY") {
		t.Errorf("help should reference ASKIT_API_KEY env var")
	}
}

// TestHelp_QueryFlagsDocumented: every query-specific flag mentions
// either its env-var counterpart or "from config" so users can tell
// precedence without reading docs.
func TestHelp_QueryFlagsDocumented(t *testing.T) {
	t.Parallel()
	stdout, _, code := runRoot(t, "query", "--help")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	required := []string{
		"--preset",
		"--file",
		"--out",
		"--force",
		"--output",
		"--stream",
		"--no-stream",
		"--timeout",
		"--stream-idle-timeout",
		"--retries",
		"--dry-run",
	}
	for _, flag := range required {
		if !strings.Contains(stdout, flag) {
			t.Errorf("query --help missing %s", flag)
		}
	}
}
