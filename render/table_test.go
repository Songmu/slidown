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

const tableMarkdown = `# Table slide

| Name | Role |
| --- | --- |
| Alice | Dev |
| Bob | Ops |
`

func TestRenderTable(t *testing.T) {
	parsed, err := md.Parse("", []byte(tableMarkdown), nil)
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
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	var slide string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			slide = string(b)
		}
	}
	for _, sub := range []string{
		`<a:tbl>`,
		`<a:tblGrid>`,
		`<a:gridCol`,
		`<a:t>Name</a:t>`,
		`<a:t>Alice</a:t>`,
		`<a:t>Ops</a:t>`,
		`D9E1F2`, // header fill
	} {
		if !strings.Contains(slide, sub) {
			t.Errorf("table slide missing %q", sub)
		}
	}
}
