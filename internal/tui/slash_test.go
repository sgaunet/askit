package tui_test

import (
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/config"
	"github.com/sgaunet/askit/internal/tui"
)

func TestIsSlash(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"/help", true},
		{"/quit", true},
		{" /clear ", true}, // leading space is allowed
		{"hello world", false},
		{"/save foo.md", true},
		{"/send @file.txt", false}, // has @ → NOT a slash command
		{"a/b/c", false},
		{"", false},
	}
	for _, c := range cases {
		if got := tui.IsSlash(c.in); got != c.want {
			t.Errorf("IsSlash(%q) = %t; want %t", c.in, got, c.want)
		}
	}
}

func TestDispatch_Help(t *testing.T) {
	t.Parallel()
	sess := &tui.Session{}
	r := tui.DispatchSlash("/help", sess, nil)
	if !r.Handled || r.Err != nil || r.Notice == "" {
		t.Errorf("unexpected result: %+v", r)
	}
}

func TestDispatch_ClearSystemPresetModel(t *testing.T) {
	t.Parallel()
	sess := &tui.Session{SystemPrompt: "old", Model: "m"}
	sess.AppendUser("hi")
	sess.StartAssistant()

	presets := map[string]config.Preset{"ocr": {System: "OCR system"}}

	// /system
	r := tui.DispatchSlash("/system you are new", sess, nil)
	if !r.Handled || sess.SystemPrompt != "you are new" {
		t.Errorf("system update failed: %+v", sess)
	}

	// /preset clears history
	r = tui.DispatchSlash("/preset ocr", sess, presets)
	if !r.Handled || r.Err != nil || sess.SystemPrompt != "OCR system" || len(sess.History) != 0 {
		t.Errorf("preset apply failed: %+v  r=%+v", sess, r)
	}

	// unknown preset
	r = tui.DispatchSlash("/preset nope", sess, presets)
	if r.Err == nil || !strings.Contains(r.Err.Error(), "ocr") {
		t.Errorf("unknown preset should mention available; got %v", r.Err)
	}

	// /model preserves history
	sess.AppendUser("u")
	r = tui.DispatchSlash("/model new-model", sess, nil)
	if !r.Handled || sess.Model != "new-model" || len(sess.History) == 0 {
		t.Errorf("model switch should preserve history: %+v", sess)
	}

	// /clear empties but keeps system
	r = tui.DispatchSlash("/clear", sess, nil)
	if len(sess.History) != 0 || sess.SystemPrompt == "" {
		t.Errorf("clear failed: %+v", sess)
	}
}

func TestDispatch_SaveLast(t *testing.T) {
	t.Parallel()
	sess := &tui.Session{}
	sess.AppendUser("hi")
	entry := sess.StartAssistant()
	entry.Text = "reply"

	dir := t.TempDir()
	r := tui.DispatchSlash("/save-last "+dir+"/out.md", sess, nil)
	if r.Err != nil {
		t.Fatalf("save-last: %v", r.Err)
	}
}

func TestDispatch_SaveUnknownExtension(t *testing.T) {
	t.Parallel()
	sess := &tui.Session{}
	sess.AppendUser("hi")
	entry := sess.StartAssistant()
	entry.Text = "reply"

	r := tui.DispatchSlash("/save /tmp/out.xyz", sess, nil)
	if r.Err == nil {
		t.Fatal("want error on unknown extension")
	}
	if !strings.Contains(r.Err.Error(), "extension") {
		t.Errorf("err = %v; want extension-related", r.Err)
	}
}

func TestDispatch_UnknownCommand(t *testing.T) {
	t.Parallel()
	sess := &tui.Session{}
	r := tui.DispatchSlash("/nosuch", sess, nil)
	if !r.Handled || r.Err == nil {
		t.Errorf("unknown cmd should be handled-with-err: %+v", r)
	}
}

func TestDispatch_Quit(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"/quit", "/exit"} {
		r := tui.DispatchSlash(c, &tui.Session{}, nil)
		if !r.Quit {
			t.Errorf("%s should set Quit", c)
		}
	}
}
