package cli

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger builds a stderr slog.Logger whose level maps from the stacked
// `-v` count: 0 → Error, 1 → Info, 2+ → Debug (FR-090).
func NewLogger(verbose int) *slog.Logger {
	return NewLoggerTo(os.Stderr, verbose)
}

// NewLoggerTo is the same as [NewLogger] but writes to an arbitrary
// [io.Writer] (primarily for tests).
func NewLoggerTo(w io.Writer, verbose int) *slog.Logger {
	h := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: levelFor(verbose),
	})
	return slog.New(h)
}

func levelFor(verbose int) slog.Level {
	switch {
	case verbose <= 0:
		return slog.LevelError
	case verbose == 1:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}
