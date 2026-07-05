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
	title, subs, bodies := classifyPlaceholders(l)
	if title == nil || title.Type != "title" {
		t.Errorf("title = %+v, want type=title", title)
	}
	if len(subs) != 1 || subs[0].ph.Type != "subTitle" {
		t.Fatalf("subs = %+v, want one type=subTitle", subs)
	}
	if subs[0].fromHint {
		t.Errorf("fromHint = true; expected real subTitle to not be marked as hint-derived")
	}
	if len(bodies) != 1 || bodies[0].Idx != 2 {
		t.Errorf("bodies = %+v, want one entry with idx=2", bodies)
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
	title, subs, bodies := classifyPlaceholders(l)
	if title == nil || title.Type != "title" {
		t.Errorf("title = %+v", title)
	}
	if len(subs) != 1 || subs[0].ph.Idx != 1 {
		t.Fatalf("subs = %+v, want hint-promoted body with idx=1", subs)
	}
	if !subs[0].fromHint {
		t.Errorf("fromHint = false; expected hint-promoted sub")
	}
	if len(bodies) != 1 || bodies[0].Idx != 2 {
		t.Errorf("bodies = %+v, want one entry with idx=2", bodies)
	}
}

func TestClassifyPlaceholdersPromotesSubtitleHintByPrompt(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "body", Idx: 1, Name: "Text Placeholder 2", Prompt: "Please enter the SUBTITLE here"},
		},
	}
	_, subs, bodies := classifyPlaceholders(l)
	if len(subs) != 1 || subs[0].ph.Idx != 1 {
		t.Fatalf("subs = %+v, want hint-promoted body via prompt", subs)
	}
	if !subs[0].fromHint {
		t.Errorf("fromHint = false; expected hint-derived sub")
	}
	if len(bodies) != 0 {
		t.Errorf("bodies = %+v, want empty (only one candidate, consumed by hint)", bodies)
	}
}

func TestClassifyPlaceholdersNoSubtitleHint(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "body", Idx: 1, Name: "Content Placeholder 1", Prompt: "Click to edit"},
		},
	}
	_, subs, bodies := classifyPlaceholders(l)
	if len(subs) != 0 {
		t.Errorf("subs = %+v, want none with no hint", subs)
	}
	if len(bodies) != 1 || bodies[0].Idx != 1 {
		t.Errorf("bodies = %+v, want one entry with idx=1", bodies)
	}
}

func TestClassifyPlaceholdersMultipleBodies(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "body", Idx: 1, Name: "Content Placeholder 1"},
			{Type: "body", Idx: 2, Name: "Content Placeholder 2"},
		},
	}
	_, subs, bodies := classifyPlaceholders(l)
	if len(subs) != 0 {
		t.Errorf("subs = %+v, want none", subs)
	}
	if len(bodies) != 2 {
		t.Fatalf("bodies = %+v, want 2 entries", bodies)
	}
	if bodies[0].Idx != 1 || bodies[1].Idx != 2 {
		t.Errorf("bodies idx = %d, %d; want 1, 2", bodies[0].Idx, bodies[1].Idx)
	}
}

// TestClassifyPlaceholdersMultipleSubtitleSlots verifies that a real subTitle
// placeholder and a hint-named body placeholder can coexist as multiple
// subtitle slots, preserving the layout's shape-tree order, while unrelated
// body placeholders remain bodies.
func TestClassifyPlaceholdersMultipleSubtitleSlots(t *testing.T) {
	l := &pptx.LayoutInfo{
		Placeholders: []*pptx.PlaceholderInfo{
			{Type: "title", Name: "Title 1"},
			{Type: "subTitle", Idx: 1, Name: "Subtitle 1"},
			{Type: "body", Idx: 2, Name: "Subtitle 2"}, // hint-promoted
			{Type: "body", Idx: 3, Name: "Content Placeholder 3"},
		},
	}
	_, subs, bodies := classifyPlaceholders(l)
	if len(subs) != 2 {
		t.Fatalf("subs = %+v, want 2 slots", subs)
	}
	if subs[0].ph.Idx != 1 || subs[0].fromHint {
		t.Errorf("subs[0] = %+v, want real subTitle idx=1", subs[0])
	}
	if subs[1].ph.Idx != 2 || !subs[1].fromHint {
		t.Errorf("subs[1] = %+v, want hint body idx=2", subs[1])
	}
	if len(bodies) != 1 || bodies[0].Idx != 3 {
		t.Errorf("bodies = %+v, want one entry with idx=3", bodies)
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
