package config_test

import (
	"testing"
	"time"

	"github.com/sgaunet/askit/internal/config"
)

func strPtr(s string) *string            { return &s }
func fPtr(f float64) *float64            { return &f }
func iPtr(i int) *int                    { return &i }
func bPtr(b bool) *bool                  { return &b }
func ofPtr(o config.OutputFormat) *config.OutputFormat { return &o }
func dPtr(d time.Duration) *config.Duration {
	x := config.Duration(d)
	return &x
}

func TestMerge_BuiltinsWhenNoLayers(t *testing.T) {
	t.Parallel()
	cfg, prov, err := config.Merge(nil, config.Overrides{}, config.Overrides{})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if cfg.Defaults.Temperature != 0.2 {
		t.Errorf("temperature = %g; want 0.2", cfg.Defaults.Temperature)
	}
	if prov["defaults.temperature"] != config.SourceBuiltin {
		t.Errorf("provenance = %q; want builtin", prov["defaults.temperature"])
	}
}

func TestMerge_ExplicitFileOverridesDefault(t *testing.T) {
	t.Parallel()
	dflt := &config.PartialConfig{
		Endpoint: strPtr("http://from-default"),
		Model:    strPtr("m-default"),
	}
	expl := &config.PartialConfig{
		Endpoint: strPtr("http://from-explicit"),
	}
	cfg, prov, err := config.Merge([]config.FileLayer{
		{Partial: dflt, Source: config.SourceDefaultFile},
		{Partial: expl, Source: config.SourceExplicitFile},
	}, config.Overrides{}, config.Overrides{})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if cfg.Endpoint != "http://from-explicit" {
		t.Errorf("endpoint = %q; want from-explicit", cfg.Endpoint)
	}
	if prov["endpoint"] != config.SourceExplicitFile {
		t.Errorf("endpoint source = %q; want explicit-file", prov["endpoint"])
	}
	// Model came from default-file because explicit didn't set it.
	if cfg.Model != "m-default" {
		t.Errorf("model = %q; want m-default", cfg.Model)
	}
	if prov["model"] != config.SourceDefaultFile {
		t.Errorf("model source = %q; want default-file", prov["model"])
	}
}

func TestMerge_EnvBeatsFiles(t *testing.T) {
	t.Parallel()
	expl := &config.PartialConfig{
		Endpoint: strPtr("http://from-file"),
		Model:    strPtr("m-file"),
	}
	cfg, prov, err := config.Merge(
		[]config.FileLayer{{Partial: expl, Source: config.SourceExplicitFile}},
		config.Overrides{Model: strPtr("m-env"), Source: config.SourceEnv},
		config.Overrides{},
	)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if cfg.Model != "m-env" {
		t.Errorf("model = %q; want m-env", cfg.Model)
	}
	if prov["model"] != config.SourceEnv {
		t.Errorf("model source = %q; want env", prov["model"])
	}
	// Endpoint untouched by env.
	if prov["endpoint"] != config.SourceExplicitFile {
		t.Errorf("endpoint source = %q; want explicit-file", prov["endpoint"])
	}
}

func TestMerge_FlagBeatsEnv(t *testing.T) {
	t.Parallel()
	cfg, prov, err := config.Merge(
		[]config.FileLayer{{Partial: &config.PartialConfig{Endpoint: strPtr("http://x"), Model: strPtr("m")}, Source: config.SourceExplicitFile}},
		config.Overrides{Model: strPtr("m-env"), Source: config.SourceEnv},
		config.Overrides{Model: strPtr("m-flag"), Source: config.SourceFlag},
	)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if cfg.Model != "m-flag" {
		t.Errorf("model = %q; want m-flag", cfg.Model)
	}
	if prov["model"] != config.SourceFlag {
		t.Errorf("model source = %q; want flag", prov["model"])
	}
}

func TestMerge_DefaultsLayers(t *testing.T) {
	t.Parallel()
	partial := &config.PartialConfig{
		Endpoint: strPtr("http://x"),
		Model:    strPtr("m"),
		Defaults: &config.PartialDefaults{
			Temperature: fPtr(0.7),
			MaxTokens:   iPtr(8000),
			Stream:      bPtr(false),
			Output:      ofPtr(config.OutputJSON),
			Timeout:     dPtr(90 * time.Second),
		},
	}
	cfg, prov, err := config.Merge(
		[]config.FileLayer{{Partial: partial, Source: config.SourceExplicitFile}},
		config.Overrides{Temperature: fPtr(0.1), Source: config.SourceFlag},
		config.Overrides{},
	)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if cfg.Defaults.Temperature != 0.1 {
		t.Errorf("temperature = %g; want 0.1 (env layer wins)", cfg.Defaults.Temperature)
	}
	if prov["defaults.temperature"] != config.SourceFlag {
		t.Errorf("temperature source = %q; want flag", prov["defaults.temperature"])
	}
	if cfg.Defaults.MaxTokens != 8000 {
		t.Errorf("max_tokens = %d; want 8000 (file preserves)", cfg.Defaults.MaxTokens)
	}
	if prov["defaults.max_tokens"] != config.SourceExplicitFile {
		t.Errorf("max_tokens source = %q; want explicit-file", prov["defaults.max_tokens"])
	}
	if cfg.Defaults.Output != config.OutputJSON {
		t.Errorf("output = %q; want json", cfg.Defaults.Output)
	}
}
