package pptx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// presentationWithNote builds a single-slide presentation, optionally with a
// speaker note, and returns its serialized bytes.
func presentationWithNote(t *testing.T, note string) []byte {
	t.Helper()
	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{
		Placeholder: PlaceholderTitle,
		Paragraphs:  []*Paragraph{{Runs: []*Run{{Text: "Title"}}}},
	})
	s.Note = note
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

// twoSlidePresentationWithFingerprints builds a 2-slide presentation where
// each slide has a distinct embedded fingerprint, and returns its bytes.
func twoSlidePresentationWithFingerprints(t *testing.T, fp1, fp2 string) []byte {
	t.Helper()
	p := New()
	s1 := p.AddSlide()
	s1.Fingerprint = fp1
	s2 := p.AddSlide()
	s2.Fingerprint = fp2
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

// TestMergeReusingUnchangedSlidesDeterministic verifies that repeated calls
// with identical inputs produce byte-identical output, regardless of map
// iteration order.
func TestMergeReusingUnchangedSlidesDeterministic(t *testing.T) {
	existing := presentationWithNote(t, "speaker note")
	newPPTX := presentationWithNote(t, "")

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := os.WriteFile(existingPath, existing, 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	reuse := map[int]string{1: "ppt/slides/slide1.xml"}
	first, err := MergeReusingUnchangedSlides(existingPath, newPPTX, reuse)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	for i := 2; i <= 5; i++ {
		got, err := MergeReusingUnchangedSlides(existingPath, newPPTX, reuse)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !bytes.Equal(first, got) {
			t.Errorf("call %d produced different bytes (output is non-deterministic)", i)
		}
	}
}

// TestMergeReusingUnchangedSlidesRestoresNotesInfra reproduces the case where a
// reused slide kept its note but the regenerated package has no notes at all:
// the notes master and the content-type overrides must be restored so the
// package is not corrupt.
func TestMergeReusingUnchangedSlidesRestoresNotesInfra(t *testing.T) {
	existing := presentationWithNote(t, "speaker note")
	newPPTX := presentationWithNote(t, "") // regenerated deck has no notes

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := os.WriteFile(existingPath, existing, 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	merged, err := MergeReusingUnchangedSlides(existingPath, newPPTX, map[int]string{1: "ppt/slides/slide1.xml"})
	if err != nil {
		t.Fatalf("MergeReusingUnchangedSlides: %v", err)
	}

	parts, _, err := readZipPartsFromBytes(merged)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}

	// The reused note and its master must both be present (no dangling rel).
	if _, ok := parts["ppt/notesSlides/notesSlide1.xml"]; !ok {
		t.Errorf("merged package is missing the reused notesSlide1.xml")
	}
	if _, ok := parts["ppt/notesMasters/notesMaster1.xml"]; !ok {
		t.Errorf("merged package is missing notesMaster1.xml (dangling relationship)")
	}

	// The notes parts must be declared in [Content_Types].xml.
	ct := string(parts["[Content_Types].xml"])
	if !strings.Contains(ct, `PartName="/ppt/notesSlides/notesSlide1.xml"`) {
		t.Errorf("[Content_Types].xml missing notesSlide1 override:\n%s", ct)
	}
	if !strings.Contains(ct, `PartName="/ppt/notesMasters/notesMaster1.xml"`) {
		t.Errorf("[Content_Types].xml missing notesMaster override:\n%s", ct)
	}
}

// TestMergeReusingUnchangedSlidesReorderedPresentation is the regression test
// for the bug where the reuse map value was treated as a filename position
// rather than a real part name. When the existing PPTX has sldIdLst order
// different from filename order (e.g. after a PowerPoint drag-reorder),
// MergeReusingUnchangedSlides must copy the part identified by its PartName,
// not the part whose filename number matches the visible position.
func TestMergeReusingUnchangedSlidesReorderedPresentation(t *testing.T) {
	// Build a 2-slide presentation where each slide has a distinct embedded
	// fingerprint so we can tell them apart after the merge.
	const fpSlide1 = "fingerprint-of-slide-one"
	const fpSlide2 = "fingerprint-of-slide-two"
	existing2 := twoSlidePresentationWithFingerprints(t, fpSlide1, fpSlide2)

	// Reorder the presentation so sldIdLst = [rId2, rId1]:
	// on disk slide2.xml is now visible-first, slide1.xml is visible-second.
	// (reorderPresentationOrder is shared with fingerprint_read_test.go)
	existingReordered := reorderPresentationOrder(t, existing2)

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := os.WriteFile(existingPath, existingReordered, 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	// Capture the raw bytes of slide2.xml from the existing file so we can
	// assert that exactly those bytes end up in the merged slide1.xml.
	existingParts, _, err := readZipPartsFromBytes(existingReordered)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes existing: %v", err)
	}
	wantSlide1Bytes := existingParts["ppt/slides/slide2.xml"]
	if len(wantSlide1Bytes) == 0 {
		t.Fatal("slide2.xml missing from existing fixture")
	}

	// newPPTX: a freshly regenerated 2-slide deck (content irrelevant here).
	newPPTX := existing2

	// Reuse the existing slide2.xml at new position 1.  Before the fix,
	// MergeReusingUnchangedSlides would have interpreted the map value as a
	// filename number (1 → slide1.xml), copying the wrong slide.
	merged, err := MergeReusingUnchangedSlides(existingPath, newPPTX, map[int]string{
		1: "ppt/slides/slide2.xml",
	})
	if err != nil {
		t.Fatalf("MergeReusingUnchangedSlides: %v", err)
	}

	mergedParts, _, err := readZipPartsFromBytes(merged)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes merged: %v", err)
	}

	gotSlide1 := mergedParts["ppt/slides/slide1.xml"]
	if !bytes.Equal(gotSlide1, wantSlide1Bytes) {
		t.Errorf("merged slide1.xml does not match the existing slide2.xml bytes; "+
			"the wrong on-disk file was used (visible-position-as-filename bug).\n"+
			"got fingerprint in slide1: %q; want %q",
			extractFingerprint(gotSlide1), fpSlide2)
	}
}

// extractFingerprint pulls the fp v="..." attribute from slide XML for
// diagnostic messages in tests.
func extractFingerprint(slideXML []byte) string {
	m := parseSlideMeta(slideXML)
	return m.Fingerprint
}
