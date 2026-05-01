package render_test

import (
	"bytes"
	"testing"

	"github.com/sgaunet/askit/internal/render"
)

func TestRaw_EmitsVerbatim(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.RawRenderer{Out: &buf}
	body := []byte(`{"id":"chatcmpl-123","object":"chat.completion"}`)
	_ = r.Stream("should-be-ignored")
	if err := r.Finalize(render.Meta{RawBody: body}); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	got := buf.String()
	if got != string(body)+"\n" {
		t.Errorf("got %q; want %q+newline", got, body)
	}
}

func TestRaw_EmptyBodyWritesNothing(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.RawRenderer{Out: &buf}
	if err := r.Finalize(render.Meta{}); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty, got %q", buf.String())
	}
}
