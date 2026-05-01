// Package tui owns the interactive `askit chat` experience. The bubbletea
// Model drives a scrollable transcript, a multi-line input area, and a
// slash-command dispatcher that never reaches the model. @path file
// references in input are expanded at submit time via the prompt package.
//
// NOTE (v0.1.x): The `@`-triggered overlay file picker described in
// spec §8.2 is DEFERRED to v0.2. In v0.1.x, @path references in typed
// input are resolved against the filesystem exactly like `askit query`.
// The slash-command, streaming, cancel, and save flows are complete.
package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sgaunet/askit/internal/client"
	"github.com/sgaunet/askit/internal/prompt"
)

// Sentinel errors for session save operations.
var (
	errUnsupportedExtension = errors.New("unsupported extension; use .json, .md, or .txt")
	errNoAssistantReply     = errors.New("no assistant reply to save")
)

// dirPerm is the permission mode for newly created directories.
// 0o750 is used (not 0o755) to avoid giving world-execute on directories
// containing potentially sensitive configuration or saved conversations.
const dirPerm = 0o750

// Role is the message role in the transcript.
type Role string

// Role values.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Entry is one message in the transcript. Streaming assistant replies
// accumulate into the most recent entry's Text field.
type Entry struct {
	Role      Role
	Text      string
	Timestamp time.Time
	Cancelled bool
}

// Session is the in-memory state of one interactive run. Never persisted.
type Session struct {
	SystemPrompt   string
	PresetName     string
	Model          string
	ConfigFilePath string
	History        []Entry
	References     []prompt.FileRef // all @path refs seen this session
	InFlight       bool
}

// AppendUser adds a user message to the transcript.
func (s *Session) AppendUser(text string) {
	s.History = append(s.History, Entry{
		Role:      RoleUser,
		Text:      text,
		Timestamp: time.Now(),
	})
}

// StartAssistant opens a new assistant entry ready to receive streamed
// chunks. Returns a pointer to allow in-place updates by callers.
func (s *Session) StartAssistant() *Entry {
	s.History = append(s.History, Entry{
		Role:      RoleAssistant,
		Timestamp: time.Now(),
	})
	return &s.History[len(s.History)-1]
}

// AppendAssistantChunk adds a streamed delta to the most recent assistant
// entry (or creates one if none exists).
func (s *Session) AppendAssistantChunk(delta string) {
	if len(s.History) == 0 || s.History[len(s.History)-1].Role != RoleAssistant {
		s.StartAssistant()
	}
	s.History[len(s.History)-1].Text += delta
}

// MarkLastCancelled tags the most recent assistant entry as cancelled.
func (s *Session) MarkLastCancelled() {
	if len(s.History) > 0 && s.History[len(s.History)-1].Role == RoleAssistant {
		s.History[len(s.History)-1].Cancelled = true
	}
}

// LastAssistantText returns the text of the most recent assistant message,
// or the empty string if none.
func (s *Session) LastAssistantText() string {
	for i := len(s.History) - 1; i >= 0; i-- {
		if s.History[i].Role == RoleAssistant {
			return s.History[i].Text
		}
	}
	return ""
}

// ClearHistory drops all messages; keeps the system prompt.
func (s *Session) ClearHistory() {
	s.History = nil
}

// RecordReferences merges newly-resolved file references into the
// session-wide list. Duplicates are skipped by resolved Path.
func (s *Session) RecordReferences(refs []prompt.FileRef) {
	seen := make(map[string]struct{}, len(s.References))
	for _, r := range s.References {
		seen[r.Path] = struct{}{}
	}
	for _, r := range refs {
		if _, ok := seen[r.Path]; ok {
			continue
		}
		s.References = append(s.References, r)
		seen[r.Path] = struct{}{}
	}
}

// SaveLastReply writes the most recent assistant reply to path. The
// format is chosen by extension per FR-056 (revised): `.json`, `.md`, or
// `.txt`. Other extensions are rejected.
func (s *Session) SaveLastReply(path string) error {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "json", "md", "txt":
		// ok
	default:
		return fmt.Errorf("extension %q: %w", ext, errUnsupportedExtension)
	}
	text := s.LastAssistantText()
	if text == "" {
		return errNoAssistantReply
	}
	if err := writeFileAtomic(path, []byte(text)); err != nil {
		return fmt.Errorf("save-last: %w", err)
	}
	return nil
}

// SaveConversation writes the full transcript to path. Format by
// extension: `.json` = array of entries; `.md` = formatted Markdown;
// `.txt` = plain role-prefixed lines.
func (s *Session) SaveConversation(path string) error {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "json":
		return saveJSON(path, s.History)
	case "md":
		return saveMarkdown(path, s.History)
	case "txt":
		return saveText(path, s.History)
	default:
		return fmt.Errorf("extension %q: %w", ext, errUnsupportedExtension)
	}
}

// writeFileAtomic uses the same sibling-tempfile + rename pattern as
// cli.AtomicWriter, duplicated here to avoid a transport-layer import.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".askit-save-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Ensure `client` stays imported (for future chunk typing).
var _ = client.Usage{}
