package pptx

import (
	"bytes"
	"os"
	"path"
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

	reuse := map[int]int{1: 1}
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

// TestMergeReusingUnchangedSlidesNoMediaCollision reproduces the bug where a
// reused slide's media overwrites a freshly-generated image with the same name.
//
// Scenario: "existing" has one slide with imageX (→ image1.png).  "newPPTX"
// renders a different imageY into image1.png.  Reusing slide 1 from existing
// must not clobber the new image1.png=Y; instead the old bytes must be stored
// under a freshly allocated name and the reused slide's rels rewritten to match.
func TestMergeReusingUnchangedSlidesNoMediaCollision(t *testing.T) {
	// Distinct fake image payloads — content intentionally differs so a
	// collision is detectable.
	imageX := []byte("fake-png-X")
	imageY := []byte("fake-png-Y")

	buildWithPicture := func(img []byte) []byte {
		t.Helper()
		p := New()
		s := p.AddSlide()
		s.AddShape(&Shape{
			Placeholder: PlaceholderTitle,
			Paragraphs:  []*Paragraph{{Runs: []*Run{{Text: "Slide"}}}},
		})
		s.AddPicture(&Picture{Data: img, Ext: "png", W: 1000000, H: 1000000})
		var buf bytes.Buffer
		if _, err := p.WriteTo(&buf); err != nil {
			t.Fatalf("WriteTo: %v", err)
		}
		return buf.Bytes()
	}

	existing := buildWithPicture(imageX)
	newPPTX := buildWithPicture(imageY)

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := os.WriteFile(existingPath, existing, 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	merged, err := MergeReusingUnchangedSlides(existingPath, newPPTX, map[int]int{1: 1})
	if err != nil {
		t.Fatalf("MergeReusingUnchangedSlides: %v", err)
	}

	parts, _, err := readZipPartsFromBytes(merged)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}

	// Both images must be present — imageY must not have been overwritten.
	foundX, foundY := false, false
	for name, data := range parts {
		if !strings.HasPrefix(name, "ppt/media/") {
			continue
		}
		if bytes.Equal(data, imageX) {
			foundX = true
		}
		if bytes.Equal(data, imageY) {
			foundY = true
		}
	}
	if !foundX {
		t.Error("merged package is missing imageX (the reused slide's image)")
	}
	if !foundY {
		t.Error("merged package is missing imageY (the new render's image was overwritten by imageX)")
	}

	// The reused slide's rels must reference a media part that carries imageX.
	rels, ok := parts["ppt/slides/_rels/slide1.xml.rels"]
	if !ok {
		t.Fatal("missing ppt/slides/_rels/slide1.xml.rels in merged package")
	}
	for _, target := range relTargets(rels) {
		resolved := path.Clean(path.Join("ppt/slides", target))
		if !strings.HasPrefix(resolved, "ppt/media/") {
			continue
		}
		data, ok := parts[resolved]
		if !ok {
			t.Errorf("slide1 rels reference %s but the part is absent from the merged package", resolved)
			continue
		}
		if !bytes.Equal(data, imageX) {
			t.Errorf("slide1 rels reference %s whose content is not imageX (collision rename likely broken)", resolved)
		}
	}
}

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

	merged, err := MergeReusingUnchangedSlides(existingPath, newPPTX, map[int]int{1: 1})
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
