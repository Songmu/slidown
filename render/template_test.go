package render

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
)

// buildTemplateFile generates a .pptx with the built-in design and returns its
// path, to be reused as a template in tests.
func buildTemplateFile(t *testing.T) string {
	t.Helper()
	parsed, err := md.Parse("", []byte("# Base\n\n- x\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "template.pptx")
	if err := ToPresentation(slides).WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestToPresentationWithTemplate(t *testing.T) {
	tmplPath := buildTemplateFile(t)
	tmpl, err := pptx.LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if len(tmpl.Layouts) == 0 {
		t.Fatalf("template has no layouts")
	}

	parsed, err := md.Parse("", []byte("# Title\n\n## Sub\n\n- a\n- **b**\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}

	// Template design parts must be carried over.
	for _, name := range []string{
		"ppt/theme/theme1.xml",
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slides/slide1.xml",
		"ppt/slides/_rels/slide1.xml.rels",
		"[Content_Types].xml",
		"ppt/presentation.xml",
	} {
		if _, ok := parts[name]; !ok {
			t.Errorf("output missing %q", name)
		}
	}
	// The slide must reference the template layout and carry content.
	if !strings.Contains(parts["ppt/slides/_rels/slide1.xml.rels"], "slideLayout1.xml") {
		t.Errorf("slide does not reference template layout")
	}
	if !strings.Contains(parts["ppt/slides/slide1.xml"], "<a:t>Title</a:t>") {
		t.Errorf("slide missing title text")
	}
	if !strings.Contains(parts["ppt/slides/slide1.xml"], "<p:ph") {
		t.Errorf("slide has no placeholder shapes")
	}

	_ = os.Remove(tmplPath)
}

// TestExternalTemplateFixture loads a committed .pptx that was authored by an
// external tool (LibreOffice) rather than slidown, exercising the template
// loader against foreign OOXML. The fixture lets this run in CI without
// LibreOffice installed.
func TestExternalTemplateFixture(t *testing.T) {
	const fixture = "../testdata/template_base.pptx"
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("template fixture missing: %v", err)
	}

	tmpl, err := pptx.LoadTemplate(fixture)
	if err != nil {
		t.Fatalf("LoadTemplate(%s): %v", fixture, err)
	}
	if len(tmpl.Layouts) == 0 {
		t.Fatalf("external template has no layouts")
	}

	parsed, err := md.Parse("", []byte("# External Title\n\n## External Sub\n\n- one\n- two\n"), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	// The output must be a valid package that slidown can read back, and the
	// design parts must originate from the external template (byte-identical
	// theme), proving the template was actually applied.
	out, err := os.CreateTemp(t.TempDir(), "*.pptx")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := out.Write(buf.Bytes()); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	out.Close()
	if _, err := pptx.LoadTemplate(out.Name()); err != nil {
		t.Fatalf("output is not a loadable pptx: %v", err)
	}

	parts := zipParts(t, buf.Bytes())
	fixtureParts := zipPartsFromFile(t, fixture)
	if got, want := parts["ppt/theme/theme1.xml"], fixtureParts["ppt/theme/theme1.xml"]; !bytes.Equal(got, want) {
		t.Errorf("output theme was not inherited from the external template")
	}
	if !bytes.Contains(parts["ppt/slides/slide1.xml"], []byte("<a:t>External Title</a:t>")) {
		t.Errorf("slide is missing the rendered title content")
	}
}

func zipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
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

func zipPartsFromFile(t *testing.T, path string) map[string][]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return zipParts(t, b)
}
