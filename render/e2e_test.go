package render

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Songmu/slidown/md"
)

// findSoffice locates a LibreOffice binary, if available.
func findSoffice() string {
	for _, name := range []string{"soffice", "libreoffice"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// TestE2ERendersWithLibreOffice is an opt-in visual end-to-end test: it builds a
// .pptx covering the main element types and verifies LibreOffice can open and
// convert it to a PDF. It is skipped when LibreOffice is not installed.
func TestE2ERendersWithLibreOffice(t *testing.T) {
	if os.Getenv("SLIDOWN_E2E") == "" {
		t.Skip("set SLIDOWN_E2E=1 to run the LibreOffice visual e2e test")
	}
	soffice := findSoffice()
	if soffice == "" {
		t.Skip("LibreOffice (soffice) not found; skipping visual e2e test")
	}

	const src = "# Title\n\n## Subtitle\n\n" +
		"- **bold** and *italic* and `code` and ~~strike~~\n" +
		"  - nested with a [link](https://example.com)\n\n" +
		"> a block quote\n\n" +
		"| A | B |\n| --- | ---: |\n| x | 1 |\n| y | 2 |\n\n" +
		"<!--\nspeaker note here\n-->\n"

	parsed, err := md.Parse("", []byte(src), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	dir := t.TempDir()
	pptxPath := filepath.Join(dir, "e2e.pptx")
	if err := ToPresentation(slides).WriteFile(pptxPath); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Use an isolated user profile dir to avoid clashing with a running instance.
	profileArg := "-env:UserInstallation=file://" + filepath.Join(dir, "louser")
	cmd := exec.Command(soffice, "--headless", profileArg, "--convert-to", "pdf", "--outdir", dir, pptxPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("soffice conversion failed: %v\n%s", err, out)
	}

	pdfPath := filepath.Join(dir, "e2e.pdf")
	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("expected PDF output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("PDF output is empty")
	}
}
