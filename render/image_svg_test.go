package render

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Songmu/slidown"
)

func renderSlidesToParts(t *testing.T, slides slidown.Slides) map[string][]byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := ToPresentation(slides).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		parts[f.Name] = b
	}
	return parts
}

// Note: the generic keys() helper used in failure messages below is defined in
// image_test.go, which is part of this same `render` test package.

func newSVGImage(t *testing.T, svg string) *slidown.Image {
	t.Helper()
	img, err := slidown.NewImageFromCodeBlock(bytes.NewReader([]byte(svg)))
	if err != nil {
		t.Fatalf("NewImageFromCodeBlock(svg): %v", err)
	}
	if !img.IsSVG() {
		t.Fatalf("expected image to be detected as SVG")
	}
	return img
}

// A simple SVG that svgshape can fully convert should become a native group of
// custom-geometry shapes (no embedded media).
func TestRenderSVGConvertsToShapes(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<rect x="10" y="10" width="80" height="80" fill="#ff0000"/>` +
		`</svg>`
	parts := renderSlidesToParts(t, slidown.Slides{
		{Titles: []string{"SVG shapes"}, Images: []*slidown.Image{newSVGImage(t, svg)}},
	})

	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<p:grpSp>") {
		t.Errorf("expected converted SVG group <p:grpSp>, got: %s", slide)
	}
	if !strings.Contains(slide, "<a:custGeom>") {
		t.Errorf("expected custom geometry in converted SVG, got: %s", slide)
	}
	if strings.Contains(slide, "<p:pic>") {
		t.Errorf("did not expect a picture for a fully-convertible SVG")
	}
	if _, ok := parts["ppt/media/image1.png"]; ok {
		t.Errorf("did not expect embedded media for a fully-convertible SVG")
	}
}

// An SVG using an unsupported feature (a filter) must fall back to a native SVG
// picture: a raster PNG fallback plus an embedded .svg referenced via svgBlip.
func TestRenderSVGFallsBackToNativePicture(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<defs><filter id="b"><feGaussianBlur stdDeviation="2"/></filter></defs>` +
		`<rect x="10" y="10" width="80" height="80" fill="#00ff00" filter="url(#b)"/>` +
		`</svg>`
	parts := renderSlidesToParts(t, slidown.Slides{
		{Titles: []string{"SVG picture"}, Images: []*slidown.Image{newSVGImage(t, svg)}},
	})

	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<p:pic>") {
		t.Errorf("expected a picture for the fallback SVG, got: %s", slide)
	}
	if strings.Contains(slide, "<p:grpSp>") {
		t.Errorf("did not expect a shape group for the fallback SVG")
	}
	if !strings.Contains(slide, "asvg:svgBlip") {
		t.Errorf("expected native SVG blip extension, got: %s", slide)
	}
	if _, ok := parts["ppt/media/image1.png"]; !ok {
		t.Errorf("expected a raster PNG fallback media part; have: %v", keys(parts))
	}
	var hasSVGMedia bool
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") && strings.HasSuffix(name, ".svg") {
			hasSVGMedia = true
		}
	}
	if !hasSVGMedia {
		t.Errorf("expected an embedded .svg media part; have: %v", keys(parts))
	}
}
