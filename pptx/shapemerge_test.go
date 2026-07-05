package pptx

import (
	"strings"
	"testing"
)

func renderSlideForTest(title, body string) []byte {
	s := &Slide{
		Shapes: []*Shape{
			{Name: "Title", Placeholder: PlaceholderTitle, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: title}}}}},
			{Name: "Content", Placeholder: PlaceholderBody, PlaceholderIdx: 1, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: body}}}}},
		},
	}
	idx := 0
	xml, _, _ := renderSlide(s, &idx)
	return []byte(xml)
}

func TestMergeSlideShapesPreservesUnchangedShape(t *testing.T) {
	newSld := renderSlideForTest("Same Title", "new body")

	// The existing slide has the same title (unchanged source) but carries a
	// manual edit (an xfrm marker) in the title, and a different body.
	oldSld := renderSlideForTest("Same Title", "old body")
	marker := `<a:xfrm><a:off x="424242" y="0"/><a:ext cx="1" cy="1"/></a:xfrm>`
	oldSld = []byte(strings.Replace(string(oldSld), `<p:spPr>`, `<p:spPr>`+marker, 1))

	merged, changed := mergeSlideShapes(newSld, oldSld)
	if !changed {
		t.Fatal("expected a shape-level merge to occur")
	}
	ms := string(merged)
	if !strings.Contains(ms, `x="424242"`) {
		t.Errorf("manual xfrm on the unchanged title was not preserved:\n%s", ms)
	}
	if !strings.Contains(ms, "new body") {
		t.Errorf("changed body should be regenerated to the new content:\n%s", ms)
	}
	if strings.Contains(ms, "old body") {
		t.Errorf("stale old body leaked into the merged slide:\n%s", ms)
	}
}

func TestMergeSlideShapesNoChangeWhenAllShapesDiffer(t *testing.T) {
	newSld := renderSlideForTest("New Title", "new body")
	oldSld := renderSlideForTest("Old Title", "old body")
	_, changed := mergeSlideShapes(newSld, oldSld)
	if changed {
		t.Error("expected no merge when every shape's fingerprint differs")
	}
}

func TestMergeSlideShapesSkipsShapeWithRels(t *testing.T) {
	newSld := renderSlideForTest("Same Title", "new body")

	// Existing title carries a hyperlink (a relationship). It must not be
	// spliced, because its r:id would not resolve in the new slide's rels.
	oldSld := renderSlideForTest("Same Title", "old body")
	link := `<a:hlinkClick xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId9"/>`
	oldSld = []byte(strings.Replace(string(oldSld), `<a:t>Same Title</a:t>`, link+`<a:t>Same Title</a:t>`, 1))

	merged, changed := mergeSlideShapes(newSld, oldSld)
	if changed {
		t.Errorf("expected no splice for a relationship-bearing shape, got:\n%s", string(merged))
	}
}

func TestShapeOverlap(t *testing.T) {
	a := []ShapeSignature{{SlotKey: "title#0", FP: "x"}, {SlotKey: "body#1", FP: "y"}}
	b := []ShapeSignature{{SlotKey: "title#0", FP: "x"}, {SlotKey: "body#1", FP: "z"}}
	if got := ShapeOverlap(a, b); got != 0.5 {
		t.Errorf("ShapeOverlap = %v, want 0.5", got)
	}
	if got := ShapeOverlap(a, a); got != 1 {
		t.Errorf("ShapeOverlap(identical) = %v, want 1", got)
	}
	if got := ShapeOverlap(a, nil); got != 0 {
		t.Errorf("ShapeOverlap(_, nil) = %v, want 0", got)
	}
}

func TestParseSlideShapesExtractsSlotAndFP(t *testing.T) {
	sld := renderSlideForTest("T", "B")
	shapes, _, err := parseSlideShapes(sld)
	if err != nil {
		t.Fatalf("parseSlideShapes: %v", err)
	}
	if len(shapes) != 2 {
		t.Fatalf("expected 2 shapes, got %d", len(shapes))
	}
	if shapes[0].slotKey != "title#0" {
		t.Errorf("shape[0] slotKey = %q, want title#0", shapes[0].slotKey)
	}
	if shapes[1].slotKey != "body#1" {
		t.Errorf("shape[1] slotKey = %q, want body#1", shapes[1].slotKey)
	}
	for i, s := range shapes {
		if s.fp == "" {
			t.Errorf("shape[%d] has no fingerprint", i)
		}
	}
}
