package pptx

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/Songmu/slidown/internal/testutil"
)

func TestReadSlideMetasFollowsPresentationSlideOrder(t *testing.T) {
	p := New()
	s1 := p.AddSlide()
	s1.Fingerprint = "fp-1"
	s2 := p.AddSlide()
	s2.Fingerprint = "fp-2"

	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	path := writeTempPPTX(t, reorderPresentationOrder(t, buf.Bytes()))

	metas, err := ReadSlideMetas(path)
	if err != nil {
		t.Fatalf("ReadSlideMetas: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}
	if got, want := metas[0].Fingerprint, "fp-2"; got != want {
		t.Fatalf("first meta fingerprint = %q, want %q", got, want)
	}
	if got, want := metas[1].Fingerprint, "fp-1"; got != want {
		t.Fatalf("second meta fingerprint = %q, want %q", got, want)
	}
}

func reorderPresentationOrder(t *testing.T, in []byte) []byte {
	t.Helper()
	parts, order := testutil.ReadZIP(t, in)
	parts["ppt/presentation.xml"] = testutil.SwapFirstTwoSlideIDs(t, parts["ppt/presentation.xml"])
	return testutil.WriteZIP(t, parts, order)
}

func slideOrderParts(ids, rels string) map[string][]byte {
	return map[string][]byte{
		"ppt/presentation.xml": []byte(`<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst>` +
			ids + `</p:sldIdLst></p:presentation>`),
		"ppt/_rels/presentation.xml.rels": []byte(`<Relationships>` + rels + `</Relationships>`),
		"ppt/slides/slide2.xml":           []byte(slideWithMeta("fp-2", "")),
		"ppt/slides/slide10.xml":          []byte(slideWithMeta("fp-10", "")),
	}
}

func TestSlideOrderFallback(t *testing.T) {
	const ids = `<p:sldId r:id="ten"/><p:sldId r:id="two"/>`
	const rels = `<Relationship Id="ten" Type="urn:test/slide" Target="slides/./slide10.xml"/><Relationship Id="two" Type="urn:test/slide" Target="/ppt/slides/slide2.xml"/>`
	numeric := []string{"ppt/slides/slide2.xml", "ppt/slides/slide10.xml"}
	presentation := []string{numeric[1], numeric[0]}
	tests := []struct {
		name   string
		change func(map[string][]byte)
		want   []string
	}{
		{"valid relative and absolute targets", nil, presentation},
		{"ppt-prefixed relative target", func(p map[string][]byte) {
			p["ppt/_rels/presentation.xml.rels"] = []byte(strings.ReplaceAll(string(p["ppt/_rels/presentation.xml.rels"]), "slides/./slide10.xml", "ppt/slides/slide10.xml"))
		}, presentation},
		{"absent presentation", func(p map[string][]byte) { delete(p, "ppt/presentation.xml") }, numeric},
		{"absent relationships", func(p map[string][]byte) { delete(p, "ppt/_rels/presentation.xml.rels") }, numeric},
		{"malformed presentation", func(p map[string][]byte) { p["ppt/presentation.xml"] = []byte("<") }, numeric},
		{"malformed relationships", func(p map[string][]byte) { p["ppt/_rels/presentation.xml.rels"] = []byte("<") }, numeric},
		{"empty order", func(p map[string][]byte) {
			p["ppt/presentation.xml"] = []byte(`<presentation/>`)
		}, numeric},
		{"duplicate relationship IDs", func(p map[string][]byte) {
			p["ppt/_rels/presentation.xml.rels"] = []byte(`<Relationships>` + rels + `<Relationship Id="ten" Type="urn:test/slide" Target="slides/slide2.xml"/></Relationships>`)
		}, numeric},
		{"duplicate presentation IDs retain repeated parts", func(p map[string][]byte) {
			p["ppt/presentation.xml"] = []byte(strings.ReplaceAll(string(p["ppt/presentation.xml"]), `r:id="two"`, `r:id="ten"`))
		}, []string{numeric[1], numeric[1]}},
		{"unresolved ID discards valid prefix", func(p map[string][]byte) {
			p["ppt/presentation.xml"] = []byte(strings.ReplaceAll(string(p["ppt/presentation.xml"]), `r:id="two"`, `r:id="missing"`))
		}, numeric},
		{"missing target part discards valid prefix", func(p map[string][]byte) {
			p["ppt/_rels/presentation.xml.rels"] = []byte(strings.ReplaceAll(string(p["ppt/_rels/presentation.xml.rels"]), "/ppt/slides/slide2.xml", "/ppt/slides/slide99.xml"))
		}, numeric},
		{"unusable relationships are ignored", func(p map[string][]byte) {
			p["ppt/_rels/presentation.xml.rels"] = []byte(`<Relationships><Relationship Type="urn:test/slide" Target="slides/slide2.xml"/><Relationship Id="ten" Type="urn:test/slide"/><Relationship Id="ten" Type="urn:test/theme" Target="theme.xml"/>` + rels + `</Relationships>`)
		}, presentation},
		{"no slide parts", func(p map[string][]byte) {
			delete(p, numeric[0])
			delete(p, numeric[1])
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := slideOrderParts(ids, rels)
			if tt.change != nil {
				tt.change(parts)
			}
			metas, _ := slideMetasAndCoreTitle(parts)
			var names []string
			for _, meta := range metas {
				names = append(names, meta.PartName)
			}
			if !slices.Equal(names, tt.want) {
				t.Fatalf("slide parts = %v, want %v", names, tt.want)
			}
		})
	}
}

func TestSlideOrderFallbackAcrossConsumers(t *testing.T) {
	// A valid first ID followed by an unresolved ID must fall back entirely.
	parts := slideOrderParts(`<p:sldId r:id="ten"/><p:sldId r:id="missing"/>`,
		`<Relationship Id="ten" Type="urn:test/slide" Target="slides/slide10.xml"/>`)
	names := []string{"ppt/slides/slide2.xml", "ppt/slides/slide10.xml"}
	for _, name := range names {
		slide := string(renderSlideForTest(name, name))
		parts[name] = []byte(strings.Replace(slide, `</p:spTree>`,
			manualShapeForStampTest("9", `<p:nvPr/>`)+`</p:spTree>`, 1))
	}
	in := testutil.WriteZIP(t, parts, nil)
	stamped, err := StampSlideKeys(in, map[int]string{1: "two", 2: "ten"})
	if err != nil {
		t.Fatal(err)
	}
	stamped, err = StampShapeKeys(stamped)
	if err != nil {
		t.Fatal(err)
	}
	outParts, _ := testutil.ReadZIP(t, stamped)
	metas, _ := slideMetasAndCoreTitle(outParts)
	if len(metas) != 2 || metas[0].PartName != names[0] || metas[0].Key != "two" ||
		metas[1].PartName != names[1] || metas[1].Key != "ten" {
		t.Fatalf("stamped metadata = %+v", metas)
	}
	sigs, err := ShapeSignaturesByPosition(stamped)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		want := shapeSignatures(parts[name])
		if len(want) == 0 || !slices.Equal(sigs[i+1], want) {
			t.Errorf("position %d signatures = %v, want %v", i+1, sigs[i+1], want)
		}
		if !bytes.Contains(outParts[name], []byte(`sk="shape#9"`)) {
			t.Errorf("%s manual shape was not stamped", name)
		}
	}
	again, err := StampSlideKeys(stamped, map[int]string{1: "two", 2: "ten"})
	if err != nil {
		t.Fatal(err)
	}
	again, err = StampShapeKeys(again)
	if err != nil || !bytes.Equal(again, stamped) {
		t.Fatalf("repeated stamping changed bytes or failed: %v", err)
	}
}
