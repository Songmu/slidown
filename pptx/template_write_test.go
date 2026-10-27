package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestTemplatePreservesTextStyleInheritance(t *testing.T) {
	const drawingNS = "http://schemas.openxmlformats.org/drawingml/2006/main"
	const inner = `<d:lvl1pPr marL="987654" indent="-123456"><d:defRPr><d:latin typeface="Custom Font"/></d:defRPr></d:lvl1pPr>` +
		`<d:extLst><d:ext uri="test"><x:marker/></d:ext></d:extLst>`
	for _, tc := range []struct {
		name  string
		style string
	}{
		{"inherited namespaces", `<q:defaultTextStyle>` + inner + `</q:defaultTextStyle>`},
		{"local namespace", `<q:defaultTextStyle xmlns:d="` + drawingNS + `">` + inner + `</q:defaultTextStyle>`},
		{"empty", `<q:defaultTextStyle/>`},
		{"absent", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts, order, err := readZipPartsFromPath(writeMinimalTemplate(t, nil))
			if err != nil {
				t.Fatal(err)
			}
			parts["ppt/presentation.xml"] = []byte(`<q:presentation xmlns:q="` + nsPresentationML +
				`" xmlns:d="` + drawingNS + `" xmlns:x="urn:custom-text-style">` +
				`<q:sldSz cx="9144000" cy="6858000"/>` + tc.style + `</q:presentation>`)
			data, err := zipFromParts(order, parts)
			if err != nil {
				t.Fatal(err)
			}
			tmpl, err := LoadTemplate(writeTempPPTX(t, data))
			if err != nil {
				t.Fatal(err)
			}
			p := samplePresentation()
			p.Template = tmpl
			got := writeToParts(t, p)
			var pres struct {
				Default *struct {
					Inner string `xml:",innerxml"`
					Level struct {
						Margin int `xml:"marL,attr"`
						Indent int `xml:"indent,attr"`
					} `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl1pPr"`
				} `xml:"http://schemas.openxmlformats.org/presentationml/2006/main defaultTextStyle"`
			}
			if err := xml.Unmarshal([]byte(got["ppt/presentation.xml"]), &pres); err != nil {
				t.Fatal(err)
			}
			if tc.style == "" {
				if pres.Default != nil {
					t.Fatal("added a default text style to a template without one")
				}
			} else if pres.Default == nil {
				t.Fatal("lost the presentation default text style or its namespace")
			} else if tc.name != "empty" {
				if pres.Default.Inner != inner || pres.Default.Level.Margin != 987654 || pres.Default.Level.Indent != -123456 {
					t.Fatalf("changed default text style: %+v", pres.Default)
				}
				dec := xml.NewDecoder(strings.NewReader(got["ppt/presentation.xml"]))
				for {
					tok, err := dec.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if elem, ok := tok.(xml.StartElement); ok && elem.Name.Local == "marker" && elem.Name.Space != "urn:custom-text-style" {
						t.Fatalf("lost extension namespace: %+v", elem.Name)
					}
				}
			}
			for _, name := range []string{"ppt/slideMasters/slideMaster1.xml", "ppt/slideLayouts/slideLayout1.xml"} {
				if got[name] != string(parts[name]) {
					t.Errorf("changed inherited design part %s", name)
				}
			}
			slide := got["ppt/slides/slide1.xml"]
			if strings.Contains(slide, ` marL=`) || strings.Contains(slide, ` indent=`) {
				t.Error("slide overrides inherited indentation")
			}
			if !strings.Contains(slide, `<a:buAutoNum type="arabicPeriod"/>`) || !strings.Contains(slide, `<a:buChar`) {
				t.Error("lost explicit Markdown bullet/number types")
			}
		})
	}
}

