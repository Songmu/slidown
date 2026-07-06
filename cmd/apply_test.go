package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/Songmu/slidown/render"
)

const (
	testManualFirstMarker  = "MANUAL_FIRST"
	testManualSecondMarker = "MANUAL_SECOND"
)

type testPresentation struct {
	SlideIDs []struct {
		RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	} `xml:"sldIdLst>sldId"`
}

func TestSlideFingerprintRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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
	updated, err := writePresentation(out, buf.Bytes(), slides, "")
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

	updated, err = writePresentation(out, buf.Bytes(), slides, "")
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

	updated, err = writePresentation(out, buf2.Bytes(), slides2, "")
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
	templatePath := filepath.Join(t.TempDir(), "template.pptx")
	if err := render.ToPresentation(slides).WriteFile(templatePath); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return templatePath
}

// applyToFileForTest mirrors apply's template selection via resolveApplyTemplate.
// The templatePath argument is treated like the --template flag: it only seeds a
// newly created output file. When the output already exists, this helper clears
// the flag so the update reuses the existing output as its own template (the real
// CLI would reject --template in that situation).
func applyToFileForTest(t *testing.T, mdText, out, templatePath string) bool {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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

	// The passed template is only a seed for a new file; on update it is ignored
	// and the existing output serves as its own template.
	flagTemplate := templatePath
	if exists, err := pathExists(out); err != nil {
		t.Fatalf("pathExists: %v", err)
	} else if exists {
		flagTemplate = ""
	}
	resolved, err := resolveApplyTemplate(out, flagTemplate, "")
	if err != nil {
		t.Fatalf("resolveApplyTemplate: %v", err)
	}

	var pres *pptx.Presentation
	if resolved != "" {
		tmpl, err := pptx.LoadTemplate(resolved)
		if err != nil {
			t.Fatalf("LoadTemplate: %v", err)
		}
		pres = render.ToPresentationWithTemplate(slides, tmpl)
	} else {
		pres = render.ToPresentation(slides)
	}
	if m.Frontmatter != nil {
		pres.Title = m.Frontmatter.Title
	}

	var buf bytes.Buffer
	if _, err := pres.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	updated, err := writePresentation(out, buf.Bytes(), slides, pres.Title)
	if err != nil {
		t.Fatalf("writePresentation: %v", err)
	}
	return updated
}

// applyFreshForTest applies a deck to a not-yet-existing output and asserts the
// write was reported as a fresh creation (updated == false).
func applyFreshForTest(t *testing.T, mdText, out, templatePath string) {
	t.Helper()
	if updated := applyToFileForTest(t, mdText, out, templatePath); updated {
		t.Fatalf("apply to a new output should report a fresh write, got updated=true")
	}
}

// applyUpdateForTest applies a deck to an existing output and asserts the write
// was reported as an in-place update (updated == true).
func applyUpdateForTest(t *testing.T, mdText, out, templatePath string) {
	t.Helper()
	if updated := applyToFileForTest(t, mdText, out, templatePath); !updated {
		t.Fatalf("apply to an existing output should report an update, got updated=false")
	}
}

// applyTwiceForTest applies v1 to a fresh output, then v2 to the same output,
// returning the slide-part snapshots taken after each apply. It asserts the
// first apply is a fresh write and the second reports an update.
func applyTwiceForTest(t *testing.T, v1, v2, templatePath string) (orig, now map[string][]byte) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "deck.pptx")
	applyFreshForTest(t, v1, out, templatePath)
	orig = readSlidePartsForTest(t, out)
	applyUpdateForTest(t, v2, out, templatePath)
	now = readSlidePartsForTest(t, out)
	return orig, now
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

// zipRawPartsForTest reads every entry of a .pptx file into a name->bytes map.
func zipRawPartsForTest(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader: %v", err)
	}
	defer zr.Close()
	parts := map[string][]byte{}
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
		parts[f.Name] = b
	}
	return parts
}

