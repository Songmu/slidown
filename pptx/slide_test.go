package pptx

import (
	"strings"
	"testing"
)

func TestRenderShapeEmitsRoleMarker(t *testing.T) {
	sh := &Shape{
		Placeholder:    PlaceholderBody,
		PlaceholderIdx: 1,
		Role:           "subTitle",
		Paragraphs:     []*Paragraph{{Runs: []*Run{{Text: "副題"}}}},
	}
	var rels []slideRel
	relIdx := 1
	out := renderShape(sh, 2, &relIdx, &rels)

	if !strings.Contains(out, `<p:ph type="body" idx="1"/>`) {
		t.Errorf("expected body placeholder type to be preserved, got: %s", out)
	}
	if !strings.Contains(out, `uri="`+shapeMetaURI+`"`) {
		t.Errorf("expected shape meta extension with uri %s, got: %s", shapeMetaURI, out)
	}
	if !strings.Contains(out, `role="subTitle"`) {
		t.Errorf("expected role=\"subTitle\" attribute, got: %s", out)
	}
	// The extension must live inside <p:nvPr>, not at the shape's top level
	// (where <p:extLst> would be invalid per the OOXML schema for p:sp).
	phIdx := strings.Index(out, `<p:ph`)
	extIdx := strings.Index(out, `<p:extLst>`)
	nvprCloseIdx := strings.Index(out, `</p:nvPr>`)
	if phIdx < 0 || extIdx < 0 || nvprCloseIdx < 0 {
		t.Fatalf("missing expected fragments in: %s", out)
	}
	if !(phIdx < extIdx && extIdx < nvprCloseIdx) {
		t.Errorf("extLst not positioned after <p:ph> and before </p:nvPr>: %s", out)
	}
}

func TestRenderShapeOmitsRoleMarkerWhenEmpty(t *testing.T) {
	sh := &Shape{
		Placeholder:    PlaceholderBody,
		PlaceholderIdx: 1,
		Paragraphs:     []*Paragraph{{Runs: []*Run{{Text: "body"}}}},
	}
	var rels []slideRel
	relIdx := 1
	out := renderShape(sh, 2, &relIdx, &rels)
	if strings.Contains(out, `<p:extLst>`) {
		t.Errorf("did not expect extLst for Role=\"\" shape, got: %s", out)
	}
}
