package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/cli"
)

func TestNewLogger_LevelMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		verbose  int
		wantInfo bool
		wantDbg  bool
	}{
		{verbose: 0, wantInfo: false, wantDbg: false},
		{verbose: 1, wantInfo: true, wantDbg: false},
		{verbose: 2, wantInfo: true, wantDbg: true},
		{verbose: 3, wantInfo: true, wantDbg: true},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			log := cli.NewLoggerTo(&buf, tt.verbose)
			log.Info("info-event", "k", "v")
			log.Debug("debug-event", "k", "v")
			out := buf.String()
			if tt.wantInfo && !strings.Contains(out, "info-event") {
				t.Errorf("verbose=%d: want info-event in output, got:\n%s", tt.verbose, out)
			}
			if !tt.wantInfo && strings.Contains(out, "info-event") {
				t.Errorf("verbose=%d: info-event should NOT appear, got:\n%s", tt.verbose, out)
			}
			if tt.wantDbg && !strings.Contains(out, "debug-event") {
				t.Errorf("verbose=%d: want debug-event, got:\n%s", tt.verbose, out)
			}
			if !tt.wantDbg && strings.Contains(out, "debug-event") {
				t.Errorf("verbose=%d: debug-event should NOT appear, got:\n%s", tt.verbose, out)
			}
		})
	}
}

func TestNewLogger_ErrorAlwaysEmitted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := cli.NewLoggerTo(&buf, 0)
	log.Error("boom", "k", "v")
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("error-level should be emitted at verbose=0, got:\n%s", buf.String())
	}
}
