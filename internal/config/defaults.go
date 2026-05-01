package config

import "time"

// Default values for built-in configuration — named to satisfy the mnd linter.
const (
	defaultTemperature        = 0.2
	defaultMaxTokens          = 4096
	defaultTimeoutSeconds     = 60
	defaultStreamIdleMinutes  = 2
	defaultRetries            = 2
	defaultMaxImageSizeMB     = 20
	defaultMaxTextSizeKB      = 500
	defaultMaxLongEdgePx      = 2048
	defaultJPEGQuality        = 85
)

// Builtins returns the baseline configuration used when no value comes from a
// file, env var, or flag. Exactly matches the "Builtins" table in
// contracts/config-schema.md.
//
// Callers MUST treat the result as a fresh copy: modifying the returned
// Config does not affect future calls.
func Builtins() *Config {
	return &Config{
		Defaults: Defaults{
			Temperature:       defaultTemperature,
			TopP:              1.0,
			MaxTokens:         defaultMaxTokens,
			Stream:            true,
			Output:            OutputPlain,
			Timeout:           Duration(defaultTimeoutSeconds * time.Second),
			StreamIdleTimeout: Duration(defaultStreamIdleMinutes * time.Minute),
			Retries:           defaultRetries,
		},
		FileReferences: FileRefsPolicy{
			ImageExtensions: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp"},
			TextExtensions: []string{
				"txt", "md", "json", "yaml", "yml", "toml", "csv", "tsv", "log",
				"go", "py", "rs", "sh", "js", "ts", "html", "xml", "sql", "ini", "conf",
			},
			MaxImageSizeMB:  defaultMaxImageSizeMB,
			MaxTextSizeKB:   defaultMaxTextSizeKB,
			UnknownStrategy: UnknownError,
			ResizeImages: ResizePolicy{
				Enabled:       false,
				MaxLongEdgePx: defaultMaxLongEdgePx,
				JPEGQuality:   defaultJPEGQuality,
			},
		},
		Presets: map[string]Preset{},
	}
}
