package svgshape

import (
	"strings"
	"testing"
	"time"

	"github.com/Songmu/slidown/pptx"
)

func mustConvert(t *testing.T, s string) *pptx.GroupShape {
	t.Helper()
	g, ok := Convert([]byte(s))
	if !ok || g == nil {
		t.Fatalf("Convert() ok=%v g=%v", ok, g)
	}
	return g
}
func near(t *testing.T, got, want int64) {
	t.Helper()
	if got < want-3 || got > want+3 {
		t.Fatalf("got %d want %d", got, want)
	}
}
func bounds(gp pptx.GeomPath) (minX, minY, maxX, maxY int64) {
	minX, minY = 1<<62, 1<<62
	for _, c := range gp.Cmds {
		for _, p := range c.Pts {
			if p.X < minX {
				minX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	return
}

func TestBasicShapes(t *testing.T) {
	cases := []struct {
		name, svg, color string
		geoms            int
	}{
		{"rect", `<svg viewBox="0 0 100 50"><rect x="10" y="5" width="20" height="10" fill="#f00"/></svg>`, "ff0000", 1},
		{"circle", `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="10" fill="blue"/></svg>`, "0000ff", 1},
		{"ellipse", `<svg viewBox="0 0 100 100"><ellipse cx="50" cy="50" rx="20" ry="10" fill="#00ff00"/></svg>`, "00ff00", 1},
		{"line", `<svg viewBox="0 0 100 100"><line x1="1" y1="2" x2="3" y2="4" stroke="black"/></svg>`, "", 1},
		{"polyline", `<svg viewBox="0 0 10 10"><polyline points="1,1 2,2 3,1" fill="none" stroke="red"/></svg>`, "", 1},
		{"polygon", `<svg viewBox="0 0 10 10"><polygon points="1,1 2,2 3,1" fill="rgb(255,0,0)"/></svg>`, "ff0000", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := mustConvert(t, tc.svg)
			if len(g.Geoms) != tc.geoms {
				t.Fatalf("geoms=%d", len(g.Geoms))
			}
			if tc.color != "" && g.Geoms[0].Fill.Color != tc.color {
				t.Fatalf("color=%s", g.Geoms[0].Fill.Color)
			}
		})
	}
	g := mustConvert(t, `<svg viewBox="0 0 100 50"><rect x="10" y="5" width="20" height="10" fill="#f00"/></svg>`)
	minX, minY, maxX, maxY := bounds(g.Geoms[0].Paths[0])
	near(t, minX, 10*9525)
	near(t, minY, 5*9525)
	near(t, maxX, 30*9525)
	near(t, maxY, 15*9525)
}

func TestPathCommands(t *testing.T) {
	g := mustConvert(t, `<svg viewBox="0 0 100 100"><path d="M0 0 L10 0 C10 1 11 2 12 3 Q13 4 15 6 z" fill="none" stroke="#000"/></svg>`)
	verbs := []pptx.PathVerb{pptx.MoveTo, pptx.LineTo, pptx.CubicTo, pptx.QuadTo, pptx.ClosePath}
	cmds := g.Geoms[0].Paths[0].Cmds
	for i, v := range verbs {
		if cmds[i].Verb != v {
			t.Fatalf("verb %d=%v", i, cmds[i].Verb)
		}
	}
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><path d="m1 1 l9 0 c1 1 2 2 3 3" fill="none" stroke="black"/></svg>`)
	near(t, g.Geoms[0].Paths[0].Cmds[1].Pts[0].X, 10*9525)
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><path d="M10 10 A10 10 0 0 1 30 10" fill="none" stroke="black"/></svg>`)
	found := false
	for _, c := range g.Geoms[0].Paths[0].Cmds {
		if c.Verb == pptx.CubicTo {
			found = true
		}
	}
	if !found {
		t.Fatal("arc did not become cubic")
	}
}

func TestTransformsAndGroups(t *testing.T) {
	g := mustConvert(t, `<svg viewBox="0 0 100 100"><rect transform="translate(10,20) scale(2)" x="1" y="1" width="1" height="1" fill="red"/></svg>`)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].X, 12*9525)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].Y, 22*9525)
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><rect transform="rotate(90)" x="1" y="0" width="1" height="1" fill="red"/></svg>`)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].X, 0)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].Y, 1*9525)
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><rect transform="matrix(1 0 0 1 5 6)" x="1" y="1" width="1" height="1" fill="red"/></svg>`)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].X, 6*9525)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].Y, 7*9525)
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><g fill="green" transform="translate(1,2)"><g transform="scale(2)"><rect x="1" y="1" width="1" height="1"/></g></g></svg>`)
	if g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("fill=%s", g.Geoms[0].Fill.Color)
	}
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].X, 3*9525)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].Y, 4*9525)
}

func TestGradientsUseCSSAndText(t *testing.T) {
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg"><stop offset="0" stop-color="red"/><stop offset="100%" stop-color="blue" stop-opacity=".5"/></linearGradient><radialGradient id="rg"><stop offset="0" stop-color="#fff"/></radialGradient></defs><rect width="5" height="5" fill="url(#lg)"/><circle cx="5" cy="5" r="2" fill="url(#rg)"/></svg>`)
	if g.Geoms[0].Fill.Kind != pptx.FillGradient || len(g.Geoms[0].Fill.Gradient.Stops) != 2 || g.Geoms[0].Fill.Gradient.Stops[0].Color != "ff0000" {
		t.Fatal("linear gradient not applied")
	}
	if g.Geoms[1].Fill.Gradient.Kind != pptx.GradientRadial {
		t.Fatal("radial gradient not applied")
	}
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><rect id="r" width="2" height="2" fill="red"/><symbol id="s"><circle cx="5" cy="5" r="1" fill="blue"/></symbol></defs><use href="#r" x="1" y="1"/><use href="#s"/></svg>`)
	if len(g.Geoms) != 2 || g.Geoms[1].Fill.Color != "0000ff" {
		t.Fatalf("use failed")
	}
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><style>.hot{fill:#0f0}</style><rect class="hot" width="1" height="1"/></svg>`)
	if g.Geoms[0].Fill.Color != "00ff00" {
		t.Fatalf("css fill=%s", g.Geoms[0].Fill.Color)
	}
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="10" y="20" fill="red" font-size="16" text-anchor="middle" font-family="Arial, sans-serif">Hi<tspan>!</tspan></text></svg>`)
	if len(g.Texts) != 1 || g.Texts[0].Paragraphs[0].Align != pptx.AlignCenter || g.Texts[0].Paragraphs[0].Runs[0].FontSize != 12 || g.Texts[0].Paragraphs[0].Runs[0].Color != "ff0000" {
		t.Fatalf("bad text: %#v", g.Texts[0])
	}
}

func TestColorsFallbackAndFillRule(t *testing.T) {
	for _, svg := range []string{`<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="rebeccapurple"/></svg>`, `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="#abc"/></svg>`, `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="#aabbcc"/></svg>`, `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="rgb(1,2,3)"/></svg>`} {
		mustConvert(t, svg)
	}
	bad := []string{`<svg viewBox="0 0 1 1"><clipPath/></svg>`, `<svg viewBox="0 0 1 1"><rect filter="url(#f)"/></svg>`, `<svg viewBox="0 0 1 1"><image href="x"/></svg>`, `<svg viewBox="0 0 1 1"><foo/></svg>`, `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="notacolor"/></svg>`, `<svg viewBox="0 0 1 1"><style>rect > .x{fill:red}</style><rect/></svg>`}
	for _, s := range bad {
		if _, ok := Convert([]byte(s)); ok {
			t.Fatalf("expected fallback: %s", s)
		}
	}
	// A single-subpath evenodd path is equivalent to nonzero and still converts.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><path fill-rule="evenodd" d="M0 0L10 0L10 10L0 10z"/></svg>`)
	if !g.Geoms[0].EvenOdd || len(g.Geoms[0].Paths) != 1 {
		t.Fatal("single-subpath evenodd failed")
	}
	// A filled evenodd path with holes (multiple subpaths) cannot be faithfully
	// reproduced with DrawingML's nonzero winding, so it falls back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><path fill-rule="evenodd" d="M0 0L10 0L10 10L0 10z M2 2L2 8L8 8L8 2z"/></svg>`)); ok {
		t.Fatal("expected fallback for filled evenodd donut")
	}
	// The same holed path with no fill (stroke only) converts, since winding
	// does not affect an unfilled outline.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><path fill="none" stroke="black" fill-rule="evenodd" d="M0 0L10 0L10 10L0 10z M2 2L2 8L8 8L8 2z"/></svg>`)); !ok {
		t.Fatal("expected unfilled evenodd donut to convert")
	}
}

