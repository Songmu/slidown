package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Songmu/slidown/internal/testutil"
	"github.com/Songmu/slidown/pptx"
)

func TestClassifyPlaceholders(t *testing.T) {
	type subtitleExpectation struct {
		kind     string
		idx      int
		fromHint bool
	}
	for _, tc := range []struct {
		name         string
		placeholders []*pptx.PlaceholderInfo
		subtitles    []subtitleExpectation
		bodies       []int
	}{
		{
			name: "PrefersRealSubTitle",
			placeholders: []*pptx.PlaceholderInfo{
				{Type: "subTitle", Idx: 1, Name: "Subtitle 1"},
				{Type: "body", Idx: 2, Name: "Content 1"},
			},
			subtitles: []subtitleExpectation{{"subTitle", 1, false}},
			bodies:    []int{2},
		},
		{
			name: "PromotesSubtitleHintByName",
			placeholders: []*pptx.PlaceholderInfo{
				{Type: "body", Idx: 1, Name: "Subtitle 1"},
				{Type: "body", Idx: 2, Name: "Content Placeholder 2"},
			},
			subtitles: []subtitleExpectation{{"body", 1, true}},
			bodies:    []int{2},
		},
		{
			name: "PromotesSubtitleHintByPrompt",
			placeholders: []*pptx.PlaceholderInfo{
				{Type: "body", Idx: 1, Name: "Text Placeholder 2", Prompt: "Please enter the SUBTITLE here"},
			},
			subtitles: []subtitleExpectation{{"body", 1, true}},
		},
		{
			name: "NoSubtitleHint",
			placeholders: []*pptx.PlaceholderInfo{
				{Type: "body", Idx: 1, Name: "Content Placeholder 1", Prompt: "Click to edit"},
			},
			bodies: []int{1},
		},
		{
			name: "MultipleBodies",
			placeholders: []*pptx.PlaceholderInfo{
				{Type: "body", Idx: 1, Name: "Content Placeholder 1"},
				{Type: "body", Idx: 2, Name: "Content Placeholder 2"},
			},
			bodies: []int{1, 2},
		},
		{
			name: "MultipleSubtitleSlots",
			placeholders: []*pptx.PlaceholderInfo{
				{Type: "subTitle", Idx: 1, Name: "Subtitle 1"},
				{Type: "body", Idx: 2, Name: "Subtitle 2"},
				{Type: "body", Idx: 3, Name: "Content Placeholder 3"},
			},
			subtitles: []subtitleExpectation{{"subTitle", 1, false}, {"body", 2, true}},
			bodies:    []int{3},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			placeholders := append([]*pptx.PlaceholderInfo{{Type: "title", Name: "Title 1"}}, tc.placeholders...)
			title, subs, bodies := classifyPlaceholders(&pptx.LayoutInfo{Placeholders: placeholders})
			if title == nil || title.Type != "title" {
				t.Errorf("title = %+v, want type=title", title)
			}
			if len(subs) != len(tc.subtitles) {
				t.Fatalf("subs = %+v, want %d slots", subs, len(tc.subtitles))
			}
			for i, want := range tc.subtitles {
				got := subtitleExpectation{subs[i].ph.Type, subs[i].ph.Idx, subs[i].fromHint}
				if got != want {
					t.Errorf("subs[%d] = %+v, want %+v", i, got, want)
				}
			}
			if len(bodies) != len(tc.bodies) {
				t.Fatalf("bodies = %+v, want %d entries", bodies, len(tc.bodies))
			}
			for i, want := range tc.bodies {
				if bodies[i].Idx != want {
					t.Errorf("bodies[%d].Idx = %d, want %d", i, bodies[i].Idx, want)
				}
			}
		})
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
	slideXML := testutil.ReadZIPStrings(t, buf.Bytes())["ppt/slides/slide1.xml"]
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
