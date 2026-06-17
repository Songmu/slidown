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

	merged, err := MergeReusingUnchangedSlides(existingPath, newPPTX, []int{1})
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
