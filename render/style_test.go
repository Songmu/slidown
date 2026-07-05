package render

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
)

const styleFixtureMarkdown = `# Styled content

- normal **bold text** here
- inline ` + "`codeword`" + ` sample
- a <span class="notice">notice span</span> here

| A | B |
| --- | --- |
| one | two |
`

func TestToPresentationWithTemplateAppliesStyleLayoutFixture(t *testing.T) {
	tmpl, err := pptx.LoadTemplate("../testdata/template_style.pptx")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	for _, layout := range tmpl.Layouts {
		if layout.Name == "style" {
			t.Fatalf("style layout should be filtered from template layouts: %+v", layout)
		}
	}

	parsed, err := md.Parse("", []byte(styleFixtureMarkdown), nil)
	if err != nil {
		t.Fatalf("md.Parse: %v", err)
	}
	slides, err := parsed.ToSlides(context.Background(), "")
	if err != nil {
		t.Fatalf("ToSlides: %v", err)
	}

	var buf bytes.Buffer
	if _, err := ToPresentationWithTemplate(slides, tmpl).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	slideXML := string(zipParts(t, buf.Bytes())["ppt/slides/slide1.xml"])
	if slideXML == "" {
		t.Fatal("ppt/slides/slide1.xml missing from output")
	}

	codeRun := xmlRunContaining(t, slideXML, "codeword")
	for _, sub := range []string{
		`srgbClr val="FF0000"`,
		`highlight`,
		`srgbClr val="FFFF00"`,
		`typeface="Courier New"`,
	} {
		if !strings.Contains(codeRun, sub) {
			t.Errorf("code run missing %q: %s", sub, codeRun)
		}
	}

	noticeRun := xmlRunContaining(t, slideXML, "notice span")
	for _, sub := range []string{`srgbClr val="0000FF"`, `b="1"`} {
		if !strings.Contains(noticeRun, sub) {
			t.Errorf("notice run missing %q: %s", sub, noticeRun)
		}
	}

	for _, sub := range []string{`srgbClr val="4472C4"`, `anchor="ctr"`} {
		if !strings.Contains(slideXML, sub) {
			t.Errorf("table header cell missing %q", sub)
		}
	}
}

func xmlRunContaining(t *testing.T, xml, text string) string {
	t.Helper()
	textIdx := strings.Index(xml, text)
	if textIdx < 0 {
		t.Fatalf("slide XML missing text %q", text)
	}
	start := strings.LastIndex(xml[:textIdx], "<a:r>")
	endRel := strings.Index(xml[textIdx:], "</a:r>")
	if start < 0 || endRel < 0 {
		t.Fatalf("could not find run around %q", text)
	}
	return xml[start : textIdx+endRel+len("</a:r>")]
}

func TestConverterInlineDefaultStyles(t *testing.T) {
	c := &converter{}

	for _, tt := range []struct {
		name     string
		style    string
		baseline string
	}{
		{name: "sup", style: "sup", baseline: "super"},
		{name: "sub", style: "sub", baseline: "sub"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := c.convertFragment(&slidown.Fragment{Value: "x", StyleName: tt.style})
			if r.Baseline != tt.baseline {
				t.Fatalf("Baseline = %q, want %q", r.Baseline, tt.baseline)
			}
		})
	}

	r := c.convertFragment(&slidown.Fragment{Value: "x", Code: true, Bold: true, Italic: true, Link: "https://example.com", StyleName: "missing"})
	if !r.Code || !r.Bold || !r.Italic || r.Link != "https://example.com" {
		t.Fatalf("built-in flags changed: %+v", r)
	}
	if r.Color != "" || r.BgColor != "" || r.FontFamily != "" || r.Baseline != "" {
		t.Fatalf("built-in path should not add custom fields: %+v", r)
	}
}

func TestConverterInlineCustomStylesApplyInDeckOrder(t *testing.T) {
	c := &converter{styles: map[string]pptx.StyleSpec{
		"code": {
			Color:      "111111",
			BgColor:    "EEEEEE",
			FontFamily: "Menlo",
			Baseline:   "super",
		},
		"bold": {
			Bold:    true,
			Italic:  true,
			Color:   "222222",
			BgColor: "DDDDDD",
		},
		"callout": {
			Underline:  true,
			Color:      "333333",
			BgColor:    "CCCCCC",
			FontFamily: "Aptos",
			Baseline:   "sub",
		},
	}}

	r := c.convertFragment(&slidown.Fragment{
		Value:     "x",
		Code:      true,
		Bold:      true,
		StyleName: "callout",
	})
	if r.Code {
		t.Fatalf("custom font style should clear Run.Code: %+v", r)
	}
	if r.Bold || r.Italic || !r.Underline || r.Strike {
		t.Fatalf("later custom class should replace earlier bool fields: %+v", r)
	}
	if r.Color != "333333" || r.BgColor != "CCCCCC" || r.FontFamily != "Aptos" || r.Baseline != "sub" {
		t.Fatalf("custom class fields not applied last: %+v", r)
	}
}

func TestConverterCustomCodeAndLinkStyles(t *testing.T) {
	c := &converter{styles: map[string]pptx.StyleSpec{
		"code": {
			Color:      "112233",
			BgColor:    "FFEEDD",
			FontFamily: "Monaspace",
			Baseline:   "super",
		},
		"link": {
			Underline: true,
			Color:     "445566",
		},
	}}

	code := c.convertFragment(&slidown.Fragment{Value: "code", Code: true})
	if code.Code || code.FontFamily != "Monaspace" || code.Color != "112233" || code.BgColor != "FFEEDD" || code.Baseline != "super" {
		t.Fatalf("custom code style not applied: %+v", code)
	}

	link := c.convertFragment(&slidown.Fragment{Value: "link", Link: "https://example.com"})
	if link.Link != "https://example.com" || !link.Underline || link.Color != "445566" {
		t.Fatalf("custom link style should apply while preserving URL: %+v", link)
	}
}

func TestConverterCustomBlockQuoteStyle(t *testing.T) {
	bq := &slidown.BlockQuote{Paragraphs: []*slidown.Paragraph{{
		Fragments: []*slidown.Fragment{{Value: "quote"}},
	}}}

	defaultRuns := (&converter{}).convertBlockQuote(bq)[0].Runs
	if len(defaultRuns) != 1 || !defaultRuns[0].Italic {
		t.Fatalf("default blockquote should be italic: %+v", defaultRuns)
	}

	c := &converter{styles: map[string]pptx.StyleSpec{
		"blockquote": {Bold: true, Color: "123456"},
	}}
	runs := c.convertBlockQuote(bq)[0].Runs
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if !runs[0].Bold || runs[0].Italic || runs[0].Color != "123456" {
		t.Fatalf("custom blockquote style not applied: %+v", runs[0])
	}
}

func TestRenderTablesAtPassesTemplateTableStyle(t *testing.T) {
	style := &pptx.TableStyleSpec{}
	sl := &pptx.Slide{}
	(&converter{}).renderTablesAt(sl, []*slidown.Table{{
		Rows: []*slidown.TableRow{{
			Cells: []*slidown.TableCell{{Fragments: []*slidown.Fragment{{Value: "cell"}}}},
		}},
	}}, 1, 2, 3, style, false)

	if len(sl.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(sl.Tables))
	}
	if sl.Tables[0].Style != style {
		t.Fatalf("table style was not passed through")
	}
}
