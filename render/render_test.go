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

const sampleMarkdown = `# Title One

## Subtitle

- alpha
- **bold item**
  - nested ~~struck~~
- ` + "`code item`" + `

---

# Second Slide

Just a paragraph with a [link](https://example.com).
`

func renderMarkdown(t *testing.T) map[string]string {
	t.Helper()
	parsed, err := md.Parse("", []byte(sampleMarkdown), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}
	if len(slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(slides))
	}

	var buf bytes.Buffer
	if _, err := ToPresentation(slides).WriteTo(&buf); err != nil {
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
	return parts
}

func TestMarkdownToPptxEndToEnd(t *testing.T) {
	parts := renderMarkdown(t)

	s1, ok := parts["ppt/slides/slide1.xml"]
	if !ok {
		t.Fatal("slide1.xml missing")
	}
	for _, sub := range []string{
		`<p:ph type="title"/>`,
		`<a:t>Title One</a:t>`,
		`<p:ph type="body" idx="1"/>`,
		`<a:t>Subtitle</a:t>`, // subtitle rendered into body
		`<a:t>alpha</a:t>`,
		`b="1"`,               // bold item
		`strike="sngStrike"`,  // strikethrough via ~~ -> del
		`typeface="Consolas"`, // code item
		`lvl="1"`,             // nested
		`<a:buChar`,           // bullets
	} {
		if !strings.Contains(s1, sub) {
			t.Errorf("slide1.xml missing %q", sub)
		}
	}

	s2, ok := parts["ppt/slides/slide2.xml"]
	if !ok {
		t.Fatal("slide2.xml missing")
	}
	if !strings.Contains(s2, `<a:t>Second Slide</a:t>`) {
		t.Errorf("slide2.xml missing title text")
	}
	if !strings.Contains(s2, `<a:hlinkClick`) {
		t.Errorf("slide2.xml missing hyperlink")
	}
	rels2 := parts["ppt/slides/_rels/slide2.xml.rels"]
	if !strings.Contains(rels2, "https://example.com") {
		t.Errorf("slide2 rels missing hyperlink target: %s", rels2)
	}
}
