package slidown

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

func dummyPNG(t *testing.T) *bytes.Buffer {
	t.Helper()
	// Create a 1x1 pixel stub PNG image
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255}) // Red pixel
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode PNG: %v", err)
	}
	return &buf
}

func TestNewImageFromCodeBlock(t *testing.T) {
	buf := dummyPNG(t)
	i, err := NewImageFromCodeBlock(buf)
	if err != nil {
		t.Fatalf("TestNewImageFromCodeBlock failed: %v", err)
	}
	if !i.fromMarkdown {
		t.Errorf("Image.fromMarkdown = false, want true")
	}
	if i.mimeType != MIMETypeImagePNG {
		t.Errorf("Image.mimeType = %q, want %q", i.mimeType, MIMETypeImagePNG)
	}
}

func TestRasterImageSizing(t *testing.T) {
	for _, path := range []string{"testdata/test.png", "testdata/test.jpeg"} {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			img, err := newImageFromBuffer(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			w, h, err := img.Dimensions()
			if err != nil || w != 400 || h != 400 {
				t.Fatalf("Dimensions() = %dx%d, %v; want 400x400, nil", w, h, err)
			}
			encoded, err := img.RasterPNG(0.01)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("RasterPNG() did not return PNG: %v", err)
			}
			if b := decoded.Bounds(); b.Dx() != 400 || b.Dy() != 400 {
				t.Fatalf("raster scale changed native bounds: %v", b)
			}
			// Cached dimensions must not read the source again.
			img.b = nil
			if cw, ch, err := img.Dimensions(); err != nil || cw != w || ch != h {
				t.Fatalf("cached Dimensions() = %dx%d, %v; want %dx%d, nil", cw, ch, err, w, h)
			}
		})
	}
}

func TestImageSizingErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		img         *Image
		dimError    string
		rasterError string
	}{
		{"nil", nil, "image is nil", "image is nil"},
		{"invalid raster", &Image{b: []byte("not an image")}, "failed to decode image config: ", "failed to decode image: "},
		{"malformed SVG", &Image{b: []byte(`<svg><path`), mimeType: MIMETypeImageSVG}, "failed to parse SVG: ", "failed to parse SVG: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, h, err := tc.img.Dimensions()
			if w != 0 || h != 0 || err == nil || !strings.HasPrefix(err.Error(), tc.dimError) {
				t.Fatalf("Dimensions() = %dx%d, %v; want zero size and %q error", w, h, err, tc.dimError)
			}
			if tc.img != nil && (tc.img.dimensionsOK || errors.Unwrap(err) == nil) {
				t.Fatal("failed Dimensions() must wrap its cause without caching a size")
			}
			b, err := tc.img.RasterPNG(1)
			if b != nil || err == nil || !strings.HasPrefix(err.Error(), tc.rasterError) {
				t.Fatalf("RasterPNG() = %v, %v; want nil and %q error", b, err, tc.rasterError)
			}
			if tc.img != nil && errors.Unwrap(err) == nil {
				t.Fatal("RasterPNG() error must retain its cause")
			}
		})
	}
}
