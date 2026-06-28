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

const tableLinkMarkdown = `# Link in table

| Col |
| --- |
| [click](https://example.com) |
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

func TestRenderTableCellHyperlink(t *testing.T) {
	parsed, err := md.Parse("", []byte(tableLinkMarkdown), nil)
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

	var slideXML, relsXML string
	for _, f := range zr.File {
		switch f.Name {
		case "ppt/slides/slide1.xml":
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			slideXML = string(b)
		case "ppt/slides/_rels/slide1.xml.rels":
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			relsXML = string(b)
		}
	}

	// The slide XML must contain an hlinkClick referencing some rId.
	if !strings.Contains(slideXML, `hlinkClick`) {
		t.Errorf("slide XML missing hlinkClick: %s", slideXML)
	}

	// The rels file must contain the hyperlink target URL.
	if !strings.Contains(relsXML, "https://example.com") {
		t.Errorf("rels XML missing hyperlink target: %s", relsXML)
	}
	// The rels file must declare TargetMode="External" for the hyperlink.
	if !strings.Contains(relsXML, `TargetMode="External"`) {
		t.Errorf("rels XML missing TargetMode=External: %s", relsXML)
	}

	// The r:id used in hlinkClick must match a Relationship Id in the rels.
	// Extract the r:id value from the hlinkClick element.
	const hlinkPrefix = `r:id="`
	idx := strings.Index(slideXML, `hlinkClick`)
	if idx < 0 {
		t.Fatal("hlinkClick not found in slide XML")
	}
	ridStart := strings.Index(slideXML[idx:], hlinkPrefix)
	if ridStart < 0 {
		t.Fatalf("r:id not found after hlinkClick in: %s", slideXML[idx:])
	}
	ridStart += idx + len(hlinkPrefix)
	ridEnd := strings.Index(slideXML[ridStart:], `"`)
	if ridEnd < 0 {
		t.Fatal("unterminated r:id value")
	}
	rid := slideXML[ridStart : ridStart+ridEnd]

	// The same Id must appear in the rels tied to the hyperlink target.
	// Locate the <Relationship .../> element for this Id and validate its attributes.
	idNeedle := `Id="` + rid + `"`
	pos := strings.Index(relsXML, idNeedle)
	if pos < 0 {
		t.Fatalf("rels XML does not contain %s: %s", idNeedle, relsXML)
	}
	relStart := strings.LastIndex(relsXML[:pos], "<Relationship")
	if relStart < 0 {
		t.Fatalf("could not locate <Relationship> for %s: %s", idNeedle, relsXML)
	}
	relEnd := strings.Index(relsXML[pos:], "/>")
	if relEnd < 0 {
		t.Fatalf("unterminated <Relationship> for %s: %s", idNeedle, relsXML)
	}
	rel := relsXML[relStart : pos+relEnd+2]
	for _, sub := range []string{
		`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"`,
		`Target="https://example.com"`,
		`TargetMode="External"`,
	} {
		if !strings.Contains(rel, sub) {
			t.Errorf("rels <Relationship> for %s missing %q: %s", idNeedle, sub, rel)
		}
	}
}
