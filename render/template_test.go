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
