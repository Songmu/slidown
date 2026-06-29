package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		"ppt/slideMasters/slideMaster1.xml":             slideMaster1(),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels":  masterRelsWithMedia,
		"ppt/slideLayouts/slideLayout1.xml":              slideLayout1(),
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels":   slideLayout1Rels,
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
	tmplPath := filepath.Join(t.TempDir(), "template_with_media.pptx")
	if err := os.WriteFile(tmplPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

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
		"ppt/theme/theme1.xml": theme1,
		"ppt/slideMasters/slideMaster1.xml": slideMaster1(),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": slideMaster1Rels,
		"ppt/slideLayouts/slideLayout1.xml": slideLayout1(),
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

	path := filepath.Join(t.TempDir(), "template.pptx")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	return path
}

