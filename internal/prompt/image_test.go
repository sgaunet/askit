package prompt_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sgaunet/askit/internal/config"
	"github.com/sgaunet/askit/internal/prompt"
)

// tinyPNG creates a simple color-filled PNG of the given dimensions.
func tinyPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 0xff, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// noisyPNG creates a PNG filled with random pixels (does not compress well)
// so callers can reliably trigger oversize checks.
func noisyPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewPCG(42, 1337))
	for i := 0; i < w*h*4; i += 4 {
		img.Pix[i] = uint8(rng.IntN(256))
		img.Pix[i+1] = uint8(rng.IntN(256))
		img.Pix[i+2] = uint8(rng.IntN(256))
		img.Pix[i+3] = 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadImageRef_Happy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "tiny.png")
	tinyPNG(t, p, 10, 10)

	dataURL, media, sz, err := prompt.LoadImageRef(p, config.Builtins().FileReferences)
	if err != nil {
		t.Fatalf("LoadImageRef: %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Errorf("unexpected data URL prefix: %q", dataURL[:50])
	}
	if media != "image/png" {
		t.Errorf("media = %q; want image/png", media)
	}
	if sz == 0 {
		t.Error("encoded size should be > 0")
	}
}

func TestLoadImageRef_Oversize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.png")
	noisyPNG(t, p, 1000, 1000) // noise PNG ~3-4 MB encoded

	policy := config.Builtins().FileReferences
	policy.MaxImageSizeMB = 1 // tight

	_, _, _, err := prompt.LoadImageRef(p, policy)
	if err == nil {
		t.Fatal("want oversize error")
	}
	if !strings.Contains(err.Error(), "max_image_size_mb") {
		t.Errorf("error missing size key: %v", err)
	}
}

func TestLoadImageRef_Resize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.png")
	tinyPNG(t, p, 3000, 1500)

	policy := config.Builtins().FileReferences
	policy.MaxImageSizeMB = 5
	policy.ResizeImages.Enabled = true
	policy.ResizeImages.MaxLongEdgePx = 1024
	policy.ResizeImages.JPEGQuality = 80

	_, media, _, err := prompt.LoadImageRef(p, policy)
	if err != nil {
		t.Fatalf("LoadImageRef: %v", err)
	}
	// PNG stays PNG after resize in our implementation
	if media != "image/png" {
		t.Errorf("media = %q; want image/png", media)
	}
}
