package config_test

import (
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/config"
)

func validBase() *config.Config {
	c := config.Builtins()
	c.Endpoint = "http://localhost:1234/v1"
	c.Model = "m"
	return c
}

func TestValidate_Happy(t *testing.T) {
	t.Parallel()
	errs := config.Validate(validBase())
	if len(errs) != 0 {
		t.Errorf("want no errors, got %v", errs)
	}
}

func TestValidate_AggregatesAllErrors(t *testing.T) {
	t.Parallel()
	c := &config.Config{} // everything missing / zeroed
	errs := config.Validate(c)
	if len(errs) < 6 {
		t.Errorf("want at least 6 aggregated errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_Each(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		mutate    func(*config.Config)
		wantSubstr string
	}{
		{"empty endpoint", func(c *config.Config) { c.Endpoint = "" }, "endpoint: required"},
		{"bad scheme", func(c *config.Config) { c.Endpoint = "ftp://x/" }, "scheme must be"},
		{"no host", func(c *config.Config) { c.Endpoint = "http:///" }, "missing host"},
		{"empty model", func(c *config.Config) { c.Model = "" }, "model: required"},
		{"temperature too high", func(c *config.Config) { c.Defaults.Temperature = 3 }, "defaults.temperature"},
		{"top_p too high", func(c *config.Config) { c.Defaults.TopP = 2 }, "defaults.top_p"},
		{"zero max_tokens", func(c *config.Config) { c.Defaults.MaxTokens = 0 }, "defaults.max_tokens"},
		{"bad output", func(c *config.Config) { c.Defaults.Output = "xml" }, "defaults.output"},
		{"zero timeout", func(c *config.Config) { c.Defaults.Timeout = 0 }, "defaults.timeout"},
		{"zero stream idle", func(c *config.Config) { c.Defaults.StreamIdleTimeout = 0 }, "stream_idle_timeout"},
		{"negative retries", func(c *config.Config) { c.Defaults.Retries = -1 }, "defaults.retries"},
		{"zero max image", func(c *config.Config) { c.FileReferences.MaxImageSizeMB = 0 }, "max_image_size_mb"},
		{"zero max text", func(c *config.Config) { c.FileReferences.MaxTextSizeKB = 0 }, "max_text_size_kb"},
		{"bad unknown strategy", func(c *config.Config) { c.FileReferences.UnknownStrategy = "fallback" }, "unknown_strategy"},
		{"uppercase ext", func(c *config.Config) { c.FileReferences.ImageExtensions = []string{"PNG"} }, "must be lowercase"},
		{"duplicate ext across lists", func(c *config.Config) {
			c.FileReferences.TextExtensions = append(c.FileReferences.TextExtensions, "png")
		}, "image_extensions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := validBase()
			tt.mutate(c)
			errs := config.Validate(c)
			if len(errs) == 0 {
				t.Fatalf("want error matching %q, got none", tt.wantSubstr)
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), tt.wantSubstr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no error matched %q in %v", tt.wantSubstr, errs)
			}
		})
	}
}

func TestValidate_ResizeValidatedEvenWhenDisabled(t *testing.T) {
	t.Parallel()
	c := validBase()
	c.FileReferences.ResizeImages.Enabled = false
	c.FileReferences.ResizeImages.MaxLongEdgePx = -10
	c.FileReferences.ResizeImages.JPEGQuality = 0
	errs := config.Validate(c)
	var sawEdge, sawQuality bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "max_long_edge_px") {
			sawEdge = true
		}
		if strings.Contains(e.Error(), "jpeg_quality") {
			sawQuality = true
		}
	}
	if !sawEdge || !sawQuality {
		t.Errorf("want both resize-param errors even when disabled; got edge=%t quality=%t; errs=%v", sawEdge, sawQuality, errs)
	}
}

func TestValidate_PresetName(t *testing.T) {
	t.Parallel()
	c := validBase()
	c.Presets = map[string]config.Preset{
		"bad name!": {System: "x"},
	}
	errs := config.Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error on invalid preset name")
	}
}

func TestValidate_PresetEmptySystem(t *testing.T) {
	t.Parallel()
	c := validBase()
	c.Presets = map[string]config.Preset{
		"ocr": {System: "   "},
	}
	errs := config.Validate(c)
	var ok bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "presets.ocr.system") {
			ok = true
			break
		}
	}
	if !ok {
		t.Errorf("want presets.ocr.system error, got %v", errs)
	}
}
