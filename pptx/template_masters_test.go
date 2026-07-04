package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"path"
	"regexp"
	"strings"
	"testing"
)

// TestTemplateCarriesNotesAndHandoutMasters guards against the bug where a
// template's notes/handout masters (and the themes they reference) were dropped
// from the generated package. Their themes were still declared in
// [Content_Types].xml but referenced by nothing, so PowerPoint reported the
// package as needing repair and stripped the orphan parts.
//
// The synthetic template wires slideMaster1 -> theme2, notesMaster1 -> theme1
// and handoutMaster1 -> theme3, so it also exercises deriving the presentation
// theme from the slide master's rels (not merely the first theme override).
func TestTemplateCarriesNotesAndHandoutMasters(t *testing.T) {
	tmplPath := writeMastersTemplate(t)
	tmpl, err := LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}

	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{Placeholder: PlaceholderTitle, IsPlaceholder: true, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: "hi"}}}}})
	p.Template = tmpl

	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts := unzipToMap(t, buf.Bytes())

	// Notes and handout masters (and their rels) must be carried over.
	for _, name := range []string{
		"ppt/notesMasters/notesMaster1.xml",
		"ppt/handoutMasters/handoutMaster1.xml",
	} {
		if _, ok := parts[name]; !ok {
			t.Errorf("expected %q to be carried into the package", name)
		}
	}

	// The loose slidownMeta part must be gone (moved into presentation.xml).
	if _, ok := parts["ppt/slidownMeta"]; ok {
		t.Error("ppt/slidownMeta loose part should no longer be written")
	}

	// No theme may be orphaned: every theme part must be referenced by some
	// relationship.
	referenced := referencedInternalTargets(parts)
	for name := range parts {
		if regexp.MustCompile(`^ppt/theme/theme\d+\.xml$`).MatchString(name) {
			if !referenced[name] {
				t.Errorf("theme %q is orphaned (declared but unreferenced)", name)
			}
		}
	}

	// Every part must be covered by [Content_Types].xml (Default or Override).
	for _, name := range undeclaredParts(parts) {
		t.Errorf("part %q has no content type declared in [Content_Types].xml", name)
	}

	// The presentation theme relationship must point at the slide master's theme
	// (theme2), not the first theme override (theme1).
	presRels := string(parts["ppt/_rels/presentation.xml.rels"])
	if !strings.Contains(presRels, `Target="theme/theme2.xml"`) {
		t.Errorf("presentation theme relationship should target theme2.xml; rels:\n%s", presRels)
	}
	// The presentation must reference the notes and handout masters.
	for _, needle := range []string{"notesMasters/notesMaster1.xml", "handoutMasters/handoutMaster1.xml"} {
		if !strings.Contains(presRels, needle) {
			t.Errorf("presentation.xml.rels missing reference to %q", needle)
		}
	}
}

// referencedInternalTargets returns the set of internal part paths referenced by
// any .rels part in parts.
func referencedInternalTargets(parts map[string][]byte) map[string]bool {
	ref := make(map[string]bool)
	for name, data := range parts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		base := path.Dir(path.Dir(name)) // strip _rels/<file>.rels
		for _, tgt := range relTargets(data) {
			ref[path.Clean(path.Join(base, tgt))] = true
		}
	}
	return ref
}

// undeclaredParts returns parts that are not covered by a Default extension or
// an Override in [Content_Types].xml.
func undeclaredParts(parts map[string][]byte) []string {
	ct := string(parts["[Content_Types].xml"])
	defaults := map[string]bool{}
	for _, m := range regexp.MustCompile(`Default Extension="([^"]+)"`).FindAllStringSubmatch(ct, -1) {
		defaults[strings.ToLower(m[1])] = true
	}
	overrides := map[string]bool{}
	for _, m := range regexp.MustCompile(`Override PartName="(/[^"]+)"`).FindAllStringSubmatch(ct, -1) {
		overrides[m[1]] = true
	}
	var out []string
	for name := range parts {
		if name == "[Content_Types].xml" {
			continue
		}
		if overrides["/"+name] {
			continue
		}
		ext := ""
		if i := strings.LastIndex(name, "."); i >= 0 {
			ext = strings.ToLower(name[i+1:])
		}
		if defaults[ext] {
			continue
		}
		out = append(out, name)
	}
	return out
}

func unzipToMap(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = b
	}
	return parts
}

// writeMastersTemplate builds a template .pptx carrying slide/notes/handout
// masters and three themes, wired so slideMaster1->theme2, notesMaster1->theme1
// and handoutMaster1->theme3.
func writeMastersTemplate(t *testing.T) string {
	t.Helper()
	rel := func(id, typ, target string) string {
		return `<Relationship Id="` + id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + typ + `" Target="` + target + `"/>`
	}
	relsWrap := func(inner string) string {
		return xmlDecl + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + inner + `</Relationships>`
	}
	minimalMaster := func(tag string) string {
		return xmlDecl + `<p:` + tag + ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
			`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld></p:` + tag + `>`
	}

	ov := func(part, ct string) string {
		return `<Override PartName="/` + part + `" ContentType="` + ct + `"/>`
	}
	contentTypes := xmlDecl +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		ov("ppt/presentation.xml", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml") +
		ov("ppt/slideMasters/slideMaster1.xml", "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml") +
		ov("ppt/notesMasters/notesMaster1.xml", "application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml") +
		ov("ppt/handoutMasters/handoutMaster1.xml", "application/vnd.openxmlformats-officedocument.presentationml.handoutMaster+xml") +
		ov("ppt/slideLayouts/slideLayout1.xml", "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml") +
		ov("ppt/theme/theme1.xml", "application/vnd.openxmlformats-officedocument.theme+xml") +
		ov("ppt/theme/theme2.xml", "application/vnd.openxmlformats-officedocument.theme+xml") +
		ov("ppt/theme/theme3.xml", "application/vnd.openxmlformats-officedocument.theme+xml") +
		`</Types>`

	parts := map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         relsWrap(rel("rId1", "officeDocument", "ppt/presentation.xml")),
		"ppt/presentation.xml": xmlDecl +
			`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
			`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
			`<p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst>` +
			`<p:handoutMasterIdLst><p:handoutMasterId r:id="rId3"/></p:handoutMasterIdLst>` +
			`<p:sldSz cx="12192000" cy="6858000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": relsWrap(
			rel("rId1", "slideMaster", "slideMasters/slideMaster1.xml") +
				rel("rId2", "notesMaster", "notesMasters/notesMaster1.xml") +
				rel("rId3", "handoutMaster", "handoutMasters/handoutMaster1.xml") +
				rel("rId4", "theme", "theme/theme2.xml")),
		"ppt/theme/theme1.xml":              theme1,
		"ppt/theme/theme2.xml":              theme1,
		"ppt/theme/theme3.xml":              theme1,
		"ppt/slideMasters/slideMaster1.xml": minimalMaster("sldMaster"),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": relsWrap(
			rel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml") +
				rel("rId2", "theme", "../theme/theme2.xml")),
		"ppt/notesMasters/notesMaster1.xml": minimalMaster("notesMaster"),
		"ppt/notesMasters/_rels/notesMaster1.xml.rels": relsWrap(
			rel("rId1", "theme", "../theme/theme1.xml")),
		"ppt/handoutMasters/handoutMaster1.xml": minimalMaster("handoutMaster"),
		"ppt/handoutMasters/_rels/handoutMaster1.xml.rels": relsWrap(
			rel("rId1", "theme", "../theme/theme3.xml")),
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
