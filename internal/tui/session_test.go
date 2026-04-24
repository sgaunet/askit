package tui_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/prompt"
	"github.com/sgaunet/askit/internal/tui"
)

func TestSession_Lifecycle(t *testing.T) {
	t.Parallel()
	s := &tui.Session{SystemPrompt: "sys"}

	s.AppendUser("hi")
	if got := len(s.History); got != 1 {
		t.Errorf("after AppendUser len = %d; want 1", got)
	}

	a := s.StartAssistant()
	a.Text = "part1"
	s.AppendAssistantChunk(" part2")
	if s.LastAssistantText() != "part1 part2" {
		t.Errorf("LastAssistantText = %q", s.LastAssistantText())
	}

	s.MarkLastCancelled()
	if !s.History[len(s.History)-1].Cancelled {
		t.Error("cancel flag missing")
	}

	s.ClearHistory()
	if len(s.History) != 0 || s.SystemPrompt != "sys" {
		t.Errorf("ClearHistory wrong: %+v", s)
	}
}

func TestSession_RecordReferencesDedup(t *testing.T) {
	t.Parallel()
	s := &tui.Session{}
	s.RecordReferences([]prompt.FileRef{{Path: "/a"}, {Path: "/b"}})
	s.RecordReferences([]prompt.FileRef{{Path: "/b"}, {Path: "/c"}})
	if len(s.References) != 3 {
		t.Errorf("refs = %d; want 3", len(s.References))
	}
}

func TestSession_SaveLastReply(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	s := &tui.Session{}
	s.AppendUser("q")
	s.StartAssistant().Text = "extracted text"

	// Happy: .txt
	txt := filepath.Join(dir, "out.txt")
	if err := s.SaveLastReply(txt); err != nil {
		t.Fatalf("txt: %v", err)
	}
	body, _ := os.ReadFile(txt)
	if string(body) != "extracted text" {
		t.Errorf("txt body = %q", body)
	}

	// Happy: .md
	md := filepath.Join(dir, "out.md")
	if err := s.SaveLastReply(md); err != nil {
		t.Fatalf("md: %v", err)
	}

	// Unknown extension rejected
	bad := filepath.Join(dir, "out.xyz")
	if err := s.SaveLastReply(bad); err == nil {
		t.Fatal("want error on unknown extension")
	} else if !strings.Contains(err.Error(), "extension") {
		t.Errorf("err = %v", err)
	}
	// And file must not exist.
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Errorf("file unexpectedly created: %v", err)
	}
}

func TestSession_SaveConversation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	s := &tui.Session{}
	s.AppendUser("hello")
	s.StartAssistant().Text = "hi back"

	// JSON
	jp := filepath.Join(dir, "c.json")
	if err := s.SaveConversation(jp); err != nil {
		t.Fatalf("json: %v", err)
	}
	body, _ := os.ReadFile(jp)
	var parsed []map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, body)
	}
	if len(parsed) != 2 {
		t.Errorf("entries = %d; want 2", len(parsed))
	}

	// Markdown
	mp := filepath.Join(dir, "c.md")
	if err := s.SaveConversation(mp); err != nil {
		t.Fatalf("md: %v", err)
	}
	mbody, _ := os.ReadFile(mp)
	if !strings.Contains(string(mbody), "### user") || !strings.Contains(string(mbody), "### assistant") {
		t.Errorf("md missing role headings: %s", mbody)
	}

	// Unknown
	if err := s.SaveConversation(filepath.Join(dir, "c.xyz")); err == nil {
		t.Fatal("want error")
	}
}
