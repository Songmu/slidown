package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeWithExistingPreservesExtraParts(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := writeZipFile(existingPath, map[string]string{
		"foo/custom.txt":             "keep me",
		"ppt/slides/slide1.xml":      "old slide",
		"ppt/slides/slide1.xml.rels": "old rels",
	}); err != nil {
		t.Fatalf("write existing zip: %v", err)
	}

	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{Placeholder: PlaceholderTitle, IsPlaceholder: true, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: "new title"}}}}})
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	merged, err := MergeWithExisting(existingPath, buf.Bytes())
	if err != nil {
		t.Fatalf("MergeWithExisting: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(merged), int64(len(merged)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}

	if got := parts["foo/custom.txt"]; got != "keep me" {
		t.Fatalf("custom part not preserved: %q", got)
	}
	if got := parts["ppt/slides/slide1.xml"]; got == "old slide" {
		t.Fatalf("slide1.xml was not updated")
	}
	if got := parts["ppt/slides/slide1.xml"]; got == "" {
		t.Fatalf("slide1.xml missing")
	}
}

// TestMergeWithExistingDropsStaleDesignParts guards against old-only design
// parts (slide layouts/masters, themes and media) surviving into the merged
// package. Such leftovers appear when the template changes to one with fewer
// layouts: the freshly generated package no longer declares them in
// [Content_Types].xml nor references them from the master, so keeping them makes
// PowerPoint report the package as needing repair.
func TestMergeWithExistingDropsStaleDesignParts(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := writeZipFile(existingPath, map[string]string{
		// Design parts from a previous, richer template.
		"ppt/slideLayouts/slideLayout20.xml":            "stale layout",
		"ppt/slideLayouts/_rels/slideLayout20.xml.rels": "stale layout rels",
		"ppt/slideMasters/slideMaster2.xml":             "stale master",
		"ppt/theme/theme9.xml":                          "stale theme",
		"ppt/media/image99.png":                         "stale media",
		// A part slidown does not manage: must be preserved.
		"customXml/item1.xml": "keep me",
	}); err != nil {
		t.Fatalf("write existing zip: %v", err)
	}

	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{Placeholder: PlaceholderTitle, IsPlaceholder: true, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: "new title"}}}}})
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	merged, err := MergeWithExisting(existingPath, buf.Bytes())
	if err != nil {
		t.Fatalf("MergeWithExisting: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(merged), int64(len(merged)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	parts := map[string]bool{}
	for _, f := range zr.File {
		parts[f.Name] = true
	}

	stale := []string{
		"ppt/slideLayouts/slideLayout20.xml",
		"ppt/slideLayouts/_rels/slideLayout20.xml.rels",
		"ppt/slideMasters/slideMaster2.xml",
		"ppt/theme/theme9.xml",
		"ppt/media/image99.png",
	}
	for _, name := range stale {
		if parts[name] {
			t.Errorf("stale design part %q was carried over into the merged package", name)
		}
	}
	if !parts["customXml/item1.xml"] {
		t.Errorf("unmanaged custom part customXml/item1.xml was not preserved")
	}
}

func writeZipFile(path string, entries map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// TestMergeWithExistingNoDuplicateEntries guards against the merge emitting an
// old-only part twice (which produced duplicate ZIP members and a corrupt
// package when the slide count shrank).
func TestMergeWithExistingNoDuplicateEntries(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.pptx")
	if err := writeZipFile(existingPath, map[string]string{
		"ppt/slides/slide1.xml":            "old1",
		"ppt/slides/slide2.xml":            "old2",
		"ppt/slides/slide3.xml":            "old3",
		"ppt/slides/_rels/slide3.xml.rels": "old3 rels",
		"foo/extra.txt":                    "extra",
	}); err != nil {
		t.Fatalf("write existing zip: %v", err)
	}

	// New package only has slide1 and slide2 (the deck shrank to two slides).
	newPPTX, err := zipFromParts(
		[]string{"ppt/slides/slide1.xml", "ppt/slides/slide2.xml"},
		map[string][]byte{
			"ppt/slides/slide1.xml": []byte("new1"),
			"ppt/slides/slide2.xml": []byte("new2"),
		},
	)
	if err != nil {
		t.Fatalf("zipFromParts: %v", err)
	}

	merged, err := MergeWithExisting(existingPath, newPPTX)
	if err != nil {
		t.Fatalf("MergeWithExisting: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(merged), int64(len(merged)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	seen := map[string]int{}
	for _, f := range zr.File {
		seen[f.Name]++
	}
	for name, n := range seen {
		if n > 1 {
			t.Errorf("duplicate ZIP entry %q appears %d times", name, n)
		}
	}
}
