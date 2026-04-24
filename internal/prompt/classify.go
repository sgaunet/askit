package prompt

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/sgaunet/askit/internal/config"
)

// Classify resolves the final Kind for a bare file reference, honoring
// any override already present on tok and otherwise consulting the active
// [config.FileRefsPolicy]. When the extension is in neither list, the
// policy's UnknownStrategy decides: `error` returns KindUnknown and lets
// the caller raise, `text` / `image` force the class, `skip` yields
// KindUnknown with no error (caller is responsible for honoring skip).
func Classify(tok Token, policy config.FileRefsPolicy) (Kind, bool) {
	if tok.KindOverride != KindUnknown {
		return tok.KindOverride, true
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(tok.RefPath), "."))
	if slices.Contains(policy.ImageExtensions, ext) {
		return KindImage, false
	}
	if slices.Contains(policy.TextExtensions, ext) {
		return KindText, false
	}
	// Unknown extension. Let the caller handle per-strategy behavior —
	// returning KindUnknown means "apply UnknownStrategy".
	switch policy.UnknownStrategy {
	case config.UnknownText:
		return KindText, false
	case config.UnknownImage:
		return KindImage, false
	default:
		return KindUnknown, false
	}
}
