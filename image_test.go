package deck

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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
