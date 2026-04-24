package client

import (
	"encoding/json"

	"github.com/sgaunet/askit/internal/prompt"
)

// Wire types for the OpenAI /v1/chat/completions contract. Kept private so
// the [Client] is the only consumer.

type chatReq struct {
	Model       string       `json:"model"`
	Messages    []wireMsg    `json:"messages"`
	Temperature *float64     `json:"temperature,omitempty"`
	TopP        *float64     `json:"top_p,omitempty"`
	MaxTokens   *int         `json:"max_tokens,omitempty"`
	Seed        *int         `json:"seed,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
	StreamOpts  *streamOpts  `json:"stream_options,omitempty"`
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type wireMsg struct {
	Role    string      `json:"role"`
	Content jsonRaw     `json:"content"`
}

// jsonRaw is either a plain string (for system messages) or a structured
// multimodal array (for user messages). We hand-encode it below so the JSON
// shape matches the OpenAI contract.
type jsonRaw []byte

func (j jsonRaw) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

type wireContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *wireImgURL `json:"image_url,omitempty"`
}

type wireImgURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// buildRequestBody converts a [Request] into the JSON body expected by the
// endpoint. System prompt (if any) becomes the first message.
func buildRequestBody(r *Request) ([]byte, error) {
	body := chatReq{
		Model:  r.Model,
		Stream: r.Stream,
	}
	if r.Temperature != 0 {
		t := r.Temperature
		body.Temperature = &t
	}
	if r.TopP != 0 {
		t := r.TopP
		body.TopP = &t
	}
	if r.MaxTokens > 0 {
		m := r.MaxTokens
		body.MaxTokens = &m
	}
	if r.Seed != nil {
		s := *r.Seed
		body.Seed = &s
	}
	if r.Stream {
		body.StreamOpts = &streamOpts{IncludeUsage: true}
	}

	if r.Prompt.System != "" {
		sysJSON, err := json.Marshal(r.Prompt.System)
		if err != nil {
			return nil, encodeErr("system", err)
		}
		body.Messages = append(body.Messages, wireMsg{
			Role:    "system",
			Content: jsonRaw(sysJSON),
		})
	}
	for _, m := range r.Prompt.Messages {
		content, err := encodeContent(m.Content)
		if err != nil {
			return nil, err
		}
		body.Messages = append(body.Messages, wireMsg{
			Role:    m.Role,
			Content: content,
		})
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, encodeErr("request body", err)
	}
	return out, nil
}

func encodeContent(parts []prompt.ContentPart) (jsonRaw, error) {
	out := make([]wireContentPart, 0, len(parts))
	for _, p := range parts {
		wp := wireContentPart{Type: string(p.Type)}
		switch p.Type {
		case prompt.PartTypeText:
			wp.Text = p.Text
		case prompt.PartTypeImageURL:
			if p.ImageURL != nil {
				wp.ImageURL = &wireImgURL{URL: p.ImageURL.URL, Detail: p.ImageURL.Detail}
			}
		}
		out = append(out, wp)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, encodeErr("content parts", err)
	}
	return jsonRaw(raw), nil
}

func encodeErr(what string, err error) error {
	return &wireError{what: what, err: err}
}

type wireError struct {
	what string
	err  error
}

func (e *wireError) Error() string { return "encode " + e.what + ": " + e.err.Error() }
func (e *wireError) Unwrap() error { return e.err }
