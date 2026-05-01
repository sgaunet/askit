package prompt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// Sentinel errors for text file loading.
var (
	errIsDirectory = errors.New("is a directory")
	errNotUTF8     = errors.New("not valid UTF-8")
)

// LoadTextRef reads a text reference, enforces the configured size limit,
// and returns the fenced-block form that will be inlined into the user
// message: three-backticks + basename + contents + three-backticks.
//
// Returns a sentinel [SizeError] when the file exceeds maxKB so callers can
// map to exit code 4 with the actionable hint.
func LoadTextRef(path string, maxKB int) (string, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("%s: %w", path, errIsDirectory)
	}
	const bytesPerKB = 1024
	sizeKB := info.Size() / bytesPerKB
	if int(sizeKB) > maxKB {
		return "", info.Size(), &SizeError{
			Path:     path,
			Got:      info.Size(),
			Limit:    int64(maxKB) * bytesPerKB,
			Kind:     "text",
			LimitKey: "max_text_size_kb",
		}
	}
	body, err := os.ReadFile(path) //nolint:gosec // path is user-supplied by design
	if err != nil {
		return "", info.Size(), fmt.Errorf("read %s: %w", path, err)
	}
	if !utf8.Valid(body) {
		return "", info.Size(), fmt.Errorf("%s: %w", path, errNotUTF8)
	}
	base := filepath.Base(path)
	return fmt.Sprintf("```%s\n%s\n```", base, body), info.Size(), nil
}

// SizeError is the typed error produced by [LoadTextRef] and LoadImageRef
// when a file exceeds its configured limit.
type SizeError struct {
	Path     string
	Got      int64
	Limit    int64
	Kind     string // "text" | "image"
	LimitKey string // "max_text_size_kb" | "max_image_size_mb"
}

// Error renders the user-facing size violation. The actionable hint lives
// on the [Hint] method so the CLI layer can include it under `-v`.
func (e *SizeError) Error() string {
	gotHuman := humanize(e.Got)
	limHuman := humanize(e.Limit)
	return fmt.Sprintf("%s: %s %s exceeds %s (%s)", e.Path, e.Kind, gotHuman, e.LimitKey, limHuman)
}

// Hint returns advice on how to fix the violation.
func (e *SizeError) Hint() string {
	if e.Kind == kindImageLabel {
		return "enable file_references.resize_images or raise file_references.max_image_size_mb"
	}
	return "raise file_references.max_text_size_kb or trim the input"
}

func humanize(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
