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
type Model struct {
	Session     *Session
	Cfg         *config.Config
	Req         Requester
	Logger      *slog.Logger
	NoColor     bool

	input       textarea.Model
	transcript  viewport.Model
	spin        spinner.Model
	width       int
	height      int

	// Stream state
	ctx         context.Context
	cancel      context.CancelFunc
	quit        bool
	notice      string // transient status line
}

// New builds a chat model from an active session and requester.
func New(sess *Session, cfg *config.Config, req Requester, logger *slog.Logger, noColor bool) *Model {
	ta := textarea.New()
	ta.Placeholder = "type a message; Enter to send, Shift-Enter for newline, /help for commands"
	ta.CharLimit = 0
	ta.Focus()
	ta.SetHeight(3)

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
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputH := 3
		statusH := 1
		vpH := max(msg.Height-inputH-statusH-2, 3)
		m.transcript.Width = msg.Width
		m.transcript.Height = vpH
		m.input.SetWidth(msg.Width)
		m.redraw()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.Session.InFlight && m.cancel != nil {
				m.cancel()
				m.notice = "cancelling…"
				return m, nil
			}
			m.quit = true
			return m, tea.Quit
		case tea.KeyCtrlD:
			if strings.TrimSpace(m.input.Value()) == "" {
				m.quit = true
				return m, tea.Quit
			}
		case tea.KeyEnter:
			if msg.Alt {
				// Shift-Enter inserts newline in the textarea; bubbletea
				// reports Shift-Enter as Alt+Enter on some terminals, so
				// handle both via the default textarea.Update below.
				break
			}
			text := strings.TrimRight(m.input.Value(), "\n")
			if text == "" {
				return m, nil
			}
			m.input.Reset()
			return m, m.submit(text)
		}

	case streamReadyMsg:
		cmds = append(cmds, nextChunk(streamState(msg)))

	case streamChunkPlusMsg:
		if msg.chunk.Delta != "" {
			m.Session.AppendAssistantChunk(msg.chunk.Delta)
			m.redraw()
		}
		cmds = append(cmds, nextChunk(msg.state))

	case streamDoneMsg:
		m.Session.InFlight = false
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.notice = ""
		m.redraw()

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
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.Session.InFlight {
			sp, cmd := m.spin.Update(msg)
			m.spin = sp
			cmds = append(cmds, cmd)
		}
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
