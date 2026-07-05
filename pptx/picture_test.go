package pptx

import (
	"strings"
	"testing"
)

func TestRenderPictureEmitsPlaceholder(t *testing.T) {
	pic := &Picture{
		Data: []byte("fake"),
		Ext:  "png",
		X:    1, Y: 2, W: 3, H: 4,
		IsPlaceholder:  true,
		Placeholder:    PlaceholderPic,
		PlaceholderIdx: 2,
	}
	var rels []slideRel
	var media []mediaPart
	relIdx, mediaIdx := 0, 0
	out := renderPicture(pic, 5, &relIdx, &rels, &mediaIdx, &media)

	if !strings.Contains(out, `<p:ph type="pic" idx="2"/>`) {
		t.Errorf("expected pic placeholder element, got: %s", out)
	}
	// The <p:ph> must live inside <p:nvPr> of the picture's <p:nvPicPr>.
	phIdx := strings.Index(out, `<p:ph`)
	nvprCloseIdx := strings.Index(out, `</p:nvPr>`)
	if phIdx < 0 || nvprCloseIdx < 0 || phIdx > nvprCloseIdx {
		t.Errorf("<p:ph> not positioned within <p:nvPr>: %s", out)
	}
}

func TestRenderPictureOmitsPlaceholderWhenNotSet(t *testing.T) {
	pic := &Picture{Data: []byte("fake"), Ext: "png", W: 3, H: 4}
	var rels []slideRel
	var media []mediaPart
	relIdx, mediaIdx := 0, 0
	out := renderPicture(pic, 1, &relIdx, &rels, &mediaIdx, &media)

	if strings.Contains(out, `<p:ph`) {
		t.Errorf("did not expect placeholder element for plain picture, got: %s", out)
	}
	if !strings.Contains(out, `<p:nvPr/>`) {
		t.Errorf("expected empty <p:nvPr/> for plain picture, got: %s", out)
	}
}
