package render

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
)

// makeImage builds an in-memory PNG-backed slidown.Image of the given size.
func makeImage(t *testing.T, w, h int) *slidown.Image {
	t.Helper()
	img, err := slidown.NewImageFromCodeBlock(bytes.NewReader(makePNG(t, w, h)))
	if err != nil {
		t.Fatalf("NewImageFromCodeBlock: %v", err)
	}
	return img
}

// TestImagePlaceholderBinding verifies that images are bound to a layout's
// picture placeholders (emitting <p:ph type="pic"/>) and that surplus images
// beyond the placeholder count fall back to the default flow layout (no ph).
func TestImagePlaceholderBinding(t *testing.T) {
	tmplPath := buildTemplateFile(t)
	tmpl, err := pptx.LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}

	// Inject two picture placeholders (with geometry) into the content layout.
	layout := tmpl.ContentLayout()
	if layout == nil {
		t.Fatal("template has no content layout")
	}
	layout.Placeholders = append(layout.Placeholders,
		&pptx.PlaceholderInfo{
			Type: "pic", Idx: 10, Name: "Picture Placeholder 1",
			HasGeom: true, X: 1000000, Y: 1000000, W: 2000000, H: 2000000,
		},
		&pptx.PlaceholderInfo{
			Type: "pic", Idx: 11, Name: "Picture Placeholder 2",
			HasGeom: true, X: 4000000, Y: 1000000, W: 2000000, H: 2000000,
		},
	)

	parsed, err := md.Parse("", []byte("# Pics\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	if len(slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(slides))
	}
	// Three images, two placeholders: two bound, one overflow.
	slides[0].Images = []*slidown.Image{
		makeImage(t, 100, 100),
		makeImage(t, 100, 100),
		makeImage(t, 100, 100),
	}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts := zipParts(t, buf.Bytes())
	slideXML := string(parts["ppt/slides/slide1.xml"])
	if slideXML == "" {
		t.Fatal("ppt/slides/slide1.xml missing")
	}

	if n := strings.Count(slideXML, "<p:pic>"); n != 3 {
		t.Errorf("expected 3 pictures total, got %d", n)
	}
	for _, tag := range []string{`<p:ph type="pic" idx="10"/>`, `<p:ph type="pic" idx="11"/>`} {
		if !strings.Contains(slideXML, tag) {
			t.Errorf("expected picture bound to placeholder %s, got: %s", tag, slideXML)
		}
	}
	// Exactly two pictures should carry a pic placeholder; the third (overflow)
	// image is therefore a plain floating picture.
	if n := strings.Count(slideXML, `<p:ph type="pic"`); n != 2 {
		t.Errorf("expected 2 pic-placeholder bindings, got %d", n)
	}
}

// TestTablePlacementIgnoresPlaceholderBoundImages verifies that images bound to
// picture placeholders do not make the body region "crowded": a table on a
// slide whose only image is placeholder-bound must be laid out at the top of the
// body region, not pushed into the lower half.
func TestTablePlacementIgnoresPlaceholderBoundImages(t *testing.T) {
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
		Type: "pic", Idx: 30, Name: "Picture Placeholder",
		HasGeom: true, X: 1000000, Y: 1000000, W: 2000000, H: 2000000,
	})

	// Expected non-crowded table top: the body region's Y.
	ry := contentY
	if _, y, _, _, ok := layout.BodyGeometry(); ok {
		ry = y
	}

	// Title + a table, but no body text; a single image that binds to the pic
	// placeholder (so no image is flow-laid into the body region).
	const markdown = "# Pics\n\n| a | b |\n| - | - |\n| 1 | 2 |\n"
	parsed, err := md.Parse("", []byte(markdown), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	slides[0].Images = []*slidown.Image{makeImage(t, 100, 100)}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts := zipParts(t, buf.Bytes())
	slideXML := string(parts["ppt/slides/slide1.xml"])

	// The image must be bound (so rest is empty).
	if !strings.Contains(slideXML, `<p:ph type="pic" idx="30"/>`) {
		t.Fatalf("expected image bound to picture placeholder, got: %s", slideXML)
	}
	// The table's graphicFrame must sit at the body region top, not pushed down.
	gf := strings.Index(slideXML, "<p:graphicFrame>")
	if gf == -1 {
		t.Fatalf("slide missing table graphicFrame: %s", slideXML)
	}
	wantY := fmt.Sprintf(`y="%d"`, ry)
	if !strings.Contains(slideXML[gf:], wantY) {
		t.Errorf("expected table placed at body top (%s); slide did not push table down: %s", wantY, slideXML[gf:])
	}
}

// images than picture placeholders, images fill placeholders in visual order
// and unused placeholders are simply left empty (no floating pictures).
func TestImagePlaceholderFewerImagesThanSlots(t *testing.T) {
	tmplPath := buildTemplateFile(t)
	tmpl, err := pptx.LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	layout := tmpl.ContentLayout()
	if layout == nil {
		t.Fatal("template has no content layout")
	}
	// Two placeholders declared out of visual order: idx=20 is to the right of
	// idx=21, so ordering by position must assign the single image to idx=21.
	layout.Placeholders = append(layout.Placeholders,
		&pptx.PlaceholderInfo{
			Type: "pic", Idx: 20, Name: "Right",
			HasGeom: true, X: 5000000, Y: 1000000, W: 2000000, H: 2000000,
		},
		&pptx.PlaceholderInfo{
			Type: "pic", Idx: 21, Name: "Left",
			HasGeom: true, X: 1000000, Y: 1000000, W: 2000000, H: 2000000,
		},
	)

	parsed, err := md.Parse("", []byte("# Pics\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	slides[0].Images = []*slidown.Image{makeImage(t, 100, 100)}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts := zipParts(t, buf.Bytes())
	slideXML := string(parts["ppt/slides/slide1.xml"])

	if n := strings.Count(slideXML, "<p:pic>"); n != 1 {
		t.Errorf("expected exactly 1 picture, got %d", n)
	}
	// The image must land in the left (visually first) placeholder, idx=21.
	if !strings.Contains(slideXML, `<p:ph type="pic" idx="21"/>`) {
		t.Errorf("expected image bound to the visually-first placeholder idx=21, got: %s", slideXML)
	}
}