// TestMalformedPathTerminates ensures malformed path data cannot spin the
// parser forever (progress guard). The whole test is bounded by a timeout so a
// regression manifests as a timeout rather than a hang.
func TestMalformedPathTerminates(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, d := range []string{"M0 0 Z 1", "M0 0 Lx", "Z", "M0 0 L", "M0 0 C 1 1", "garbage"} {
			// Must not hang; result (ok) is irrelevant here.
			Convert([]byte(`<svg viewBox="0 0 10 10"><path d="` + d + `"/></svg>`))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Convert did not terminate on malformed path data")
	}
}

func TestTransparentAndCurrentColor(t *testing.T) {
	// fill:transparent renders as no fill.
	g := mustConvert(t, `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="transparent"/></svg>`)
	if g.Geoms[0].Fill.Kind != pptx.FillNone {
		t.Fatalf("transparent should be no fill, got %#v", g.Geoms[0].Fill)
	}
	// currentColor resolves to the inherited color.
	g = mustConvert(t, `<svg viewBox="0 0 1 1"><g color="#00ff00"><rect width="1" height="1" fill="currentColor"/></g></svg>`)
	if g.Geoms[0].Fill.Kind != pptx.FillSolid || g.Geoms[0].Fill.Color != "00ff00" {
		t.Fatalf("currentColor should resolve to inherited color, got %#v", g.Geoms[0].Fill)
	}
}