// injectZipPartForTest rewrites a .pptx file with an extra part appended,
// simulating an out-of-band part added by PowerPoint or a user.
func injectZipPartForTest(t *testing.T, path, name string, data []byte) {
	t.Helper()
	existing := zipRawPartsForTest(t, path)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, d := range existing {
		fw, err := zw.Create(n)
		if err != nil {
			t.Fatalf("zip create %s: %v", n, err)
		}
		if _, err := fw.Write(d); err != nil {
			t.Fatalf("zip write %s: %v", n, err)
		}
	}
	fw, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create %s: %v", name, err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("zip write %s: %v", name, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// slideMetaExtRe matches the slidown fingerprint extLst embedded in a slide, so
// tests can strip it to simulate a slide imported from another presentation
// (which carries no slidown metadata).
var slideMetaExtRe = regexp.MustCompile(`<p:extLst><p:ext uri="\{[^"]+\}"><slidown:fp[^>]*/></p:ext></p:extLst>`)

// stripSlideMetaForTest rewrites a .pptx file removing the slidown fingerprint
// extension from every slide, simulating slides pasted in from another deck.
func stripSlideMetaForTest(t *testing.T, path string) {
	t.Helper()
	parts := zipRawPartsForTest(t, path)
	for name, data := range parts {
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		parts[name] = slideMetaExtRe.ReplaceAll(data, nil)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, d := range parts {
		fw, err := zw.Create(n)
		if err != nil {
			t.Fatalf("zip create %s: %v", n, err)
		}
		if _, err := fw.Write(d); err != nil {
			t.Fatalf("zip write %s: %v", n, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestApplyIncrementalReuse covers the incremental-rebuild behavior: applying a
// deck, then re-applying a changed deck to the same output, and asserting which
// slides are reused verbatim (unchanged or frozen) and which are regenerated.
// Cases exercise plain rebuilds, frozen slides, keyed reuse across an insert,
// and rebuilds seeded from an explicit template.
func TestApplyIncrementalReuse(t *testing.T) {
	const deck3 = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n---\n\n# Three\n\nbody three\n"
	const deck3Changed = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two changed\n\n---\n\n# Three\n\nbody three\n"
	const frozen2 = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n\n<!-- {\"freeze\": true} -->\n"
	const frozen2Changed = "# One\n\nbody one changed\n\n---\n\n# Two\n\nbody two changed\n\n<!-- {\"freeze\": true} -->\n"
	const keyedV1 = "# A\n\n<!-- {\"key\":\"a\",\"freeze\":true} -->\n\n---\n\n# B\n\n<!-- {\"key\":\"b\"} -->\n"
	const keyedV2 = "# New\n\n<!-- {\"key\":\"new\"} -->\n\n---\n\n# A CHANGED\n\n<!-- {\"key\":\"a\",\"freeze\":true} -->\n\n---\n\n# B\n\n<!-- {\"key\":\"b\"} -->\n"

	tests := []struct {
		name         string
		v1, v2       string
		withTemplate bool
		wantCount    int
		reused       [][2]int       // {origPos, newPos} pairs that must be byte-equal
		changed      []int          // new positions whose bytes must differ from the same orig position
		contains     map[int]string // new position -> substring it must contain
		notContains  map[int]string // new position -> substring it must not contain
	}{
		{
			name:      "unchanged slides are reused",
			v1:        deck3,
			v2:        deck3Changed,
			wantCount: 3,
			reused:    [][2]int{{1, 1}, {3, 3}},
			changed:   []int{2},
			contains:  map[int]string{2: "body two changed"},
		},
		{
			name:        "frozen slide keeps its content",
			v1:          frozen2,
			v2:          frozen2Changed,
			wantCount:   2,
			reused:      [][2]int{{2, 2}},
			changed:     []int{1},
			contains:    map[int]string{1: "body one changed"},
			notContains: map[int]string{2: "body two changed"},
		},
		{
			name:        "keyed slide reused across an insert",
			v1:          keyedV1,
			v2:          keyedV2,
			wantCount:   3,
			reused:      [][2]int{{1, 2}, {2, 3}}, // A moved 1->2 (frozen), B moved 2->3 (unchanged)
			notContains: map[int]string{2: "CHANGED"},
		},
		{
			name:         "unchanged slides reused with explicit template",
			v1:           deck3,
			v2:           deck3Changed,
			withTemplate: true,
			wantCount:    3,
			reused:       [][2]int{{1, 1}, {3, 3}},
			changed:      []int{2},
		},
		{
			name:         "frozen slide kept with explicit template",
			v1:           frozen2,
			v2:           frozen2Changed,
			withTemplate: true,
			wantCount:    2,
			reused:       [][2]int{{2, 2}},
			changed:      []int{1},
		},
	}

	slidePart := func(pos int) string { return fmt.Sprintf("ppt/slides/slide%d.xml", pos) }

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tmpl string
			if tc.withTemplate {
				tmpl = buildTemplateFileForTest(t)
			}

			orig, now := applyTwiceForTest(t, tc.v1, tc.v2, tmpl)

			if tc.wantCount != 0 && len(now) != tc.wantCount {
				t.Fatalf("slide count = %d, want %d", len(now), tc.wantCount)
			}
			for _, p := range tc.reused {
				if !bytes.Equal(orig[slidePart(p[0])], now[slidePart(p[1])]) {
					t.Errorf("slide at orig position %d was not reused at new position %d", p[0], p[1])
				}
			}
			for _, pos := range tc.changed {
				if bytes.Equal(orig[slidePart(pos)], now[slidePart(pos)]) {
					t.Errorf("slide %d was expected to change but was reused verbatim", pos)
				}
			}
			for pos, sub := range tc.contains {
				if !bytes.Contains(now[slidePart(pos)], []byte(sub)) {
					t.Errorf("slide %d does not contain %q: %s", pos, sub, now[slidePart(pos)])
				}
			}
			for pos, sub := range tc.notContains {
				if bytes.Contains(now[slidePart(pos)], []byte(sub)) {
					t.Errorf("slide %d unexpectedly contains %q", pos, sub)
				}
			}
		})
	}
}

func TestResolveApplyTemplate(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.pptx")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	missing := filepath.Join(dir, "missing.pptx")

	t.Run("flag on existing output errors", func(t *testing.T) {
		if _, err := resolveApplyTemplate(existing, "theme.pptx", ""); err == nil {
			t.Fatal("expected an error when --template is given for an existing output")
		}
	})
	t.Run("flag on new output seeds from flag", func(t *testing.T) {
		got, err := resolveApplyTemplate(missing, "theme.pptx", "cfg.pptx")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "theme.pptx" {
			t.Errorf("got %q, want the flag template", got)
		}
	})
	t.Run("existing output self-templates and ignores config", func(t *testing.T) {
		got, err := resolveApplyTemplate(existing, "", "cfg.pptx")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("got %q, want the existing output %q", got, existing)
		}
	})
	t.Run("config seeds a new output", func(t *testing.T) {
		got, err := resolveApplyTemplate(missing, "", "cfg.pptx")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "cfg.pptx" {
			t.Errorf("got %q, want the config template", got)
		}
	})
	t.Run("no template for a new output", func(t *testing.T) {
		got, err := resolveApplyTemplate(missing, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want built-in design (empty)", got)
		}
	})
}

// TestApplyUpdatesTitleOnlyChange verifies that editing only the deck title
// (frontmatter "title", stored in docProps/core.xml) is reflected on rebuild
// even though every slide's source is unchanged. The per-slide fingerprints do
// not cover the deck title, so the identity fast-path must compare it. The
// update must also preserve every other part verbatim, including unmanaged
// parts a user or PowerPoint may have added (e.g. customXml) and the slide
// bytes themselves.
func TestApplyUpdatesTitleOnlyChange(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const body = "\n\n# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n"
	base := "---\ntitle: First Title\n---" + body
	changed := "---\ntitle: Second Title\n---" + body

	applyFreshForTest(t, base, out, "")
	if got := pptx.ReadCoreTitle(out); got != "First Title" {
		t.Fatalf("initial title = %q, want %q", got, "First Title")
	}

	// Simulate an unmanaged part added out-of-band (as PowerPoint or a user
	// might), plus capture the slide bytes, to prove the title-only update
	// preserves the rest of the package verbatim.
	const customPart = "customXml/item1.xml"
	const customData = "<custom>keep me</custom>"
	injectZipPartForTest(t, out, customPart, []byte(customData))
	before := zipRawPartsForTest(t, out)

	// Only the title changed; all slides are identical.
	applyUpdateForTest(t, changed, out, "")
	after := zipRawPartsForTest(t, out)

	if got := pptx.ReadCoreTitle(out); got != "Second Title" {
		t.Errorf("title-only change was dropped: title = %q, want %q", got, "Second Title")
	}
	if !bytes.Equal(after[customPart], []byte(customData)) {
		t.Errorf("unmanaged part %q was not preserved on a title-only update: %q", customPart, after[customPart])
	}
	for _, name := range []string{"ppt/slides/slide1.xml", "ppt/slides/slide2.xml"} {
		if !bytes.Equal(before[name], after[name]) {
			t.Errorf("slide %q was rewritten on a title-only update; expected byte-for-byte preservation", name)
		}
	}
}

// TestApplyIgnoresFrontmatterTemplate verifies that a "template" field in a
// deck's frontmatter is silently ignored: the generated file uses the built-in
// design rather than the frontmatter-referenced template.
func TestApplyIgnoresFrontmatterTemplate(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")
	// An alternate template with a distinctive accent color (FF0000). If the
	// frontmatter template were honored, the output theme would carry it.
	altTemplate := buildAltTemplateFileForTest(t)

	deck := fmt.Sprintf("---\ntemplate: %s\n---\n\n# One\n\nbody one\n", altTemplate)

	applyFreshForTest(t, deck, out, "")

	parts, err := zipPartsForTest(t, mustReadFileForTest(t, out))
	if err != nil {
		t.Fatalf("zipPartsForTest: %v", err)
	}
	if strings.Contains(parts["ppt/theme/theme1.xml"], "FF0000") {
		t.Errorf("frontmatter template was honored: theme carries the alt template accent color FF0000")
	}
	if !strings.Contains(parts["ppt/theme/theme1.xml"], "4472C4") {
		t.Errorf("expected built-in design accent color 4472C4 when frontmatter template is ignored")
	}
	assertSlideRelsResolveInPackage(t, parts)
}

func TestApplyPreservesVisibleOrderBytesAfterReorderedPresentation(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n"
	const reorderedSource = "# Two\n\nbody two\n\n---\n\n# One\n\nbody one\n"

	applyFreshForTest(t, v1, out, "")

	reorderPresentationAndMarkSlidesInFileForTest(t, out)
	beforeVisible := readVisibleSlidePartsForTest(t, out)

	// NOTE: this second apply hits the isIdentityReuse short-circuit (all 2
	// slides match their same visible position) and returns early without
	// running MergeReusingUnchangedSlides. The test below
	// (TestApplyCorrectlyReusesSlideAfterReorderAndNonIdentityRebuild) covers
	// the merge path explicitly.
	applyUpdateForTest(t, reorderedSource, out, "")
	afterVisible := readVisibleSlidePartsForTest(t, out)

	if !bytes.Equal(beforeVisible[0], afterVisible[0]) {
		t.Fatalf("first visible slide bytes were not preserved after rebuild")
	}
	if !bytes.Equal(beforeVisible[1], afterVisible[1]) {
		t.Fatalf("second visible slide bytes were not preserved after rebuild")
	}
}

// TestApplyCorrectlyReusesSlideAfterReorderAndNonIdentityRebuild is the
// regression test for the bug introduced in PR #26: when a presentation has
// sldIdLst order different from filename order (e.g. after a PowerPoint
// drag-reorder), and the rebuild is non-identity (slide count changes), the
// merge must copy the slide identified by its on-disk part name — NOT the part
// whose filename number matches the visible position.
//
// This test MUST NOT hit the isIdentityReuse short-circuit; it exercises
// MergeReusingUnchangedSlides directly.
func TestApplyCorrectlyReusesSlideAfterReorderAndNonIdentityRebuild(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	// Step 1: generate a 2-slide deck.
	//   slide1.xml = "One" content, slide2.xml = "Two" content.
	const v1 = "# One\n\nbody one\n\n---\n\n# Two\n\nbody two\n"
	applyFreshForTest(t, v1, out, "")

	// Step 2: simulate a PowerPoint reorder + manual edits.
	//   - sldIdLst is flipped to [rId2, rId1]:
	//       visible position 1 → on-disk slide2.xml (Two content)
	//       visible position 2 → on-disk slide1.xml (One content)
	//   - testManualFirstMarker is injected into on-disk slide2.xml (visible-first)
	//   - testManualSecondMarker is injected into on-disk slide1.xml (visible-second)
	reorderPresentationAndMarkSlidesInFileForTest(t, out)

	// Step 3: rebuild with 3 slides — Two first, One second, Three new.
	//   Source slides 1 and 2 still match the fingerprints of visible positions
	//   1 and 2 respectively, so they are candidates for reuse. Slide 3 is
	//   new, so len(reuse)=2 ≠ sourceLen=3: isIdentityReuse is false and
	//   MergeReusingUnchangedSlides is invoked.
	const v3 = "# Two\n\nbody two\n\n---\n\n# One\n\nbody one\n\n---\n\n# Three\n\nbody three\n"
	applyUpdateForTest(t, v3, out, "")

	// Step 4: assertions.
	// The new presentation has 3 slides in natural order (slide1, slide2, slide3).
	// Because the reuse map is {1: "ppt/slides/slide2.xml", 2: "ppt/slides/slide1.xml"}:
	//   new slide1.xml ← old slide2.xml  (has testManualFirstMarker)
	//   new slide2.xml ← old slide1.xml  (has testManualSecondMarker)
	//   new slide3.xml ← freshly generated (no marker)
	// Before the fix, slide1.xml ← old slide1.xml (wrong: testManualSecondMarker
	// appears at visible position 1, and the user's manual edit on slide2 is lost).
	now := readSlidePartsForTest(t, out)
	if len(now) != 3 {
		t.Fatalf("expected 3 slide parts after rebuild, got %d", len(now))
	}

	slide1 := string(now["ppt/slides/slide1.xml"])
	slide2 := string(now["ppt/slides/slide2.xml"])
	slide3 := string(now["ppt/slides/slide3.xml"])

	// slide1.xml must come from old slide2.xml → must contain testManualFirstMarker.
	if !strings.Contains(slide1, testManualFirstMarker) {
		t.Errorf("slide1.xml missing %q: the wrong on-disk file was reused.\n"+
			"slide1.xml content (truncated): %.200s", testManualFirstMarker, slide1)
	}
	// slide1.xml must NOT contain testManualSecondMarker (that belongs to slide2).
	if strings.Contains(slide1, testManualSecondMarker) {
		t.Errorf("slide1.xml contains %q: old slide1.xml was copied instead of slide2.xml",
			testManualSecondMarker)
	}

	// slide2.xml must come from old slide1.xml → must contain testManualSecondMarker.
	if !strings.Contains(slide2, testManualSecondMarker) {
		t.Errorf("slide2.xml missing %q: the wrong on-disk file was reused.\n"+
			"slide2.xml content (truncated): %.200s", testManualSecondMarker, slide2)
	}

	// slide3.xml is freshly generated and must not carry either manual marker.
	if strings.Contains(slide3, testManualFirstMarker) || strings.Contains(slide3, testManualSecondMarker) {
		t.Errorf("slide3.xml should be freshly generated but contains a manual marker")
	}
}

func readVisibleSlidePartsForTest(t *testing.T, pptxPath string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(pptxPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
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
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		parts[f.Name] = b
	}

	var p testPresentation
	if err := xml.Unmarshal(parts["ppt/presentation.xml"], &p); err != nil {
		t.Fatalf("unmarshal presentation.xml: %v", err)
	}
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(parts["ppt/_rels/presentation.xml.rels"], &rels); err != nil {
		t.Fatalf("unmarshal presentation.xml.rels: %v", err)
	}

	targetByID := map[string]string{}
	for _, r := range rels.Rels {
		if strings.HasSuffix(r.Type, "/slide") {
			targetByID[r.ID] = r.Target
		}
	}

	visible := make([][]byte, 0, len(p.SlideIDs))
	for _, s := range p.SlideIDs {
		target, ok := targetByID[s.RelID]
		if !ok {
			t.Fatalf("missing rel target for %q", s.RelID)
		}
		slideName := target
		if strings.HasPrefix(slideName, "/") {
			slideName = strings.TrimPrefix(slideName, "/")
		} else {
			slideName = path.Clean(path.Join("ppt", slideName))
		}
		b, ok := parts[slideName]
		if !ok {
			t.Fatalf("missing slide part %q", slideName)
		}
		visible = append(visible, b)
	}
	return visible
}

func reorderPresentationAndMarkSlidesInFileForTest(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	parts := map[string][]byte{}
	order := make([]string, 0, len(zr.File))
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
		parts[f.Name] = b
		order = append(order, f.Name)
	}

	parts["ppt/presentation.xml"] = reorderPresentationXMLForApplyTest(t, parts["ppt/presentation.xml"])

	s1 := parts["ppt/slides/slide1.xml"]
	s1Updated := bytes.Replace(s1, []byte("body one"), []byte("body one "+testManualSecondMarker), 1)
	if bytes.Equal(s1Updated, s1) {
		t.Fatalf("failed to mark slide1.xml with %q", testManualSecondMarker)
	}
	parts["ppt/slides/slide1.xml"] = s1Updated

	s2 := parts["ppt/slides/slide2.xml"]
	s2Updated := bytes.Replace(s2, []byte("body two"), []byte("body two "+testManualFirstMarker), 1)
	if bytes.Equal(s2Updated, s2) {
		t.Fatalf("failed to mark slide2.xml with %q", testManualFirstMarker)
	}
	parts["ppt/slides/slide2.xml"] = s2Updated

	var outBuf bytes.Buffer
	zw := zip.NewWriter(&outBuf)
	for _, name := range order {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fw.Write(parts[name]); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := os.WriteFile(path, outBuf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func reorderPresentationXMLForApplyTest(t *testing.T, presentationXML []byte) []byte {
	t.Helper()
	var p testPresentation
	if err := xml.Unmarshal(presentationXML, &p); err != nil {
		t.Fatalf("xml.Unmarshal presentation.xml: %v", err)
	}
	if len(p.SlideIDs) < 2 {
		t.Fatalf("presentation.xml has less than 2 slides")
	}
	p.SlideIDs[0], p.SlideIDs[1] = p.SlideIDs[1], p.SlideIDs[0]

	var b strings.Builder
	b.WriteString("<p:sldIdLst>")
	for i, s := range p.SlideIDs {
		b.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="%s"/>`, 256+i, s.RelID))
	}
	b.WriteString("</p:sldIdLst>")

	re := regexp.MustCompile(`(?s)<p:sldIdLst>.*?</p:sldIdLst>`)
	updated := re.ReplaceAll(presentationXML, []byte(b.String()))
	if bytes.Equal(updated, presentationXML) {
		t.Fatalf("failed to rewrite sldIdLst in presentation.xml")
	}
	return updated
}

// buildAltTemplateFileForTest creates a template .pptx that is visually and
// content-hash distinct from the default built-in design by swapping the accent1
// color in the theme XML.
func buildAltTemplateFileForTest(t *testing.T) string {
	t.Helper()
	basePath := buildTemplateFileForTest(t)

	// Open the base template zip, modify the theme part, write a new zip.
	zr, err := zip.OpenReader(basePath)
	if err != nil {
		t.Fatalf("buildAltTemplateFileForTest OpenReader: %v", err)
	}
	defer zr.Close()

	parts := map[string][]byte{}
	order := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("buildAltTemplateFileForTest open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("buildAltTemplateFileForTest read %s: %v", f.Name, err)
		}
		parts[f.Name] = b
		order = append(order, f.Name)
	}

	// Patch the theme: replace the default accent1 blue with red.
	const (
		origColor = "4472C4"
		newColor  = "FF0000"
	)
	themeKey := "ppt/theme/theme1.xml"
	patched := bytes.ReplaceAll(parts[themeKey], []byte(origColor), []byte(newColor))
	if bytes.Equal(patched, parts[themeKey]) {
		t.Fatalf("buildAltTemplateFileForTest: accent1 color %q not found in %s", origColor, themeKey)
	}
	parts[themeKey] = patched

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("buildAltTemplateFileForTest zip create %s: %v", name, err)
		}
		if _, err := fw.Write(parts[name]); err != nil {
			t.Fatalf("buildAltTemplateFileForTest zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("buildAltTemplateFileForTest zip close: %v", err)
	}

	altPath := filepath.Join(t.TempDir(), "alt_template.pptx")
	if err := os.WriteFile(altPath, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("buildAltTemplateFileForTest WriteFile: %v", err)
	}
	return altPath
}

func mustReadFileForTest(t *testing.T, filePath string) []byte {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("mustReadFileForTest: %v", err)
	}
	return data
}

// assertSlideRelsResolveInPackage checks that every internal relationship
// target in each slide's .rels file resolves to a part that exists in the
// package, guarding against dangling rels.
func assertSlideRelsResolveInPackage(t *testing.T, parts map[string]string) {
	t.Helper()
	var relDoc struct {
		Rels []struct {
			Type       string `xml:"Type,attr"`
			Target     string `xml:"Target,attr"`
			TargetMode string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	for i := 1; ; i++ {
		relsName := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i)
		relsXML, ok := parts[relsName]
		if !ok {
			break
		}
		if err := xml.Unmarshal([]byte(relsXML), &relDoc); err != nil {
			t.Errorf("failed to parse %s: %v", relsName, err)
			continue
		}
		for _, r := range relDoc.Rels {
			if strings.EqualFold(r.TargetMode, "External") || r.Target == "" {
				continue
			}
			resolved := path.Clean(path.Join("ppt/slides", r.Target))
			if _, ok := parts[resolved]; !ok {
				t.Errorf("slide%d rel Target=%q resolves to %q which does not exist in the package",
					i, r.Target, resolved)
			}
		}
	}
}

// rewriteSlidePartForTest rewrites a single slide part inside an existing .pptx,
// leaving every other entry untouched (no duplicate entries), simulating a
// manual edit made in PowerPoint.
func rewriteSlidePartForTest(t *testing.T, path, name string, transform func([]byte) []byte) {
	t.Helper()
	parts := zipRawPartsForTest(t, path)
	got, ok := parts[name]
	if !ok {
		t.Fatalf("part %q not found in %s", name, path)
	}
	parts[name] = transform(got)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, d := range parts {
		fw, err := zw.Create(n)
		if err != nil {
			t.Fatalf("zip create %s: %v", n, err)
		}
		if _, err := fw.Write(d); err != nil {
			t.Fatalf("zip write %s: %v", n, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestApplyShapeLevelMergePreservesManualEditOnUnchangedShape verifies the
// shape-level incremental rebuild: when only one text box's source changes, the
// slide is not wholly regenerated. The unchanged title keeps a manual xfrm edit
// made in PowerPoint, while the changed body is updated to the new content.
func TestApplyShapeLevelMergePreservesManualEditOnUnchangedShape(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# Title A\n\nbody one\n\n---\n\n# Title B\n\nbody two\n"
	applyFreshForTest(t, v1, out, "")

	// Snapshot the untouched second slide to assert it is reused verbatim.
	origSlide2 := readSlidePartsForTest(t, out)["ppt/slides/slide2.xml"]

	// Simulate a manual PowerPoint edit: give the (source-unchanged) title of
	// slide 1 an explicit position. This lives in spPr and does not affect the
	// title's per-shape fingerprint (which is carried in extLst).
	const xfrmMarker = `x="424242"`
	rewriteSlidePartForTest(t, out, "ppt/slides/slide1.xml", func(b []byte) []byte {
		marker := `<a:xfrm><a:off x="424242" y="111"/><a:ext cx="1" cy="1"/></a:xfrm>`
		return bytes.Replace(b, []byte(`<p:spPr>`), []byte(`<p:spPr>`+marker), 1)
	})

	// Change only slide 1's body text.
	const v2 = "# Title A\n\nbody one EDITED\n\n---\n\n# Title B\n\nbody two\n"
	applyUpdateForTest(t, v2, out, "")

	now := readSlidePartsForTest(t, out)
	slide1 := string(now["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, xfrmMarker) {
		t.Errorf("manual xfrm on the unchanged title was lost during rebuild:\n%.400s", slide1)
	}
	if !strings.Contains(slide1, "body one EDITED") {
		t.Errorf("changed body was not updated to the new content:\n%.400s", slide1)
	}
	if strings.Contains(slide1, ">body one<") {
		t.Errorf("stale body content survived on slide 1:\n%.400s", slide1)
	}
	if !bytes.Equal(origSlide2, now["ppt/slides/slide2.xml"]) {
		t.Errorf("unchanged slide 2 was not reused verbatim")
	}
}

// TestApplyShapeLevelMergeWithSkippedSlide guards the position mapping: a
// skipped source slide emits no output slide, so alignment must use rendered
// positions. A leading skipped page must not shift shape-level merges onto the
// wrong slide.
func TestApplyShapeLevelMergeWithSkippedSlide(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# Skipme\n\n<!-- {\"skip\": true} -->\n\n---\n\n# Alpha\n\nbody a\n\n---\n\n# Bravo\n\nbody b\n"
	applyFreshForTest(t, v1, out, "")

	parts := readSlidePartsForTest(t, out)
	if len(parts) != 2 {
		t.Fatalf("expected 2 rendered slides (one skipped), got %d", len(parts))
	}
	origSlide2 := parts["ppt/slides/slide2.xml"] // Bravo, must stay reused verbatim

	const xfrmMarker = `x="515151"`
	rewriteSlidePartForTest(t, out, "ppt/slides/slide1.xml", func(b []byte) []byte {
		marker := `<a:xfrm><a:off x="515151" y="9"/><a:ext cx="1" cy="1"/></a:xfrm>`
		return bytes.Replace(b, []byte(`<p:spPr>`), []byte(`<p:spPr>`+marker), 1)
	})

	// Change only Alpha's body; skip and Bravo unchanged.
	const v2 = "# Skipme\n\n<!-- {\"skip\": true} -->\n\n---\n\n# Alpha\n\nbody a EDITED\n\n---\n\n# Bravo\n\nbody b\n"
	applyUpdateForTest(t, v2, out, "")

	now := readSlidePartsForTest(t, out)
	slide1 := string(now["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, xfrmMarker) {
		t.Errorf("Alpha's manual title edit was lost; positions likely misaligned by the skipped slide:\n%.400s", slide1)
	}
	if !strings.Contains(slide1, "body a EDITED") {
		t.Errorf("Alpha's body was not updated:\n%.400s", slide1)
	}
	if !bytes.Equal(origSlide2, now["ppt/slides/slide2.xml"]) {
		t.Errorf("Bravo (slide 2) was not reused verbatim")
	}
}

func TestRenderedSlidesExcludesSkipped(t *testing.T) {
	skip := &slidown.Slide{Skip: true}
	a := &slidown.Slide{Titles: []string{"A"}}
	b := &slidown.Slide{Titles: []string{"B"}}
	got := renderedSlides(slidown.Slides{skip, a, nil, b})
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("renderedSlides did not drop skipped/nil slides: %+v", got)
	}
}

func TestAnchorsReordered(t *testing.T) {
	if anchorsReordered([]int{-1, 0, -1, 1, 2}) {
		t.Error("monotonic anchors flagged as reordered")
	}
	if !anchorsReordered([]int{1, 0}) {
		t.Error("descending anchors not flagged as reordered")
	}
}

// TestApplyOrphanedKeyedSlideReusedByFrozenPage documents design B: an existing
// slide whose stable key is no longer present in the deck source is "orphaned"
// and is no longer reserved for key matching, so it may be re-paired by
// position. A frozen page occupying that position therefore keeps the existing
// slide (freeze's defined behavior) and the final stamping pass clears the
// now-absent key. This is the deliberate trade-off that lets a renamed key
// re-pair with its slide (see TestApplyRenamedKeyReclaimsFrozenSlide).
func TestApplyOrphanedKeyedSlideReusedByFrozenPage(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# Keep\n\n<!-- {\"key\":\"k\"} -->\n\nkeep body\n"
	applyFreshForTest(t, v1, out, "")

	// New source has no slide with key "k"; a keyless frozen slide sits at the
	// same position.
	const v2 = "# Other\n\n<!-- {\"freeze\": true} -->\n\nother body\n"
	applyUpdateForTest(t, v2, out, "")

	slide1 := string(readSlidePartsForTest(t, out)["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, "keep body") {
		t.Errorf("frozen page should keep the existing slide at its position:\n%.400s", slide1)
	}
	if strings.Contains(slide1, `k="k"`) {
		t.Errorf("orphaned key 'k' should have been cleared by the stamping pass:\n%.400s", slide1)
	}
}

// TestApplyRenamedKeyReclaimsFrozenSlide is the motivating case for design B and
// the key-stamping pass: when a frozen page's key is renamed, the existing slide
// (whose old key is now orphaned) is re-paired by position, kept verbatim by
// freeze even though the body changed, and re-tagged with the new key so later
// rebuilds match it by key.
func TestApplyRenamedKeyReclaimsFrozenSlide(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# Slide\n\n<!-- {\"key\":\"k1\",\"freeze\":true} -->\n\noriginal body\n"
	applyFreshForTest(t, v1, out, "")

	const v2 = "# Slide\n\n<!-- {\"key\":\"k2\",\"freeze\":true} -->\n\nedited body\n"
	applyUpdateForTest(t, v2, out, "")

	slide1 := string(readSlidePartsForTest(t, out)["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, "original body") || strings.Contains(slide1, "edited body") {
		t.Errorf("frozen slide should be kept verbatim across the key rename:\n%.400s", slide1)
	}
	if !strings.Contains(slide1, `k="k2"`) {
		t.Errorf("slide should be re-stamped with the new key k2:\n%.400s", slide1)
	}
	if strings.Contains(slide1, `k="k1"`) {
		t.Errorf("old key k1 should no longer be present:\n%.400s", slide1)
	}
}

// TestApplyImportedSlideStampedWithKey mirrors bringing in a slide from another
// presentation: a slide that carries no slidown metadata but sits where a keyed
// frozen page is declared should be tagged with that key by the stamping pass,
// so subsequent rebuilds can match it by key rather than by position.
func TestApplyImportedSlideStampedWithKey(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")

	const v1 = "# Placeholder\n\n<!-- {\"key\":\"imported\",\"freeze\":true} -->\n\nplaceholder\n"
	applyFreshForTest(t, v1, out, "")

	// Simulate an imported slide by stripping all slidown metadata from slide 1,
	// as a paste from another presentation would carry none.
	stripSlideMetaForTest(t, out)
	before := string(readSlidePartsForTest(t, out)["ppt/slides/slide1.xml"])
	if strings.Contains(before, "slidown:fp") {
		t.Fatalf("test setup failed to strip slidown metadata:\n%.400s", before)
	}

	applyUpdateForTest(t, v1, out, "")

	slide1 := string(readSlidePartsForTest(t, out)["ppt/slides/slide1.xml"])
	if !strings.Contains(slide1, `k="imported"`) {
		t.Errorf("imported slide should be stamped with its page key:\n%.400s", slide1)
	}
}
