package cli

import (
	"sort"
	"strings"

	"github.com/sgaunet/askit/internal/config"
)

// ResolvedPreset is the effective set of overrides after merging a named
// preset on top of config.Defaults and below explicit flags. All fields
// are concrete (no pointers) so the downstream request builder doesn't
// need to re-check layered nils.
type ResolvedPreset struct {
	Name         string
	System       string
	Temperature  float64
	TopP         float64
	MaxTokens    int
	Seed         *int
	Stream       bool
	Output       config.OutputFormat
	Model        string
}

// PresetFlags is the subset of explicit CLI flags relevant to preset
// resolution. Pointer fields distinguish "flag was set" from "default
// zero value"; nil fields fall through to the preset or config.
type PresetFlags struct {
	System      *string
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Seed        *int
	Stream      *bool
	Output      *config.OutputFormat
	Model       *string
}

// ResolvePreset composes the effective [ResolvedPreset] for a request by
// layering: config.Defaults → preset[name] → flags. Passing an empty name
// skips the preset layer entirely.
//
// When name is non-empty but not found in cfg.Presets, returns a typed
// ConfigError enumerating the available preset names (FR-033).
func ResolvePreset(cfg *config.Config, name string, flags PresetFlags) (ResolvedPreset, error) {
	out := ResolvedPreset{
		Name:        name,
		Temperature: cfg.Defaults.Temperature,
		TopP:        cfg.Defaults.TopP,
		MaxTokens:   cfg.Defaults.MaxTokens,
		Stream:      cfg.Defaults.Stream,
		Output:      cfg.Defaults.Output,
		Model:       cfg.Model,
	}

	if strings.TrimSpace(name) != "" {
		if err := applyPresetLayer(cfg, name, &out); err != nil {
			return ResolvedPreset{}, err
		}
	}
	applyFlagLayer(flags, &out)
	return out, nil
}

// applyPresetLayer merges the named preset into out. Returns an error when
// the preset name is not found in cfg.Presets.
func applyPresetLayer(cfg *config.Config, name string, out *ResolvedPreset) error {
	preset, ok := cfg.Presets[name]
	if !ok {
		return NewConfigErr(
			"unknown preset %q; available: %s",
			name, formatPresetNames(cfg.Presets),
		)
	}
	// Preset system replaces — never merges.
	out.System = preset.System
	if preset.Temperature != nil {
		out.Temperature = *preset.Temperature
	}
	if preset.TopP != nil {
		out.TopP = *preset.TopP
	}
	if preset.MaxTokens != nil {
		out.MaxTokens = *preset.MaxTokens
	}
	if preset.Seed != nil {
		s := *preset.Seed
		out.Seed = &s
	}
	if preset.Stream != nil {
		out.Stream = *preset.Stream
	}
	if preset.Output != nil {
		out.Output = *preset.Output
	}
	if preset.Model != nil {
		out.Model = *preset.Model
	}
	return nil
}

// applyFlagLayer overlays explicit CLI flags (highest precedence) onto out.
func applyFlagLayer(flags PresetFlags, out *ResolvedPreset) {
	if flags.System != nil {
		out.System = *flags.System
	}
	if flags.Temperature != nil {
		out.Temperature = *flags.Temperature
	}
	if flags.TopP != nil {
		out.TopP = *flags.TopP
	}
	if flags.MaxTokens != nil {
		out.MaxTokens = *flags.MaxTokens
	}
	if flags.Seed != nil {
		s := *flags.Seed
		out.Seed = &s
	}
	if flags.Stream != nil {
		out.Stream = *flags.Stream
	}
	if flags.Output != nil {
		out.Output = *flags.Output
	}
	if flags.Model != nil {
		out.Model = *flags.Model
	}
}

func formatPresetNames(presets map[string]config.Preset) string {
	if len(presets) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
