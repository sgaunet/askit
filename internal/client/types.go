// Package client is the HTTP + SSE transport for OpenAI-compatible endpoints.
// Stdlib-only (no OpenAI SDK) to maximize compatibility across LM Studio,
// vLLM, llama.cpp server, Ollama, and OpenAI itself.
package client

import (
	"time"

	"github.com/sgaunet/askit/internal/prompt"
)

// Request is the input to [Client.Complete] / [Client.Stream].
type Request struct {
	Model       string         `json:"model"`
	Prompt      *prompt.Prompt `json:"prompt,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Seed        *int           `json:"seed,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

// Response is the buffered-completion result.
type Response struct {
	Text         string
	FinishReason string
	Usage        Usage
	Raw          []byte // upstream body verbatim for --output raw
	Duration     time.Duration
}

// Usage mirrors the OpenAI usage object.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk is one delta produced during a streaming completion.
type StreamChunk struct {
	Delta        string
	FinishReason string
	Usage        *Usage // set only on the final chunk when upstream reports it
}
