package cli_test

import (
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/cli"
)

func TestReadPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		stdin     string
		stdinTTY  bool
		want      string
		wantErr   bool
	}{
		{"positional only, TTY stdin", []string{"hello"}, "", true, "hello", false},
		{"stdin only, no positional", nil, "from stdin\n", false, "from stdin\n", false},
		{"explicit dash", []string{"-"}, "dash stdin\n", false, "dash stdin\n", false},
		{"combined stdin + positional",
			[]string{"summarize"}, "paste this\nand this\n", false,
			"paste this\nand this\n\nsummarize", false,
		},
		{"no prompt, TTY stdin", nil, "", true, "", true},
		{"dash but TTY stdin", []string{"-"}, "", true, "", true},
		{"dash + second arg", []string{"-", "extra"}, "x", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := cli.ReadPrompt(tt.args, strings.NewReader(tt.stdin), tt.stdinTTY)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v; wantErr = %t", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}
