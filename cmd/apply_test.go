package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/Songmu/slidown/render"
)

func TestSlideFingerprintRoundTrip(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	mdPath := filepath.Join(t.TempDir(), "roundtrip.md")
	if err := os.WriteFile(mdPath, []byte("# Title\n\nbody\n\n---\n\n# Two\n\n- a\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m, err := md.ParseFile(mdPath, cfg)
	if err != nil {
		t.Fatalf("md.ParseFile: %v", err)
	}
	slides, err := m.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	out := filepath.Join(t.TempDir(), "deck.pptx")
	if err := render.ToPresentation(slides).WriteFile(out); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	metas, err := pptx.ReadSlideMetas(out)
	if err != nil {
		t.Fatalf("ReadSlideMetas: %v", err)
	}
	if len(metas) != len(slides) {
		t.Fatalf("fingerprint count mismatch: %d vs %d", len(metas), len(slides))
	}
	for i := range slides {
		if want := slides[i].Fingerprint(); metas[i].Fingerprint != want {
			t.Errorf("slide %d fingerprint mismatch: embedded %q, source %q", i+1, metas[i].Fingerprint, want)
		}
	}
}

func TestWritePresentationUpdatesExistingFile(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "deck.md")
	if err := os.WriteFile(mdPath, []byte("# Title\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m, err := md.ParseFile(mdPath, cfg)
	if err != nil {
		t.Fatalf("md.ParseFile: %v", err)
	}
	slides, err := m.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	var buf bytes.Buffer
	if _, err := render.ToPresentation(slides).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	out := filepath.Join(tmpDir, "deck.pptx")
	updated, err := writePresentation(out, buf.Bytes(), slides)
	if err != nil {
		t.Fatalf("writePresentation: %v", err)
	}
	if updated {
		t.Fatalf("expected first write to report new file")
	}

	rewrite, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	updated, err = writePresentation(out, buf.Bytes(), slides)
	if err != nil {
		t.Fatalf("writePresentation second time: %v", err)
	}
	if !updated {
		t.Fatalf("expected second write to report existing file")
	}
	if got, err := os.ReadFile(out); err != nil {
		t.Fatalf("ReadFile after no-op: %v", err)
	} else if !bytes.Equal(got, rewrite) {
		t.Fatalf("no-op rewrite changed file")
	}

	mdPath2 := filepath.Join(tmpDir, "deck2.md")
	if err := os.WriteFile(mdPath2, []byte("# Changed\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("WriteFile changed deck: %v", err)
	}
	m2, err := md.ParseFile(mdPath2, cfg)
	if err != nil {
		t.Fatalf("md.ParseFile changed deck: %v", err)
	}
	slides2, err := m2.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides changed deck: %v", err)
	}
	var buf2 bytes.Buffer
	if _, err := render.ToPresentation(slides2).WriteTo(&buf2); err != nil {
		t.Fatalf("WriteTo changed deck: %v", err)
	}

	updated, err = writePresentation(out, buf2.Bytes(), slides2)
	if err != nil {
		t.Fatalf("writePresentation changed deck: %v", err)
	}
	if !updated {
		t.Fatalf("expected changed deck to update existing file")
	}
	metas, err := pptx.ReadSlideMetas(out)
	if err != nil {
		t.Fatalf("ReadSlideMetas updated deck: %v", err)
	}
	if len(metas) != 1 || metas[0].Fingerprint != slides2[0].Fingerprint() {
		t.Fatalf("updated slide fingerprint not embedded: %v", metas)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	parts, _ := zipPartsForTest(t, data)
	if !strings.Contains(parts["ppt/slides/slide1.xml"], "Changed") {
		t.Fatalf("updated slide does not contain new title")
	}
}

// zipPartsForTest returns the text parts of a .pptx given its bytes.
func zipPartsForTest(t *testing.T, data []byte) (map[string]string, error) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}
	return parts, nil
}

func buildTemplateFileForTest(t *testing.T) string {
	t.Helper()
	parsed, err := md.Parse("", []byte("# Base\n\n- x\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	path := filepath.Join(t.TempDir(), "template.pptx")
	if err := render.ToPresentation(slides).WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// applyToFileForTest mirrors the apply command's template selection: an
// explicit template wins, then frontmatter, then a pre-existing output is
// reused as the design template.
func applyToFileForTest(t *testing.T, mdText, out, templatePath string) bool {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	mdPath := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(mdPath, []byte(mdText), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := md.ParseFile(mdPath, cfg)
	if err != nil {
		t.Fatalf("md.ParseFile: %v", err)
	}
	slides, err := m.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	if templatePath == "" && m.Frontmatter != nil {
		templatePath = m.Frontmatter.Template
	}
	if templatePath == "" {
		if exists, err := pathExists(out); err != nil {
			t.Fatalf("pathExists: %v", err)
		} else if exists {
			templatePath = out
		}
	}

	var pres *pptx.Presentation
	if templatePath != "" {
		tmpl, err := pptx.LoadTemplate(templatePath)
		if err != nil {
			t.Fatalf("LoadTemplate: %v", err)
		}
		pres = render.ToPresentationWithTemplate(slides, tmpl)
	} else {
		pres = render.ToPresentation(slides)
	}

	var buf bytes.Buffer
	if _, err := pres.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	updated, err := writePresentation(out, buf.Bytes(), slides)
	if err != nil {
		t.Fatalf("writePresentation: %v", err)
	}
	return updated
}

func readSlidePartsForTest(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader: %v", err)
	}
	defer zr.Close()
	parts := map[string][]byte{}
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		parts[f.Name] = b
	}
	return parts
}

func TestApplyReusesUnchangedSlides(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const deck3 = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n---\n\n# Three\n\nbody three\n"
	const deck3changed = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two changed\n\n---\n\n# Three\n\nbody three\n"

	if updated := applyToFileForTest(t, deck3, out, ""); updated {
		t.Fatalf("first apply should report a fresh write, got updated=true")
	}
	orig := readSlidePartsForTest(t, out)
	if len(orig) != 3 {
		t.Fatalf("expected 3 slide parts, got %d", len(orig))
	}

	if updated := applyToFileForTest(t, deck3changed, out, ""); !updated {
		t.Fatalf("second apply should report an update, got updated=false")
	}
	now := readSlidePartsForTest(t, out)

	s1 := "ppt/slides/slide1.xml"
	s2 := "ppt/slides/slide2.xml"
	s3 := "ppt/slides/slide3.xml"
	if !bytes.Equal(orig[s1], now[s1]) {
		t.Errorf("unchanged slide 1 was rewritten")
	}
	if !bytes.Equal(orig[s3], now[s3]) {
		t.Errorf("unchanged slide 3 was rewritten")
	}
	if bytes.Equal(orig[s2], now[s2]) {
		t.Errorf("changed slide 2 was not updated")
	}
	if !bytes.Contains(now[s2], []byte("body two changed")) {
		t.Errorf("changed slide 2 does not contain new content: %s", now[s2])
	}
}

func TestApplyKeepsFrozenSlides(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	// Slide 2 is frozen, so its content must be kept even when the source changes.
	const base = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n<!-- {\"freeze\": true} -->\n"
	const changed = "# One\n\nbody one changed\n\n---\n\n# Two\n\nbody two changed\n\n<!-- {\"freeze\": true} -->\n"

	if updated := applyToFileForTest(t, base, out, ""); updated {
		t.Fatalf("first apply should report a fresh write, got updated=true")
	}
	orig := readSlidePartsForTest(t, out)

	if updated := applyToFileForTest(t, changed, out, ""); !updated {
		t.Fatalf("second apply should report an update, got updated=false")
	}
	now := readSlidePartsForTest(t, out)

	s1 := "ppt/slides/slide1.xml"
	s2 := "ppt/slides/slide2.xml"
	if bytes.Equal(orig[s1], now[s1]) {
		t.Errorf("changed slide 1 was not updated")
	}
	if !bytes.Contains(now[s1], []byte("body one changed")) {
		t.Errorf("changed slide 1 does not contain new content: %s", now[s1])
	}
	if !bytes.Equal(orig[s2], now[s2]) {
		t.Errorf("frozen slide 2 was rewritten despite freeze")
	}
	if bytes.Contains(now[s2], []byte("body two changed")) {
		t.Errorf("frozen slide 2 picked up the changed source content")
	}
}

func TestApplyReusesKeyedSlideAcrossInsert(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# A\n\n<!-- {\"key\":\"a\",\"freeze\":true} -->\n\n---\n\n# B\n\n<!-- {\"key\":\"b\"} -->\n"
	// Insert a new slide before A and change A's markdown. A is frozen and B is
	// unchanged, so both must be reused at their new positions despite the shift.
	const v2 = "# New\n\n<!-- {\"key\":\"new\"} -->\n\n---\n\n# A CHANGED\n\n<!-- {\"key\":\"a\",\"freeze\":true} -->\n\n---\n\n# B\n\n<!-- {\"key\":\"b\"} -->\n"

	if updated := applyToFileForTest(t, v1, out, ""); updated {
		t.Fatalf("first apply should be a fresh write")
	}
	orig := readSlidePartsForTest(t, out)

	if updated := applyToFileForTest(t, v2, out, ""); !updated {
		t.Fatalf("second apply should report an update")
	}
	now := readSlidePartsForTest(t, out)

	if len(now) != 3 {
		t.Fatalf("expected 3 slides after insert, got %d", len(now))
	}
	// Frozen keyed slide A moved from position 1 to 2 and must be reused verbatim.
	if !bytes.Equal(orig["ppt/slides/slide1.xml"], now["ppt/slides/slide2.xml"]) {
		t.Errorf("frozen keyed slide A was not reused at its new position")
	}
	if bytes.Contains(now["ppt/slides/slide2.xml"], []byte("CHANGED")) {
		t.Errorf("frozen slide picked up changed content")
	}
	// Unchanged keyed slide B moved from position 2 to 3 and must be reused.
	if !bytes.Equal(orig["ppt/slides/slide2.xml"], now["ppt/slides/slide3.xml"]) {
		t.Errorf("unchanged keyed slide B was not reused at its new position")
	}
}

func TestApplyReusesUnchangedSlidesWithExplicitTemplate(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")
	templatePath := buildTemplateFileForTest(t)

	const base = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n---\n\n# Three\n\nbody three\n"
	const changed = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two changed\n\n---\n\n# Three\n\nbody three\n"

	if updated := applyToFileForTest(t, base, out, templatePath); updated {
		t.Fatalf("first apply should report a fresh write, got updated=true")
	}
	orig := readSlidePartsForTest(t, out)

	if updated := applyToFileForTest(t, changed, out, templatePath); !updated {
		t.Fatalf("second apply should report an update, got updated=false")
	}
	now := readSlidePartsForTest(t, out)

	if !bytes.Equal(orig["ppt/slides/slide1.xml"], now["ppt/slides/slide1.xml"]) {
		t.Errorf("unchanged slide 1 was rewritten with explicit template")
	}
	if !bytes.Equal(orig["ppt/slides/slide3.xml"], now["ppt/slides/slide3.xml"]) {
		t.Errorf("unchanged slide 3 was rewritten with explicit template")
	}
	if bytes.Equal(orig["ppt/slides/slide2.xml"], now["ppt/slides/slide2.xml"]) {
		t.Errorf("changed slide 2 was not updated with explicit template")
	}
}

func TestApplyKeepsFrozenSlidesWithExplicitTemplate(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")
	templatePath := buildTemplateFileForTest(t)

	const base = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n<!-- {\"freeze\": true} -->\n"
	const changed = "# One\n\nbody one changed\n\n---\n\n# Two\n\nbody two changed\n\n<!-- {\"freeze\": true} -->\n"

	if updated := applyToFileForTest(t, base, out, templatePath); updated {
		t.Fatalf("first apply should report a fresh write, got updated=true")
	}
	orig := readSlidePartsForTest(t, out)

	if updated := applyToFileForTest(t, changed, out, templatePath); !updated {
		t.Fatalf("second apply should report an update, got updated=false")
	}
	now := readSlidePartsForTest(t, out)

	if bytes.Equal(orig["ppt/slides/slide1.xml"], now["ppt/slides/slide1.xml"]) {
		t.Errorf("changed slide 1 was not updated with explicit template")
	}
	if !bytes.Equal(orig["ppt/slides/slide2.xml"], now["ppt/slides/slide2.xml"]) {
		t.Errorf("frozen slide 2 was rewritten with explicit template")
	}
}

func TestApplyKeepsUnchangedAndFrozenSlidesWithFrontmatterTemplate(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")
	templatePath := buildTemplateFileForTest(t)

	base := fmt.Sprintf("---\ntemplate: %s\n---\n\n# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n---\n\n# Three\n\nbody three\n\n<!-- {\"freeze\": true} -->\n", templatePath)
	changed := fmt.Sprintf("---\ntemplate: %s\n---\n\n# One\n\nbody one\n\n---\n\n# Two\n\nbody two changed\n\n---\n\n# Three\n\nbody three changed\n\n<!-- {\"freeze\": true} -->\n", templatePath)

	if updated := applyToFileForTest(t, base, out, ""); updated {
		t.Fatalf("first apply should report a fresh write, got updated=true")
	}
	orig := readSlidePartsForTest(t, out)

	if updated := applyToFileForTest(t, changed, out, ""); !updated {
		t.Fatalf("second apply should report an update, got updated=false")
	}
	now := readSlidePartsForTest(t, out)

	if !bytes.Equal(orig["ppt/slides/slide1.xml"], now["ppt/slides/slide1.xml"]) {
		t.Errorf("unchanged slide 1 was rewritten with frontmatter template")
	}
	if bytes.Equal(orig["ppt/slides/slide2.xml"], now["ppt/slides/slide2.xml"]) {
		t.Errorf("changed slide 2 was not updated with frontmatter template")
	}
	if !bytes.Equal(orig["ppt/slides/slide3.xml"], now["ppt/slides/slide3.xml"]) {
		t.Errorf("frozen slide 3 was rewritten with frontmatter template")
	}
}
