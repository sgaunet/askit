package render

import (
	"fmt"
	"io"
)

// RawRenderer prints the upstream response body verbatim on Finalize
// (FR-044). Stream deltas are ignored (the raw body comes from
// [Meta.RawBody]).
type RawRenderer struct {
	Out io.Writer
}

// Stream is a no-op for RawRenderer.
func (r *RawRenderer) Stream(_ string) error { return nil }

// Finalize writes [Meta.RawBody] to Out, with a trailing newline if
// missing.
func (r *RawRenderer) Finalize(meta Meta) error {
	body := meta.RawBody
	if len(body) == 0 {
		return nil
	}
	if _, err := r.Out.Write(body); err != nil {
		return fmt.Errorf("raw render: write: %w", err)
	}
	if body[len(body)-1] != '\n' {
		if _, err := r.Out.Write([]byte("\n")); err != nil {
			return fmt.Errorf("raw render: newline: %w", err)
		}
	}
	return nil
}
