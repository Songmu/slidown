package render

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
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

// TestSVGPlaceholderBinding verifies that an SVG bound to a layout picture
// placeholder is embedded as a native SVG picture (asvg:svgBlip + both PNG and
// SVG media parts) carrying the <p:ph> binding.
func TestSVGPlaceholderBinding(t *testing.T) {
	tmplPath := buildTemplateFile(t)
	tmpl, err := pptx.LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	layout := tmpl.ContentLayout()
	if layout == nil {
		t.Fatal("template has no content layout")
	}
	layout.Placeholders = append(layout.Placeholders, &pptx.PlaceholderInfo{
		Type: "pic", Idx: 20, Name: "Picture Placeholder",
		HasGeom: true, X: 1000000, Y: 1000000, W: 2000000, H: 2000000,
	})

	parsed, err := md.Parse("", []byte("# Pic\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" fill="#4C9AFF"/></svg>`
	slides[0].Images = []*slidown.Image{newSVGImage(t, svg)}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts := zipParts(t, buf.Bytes())
	slideXML := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slideXML, `<p:ph type="pic" idx="20"/>`) {
		t.Errorf("expected SVG bound to pic placeholder, got: %s", slideXML)
	}
	if !strings.Contains(slideXML, "asvg:svgBlip") {
		t.Errorf("expected native SVG blip, got: %s", slideXML)
	}
	if !strings.Contains(slideXML, "<p:pic>") {
		t.Errorf("expected a picture element for the placeholder SVG")
	}
	var hasPNG, hasSVG bool
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") {
			if strings.HasSuffix(name, ".png") {
				hasPNG = true
			}
			if strings.HasSuffix(name, ".svg") {
				hasSVG = true
			}
		}
	}
	if !hasPNG || !hasSVG {
		t.Errorf("expected both PNG and SVG media parts; png=%v svg=%v", hasPNG, hasSVG)
	}
}

// An SVG that falls back but references an external raster image is embedded as
// a best-effort raster-only picture (no native svgBlip): the image is still
// shown rather than dropped, but the native SVG isn't embedded because its
// external reference can't be resolved.
func TestRenderSVGExternalResourceRasterOnly(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" fill="red"/><image href="asset.png" width="100" height="100"/></svg>`
	parts := renderSlidesToParts(t, slidown.Slides{
		{Titles: []string{"ext"}, Images: []*slidown.Image{newSVGImage(t, svg)}},
	})
	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<p:pic>") {
		t.Errorf("expected a best-effort raster picture for the external-resource SVG")
	}
	if strings.Contains(slide, "asvg:svgBlip") {
		t.Errorf("did not expect a native SVG blip for an external-resource SVG")
	}
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") && strings.HasSuffix(name, ".svg") {
			t.Errorf("did not expect an embedded .svg media part: %s", name)
		}
	}
}

// A hyperlink or <desc> containing "href" text must not be misclassified as an
// external resource: the native SVG (svgBlip + .svg part) is preserved.
func TestRenderSVGHyperlinkNotExternal(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><desc>see href</desc><a href="https://example.com"><rect width="10" height="10" fill="red"/></a></svg>`
	parts := renderSlidesToParts(t, slidown.Slides{
		{Titles: []string{"link"}, Images: []*slidown.Image{newSVGImage(t, svg)}},
	})
	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "asvg:svgBlip") {
		t.Errorf("hyperlink/desc href must not be treated as external; native SVG should be kept: %s", slide)
	}
	var hasSVG bool
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") && strings.HasSuffix(name, ".svg") {
			hasSVG = true
		}
	}
	if !hasSVG {
		t.Errorf("expected an embedded .svg media part for the native SVG")
	}
}

// An unsupported SVG that references an external paint via url(file#id) is
// embedded as a best-effort raster-only picture (no native svgBlip).
func TestRenderSVGExternalURLRasterOnly(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="url(paints.svg#g)" clip-path="url(#c)"/></svg>`
	parts := renderSlidesToParts(t, slidown.Slides{
		{Titles: []string{"ext"}, Images: []*slidown.Image{newSVGImage(t, svg)}},
	})
	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<p:pic>") {
		t.Errorf("expected a best-effort raster picture for the external-url SVG")
	}
	if strings.Contains(slide, "asvg:svgBlip") {
		t.Errorf("did not expect a native SVG blip for an external-url SVG")
	}
}

// SVG text whose runs are separated by whitespace must keep that separator in
// the generated slide XML via xml:space="preserve".
func TestRenderSVGTextPreservesSeparator(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 40"><text x="1" y="20" fill="blue" font-size="10">Hello <tspan fill="red">world</tspan></text></svg>`
	parts := renderSlidesToParts(t, slidown.Slides{
		{Titles: []string{"txt"}, Images: []*slidown.Image{newSVGImage(t, svg)}},
	})
	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<a:custGeom>") && !strings.Contains(slide, `xml:space="preserve"`) {
		t.Errorf("expected xml:space=preserve on the whitespace-carrying run, got: %s", slide)
	}
	if !strings.Contains(slide, `xml:space="preserve"`) {
		t.Errorf("expected xml:space=preserve for the text separator, got: %s", slide)
	}
}
