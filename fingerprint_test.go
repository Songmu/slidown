package slidown_test

import (
	"context"
	"testing"

	slidown "github.com/Songmu/slidown"
	"github.com/Songmu/slidown/md"
)

func fpForMarkdown(t *testing.T, src string) []string {
	t.Helper()
	m, err := md.Parse("testdata", []byte(src), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := m.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	fps := make([]string, len(slides))
	for i, s := range slides {
		fps[i] = s.Fingerprint()
	}
	return fps
}

// TestFingerprintDetectsSourceChanges covers the formatting-only changes that a
// plain-text comparison (the previous reuse heuristic) silently missed.
func TestFingerprintDetectsSourceChanges(t *testing.T) {
	cases := []struct {
		name   string
		a, b   string
		differ bool
	}{
		{"identical", "# Title\n\nbody\n", "# Title\n\nbody\n", false},
		{"title bold", "# **Title**\n\nbody\n", "# Title\n\nbody\n", true},
		{"body bold", "# T\n\n**body**\n", "# T\n\nbody\n", true},
		{"underline", "# T\n\n<u>body</u>\n", "# T\n\nbody\n", true},
		{"strikethrough", "# T\n\n~~body~~\n", "# T\n\nbody\n", true},
		{"speaker note", "# T\n\nbody\n\n<!--\nnote\n-->\n", "# T\n\nbody\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := fpForMarkdown(t, tc.a)
			fb := fpForMarkdown(t, tc.b)
			if len(fa) != 1 || len(fb) != 1 {
				t.Fatalf("expected single slides, got %d and %d", len(fa), len(fb))
			}
			if fa[0] == "" {
				t.Fatalf("empty fingerprint")
			}
			if tc.differ && fa[0] == fb[0] {
				t.Errorf("fingerprints should differ for %s but matched", tc.name)
			}
			if !tc.differ && fa[0] != fb[0] {
				t.Errorf("fingerprints should match for %s but differed", tc.name)
			}
		})
	}
}

func slideFor(t *testing.T, src string) *slidown.Slide {
	t.Helper()
	m, err := md.Parse("testdata", []byte(src), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := m.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	if len(slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(slides))
	}
	return slides[0]
}

func TestMatchesFingerprintImageTolerance(t *testing.T) {
	// A recompressed but visually-equivalent image is treated as unchanged.
	orig := slideFor(t, "# T\n\n![](test.jpeg)\n")
	recompressed := slideFor(t, "# T\n\n![](test.compressed.jpeg)\n")
	if !recompressed.MatchesFingerprint(orig.Fingerprint()) {
		t.Errorf("recompressed image should be treated as unchanged")
	}

	// Reordering images on a slide is treated as unchanged (order-independent),
	// even with two visually-distinct images.
	ab := slideFor(t, "# T\n\n![](test.png)\n\n![](test.jpeg)\n")
	ba := slideFor(t, "# T\n\n![](test.jpeg)\n\n![](test.png)\n")
	if !ba.MatchesFingerprint(ab.Fingerprint()) {
		t.Errorf("reordered images should be treated as unchanged")
	}

	// A genuinely different image forces a regenerate (test.png and test.jpeg
	// are visually different pictures).
	png := slideFor(t, "# T\n\n![](test.png)\n")
	jpeg := slideFor(t, "# T\n\n![](test.jpeg)\n")
	if jpeg.MatchesFingerprint(png.Fingerprint()) {
		t.Errorf("a different image should not match")
	}

	// A text change forces a regenerate even when the image is unchanged.
	changed := slideFor(t, "# Changed\n\n![](test.png)\n")
	if changed.MatchesFingerprint(png.Fingerprint()) {
		t.Errorf("a text change must force a regenerate")
	}
}
