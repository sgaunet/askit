package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// outDirPerm is the permission for newly created output parent directories.
// 0o750 keeps group-read but excludes world access on sensitive output paths.
const outDirPerm = 0o750

// AtomicWriter writes to a temporary sibling file and, on Commit, atomically
// renames it over the target (FR-006). Writer failures trigger automatic
// cleanup via Rollback.
type AtomicWriter struct {
	target string
	tmp    *os.File
	done   bool
}

// OpenOutput creates an [AtomicWriter] for path. If the target already
// exists as a regular file and force is false, returns a usage-level
// error (exit 2). Parent directories are created automatically.
func OpenOutput(path string, force bool) (*AtomicWriter, error) {
	if path == "" {
		return nil, NewUsageErr("output path is empty")
	}
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if info.IsDir() {
			return nil, NewUsageErr("%s: is a directory; pick a file path", path)
		}
		if !info.Mode().IsRegular() {
			return nil, NewUsageErr("%s: not a regular file; refusing to overwrite", path)
		}
		if !force {
			return nil, NewUsageErr("%s: already exists; pass --force / -F to overwrite", path)
		}
	case errors.Is(err, fs.ErrNotExist):
		// ok
	default:
		return nil, NewUsageErr("stat %s: %v", path, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, outDirPerm); err != nil {
		return nil, NewUsageErr("mkdir %s: %v", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".askit-out-*")
	if err != nil {
		return nil, NewUsageErr("create temp in %s: %v", dir, err)
	}
	return &AtomicWriter{target: path, tmp: tmp}, nil
}

// Write writes data to the temp file.
func (a *AtomicWriter) Write(p []byte) (int, error) {
	n, err := a.tmp.Write(p)
	if err != nil {
		return n, fmt.Errorf("write temp: %w", err)
	}
	return n, nil
}

// Commit closes the temp file and atomically renames it over the target.
// After a successful Commit, subsequent calls are no-ops.
func (a *AtomicWriter) Commit() error {
	if a.done {
		return nil
	}
	if err := a.tmp.Close(); err != nil {
		return NewUsageErr("close temp: %v", err)
	}
	if err := os.Rename(a.tmp.Name(), a.target); err != nil {
		return NewUsageErr("rename %s -> %s: %v", a.tmp.Name(), a.target, err)
	}
	a.done = true
	return nil
}

// Rollback removes the temp file. Safe to call after Commit (it's a no-op
// then) or before any Write (removes an empty file).
func (a *AtomicWriter) Rollback() error {
	if a.done {
		return nil
	}
	_ = a.tmp.Close()
	if err := os.Remove(a.tmp.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove temp: %w", err)
	}
	return nil
}

// Close is an [io.Closer] helper that defers to Rollback. Callers should
// pair OpenOutput with `defer w.Close()` and an explicit Commit.
func (a *AtomicWriter) Close() error { return a.Rollback() }

// Sentinel to keep io.Writer/Closer interfaces exported-as-used.
var _ io.WriteCloser = (*AtomicWriter)(nil)
