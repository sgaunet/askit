package config_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/config"
)

func TestDefaultConfigPath_XDGWins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := config.DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	want := filepath.Join("/tmp/xdg", "askit", "config.yml")
	if got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestDefaultConfigPath_XDGEmptyFallsThrough(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home")

	got, err := config.DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	switch runtime.GOOS {
	case "windows":
		// On Windows the Unix branch is skipped; the fallback to
		// os.UserConfigDir() produces a path we don't assert on
		// precisely (it depends on %APPDATA% / USERPROFILE), but the
		// result must not be the ~/.config path.
		if strings.Contains(got, `.config/askit`) {
			t.Errorf("Windows should not use .config/askit; got %q", got)
		}
	default:
		want := filepath.Join("/tmp/home", ".config", "askit", "config.yml")
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	}
}

func TestDefaultConfigPath_XDGWhitespaceTreatedAsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "   ")
	t.Setenv("HOME", "/tmp/home")
	got, err := config.DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	if runtime.GOOS != "windows" {
		want := filepath.Join("/tmp/home", ".config", "askit", "config.yml")
		if got != want {
			t.Errorf("whitespace XDG should not count; got %q; want %q", got, want)
		}
	}
}
