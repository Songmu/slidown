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

	// A .potx path loads directly too: PowerPoint templates share the same
	// OOXML structure as .pptx presentations for theme/master/layout parts.
	potxPath := filepath.Join(t.TempDir(), "template.potx")
	if err := copyFile(fixture, potxPath); err != nil {
		t.Fatalf("copy fixture to .potx: %v", err)
	}
	tmpl, err = resolveTemplate(potxPath)
	if err != nil {
		t.Fatalf("resolveTemplate(potx): %v", err)
	}
	if tmpl == nil || len(tmpl.Layouts) == 0 {
		t.Fatalf("expected layouts from the .potx template, got %v", tmpl)
	}

	// Case-insensitive extension matching: an upper-case .PPTX must be treated
	// as a template, not as a markdown deck.
	upperPath := filepath.Join(t.TempDir(), "TEMPLATE.PPTX")
	if err := copyFile(fixture, upperPath); err != nil {
		t.Fatalf("copy fixture to upper-case path: %v", err)
	}
	tmpl, err = resolveTemplate(upperPath)
	if err != nil {
		t.Fatalf("resolveTemplate(upper): %v", err)
	}
	if tmpl == nil || len(tmpl.Layouts) == 0 {
		t.Fatalf("expected layouts from the .PPTX template, got %v", tmpl)
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

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}
