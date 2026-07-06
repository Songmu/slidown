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

func TestMergeSlideShapesSkipsCarryOverForRelsBearingNonPlaceholder(t *testing.T) {
	newSld := renderSlideForTest("Title", "new body")
	oldSld := renderSlideForTest("Title", "old body")
	oldSld = []byte(strings.Replace(string(oldSld), `</p:spTree>`,
		`<p:sp><p:nvSpPr><p:cNvPr id="9" name="Manual Linked"/><p:cNvSpPr/><p:nvPr><p:extLst><p:ext uri="`+shapeMetaURI+`"><slidown:shape xmlns:slidown="`+fingerprintNS+`" sk="shape#linked"/></p:ext></p:extLst></p:nvPr></p:nvSpPr><p:spPr><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"><a:hlinkClick xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId99"/></a:rPr><a:t>manual linked</a:t></a:r></a:p></p:txBody></p:sp></p:spTree>`, 1))
	merged, _ := mergeSlideShapes(newSld, oldSld)
	if strings.Contains(string(merged), "manual linked") {
		t.Fatalf("relationship-bearing non-placeholder shape must not be carried over")
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

func TestShapeSlotKeyEmptyForNonPlaceholder(t *testing.T) {
	// A plain (non-placeholder) text box has no stable identity and must not be
	// slot-matched as a placeholder.
	plain := &Shape{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: "x"}}}}}
	if got := shapeSlotKey(plain); got != "" {
		t.Errorf("non-placeholder slot key = %q, want empty", got)
	}
	// An untyped placeholder still gets a slot key from its index.
	ph := &Shape{IsPlaceholder: true, PlaceholderIdx: 2, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: "x"}}}}}
	if got := shapeSlotKey(ph); got != "#2" {
		t.Errorf("untyped placeholder slot key = %q, want #2", got)
	}
}

func TestMergeSlideShapesCarriesOverStampedNonPlaceholderShape(t *testing.T) {
	newSld := renderSlideForTest("Title", "new body")
	oldSld := renderSlideForTest("Title", "old body")
	oldSld = []byte(strings.Replace(string(oldSld), `</p:spTree>`,
		`<p:sp><p:nvSpPr><p:cNvPr id="9" name="Manual Box"/><p:cNvSpPr/><p:nvPr><p:extLst><p:ext uri="`+shapeMetaURI+`"><slidown:shape xmlns:slidown="`+fingerprintNS+`" sk="shape#manual"/></p:ext></p:extLst></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="123456" y="222"/><a:ext cx="1" cy="1"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>manual keep</a:t></a:r></a:p></p:txBody></p:sp></p:spTree>`, 1))

	merged, changed := mergeSlideShapes(newSld, oldSld)
	if !changed {
		t.Fatal("expected merge with carried non-placeholder shape")
	}
	ms := string(merged)
	if !strings.Contains(ms, "manual keep") {
		t.Fatalf("manual non-placeholder shape was not carried over:\n%s", ms)
	}
	if !strings.Contains(ms, `x="123456"`) {
		t.Fatalf("manual geometry marker was not preserved:\n%s", ms)
	}
	if !strings.Contains(ms, "new body") || strings.Contains(ms, "old body") {
		t.Fatalf("changed generated body was not regenerated cleanly:\n%s", ms)
	}
}

func TestMergeSlideShapesDoesNotCarryOverPlaceholderShape(t *testing.T) {
	// An old slide has an extra placeholder (e.g. a subtitle) that is absent
	// from the new slide. Even though the placeholder has no match in newKeys,
	// it must NOT be carried over because it is a placeholder shape.
	newSld := renderSlideForTest("Title", "new body")
	oldSld := renderSlideForTest("Title", "old body")
	// Inject a subtitle placeholder with a stamped sk into the old slide's spTree.
	// Its slotKey will be "subtitle#..." so the carry-over loop must skip it.
	subtitlePh := `<p:sp><p:nvSpPr><p:cNvPr id="9" name="Subtitle"/><p:cNvSpPr/><p:nvPr><p:ph type="subTitle" idx="1"/><p:extLst><p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" sk="shape#subtitle"/></p:ext></p:extLst></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>old subtitle</a:t></a:r></a:p></p:txBody></p:sp>`
	oldSld = []byte(strings.Replace(string(oldSld), `</p:spTree>`, subtitlePh+`</p:spTree>`, 1))

	merged, _ := mergeSlideShapes(newSld, oldSld)
	if strings.Contains(string(merged), "old subtitle") {
		t.Fatalf("placeholder shape must not be carried over into the new slide:\n%s", string(merged))
	}
}

