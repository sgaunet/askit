package prompt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/config"
	"github.com/sgaunet/askit/internal/prompt"
)

func TestAssemble_TextOnly(t *testing.T) {
	t.Parallel()
	p, refs, err := prompt.Assemble("just some prose", prompt.AssembleOptions{
		Policy:       config.Builtins().FileReferences,
		SystemPrompt: "you are helpful",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("refs = %d; want 0", len(refs))
	}
	if p.System != "you are helpful" {
		t.Errorf("system = %q", p.System)
	}
	if len(p.Messages) != 1 {
		t.Fatalf("messages = %d; want 1", len(p.Messages))
	}
	if len(p.Messages[0].Content) != 1 || p.Messages[0].Content[0].Text != "just some prose" {
		t.Errorf("content = %+v", p.Messages[0].Content)
	}
}

func TestAssemble_SingleImage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "scan.png")
	tinyPNG(t, p, 20, 20)

	pr, refs, err := prompt.Assemble("ocr @"+p, prompt.AssembleOptions{
		Policy: config.Builtins().FileReferences,
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if len(refs) != 1 || refs[0].Kind != prompt.KindImage {
		t.Errorf("refs = %+v", refs)
	}
	var hasImage, hasText bool
	for _, c := range pr.Messages[0].Content {
		if c.Type == prompt.PartTypeImageURL {
			hasImage = true
			if !strings.HasPrefix(c.ImageURL.URL, "data:image/png;base64,") {
				t.Errorf("bad image URL: %q", c.ImageURL.URL)
			}
		}
		if c.Type == prompt.PartTypeText {
			hasText = true
		}
	}
	if !hasImage {
		t.Error("want image content part")
	}
	if !hasText {
		t.Error("want text content part for the 'ocr ' prose")
	}
}

func TestAssemble_TextFileInlined(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(p, []byte("# Hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	pr, _, err := prompt.Assemble("summarize @"+p, prompt.AssembleOptions{
		Policy: config.Builtins().FileReferences,
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	joined := ""
	for _, c := range pr.Messages[0].Content {
		if c.Type == prompt.PartTypeText {
			joined += c.Text
		}
	}
	if !strings.Contains(joined, "```notes.md") {
		t.Errorf("expected fenced notes.md block in: %q", joined)
	}
	if !strings.Contains(joined, "# Hi") {
		t.Errorf("expected file contents in: %q", joined)
	}
	if !strings.Contains(joined, "summarize ") {
		t.Errorf("original prose missing in: %q", joined)
	}
}

func TestAssemble_UnknownExtensionErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "blob.xyz")
	if err := os.WriteFile(p, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := prompt.Assemble("read @"+p, prompt.AssembleOptions{
		Policy: config.Builtins().FileReferences,
	})
	if err == nil {
		t.Fatal("want unknown-extension error under default strategy")
	}
	if !strings.Contains(err.Error(), "unknown extension") {
		t.Errorf("error message: %v", err)
	}
}

func TestAssemble_MissingFile(t *testing.T) {
	t.Parallel()
	_, _, err := prompt.Assemble("read @/does/not/exist.png", prompt.AssembleOptions{
		Policy: config.Builtins().FileReferences,
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestAssemble_ExtraFileFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "scan.png")
	tinyPNG(t, p, 10, 10)
	pr, refs, err := prompt.Assemble("describe", prompt.AssembleOptions{
		Policy:     config.Builtins().FileReferences,
		ExtraFiles: []string{p},
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("refs = %d; want 1", len(refs))
	}
	if len(pr.Messages[0].Content) < 2 {
		t.Errorf("want text+image parts, got %+v", pr.Messages[0].Content)
	}
}

func TestAssemble_EmptyPromptErrors(t *testing.T) {
	t.Parallel()
	_, _, err := prompt.Assemble("", prompt.AssembleOptions{
		Policy: config.Builtins().FileReferences,
	})
	if err == nil {
		t.Fatal("want error on empty prompt")
	}
}
