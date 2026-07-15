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

func TestSVGRasterPreservesAspect(t *testing.T) {
	// width:height is 2:1 but the viewBox is 1:1, so under the default
	// xMidYMid meet the content is a centered 100x100 square with transparent
	// bars on the left/right rather than a 2:1 stretch.
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100" viewBox="0 0 100 100"><rect width="100" height="100" fill="red"/></svg>`)
	img, err := newImageFromBuffer(bytes.NewReader(svg))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	pngBytes, err := img.RasterPNG(1)
	if err != nil {
		t.Fatalf("RasterPNG: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("bounds = %dx%d, want 200x100", b.Dx(), b.Dy())
	}
	// Center is opaque red; the left/right bars are transparent (letterboxed).
	if _, _, _, a := decoded.At(100, 50).RGBA(); a == 0 {
		t.Errorf("center pixel should be opaque (letterboxed square), got transparent")
	}
	if _, _, _, a := decoded.At(10, 50).RGBA(); a != 0 {
		t.Errorf("left bar should be transparent, got opaque (content was stretched)")
	}
	if _, _, _, a := decoded.At(190, 50).RGBA(); a != 0 {
		t.Errorf("right bar should be transparent, got opaque (content was stretched)")
	}
}

func TestParsePreserveAspect(t *testing.T) {
	cases := []struct {
		in                 string
		fx, fy             float64
		wantNone, wantSlce bool
	}{
		{"", 0.5, 0.5, false, false},
		{"xMidYMid meet", 0.5, 0.5, false, false},
		{"none", 0.5, 0.5, true, false},
		{"xMinYMin meet", 0, 0, false, false},
		{"xMaxYMax slice", 1, 1, false, true},
		{"defer xMinYMax meet", 0, 1, false, false},
	}
	for _, c := range cases {
		fx, fy, none, slice := parsePreserveAspect(c.in)
		if fx != c.fx || fy != c.fy || none != c.wantNone || slice != c.wantSlce {
			t.Errorf("parsePreserveAspect(%q) = (%v,%v,none=%v,slice=%v), want (%v,%v,none=%v,slice=%v)",
				c.in, fx, fy, none, slice, c.fx, c.fy, c.wantNone, c.wantSlce)
		}
	}
}

func TestSVGZeroRelativeUnitSizeSkipped(t *testing.T) {
	// width="0em" is an explicit zero in a relative unit; zero is
	// unit-independent, so the SVG viewport is empty and must report 0x0.
	img, err := newImageFromBuffer(bytes.NewReader([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="0em" height="100" viewBox="0 0 100 100"></svg>`)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 0 || h != 0 {
		t.Fatalf("width=0em should report 0x0, got %dx%d", w, h)
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

// Two SVGs differing only in <text> content must yield different slide
// fingerprints; the best-effort raster hash can't distinguish them, so SVG
// signatures use the raw checksum instead.
func TestSVGFingerprintUsesChecksum(t *testing.T) {
	mk := func(label string) *Slide {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 20"><text x="0" y="15">` + label + `</text></svg>`
		img, err := NewImageFromCodeBlock(bytes.NewReader([]byte(svg)))
		if err != nil {
			t.Fatalf("NewImageFromCodeBlock: %v", err)
		}
		return &Slide{Titles: []string{"t"}, Images: []*Image{img}}
	}
	a := mk("Alpha").Fingerprint()
	b := mk("Beta").Fingerprint()
	if a == b {
		t.Fatalf("fingerprints should differ for SVGs with different text content: %s", a)
	}
	if mk("Alpha").Fingerprint() != a {
		t.Fatalf("fingerprint should be stable for identical SVG content")
	}
}

func TestSVGDimensionsUnits(t *testing.T) {
	cases := []struct {
		name         string
		svg          string
		wantW, wantH int
	}{
		{"inches", `<svg xmlns="http://www.w3.org/2000/svg" width="1in" height="2in"></svg>`, 96, 192},
		{"points", `<svg xmlns="http://www.w3.org/2000/svg" width="72pt" height="144pt"></svg>`, 96, 192},
		{"percent with viewBox", `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 120 60"></svg>`, 120, 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img, err := newImageFromBuffer(bytes.NewReader([]byte(tc.svg)))
			if err != nil {
				t.Fatalf("newImageFromBuffer: %v", err)
			}
			w, h, err := img.Dimensions()
			if err != nil {
				t.Fatalf("Dimensions: %v", err)
			}
			if w != tc.wantW || h != tc.wantH {
				t.Fatalf("Dimensions = %dx%d, want %dx%d", w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestSVGExplicitZeroSizeSkipped(t *testing.T) {
	img, err := newImageFromBuffer(bytes.NewReader([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="100" viewBox="0 0 100 100"></svg>`)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 0 || h != 0 {
		t.Fatalf("explicit zero size should report 0x0, got %dx%d", w, h)
	}
}

func TestSVGRootSizeLenient(t *testing.T) {
	// A non-strict SVG (unquoted attribute value) is detected as SVG by the
	// lenient isSVG decoder; svgRootSize must use the same lenient settings so
	// the intrinsic size is still read instead of being dropped.
	svg := []byte(`<svg width="10" height="20" viewBox="0 0 10 20" data-x=bar></svg>`)
	if !isSVG(svg) {
		t.Fatal("expected the document to be detected as SVG")
	}
	w, h, vb := svgRootSize(svg)
	if w != "10" || h != "20" || vb != "0 0 10 20" {
		t.Fatalf("svgRootSize = (%q,%q,%q), want (10,20,0 0 10 20)", w, h, vb)
	}
}

func TestSVGCSSSizeOverridesAttr(t *testing.T) {
	// SVG 2 CSS width/height override the presentation attributes, so a
	// width="0" with style="width:100px" is sized 100x50, not dropped as zero.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="0" height="50" style="width:100px" viewBox="0 0 100 50"></svg>`
	img, err := newImageFromBuffer(bytes.NewReader([]byte(svg)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 100 || h != 50 {
		t.Fatalf("CSS width override: Dimensions = %dx%d, want 100x50", w, h)
	}
}

func TestSVGRelativeUnitDimensions(t *testing.T) {
	// A relative height (1em = 16px at the default font) must be resolved, so a
	// 32px x 1em SVG reports 32x16 rather than substituting the viewBox axis.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="32px" height="1em" viewBox="0 0 32 32"></svg>`
	img, err := newImageFromBuffer(bytes.NewReader([]byte(svg)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 32 || h != 16 {
		t.Fatalf("Dimensions = %dx%d, want 32x16", w, h)
	}
}

func TestSVGInvalidUnitNotZeroSize(t *testing.T) {
	// An unknown/malformed unit is an invalid dimension, not an explicit zero,
	// so the SVG uses its viewBox size rather than being dropped.
	for _, svg := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="0foo" height="100" viewBox="0 0 100 50"></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="0e" height="100" viewBox="0 0 100 50"></svg>`,
	} {
		img, err := newImageFromBuffer(bytes.NewReader([]byte(svg)))
		if err != nil {
			t.Fatalf("newImageFromBuffer: %v", err)
		}
		w, h, err := img.Dimensions()
		if err != nil {
			t.Fatalf("Dimensions: %v", err)
		}
		if w == 0 || h == 0 {
			t.Fatalf("invalid width should fall back to viewBox, got %dx%d for %q", w, h, svg)
		}
	}
}

func TestSVGRasterQuotedGtInAttr(t *testing.T) {
	// A '>' inside a quoted attribute must not be treated as the tag end, so the
	// non-pixel root size is still normalized and the SVG rasterizes.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" data-note=">" width="1in" height="1in" viewBox="0 0 96 96"><rect width="96" height="96" fill="red"/></svg>`
	img, err := newImageFromBuffer(bytes.NewReader([]byte(svg)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	pngBytes, err := img.RasterPNG(1)
	if err != nil {
		t.Fatalf("RasterPNG: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	// The red fill must be present (not a transparent placeholder).
	if _, _, _, a := decoded.At(48, 48).RGBA(); a == 0 {
		t.Errorf("expected a rendered fill, got a transparent placeholder")
	}
}

func TestSVGNamespacedSizeAttrIgnored(t *testing.T) {
	// A namespaced foo:width shares Name.Local "width" but must not affect the
	// intrinsic size; the real dimensions come from viewBox.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:foo="http://example.com/foo" foo:width="0" foo:height="0" viewBox="0 0 100 50"></svg>`
	img, err := newImageFromBuffer(bytes.NewReader([]byte(svg)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 100 || h != 50 {
		t.Fatalf("namespaced size attrs should be ignored, got %dx%d, want 100x50", w, h)
	}
}

func TestSVGZeroPercentSizeSkipped(t *testing.T) {
	img, err := newImageFromBuffer(bytes.NewReader([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="0%" height="100%" viewBox="0 0 100 100"></svg>`)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 0 || h != 0 {
		t.Fatalf("width=0%% should report 0x0, got %dx%d", w, h)
	}
}

func TestNormalizeSVGRootSizeIgnoresComment(t *testing.T) {
	// "<svg>" text inside a prolog comment must not be treated as the root tag.
	b := []byte(`<!-- <svg> --><svg xmlns="http://www.w3.org/2000/svg" width="1in" height="1in"><rect width="10" height="10"/></svg>`)
	out, ok := normalizeSVGRootSize(b)
	if !ok {
		t.Fatal("expected normalization to find the real root")
	}
	if !bytes.Contains(out, []byte(`width="96"`)) {
		t.Fatalf("expected the real root width to be normalized to px, got: %s", out)
	}
}

func TestSVGSingleDimensionNoViewBox(t *testing.T) {
	// width only, no viewBox: keep width, default height to 150.
	img, err := newImageFromBuffer(bytes.NewReader([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="200"></svg>`)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 200 || h != 150 {
		t.Fatalf("expected 200x150, got %dx%d", w, h)
	}
}

func TestSVGFractionalDimensionRatio(t *testing.T) {
	// A 0.1x100 viewBox must keep its 1:1000 ratio (not become 1x100).
	img, err := newImageFromBuffer(bytes.NewReader([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 0.1 100"></svg>`)))
	if err != nil {
		t.Fatalf("newImageFromBuffer: %v", err)
	}
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 1 || h != 1000 {
		t.Fatalf("expected 1x1000 (ratio preserved), got %dx%d", w, h)
	}
}
