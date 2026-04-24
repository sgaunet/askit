// Package cli wires cobra commands, config loading, env/flag overrides,
// and top-level error-to-exit-code mapping. Thin transport layer: business
// logic lives in internal/config, internal/prompt, internal/client, and
// internal/render.
package cli

import (
	"errors"
	"fmt"
)

// ExitCode is the process exit code taxonomy defined by contracts/cli-surface.md.
type ExitCode int

// Stable exit codes — part of the tool's public contract.
const (
	ExitOK      ExitCode = 0
	ExitGeneric ExitCode = 1
	ExitUsage   ExitCode = 2
	ExitConfig  ExitCode = 3
	ExitFile    ExitCode = 4
	ExitNetwork ExitCode = 5
	ExitAPI     ExitCode = 6
	ExitTimeout ExitCode = 7
)

// Category is the short tag ("config", "file", "endpoint", "api", …) that
// appears after "askit:" in stderr output (FR-060).
type Category string

// Canonical categories.
const (
	CatUsage    Category = "usage"
	CatConfig   Category = "config"
	CatFile     Category = "file"
	CatEndpoint Category = "endpoint"
	CatAPI      Category = "api"
	CatTimeout  Category = "timeout"
	CatGeneric  Category = ""
)

// Categorized is an error with an attached [ExitCode] and [Category] so the
// top-level mapper can render the correct stderr line and choose the
// correct exit code.
type Categorized struct {
	Cat  Category
	Code ExitCode
	Err  error
}

// Error renders the wrapped error's message; category / exit code are
// applied by the outermost renderer in errout.go.
func (c *Categorized) Error() string {
	if c == nil || c.Err == nil {
		return "<nil>"
	}
	return c.Err.Error()
}

// Unwrap exposes the wrapped error for errors.Is / errors.As.
func (c *Categorized) Unwrap() error { return c.Err }

// CodeOf walks an error chain and returns the first embedded ExitCode.
// Unrecognized errors map to ExitGeneric.
func CodeOf(err error) ExitCode {
	if err == nil {
		return ExitOK
	}
	var c *Categorized
	if errors.As(err, &c) {
		return c.Code
	}
	return ExitGeneric
}

// CategoryOf walks an error chain and returns the first embedded Category.
// Errors without a category map to CatGeneric.
func CategoryOf(err error) Category {
	if err == nil {
		return CatGeneric
	}
	var c *Categorized
	if errors.As(err, &c) {
		return c.Cat
	}
	return CatGeneric
}

// NewUsageErr wraps a usage violation (unknown flag, missing prompt, bad
// -o target, TTY required, …) as exit-2.
func NewUsageErr(format string, args ...any) error {
	return &Categorized{Cat: CatUsage, Code: ExitUsage, Err: fmt.Errorf(format, args...)}
}

// NewConfigErr wraps a configuration-level failure (malformed YAML, failed
// validation, unknown preset, unresolved required field) as exit-3.
func NewConfigErr(format string, args ...any) error {
	return &Categorized{Cat: CatConfig, Code: ExitConfig, Err: fmt.Errorf(format, args...)}
}

// NewFileErr wraps a file-reference failure (missing, oversize, unreadable
// under the active unknown-strategy) as exit-4.
func NewFileErr(format string, args ...any) error {
	return &Categorized{Cat: CatFile, Code: ExitFile, Err: fmt.Errorf(format, args...)}
}

// NewNetworkErr wraps an HTTP transport failure (connection refused, DNS,
// TLS, connection reset) as exit-5.
func NewNetworkErr(format string, args ...any) error {
	return &Categorized{Cat: CatEndpoint, Code: ExitNetwork, Err: fmt.Errorf(format, args...)}
}

// NewAPIErr wraps a non-2xx API response (after retry exhaustion) as exit-6.
func NewAPIErr(format string, args ...any) error {
	return &Categorized{Cat: CatAPI, Code: ExitAPI, Err: fmt.Errorf(format, args...)}
}

// NewTimeoutErr wraps a deadline or cancel event as exit-7.
func NewTimeoutErr(format string, args ...any) error {
	return &Categorized{Cat: CatTimeout, Code: ExitTimeout, Err: fmt.Errorf(format, args...)}
}

// WrapCategorized lets callers attach a Category/ExitCode to an already-typed
// error without double-wrapping when it's already a *Categorized.
func WrapCategorized(cat Category, code ExitCode, err error) error {
	if err == nil {
		return nil
	}
	var existing *Categorized
	if errors.As(err, &existing) {
		return err
	}
	return &Categorized{Cat: cat, Code: code, Err: err}
}
