package cli_test

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/cli"
)

func TestExitCodes_Stable(t *testing.T) {
	t.Parallel()
	got := []cli.ExitCode{
		cli.ExitOK, cli.ExitGeneric, cli.ExitUsage, cli.ExitConfig,
		cli.ExitFile, cli.ExitNetwork, cli.ExitAPI, cli.ExitTimeout,
	}
	want := []cli.ExitCode{0, 1, 2, 3, 4, 5, 6, 7}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ExitCode[%d] = %d; want %d", i, got[i], want[i])
		}
	}
}

func TestCodeOf_RespectsWrapping(t *testing.T) {
	t.Parallel()
	base := cli.NewFileErr("missing: %s", "./scan.png")
	wrapped := errors.Join(errors.New("context"), base)
	if got := cli.CodeOf(wrapped); got != cli.ExitFile {
		t.Errorf("CodeOf = %d; want %d", got, cli.ExitFile)
	}
	if got := cli.CategoryOf(base); got != cli.CatFile {
		t.Errorf("CategoryOf = %q; want %q", got, cli.CatFile)
	}
}

func TestFormatError_CanonicalRegex(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^askit: [a-z]+: .+  \(exit \d+\)$`)
	cases := []error{
		cli.NewUsageErr("bad flag %q", "--oops"),
		cli.NewConfigErr("line 7: invalid"),
		cli.NewFileErr("./x.png: not found"),
		cli.NewNetworkErr("GET http://x/v1: refused"),
		cli.NewAPIErr("500: Internal Server Error"),
		cli.NewTimeoutErr("connect: 60s exceeded"),
	}
	for i, err := range cases {
		var buf bytes.Buffer
		cli.FormatError(&buf, err, 0)
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("case %d: got %d lines, want 1: %q", i, len(lines), buf.String())
			continue
		}
		if !re.MatchString(lines[0]) {
			t.Errorf("case %d: %q does not match %q", i, lines[0], re)
		}
	}
}

type hintedErr struct{ err string; hint string }

func (h *hintedErr) Error() string { return h.err }
func (h *hintedErr) Hint() string  { return h.hint }

func TestFormatError_VerboseEmitsHint(t *testing.T) {
	t.Parallel()
	inner := &hintedErr{err: "./huge.png: 42 MB > 20 MB limit", hint: "enable resize_images in config"}
	err := cli.WrapCategorized(cli.CatFile, cli.ExitFile, inner)
	var buf bytes.Buffer
	cli.FormatError(&buf, err, 1)
	out := buf.String()
	if !strings.Contains(out, "hint: enable resize_images") {
		t.Errorf("verbose=1 should emit hint, got:\n%s", out)
	}
}

type apiBodyErr struct{ err string; body string }

func (a *apiBodyErr) Error() string            { return a.err }
func (a *apiBodyErr) APIResponseBody() string  { return a.body }

func TestFormatError_VVEmitsAPIBody(t *testing.T) {
	t.Parallel()
	inner := &apiBodyErr{err: "400 Bad Request", body: `{"error":"bad_model"}`}
	err := cli.WrapCategorized(cli.CatAPI, cli.ExitAPI, inner)
	var buf bytes.Buffer
	cli.FormatError(&buf, err, 2)
	out := buf.String()
	if !strings.Contains(out, `{"error":"bad_model"}`) {
		t.Errorf("verbose=2 on API category should emit body, got:\n%s", out)
	}
	// Non-API category with -vv should NOT include any body.
	var buf2 bytes.Buffer
	cli.FormatError(&buf2, cli.NewFileErr("./x.png: not found"), 2)
	if strings.Contains(buf2.String(), "body:") {
		t.Errorf("non-API -vv should not emit body, got:\n%s", buf2.String())
	}
}

func TestFormatError_NilNoop(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cli.FormatError(&buf, nil, 0)
	if buf.Len() != 0 {
		t.Errorf("nil err should produce no output, got %q", buf.String())
	}
}
