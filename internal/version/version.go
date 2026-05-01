// Package version exposes compile-time build information.
//
// Values are overridden via -ldflags at build time; defaults keep the binary
// runnable and self-describing during local development.
package version

var (
	// Version is the semver tag (or "dev" for untagged local builds).
	Version = "dev"
	// Commit is the short git revision of the build.
	Commit = "unknown"
	// Date is the RFC3339 UTC build timestamp.
	Date = "unknown"
)

// Info returns a single-line human-readable version string.
func Info() string {
	return "askit " + Version + " (" + Commit + ", " + Date + ")"
}
