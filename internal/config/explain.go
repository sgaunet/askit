package config

import (
	"fmt"
	"sort"
	"strings"
)

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
	switch path {
	case "endpoint":
		return cfg.Endpoint, nil
	case "api_key":
		return cfg.APIKey, nil
	case "model":
		return cfg.Model, nil
	case "defaults.temperature":
		return formatFloat(cfg.Defaults.Temperature), nil
	case "defaults.top_p":
		return formatFloat(cfg.Defaults.TopP), nil
	case "defaults.max_tokens":
		return fmt.Sprintf("%d", cfg.Defaults.MaxTokens), nil
	case "defaults.stream":
		return fmt.Sprintf("%t", cfg.Defaults.Stream), nil
	case "defaults.output":
		return string(cfg.Defaults.Output), nil
	case "defaults.timeout":
		return cfg.Defaults.Timeout.AsDuration().String(), nil
	case "defaults.stream_idle_timeout":
		return cfg.Defaults.StreamIdleTimeout.AsDuration().String(), nil
	case "defaults.retries":
		return fmt.Sprintf("%d", cfg.Defaults.Retries), nil
	case "file_references.image_extensions":
		return "[" + strings.Join(cfg.FileReferences.ImageExtensions, ", ") + "]", nil
	case "file_references.text_extensions":
		return "[" + strings.Join(cfg.FileReferences.TextExtensions, ", ") + "]", nil
	case "file_references.max_image_size_mb":
		return fmt.Sprintf("%d", cfg.FileReferences.MaxImageSizeMB), nil
	case "file_references.max_text_size_kb":
		return fmt.Sprintf("%d", cfg.FileReferences.MaxTextSizeKB), nil
	case "file_references.unknown_strategy":
		return string(cfg.FileReferences.UnknownStrategy), nil
	case "file_references.resize_images.enabled":
		return fmt.Sprintf("%t", cfg.FileReferences.ResizeImages.Enabled), nil
	case "file_references.resize_images.max_long_edge_px":
		return fmt.Sprintf("%d", cfg.FileReferences.ResizeImages.MaxLongEdgePx), nil
	case "file_references.resize_images.jpeg_quality":
		return fmt.Sprintf("%d", cfg.FileReferences.ResizeImages.JPEGQuality), nil
	case "presets":
		names := make([]string, 0, len(cfg.Presets))
		for n := range cfg.Presets {
			names = append(names, n)
		}
		sort.Strings(names)
		return "[" + strings.Join(names, ", ") + "]", nil
	}
	return "", fmt.Errorf("unknown field path %q", path)
}

func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}
