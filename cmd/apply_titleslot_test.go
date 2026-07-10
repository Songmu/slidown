package cmd

import (
	"archive/zip"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// layoutNameByTitleForTest returns a map from each slide's first text run (used
// here as a stand-in for its title) to the human name (cSld/@name) of the
// layout that slide references, resolving the slide -> layout relationship
// through the package's _rels. Layout part numbers are renumbered per output
// package, so tests must compare resolved layout names, not part paths.
func layoutNameByTitleForTest(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader: %v", err)
	}
	defer zr.Close()
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		parts[f.Name] = string(b)
	}
	nameRe := regexp.MustCompile(`<p:cSld name="([^"]*)"`)
	textRe := regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
	out := map[string]string{}
	for name, xmlStr := range parts {
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		base := strings.TrimPrefix(name, "ppt/slides/")
		rels := parts["ppt/slides/_rels/"+base+".rels"]
		i := strings.Index(rels, "slideLayouts/")
		if i < 0 {
			continue
		}
		target := rels[i:]
		target = target[:strings.IndexByte(target, '"')]
		layoutXML := parts["ppt/slideLayouts/"+strings.TrimPrefix(target, "slideLayouts/")]
		layoutName := target
		if m := nameRe.FindStringSubmatch(layoutXML); m != nil {
			layoutName = m[1]
		}
		title := ""
		if m := textRe.FindStringSubmatch(xmlStr); m != nil {
			title = m[1]
		}
		out[title] = layoutName
	}
	return out
}

// TestApplyReRendersImplicitFirstSlideLayoutOnReposition is the regression test
// for the bug where a slide that occupied the title-layout slot only by the
// built-in first-slide default kept its stale title layout after another slide
// was inserted ahead of it. Because that implicit designation is now folded
// into the content fingerprint, moving out of the first position forces a
// re-render onto the content layout.
func TestApplyReRendersImplicitFirstSlideLayoutOnReposition(t *testing.T) {
	tmpl := "../testdata/template_base.pptx"

	// v1: a single slide with no explicit layout -> it takes the title layout.
	const v1 = "# Alpha\n\n- a\n"
	// v2: prepend a new slide so Alpha is no longer first and must switch to the
	// content layout.
	const v2 = "# Zero\n\n- z\n\n---\n\n# Alpha\n\n- a\n"

	// Learn the expected content-slot layout for Alpha from a clean generation.
	fresh := filepath.Join(t.TempDir(), "fresh.pptx")
	applyFreshForTest(t, v2, fresh, tmpl)
	wantAlpha := layoutNameByTitleForTest(t, fresh)["Alpha"]

	out := filepath.Join(t.TempDir(), "deck.pptx")
	applyFreshForTest(t, v1, out, tmpl)
	firstLayout := layoutNameByTitleForTest(t, out)["Alpha"]

	applyUpdateForTest(t, v2, out, tmpl)
	got := layoutNameByTitleForTest(t, out)

	if got["Alpha"] == firstLayout {
		t.Errorf("Alpha kept its stale first-slide layout %q after being repositioned", firstLayout)
	}
	if got["Alpha"] != wantAlpha {
		t.Errorf("Alpha layout after reposition = %q, want %q (matching a fresh build)", got["Alpha"], wantAlpha)
	}
}
