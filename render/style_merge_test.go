package render

import (
	"testing"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/pptx"
)

// TestBlockQuoteCustomStyleMergesWithInline verifies that a custom "blockquote"
// style is applied as a base that inline emphasis overrides, so a bold fragment
// inside a block quote keeps its bold while still gaining the block style's
// other properties. This mirrors deck, where the block base style is applied
// first and inline styles win.
func TestBlockQuoteCustomStyleMergesWithInline(t *testing.T) {
	c := &converter{styles: map[string]pptx.StyleSpec{
		"blockquote": {Italic: true, Color: "336699"},
	}}
	bq := &slidown.BlockQuote{Paragraphs: []*slidown.Paragraph{
		{Fragments: []*slidown.Fragment{{Value: "x", Bold: true}}},
	}}
	paras := c.convertBlockQuote(bq)
	if len(paras) != 1 || len(paras[0].Runs) != 1 {
		t.Fatalf("unexpected paragraphs/runs: %+v", paras)
	}
	r := paras[0].Runs[0]
	if !r.Bold {
		t.Errorf("inline bold was lost by the custom blockquote style")
	}
	if !r.Italic {
		t.Errorf("blockquote style italic was not applied")
	}
	if r.Color != "336699" {
		t.Errorf("blockquote style color not filled in, got %q", r.Color)
	}
}

// TestBlockQuoteDefaultItalicUnchanged ensures the default (no custom style)
// blockquote path still renders italic without affecting other properties.
func TestBlockQuoteDefaultItalicUnchanged(t *testing.T) {
	c := &converter{}
	bq := &slidown.BlockQuote{Paragraphs: []*slidown.Paragraph{
		{Fragments: []*slidown.Fragment{{Value: "x", Bold: true}}},
	}}
	r := c.convertBlockQuote(bq)[0].Runs[0]
	if !r.Italic || !r.Bold {
		t.Errorf("default blockquote should keep inline bold and add italic, got %+v", r)
	}
}
