package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/config"
)

// Requester abstracts the HTTP/SSE call for testability. The default
// implementation is [RealRequester] which wraps a *client.Client.
type Requester interface {
	Stream(ctx context.Context, req *client.Request) (<-chan client.StreamChunk, <-chan error)
}

// Model is the bubbletea Model driving the chat TUI.
//
//nolint:containedctx // ctx is request-scoped: each stream spawns its own ctx and the Model owns that lifecycle.
type Model struct {
	Session     *Session
	Cfg         *config.Config
	Req         Requester
	Logger      *slog.Logger
	NoColor     bool

	input      textarea.Model
	transcript viewport.Model
	spin       spinner.Model
	width      int
	height     int

	// Stream state
	ctx    context.Context
	cancel context.CancelFunc
	quit   bool
	notice string // transient status line
}

const (
	inputAreaHeight = 3 // lines for the textarea input component
	viewportGap     = 2 // lines consumed by the separator and status line
	updateCmdCap    = 2 // preallocated capacity for Update's cmds slice
)

// New builds a chat model from an active session and requester.
func New(sess *Session, cfg *config.Config, req Requester, logger *slog.Logger, noColor bool) *Model {
	ta := textarea.New()
	ta.Placeholder = "type a message; Enter to send, Shift-Enter for newline, /help for commands"
	ta.CharLimit = 0
	ta.Focus()
	ta.SetHeight(inputAreaHeight)

	vp := viewport.New(0, 0)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &Model{
		Session:    sess,
		Cfg:        cfg,
		Req:        req,
		Logger:     logger,
		NoColor:    noColor,
		input:      ta,
		transcript: vp,
		spin:       sp,
	}
}

// Init satisfies tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spin.Tick)
}

// Update processes one message (keystroke, stream chunk, etc).
//
//nolint:ireturn // tea.Model is mandated by the bubbletea interface; returning *Model is not permitted.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, updateCmdCap)

	if model, cmd, handled := m.handleMsg(msg, &cmds); handled {
		return model, cmd
	}

	ta, cmd := m.input.Update(msg)
	m.input = ta
	cmds = append(cmds, cmd)

	vp, cmd := m.transcript.Update(msg)
	m.transcript = vp
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the current frame.
func (m *Model) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.transcript.View(),
		"─",
		m.input.View(),
		m.statusLine(),
	)
}

// Quit reports whether the model has requested termination.
func (m *Model) Quit() bool { return m.quit }

// handleMsg dispatches typed messages that require early-return semantics.
// Returns (model, cmd, true) when the caller should return immediately;
// otherwise returns (nil, nil, false) and the caller continues.
//
//nolint:cyclop // Each case dispatches to a focused helper; the switch itself is the complexity.
func (m *Model) handleMsg(msg tea.Msg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd, bool) { //nolint:ireturn // tea.Model is mandated by the bubbletea interface.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.onWindowSize(msg)
	case tea.KeyMsg:
		if model, cmd, early := m.onKey(msg); early {
			return model, cmd, true
		}
	case streamReadyMsg:
		*cmds = append(*cmds, nextChunk(streamState(msg)))
	case streamChunkPlusMsg:
		m.handleChunk(msg, cmds)
	case streamDoneMsg:
		m.onStreamDone()
	case streamErrMsg:
		m.Session.InFlight = false
		m.notice = "error: " + string(msg)
		m.redraw()
	case streamCancelledMsg:
		m.Session.InFlight = false
		m.Session.MarkLastCancelled()
		m.notice = "cancelled"
		m.redraw()
	case slashResultMsg:
		m.handleSlashResult(SlashResult(msg))
		if SlashResult(msg).Quit {
			m.quit = true
			return m, tea.Quit, true
		}
	case spinner.TickMsg:
		m.handleSpinTick(msg, cmds)
	}
	return nil, nil, false
}

func (m *Model) handleChunk(msg streamChunkPlusMsg, cmds *[]tea.Cmd) {
	if msg.chunk.Delta != "" {
		m.Session.AppendAssistantChunk(msg.chunk.Delta)
		m.redraw()
	}
	*cmds = append(*cmds, nextChunk(msg.state))
}

