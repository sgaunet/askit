package config

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var presetNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Validate returns every validation violation found in cfg, in a stable
// order. An empty slice means cfg is valid.
//
// Callers MUST NOT short-circuit on the first error (FR-013): the
// user-facing path wraps the slice in an aggregated message so one `askit`
// run surfaces every problem.
func Validate(cfg *Config) []error {
	var errs []error

	// Endpoint
	if cfg.Endpoint == "" {
		errs = append(errs, fmt.Errorf("endpoint: required (set in config, --endpoint, or ASKIT_ENDPOINT)"))
	} else {
		u, err := url.Parse(cfg.Endpoint)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("endpoint: invalid URL %q: %w", cfg.Endpoint, err))
		case u.Scheme != "http" && u.Scheme != "https":
			errs = append(errs, fmt.Errorf("endpoint: scheme must be http or https (got %q)", u.Scheme))
		case u.Host == "":
			errs = append(errs, fmt.Errorf("endpoint: missing host in %q", cfg.Endpoint))
		}
	}

	// Model
	if cfg.Model == "" {
		errs = append(errs, fmt.Errorf("model: required (set in config, --model, or ASKIT_MODEL)"))
	}

	// Defaults
	if cfg.Defaults.Temperature < 0 || cfg.Defaults.Temperature > 2 {
		errs = append(errs, fmt.Errorf("defaults.temperature: must be in [0, 2] (got %g)", cfg.Defaults.Temperature))
	}
	if cfg.Defaults.TopP < 0 || cfg.Defaults.TopP > 1 {
		errs = append(errs, fmt.Errorf("defaults.top_p: must be in [0, 1] (got %g)", cfg.Defaults.TopP))
	}
	if cfg.Defaults.MaxTokens <= 0 {
		errs = append(errs, fmt.Errorf("defaults.max_tokens: must be > 0 (got %d)", cfg.Defaults.MaxTokens))
	}
	if !cfg.Defaults.Output.Valid() {
		errs = append(errs, fmt.Errorf("defaults.output: must be one of plain|json|raw (got %q)", cfg.Defaults.Output))
	}
	if cfg.Defaults.Timeout.AsDuration() <= 0 {
		errs = append(errs, fmt.Errorf("defaults.timeout: must be > 0"))
	}
	if cfg.Defaults.StreamIdleTimeout.AsDuration() <= 0 {
		errs = append(errs, fmt.Errorf("defaults.stream_idle_timeout: must be > 0"))
	}
	if cfg.Defaults.Retries < 0 {
		errs = append(errs, fmt.Errorf("defaults.retries: must be >= 0 (got %d)", cfg.Defaults.Retries))
	}

	// File references
	errs = append(errs, validateExtensionLists(&cfg.FileReferences)...)
	if cfg.FileReferences.MaxImageSizeMB <= 0 {
		errs = append(errs, fmt.Errorf("file_references.max_image_size_mb: must be > 0 (got %d)", cfg.FileReferences.MaxImageSizeMB))
	}
	if cfg.FileReferences.MaxTextSizeKB <= 0 {
		errs = append(errs, fmt.Errorf("file_references.max_text_size_kb: must be > 0 (got %d)", cfg.FileReferences.MaxTextSizeKB))
	}
	if !cfg.FileReferences.UnknownStrategy.Valid() {
		errs = append(errs, fmt.Errorf("file_references.unknown_strategy: must be one of error|skip|text|image (got %q)", cfg.FileReferences.UnknownStrategy))
	}
	// Resize params validated regardless of enabled (per revised data-model §1).
	if cfg.FileReferences.ResizeImages.MaxLongEdgePx <= 0 {
		errs = append(errs, fmt.Errorf("file_references.resize_images.max_long_edge_px: must be > 0 (got %d)", cfg.FileReferences.ResizeImages.MaxLongEdgePx))
	}
	q := cfg.FileReferences.ResizeImages.JPEGQuality
	if q < 1 || q > 100 {
		errs = append(errs, fmt.Errorf("file_references.resize_images.jpeg_quality: must be in [1, 100] (got %d)", q))
	}

	// Presets
	errs = append(errs, validatePresets(cfg.Presets)...)

	return errs
}

func validateExtensionLists(p *FileRefsPolicy) []error {
	var errs []error
	seen := map[string]string{} // ext → list name
	for _, ext := range p.ImageExtensions {
		if err := checkExtension("image_extensions", ext); err != nil {
			errs = append(errs, err)
			continue
		}
		if prev, ok := seen[ext]; ok {
			errs = append(errs, fmt.Errorf("file_references: extension %q appears in %s and image_extensions", ext, prev))
			continue
		}
		seen[ext] = "image_extensions"
	}
	for _, ext := range p.TextExtensions {
		if err := checkExtension("text_extensions", ext); err != nil {
			errs = append(errs, err)
			continue
		}
		if prev, ok := seen[ext]; ok {
			errs = append(errs, fmt.Errorf("file_references: extension %q appears in %s and text_extensions", ext, prev))
			continue
		}
		seen[ext] = "text_extensions"
	}
	return errs
}

func checkExtension(list, ext string) error {
	if ext == "" {
		return fmt.Errorf("file_references.%s: empty extension entry", list)
	}
	if strings.ToLower(ext) != ext {
		return fmt.Errorf("file_references.%s: extension %q must be lowercase", list, ext)
	}
	for _, r := range ext {
		if !isAlnum(r) {
			return fmt.Errorf("file_references.%s: extension %q contains non-alphanumeric character", list, ext)
		}
	}
	return nil
}

func isAlnum(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	return false
}

func validatePresets(presets map[string]Preset) []error {
	var errs []error
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := presets[name]
		if !presetNamePattern.MatchString(name) {
			errs = append(errs, fmt.Errorf("presets.%s: name must match [a-zA-Z0-9_-]+", name))
		}
		if strings.TrimSpace(p.System) == "" {
			errs = append(errs, fmt.Errorf("presets.%s.system: required and non-empty", name))
		}
		if p.Temperature != nil && (*p.Temperature < 0 || *p.Temperature > 2) {
			errs = append(errs, fmt.Errorf("presets.%s.temperature: must be in [0, 2] (got %g)", name, *p.Temperature))
		}
		if p.TopP != nil && (*p.TopP < 0 || *p.TopP > 1) {
			errs = append(errs, fmt.Errorf("presets.%s.top_p: must be in [0, 1] (got %g)", name, *p.TopP))
		}
		if p.MaxTokens != nil && *p.MaxTokens <= 0 {
			errs = append(errs, fmt.Errorf("presets.%s.max_tokens: must be > 0 (got %d)", name, *p.MaxTokens))
		}
		if p.Output != nil && !p.Output.Valid() {
			errs = append(errs, fmt.Errorf("presets.%s.output: must be one of plain|json|raw (got %q)", name, *p.Output))
		}
	}
	return errs
}
