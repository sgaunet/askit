package render

import (
	"fmt"
	"io"
	"strings"
)

// PlainRenderer streams text to an [io.Writer] and guarantees a trailing
// newline at Finalize time (FR-041).
type PlainRenderer struct {
	Out    io.Writer
	sawNL  bool
	wrote  bool
}

// Stream writes delta directly. Tracks whether the last emitted byte was a
// newline so Finalize can add one iff missing.
func (p *PlainRenderer) Stream(delta string) error {
	if delta == "" {
		return nil
	}
	if _, err := io.WriteString(p.Out, delta); err != nil {
		return fmt.Errorf("plain render: write: %w", err)
	}
	p.wrote = true
	p.sawNL = strings.HasSuffix(delta, "\n")
	return nil
}

// Finalize appends a trailing newline if the last chunk did not end with
// one. Metadata is ignored.
func (p *PlainRenderer) Finalize(_ Meta) error {
	if !p.wrote || p.sawNL {
		return nil
	}
	if _, err := io.WriteString(p.Out, "\n"); err != nil {
		return fmt.Errorf("plain render: finalize: %w", err)
	}
	return nil
}
