package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var presetNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Sentinel base errors — wrapped by formatters below so callers can use
// errors.Is / errors.As while validation messages remain human-readable.
var (
	errEndpointRequired    = errors.New("endpoint: required (set in config, --endpoint, or ASKIT_ENDPOINT)")
	errEndpointScheme      = errors.New("endpoint: scheme must be http or https")
	errEndpointMissingHost = errors.New("endpoint: missing host")
	errModelRequired       = errors.New("model: required (set in config, --model, or ASKIT_MODEL)")
	errTemperatureRange    = errors.New("defaults.temperature: must be in [0, 2]")
	errTopPRange           = errors.New("defaults.top_p: must be in [0, 1]")
	errMaxTokensPositive   = errors.New("defaults.max_tokens: must be > 0")
	errOutputInvalid       = errors.New("defaults.output: must be one of plain|json|raw")
	errTimeoutPositive     = errors.New("defaults.timeout: must be > 0")
	errStreamIdlePositive  = errors.New("defaults.stream_idle_timeout: must be > 0")
	errRetriesNonNeg       = errors.New("defaults.retries: must be >= 0")
	errMaxImageSizePos     = errors.New("file_references.max_image_size_mb: must be > 0")
	errMaxTextSizePos      = errors.New("file_references.max_text_size_kb: must be > 0")
	errUnknownStrategyInv  = errors.New("file_references.unknown_strategy: must be one of error|skip|text|image")
	errMaxLongEdgePos      = errors.New("file_references.resize_images.max_long_edge_px: must be > 0")
	errJPEGQualityRange    = errors.New("file_references.resize_images.jpeg_quality: must be in [1, 100]")
	errExtDuplicate        = errors.New("file_references: extension appears in multiple lists")
	errExtEmpty            = errors.New("file_references: empty extension entry")
	errExtNotLowercase     = errors.New("file_references: extension must be lowercase")
	errExtNonAlphanumeric  = errors.New("file_references: extension contains non-alphanumeric character")
	errPresetNameInvalid   = errors.New("presets: name must match [a-zA-Z0-9_-]+")
	errPresetSystemEmpty   = errors.New("presets: system required and non-empty")
	errPresetTempRange     = errors.New("presets: temperature must be in [0, 2]")
	errPresetTopPRange     = errors.New("presets: top_p must be in [0, 1]")
	errPresetMaxTokPos     = errors.New("presets: max_tokens must be > 0")
	errPresetOutputInvalid = errors.New("presets: output must be one of plain|json|raw")
)

// Validate returns every validation violation found in cfg, in a stable
// order. An empty slice means cfg is valid.
//
// Callers MUST NOT short-circuit on the first error (FR-013): the
// user-facing path wraps the slice in an aggregated message so one `askit`
// run surfaces every problem.
func Validate(cfg *Config) []error {
	errs := make([]error, 0, 6) //nolint:mnd // 6 = number of validation groups below
	errs = append(errs, validateEndpoint(cfg)...)
	errs = append(errs, validateModel(cfg)...)
	errs = append(errs, validateDefaults(cfg)...)
	errs = append(errs, validateExtensionLists(&cfg.FileReferences)...)
	errs = append(errs, validateFileRefSizes(cfg)...)
	errs = append(errs, validatePresets(cfg.Presets)...)
	return errs
}

func validateEndpoint(cfg *Config) []error {
	var errs []error
	if cfg.Endpoint == "" {
		errs = append(errs, errEndpointRequired)
		return errs
	}
	u, err := url.Parse(cfg.Endpoint)
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("endpoint: invalid URL %q: %w", cfg.Endpoint, err))
	case u.Scheme != "http" && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("%w (got %q)", errEndpointScheme, u.Scheme))
	case u.Host == "":
		errs = append(errs, fmt.Errorf("%w in %q", errEndpointMissingHost, cfg.Endpoint))
	}
	return errs
}

func validateModel(cfg *Config) []error {
	if cfg.Model == "" {
		return []error{errModelRequired}
	}
	return nil
}

func validateDefaults(cfg *Config) []error {
	var errs []error
	if cfg.Defaults.Temperature < 0 || cfg.Defaults.Temperature > maxTemperature {
		errs = append(errs, fmt.Errorf("%w (got %g)", errTemperatureRange, cfg.Defaults.Temperature))
	}
	if cfg.Defaults.TopP < 0 || cfg.Defaults.TopP > 1 {
		errs = append(errs, fmt.Errorf("%w (got %g)", errTopPRange, cfg.Defaults.TopP))
	}
	if cfg.Defaults.MaxTokens <= 0 {
		errs = append(errs, fmt.Errorf("%w (got %d)", errMaxTokensPositive, cfg.Defaults.MaxTokens))
	}
	if !cfg.Defaults.Output.Valid() {
		errs = append(errs, fmt.Errorf("%w (got %q)", errOutputInvalid, cfg.Defaults.Output))
	}
	if cfg.Defaults.Timeout.AsDuration() <= 0 {
		errs = append(errs, errTimeoutPositive)
	}
	if cfg.Defaults.StreamIdleTimeout.AsDuration() <= 0 {
		errs = append(errs, errStreamIdlePositive)
	}
	if cfg.Defaults.Retries < 0 {
		errs = append(errs, fmt.Errorf("%w (got %d)", errRetriesNonNeg, cfg.Defaults.Retries))
	}
	return errs
}

