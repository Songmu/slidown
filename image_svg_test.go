package slidown

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"testing"
)

func TestIsSVG(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "svg with xmlns",
			data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"></svg>`),
			want: true,
		},
		{
			name: "xml declaration",
			data: []byte(`<?xml version="1.0"?><svg viewBox="0 0 100 50"></svg>`),
			want: true,
		},
		{
			name: "bom and whitespace",
			data: append([]byte{0xef, 0xbb, 0xbf}, []byte("\n\t <svg viewBox=\"0 0 100 50\"></svg>")...),
			want: true,
		},
		{
			name: "png",
			data: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
			want: false,
		},
		{
			name: "jpeg",
			data: []byte{0xff, 0xd8, 0xff, 0xe0},
			want: false,
		},
		{
			name: "gif",
			data: []byte("GIF89a"),
			want: false,
		},
		{
			name: "text",
			data: []byte("not an image"),
			want: false,
		},
		{
			name: "html with svg in comment",
			data: []byte(`<html><body><!-- <svg> --></body></html>`),
			want: false,
		},
		{
			name: "html mentioning svg entity",
			data: []byte(`<html><p>see &lt;svg&gt;</p></html>`),
			want: false,
		},
		{
			name: "svg not the root element",
			data: []byte(`<wrapper><svg viewBox="0 0 1 1"></svg></wrapper>`),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSVG(tt.data); got != tt.want {
				t.Fatalf("isSVG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewImageAcceptsSVG(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"></svg>`)
	img, err := newImageFromBuffer(bytes.NewReader(svg))
	if err != nil {
		t.Fatalf("newImageFromBuffer() error = %v", err)
	}
	if !img.IsSVG() {
		t.Fatalf("IsSVG() = false, want true")
	}

	path := "test_svg_image.svg"
	if err := os.WriteFile(path, svg, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	defer os.Remove(path)

	img, err = NewImageFromMarkdown(path)
	if err != nil {
		t.Fatalf("NewImageFromMarkdown() error = %v", err)
	}
	if !img.IsSVG() {
		t.Fatalf("IsSVG() = false, want true")
	}
}

func TestSVGDimensions(t *testing.T) {
	tests := []struct {
		name  string
		svg   string
		wantW int
		wantH int
	}{
		{
			name:  "viewBox",
			svg:   `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"></svg>`,
			wantW: 100,
			wantH: 50,
		},
		{
			name:  "width height",
			svg:   `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"></svg>`,
			wantW: 24,
			wantH: 24,
		},
		{
			name:  "declared size overrides viewBox aspect",
			svg:   `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100" viewBox="0 0 100 100"></svg>`,
			wantW: 200,
			wantH: 100,
		},
		{
			name:  "px units on declared size",
			svg:   `<svg xmlns="http://www.w3.org/2000/svg" width="48px" height="24px" viewBox="0 0 10 10"></svg>`,
			wantW: 48,
			wantH: 24,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := newImageFromBuffer(bytes.NewReader([]byte(tt.svg)))
			if err != nil {
				t.Fatalf("newImageFromBuffer() error = %v", err)
			}
			w, h, err := img.Dimensions()
			if err != nil {
				t.Fatalf("Dimensions() error = %v", err)
			}
			if w != tt.wantW || h != tt.wantH {
				t.Fatalf("Dimensions() = %dx%d, want %dx%d", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestSVGRasterPNG(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect width="100" height="50" fill="red"/></svg>`)
	img, err := newImageFromBuffer(bytes.NewReader(svg))
	if err != nil {
		t.Fatalf("newImageFromBuffer() error = %v", err)
	}
	pngBytes, err := img.RasterPNG(1)
	if err != nil {
		t.Fatalf("RasterPNG() error = %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 50 {
		t.Fatalf("bounds = %dx%d, want 100x50", bounds.Dx(), bounds.Dy())
	}
	if _, err := img.Image(); err != nil {
		t.Fatalf("Image() error = %v", err)
	}
	if _, err := img.PHash(); err != nil {
		t.Fatalf("PHash() error = %v", err)
	}
}

func TestSVGJSONRoundTrip(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"></svg>`)
	img, err := newImageFromBuffer(bytes.NewReader(svg))
	if err != nil {
		t.Fatalf("newImageFromBuffer() error = %v", err)
	}
	data, err := json.Marshal(img)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got Image
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.IsSVG() {
		t.Fatalf("IsSVG() = false, want true")
	}
	if !bytes.Equal(got.Bytes(), svg) {
		t.Fatalf("Bytes() = %q, want %q", got.Bytes(), svg)
	}
	if got.mimeType != MIMETypeImageSVG {
		t.Fatalf("mimeType = %q, want %q", got.mimeType, MIMETypeImageSVG)
	}
}
