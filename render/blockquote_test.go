package render

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Songmu/slidown/md"
)

func TestRenderBlockQuote(t *testing.T) {
	src := "# Quote slide\n\n> wisdom here\n"
	parsed, err := md.Parse("", []byte(src), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	var buf bytes.Buffer
	if _, err := ToPresentation(slides).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	var slide string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			slide = string(b)
		}
	}
	if !strings.Contains(slide, "<a:t>wisdom here</a:t>") {
		t.Errorf("blockquote text missing")
	}
	if !strings.Contains(slide, `i="1"`) {
		t.Errorf("blockquote should be italic")
	}
}
