package prompt

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/sgaunet/askit/internal/config"
)

// LoadImageRef reads an image reference, optionally resizes it, base64
// encodes the result, and returns a data URL suitable for an OpenAI
// `image_url` content part. The size limit is enforced on the ENCODED
// payload (FR-028): a large raw image that fits after resize+recompress
// passes.
func LoadImageRef(path string, policy config.FileRefsPolicy) (dataURL, mediaType string, bytes int64, err error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path is user-supplied by design
	if err != nil {
		return "", "", 0, fmt.Errorf("read %s: %w", path, err)
	}
	mediaType = detectMediaType(path, raw)

	// Optional resize: decode → resample with CatmullRom → re-encode as JPEG.
	if policy.ResizeImages.Enabled {
		resized, newMedia, rerr := maybeResize(raw, mediaType, policy.ResizeImages)
		if rerr != nil {
			return "", "", int64(len(raw)), fmt.Errorf("resize %s: %w", path, rerr)
		}
		raw = resized
		mediaType = newMedia
	}

	// Base64 encode and enforce size cap on encoded payload.
	encoded := base64.StdEncoding.EncodeToString(raw)
	totalBytes := int64(len(encoded))
	limit := int64(policy.MaxImageSizeMB) * 1024 * 1024
	if totalBytes > limit {
		return "", mediaType, totalBytes, &SizeError{
			Path:     path,
			Got:      totalBytes,
			Limit:    limit,
			Kind:     "image",
			LimitKey: "max_image_size_mb",
		}
	}
	dataURL = "data:" + mediaType + ";base64," + encoded
	return dataURL, mediaType, totalBytes, nil
}

// detectMediaType derives the MIME type from the extension when known,
// otherwise sniffs from the first 512 bytes per `net/http.DetectContentType`.
func detectMediaType(path string, body []byte) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	}
	return http.DetectContentType(body[:min(len(body), 512)])
}

// maybeResize downsamples the image if its longer edge exceeds the limit,
// returning the re-encoded byte slice and its new media type. Non-image
// media types pass through unchanged.
func maybeResize(raw []byte, media string, policy config.ResizePolicy) ([]byte, string, error) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		// Not a decodable image — leave as-is.
		return raw, media, nil //nolint:nilerr // non-image passthrough is intentional
	}

	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	longEdge := max(w, h)
	if longEdge <= policy.MaxLongEdgePx {
		return raw, media, nil
	}

	scale := float64(policy.MaxLongEdgePx) / float64(longEdge)
	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	switch media {
	case "image/png":
		if err := png.Encode(&buf, dst); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
		return buf.Bytes(), "image/png", nil
	default:
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: policy.JPEGQuality}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		return buf.Bytes(), "image/jpeg", nil
	}
}

// Read reads the entire reader into a byte slice. Small helper used only
// in tests for symmetry with os.ReadFile.
func readAll(r io.Reader) ([]byte, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read all: %w", err)
	}
	return buf, nil
}

var _ = readAll
