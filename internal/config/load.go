package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// PartialConfig mirrors [Config] but uses pointers on every top-level block
// so we can tell "field was absent from YAML" from "field was set to zero"
// during a [Merge].
type PartialConfig struct {
	Endpoint       *string             `yaml:"endpoint"`
	APIKey         *string             `yaml:"api_key"`
	Model          *string             `yaml:"model"`
	Defaults       *PartialDefaults    `yaml:"defaults"`
	FileReferences *PartialFileRefs    `yaml:"file_references"`
	Presets        map[string]Preset   `yaml:"presets"`
}

// PartialDefaults is [Defaults] with absence-aware pointer fields.
type PartialDefaults struct {
	Temperature       *float64      `yaml:"temperature"`
	TopP              *float64      `yaml:"top_p"`
	MaxTokens         *int          `yaml:"max_tokens"`
	Stream            *bool         `yaml:"stream"`
	Output            *OutputFormat `yaml:"output"`
	Timeout           *Duration     `yaml:"timeout"`
	StreamIdleTimeout *Duration     `yaml:"stream_idle_timeout"`
	Retries           *int          `yaml:"retries"`
}

// PartialFileRefs is [FileRefsPolicy] with absence-aware pointer fields.
type PartialFileRefs struct {
	ImageExtensions []string            `yaml:"image_extensions"`
	TextExtensions  []string            `yaml:"text_extensions"`
	MaxImageSizeMB  *int                `yaml:"max_image_size_mb"`
	MaxTextSizeKB   *int                `yaml:"max_text_size_kb"`
	UnknownStrategy *UnknownKind        `yaml:"unknown_strategy"`
	ResizeImages    *PartialResize      `yaml:"resize_images"`
}

// PartialResize is [ResizePolicy] with absence-aware pointer fields.
type PartialResize struct {
	Enabled       *bool `yaml:"enabled"`
	MaxLongEdgePx *int  `yaml:"max_long_edge_px"`
	JPEGQuality   *int  `yaml:"jpeg_quality"`
}

// ErrConfigMissing is returned by [LoadFile] when the requested file does
// not exist. Callers can inspect with errors.Is.
var ErrConfigMissing = errors.New("config file missing")

// LoadFile reads and decodes a YAML config file. A missing file at path is
// reported as ErrConfigMissing so callers can distinguish absence from
// syntax errors.
func LoadFile(path string) (*PartialConfig, error) {
	f, err := os.Open(path) //nolint:gosec // path is user-provided by design
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", path, ErrConfigMissing)
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var p PartialConfig
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &p, nil
}
