package config

// Overrides carries values supplied by environment variables and command-line
// flags. Unlike YAML files, env/flag overrides are sparse: only populated
// fields participate in the merge.
type Overrides struct {
	// Global
	Endpoint    *string
	APIKey      *string
	Model       *string
	// Query-level flags overriding defaults
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Seed        *int
	Stream      *bool
	Output      *OutputFormat
	Timeout     *Duration
	StreamIdle  *Duration
	Retries     *int
	// Source tag to attribute writes from this Overrides struct.
	// Typically SourceEnv or SourceFlag; callers merging env then flags
	// pass the Overrides twice with different sources.
	Source Source
}

// Merge composes the final Config by layering sources in ascending precedence:
// builtin → default-file → explicit-file → env → flag.
//
// files maps each [PartialConfig] to the [Source] it originated from and is
// applied in slice order. envOverrides and flagOverrides are applied last
// (env before flag, since flag wins).
//
// The returned Provenance records the source that supplied each resolved
// field, keyed by a dotted field path such as "defaults.temperature".
func Merge(
	files []FileLayer,
	envOverrides Overrides,
	flagOverrides Overrides,
) (*Config, Provenance, error) {
	cfg := Builtins()
	prov := Provenance{}
	markAllBuiltin(cfg, prov)

	for _, layer := range files {
		if layer.Partial == nil {
			continue
		}
		applyPartial(cfg, prov, layer.Partial, layer.Source)
	}
	applyOverrides(cfg, prov, envOverrides)
	applyOverrides(cfg, prov, flagOverrides)
	return cfg, prov, nil
}

// FileLayer pairs a decoded [PartialConfig] with its [Source] so that
// [Merge] can attribute each field correctly.
type FileLayer struct {
	Partial *PartialConfig
	Source  Source
}

// markAllBuiltin records every configurable field as sourced from the
// builtins. Later layers overwrite these entries.
func markAllBuiltin(_ *Config, prov Provenance) {
	for _, p := range allFieldPaths() {
		prov[p] = SourceBuiltin
	}
}

// allFieldPaths returns every dotted field path that [Merge] may report in
// the provenance map. Kept in sync with the field set exposed by the
// explain subcommand.
func allFieldPaths() []string {
	return []string{
		"endpoint",
		"api_key",
		"model",
		"defaults.temperature",
		"defaults.top_p",
		"defaults.max_tokens",
		"defaults.stream",
		"defaults.output",
		"defaults.timeout",
		"defaults.stream_idle_timeout",
		"defaults.retries",
		"file_references.image_extensions",
		"file_references.text_extensions",
		"file_references.max_image_size_mb",
		"file_references.max_text_size_kb",
		"file_references.unknown_strategy",
		"file_references.resize_images.enabled",
		"file_references.resize_images.max_long_edge_px",
		"file_references.resize_images.jpeg_quality",
		"presets",
	}
}

func applyPartial(cfg *Config, prov Provenance, p *PartialConfig, src Source) {
	if p.Endpoint != nil {
		cfg.Endpoint = *p.Endpoint
		prov["endpoint"] = src
	}
	if p.APIKey != nil {
		cfg.APIKey = *p.APIKey
		prov["api_key"] = src
	}
	if p.Model != nil {
		cfg.Model = *p.Model
		prov["model"] = src
	}
	if p.Defaults != nil {
		applyDefaults(cfg, prov, p.Defaults, src)
	}
	if p.FileReferences != nil {
		applyFileRefs(cfg, prov, p.FileReferences, src)
	}
	if p.Presets != nil {
		cfg.Presets = p.Presets
		prov["presets"] = src
	}
}

//nolint:dupl // applyDefaults and applyOverridesDefaults are structurally identical but differ in input types (PartialDefaults vs Overrides); unifying them would require reflection or an interface.
func applyDefaults(cfg *Config, prov Provenance, d *PartialDefaults, src Source) {
	if d.Temperature != nil {
		cfg.Defaults.Temperature = *d.Temperature
		prov["defaults.temperature"] = src
	}
	if d.TopP != nil {
		cfg.Defaults.TopP = *d.TopP
		prov["defaults.top_p"] = src
	}
	if d.MaxTokens != nil {
		cfg.Defaults.MaxTokens = *d.MaxTokens
		prov["defaults.max_tokens"] = src
	}
	if d.Stream != nil {
		cfg.Defaults.Stream = *d.Stream
		prov["defaults.stream"] = src
	}
	if d.Output != nil {
		cfg.Defaults.Output = *d.Output
		prov["defaults.output"] = src
	}
	if d.Timeout != nil {
		cfg.Defaults.Timeout = *d.Timeout
		prov["defaults.timeout"] = src
	}
	if d.StreamIdleTimeout != nil {
		cfg.Defaults.StreamIdleTimeout = *d.StreamIdleTimeout
		prov["defaults.stream_idle_timeout"] = src
	}
	if d.Retries != nil {
		cfg.Defaults.Retries = *d.Retries
		prov["defaults.retries"] = src
	}
}

