package cli

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/tui"
)

type chatFlags struct {
	preset      string
	system      string
	firstPrompt string
	timeout     time.Duration
	idleTimeout time.Duration
	retries     int
}

func newChatCommand(g *Globals) *cobra.Command {
	f := &chatFlags{}
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Open an interactive TUI chat session",
		Long: "Launches an interactive terminal session with a streaming " +
			"transcript, multi-line input, and slash commands. The `@`-triggered " +
			"file picker described in spec §8.2 is deferred to v0.2; @path " +
			"references typed in the input are resolved at submit time.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runChat(g, f)
		},
	}
	cmd.Flags().StringVarP(&f.preset, "preset", "p", "", "apply preset at session start")
	cmd.Flags().StringVarP(&f.system, "system", "s", "", "override system prompt for this session")
	cmd.Flags().StringVar(&f.firstPrompt, "first-prompt", "", "send this prompt automatically once the UI is up")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 0, "connect + time-to-first-byte deadline")
	cmd.Flags().DurationVar(&f.idleTimeout, "stream-idle-timeout", 0, "max silence between streamed chunks")
	cmd.Flags().IntVar(&f.retries, "retries", -1, "retry budget (0 disables)")
	return cmd
}

func runChat(g *Globals, f *chatFlags) error {
	if !StdinIsTTY() {
		return NewUsageErr("chat requires a TTY on stdin")
	}
	if !isStdoutTTY() {
		return NewUsageErr("chat requires a TTY on stdout")
	}

	res, err := loadConfig(g)
	if err != nil {
		return err
	}
	cfg := res.Config

	// Resolve preset to initialize session.
	resolved, err := ResolvePreset(cfg, f.preset, PresetFlags{})
	if err != nil {
		return err
	}
	systemPrompt := resolved.System
	if f.system != "" {
		systemPrompt = f.system
	}

	// Build Session + HTTP client.
	connect := cfg.Defaults.Timeout.AsDuration()
	if f.timeout > 0 {
		connect = f.timeout
	}
	idle := cfg.Defaults.StreamIdleTimeout.AsDuration()
	if f.idleTimeout > 0 {
		idle = f.idleTimeout
	}
	hc := client.New(cfg.Endpoint, cfg.APIKey, connect, idle)

	sess := &tui.Session{
		SystemPrompt:   systemPrompt,
		PresetName:     resolved.Name,
		Model:          resolved.Model,
		ConfigFilePath: res.ResolvedPath,
	}
	m := tui.New(sess, cfg, tui.RealRequester{Client: hc}, NewLogger(g.Verbose), g.NoColor)

	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("chat tui: %w", err)
	}
	return nil
}

func isStdoutTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
