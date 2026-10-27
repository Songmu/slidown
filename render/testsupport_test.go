package render

import (
	"bytes"
	"context"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/internal/testutil"
	"github.com/Songmu/slidown/md"
)

func markdownSlides(t testing.TB, source string) slidown.Slides {
	t.Helper()
	parsed, err := md.Parse("", []byte(source), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	return slides
}

func renderSlidesToParts(t testing.TB, slides slidown.Slides) map[string][]byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := ToPresentation(slides).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	parts, _ := testutil.ReadZIP(t, buf.Bytes())
	return parts
}
