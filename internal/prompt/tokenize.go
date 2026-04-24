package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Tokenize walks the prompt string once and returns the sequence of text
// chunks and `@path` references it contains. Handles:
//
//   - `@/abs/path` / `@./rel` / `@~/home`
//   - backtick-quoted paths with spaces: `` @`my scan.png` ``
//   - escaped literal `@`: `\@` → produces an `@` in a TokenText
//   - optional `:kind` suffix (`:text` | `:image`) overriding classification
//
// An unterminated backtick produces an error so we never swallow malformed
// input silently.
func Tokenize(s string) ([]Token, error) {
	var (
		tokens  []Token
		cur     strings.Builder
		flushText = func() {
			if cur.Len() > 0 {
				tokens = append(tokens, Token{Kind: TokenText, Text: cur.String()})
				cur.Reset()
			}
		}
	)

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\\':
			if i+1 < len(runes) && runes[i+1] == '@' {
				cur.WriteRune('@')
				i++
				continue
			}
			cur.WriteRune(r)
		case '@':
			if !atWordBoundary(runes, i) {
				cur.WriteRune(r)
				continue
			}
			flushText()
			ref, advance, err := readRef(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, ref)
			i += advance
		default:
			cur.WriteRune(r)
		}
	}
	flushText()
	return tokens, nil
}

// atWordBoundary reports whether the `@` at index i in runes is at a word
// boundary — i.e. preceded by start-of-string, whitespace, or a punctuation
// character. This matches the user-intuitive rule that `foo@bar.com` is
// not a file reference.
func atWordBoundary(runes []rune, i int) bool {
	if i == 0 {
		return true
	}
	prev := runes[i-1]
	return prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' ||
		prev == '(' || prev == '[' || prev == '{' || prev == ',' || prev == ';' || prev == '"' || prev == '\''
}

// readRef reads the path portion starting at `@` (index i), plus any
// optional `:kind` suffix. Returns the built Token and the number of runes
// consumed *after* the leading `@`, so the caller advances by that plus 0
// (the outer loop's i++ accounts for `@` itself).
func readRef(runes []rune, i int) (Token, int, error) {
	if i+1 >= len(runes) {
		return Token{}, 0, fmt.Errorf("bare `@` at end of input (use \\@ to write a literal at-sign)")
	}
	// Path body.
	start := i + 1
	var (
		pathBuf strings.Builder
		j       = start
	)

	if runes[start] == '`' {
		// Backtick-quoted: consume until matching `.
		j = start + 1
		for ; j < len(runes) && runes[j] != '`'; j++ {
			pathBuf.WriteRune(runes[j])
		}
		if j >= len(runes) {
			return Token{}, 0, fmt.Errorf("unterminated `@`<backtick> quoted path")
		}
		// skip closing backtick
		j++
	} else {
		// Bare path: consume up to whitespace or EOF (but treat `:` specially
		// for kind suffix below).
		for ; j < len(runes); j++ {
			c := runes[j]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				break
			}
			pathBuf.WriteRune(c)
		}
	}

	raw := pathBuf.String()
	if raw == "" {
		return Token{}, 0, fmt.Errorf("empty `@` reference at position %d", i)
	}

	// Split trailing :kind off (only for bare paths — backtick quotes are
	// considered to contain only the path).
	kindOverride := KindUnknown
	path := raw
	if idx := strings.LastIndex(raw, ":"); idx >= 0 && runes[start] != '`' {
		suffix := raw[idx+1:]
		switch suffix {
		case "text":
			kindOverride = KindText
			path = raw[:idx]
		case "image":
			kindOverride = KindImage
			path = raw[:idx]
		}
	}

	// Expand ~.
	expanded := expandHome(path)

	// Advance count = (j - i - 1) because the outer loop's `i++` will consume
	// the `@` itself.
	advance := j - i - 1
	return Token{
		Kind:         TokenFileRef,
		RefPath:      expanded,
		KindOverride: kindOverride,
	}, advance, nil
}

// expandHome expands a leading "~" or "~/" to the user's home directory.
// Any other use of "~" (including "~user" for other users) is left alone
// because POSIX `~user` is rarely what a modern user intends.
func expandHome(p string) string {
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
