package config

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// errUnknownFieldPath is the sentinel returned by fieldValueAsString when the
// field path is not recognised. Wrapped with %w so callers can use errors.Is.
var errUnknownFieldPath = errors.New("unknown field path")

// ExplainLine is one row in the `askit config --explain` table.
type ExplainLine struct {
	Field  string
	Value  string
	Source Source
}

// Explain returns one [ExplainLine] per tracked field, in stable display
// order, combining the resolved Config with its Provenance.
//
// Error return is reserved for future extensibility (e.g. renderer failures);
// the current implementation never returns a non-nil error.
func Explain(cfg *Config, prov Provenance) ([]ExplainLine, error) {
	out := make([]ExplainLine, 0, len(allFieldPaths()))
	for _, path := range allFieldPaths() {
		v, err := fieldValueAsString(cfg, path)
		if err != nil {
			return nil, err
		}
		out = append(out, ExplainLine{
			Field:  path,
			Value:  v,
			Source: prov[path],
		})
	}
	return out, nil
}

func fieldValueAsString(cfg *Config, path string) (string, error) {
	if v, ok := fieldValueScalar(cfg, path); ok {
		return v, nil
	}
	if v, ok := fieldValueComplex(cfg, path); ok {
		return v, nil
	}
	return "", fmt.Errorf("%w %q", errUnknownFieldPath, path)
}

// fieldValueScalar handles simple scalar fields to keep fieldValueAsString
// complexity below the cyclop threshold.
func fieldValueScalar(cfg *Config, path string) (string, bool) {
	if v, ok := fieldValueTopLevel(cfg, path); ok {
		return v, true
	}
	return fieldValueDefaults(cfg, path)
}

// fieldValueTopLevel covers the top-level (non-defaults, non-file_references) paths.
func fieldValueTopLevel(cfg *Config, path string) (string, bool) {
	switch path {
	case "endpoint":
		return cfg.Endpoint, true
	case "api_key":
		return cfg.APIKey, true
	case "model":
		return cfg.Model, true
	}
	return "", false
}

// fieldValueDefaults covers the defaults.* paths.
func fieldValueDefaults(cfg *Config, path string) (string, bool) {
	switch path {
	case "defaults.temperature":
		return formatFloat(cfg.Defaults.Temperature), true
	case "defaults.top_p":
		return formatFloat(cfg.Defaults.TopP), true
	case "defaults.max_tokens":
		return strconv.Itoa(cfg.Defaults.MaxTokens), true
	case "defaults.stream":
		return strconv.FormatBool(cfg.Defaults.Stream), true
	case "defaults.output":
		return string(cfg.Defaults.Output), true
	case "defaults.timeout":
		return cfg.Defaults.Timeout.AsDuration().String(), true
	case "defaults.stream_idle_timeout":
		return cfg.Defaults.StreamIdleTimeout.AsDuration().String(), true
	case "defaults.retries":
		return strconv.Itoa(cfg.Defaults.Retries), true
	}
	return "", false
}

// fieldValueComplex handles the remaining (non-scalar / multi-field) paths.
func fieldValueComplex(cfg *Config, path string) (string, bool) {
	if v, ok := fieldValueFileRefs(cfg, path); ok {
		return v, true
	}
	return fieldValuePresets(cfg, path)
}

// fieldValueFileRefs covers the file_references.* paths.
func fieldValueFileRefs(cfg *Config, path string) (string, bool) {
	switch path {
	case "file_references.image_extensions":
		return "[" + strings.Join(cfg.FileReferences.ImageExtensions, ", ") + "]", true
	case "file_references.text_extensions":
		return "[" + strings.Join(cfg.FileReferences.TextExtensions, ", ") + "]", true
	case "file_references.max_image_size_mb":
		return strconv.Itoa(cfg.FileReferences.MaxImageSizeMB), true
	case "file_references.max_text_size_kb":
		return strconv.Itoa(cfg.FileReferences.MaxTextSizeKB), true
	case "file_references.unknown_strategy":
		return string(cfg.FileReferences.UnknownStrategy), true
	case "file_references.resize_images.enabled":
		return strconv.FormatBool(cfg.FileReferences.ResizeImages.Enabled), true
	case "file_references.resize_images.max_long_edge_px":
		return strconv.Itoa(cfg.FileReferences.ResizeImages.MaxLongEdgePx), true
	case "file_references.resize_images.jpeg_quality":
		return strconv.Itoa(cfg.FileReferences.ResizeImages.JPEGQuality), true
	}
	return "", false
}

// fieldValuePresets covers the "presets" path.
func fieldValuePresets(cfg *Config, path string) (string, bool) {
	if path != "presets" {
		return "", false
	}
	names := make([]string, 0, len(cfg.Presets))
	for n := range cfg.Presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return "[" + strings.Join(names, ", ") + "]", true
}

func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}
