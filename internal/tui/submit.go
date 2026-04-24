package tui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/prompt"
)

// Messages used by the bubbletea program.
type (
	streamDoneMsg      struct{}
	streamErrMsg       string
	streamCancelledMsg struct{}
	slashResultMsg     SlashResult
)

// submit builds the command(s) for a freshly submitted user input:
//   - If the input is a slash command, dispatch it and emit a single
//     slashResultMsg.
//   - Otherwise, expand @path references, append the user turn to the
//     transcript, and kick off the stream goroutine.
func (m *Model) submit(text string) tea.Cmd {
	if IsSlash(text) {
		res := DispatchSlash(text, m.Session, m.Cfg.Presets)
		return func() tea.Msg { return slashResultMsg(res) }
	}

	// Non-slash: assemble prompt + fire stream.
	assembled, refs, err := prompt.Assemble(text, prompt.AssembleOptions{
		Policy:       m.Cfg.FileReferences,
		SystemPrompt: m.Session.SystemPrompt,
		Logger:       m.Logger,
	})
	if err != nil {
		return func() tea.Msg { return streamErrMsg(err.Error()) }
	}
	m.Session.RecordReferences(refs)
	m.Session.AppendUser(text)
	m.Session.StartAssistant()
	m.Session.InFlight = true

	// Build request.
	req := &client.Request{
		Model:       modelOrDefault(m.Session, m.Cfg),
		Prompt:      assembled,
		Temperature: m.Cfg.Defaults.Temperature,
		TopP:        m.Cfg.Defaults.TopP,
		MaxTokens:   m.Cfg.Defaults.MaxTokens,
		Stream:      true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	m.redraw()
	return kickStream(m.Req, ctx, req)
}

// Stream plumbing: bubbletea commands return a single tea.Msg and then
// terminate. To keep pumping chunks from a channel we emit a
// [streamReadyMsg] first, then in response the Update loop schedules a
// chain of [nextChunk] commands, each returning [streamChunkPlusMsg] that
// embeds the channel state so the next pull is queued.

type streamState struct {
	chunks <-chan client.StreamChunk
	errs   <-chan error
}

type streamReadyMsg streamState
type streamChunkPlusMsg struct {
	chunk client.StreamChunk
	state streamState
}

func kickStream(req Requester, ctx context.Context, clientReq *client.Request) tea.Cmd {
	return func() tea.Msg {
		chunks, errs := req.Stream(ctx, clientReq)
		return streamReadyMsg{chunks: chunks, errs: errs}
	}
}

func nextChunk(s streamState) tea.Cmd {
	return func() tea.Msg {
		select {
		case chunk, ok := <-s.chunks:
			if !ok {
				if err, open := <-s.errs; open && err != nil {
					if errors.Is(err, context.Canceled) {
						return streamCancelledMsg{}
					}
					return streamErrMsg(err.Error())
				}
				return streamDoneMsg{}
			}
			return streamChunkPlusMsg{chunk: chunk, state: s}
		case err := <-s.errs:
			if errors.Is(err, context.Canceled) {
				return streamCancelledMsg{}
			}
			if err != nil {
				return streamErrMsg(err.Error())
			}
			return streamDoneMsg{}
		}
	}
}

// RealRequester is the production Requester implementation backed by a
// *client.Client.
type RealRequester struct{ Client *client.Client }

// Stream delegates to Client.Stream.
func (r RealRequester) Stream(ctx context.Context, req *client.Request) (<-chan client.StreamChunk, <-chan error) {
	return r.Client.Stream(ctx, req)
}
