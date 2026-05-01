// Package render emits completion output in plain, json, or raw form.
// The renderer contract is intentionally minimal: Stream receives each
// chunk as it arrives (plain only; json/raw buffer), Finalize is called
// exactly once at the end with metadata needed to build the final
// envelope.
package render

import (
	"time"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/prompt"
)

// Renderer is implemented by output-format backends.
type Renderer interface {
	// Stream writes a delta produced by a streaming completion. Plain
	// renderers flush immediately; json/raw renderers buffer.
	Stream(delta string) error
	// Finalize emits the end-of-response artifact (trailing newline,
	// JSON envelope, or raw body). Called exactly once per request.
	Finalize(meta Meta) error
}

// Meta is the per-request metadata passed to [Renderer.Finalize].
type Meta struct {
	AskitVersion string
	Model        string
	Endpoint     string
	PresetName   string
	System       string
	UserPrompt   string
	Inputs       []prompt.FileRef
	StartedAt    time.Time
	CompletedAt  time.Time
	Duration     time.Duration
	FinishReason string
	Usage        client.Usage
	RawBody      []byte // upstream response body for RawRenderer
	Text         string // finalized assistant text (for JSON envelope)
}
