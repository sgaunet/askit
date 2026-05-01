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

// Sentinel errors for assembly failures.
var (
	errEmptyPrompt        = errors.New("empty prompt (no prose and no file references)")
	errUnknownExtension   = errors.New("unknown extension (adjust file_references.*_extensions or pass :kind suffix)")
	errUnknownHandleLogic = errors.New("internal: unknown extension reached handleUnknown")
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
		newParts, newRefs, err := processFileRef(tok, opts, &textBuf)
		if err != nil {
			return nil, nil, err
		}
		if len(newParts) > 0 {
			// Flush buffered text before appending non-text content parts
			// (e.g. images), preserving original ordering.
			flushText()
		}
		parts = append(parts, newParts...)
		refs = append(refs, newRefs...)
	}
	flushText()

	// Ensure non-empty user message: if all tokens were file refs and we
	// only have image parts, keep it that way — OpenAI accepts multimodal
	// user messages without a text part.
	msg := Message{Role: "user", Content: parts}
	if len(msg.Content) == 0 {
		return nil, nil, errEmptyPrompt
	}

	return &Prompt{
		System:   opts.SystemPrompt,
		Messages: []Message{msg},
	}, refs, nil
}

// processFileRef resolves and loads a single file-reference token, returning
// the content parts and FileRef records it produced. Text content is appended
// directly to textBuf (for inline text files) rather than returned as a part.
func processFileRef(tok Token, opts AssembleOptions, textBuf *strings.Builder) ([]ContentPart, []FileRef, error) {
	abs, err := resolvePath(tok.RefPath)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %s: %w", tok.RefPath, err)
	}
	kind, explicit := Classify(Token{RefPath: abs, KindOverride: tok.KindOverride}, opts.Policy)
	switch kind {
	case KindImage:
		return processImageRef(tok, abs, kind, explicit, opts)
	case KindText:
		return processTextRef(tok, abs, kind, explicit, opts, textBuf)
	case KindUnknown:
		var refs []FileRef
		if err := handleUnknown(abs, opts, &refs); err != nil {
			return nil, nil, err
		}
		return nil, refs, nil
	}
	return nil, nil, nil
}

func processImageRef(tok Token, abs string, kind Kind, explicit bool, opts AssembleOptions) ([]ContentPart, []FileRef, error) {
	dataURL, media, sz, err := LoadImageRef(abs, opts.Policy)
	if err != nil {
		return nil, nil, err
	}
	part := ContentPart{Type: PartTypeImageURL, ImageURL: &ImageURL{URL: dataURL, Detail: "auto"}}
	ref := FileRef{Raw: tok.RefPath, Path: abs, Kind: kind, KindExplicit: explicit, SizeBytes: sz, MediaType: media}
	return []ContentPart{part}, []FileRef{ref}, nil
}

func processTextRef(tok Token, abs string, kind Kind, explicit bool, opts AssembleOptions, textBuf *strings.Builder) ([]ContentPart, []FileRef, error) {
	block, sz, err := LoadTextRef(abs, opts.Policy.MaxTextSizeKB)
	if err != nil {
		return nil, nil, err
	}
	textBuf.WriteString("\n")
	textBuf.WriteString(block)
	textBuf.WriteString("\n")
	ref := FileRef{Raw: tok.RefPath, Path: abs, Kind: kind, KindExplicit: explicit, SizeBytes: sz, MediaType: detectTextMedia(abs)}
	return nil, []FileRef{ref}, nil
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
		return fmt.Errorf("%s: %w", path, errUnknownExtension)
	case config.UnknownSkip:
		if opts.Logger != nil {
			opts.Logger.Info("skipping unknown-extension file", "path", path)
		}
		*refs = append(*refs, FileRef{Raw: path, Path: path, Kind: KindUnknown})
		return nil
	case config.UnknownText, config.UnknownImage:
		// These are handled in Classify already by returning KindText/KindImage,
		// so reaching here is a logic error.
		return fmt.Errorf("%s: %w", path, errUnknownHandleLogic)
	}
	// UnknownText and UnknownImage are handled in Classify already by
	// returning KindText/KindImage, so reaching here is a logic error.
	return fmt.Errorf("%s: %w", path, errUnknownHandleLogic)
}