func applyFileRefs(cfg *Config, prov Provenance, f *PartialFileRefs, src Source) {
	if f.ImageExtensions != nil {
		cfg.FileReferences.ImageExtensions = f.ImageExtensions
		prov["file_references.image_extensions"] = src
	}
	if f.TextExtensions != nil {
		cfg.FileReferences.TextExtensions = f.TextExtensions
		prov["file_references.text_extensions"] = src
	}
	if f.MaxImageSizeMB != nil {
		cfg.FileReferences.MaxImageSizeMB = *f.MaxImageSizeMB
		prov["file_references.max_image_size_mb"] = src
	}
	if f.MaxTextSizeKB != nil {
		cfg.FileReferences.MaxTextSizeKB = *f.MaxTextSizeKB
		prov["file_references.max_text_size_kb"] = src
	}
	if f.UnknownStrategy != nil {
		cfg.FileReferences.UnknownStrategy = *f.UnknownStrategy
		prov["file_references.unknown_strategy"] = src
	}
	if f.ResizeImages != nil {
		applyResize(cfg, prov, f.ResizeImages, src)
	}
}

func applyResize(cfg *Config, prov Provenance, r *PartialResize, src Source) {
	if r.Enabled != nil {
		cfg.FileReferences.ResizeImages.Enabled = *r.Enabled
		prov["file_references.resize_images.enabled"] = src
	}
	if r.MaxLongEdgePx != nil {
		cfg.FileReferences.ResizeImages.MaxLongEdgePx = *r.MaxLongEdgePx
		prov["file_references.resize_images.max_long_edge_px"] = src
	}
	if r.JPEGQuality != nil {
		cfg.FileReferences.ResizeImages.JPEGQuality = *r.JPEGQuality
		prov["file_references.resize_images.jpeg_quality"] = src
	}
}

func applyOverrides(cfg *Config, prov Provenance, ov Overrides) {
	if ov.Source == "" {
		return
	}
	src := ov.Source
	applyOverridesTopLevel(cfg, prov, ov, src)
	applyOverridesDefaults(cfg, prov, ov, src)
}

func applyOverridesTopLevel(cfg *Config, prov Provenance, ov Overrides, src Source) {
	if ov.Endpoint != nil {
		cfg.Endpoint = *ov.Endpoint
		prov["endpoint"] = src
	}
	if ov.APIKey != nil {
		cfg.APIKey = *ov.APIKey
		prov["api_key"] = src
	}
	if ov.Model != nil {
		cfg.Model = *ov.Model
		prov["model"] = src
	}
}

//nolint:dupl // See applyDefaults — same reasoning applies here.
func applyOverridesDefaults(cfg *Config, prov Provenance, ov Overrides, src Source) {
	if ov.Temperature != nil {
		cfg.Defaults.Temperature = *ov.Temperature
		prov["defaults.temperature"] = src
	}
	if ov.TopP != nil {
		cfg.Defaults.TopP = *ov.TopP
		prov["defaults.top_p"] = src
	}
	if ov.MaxTokens != nil {
		cfg.Defaults.MaxTokens = *ov.MaxTokens
		prov["defaults.max_tokens"] = src
	}
	if ov.Stream != nil {
		cfg.Defaults.Stream = *ov.Stream
		prov["defaults.stream"] = src
	}
	if ov.Output != nil {
		cfg.Defaults.Output = *ov.Output
		prov["defaults.output"] = src
	}
	if ov.Timeout != nil {
		cfg.Defaults.Timeout = *ov.Timeout
		prov["defaults.timeout"] = src
	}
	if ov.StreamIdle != nil {
		cfg.Defaults.StreamIdleTimeout = *ov.StreamIdle
		prov["defaults.stream_idle_timeout"] = src
	}
	if ov.Retries != nil {
		cfg.Defaults.Retries = *ov.Retries
		prov["defaults.retries"] = src
	}
}
