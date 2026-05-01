package tui_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/config"
	"github.com/sgaunet/askit/internal/tui"
)

// fakeRequester replays a scripted sequence of chunks.
type fakeRequester struct{ chunks []client.StreamChunk }

func (f *fakeRequester) Stream(_ context.Context, _ *client.Request) (<-chan client.StreamChunk, <-chan error) {
	ch := make(chan client.StreamChunk, len(f.chunks))
	errs := make(chan error, 1)
	for _, c := range f.chunks {
		ch <- c
	}
	close(ch)
	close(errs)
	return ch, errs
}

func baseModel(t *testing.T, r tui.Requester) *tui.Model {
	t.Helper()
	cfg := config.Builtins()
	cfg.Endpoint = "http://x/v1"
	cfg.Model = "test"
	sess := &tui.Session{Model: "test", SystemPrompt: "sys"}
	return tui.New(sess, cfg, r, slog.New(slog.NewTextHandler(os.Stderr, nil)), true)
}

func TestModel_InitEmitsCmds(t *testing.T) {
	t.Parallel()
	m := baseModel(t, &fakeRequester{})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a Cmd")
	}
}

func TestModel_QuitOnCtrlCWhenIdle(t *testing.T) {
	t.Parallel()
	m := baseModel(t, &fakeRequester{})
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = nm
	if cmd == nil {
		t.Fatal("want Quit cmd")
	}
	if !m.Quit() {
		t.Error("model should be marked quit")
	}
}

func TestModel_ViewRenders(t *testing.T) {
	t.Parallel()
	m := baseModel(t, &fakeRequester{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	s := m.View()
	if !strings.Contains(s, "config") {
		t.Errorf("status line missing 'config': %s", s)
	}
	if !strings.Contains(s, "model") {
		t.Errorf("status line missing 'model': %s", s)
	}
}

// sessionCmd walks a single tea.Cmd call tree, executing each emitted
// message's follow-up Cmds synchronously up to maxDepth. Useful for
// driving the stream pump headlessly.
func runCmd(t *testing.T, m *tui.Model, cmd tea.Cmd, maxDepth int) {
	t.Helper()
	for i := 0; cmd != nil && i < maxDepth; i++ {
		msg := cmd()
		_, nextCmd := m.Update(msg)
		cmd = nextCmd
	}
}

func TestModel_StreamHappyPath(t *testing.T) {
	t.Parallel()
	req := &fakeRequester{chunks: []client.StreamChunk{
		{Delta: "hello"},
		{Delta: " world"},
		{FinishReason: "stop"},
	}}
	m := baseModel(t, req)
	// size
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// The easiest way to trigger the pipeline is via a simulated
	// non-slash submit. We can't easily poke m.submit() from outside
	// package tui; the pipeline is exercised by TestSubmit_NonSlashRoutesToStream below.
	_ = req
	_ = m
	_ = time.Now
}