func TestMergeSlideShapesCarryOverIDAvoidsPictureID(t *testing.T) {
	// The new slide contains a <p:pic> with a high cNvPr id. The carried-over
	// non-placeholder shape must receive an id above the picture's id, not just
	// above the set of <p:sp> shape ids.
	newSld := renderSlideForTest("Title", "new body")
	// Inject a picture element (not a <p:sp>) with id="50" into the new slide.
	pic := `<p:pic><p:nvPicPr><p:cNvPr id="50" name="Picture 50"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill/><p:spPr/></p:pic>`
	newSld = []byte(strings.Replace(string(newSld), `</p:spTree>`, pic+`</p:spTree>`, 1))

	oldSld := renderSlideForTest("Title", "old body")
	manualShape := `<p:sp><p:nvSpPr><p:cNvPr id="9" name="Manual Box"/><p:cNvSpPr/><p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" sk="shape#manual"/></p:ext></p:extLst></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>manual keep</a:t></a:r></a:p></p:txBody></p:sp>`
	oldSld = []byte(strings.Replace(string(oldSld), `</p:spTree>`, manualShape+`</p:spTree>`, 1))

	merged, changed := mergeSlideShapes(newSld, oldSld)
	if !changed {
		t.Fatal("expected merge with carried non-placeholder shape")
	}
	if !strings.Contains(string(merged), `id="51"`) {
		t.Fatalf("carried shape must receive id=51 (above picture id=50), got:\n%s", string(merged))
	}
}

func TestStampShapeKeysIdempotent(t *testing.T) {
	in := twoSlidePresentationWithFingerprints(t, "fp-1", "fp-2")
	parts, order, err := readZipPartsFromBytes(in)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes: %v", err)
	}
	parts["ppt/slides/slide1.xml"] = []byte(strings.Replace(string(parts["ppt/slides/slide1.xml"]), `</p:spTree>`,
		`<p:sp><p:nvSpPr><p:cNvPr id="9" name="Manual Box"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>manual keep</a:t></a:r></a:p></p:txBody></p:sp></p:spTree>`, 1))
	withManual, err := zipFromParts(order, parts)
	if err != nil {
		t.Fatalf("zipFromParts: %v", err)
	}

	once, err := StampShapeKeys(withManual)
	if err != nil {
		t.Fatalf("StampShapeKeys (first): %v", err)
	}
	twice, err := StampShapeKeys(once)
	if err != nil {
		t.Fatalf("StampShapeKeys (second): %v", err)
	}
	if string(once) != string(twice) {
		t.Fatal("stamping shape keys twice should be idempotent")
	}
	onceParts, _, err := readZipPartsFromBytes(once)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes(once): %v", err)
	}
	slide1 := string(onceParts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, `sk="shape#9"`) {
		t.Fatalf("manual shape key was not stamped: %s", slide1)
	}
}

func TestStampShapeKeysHandlesSpacedSelfClosingNvPr(t *testing.T) {
	in := twoSlidePresentationWithFingerprints(t, "fp-1", "fp-2")
	parts, order, err := readZipPartsFromBytes(in)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes: %v", err)
	}
	// Some serializers emit "<p:nvPr />" (space before the slash) for a
	// hand-added text box; stamping must still apply.
	parts["ppt/slides/slide1.xml"] = []byte(strings.Replace(string(parts["ppt/slides/slide1.xml"]), `</p:spTree>`,
		`<p:sp><p:nvSpPr><p:cNvPr id="9" name="Manual Box"/><p:cNvSpPr/><p:nvPr /></p:nvSpPr><p:spPr><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>manual keep</a:t></a:r></a:p></p:txBody></p:sp></p:spTree>`, 1))
	withManual, err := zipFromParts(order, parts)
	if err != nil {
		t.Fatalf("zipFromParts: %v", err)
	}

	stamped, err := StampShapeKeys(withManual)
	if err != nil {
		t.Fatalf("StampShapeKeys: %v", err)
	}
	stampedParts, _, err := readZipPartsFromBytes(stamped)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes(stamped): %v", err)
	}
	slide1 := string(stampedParts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, `sk="shape#9"`) {
		t.Fatalf("shape with spaced self-closing nvPr was not stamped: %s", slide1)
	}
	if !strings.Contains(slide1, `<p:nvPr><p:extLst>`) {
		t.Fatalf("spaced nvPr was not expanded to carry the extension: %s", slide1)
	}
}
