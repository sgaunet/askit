package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sgaunet/askit/internal/config"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestLoadFile_MinimalValid(t *testing.T) {
	t.Parallel()
	p := writeTemp(t, `
endpoint: http://localhost:1234/v1
model: test-model
`)
	pc, err := config.LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if pc.Endpoint == nil || *pc.Endpoint != "http://localhost:1234/v1" {
		t.Errorf("endpoint not loaded: %+v", pc.Endpoint)
	}
	if pc.Model == nil || *pc.Model != "test-model" {
		t.Errorf("model not loaded: %+v", pc.Model)
	}
}

func TestLoadFile_Missing(t *testing.T) {
	t.Parallel()
	_, err := config.LoadFile(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if !errors.Is(err, config.ErrConfigMissing) {
		t.Errorf("want ErrConfigMissing, got %v", err)
	}
}

func TestLoadFile_MalformedYAML(t *testing.T) {
	t.Parallel()
	p := writeTemp(t, "endpoint: [unclosed\n")
	_, err := config.LoadFile(p)
	if err == nil {
		t.Fatal("want error on malformed YAML")
	}
}

func TestLoadFile_UnknownField(t *testing.T) {
	t.Parallel()
	p := writeTemp(t, `
endpoint: http://x
model: m
not_a_real_field: true
`)
	_, err := config.LoadFile(p)
	if err == nil {
		t.Fatal("want error on unknown field (KnownFields should be enforced)")
	}
}

func TestLoadFile_BadDuration(t *testing.T) {
	t.Parallel()
	p := writeTemp(t, `
endpoint: http://x
model: m
defaults:
  timeout: not-a-duration
`)
	_, err := config.LoadFile(p)
	if err == nil {
		t.Fatal("want error on bad duration")
	}
}

func TestLoadFile_AllFields(t *testing.T) {
	t.Parallel()
	p := writeTemp(t, `
endpoint: https://api.example.com/v1
api_key: secret
model: gpt-test
defaults:
  temperature: 0.3
  top_p: 0.9
  max_tokens: 2048
  stream: false
  output: json
  timeout: 30s
  stream_idle_timeout: 1m
  retries: 5
file_references:
  image_extensions: [png, jpg]
  text_extensions: [txt, md]
  max_image_size_mb: 10
  max_text_size_kb: 100
  unknown_strategy: skip
  resize_images:
    enabled: true
    max_long_edge_px: 1024
    jpeg_quality: 75
presets:
  ocr:
    system: "you are an ocr engine"
    temperature: 0.0
`)
	pc, err := config.LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if pc.Defaults == nil || pc.Defaults.Temperature == nil || *pc.Defaults.Temperature != 0.3 {
		t.Errorf("temperature not parsed: %+v", pc.Defaults)
	}
	if pc.FileReferences == nil || pc.FileReferences.ResizeImages == nil {
		t.Fatalf("resize block not parsed")
	}
	if pc.FileReferences.ResizeImages.Enabled == nil || !*pc.FileReferences.ResizeImages.Enabled {
		t.Errorf("resize.enabled not parsed")
	}
	if _, ok := pc.Presets["ocr"]; !ok {
		t.Errorf("preset 'ocr' not parsed: %+v", pc.Presets)
	}
}
