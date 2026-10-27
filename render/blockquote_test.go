package render

import (
	"strings"
	"testing"
)

func TestRenderBlockQuote(t *testing.T) {
	src := "# Quote slide\n\n> wisdom here\n"
	slides := markdownSlides(t, src)
	slide := string(renderSlidesToParts(t, slides)["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<a:t>wisdom here</a:t>") {
		t.Errorf("blockquote text missing")
	}
	if !strings.Contains(slide, `i="1"`) {
		t.Errorf("blockquote should be italic")
	}
}
