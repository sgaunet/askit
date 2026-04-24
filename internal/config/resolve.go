package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LoadOptions controls [Load]'s source discovery.
type LoadOptions struct {
	// ExplicitPath is the value of -c / --config or ASKIT_CONFIG. Empty means
	// fall back to DefaultConfigPath().
	ExplicitPath string
	// ExplicitSource is the provenance label for ExplicitPath. Typically
	// [SourceFlag] when supplied by -c or [SourceEnv] when from ASKIT_CONFIG.
	ExplicitSource Source
	// EnvOverrides is the env-var override bag (already harvested by the CLI
	// globals layer). Pass Overrides{} to skip.
	EnvOverrides Overrides
	// FlagOverrides is the command-line flag override bag. Pass Overrides{}
	// to skip.
	FlagOverrides Overrides
}

// Result is the product of a successful Load.
type Result struct {
	Config          *Config
	Provenance      Provenance
	ResolvedPath    string // empty if no file contributed
	LoadedFromPath  bool
}

// DefaultConfigPath returns the platform default config file location.
//
// Resolution:
//  1. $XDG_CONFIG_HOME/askit/config.yml if $XDG_CONFIG_HOME is non-empty.
//  2. $HOME/.config/askit/config.yml on Unix (Linux, macOS, *BSD).
//  3. os.UserConfigDir()/askit/config.yml on Windows (i.e. %APPDATA%).
//
// This matches the documented contract (spec §Assumptions, quickstart,
// contracts/config-schema.md) and the convention followed by gh, uv,
// starship, mise, direnv, and other modern CLIs on macOS, where Go's
// os.UserConfigDir() would otherwise return ~/Library/Application Support.
func DefaultConfigPath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "askit", "config.yml"), nil
	}
	if runtime.GOOS != "windows" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".config", "askit", "config.yml"), nil
		}
		// fall through to os.UserConfigDir below as graceful degradation
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "askit", "config.yml"), nil
}

// Load resolves the final configuration from the full source chain:
// builtin → default-file → explicit-file → env → flag. Missing files are
// tolerated; returns validation errors aggregated into a single wrapped
// ValidationError.
func Load(opts LoadOptions) (*Result, error) {
	var files []FileLayer

	// Default file (only loaded if no explicit override; contracts §precedence).
	// We always try both when present: an explicit file takes precedence
	// via layer order, but the default file's values contribute whichever
	// fields the explicit file doesn't set. This matches the spec's
	// "precedence per field" reading.
	defaultPath, derr := DefaultConfigPath()
	if derr == nil {
		defaultLayer, err := loadOptionalFile(defaultPath, SourceDefaultFile)
		if err != nil {
			return nil, err
		}
		if defaultLayer != nil {
			files = append(files, *defaultLayer)
		}
	}

	// Explicit file: used only if ExplicitPath is set AND differs from
	// default; otherwise default-file already covered it. Explicit-file
	// errors (missing, malformed) are fatal because the user asked for
	// that file specifically.
	var resolvedPath string
	loaded := false
	if strings.TrimSpace(opts.ExplicitPath) != "" {
		resolvedPath = opts.ExplicitPath
		src := opts.ExplicitSource
		if src == "" {
			src = SourceExplicitFile
		}
		partial, err := LoadFile(opts.ExplicitPath)
		if err != nil {
			return nil, err
		}
		files = append(files, FileLayer{Partial: partial, Source: src})
		loaded = true
	} else if defaultPath != "" {
		// Loaded-from-path reflects the default file only if it existed.
		for _, f := range files {
			if f.Source == SourceDefaultFile && f.Partial != nil {
				resolvedPath = defaultPath
				loaded = true
				break
			}
		}
	}

	cfg, prov, err := Merge(files, opts.EnvOverrides, opts.FlagOverrides)
	if err != nil {
		return nil, fmt.Errorf("merge config: %w", err)
	}
	if verrs := Validate(cfg); len(verrs) > 0 {
		return nil, &ValidationError{Errors: verrs}
	}
	return &Result{
		Config:         cfg,
		Provenance:     prov,
		ResolvedPath:   resolvedPath,
		LoadedFromPath: loaded,
	}, nil
}

func loadOptionalFile(path string, src Source) (*FileLayer, error) {
	partial, err := LoadFile(path)
	if err != nil {
		if errors.Is(err, ErrConfigMissing) {
			return nil, nil //nolint:nilnil // "missing optional file" is intentional
		}
		return nil, err
	}
	return &FileLayer{Partial: partial, Source: src}, nil
}

// ValidationError aggregates every violation Validate reported.
type ValidationError struct {
	Errors []error
}

// Error renders all wrapped problems, one per line.
func (v *ValidationError) Error() string {
	if len(v.Errors) == 1 {
		return v.Errors[0].Error()
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d configuration errors:", len(v.Errors))
	for _, e := range v.Errors {
		sb.WriteString("\n  - ")
		sb.WriteString(e.Error())
	}
	return sb.String()
}

// Unwrap exposes the aggregated errors for errors.Is / errors.As.
func (v *ValidationError) Unwrap() []error { return v.Errors }
