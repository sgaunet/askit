package prompt_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/prompt"
)

func TestLoadTextRef_Happy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(p, []byte("# Title\nBody.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, sz, err := prompt.LoadTextRef(p, 100)
	if err != nil {
		t.Fatalf("LoadTextRef: %v", err)
	}
	if !strings.HasPrefix(got, "```notes.md\n") {
		t.Errorf("missing fenced prefix, got %q", got)
	}
	if !strings.Contains(got, "# Title") {
		t.Errorf("content missing, got %q", got)
	}
	if !strings.HasSuffix(got, "```") {
		t.Errorf("missing closing fence, got %q", got)
	}
	if sz == 0 {
		t.Error("size should be non-zero")
	}
}

func TestLoadTextRef_Oversize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	big := strings.Repeat("x", 200*1024) // 200 KB
	if err := os.WriteFile(p, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := prompt.LoadTextRef(p, 100)
	if err == nil {
		t.Fatal("want oversize error")
	}
	var se *prompt.SizeError
	if !errors.As(err, &se) {
		t.Fatalf("want *SizeError, got %T", err)
	}
	if se.Kind != "text" {
		t.Errorf("kind = %q; want text", se.Kind)
	}
	if !strings.Contains(se.Hint(), "max_text_size_kb") {
		t.Errorf("hint missing config key: %q", se.Hint())
	}
}

func TestLoadTextRef_Missing(t *testing.T) {
	t.Parallel()
	_, _, err := prompt.LoadTextRef(filepath.Join(t.TempDir(), "nope.txt"), 100)
	if err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestLoadTextRef_NonUTF8(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "bin.txt")
	if err := os.WriteFile(p, []byte{0xff, 0xfe, 0xfd}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := prompt.LoadTextRef(p, 100)
	if err == nil {
		t.Fatal("want error for non-UTF-8")
	}
}
