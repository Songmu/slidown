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
