package prompt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors for tokenize failures.
var (
	errBareAtEnd        = errors.New("bare `@` at end of input (use \\@ to write a literal at-sign)")
	errUnterminatedTick = errors.New("unterminated `@`<backtick> quoted path")
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
// wordBoundaryPreceders is the set of characters that, when immediately before
// an `@`, mark a word boundary (i.e. the `@` starts a file reference).
const wordBoundaryPreceders = " \t\n\r([{,;\"'"

func atWordBoundary(runes []rune, i int) bool {
	if i == 0 {
		return true
	}
	return strings.ContainsRune(wordBoundaryPreceders, runes[i-1])
}

// readRef reads the path portion starting at `@` (index i), plus any
// optional `:kind` suffix. Returns the built Token and the number of runes
// consumed *after* the leading `@`, so the caller advances by that plus 0
// (the outer loop's i++ accounts for `@` itself).
func readRef(runes []rune, i int) (Token, int, error) {
	if i+1 >= len(runes) {
		return Token{}, 0, errBareAtEnd
	}
	start := i + 1
	raw, j, isQuoted, err := readRefPath(runes, start)
	if err != nil {
		return Token{}, 0, err
	}
	if raw == "" {
		return Token{}, 0, fmt.Errorf("empty `@` reference at position %d: %w", i, errBareAtEnd)
	}
	kindOverride, path := parseKindSuffix(raw, isQuoted)
	advance := j - i - 1
	return Token{
		Kind:         TokenFileRef,
		RefPath:      expandHome(path),
		KindOverride: kindOverride,
	}, advance, nil
}

// readRefPath reads the path portion of a @ref token starting at runes[start].
// Returns the raw path text, the index after the last consumed rune, whether
// the path was backtick-quoted, and any error.
func readRefPath(runes []rune, start int) (string, int, bool, error) {
	var pathBuf strings.Builder
	j := start
	if runes[start] == '`' {
		j = start + 1
		for ; j < len(runes) && runes[j] != '`'; j++ {
			pathBuf.WriteRune(runes[j])
		}
		if j >= len(runes) {
			return "", 0, false, errUnterminatedTick
		}
		j++ // skip closing backtick
		return pathBuf.String(), j, true, nil
	}
	for ; j < len(runes); j++ {
		c := runes[j]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		pathBuf.WriteRune(c)
	}
	return pathBuf.String(), j, false, nil
}

// parseKindSuffix splits a trailing :text or :image kind specifier off raw.
// isQuoted means the path came from backtick notation (no suffix allowed).
func parseKindSuffix(raw string, isQuoted bool) (Kind, string) {
	if isQuoted {
		return KindUnknown, raw
	}
	idx := strings.LastIndex(raw, ":")
	if idx < 0 {
		return KindUnknown, raw
	}
	switch raw[idx+1:] {
	case "text":
		return KindText, raw[:idx]
	case "image":
		return KindImage, raw[:idx]
	}
	return KindUnknown, raw
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
