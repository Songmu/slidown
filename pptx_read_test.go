package slidown_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/render"
)

func TestReadSlidesFromPPTXRoundTrip(t *testing.T) {
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

	readSlides, _, err := slidown.ReadSlidesFromPPTX(tmp.Name())
	if err != nil {
		t.Fatalf("ReadSlidesFromPPTX: %v", err)
	}
	if len(readSlides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(readSlides))
	}
	if got := readSlides[0].Titles; len(got) != 1 || got[0] != "Title" {
		t.Fatalf("unexpected titles: %#v", got)
	}
	if got := readSlides[0].Bodies[0].String(); got != "body\n" {
		t.Fatalf("unexpected body: %q", got)
	}
	if readSlides[0].Layout == "" {
		t.Fatalf("expected recovered layout to be non-empty")
	}
}
