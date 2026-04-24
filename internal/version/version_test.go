package version_test

import (
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/version"
)

func TestDefaultsNonEmpty(t *testing.T) {
	t.Parallel()
	if version.Version == "" {
		t.Error("Version must not be empty")
	}
	if version.Commit == "" {
		t.Error("Commit must not be empty")
	}
	if version.Date == "" {
		t.Error("Date must not be empty")
	}
}

func TestInfoContainsAllFields(t *testing.T) {
	t.Parallel()
	got := version.Info()
	for _, want := range []string{version.Version, version.Commit, version.Date} {
		if !strings.Contains(got, want) {
			t.Errorf("Info() = %q; want substring %q", got, want)
		}
	}
	if !strings.HasPrefix(got, "askit ") {
		t.Errorf("Info() = %q; want prefix %q", got, "askit ")
	}
}
