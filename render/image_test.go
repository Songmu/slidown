package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/Songmu/slidown"
)

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func TestRenderImage(t *testing.T) {
	imgData := makePNG(t, 100, 200) // portrait 1:2
	img, err := slidown.NewImageFromCodeBlock(bytes.NewReader(imgData))
	if err != nil {
		t.Fatalf("NewImageFromCodeBlock: %v", err)
	}

	slides := slidown.Slides{
		{
			Titles: []string{"Image slide"},
			Images: []*slidown.Image{img},
		},
	}

	parts := renderSlidesToParts(t, slides)

	if _, ok := parts["ppt/media/image1.png"]; !ok {
		t.Errorf("missing embedded media part; have: %v", keys(parts))
	}
	slide := string(parts["ppt/slides/slide1.xml"])
	if !strings.Contains(slide, "<p:pic>") {
		t.Errorf("slide missing picture element")
	}
	if !strings.Contains(slide, `<a:blip r:embed=`) {
		t.Errorf("slide missing blip embed")
	}
	rels := string(parts["ppt/slides/_rels/slide1.xml.rels"])
	if !strings.Contains(rels, "../media/image1.png") {
		t.Errorf("slide rels missing image relationship: %s", rels)
	}
	if !strings.Contains(rels, "relationships/image") {
		t.Errorf("slide rels missing image relationship type")
	}
}

func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