func validateFileRefSizes(cfg *Config) []error {
	var errs []error
	if cfg.FileReferences.MaxImageSizeMB <= 0 {
		errs = append(errs, fmt.Errorf("%w (got %d)", errMaxImageSizePos, cfg.FileReferences.MaxImageSizeMB))
	}
	if cfg.FileReferences.MaxTextSizeKB <= 0 {
		errs = append(errs, fmt.Errorf("%w (got %d)", errMaxTextSizePos, cfg.FileReferences.MaxTextSizeKB))
	}
	if !cfg.FileReferences.UnknownStrategy.Valid() {
		errs = append(errs, fmt.Errorf("%w (got %q)", errUnknownStrategyInv, cfg.FileReferences.UnknownStrategy))
	}
	// Resize params validated regardless of enabled (per revised data-model §1).
	if cfg.FileReferences.ResizeImages.MaxLongEdgePx <= 0 {
		errs = append(errs, fmt.Errorf("%w (got %d)", errMaxLongEdgePos, cfg.FileReferences.ResizeImages.MaxLongEdgePx))
	}
	q := cfg.FileReferences.ResizeImages.JPEGQuality
	if q < 1 || q > 100 {
		errs = append(errs, fmt.Errorf("%w (got %d)", errJPEGQualityRange, q))
	}
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
			errs = append(errs, fmt.Errorf("%w: %q appears in %s and image_extensions", errExtDuplicate, ext, prev))
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
			errs = append(errs, fmt.Errorf("%w: %q appears in %s and text_extensions", errExtDuplicate, ext, prev))
			continue
		}
		seen[ext] = "text_extensions"
	}
	return errs
}

func checkExtension(list, ext string) error {
	if ext == "" {
		return fmt.Errorf("%w in %s", errExtEmpty, list)
	}
	if strings.ToLower(ext) != ext {
		return fmt.Errorf("%w: extension %q in %s", errExtNotLowercase, ext, list)
	}
	for _, r := range ext {
		if !isAlnum(r) {
			return fmt.Errorf("%w: extension %q in %s", errExtNonAlphanumeric, ext, list)
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
	errs := make([]error, 0, len(presets))
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		errs = append(errs, validateOnePreset(name, presets[name])...)
	}
	return errs
}

func validateOnePreset(name string, p Preset) []error {
	errs := validatePresetName(name)
	errs = append(errs, validatePresetFields(name, p)...)
	return errs
}

func validatePresetName(name string) []error {
	if !presetNamePattern.MatchString(name) {
		return []error{fmt.Errorf("%w: presets.%s", errPresetNameInvalid, name)}
	}
	return nil
}

func validatePresetFields(name string, p Preset) []error {
	errs := validatePresetSystem(name, p)
	errs = append(errs, validatePresetSampling(name, p)...)
	return errs
}

func validatePresetSystem(name string, p Preset) []error {
	if strings.TrimSpace(p.System) == "" {
		return []error{fmt.Errorf("%w: presets.%s.system", errPresetSystemEmpty, name)}
	}
	return nil
}

// maxTemperature is the upper bound for temperature per the OpenAI API spec.
const maxTemperature = 2.0

func validatePresetSampling(name string, p Preset) []error {
	errs := validatePresetNumericParams(name, p)
	if p.Output != nil && !p.Output.Valid() {
		errs = append(errs, fmt.Errorf("%w: presets.%s.output (got %q)", errPresetOutputInvalid, name, *p.Output))
	}
	return errs
}

func validatePresetNumericParams(name string, p Preset) []error {
	var errs []error
	if p.Temperature != nil && (*p.Temperature < 0 || *p.Temperature > maxTemperature) {
		errs = append(errs, fmt.Errorf("%w: presets.%s.temperature (got %g)", errPresetTempRange, name, *p.Temperature))
	}
	if p.TopP != nil && (*p.TopP < 0 || *p.TopP > 1) {
		errs = append(errs, fmt.Errorf("%w: presets.%s.top_p (got %g)", errPresetTopPRange, name, *p.TopP))
	}
	if p.MaxTokens != nil && *p.MaxTokens <= 0 {
		errs = append(errs, fmt.Errorf("%w: presets.%s.max_tokens (got %d)", errPresetMaxTokPos, name, *p.MaxTokens))
	}
	return errs
}
