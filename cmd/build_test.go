package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	deck "github.com/Songmu/slidown"
	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/render"
)

func TestSlidesEquivalentForUpdateIgnoresDefaultLayout(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	mdPath := filepath.Join(t.TempDir(), "roundtrip.md")
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

	tmp, err := os.CreateTemp(t.TempDir(), "*.pptx")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := render.ToPresentation(slides).WriteTo(tmp); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	readSlides, _, err := deck.ReadSlidesFromPPTX(tmp.Name())
	if err != nil {
		t.Fatalf("ReadSlidesFromPPTX: %v", err)
	}
	if !slidesEquivalentForUpdate(slides, readSlides) {
		t.Fatalf("slidesEquivalentForUpdate returned false")
	}
}

func TestWritePresentationUpdatesExistingFile(t *testing.T) {
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
	updated, err := writePresentation(out, buf.Bytes(), slides, true)
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

	updated, err = writePresentation(out, buf.Bytes(), slides, true)
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

	updated, err = writePresentation(out, buf2.Bytes(), slides2, true)
	if err != nil {
		t.Fatalf("writePresentation changed deck: %v", err)
	}
	if !updated {
		t.Fatalf("expected changed deck to update existing file")
	}
	readSlides, _, err := deck.ReadSlidesFromPPTX(out)
	if err != nil {
		t.Fatalf("ReadSlidesFromPPTX updated deck: %v", err)
	}
	if len(readSlides) != 1 || len(readSlides[0].Titles) != 1 || readSlides[0].Titles[0] != "Changed" {
		t.Fatalf("unexpected updated slides: %#v", readSlides)
	}
}