func TestTemplatePreservesDefaultStylePictureBullets(t *testing.T) {
	parts, order, err := readZipPartsFromPath(writeMinimalTemplate(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	style := `<p:defaultTextStyle xmlns:s="` + nsRelationships + `">` +
		`<a:lvl1pPr><a:buBlip><a:blip s:embed='rId1'/></a:buBlip></a:lvl1pPr>` +
		`<a:lvl2pPr><a:buBlip><a:blip s:embed="rId1"/></a:buBlip></a:lvl2pPr>` +
		`<a:lvl3pPr><a:buBlip><a:blip s:link="rId900"/></a:buBlip></a:lvl3pPr>` +
		`</p:defaultTextStyle>`
	pres := strings.ReplaceAll(string(parts["ppt/presentation.xml"]), `r:id="rId1"`, `r:id="rId7"`)
	parts["ppt/presentation.xml"] = []byte(strings.Replace(pres, `</p:presentation>`, style+`</p:presentation>`, 1))
	parts["ppt/_rels/presentation.xml.rels"] = []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId7" Type="` + nsRelationships + `/slideMaster" Target="slideMasters/slideMaster1.xml"/>` +
		`<Relationship Id="rId1" Type="` + nsRelationships + `/image" Target="/ppt/media/bullet.png"/>` +
		`<Relationship Id="rId900" Type="` + nsRelationships + `/image" Target="https://example.com/bullet.png?a=1&amp;b=2" TargetMode="External"/>` +
		`<Relationship Id="rId42" Type="` + nsRelationships + `/image" Target="media/unused.png"/>` +
		`</Relationships>`)
	parts["ppt/media/bullet.png"] = []byte("picture-bullet-image")
	parts["ppt/media/unused.png"] = []byte("unused-image")
	parts["[Content_Types].xml"] = []byte(strings.Replace(string(parts["[Content_Types].xml"]), `</Types>`,
		`<Override PartName="/ppt/media/bullet.png" ContentType="image/png"/></Types>`, 1))
	order = append(order, "ppt/media/bullet.png", "ppt/media/unused.png")
	data, err := zipFromParts(order, parts)
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		tmpl, err := LoadTemplate(writeTempPPTX(t, data))
		if err != nil {
			t.Fatal(err)
		}
		p := samplePresentation()
		p.Template = tmpl
		got := writeToParts(t, p)
		if got["ppt/media/bullet.png"] != "picture-bullet-image" {
			t.Fatal("lost the image referenced only by the default text style")
		}
		if _, ok := got["ppt/media/unused.png"]; ok {
			t.Error("preserved an image not referenced by the retained design")
		}
		if !strings.Contains(got["[Content_Types].xml"], `<Override PartName="/ppt/media/bullet.png" ContentType="image/png"/>`) {
			t.Error("lost the picture bullet's content type override")
		}
		var pres struct {
			Blips []struct {
				Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
				Link  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr"`
			} `xml:"defaultTextStyle>lvl1pPr>buBlip>blip"`
			Second struct {
				Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
			} `xml:"defaultTextStyle>lvl2pPr>buBlip>blip"`
			External struct {
				Link string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr"`
			} `xml:"defaultTextStyle>lvl3pPr>buBlip>blip"`
		}
		if err := xml.Unmarshal([]byte(got["ppt/presentation.xml"]), &pres); err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Items []struct {
				ID         string `xml:"Id,attr"`
				Target     string `xml:"Target,attr"`
				Type       string `xml:"Type,attr"`
				TargetMode string `xml:"TargetMode,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal([]byte(got["ppt/_rels/presentation.xml.rels"]), &parsed); err != nil {
			t.Fatal(err)
		}
		if len(pres.Blips) != 1 || pres.Blips[0].Embed == "" || pres.Second.Embed != pres.Blips[0].Embed || pres.External.Link == "" {
			t.Fatalf("lost or inconsistently remapped picture bullet references: %+v", pres)
		}
		seen := map[string]bool{}
		for _, rel := range parsed.Items {
			if seen[rel.ID] {
				t.Fatalf("duplicate presentation relationship ID %q", rel.ID)
			}
			seen[rel.ID] = true
			switch rel.ID {
			case pres.Blips[0].Embed:
				if rel.Target != "/ppt/media/bullet.png" || rel.Type != nsRelationships+"/image" {
					t.Errorf("picture bullet points at the wrong relationship: %+v", rel)
				}
			case pres.External.Link:
				if rel.Target != "https://example.com/bullet.png?a=1&b=2" || rel.TargetMode != "External" {
					t.Errorf("changed external picture bullet relationship: %+v", rel)
				}
			}
		}
		if !seen[pres.Blips[0].Embed] || !seen[pres.External.Link] {
			t.Fatal("missing picture bullet relationships")
		}
		var buf bytes.Buffer
		if _, err := p.WriteTo(&buf); err != nil {
			t.Fatal(err)
		}
		data = buf.Bytes()
	}
}

// TestContentTypesPreservesTemplateDefaults guards against regressing the bug
// where templates carrying non-standard media (emf, svg, wdp, ...) produced an
// output package whose [Content_Types].xml lacked the matching Default entries.
// PowerPoint flagged the package for repair because the verbatim-copied media
// parts had no declared content type.
func TestContentTypesPreservesTemplateDefaults(t *testing.T) {
	tmpl := &Template{
		designParts: map[string][]byte{},
		partTypes:   map[string]string{},
		defaultTypes: map[string]string{
			"emf": "image/x-emf",
			"svg": "image/svg+xml",
			"wdp": "image/vnd.ms-photo",
			// Already covered by the baseline; must not be duplicated.
			"png": "image/png",
		},
	}
	out := tmpl.contentTypesXML(1, nil)

	for _, ext := range []string{"emf", "svg", "wdp"} {
		needle := `<Default Extension="` + ext + `"`
		if c := strings.Count(out, needle); c != 1 {
			t.Errorf("expected exactly one Default for %q, got %d in:\n%s", ext, c, out)
		}
	}
	if c := strings.Count(out, `<Default Extension="png"`); c != 1 {
		t.Errorf("png Default duplicated: got %d entries", c)
	}
}

// TestLoadTemplateReadsDefaults verifies the loader extracts every Default
// extension declaration from the template's [Content_Types].xml so that
// contentTypesXML can re-emit them on write.
func TestLoadTemplateReadsDefaults(t *testing.T) {
	path := writeMinimalTemplate(t, map[string]string{
		"emf": "image/x-emf",
		"svg": "image/svg+xml",
	})
	tmpl, err := LoadTemplate(path)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	for ext, want := range map[string]string{
		"emf": "image/x-emf",
		"svg": "image/svg+xml",
	} {
		if got := tmpl.defaultTypes[ext]; got != want {
			t.Errorf("defaultTypes[%q] = %q, want %q", ext, got, want)
		}
	}
}

// TestLoadTemplateGCsOrphanedMedia verifies that LoadTemplate only retains
// ppt/media/* files that are directly referenced by a design-part .rels file
// (master, layout, or theme). Media from previous slide content that happens
// to be present in the zip (e.g. when reusing an existing .pptx as a template)
// must be discarded so it does not accumulate across incremental rebuilds.
func TestLoadTemplateGCsOrphanedMedia(t *testing.T) {
	// Build a slide master .rels that references one design image.
	masterRelsWithMedia := xmlDecl +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/design1.png"/>` +
		`</Relationships>`

	contentTypes := xmlDecl +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="png" ContentType="image/png"/>` +
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		`</Types>`

	textParts := map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels": xmlDecl +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
			`</Relationships>`,
		"ppt/presentation.xml": xmlDecl +
			`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
			`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
			`<p:sldSz cx="9144000" cy="6858000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": xmlDecl +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>` +
			`</Relationships>`,
		"ppt/theme/theme1.xml":                         theme1,
		"ppt/slideMasters/slideMaster1.xml":            slideMaster1(),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": masterRelsWithMedia,
		"ppt/slideLayouts/slideLayout1.xml":            slideLayout1(),
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": slideLayout1Rels,
	}
	binParts := map[string][]byte{
		// Referenced by the master .rels → must be preserved.
		"ppt/media/design1.png": []byte("design-image-bytes"),
		// Not referenced by any design part (simulates a slide image from a
		// previous build) → must be discarded.
		"ppt/media/slide1.png": []byte("old-slide-image-bytes"),
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range textParts {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := io.WriteString(fw, data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	for name, data := range binParts {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	tmplPath := writeTempPPTX(t, buf.Bytes())

	tmpl, err := LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}

	// The design image (referenced by master .rels) must survive.
	if _, ok := tmpl.designParts["ppt/media/design1.png"]; !ok {
		t.Error("design1.png should be preserved in designParts (referenced by master .rels)")
	}
	// The orphaned slide image must not survive.
	if _, ok := tmpl.designParts["ppt/media/slide1.png"]; ok {
		t.Error("slide1.png should be GC'd from designParts (not referenced by any design .rels)")
	}

	// Exactly one media file should remain.
	var mediaCount int
	for name := range tmpl.designParts {
		if strings.HasPrefix(name, "ppt/media/") {
			mediaCount++
		}
	}
	if mediaCount != 1 {
		t.Errorf("expected 1 media file in designParts, got %d", mediaCount)
	}
}

// [Content_Types].xml declares the given extra Default extensions in addition
// to the spec-required ones. It returns the file path.
func writeMinimalTemplate(t *testing.T, extras map[string]string) string {
	t.Helper()
	defaults := map[string]string{
		"rels": "application/vnd.openxmlformats-package.relationships+xml",
		"xml":  "application/xml",
	}
	for k, v := range extras {
		defaults[k] = v
	}
	var sb strings.Builder
	sb.WriteString(xmlDecl)
	sb.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	for ext, ct := range defaults {
		sb.WriteString(`<Default Extension="` + ext + `" ContentType="` + ct + `"/>`)
	}
	sb.WriteString(`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	sb.WriteString(`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`)
	sb.WriteString(`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`)
	sb.WriteString(`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	sb.WriteString(`</Types>`)
	contentTypes := sb.String()

	parts := map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels": xmlDecl +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
			`</Relationships>`,
		"ppt/presentation.xml": xmlDecl +
			`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
			`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
			`<p:sldSz cx="9144000" cy="6858000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": xmlDecl +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>` +
			`</Relationships>`,
		"ppt/theme/theme1.xml":                         theme1,
		"ppt/slideMasters/slideMaster1.xml":            slideMaster1(),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": slideMaster1Rels,
		"ppt/slideLayouts/slideLayout1.xml":            slideLayout1(),
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": slideLayout1Rels,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range parts {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := io.WriteString(fw, data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	return writeTempPPTX(t, buf.Bytes())
}
