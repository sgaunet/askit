// Package prompt assembles OpenAI-compatible chat-completion messages from
// user prose and `@path` file references.
//
// The surface is small and functional: [Tokenize] splits a prompt string into
// text segments and file-ref tokens; [Classify] maps each ref to its Kind
// (image/text/unknown) under the active FileRefsPolicy; [Assemble] reads,
// encodes, and composes the final [Prompt] suitable for an OpenAI chat
// completion request.
package prompt

// Kind is the classification of a single file reference.
type Kind int

// Kind values.
const (
	KindUnknown Kind = iota
	KindImage
	KindText
)

// String returns a short lowercase label matching the spec's
// classification badges: "image", "text", "unknown".
func (k Kind) String() string {
	switch k {
	case KindImage:
		return "image"
	case KindText:
		return "text"
	default:
		return "unknown"
	}
}

// Token is one element produced by [Tokenize]: either a chunk of user prose
// or a single file reference. Tokens appear in original-source order.
type Token struct {
	Kind    TokenKind
	Text    string // populated for TokenText
	RefPath string // populated for TokenFileRef — the raw path as written, tilde-expanded
	// KindOverride is set from a trailing ":text" / ":image" suffix;
	// KindUnknown means "no override, use extension-based classification".
	KindOverride Kind
}

// TokenKind distinguishes text tokens from file-reference tokens.
type TokenKind int

// TokenKind values.
const (
	TokenText TokenKind = iota + 1
	TokenFileRef
)

// FileRef is an `@path` occurrence resolved against the filesystem and
// ready for content-part assembly.
type FileRef struct {
	Raw          string // original token source text (e.g. "@./scan.png")
	Path         string // resolved absolute path
	Kind         Kind   // final classification after applying override + policy
	KindExplicit bool   // true if classification came from a :kind override
	SizeBytes    int64
	MediaType    string
}

// PartType identifies the content-part variant in a [ContentPart].
type PartType string

// PartType values matching the OpenAI chat-completion schema.
const (
	PartTypeText     PartType = "text"
	PartTypeImageURL PartType = "image_url"
)

// Message is a single chat-completion message (role + content parts).
type Message struct {
	Role    string
	Content []ContentPart
}

// ContentPart is one multimodal content part of a [Message].
type ContentPart struct {
	Type     PartType
	Text     string    // for PartTypeText
	ImageURL *ImageURL // for PartTypeImageURL
}

// ImageURL is the image_url payload in a [ContentPart].
type ImageURL struct {
	URL    string
	Detail string // "auto" in v1
}

// Prompt is the assembled system + user messages ready to be serialized
// into a chat-completion request body.
type Prompt struct {
	System   string
	Messages []Message
}
