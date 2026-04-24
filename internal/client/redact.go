package client

import "net/http"

// Redacted is the fixed string used in place of any secret value.
const Redacted = "***"

// RedactHeaders returns a copy of h where every sensitive header value is
// replaced with [Redacted]. The input is never mutated so callers can pass
// *http.Request.Header directly without touching the live request.
func RedactHeaders(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vs := range h {
		switch http.CanonicalHeaderKey(k) {
		case "Authorization", "X-Api-Key", "Api-Key", "Openai-Api-Key":
			out[k] = []string{Redacted}
		default:
			cp := make([]string, len(vs))
			copy(cp, vs)
			out[k] = cp
		}
	}
	return out
}

// RedactRequest returns a shallow-cloned [http.Request] whose Header field
// has sensitive values replaced with [Redacted]. The body is NOT cloned;
// callers wanting the body should use a separately-captured copy.
func RedactRequest(req *http.Request) *http.Request {
	if req == nil {
		return nil
	}
	c := req.Clone(req.Context())
	c.Header = RedactHeaders(req.Header)
	return c
}
