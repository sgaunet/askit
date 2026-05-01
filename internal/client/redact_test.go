package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/client"
)

func TestRedactHeaders(t *testing.T) {
	t.Parallel()
	in := http.Header{}
	in.Set("Authorization", "Bearer sk-live-secret")
	in.Set("Content-Type", "application/json")
	in.Set("X-Api-Key", "also-a-secret")

	out := client.RedactHeaders(in)
	if out.Get("Authorization") != client.Redacted {
		t.Errorf("Authorization not redacted: %q", out.Get("Authorization"))
	}
	if out.Get("X-Api-Key") != client.Redacted {
		t.Errorf("X-Api-Key not redacted: %q", out.Get("X-Api-Key"))
	}
	if out.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type mutated: %q", out.Get("Content-Type"))
	}
	// Original untouched.
	if in.Get("Authorization") != "Bearer sk-live-secret" {
		t.Errorf("original mutated: %q", in.Get("Authorization"))
	}
}

func TestRedactRequest_NeverLeaksKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, http.NoBody)
	req.Header.Set("Authorization", "Bearer sk-live-secret")

	cloned := client.RedactRequest(req)
	if strings.Contains(cloned.Header.Get("Authorization"), "sk-live-secret") {
		t.Errorf("cloned header still contains secret: %q", cloned.Header.Get("Authorization"))
	}
	if req.Header.Get("Authorization") != "Bearer sk-live-secret" {
		t.Errorf("original req header mutated")
	}
}
