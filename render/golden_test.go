package render

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/tenntenn/golden"
)

// TestSubtitleDistributionGolden locks the full slide XML produced when a layout
// exposes several subtitle-capable placeholders (a mix of native subTitle
// placeholders and a hint-promoted body placeholder) and a slide carries several
// subtitle-level headings. It exercises the whole path end to end: multi-slot
// classification, visual ordering by geometry (the slots are declared out of
// visual order on purpose), one-subtitle-per-slot distribution and the
// role="subTitle" marker on the hint-promoted slot.
//
// Run with UPDATE_GOLDEN=1 to (re)generate testdata/subtitle_slots.xml.golden.
func TestSubtitleDistributionGolden(t *testing.T) {
	tmplPath := buildTemplateFile(t)
	tmpl, err := pptx.LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	layout := tmpl.ContentLayout()
	if layout == nil {
		t.Fatal("template has no content layout")
	}
	// Three subtitle slots declared in shape-tree order idx=1,2,3 but positioned
	// so the visual (top-to-bottom) order is idx=3, idx=1, idx=2.
	layout.Placeholders = append(layout.Placeholders,
		&pptx.PlaceholderInfo{Type: "subTitle", Idx: 1, Name: "Subtitle 1", HasGeom: true, X: 100, Y: 2000, W: 500, H: 100},
		&pptx.PlaceholderInfo{Type: "body", Idx: 2, Name: "Subtitle 2", HasGeom: true, X: 100, Y: 3000, W: 500, H: 100},
		&pptx.PlaceholderInfo{Type: "subTitle", Idx: 3, Name: "Subtitle 3", HasGeom: true, X: 100, Y: 1000, W: 500, H: 100},
	)

	const markdown = "# Title\n\n## First Sub\n\n## Second Sub\n\n## Third Sub\n\nbody text\n"
	parsed, err := md.Parse("", []byte(markdown), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts := zipParts(t, buf.Bytes())
	slideXML := string(parts["ppt/slides/slide1.xml"])
	if slideXML == "" {
		t.Fatal("ppt/slides/slide1.xml missing from output")
	}
	// One tag per line keeps the golden diff-friendly and readable.
	pretty := strings.ReplaceAll(slideXML, "><", ">\n<")

	const name = "../testdata/subtitle_slots.xml"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		golden.Update(t, "", name, pretty)
		return
	}
	if diff := golden.Diff(t, "", name, pretty); diff != "" {
		t.Error(diff)
	}
}
