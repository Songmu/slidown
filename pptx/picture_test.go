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
	if strings.Contains(out, `<a:extLst>`) || strings.Contains(out, `<asvg:svgBlip`) {
		t.Errorf("did not expect SVG blip extension for plain picture, got: %s", out)
	}
	if len(media) != 1 || media[0].name != "image1.png" {
		t.Errorf("plain picture media = %+v, want one png part", media)
	}
	if len(rels) != 1 || rels[0].target != "../media/image1.png" {
		t.Errorf("plain picture rels = %+v, want one raster image relationship", rels)
	}
}

func TestRenderPictureWithSVGData(t *testing.T) {
	pic := &Picture{
		Data:    []byte("png"),
		SVGData: []byte("<svg/>"),
		Ext:     "png",
		W:       3,
		H:       4,
	}
	var rels []slideRel
	var media []mediaPart
	relIdx, mediaIdx := 0, 0
	out := renderPicture(pic, 1, &relIdx, &rels, &mediaIdx, &media)

	for _, want := range []string{
		`<a:blip r:embed="rId1"><a:extLst><a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">`,
		`<asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main" r:embed="rId2"/>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in: %s", want, out)
		}
	}
	if len(media) != 2 {
		t.Fatalf("media count = %d, want 2: %+v", len(media), media)
	}
	if media[0].name != "image1.png" || string(media[0].data) != "png" {
		t.Errorf("raster media = %+v, want image1.png", media[0])
	}
	if media[1].name != "image2.svg" || string(media[1].data) != "<svg/>" {
		t.Errorf("svg media = %+v, want image2.svg", media[1])
	}
	if len(rels) != 2 {
		t.Fatalf("rels count = %d, want 2: %+v", len(rels), rels)
	}
	if rels[0].target != "../media/image1.png" || rels[1].target != "../media/image2.svg" {
		t.Errorf("rels = %+v, want png and svg image targets", rels)
	}
}