func (m *Model) handleSpinTick(msg spinner.TickMsg, cmds *[]tea.Cmd) {
	if m.Session.InFlight {
		sp, cmd := m.spin.Update(msg)
		m.spin = sp
		*cmds = append(*cmds, cmd)
	}
}

func (m *Model) onWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	inputH := inputAreaHeight
	statusH := 1
	vpH := max(msg.Height-inputH-statusH-viewportGap, inputAreaHeight)
	m.transcript.Width = msg.Width
	m.transcript.Height = vpH
	m.input.SetWidth(msg.Width)
	m.redraw()
}

// onKey handles key events. Returns (model, cmd, true) for early exits.
//
//nolint:ireturn // tea.Model is mandated by the bubbletea interface.
func (m *Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type { //nolint:exhaustive // Only CtrlC, CtrlD, and Enter are handled; all others pass through to textarea/viewport.
	case tea.KeyCtrlC:
		if m.Session.InFlight && m.cancel != nil {
			m.cancel()
			m.notice = "cancelling…"
			return m, nil, true
		}
		m.quit = true
		return m, tea.Quit, true
	case tea.KeyCtrlD:
		if strings.TrimSpace(m.input.Value()) == "" {
			m.quit = true
			return m, tea.Quit, true
		}
	case tea.KeyEnter:
		return m.onEnter(msg)
	default:
		// All other key types are forwarded to the textarea and viewport below.
	}
	return nil, nil, false
}

func (m *Model) onEnter(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) { //nolint:ireturn // tea.Model is mandated by the bubbletea interface.
	if msg.Alt {
		// Shift-Enter inserts newline; bubbletea reports it as Alt+Enter.
		return nil, nil, false
	}
	text := strings.TrimRight(m.input.Value(), "\n")
	if text == "" {
		return m, nil, true
	}
	m.input.Reset()
	return m, m.submit(text), true
}

func (m *Model) onStreamDone() {
	m.Session.InFlight = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.notice = ""
	m.redraw()
}

func (m *Model) redraw() {
	var sb strings.Builder
	for _, e := range m.Session.History {
		switch e.Role {
		case RoleUser:
			fmt.Fprintf(&sb, "\n> you [%s]\n%s\n", e.Timestamp.Format("15:04"), e.Text)
		case RoleAssistant:
			tag := m.Cfg.Model
			if m.Session.Model != "" {
				tag = m.Session.Model
			}
			fmt.Fprintf(&sb, "\n> %s [%s]\n%s", tag, e.Timestamp.Format("15:04"), e.Text)
			if e.Cancelled {
				sb.WriteString("\n[cancelled]")
			}
			sb.WriteString("\n")
		case RoleSystem:
			// System messages are not displayed in the transcript.
		}
	}
	m.transcript.SetContent(strings.TrimLeft(sb.String(), "\n"))
	m.transcript.GotoBottom()
}

func (m *Model) statusLine() string {
	left := fmt.Sprintf("config %s | preset %s | model %s",
		shortPath(m.Session.ConfigFilePath),
		orDash(m.Session.PresetName),
		orDash(modelOrDefault(m.Session, m.Cfg)),
	)
	right := ""
	if m.Session.InFlight {
		right = m.spin.View() + " thinking"
	}
	if m.notice != "" {
		right = m.notice
	}
	return left + "  " + right
}

func modelOrDefault(s *Session, c *config.Config) string {
	if s.Model != "" {
		return s.Model
	}
	return c.Model
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func shortPath(p string) string {
	if p == "" {
		return "<builtins>"
	}
	return p
}

// handleSlashResult applies the UI-side effects of a slash command.
func (m *Model) handleSlashResult(r SlashResult) {
	if r.Err != nil {
		m.notice = "! " + r.Err.Error()
	} else if r.Notice != "" {
		m.notice = r.Notice
	}
	if r.ClearView {
		m.Session.ClearHistory()
	}
	m.redraw()
}
