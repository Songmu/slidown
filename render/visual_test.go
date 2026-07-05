package render

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/corona10/goimagehash"
)

// visualGoldenCases lists the markdown fixtures whose rendered slides are
// compared against committed golden PNGs. They cover the main visual element
// types and render fully offline (no remote images, no external commands).
var visualGoldenCases = []string{
	"slide.md",
	"tables.md",
	"blockquote.md",
	"style.md",
	"nested_list.md",
	"paragraphs.md",
	"bold_and_italic.md",
	"code.md",
	"emoji.md",
	"br.md",
	"breaks_default.md",
	"breaks_enabled.md",
	"cap.md",
	"empty_list.md",
	"empty_link.md",
	"list_and_paragraph.md",
	"paragraph_and_list.md",
	"heading.md",
	"html_element_style.md",
	"autolink.md",
	"multi_subtitle.md",
	"multi_subtitle_columns.md",
	"multi_subtitle_overflow.md",
	"image_placeholder.md",
	"style_layout.md",
}

// visualTemplateOverride maps a fixture to a non-default template. Fixtures not
// listed here render through template_base.pptx. The style_layout fixture is
// rendered through template_style.pptx (which carries a "style" layout) so the
// visual goldens exercise the style-override code path end to end.
var visualTemplateOverride = map[string]string{
	"style_layout.md": "../testdata/template_style.pptx",
}

// visualHashThreshold is the maximum allowed perceptual-hash distance between a
// freshly rendered slide and its golden. A small budget tolerates minor
// anti-aliasing differences while still catching real layout regressions.
const visualHashThreshold = 8

// TestVisualGolden recreates deck's per-slide visual regression test for
// slidown: it renders each fixture to a .pptx, rasterizes every slide with
// LibreOffice + pdftoppm, and compares each page against a committed golden via
// perceptual hashing. It is skipped under `go test -short` and when the
// required tools are missing. Set UPDATE_GOLDEN=1 to (re)generate the goldens.
func TestVisualGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LibreOffice visual golden test in short mode")
	}
	soffice := findSoffice()
	if soffice == "" {
		t.Skip("LibreOffice (soffice) not found; skipping visual golden test")
	}
	pdftoppm, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm (poppler) not found; skipping visual golden test")
	}

	update := os.Getenv("UPDATE_GOLDEN") != ""

	for _, name := range visualGoldenCases {
		t.Run(name, func(t *testing.T) {
			pages := renderSlidePNGs(t, soffice, pdftoppm, name)
			if len(pages) == 0 {
				t.Fatalf("%s produced no rendered pages", name)
			}
			for i, got := range pages {
				page := i + 1
				goldenPath := filepath.Join("..", "testdata", fmt.Sprintf("%s-%d.golden.png", name, page))
				if update {
					if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
						t.Fatalf("write golden %s: %v", goldenPath, err)
					}
					continue
				}
				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("read golden %s (run with UPDATE_GOLDEN=1 to create): %v", goldenPath, err)
				}
				dist, err := perceptualDistance(got, want)
				if err != nil {
					t.Fatalf("compare %s page %d: %v", name, page, err)
				}
				if dist > visualHashThreshold {
					diffPath := filepath.Join("..", "testdata", fmt.Sprintf("%s-%d.diff.png", name, page))
					_ = os.WriteFile(diffPath, got, 0o600)
					t.Errorf("%s page %d differs from golden: distance %d > %d (see %s)",
						name, page, dist, visualHashThreshold, diffPath)
				}
			}
		})
	}
}

// renderSlidePNGs builds the fixture into a .pptx (rendered through the deck
// template fixture so the visual goldens exercise the external-template code
// path) and rasterizes each slide to a PNG, returning the per-page PNG bytes
// in slide order.
func renderSlidePNGs(t *testing.T, soffice, pdftoppm, name string) [][]byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	parsed, err := md.Parse(filepath.Join("..", "testdata"), src, nil)
	if err != nil {
		t.Fatalf("md.Parse %s: %v", name, err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides %s: %v", name, err)
	}

	const templateFixture = "../testdata/template_base.pptx"
	templatePath := templateFixture
	if override, ok := visualTemplateOverride[name]; ok {
		templatePath = override
	}
	tmpl, err := pptx.LoadTemplate(templatePath)
	if err != nil {
		t.Fatalf("LoadTemplate %s: %v", templatePath, err)
	}

	dir := t.TempDir()
	pptxPath := filepath.Join(dir, "out.pptx")
	if err := ToPresentationWithTemplate(slides, tmpl).WriteFile(pptxPath); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}

	profileArg := "-env:UserInstallation=file://" + filepath.Join(dir, "louser")
	cmd := exec.Command(soffice, "--headless", profileArg, "--convert-to", "pdf", "--outdir", dir, pptxPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("soffice conversion failed for %s: %v\n%s", name, err, out)
	}
	pdfPath := filepath.Join(dir, "out.pdf")

	// Rasterize each page to PNG. -scale-to bounds the longest side so goldens
	// stay small; perceptual hashing is resolution-independent anyway.
	prefix := filepath.Join(dir, "page")
	cmd = exec.Command(pdftoppm, "-png", "-scale-to", "512", pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm failed for %s: %v\n%s", name, err, out)
	}

	matches, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		t.Fatalf("glob pages for %s: %v", name, err)
	}
	sort.Slice(matches, func(i, j int) bool {
		return pageIndex(matches[i]) < pageIndex(matches[j])
	})
	var pages [][]byte
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("read page %s: %v", m, err)
		}
		pages = append(pages, b)
	}
	return pages
}

func pageIndex(path string) int {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".png")
	if i := strings.LastIndex(base, "-"); i >= 0 {
		n, _ := strconv.Atoi(base[i+1:])
		return n
	}
	return 0
}

func perceptualDistance(gotPNG, wantPNG []byte) (int, error) {
	gotImg, err := png.Decode(bytes.NewReader(gotPNG))
	if err != nil {
		return 0, fmt.Errorf("decode rendered png: %w", err)
	}
	wantImg, err := png.Decode(bytes.NewReader(wantPNG))
	if err != nil {
		return 0, fmt.Errorf("decode golden png: %w", err)
	}
	gotHash, err := goimagehash.PerceptionHash(gotImg)
	if err != nil {
		return 0, err
	}
	wantHash, err := goimagehash.PerceptionHash(wantImg)
	if err != nil {
		return 0, err
	}
	return gotHash.Distance(wantHash)
}
