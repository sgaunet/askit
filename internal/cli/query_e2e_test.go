package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeEndpoint builds an httptest.Server that mimics the OpenAI
// chat-completions contract. The response is fixed but parameterized by
// status so tests can exercise happy and error paths.
func fakeEndpoint(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
}

func writeConfig(t *testing.T, dir, endpoint, extra string) string {
	t.Helper()
	p := filepath.Join(dir, "config.yml")
	body := fmt.Sprintf(`
endpoint: %s
api_key: test-key
model: test-model
defaults:
  stream: false
  retries: 0
  timeout: 5s
  stream_idle_timeout: 5s
presets:
  ocr:
    system: "OCR"
    temperature: 0.0
%s
`, endpoint, extra)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func mkPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := range 20 {
		for x := range 20 {
			img.Set(x, y, color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func runWithConfig(t *testing.T, ctx context.Context, cfg string, stdin string, args ...string) (string, string, ExitCode) {
	t.Helper()
	_ = ctx
	// Drive via the package-level Execute wrapper with a custom set of
	// globals: we emulate the CLI by running the root command in-process
	// with `-c cfg` added to args.
	full := append([]string{"-c", cfg}, args...)
	return runRootWithStdin(t, full, stdin)
}

// stdinMu serializes every test that mutates os.Stdin so -race doesn't
// catch multiple concurrent swaps.
var stdinMu sync.Mutex

// runRootWithStdin is a test helper that mirrors runRoot but hands a custom
// stdin to the os.Stdin reader used by ReadPrompt. Because the runtime
// reads os.Stdin directly, we swap it in/out for the duration.
func runRootWithStdin(t *testing.T, args []string, stdin string) (string, string, ExitCode) {
	t.Helper()
	stdinMu.Lock()
	defer stdinMu.Unlock()

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	defer func() { os.Stdin = origStdin; _ = r.Close() }()

	go func() {
		defer w.Close()
		_, _ = w.Write([]byte(stdin))
	}()

	stdout, stderr, code := runRoot(t, args...)
	return stdout, stderr, code
}

func TestE2E_QueryHappyPath(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusOK, `{
		"choices":[{"message":{"content":"extracted text"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}
	}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")
	imgPath := filepath.Join(dir, "scan.png")
	mkPNG(t, imgPath)

	stdout, _, code := runWithConfig(t, context.Background(), cfg, "", "query", "-p", "ocr", "@"+imgPath)
	if code != ExitOK {
		t.Errorf("exit = %d; want 0", code)
	}
	if !strings.Contains(stdout, "extracted text") {
		t.Errorf("stdout missing expected reply: %q", stdout)
	}
}

func TestE2E_QueryMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, "http://127.0.0.1:1/v1", "")
	_, stderr, code := runWithConfig(t, context.Background(), cfg, "", "query", "@/does/not/exist.png")
	if code != ExitFile {
		t.Errorf("exit = %d; want %d", code, ExitFile)
	}
	if !strings.Contains(stderr, "does/not/exist.png") {
		t.Errorf("stderr should name the path, got:\n%s", stderr)
	}
}

func TestE2E_QueryAPIError(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusUnauthorized, `{"error":{"message":"bad key"}}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")
	imgPath := filepath.Join(dir, "scan.png")
	mkPNG(t, imgPath)

	_, stderr, code := runWithConfig(t, context.Background(), cfg, "", "query", "@"+imgPath)
	if code != ExitAPI {
		t.Errorf("exit = %d; want %d", code, ExitAPI)
	}
	if !strings.Contains(stderr, "401") {
		t.Errorf("stderr should include status, got:\n%s", stderr)
	}
}

func TestE2E_QueryUnreachable(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, "http://127.0.0.1:1/v1", "")
	imgPath := filepath.Join(dir, "scan.png")
	mkPNG(t, imgPath)

	_, stderr, code := runWithConfig(t, context.Background(), cfg, "", "query", "@"+imgPath)
	if code != ExitNetwork {
		t.Errorf("exit = %d; want %d", code, ExitNetwork)
	}
	if !strings.Contains(stderr, "askit: endpoint:") {
		t.Errorf("stderr should be category=endpoint, got:\n%s", stderr)
	}
}

func TestE2E_QueryDryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, "http://127.0.0.1:1/v1", "")
	imgPath := filepath.Join(dir, "scan.png")
	mkPNG(t, imgPath)

	stdout, stderr, code := runWithConfig(t, context.Background(), cfg, "", "query", "--dry-run", "-p", "ocr", "@"+imgPath)
	if code != ExitOK {
		t.Errorf("exit = %d; want 0", code)
	}
	// Dry-run writes JSON to stderr.
	if !strings.Contains(stderr, `"Authorization": "***"`) {
		t.Errorf("dry-run should redact Authorization; stderr=%s", stderr)
	}
	if strings.Contains(stderr, "test-key") {
		t.Errorf("dry-run stderr leaked the API key!\n%s", stderr)
	}
	// Stdout is empty.
	if stdout != "" {
		t.Errorf("dry-run stdout should be empty, got %q", stdout)
	}

	// The dry-run body should be valid JSON.
	var parsed map[string]any
	// Slice off any error-before-JSON noise: pick from the first `{` onward.
	if i := strings.Index(stderr, "{"); i >= 0 {
		_ = json.Unmarshal([]byte(stderr[i:]), &parsed)
	}
	if parsed["method"] != "POST" {
		t.Errorf("dry-run JSON malformed: %v", parsed)
	}
}

func TestE2E_QueryWriteToFile(t *testing.T) {
	srv := fakeEndpoint(t, http.StatusOK, `{"choices":[{"message":{"content":"hello on disk"}}]}`)
	defer srv.Close()

	dir := t.TempDir()
	cfg := writeConfig(t, dir, srv.URL+"/v1", "")
	imgPath := filepath.Join(dir, "scan.png")
	mkPNG(t, imgPath)
	outPath := filepath.Join(dir, "sub", "out.txt")

	_, _, code := runWithConfig(t, context.Background(), cfg, "", "query", "-p", "ocr", "-o", outPath, "@"+imgPath)
	if code != ExitOK {
		t.Errorf("exit = %d; want 0", code)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	if string(got) != "hello on disk\n" {
		t.Errorf("file content = %q; want 'hello on disk\\n'", string(got))
	}

	// Second run without --force MUST fail with exit 2.
	_, stderr, code := runWithConfig(t, context.Background(), cfg, "", "query", "-p", "ocr", "-o", outPath, "@"+imgPath)
	if code != ExitUsage {
		t.Errorf("second run exit = %d; want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr should hint at --force, got:\n%s", stderr)
	}

	// With --force it succeeds.
	_, _, code = runWithConfig(t, context.Background(), cfg, "", "query", "-p", "ocr", "--force", "-o", outPath, "@"+imgPath)
	if code != ExitOK {
		t.Errorf("forced run exit = %d; want 0", code)
	}
}
