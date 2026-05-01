// Package config owns every read of user configuration: YAML files,
// environment variables, and command-line flags. Loading lives in exactly one
// place to keep precedence visible and unit-testable (constitution principle
// II: "no implicit globals").
//
// The public type is [Config]; use [Load] to resolve it from the full source
// chain, or the lower-level [LoadFile] / [Merge] primitives when tests need to
// exercise individual layers.
package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// OutputFormat is the renderer selection for `--output`.
type OutputFormat string

// Output format constants.
const (
	OutputPlain OutputFormat = "plain"
	OutputJSON  OutputFormat = "json"
	OutputRaw   OutputFormat = "raw"
)

// Valid reports whether the value is one of the three documented formats.
func (o OutputFormat) Valid() bool {
	switch o {
	case OutputPlain, OutputJSON, OutputRaw:
		return true
	}
	return false
}

// UnknownKind controls how the `@path` expander handles files whose extension
// appears in neither `image_extensions` nor `text_extensions`.
type UnknownKind string

// Unknown-extension strategies.
const (
	UnknownError UnknownKind = "error"
	UnknownSkip  UnknownKind = "skip"
	UnknownText  UnknownKind = "text"
	UnknownImage UnknownKind = "image"
)

// Valid reports whether the value is one of the four documented strategies.
func (u UnknownKind) Valid() bool {
	switch u {
	case UnknownError, UnknownSkip, UnknownText, UnknownImage:
		return true
	}
	return false
}

// Duration is a YAML-friendly wrapper around time.Duration that accepts
// strings like "5m", "60s", "2h30m".
type Duration time.Duration

// UnmarshalYAML accepts a scalar duration string.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("decode duration: %w", err)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalYAML emits the canonical duration string.
func (d *Duration) MarshalYAML() (any, error) {
	return time.Duration(*d).String(), nil
}

// AsDuration returns the underlying time.Duration.
func (d *Duration) AsDuration() time.Duration { return time.Duration(*d) }

// Config is the resolved configuration for a single invocation.
type Config struct {
	Endpoint       string            `yaml:"endpoint"`
	APIKey         string            `yaml:"api_key"`
	Model          string            `yaml:"model"`
	Defaults       Defaults          `yaml:"defaults"`
	FileReferences FileRefsPolicy    `yaml:"file_references"`
	Presets        map[string]Preset `yaml:"presets"`
}

// Defaults is the non-preset block of default sampling, streaming, and
// network parameters.
type Defaults struct {
	Temperature       float64      `yaml:"temperature"`
	TopP              float64      `yaml:"top_p"`
	MaxTokens         int          `yaml:"max_tokens"`
	Stream            bool         `yaml:"stream"`
	Output            OutputFormat `yaml:"output"`
	Timeout           Duration     `yaml:"timeout"`
	StreamIdleTimeout Duration     `yaml:"stream_idle_timeout"`
	Retries           int          `yaml:"retries"`
}

// FileRefsPolicy governs `@path` classification, sizing, and optional
// resizing.
type FileRefsPolicy struct {
	ImageExtensions []string     `yaml:"image_extensions"`
	TextExtensions  []string     `yaml:"text_extensions"`
	MaxImageSizeMB  int          `yaml:"max_image_size_mb"`
	MaxTextSizeKB   int          `yaml:"max_text_size_kb"`
	UnknownStrategy UnknownKind  `yaml:"unknown_strategy"`
	ResizeImages    ResizePolicy `yaml:"resize_images"`
}

// ResizePolicy captures the optional downscale-before-encode behavior.
type ResizePolicy struct {
	Enabled       bool `yaml:"enabled"`
	MaxLongEdgePx int  `yaml:"max_long_edge_px"`
	JPEGQuality   int  `yaml:"jpeg_quality"`
}

// Preset is a named bundle of overrides applied before explicit flags.
// Pointer fields distinguish "preset sets this" from "preset leaves this to
// Defaults".
type Preset struct {
	System      string        `yaml:"system"`
	Temperature *float64      `yaml:"temperature,omitempty"`
	TopP        *float64      `yaml:"top_p,omitempty"`
	MaxTokens   *int          `yaml:"max_tokens,omitempty"`
	Seed        *int          `yaml:"seed,omitempty"`
	Stream      *bool         `yaml:"stream,omitempty"`
	Output      *OutputFormat `yaml:"output,omitempty"`
	Model       *string       `yaml:"model,omitempty"`
}

// Source identifies where a field's resolved value came from, used by
// `askit config --explain`.
type Source string

// Canonical sources in ascending precedence order.
const (
	SourceBuiltin      Source = "builtin"
	SourceDefaultFile  Source = "default-file"
	SourceExplicitFile Source = "explicit-file"
	SourceEnv          Source = "env"
	SourceFlag         Source = "flag"
)

// Provenance maps a dotted field path (e.g. "defaults.temperature") to the
// source that supplied the current value.
type Provenance map[string]Source
