package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/cli"
)

func TestOpenOutput_NewPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "out.txt")
	w, err := cli.OpenOutput(target, false)
	if err != nil {
		t.Fatalf("OpenOutput: %v", err)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q; want hello", got)
	}
	// No leftover temp siblings.
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".askit-out-") {
			t.Errorf("leftover temp: %s", e.Name())
		}
	}
}

func TestOpenOutput_ExistingRefusesWithoutForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := cli.OpenOutput(target, false)
	if err == nil {
		t.Fatal("want error when target exists and force=false")
	}
	if cli.CodeOf(err) != cli.ExitUsage {
		t.Errorf("exit code = %d; want %d", cli.CodeOf(err), cli.ExitUsage)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error missing --force hint: %v", err)
	}
	// File unchanged.
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Errorf("target was modified: %q", got)
	}
}

func TestOpenOutput_ExistingForceOverwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := cli.OpenOutput(target, true)
	if err != nil {
		t.Fatalf("OpenOutput: %v", err)
	}
	if _, err := w.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("got %q; want new", got)
	}
}

func TestOpenOutput_RollbackLeavesTargetUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := cli.OpenOutput(target, true)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("incomplete"))
	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("target mutated after rollback: %q", got)
	}
	// No temp leftovers.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".askit-out-") {
			t.Errorf("leftover temp after rollback: %s", e.Name())
		}
	}
}

func TestOpenOutput_DirectoryIsUsageError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := cli.OpenOutput(dir, true)
	if err == nil {
		t.Fatal("want error for directory target")
	}
}
