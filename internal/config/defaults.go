package config

import "time"

// Builtins returns the baseline configuration used when no value comes from a
// file, env var, or flag. Exactly matches the "Builtins" table in
// contracts/config-schema.md.
//
// Callers MUST treat the result as a fresh copy: modifying the returned
// Config does not affect future calls.
func Builtins() *Config {
	return &Config{
		Defaults: Defaults{
			Temperature:       0.2,
			TopP:              1.0,
			MaxTokens:         4096,
			Stream:            true,
			Output:            OutputPlain,
			Timeout:           Duration(60 * time.Second),
			StreamIdleTimeout: Duration(2 * time.Minute),
			Retries:           2,
		},
		FileReferences: FileRefsPolicy{
			ImageExtensions: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp"},
			TextExtensions: []string{
				"txt", "md", "json", "yaml", "yml", "toml", "csv", "tsv", "log",
				"go", "py", "rs", "sh", "js", "ts", "html", "xml", "sql", "ini", "conf",
			},
			MaxImageSizeMB:  20,
			MaxTextSizeKB:   500,
			UnknownStrategy: UnknownError,
			ResizeImages: ResizePolicy{
				Enabled:       false,
				MaxLongEdgePx: 2048,
				JPEGQuality:   85,
			},
		},
		Presets: map[string]Preset{},
	}
}
