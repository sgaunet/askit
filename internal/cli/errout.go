package cli

import (
	"fmt"
	"io"
	"strings"
)

// FormatError renders an error for stderr in the canonical
// `askit: <category>: <detail>  (exit <N>)` form (FR-060/061).
// When verbose > 0, an additional indented "hint:" line is emitted with
// any hint supplied via Hint(err). When verbose >= 2 and the error is in
// the API category, the truncated body from APIBody(err) is also emitted.
func FormatError(w io.Writer, err error, verbose int) {
	if err == nil {
		return
	}
	cat := CategoryOf(err)
	code := CodeOf(err)
	msg := strings.TrimSpace(err.Error())

	if cat == CatGeneric {
		fmt.Fprintf(w, "askit: %s  (exit %d)\n", msg, code)
	} else {
		fmt.Fprintf(w, "askit: %s: %s  (exit %d)\n", cat, msg, code)
	}

	if verbose >= 1 {
		if h := Hint(err); h != "" {
			fmt.Fprintf(w, "  hint: %s\n", h)
		}
	}
	if verbose >= 2 && cat == CatAPI {
		if body := APIBody(err); body != "" {
			fmt.Fprintln(w, "  body:")
			for line := range strings.SplitSeq(truncate(body, 4096), "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}
}

// Hinter is implemented by errors that carry a verbose-only hint line.
type Hinter interface {
	Hint() string
}

// BodyCarrier is implemented by API errors that carry an upstream response
// body for inclusion under -vv.
type BodyCarrier interface {
	APIResponseBody() string
}

// Hint walks the error chain and returns the first available hint string.
func Hint(err error) string {
	for err != nil {
		if h, ok := err.(Hinter); ok {
			return h.Hint()
		}
		err = unwrapOnce(err)
	}
	return ""
}

// APIBody walks the error chain and returns the first available API response body.
func APIBody(err error) string {
	for err != nil {
		if b, ok := err.(BodyCarrier); ok {
			return b.APIResponseBody()
		}
		err = unwrapOnce(err)
	}
	return ""
}

func unwrapOnce(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "… (truncated)"
}
