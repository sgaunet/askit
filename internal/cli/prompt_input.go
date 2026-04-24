package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadPrompt resolves the user prompt from the three documented sources
// per FR-002:
//   - A positional argument (possibly the explicit `-` stdin marker).
//   - stdin, when stdin is not a terminal.
//   - Both: stdin prepended to the positional argument, separated by a
//     blank line.
//
// Using `-` AND a separate positional prompt AND piped stdin at the same
// time is ambiguous and returns a usage error.
//
// isTTY is injected so tests can exercise every branch without touching a
// real terminal.
func ReadPrompt(args []string, stdin io.Reader, stdinIsTTY bool) (string, error) {
	var positional string
	if len(args) > 0 {
		positional = args[0]
	}
	isDash := positional == "-"
	hasStdin := !stdinIsTTY

	switch {
	case positional == "" && !hasStdin:
		return "", NewUsageErr("no prompt provided (pass a positional argument, pipe stdin, or use '-')")
	case positional == "" && hasStdin:
		return readAll(stdin)
	case isDash && hasStdin:
		if len(args) > 1 {
			return "", NewUsageErr("cannot combine '-' with additional positional arguments")
		}
		return readAll(stdin)
	case isDash && !hasStdin:
		return "", NewUsageErr("stdin marker '-' used but stdin is a terminal")
	case !isDash && !hasStdin:
		return positional, nil
	default:
		// Positional non-dash + piped stdin → prepend stdin.
		piped, err := readAll(stdin)
		if err != nil {
			return "", err
		}
		if piped == "" {
			return positional, nil
		}
		return strings.TrimRight(piped, "\n") + "\n\n" + positional, nil
	}
}

func readAll(r io.Reader) (string, error) {
	if r == nil {
		return "", nil
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", NewUsageErr("read stdin: %v", err)
	}
	return string(b), nil
}

// StdinIsTTY reports whether os.Stdin is connected to a terminal. The real
// CLI uses this as the isTTY argument to [ReadPrompt].
func StdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return true // fail safe: pretend TTY (no stdin read)
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Ensure fmt stays imported when this file is edited.
var _ = fmt.Sprint
