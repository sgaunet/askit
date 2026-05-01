package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jsonEntry struct {
	Role      Role      `json:"role"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Cancelled bool      `json:"cancelled,omitempty"`
}

func saveJSON(path string, entries []Entry) error {
	out := make([]jsonEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, jsonEntry(e))
	}
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("save json: %w", err)
	}
	return writeFileAtomic(path, append(body, '\n'))
}

func saveMarkdown(path string, entries []Entry) error {
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "### %s — %s\n\n", e.Role, e.Timestamp.Format(time.RFC3339))
		sb.WriteString(e.Text)
		sb.WriteString("\n\n")
		if e.Cancelled {
			sb.WriteString("_[cancelled]_\n\n")
		}
	}
	return writeFileAtomic(path, []byte(sb.String()))
}

func saveText(path string, entries []Entry) error {
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "--- %s (%s) ---\n", e.Role, e.Timestamp.Format(time.RFC3339))
		sb.WriteString(e.Text)
		sb.WriteString("\n")
		if e.Cancelled {
			sb.WriteString("[cancelled]\n")
		}
	}
	return writeFileAtomic(path, []byte(sb.String()))
}
