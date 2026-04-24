package cli_test

import (
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/cli"
	"github.com/sgaunet/askit/internal/config"
)

func baseCfg() *config.Config {
	c := config.Builtins()
	c.Endpoint = "http://x/v1"
	c.Model = "default-model"
	c.Defaults.Temperature = 0.2
	c.Presets = map[string]config.Preset{
		"ocr": {
			System:      "OCR system prompt",
			Temperature: pfloat(0.0),
			MaxTokens:   pint(8000),
		},
	}
	return c
}

func pfloat(f float64) *float64 { return &f }
func pint(i int) *int           { return &i }

func TestResolvePreset_NoPresetNoFlags(t *testing.T) {
	t.Parallel()
	r, err := cli.ResolvePreset(baseCfg(), "", cli.PresetFlags{})
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}
	if r.Temperature != 0.2 {
		t.Errorf("temperature = %g; want 0.2 (default)", r.Temperature)
	}
	if r.Model != "default-model" {
		t.Errorf("model = %q", r.Model)
	}
	if r.System != "" {
		t.Errorf("system = %q; want empty", r.System)
	}
}

func TestResolvePreset_AppliesPreset(t *testing.T) {
	t.Parallel()
	r, err := cli.ResolvePreset(baseCfg(), "ocr", cli.PresetFlags{})
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}
	if r.System != "OCR system prompt" {
		t.Errorf("system = %q", r.System)
	}
	if r.Temperature != 0.0 {
		t.Errorf("temperature = %g; want 0.0 (preset)", r.Temperature)
	}
	if r.MaxTokens != 8000 {
		t.Errorf("max_tokens = %d; want 8000 (preset)", r.MaxTokens)
	}
}

func TestResolvePreset_FlagBeatsPreset(t *testing.T) {
	t.Parallel()
	r, err := cli.ResolvePreset(baseCfg(), "ocr", cli.PresetFlags{
		Temperature: pfloat(0.5),
	})
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}
	if r.Temperature != 0.5 {
		t.Errorf("temperature = %g; want 0.5 (flag)", r.Temperature)
	}
	if r.MaxTokens != 8000 {
		t.Errorf("max_tokens = %d; want 8000 (preset unchanged)", r.MaxTokens)
	}
}

func TestResolvePreset_SystemReplaces(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()
	// No config-level system either — preset should be the only source.
	r, _ := cli.ResolvePreset(cfg, "ocr", cli.PresetFlags{})
	if r.System != "OCR system prompt" {
		t.Errorf("preset system should have replaced default, got %q", r.System)
	}
	// Now --system beats it.
	flagSys := "flag system"
	r2, _ := cli.ResolvePreset(cfg, "ocr", cli.PresetFlags{System: &flagSys})
	if r2.System != flagSys {
		t.Errorf("flag system should beat preset, got %q", r2.System)
	}
}

func TestResolvePreset_UnknownPresetListsNames(t *testing.T) {
	t.Parallel()
	cfg := baseCfg()
	cfg.Presets["md"] = config.Preset{System: "md"}
	cfg.Presets["describe"] = config.Preset{System: "describe"}
	_, err := cli.ResolvePreset(cfg, "nonexistent", cli.PresetFlags{})
	if err == nil {
		t.Fatal("want error")
	}
	if cli.CodeOf(err) != cli.ExitConfig {
		t.Errorf("exit = %d; want %d", cli.CodeOf(err), cli.ExitConfig)
	}
	// Error message should enumerate available presets in sorted order.
	msg := err.Error()
	for _, want := range []string{"describe", "md", "ocr"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing preset name %q in %q", want, msg)
		}
	}
}
