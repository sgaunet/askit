package cli

import (
	"bytes"
	"strings"
	"testing"
)

// These tests exercise the root command in-process via Execute(args) —
// cheap, hermetic, and covers the flag surface, --version, --help, and
// the unknown-flag → exit-2 path (T028).
//
// They live in package cli (not cli_test) so they can drive the internal
// rootCommand directly via newRootCommand.

func runRoot(t *testing.T, args ...string) (stdout, stderr string, code ExitCode) {
	t.Helper()
	root, globals := newRootCommand()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		if CategoryOf(err) == CatGeneric {
			err = WrapCategorized(CatUsage, ExitUsage, err)
		}
		FormatError(&errBuf, err, globals.Verbose)
		return outBuf.String(), errBuf.String(), CodeOf(err)
	}
	return outBuf.String(), errBuf.String(), ExitOK
}

func TestRoot_VersionPrintsBuildInfo(t *testing.T) {
	t.Parallel()
	stdout, _, code := runRoot(t, "--version")
	if code != ExitOK {
		t.Errorf("exit code = %d; want 0", code)
	}
	if !strings.HasPrefix(stdout, "askit ") {
		t.Errorf("stdout = %q; want askit-prefixed", stdout)
	}
}

func TestRoot_HelpListsSubcommands(t *testing.T) {
	t.Parallel()
	stdout, _, code := runRoot(t, "--help")
	if code != ExitOK {
		t.Errorf("exit code = %d; want 0", code)
	}
	for _, want := range []string{"query", "chat", "config", "models"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing subcommand %q; got:\n%s", want, stdout)
		}
	}
}

func TestRoot_UnknownFlagExits2(t *testing.T) {
	t.Parallel()
	_, stderr, code := runRoot(t, "--definitely-not-a-flag")
	if code != ExitUsage {
		t.Errorf("exit code = %d; want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "askit:") {
		t.Errorf("stderr should be askit-prefixed, got:\n%s", stderr)
	}
}

func TestRoot_QueryWithoutConfigErrorsOut(t *testing.T) {
	t.Parallel()
	// Running `query` with no config file and no endpoint/model supplied
	// produces a config error (exit 3).
	_, stderr, code := runRoot(t, "query", "hello")
	if code != ExitConfig {
		t.Errorf("exit = %d; want %d", code, ExitConfig)
	}
	if !strings.Contains(stderr, "endpoint: required") {
		t.Errorf("stderr should cite missing endpoint, got:\n%s", stderr)
	}
}

func TestRoot_ChatRefusesNonTTY(t *testing.T) {
	t.Parallel()
	_, stderr, code := runRoot(t, "chat")
	if code != ExitUsage {
		t.Errorf("chat exit = %d; want %d (non-TTY path)", code, ExitUsage)
	}
	if !strings.Contains(stderr, "TTY") {
		t.Errorf("stderr should mention TTY, got:\n%s", stderr)
	}
}
