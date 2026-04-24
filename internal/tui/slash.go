package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sgaunet/askit/internal/config"
)

// SlashResult is produced by [DispatchSlash] so the bubbletea model can
// react without the dispatcher itself touching the UI.
type SlashResult struct {
	Handled   bool   // true if the input was a slash command (don't send to model)
	Notice    string // optional inline transcript message (info/ok)
	Err       error  // inline transcript error; session stays alive
	Quit      bool   // true if /quit or /exit
	ClearView bool   // true after /clear (UI may want to redraw)
}

// IsSlash reports whether a submitted line is a slash command. A slash
// command starts with "/" AND contains no "@" token — matching FR-055.
// Returning false means the input is a regular user message.
func IsSlash(input string) bool {
	trimmed := strings.TrimLeft(input, " \t")
	if !strings.HasPrefix(trimmed, "/") {
		return false
	}
	if strings.ContainsRune(trimmed, '@') {
		return false
	}
	return true
}

// DispatchSlash parses and executes a slash command against sess.
// Presets is passed separately so /preset can introspect the available set
// without importing config into the model file.
//
// Unrecognized commands produce a SlashResult{Handled: true, Err: ...} so
// the TUI can render the error inline without re-routing to the model.
func DispatchSlash(input string, sess *Session, presets map[string]config.Preset) SlashResult {
	parts := strings.Fields(strings.TrimLeft(input, " \t"))
	if len(parts) == 0 {
		return SlashResult{Handled: true, Err: fmt.Errorf("empty slash command")}
	}
	cmd, args := parts[0], parts[1:]
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(input, cmd), " "))

	switch cmd {
	case "/help":
		return SlashResult{Handled: true, Notice: helpText()}

	case "/clear":
		sess.ClearHistory()
		return SlashResult{Handled: true, ClearView: true, Notice: "history cleared"}

	case "/system":
		if arg == "" {
			return SlashResult{Handled: true, Err: fmt.Errorf("/system requires the new system prompt")}
		}
		sess.SystemPrompt = arg
		return SlashResult{Handled: true, Notice: "system prompt updated"}

	case "/preset":
		if len(args) == 0 {
			return SlashResult{Handled: true, Err: fmt.Errorf("/preset NAME — available: %s", namesOf(presets))}
		}
		name := args[0]
		p, ok := presets[name]
		if !ok {
			return SlashResult{Handled: true, Err: fmt.Errorf("unknown preset %q; available: %s", name, namesOf(presets))}
		}
		sess.SystemPrompt = p.System
		sess.PresetName = name
		sess.ClearHistory()
		return SlashResult{Handled: true, ClearView: true, Notice: "preset " + name + " applied; history cleared"}

	case "/model":
		if len(args) == 0 {
			return SlashResult{Handled: true, Err: fmt.Errorf("/model NAME")}
		}
		sess.Model = args[0]
		return SlashResult{Handled: true, Notice: "model → " + args[0]}

	case "/save":
		if arg == "" {
			return SlashResult{Handled: true, Err: fmt.Errorf("/save FILE (extension: .json, .md, .txt)")}
		}
		if err := sess.SaveConversation(arg); err != nil {
			return SlashResult{Handled: true, Err: err}
		}
		return SlashResult{Handled: true, Notice: "conversation → " + arg}

	case "/save-last":
		if arg == "" {
			return SlashResult{Handled: true, Err: fmt.Errorf("/save-last FILE (extension: .json, .md, .txt)")}
		}
		if err := sess.SaveLastReply(arg); err != nil {
			return SlashResult{Handled: true, Err: err}
		}
		return SlashResult{Handled: true, Notice: "last reply → " + arg}

	case "/files":
		return SlashResult{Handled: true, Notice: formatFilesList(sess)}

	case "/cancel":
		// The dispatcher only signals; actual cancellation is done by the
		// bubbletea model via its cancel func.
		return SlashResult{Handled: true, Notice: "cancel signal"}

	case "/quit", "/exit":
		return SlashResult{Handled: true, Quit: true}
	}
	return SlashResult{Handled: true, Err: fmt.Errorf("unknown command: %s (try /help)", cmd)}
}

func helpText() string {
	return "slash commands: /help /clear /system TEXT /preset NAME /model NAME " +
		"/save FILE /save-last FILE /files /cancel /quit"
}

func namesOf(presets map[string]config.Preset) string {
	if len(presets) == 0 {
		return "(none defined)"
	}
	out := make([]string, 0, len(presets))
	for k := range presets {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func formatFilesList(sess *Session) string {
	if len(sess.References) == 0 {
		return "no @ references this session"
	}
	var sb strings.Builder
	for _, r := range sess.References {
		fmt.Fprintf(&sb, "%s [%s] %s\n", r.Kind, r.MediaType, r.Path)
	}
	return strings.TrimRight(sb.String(), "\n")
}
