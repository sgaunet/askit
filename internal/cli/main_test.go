package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain isolates every test in this package from the host user's real
// askit config file. Without this, tests that call Load with no -c flag
// would pick up ~/.config/askit/config.yml on macOS/Linux, making tests
// behave differently depending on whose laptop is running them.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "askit-cli-test-xdg-*")
	if err != nil {
		panic("setup: " + err.Error())
	}
	defer os.RemoveAll(dir)

	// Point XDG_CONFIG_HOME at an empty tempdir so DefaultConfigPath()
	// resolves to <tempdir>/askit/config.yml which does not exist.
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(dir))
	// Defensive: clear the other env vars that could bleed in.
	_ = os.Unsetenv("ASKIT_CONFIG")
	_ = os.Unsetenv("ASKIT_ENDPOINT")
	_ = os.Unsetenv("ASKIT_MODEL")
	_ = os.Unsetenv("ASKIT_API_KEY")

	code := m.Run()
	os.Exit(code)
}
