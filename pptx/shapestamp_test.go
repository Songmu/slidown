package pptx

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func manualShapeForStampTest(id, nvPr string) string {
	idAttr := ""
	if id != "" {
		idAttr = `id="` + id + `" `
	}
	return `<p:sp><p:nvSpPr><p:cNvPr ` + idAttr + `name="Manual Box"/><p:cNvSpPr/>` + nvPr + `</p:nvSpPr><p:spPr><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>manual keep</a:t></a:r></a:p></p:txBody></p:sp>`
}

func presentationWithShapesForStampTest(t *testing.T, shapes ...string) []byte {
	t.Helper()
	in := twoSlidePresentationWithFingerprints(t, "fp-1", "fp-2")
	parts, order, err := readZipPartsFromBytes(in)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes: %v", err)
	}
	parts["ppt/slides/slide1.xml"] = []byte(strings.Replace(string(parts["ppt/slides/slide1.xml"]),
		`</p:spTree>`, strings.Join(shapes, "")+`</p:spTree>`, 1))
	out, err := zipFromParts(order, parts)
	if err != nil {
		t.Fatalf("zipFromParts: %v", err)
	}
	return out
}

func TestStampShapeKeyPreservesXML(t *testing.T) {
	const meta = `<slidown:shape xmlns:slidown="` + fingerprintNS + `" role="manual" fp="a&amp;&#38;&#x26;"/>`
	const keyedMeta = `<slidown:shape xmlns:slidown="` + fingerprintNS + `" role="manual" fp="a&amp;&#38;&#x26;" sk="existing&amp;&#38;"/>`
	const ext = `<p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" sk="shape#9"/></p:ext>`
	const foreign = `<p:ext uri="{OTHER}"><other:data xmlns:other="urn:other" value="a&#38;b"/></p:ext>`
	const specialKey = `&<>"'&amp;`
	const escapedKey = `&amp;&lt;&gt;&quot;&apos;&amp;amp;`
	tests := []struct {
		name    string
		nvPr    string
		key     string
		want    string
		changed bool
	}{
		{
			name:    "add key without rewriting role or fingerprint entities",
			nvPr:    `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `">` + meta + `</p:ext></p:extLst></p:nvPr>`,
			key:     "shape#9",
			want:    `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" role="manual" fp="a&amp;&#38;&#x26;" sk="shape#9"/></p:ext></p:extLst></p:nvPr>`,
			changed: true,
		},
		{
			name: "existing key is not overwritten",
			nvPr: `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `">` + keyedMeta + `</p:ext></p:extLst></p:nvPr>`,
			key:  "replacement",
			want: `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `">` + keyedMeta + `</p:ext></p:extLst></p:nvPr>`,
		},
		{
			name:    "expanded nvPr retains unrelated child",
			nvPr:    `<p:nvPr><p:custDataLst/></p:nvPr>`,
			key:     "shape#9",
			want:    `<p:nvPr><p:custDataLst/><p:extLst>` + ext + `</p:extLst></p:nvPr>`,
			changed: true,
		},
		{
			name:    "reuse foreign extension list preserving trailing whitespace",
			nvPr:    "<p:nvPr><p:extLst>" + foreign + "</p:extLst>\n  </p:nvPr>",
			key:     "shape#9",
			want:    "<p:nvPr><p:extLst>" + foreign + ext + "</p:extLst>\n  </p:nvPr>",
			changed: true,
		},
		{
			name: "missing nvPr is unchanged",
			key:  "shape#9",
		},
		{
			name:    "escape key in new metadata",
			nvPr:    `<p:nvPr/>`,
			key:     specialKey,
			want:    `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" sk="` + escapedKey + `"/></p:ext></p:extLst></p:nvPr>`,
			changed: true,
		},
		{
			name:    "escape key added to existing metadata",
			nvPr:    `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `">` + meta + `</p:ext></p:extLst></p:nvPr>`,
			key:     specialKey,
			want:    `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" role="manual" fp="a&amp;&#38;&#x26;" sk="` + escapedKey + `"/></p:ext></p:extLst></p:nvPr>`,
			changed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []byte(manualShapeForStampTest("9", tt.nvPr))
			original := bytes.Clone(in)
			got, changed := stampShapeKey(in, tt.key)
			if changed != tt.changed {
				t.Errorf("changed = %v, want %v", changed, tt.changed)
			}
			if want := manualShapeForStampTest("9", tt.want); string(got) != want {
				t.Errorf("stampShapeKey() = %s, want %s", got, want)
			}
			if !bytes.Equal(in, original) {
				t.Error("stampShapeKey mutated its input")
			}
			if twice, changed := stampShapeKey(got, tt.key); changed || !bytes.Equal(twice, got) {
				t.Errorf("stamping twice changed the shape: %s", twice)
			}
		})
	}
}

func TestStampShapeKeysUniqueKeys(t *testing.T) {
	const reserved = `<p:nvPr><p:extLst><p:ext uri="` + shapeMetaURI + `"><slidown:shape xmlns:slidown="` + fingerprintNS + `" sk="shape#9-2"/></p:ext></p:extLst></p:nvPr>`
	tests := []struct {
		name   string
		shapes []string
		want   []string
	}{
		{
			name: "duplicate IDs skip an existing suffix even on a later shape",
			shapes: []string{
				manualShapeForStampTest("9", `<p:nvPr/>`),
				manualShapeForStampTest("9", `<p:nvPr/>`),
				manualShapeForStampTest("9", `<p:nvPr/>`),
				manualShapeForStampTest("10", reserved),
			},
			want: []string{"shape#9", "shape#9-3", "shape#9-4", "shape#9-2"},
		},
		{
			name: "missing IDs use shape positions and disambiguate collisions",
			shapes: []string{
				manualShapeForStampTest("", `<p:nvPr/>`),
				manualShapeForStampTest("1", `<p:nvPr/>`),
				manualShapeForStampTest("", `<p:nvPr/>`),
				manualShapeForStampTest("3", `<p:nvPr/>`),
			},
			want: []string{"shape#1", "shape#1-2", "shape#3", "shape#3-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := presentationWithShapesForStampTest(t, tt.shapes...)
			original := bytes.Clone(in)
			out, err := StampShapeKeys(in)
			if err != nil {
				t.Fatalf("StampShapeKeys: %v", err)
			}
			if !bytes.Equal(in, original) {
				t.Error("StampShapeKeys mutated its input")
			}
			parts, _, err := readZipPartsFromBytes(out)
			if err != nil {
				t.Fatalf("readZipPartsFromBytes: %v", err)
			}
			shapes, _, err := parseSlideShapes(parts["ppt/slides/slide1.xml"])
			if err != nil {
				t.Fatalf("parseSlideShapes: %v", err)
			}
			var keys []string
			for _, s := range shapes {
				keys = append(keys, s.key)
			}
			if !slices.Equal(keys, tt.want) {
				t.Fatalf("shape keys = %v, want %v", keys, tt.want)
			}
			for i, s := range shapes {
				if strings.Contains(tt.shapes[i], `sk="`) && string(s.raw) != tt.shapes[i] {
					t.Errorf("existing keyed shape was changed: %s", s.raw)
				}
			}
			twice, err := StampShapeKeys(out)
			if err != nil {
				t.Fatalf("StampShapeKeys (second): %v", err)
			}
			if !bytes.Equal(out, twice) {
				t.Error("stamping twice changed the presentation bytes")
			}
		})
	}
}
