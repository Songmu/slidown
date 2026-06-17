package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTemplate(t *testing.T) {
	const fixture = "../testdata/template_base.pptx"
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("template fixture missing: %v", err)
	}

	// A .pptx path loads directly.
	tmpl, err := resolveTemplate(fixture)
	if err != nil {
		t.Fatalf("resolveTemplate(pptx): %v", err)
	}
	if tmpl == nil || len(tmpl.Layouts) == 0 {
		t.Fatalf("expected layouts from the external template, got %v", tmpl)
	}

	// A markdown deck with no template resolves to the built-in design (nil).
	mdPath := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(mdPath, []byte("# Hello\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	tmpl, err = resolveTemplate(mdPath)
	if err != nil {
		t.Fatalf("resolveTemplate(md): %v", err)
	}
	if tmpl != nil {
		t.Fatalf("expected nil template for a deck without a template, got %v", tmpl)
	}

	// A markdown deck pointing at a template resolves it.
	mdPath2 := filepath.Join(t.TempDir(), "deck2.md")
	absFixture, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	body := "---\ntemplate: " + absFixture + "\n---\n\n# Hello\n\nbody\n"
	if err := os.WriteFile(mdPath2, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	tmpl, err = resolveTemplate(mdPath2)
	if err != nil {
		t.Fatalf("resolveTemplate(md+template): %v", err)
	}
	if tmpl == nil || len(tmpl.Layouts) == 0 {
		t.Fatalf("expected layouts from the deck's configured template, got %v", tmpl)
	}
}
