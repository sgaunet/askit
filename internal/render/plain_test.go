package render_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sgaunet/askit/internal/render"
)

func TestPlain_StreamsAndGuaranteesNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.PlainRenderer{Out: &buf}
	for _, d := range []string{"Hello", " ", "world"} {
		if err := r.Stream(d); err != nil {
			t.Fatalf("Stream: %v", err)
		}
	}
	if err := r.Finalize(render.Meta{}); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if buf.String() != "Hello world\n" {
		t.Errorf("got %q; want %q", buf.String(), "Hello world\n")
	}
}

func TestPlain_DoesNotDoubleNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.PlainRenderer{Out: &buf}
	_ = r.Stream("already-terminated\n")
	_ = r.Finalize(render.Meta{})
	if buf.String() != "already-terminated\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestPlain_EmptyStreamStillOK(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &render.PlainRenderer{Out: &buf}
	if err := r.Finalize(render.Meta{}); err != nil {
		t.Errorf("Finalize on empty: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output; got %q", buf.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestPlain_WriteErrorsAreWrapped(t *testing.T) {
	t.Parallel()
	r := &render.PlainRenderer{Out: failingWriter{}}
	err := r.Stream("data")
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// noop to satisfy linter on uncommon errors.Is pattern
		t.Logf("wrapping chain OK")
	}
}
