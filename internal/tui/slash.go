package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sgaunet/askit/internal/config"
)

// Sentinel errors for slash command validation.
var (
	errEmptySlashCmd     = errors.New("empty slash command")
	errSystemNoArg       = errors.New("/system requires the new system prompt")
	errPresetNoArg       = errors.New("/preset NAME — use /preset <name>")
	errPresetUnknown     = errors.New("unknown preset")
	errModelNoArg        = errors.New("/model NAME")
	errSaveNoArg         = errors.New("/save FILE (extension: .json, .md, .txt)")
	errSaveLastNoArg     = errors.New("/save-last FILE (extension: .json, .md, .txt)")
	errUnknownSlashCmd   = errors.New("unknown command (try /help)")
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
//nolint:cyclop // One case per slash command; the switch is the canonical dispatcher and cannot be split further.
func DispatchSlash(input string, sess *Session, presets map[string]config.Preset) SlashResult {
	parts := strings.Fields(strings.TrimLeft(input, " \t"))
	if len(parts) == 0 {
		return SlashResult{Handled: true, Err: errEmptySlashCmd}
	}
	cmd, args := parts[0], parts[1:]
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(input, cmd), " "))

	switch cmd {
	case "/help":
		return SlashResult{Handled: true, Notice: helpText()}
	case "/clear":
		return dispatchClear(sess)
	case "/system":
		return dispatchSystem(sess, arg)
	case "/preset":
		return dispatchPreset(sess, args, presets)
	case "/model":
		return dispatchModel(sess, args)
	case "/save":
		return dispatchSave(sess, arg)
	case "/save-last":
		return dispatchSaveLast(sess, arg)
	case "/files":
		return SlashResult{Handled: true, Notice: formatFilesList(sess)}
	case "/cancel":
		return SlashResult{Handled: true, Notice: "cancel signal"}
	case "/quit", "/exit":
		return SlashResult{Handled: true, Quit: true}
	}
	return SlashResult{Handled: true, Err: fmt.Errorf("%s: %w", cmd, errUnknownSlashCmd)}
}

func dispatchClear(sess *Session) SlashResult {
	sess.ClearHistory()
	return SlashResult{Handled: true, ClearView: true, Notice: "history cleared"}
}

func dispatchSystem(sess *Session, arg string) SlashResult {
	if arg == "" {
		return SlashResult{Handled: true, Err: errSystemNoArg}
	}
	sess.SystemPrompt = arg
	return SlashResult{Handled: true, Notice: "system prompt updated"}
}

func dispatchPreset(sess *Session, args []string, presets map[string]config.Preset) SlashResult {
	if len(args) == 0 {
		return SlashResult{Handled: true, Err: fmt.Errorf("%w; available: %s", errPresetNoArg, namesOf(presets))}
	}
	name := args[0]
	p, ok := presets[name]
	if !ok {
		return SlashResult{Handled: true, Err: fmt.Errorf("%w %q; available: %s", errPresetUnknown, name, namesOf(presets))}
	}
	sess.SystemPrompt = p.System
	sess.PresetName = name
	sess.ClearHistory()
	return SlashResult{Handled: true, ClearView: true, Notice: "preset " + name + " applied; history cleared"}
}

func dispatchModel(sess *Session, args []string) SlashResult {
	if len(args) == 0 {
		return SlashResult{Handled: true, Err: errModelNoArg}
	}
	sess.Model = args[0]
	return SlashResult{Handled: true, Notice: "model → " + args[0]}
}

func dispatchSave(sess *Session, arg string) SlashResult {
	if arg == "" {
		return SlashResult{Handled: true, Err: errSaveNoArg}
	}
	if err := sess.SaveConversation(arg); err != nil {
		return SlashResult{Handled: true, Err: err}
	}
	return SlashResult{Handled: true, Notice: "conversation → " + arg}
}

func dispatchSaveLast(sess *Session, arg string) SlashResult {
	if arg == "" {
		return SlashResult{Handled: true, Err: errSaveLastNoArg}
	}
	if err := sess.SaveLastReply(arg); err != nil {
		return SlashResult{Handled: true, Err: err}
	}
	return SlashResult{Handled: true, Notice: "last reply → " + arg}
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