func TestHiddenElementsNotRendered(t *testing.T) {
	g := mustConvert(t, `<svg viewBox="0 0 10 10">`+
		`<rect width="1" height="1" fill="red" display="none"/>`+
		`<rect width="1" height="1" fill="blue" style="visibility:hidden"/>`+
		`<rect width="1" height="1" fill="green"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("hidden elements should be skipped, got %#v", g.Geoms)
	}
}

func TestTextFillEdgeCases(t *testing.T) {
	// Text with fill:none is invisible and is simply skipped (no text emitted).
	g := mustConvert(t, `<svg viewBox="0 0 100 100"><text x="10" y="20" fill="none" font-size="10">Hi</text></svg>`)
	if len(g.Texts) != 0 {
		t.Fatalf("fill:none text should be skipped, got %d", len(g.Texts))
	}
	// Translucent text cannot be represented (pptx runs have no alpha) -> fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="10" y="20" fill="red" fill-opacity="0.5" font-size="10">Hi</text></svg>`)); ok {
		t.Fatal("expected fallback for translucent text")
	}
	// x beyond the viewBox must not yield a negative text-box width.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="500" y="20" fill="red" font-size="10">Hi</text></svg>`)
	if len(g.Texts) != 1 || g.Texts[0].W <= 0 {
		t.Fatalf("text width must stay positive, got %#v", g.Texts)
	}
}

func TestTransparentGradientStop(t *testing.T) {
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="g">`+
		`<stop offset="0" stop-color="red"/><stop offset="1" stop-color="transparent"/>`+
		`</linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
	gr := g.Geoms[0].Fill.Gradient
	if gr == nil || len(gr.Stops) != 2 {
		t.Fatalf("expected 2 gradient stops, got %#v", gr)
	}
	if gr.Stops[1].Alpha != 0 {
		t.Fatalf("transparent stop should have alpha 0, got %v", gr.Stops[1].Alpha)
	}
}

func TestCSSSpecificityPerMatchingSelector(t *testing.T) {
	// The rule "rect, #hot" must apply type-level specificity to an element that
	// only matches "rect", so a later class rule (higher specificity) wins.
	g := mustConvert(t, `<svg viewBox="0 0 10 10">`+
		`<style>rect, #hot{fill:#ff0000} .b{fill:#0000ff}</style>`+
		`<rect class="b" width="1" height="1"/></svg>`)
	if g.Geoms[0].Fill.Color != "0000ff" {
		t.Fatalf("class rule should win over type selector in a list, got %s", g.Geoms[0].Fill.Color)
	}
}

func TestDeeplyNestedFallsBack(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 10 10">`)
	for i := 0; i < maxDepth+5; i++ {
		b.WriteString("<g>")
	}
	b.WriteString(`<rect width="1" height="1"/>`)
	for i := 0; i < maxDepth+5; i++ {
		b.WriteString("</g>")
	}
	b.WriteString("</svg>")
	if _, ok := Convert([]byte(b.String())); ok {
		t.Fatal("expected fallback for excessively nested SVG")
	}
}

func TestNoViewBoxPerDimensionFallback(t *testing.T) {
	// width present, height omitted: width must be preserved, height defaults 150.
	g := mustConvert(t, `<svg xmlns="http://www.w3.org/2000/svg" width="200"><rect width="10" height="10" fill="red"/></svg>`)
	if g.ChW != round(200*emuPerUnit) {
		t.Fatalf("width should be preserved (200), got ChW=%d", g.ChW)
	}
	if g.ChH != round(150*emuPerUnit) {
		t.Fatalf("height should default to 150, got ChH=%d", g.ChH)
	}
}

func TestClipPathAndMaskAttrsFallback(t *testing.T) {
	// clip-path/mask via presentation attribute must trigger fallback even if
	// the referenced element is external/missing (so not caught as an element).
	for _, s := range []string{
		`<svg viewBox="0 0 10 10"><rect width="10" height="10" clip-path="url(ext.svg#c)"/></svg>`,
		`<svg viewBox="0 0 10 10"><rect width="10" height="10" mask="url(ext.svg#m)"/></svg>`,
	} {
		if _, ok := Convert([]byte(s)); ok {
			t.Fatalf("expected fallback for: %s", s)
		}
	}
}
