package pptx

import (
	"archive/zip"
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Songmu/slidown/internal/testutil"
)

func minimalTemplateParts(t *testing.T) map[string][]byte {
	t.Helper()
	parts, _, err := readZipPartsFromPath(writeMinimalTemplate(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	return parts
}

func loadTemplateParts(t *testing.T, parts map[string][]byte) *Template {
	t.Helper()
	tmpl, err := LoadTemplate(writeTempPPTX(t, testutil.WriteZIP(t, parts, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func TestLoadTemplatePackageErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadTemplate(filepath.Join(t.TempDir(), "missing.pptx"))
		if err == nil || !strings.Contains(err.Error(), "failed to open template") {
			t.Fatalf("LoadTemplate error = %v", err)
		}
	})
	t.Run("invalid zip", func(t *testing.T) {
		_, err := LoadTemplate(writeTempPPTX(t, []byte("not a ZIP")))
		if !errors.Is(err, zip.ErrFormat) {
			t.Fatalf("LoadTemplate error = %v, want ZIP format error", err)
		}
	})
	for _, tc := range []struct {
		name   string
		method uint16
		crc    uint32
		want   error
	}{
		{"unsupported compression", 99, 0, zip.ErrAlgorithm},
		{"checksum mismatch", zip.Store, 1, zip.ErrChecksum},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			w, err := zw.CreateRaw(&zip.FileHeader{
				Name: "broken.xml", Method: tc.method, CRC32: tc.crc,
				CompressedSize64: 1, UncompressedSize64: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			_, err = LoadTemplate(writeTempPPTX(t, buf.Bytes()))
			if !errors.Is(err, tc.want) {
				t.Fatalf("LoadTemplate error = %v, want %v", err, tc.want)
			}
			if tc.method == 99 && !strings.Contains(err.Error(), `failed to read template part "broken.xml"`) {
				t.Fatalf("missing part context: %v", err)
			}
		})
	}
}

func TestLoadTemplateValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string][]byte)
		want   string
	}{
		{"missing content types", func(parts map[string][]byte) {
			delete(parts, "[Content_Types].xml")
		}, "is missing [Content_Types].xml"},
		{"malformed content types", func(parts map[string][]byte) {
			parts["[Content_Types].xml"] = []byte("<Types>")
		}, "failed to parse template content types:"},
		{"missing master declaration", func(parts map[string][]byte) {
			parts["[Content_Types].xml"] = bytes.ReplaceAll(parts["[Content_Types].xml"], []byte("slideMaster+xml"), []byte("unknown+xml"))
		}, "has no slide master"},
		{"malformed layout", func(parts map[string][]byte) {
			parts["ppt/slideLayouts/slideLayout1.xml"] = []byte("<sldLayout>")
		}, "has no slide layouts"},
		{"style layout only", func(parts map[string][]byte) {
			parts["ppt/slideLayouts/slideLayout1.xml"] = []byte(`<sldLayout><cSld name="style"/></sldLayout>`)
		}, "has no slide layouts"},
		{"malformed presentation", func(parts map[string][]byte) {
			parts["ppt/presentation.xml"] = []byte(`<p:presentation xmlns:p="` + nsPresentationML + `"><p:defaultTextStyle>`)
		}, "failed to parse template presentation:"},
		{"missing style relationship", func(parts map[string][]byte) {
			addDefaultStyleReference(parts, "missing")
		}, `failed to preserve template default text style: missing relationship "missing"`},
		{"malformed style relationships", func(parts map[string][]byte) {
			addDefaultStyleReference(parts, "rId1")
			parts["ppt/_rels/presentation.xml.rels"] = []byte("<Relationships>")
		}, "failed to preserve template default text style:"},
		{"missing style dependency", func(parts map[string][]byte) {
			addDefaultStyleReference(parts, "bullet")
			parts["ppt/_rels/presentation.xml.rels"] = []byte(`<Relationships><Relationship Id="bullet" Target="media/missing.png"/></Relationships>`)
		}, `failed to preserve template default text style: missing related part "ppt/media/missing.png"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := minimalTemplateParts(t)
			tc.change(parts)
			tmpl, err := LoadTemplate(writeTempPPTX(t, testutil.WriteZIP(t, parts, nil)))
			if err == nil || !strings.Contains(err.Error(), tc.want) || tmpl != nil {
				t.Fatalf("LoadTemplate = (%v, %v), want nil and error containing %q", tmpl, err, tc.want)
			}
		})
	}
}

func addDefaultStyleReference(parts map[string][]byte, id string) {
	style := `<p:defaultTextStyle><a:lvl1pPr><a:buBlip><a:blip r:embed="` + id + `"/></a:buBlip></a:lvl1pPr></p:defaultTextStyle>`
	parts["ppt/presentation.xml"] = bytes.Replace(parts["ppt/presentation.xml"], []byte("</p:presentation>"), []byte(style+"</p:presentation>"), 1)
}

func TestLoadTemplateDefaultDeclarations(t *testing.T) {
	parts := minimalTemplateParts(t)
	declarations := `<Default Extension="EMF" ContentType="first"/>` +
		`<Default Extension="emf" ContentType="last"/>` +
		`<Default Extension="" ContentType="ignored"/>` +
		`<Default Extension="svg" ContentType=""/>`
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"], []byte("</Types>"), []byte(declarations+"</Types>"), 1)
	tmpl := loadTemplateParts(t, parts)
	if tmpl.defaultTypes["emf"] != "last" {
		t.Errorf("defaultTypes = %v, want last case-insensitive declaration", tmpl.defaultTypes)
	}
	for _, ext := range []string{"EMF", "", "svg"} {
		if _, ok := tmpl.defaultTypes[ext]; ok {
			t.Errorf("unexpected default extension %q", ext)
		}
	}
}

func TestLoadTemplateDesignOverrideOrder(t *testing.T) {
	parts := minimalTemplateParts(t)
	// Deliberately unsorted: masters and content layouts are sorted later,
	// but the last style layout and optional-part declarations must win.
	var declarations strings.Builder
	for _, entry := range []struct {
		name, kind, data string
	}{
		{"ppt/slideMasters/a.xml", "slideMaster", slideMaster1()},
		{"ppt/slideLayouts/a.xml", "slideLayout", slideLayout1()},
		{"ppt/slideLayouts/style-z.xml", "slideLayout", `<sldLayout><cSld name="style"><spTree><sp><txBody><p><r><rPr b="1"/><t>code</t></r></p></txBody></sp></spTree></cSld></sldLayout>`},
		{"ppt/slideLayouts/style-a.xml", "slideLayout", `<sldLayout><cSld name="style"><spTree><sp><txBody><p><r><rPr i="1"/><t>code</t></r></p></txBody></sp></spTree></cSld></sldLayout>`},
		{"ppt/presProps-z.xml", "presProps", "<properties/>"},
		{"ppt/presProps-a.xml", "presProps", "<properties/>"},
		{"ppt/viewProps.xml", "viewProps", "<properties/>"},
		{"ppt/tableStyles.xml", "tableStyles", "<styles/>"},
		{"ppt/theme/selected.xml", "theme", theme1},
		{"ppt/unknown.xml", "unknown", "<ignored/>"},
	} {
		declarations.WriteString(`<Override PartName="/` + entry.name + `" ContentType="test.` + entry.kind + `+xml"/>`)
		parts[entry.name] = []byte(entry.data)
		parts[relsPath(entry.name)] = []byte("<Relationships/>")
	}
	parts["ppt/slideMasters/_rels/a.xml.rels"] = []byte(`<Relationships><Relationship Target="../theme/selected.xml"/></Relationships>`)
	// An unclassified duplicate still supplies the final override content type
	// for a retained part without reclassifying or dropping it.
	declarations.WriteString(`<Override PartName="/ppt/tableStyles.xml" ContentType="final/type"/>`)
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"], []byte("</Types>"), []byte(declarations.String()+"</Types>"), 1)
	tmpl := loadTemplateParts(t, parts)
	if !slices.Equal(tmpl.masterParts, []string{"ppt/slideMasters/a.xml", "ppt/slideMasters/slideMaster1.xml"}) {
		t.Errorf("master order = %v", tmpl.masterParts)
	}
	if len(tmpl.Layouts) != 2 || tmpl.Layouts[0].PartName != "ppt/slideLayouts/a.xml" || tmpl.Layouts[1].PartName != "ppt/slideLayouts/slideLayout1.xml" {
		t.Fatalf("content layouts = %+v, want sorted layouts excluding style layouts", tmpl.Layouts)
	}
	if tmpl.themePart != "ppt/theme/selected.xml" {
		t.Errorf("theme = %q, want theme referenced by the first sorted master", tmpl.themePart)
	}
	if style := tmpl.SyntaxStyles["code"]; !style.Italic || style.Bold {
		t.Errorf("style = %+v, want last declared style layout", style)
	}
	if tmpl.presPropsPart != "ppt/presProps-a.xml" || tmpl.viewPropsPart != "ppt/viewProps.xml" || tmpl.tableStylesPart != "ppt/tableStyles.xml" {
		t.Error("lost optional part selection")
	}
	if tmpl.partTypes["ppt/tableStyles.xml"] != "final/type" {
		t.Errorf("table style content type = %q", tmpl.partTypes["ppt/tableStyles.xml"])
	}
	assertRetainedDesignParts(t, tmpl, parts, []string{
		"ppt/slideMasters/a.xml", "ppt/slideLayouts/a.xml", "ppt/theme/selected.xml",
		"ppt/slideLayouts/style-z.xml", "ppt/slideLayouts/style-a.xml",
	}, true)
	assertRetainedDesignParts(t, tmpl, parts, []string{
		"ppt/presProps-z.xml", "ppt/presProps-a.xml", "ppt/viewProps.xml", "ppt/tableStyles.xml",
	}, false)
	if _, ok := tmpl.designParts["ppt/unknown.xml"]; ok {
		t.Error("retained an unclassified design part")
	}
}

func assertRetainedDesignParts(t *testing.T, tmpl *Template, parts map[string][]byte, names []string, withRels bool) {
	t.Helper()
	for _, name := range names {
		if !bytes.Equal(tmpl.designParts[name], parts[name]) {
			t.Errorf("changed or lost design part %s", name)
		}
		if _, ok := tmpl.partTypes[name]; !ok {
			t.Errorf("lost content type for %s", name)
		}
		rels, ok := tmpl.designParts[relsPath(name)]
		if ok != withRels || (withRels && !bytes.Equal(rels, parts[relsPath(name)])) {
			t.Errorf("retained relationships for %s = %q, want copied = %v", name, rels, withRels)
		}
	}
}

func TestLoadTemplateContentTypePriority(t *testing.T) {
	parts := minimalTemplateParts(t)
	// Content types have historically been matched by substring in this order.
	// A later matching substring must not reclassify the part.
	ambiguous := "theme+xml slideMaster+xml slideLayout+xml"
	override := `<Override PartName="/ppt/theme/ambiguous.xml" ContentType="` + ambiguous + `"/>`
	parts["[Content_Types].xml"] = bytes.Replace(parts["[Content_Types].xml"], []byte("</Types>"), []byte(override+"</Types>"), 1)
	parts["ppt/theme/ambiguous.xml"] = []byte(theme1)
	tmpl := loadTemplateParts(t, parts)
	if len(tmpl.masterParts) != 1 || len(tmpl.Layouts) != 1 || tmpl.partTypes["ppt/theme/ambiguous.xml"] != ambiguous {
		t.Error("changed content type classification priority")
	}
}
