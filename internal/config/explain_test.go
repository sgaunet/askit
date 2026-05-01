package config_test

import (
	"testing"

	"github.com/sgaunet/askit/internal/config"
)

func TestExplain_AllFieldsPresent(t *testing.T) {
	// Isolate from any real config file on the host: point the default
	// path at an empty tempdir via XDG_CONFIG_HOME. t.Setenv forbids
	// t.Parallel(), so this test is sequential.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	res, err := config.Load(config.LoadOptions{
		FlagOverrides: config.Overrides{
			Endpoint: strPtr("http://flag-endpoint"),
			Model:    strPtr("flag-model"),
			Source:   config.SourceFlag,
		},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	lines, err := config.Explain(res.Config, res.Provenance)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	seen := map[string]config.ExplainLine{}
	for _, l := range lines {
		seen[l.Field] = l
	}
	if seen["endpoint"].Source != config.SourceFlag {
		t.Errorf("endpoint source = %q; want flag", seen["endpoint"].Source)
	}
	if seen["endpoint"].Value != "http://flag-endpoint" {
		t.Errorf("endpoint value = %q; want http://flag-endpoint", seen["endpoint"].Value)
	}
	if seen["defaults.temperature"].Source != config.SourceBuiltin {
		t.Errorf("defaults.temperature source = %q; want builtin", seen["defaults.temperature"].Source)
	}
	if seen["defaults.timeout"].Value != "1m0s" {
		t.Errorf("defaults.timeout value = %q; want 1m0s", seen["defaults.timeout"].Value)
	}
}
