package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func samplePresentation() *Presentation {
	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{
		Placeholder: PlaceholderTitle,
		Paragraphs: []*Paragraph{
			{Runs: []*Run{{Text: "Hello slidown"}}},
		},
	})
	s.AddShape(&Shape{
		Placeholder:    PlaceholderBody,
		PlaceholderIdx: 1,
		Paragraphs: []*Paragraph{
			{Bullet: true, Runs: []*Run{
				{Text: "plain "},
				{Text: "bold", Bold: true},
				{Text: " "},
				{Text: "code", Code: true},
			}},
			{Bullet: true, Level: 1, Runs: []*Run{
				{Text: "nested with "},
				{Text: "link", Link: "https://example.com"},
			}},
			{Bullet: true, Numbered: true, Runs: []*Run{{Text: "numbered item"}}},
		},
	})
	return p
}

func TestWriteToProducesValidZip(t *testing.T) {
	var buf bytes.Buffer
	if _, err := samplePresentation().WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}

	want := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels",
		"ppt/theme/theme1.xml",
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slides/slide1.xml",
		"ppt/slides/_rels/slide1.xml.rels",
	}
	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("missing part %q", name)
		}
	}

	slide := got["ppt/slides/slide1.xml"]
	for _, sub := range []string{
		`<p:ph type="title"/>`,
		`<p:ph type="body" idx="1"/>`,
		`<a:t>Hello slidown</a:t>`,
		`b="1"`,               // bold run
		`typeface="Noto Sans Mono"`, // code run
		`<a:buAutoNum`,        // numbered bullet
		`lvl="1"`,             // nested level
		`<a:hlinkClick`,       // hyperlink
	} {
		if !strings.Contains(slide, sub) {
			t.Errorf("slide1.xml missing %q", sub)
		}
	}

	rels := got["ppt/slides/_rels/slide1.xml.rels"]
	if !strings.Contains(rels, "slideLayout1.xml") {
		t.Errorf("slide rels missing layout relationship")
	}
	if !strings.Contains(rels, `TargetMode="External"`) || !strings.Contains(rels, "https://example.com") {
		t.Errorf("slide rels missing external hyperlink relationship: %s", rels)
	}
	// The hyperlink rId in the slide must not collide with the layout rId1.
	if strings.Contains(slide, `r:id="rId1"`) {
		t.Errorf("hyperlink relationship id collides with layout rId1")
	}
}

func TestWriteToWithSpeakerNotes(t *testing.T) {
	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{Placeholder: PlaceholderTitle, Paragraphs: []*Paragraph{{Runs: []*Run{{Text: "T"}}}}})
	s.Note = "line one\nline two"

	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}

	for _, name := range []string{
		"ppt/notesMasters/notesMaster1.xml",
		"ppt/notesSlides/notesSlide1.xml",
		"ppt/notesSlides/_rels/notesSlide1.xml.rels",
	} {
		if _, ok := parts[name]; !ok {
			t.Errorf("missing notes part %q", name)
		}
	}
	notes := parts["ppt/notesSlides/notesSlide1.xml"]
	if !strings.Contains(notes, "<a:t>line one</a:t>") || !strings.Contains(notes, "<a:t>line two</a:t>") {
		t.Errorf("notes slide missing note text: %s", notes)
	}
	if !strings.Contains(parts["[Content_Types].xml"], "notesSlide1.xml") {
		t.Errorf("content types missing notes slide override")
	}
	if !strings.Contains(parts["ppt/slides/_rels/slide1.xml.rels"], "notesSlide1.xml") {
		t.Errorf("slide rels missing notesSlide relationship")
	}
}

func TestPresentationTitleMetadata(t *testing.T) {
	cases := []struct {
		name  string
		title string
		want  string
	}{
		{"plain", "My Talk", "<dc:title>My Talk</dc:title>"},
		{"escaped", "A & B <x>", "<dc:title>A &amp; B &lt;x&gt;</dc:title>"},
		{"empty", "", "<dc:title></dc:title>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := samplePresentation()
			p.Title = tc.title
			var buf bytes.Buffer
			if _, err := p.WriteTo(&buf); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}
			zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatalf("zip: %v", err)
			}
			var core string
			for _, f := range zr.File {
				if f.Name == "docProps/core.xml" {
					rc, _ := f.Open()
					b, _ := io.ReadAll(rc)
					rc.Close()
					core = string(b)
				}
			}
			if !strings.Contains(core, tc.want) {
				t.Errorf("core.xml missing %q\ngot: %s", tc.want, core)
			}
		})
	}
}

// TestRunCodeLinkRPrOrder verifies that for a run that is both code and a
// hyperlink, the font (latin/cs) is emitted before hlinkClick, as the OOXML
// CT_TextCharacterProperties schema requires.
func TestRunCodeLinkRPrOrder(t *testing.T) {
	p := New()
	s := p.AddSlide()
	s.AddShape(&Shape{
		Placeholder: PlaceholderBody,
		Paragraphs: []*Paragraph{
			{Runs: []*Run{{Text: "code", Code: true, Link: "https://example.com"}}},
		},
	})
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip: %v", err)
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
	latin := strings.Index(slide, "<a:latin")
	hlink := strings.Index(slide, "<a:hlinkClick")
	if latin < 0 || hlink < 0 {
		t.Fatalf("expected both latin and hlinkClick in slide:\n%s", slide)
	}
	if latin > hlink {
		t.Errorf("latin (%d) must precede hlinkClick (%d) per the schema", latin, hlink)
	}
}
