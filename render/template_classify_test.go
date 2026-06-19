package render

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Songmu/slidown/pptx"
)

func TestClassifyPlaceholdersPrefersRealSubTitle(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "subTitle", Idx: 1, Name: "Subtitle 1"},
			{Type: "body", Idx: 2, Name: "Content 1"},
		},
	}
	title, sub, fromHint, body := classifyPlaceholders(l)
	if title == nil || title.Type != "title" {
		t.Errorf("title = %+v, want type=title", title)
	}
	if sub == nil || sub.Type != "subTitle" {
		t.Errorf("sub = %+v, want type=subTitle", sub)
	}
	if fromHint {
		t.Errorf("fromHint = true; expected real subTitle to not be marked as hint-derived")
	}
	if body == nil || body.Idx != 2 {
		t.Errorf("body = %+v, want idx=2", body)
	}
}

func TestClassifyPlaceholdersPromotesSubtitleHintByName(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "body", Idx: 1, Name: "Subtitle 1"}, // hinted via Selection Pane name
			{Type: "body", Idx: 2, Name: "Content Placeholder 2"},
		},
	}
	title, sub, fromHint, body := classifyPlaceholders(l)
	if title == nil || title.Type != "title" {
		t.Errorf("title = %+v", title)
	}
	if sub == nil || sub.Idx != 1 {
		t.Errorf("sub = %+v, want hint-promoted body with idx=1", sub)
	}
	if !fromHint {
		t.Errorf("fromHint = false; expected hint-promoted sub")
	}
	if body == nil || body.Idx != 2 {
		t.Errorf("body = %+v, want remaining body with idx=2", body)
	}
}

func TestClassifyPlaceholdersPromotesSubtitleHintByPrompt(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "body", Idx: 1, Name: "Text Placeholder 2", Prompt: "Please enter the SUBTITLE here"},
		},
	}
	_, sub, fromHint, body := classifyPlaceholders(l)
	if sub == nil || sub.Idx != 1 {
		t.Errorf("sub = %+v, want hint-promoted body via prompt", sub)
	}
	if !fromHint {
		t.Errorf("fromHint = false; expected hint-derived sub")
	}
	if body != nil {
		t.Errorf("body = %+v, want nil (only one candidate, consumed by hint)", body)
	}
}

func TestClassifyPlaceholdersNoSubtitleHint(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "body", Idx: 1, Name: "Content Placeholder 1", Prompt: "Click to edit"},
		},
	}
	_, sub, fromHint, body := classifyPlaceholders(l)
	if sub != nil {
		t.Errorf("sub = %+v, want nil with no hint", sub)
	}
	if fromHint {
		t.Errorf("fromHint = true; expected false")
	}
	if body == nil || body.Idx != 1 {
		t.Errorf("body = %+v, want idx=1", body)
	}
}

// TestRenderHintedSubtitleEmitsRoleMarker exercises the synthesis path: a
// shape configured the way renderSlideWithLayout would for a hint-promoted
// subtitle (body placeholder type + Role="subTitle") must serialise with the
// slidown role marker embedded under <p:nvPr>, while keeping the underlying
// placeholder type intact.
func TestRenderHintedSubtitleEmitsRoleMarker(t *testing.T) {
	sh := &pptx.Shape{
		Name:           "Subtitle",
		IsPlaceholder:  true,
		Placeholder:    pptx.PlaceholderBody,
		PlaceholderIdx: 1,
		Role:           "subTitle",
		Paragraphs:     []*pptx.Paragraph{{Runs: []*pptx.Run{{Text: "副題"}}}},
	}
	p := pptx.New()
	slide := p.AddSlide()
	slide.AddShape(sh)

	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	var slideXML string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			slideXML = string(b)
			break
		}
	}
	if slideXML == "" {
		t.Fatal("ppt/slides/slide1.xml missing from output")
	}
	if !strings.Contains(slideXML, `role="subTitle"`) {
		t.Errorf("rendered slide missing role=\"subTitle\" marker:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `<p:ph type="body" idx="1"/>`) {
		t.Errorf("rendered shape did not preserve the underlying body placeholder type:\n%s", slideXML)
	}
}

