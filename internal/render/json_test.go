package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/prompt"
	"github.com/sgaunet/askit/internal/render"
)

func TestJSON_EnvelopeShape(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.JSONRenderer{Out: &buf}
	meta := render.Meta{
		AskitVersion: "0.1.0",
		Model:        "chandra-ocr-2",
		Endpoint:     "http://localhost:1234/v1",
		PresetName:   "ocr-plain",
		System:       "You are an OCR engine.",
		UserPrompt:   "extract text",
		Inputs: []prompt.FileRef{
			{Kind: prompt.KindImage, Path: "/home/u/scan.png", MediaType: "image/png", SizeBytes: 184523},
		},
		StartedAt:    time.Date(2026, 4, 23, 10, 15, 0, 0, time.UTC),
		CompletedAt:  time.Date(2026, 4, 23, 10, 15, 4, 0, time.UTC),
		Duration:     4 * time.Second,
		FinishReason: "stop",
		Usage:        client.Usage{PromptTokens: 1234, CompletionTokens: 567, TotalTokens: 1801},
		Text:         "…extracted text…",
	}
	if err := r.Finalize(meta); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	// Output must be one JSON object, newline-terminated.
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output should end with newline; got %q", out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	req := parsed["request"].(map[string]any)
	if req["model"] != "chandra-ocr-2" || req["endpoint"] != "http://localhost:1234/v1" {
		t.Errorf("request metadata wrong: %+v", req)
	}
	inputs := req["inputs"].([]any)
	if len(inputs) != 1 {
		t.Fatalf("inputs = %v", inputs)
	}
	input := inputs[0].(map[string]any)
	if input["type"] != "image" || input["media_type"] != "image/png" || input["bytes"].(float64) != 184523 {
		t.Errorf("input shape wrong: %+v", input)
	}
	resp := parsed["response"].(map[string]any)
	if resp["text"] != "…extracted text…" || resp["finish_reason"] != "stop" {
		t.Errorf("response wrong: %+v", resp)
	}
	usage := resp["usage"].(map[string]any)
	if usage["total_tokens"].(float64) != 1801 {
		t.Errorf("usage wrong: %+v", usage)
	}
	// Timestamps RFC3339 UTC.
	if !strings.HasSuffix(req["started_at"].(string), "Z") {
		t.Errorf("started_at not UTC: %q", req["started_at"])
	}
}

func TestJSON_UsesStreamedBufferWhenTextAbsent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.JSONRenderer{Out: &buf}
	_ = r.Stream("Hello ")
	_ = r.Stream("world")
	_ = r.Finalize(render.Meta{AskitVersion: "v"})
	var parsed map[string]any
	_ = json.Unmarshal(buf.Bytes(), &parsed)
	resp := parsed["response"].(map[string]any)
	if resp["text"] != "Hello world" {
		t.Errorf("streamed text not buffered: %v", resp["text"])
	}
}

func TestJSON_NeverLeaksAPIKey(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.JSONRenderer{Out: &buf}
	_ = r.Finalize(render.Meta{
		AskitVersion: "v",
		Endpoint:     "http://x/v1",
		Model:        "m",
		Text:         "ok",
	})
	if strings.Contains(buf.String(), "api_key") || strings.Contains(buf.String(), "Authorization") {
		t.Errorf("envelope leaked auth fields: %s", buf.String())
	}
}
