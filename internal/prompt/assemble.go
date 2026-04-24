package prompt

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/sgaunet/askit/internal/config"
)

// AssembleOptions controls [Assemble]'s behavior.
type AssembleOptions struct {
	Policy        config.FileRefsPolicy
	SystemPrompt  string
	ExtraFiles    []string    // paths from repeated --file flags
	Logger        *slog.Logger // optional; nil disables diagnostic logging
}

// Assemble parses the prompt, resolves every `@path` token, and returns the
// user-visible [Prompt] with multimodal content parts attached. It stops at
// the first hard failure (missing file, oversize under `error` strategy)
// and surfaces typed errors the CLI layer maps to exit code 4.
func Assemble(userPrompt string, opts AssembleOptions) (*Prompt, []FileRef, error) {
	tokens, err := Tokenize(userPrompt)
	if err != nil {
		return nil, nil, fmt.Errorf("tokenize: %w", err)
	}

	// Append --file attachments as implicit TokenFileRef entries at the end.
	for _, p := range opts.ExtraFiles {
		tokens = append(tokens, Token{Kind: TokenFileRef, RefPath: expandHome(p)})
	}

	var (
		parts   []ContentPart
		refs    []FileRef
		textBuf strings.Builder
	)

	flushText := func() {
		if textBuf.Len() > 0 {
			parts = append(parts, ContentPart{Type: PartTypeText, Text: textBuf.String()})
			textBuf.Reset()
		}
	}

	for _, tok := range tokens {
		if tok.Kind == TokenText {
			textBuf.WriteString(tok.Text)
			continue
		}
		// File reference.
		abs, err := resolvePath(tok.RefPath)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve %s: %w", tok.RefPath, err)
		}
		kind, explicit := Classify(Token{RefPath: abs, KindOverride: tok.KindOverride}, opts.Policy)
		switch kind {
		case KindImage:
			dataURL, media, sz, err := LoadImageRef(abs, opts.Policy)
			if err != nil {
				return nil, nil, err
			}
			flushText()
			parts = append(parts, ContentPart{
				Type: PartTypeImageURL,
				ImageURL: &ImageURL{URL: dataURL, Detail: "auto"},
			})
			refs = append(refs, FileRef{
				Raw:          tok.RefPath,
				Path:         abs,
				Kind:         kind,
				KindExplicit: explicit,
				SizeBytes:    sz,
				MediaType:    media,
			})
		case KindText:
			block, sz, err := LoadTextRef(abs, opts.Policy.MaxTextSizeKB)
			if err != nil {
				return nil, nil, err
			}
			textBuf.WriteString("\n")
			textBuf.WriteString(block)
			textBuf.WriteString("\n")
			refs = append(refs, FileRef{
				Raw:          tok.RefPath,
				Path:         abs,
				Kind:         kind,
				KindExplicit: explicit,
				SizeBytes:    sz,
				MediaType:    detectTextMedia(abs),
			})
		case KindUnknown:
			if err := handleUnknown(abs, opts, &refs); err != nil {
				return nil, nil, err
			}
		}
	}
	flushText()

	// Ensure non-empty user message: if all tokens were file refs and we
	// only have image parts, keep it that way — OpenAI accepts multimodal
	// user messages without a text part.
	msg := Message{Role: "user", Content: parts}
	if len(msg.Content) == 0 {
		return nil, nil, errors.New("empty prompt (no prose and no file references)")
	}

	return &Prompt{
		System:   opts.SystemPrompt,
		Messages: []Message{msg},
	}, refs, nil
}

func resolvePath(p string) (string, error) {
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("absolute path: %w", err)
		}
		p = abs
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s: %w", p, err)
	}
	return p, nil
}

func detectTextMedia(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "md":
		return "text/markdown"
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "application/yaml"
	case "toml":
		return "application/toml"
	case "html":
		return "text/html"
	case "xml":
		return "application/xml"
	case "csv":
		return "text/csv"
	case "tsv":
		return "text/tab-separated-values"
	}
	return "text/plain"
}

func handleUnknown(path string, opts AssembleOptions, refs *[]FileRef) error {
	switch opts.Policy.UnknownStrategy {
	case config.UnknownError:
		return fmt.Errorf("%s: unknown extension (adjust file_references.*_extensions or pass :kind suffix)", path)
	case config.UnknownSkip:
		if opts.Logger != nil {
			opts.Logger.Info("skipping unknown-extension file", "path", path)
		}
		*refs = append(*refs, FileRef{Raw: path, Path: path, Kind: KindUnknown})
		return nil
	}
	// UnknownText and UnknownImage are handled in Classify already by
	// returning KindText/KindImage, so reaching here is a logic error.
	return fmt.Errorf("%s: internal: unknown extension reached handleUnknown", path)
}
