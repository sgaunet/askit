package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/prompt"
)

// JSONRenderer buffers streamed deltas and emits a single structured
// envelope on Finalize per contracts/output-json.md.
type JSONRenderer struct {
	Out io.Writer
	buf strings.Builder
}

// Stream buffers the delta; JSONRenderer never writes to Out until
// Finalize (FR-042).
func (j *JSONRenderer) Stream(delta string) error {
	j.buf.WriteString(delta)
	return nil
}

// Finalize writes the JSON envelope defined in
// contracts/output-json.md. Callers MUST populate [Meta.Text] (for the
// non-streaming path) or rely on the buffered delta (for the streaming
// path).
func (j *JSONRenderer) Finalize(meta Meta) error {
	text := meta.Text
	if text == "" {
		text = j.buf.String()
	}

	env := envelope{
		AskitVersion: meta.AskitVersion,
		Request: requestRecord{
			Model:     meta.Model,
			Endpoint:  meta.Endpoint,
			Preset:    meta.PresetName,
			System:    meta.System,
			Prompt:    meta.UserPrompt,
			Inputs:    convertInputs(meta.Inputs),
			StartedAt: meta.StartedAt.UTC().Format(time.RFC3339),
		},
		Response: responseRecord{
			Text:         text,
			FinishReason: meta.FinishReason,
			Usage:        meta.Usage,
			DurationMs:   meta.Duration.Milliseconds(),
			CompletedAt:  meta.CompletedAt.UTC().Format(time.RFC3339),
		},
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("json render: marshal: %w", err)
	}
	if _, err := j.Out.Write(body); err != nil {
		return fmt.Errorf("json render: write: %w", err)
	}
	if _, err := j.Out.Write([]byte("\n")); err != nil {
		return fmt.Errorf("json render: write newline: %w", err)
	}
	return nil
}

type envelope struct {
	AskitVersion string         `json:"askit_version"`
	Request      requestRecord  `json:"request"`
	Response     responseRecord `json:"response"`
}

type requestRecord struct {
	Model     string      `json:"model"`
	Endpoint  string      `json:"endpoint"`
	Preset    string      `json:"preset,omitempty"`
	System    string      `json:"system,omitempty"`
	Prompt    string      `json:"prompt"`
	Inputs    []inputInfo `json:"inputs"`
	StartedAt string      `json:"started_at"`
}

type inputInfo struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Bytes     int64  `json:"bytes"`
}

type responseRecord struct {
	Text         string       `json:"text"`
	FinishReason string       `json:"finish_reason"`
	Usage        client.Usage `json:"usage"`
	DurationMs   int64        `json:"duration_ms"`
	CompletedAt  string       `json:"completed_at"`
}

func convertInputs(refs []prompt.FileRef) []inputInfo {
	out := make([]inputInfo, 0, len(refs))
	for _, r := range refs {
		t := "image"
		if r.Kind == prompt.KindText {
			t = "text"
		}
		out = append(out, inputInfo{
			Type:      t,
			Path:      r.Path,
			MediaType: r.MediaType,
			Bytes:     r.SizeBytes,
		})
	}
	return out
}
