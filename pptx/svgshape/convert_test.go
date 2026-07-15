package svgshape

import (
	"fmt"
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
	g := mustConvert(t, `<svg viewBox="0 0 100 100"><path d="M1 1 L11 1 C11 2 12 3 13 4 Q14 5 16 7 z" fill="none" stroke="#000"/></svg>`)
	verbs := []pptx.PathVerb{pptx.MoveTo, pptx.LineTo, pptx.CubicTo, pptx.QuadTo, pptx.ClosePath}
	cmds := g.Geoms[0].Paths[0].Cmds
	for i, v := range verbs {
		if cmds[i].Verb != v {
			t.Fatalf("verb %d=%v", i, cmds[i].Verb)
		}
	}
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><path d="m1 1 l9 0 c1 1 2 2 3 3" fill="none" stroke="black"/></svg>`)
	near(t, g.Geoms[0].Paths[0].Cmds[1].Pts[0].X, 10*9525)
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><path d="M10 40 A10 10 0 0 1 30 40" fill="none" stroke="black"/></svg>`)
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
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><rect transform="translate(1,1) rotate(90)" x="1" y="0" width="1" height="1" fill="red"/></svg>`)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].X, 1*9525)
	near(t, g.Geoms[0].Paths[0].Cmds[0].Pts[0].Y, 2*9525)
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
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="10" y="20" fill="red" font-size="16" text-anchor="start" font-family="Arial, sans-serif">Hi<tspan>!</tspan></text></svg>`)
	if len(g.Texts) != 1 || g.Texts[0].Paragraphs[0].Align != pptx.AlignLeft || g.Texts[0].Paragraphs[0].Runs[0].FontSize != 12 || g.Texts[0].Paragraphs[0].Runs[0].Color != "ff0000" {
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
	// Any filled evenodd path falls back: DrawingML custom geometry only fills
	// with nonzero winding, which can differ even for a single self-intersecting
	// subpath.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><path fill-rule="evenodd" d="M0 0L10 0L10 10L0 10z"/></svg>`)); ok {
		t.Fatal("expected fallback for filled evenodd path")
	}
	// A filled evenodd path with holes (multiple subpaths) cannot be faithfully
	// reproduced with DrawingML's nonzero winding, so it falls back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><path fill-rule="evenodd" d="M0 0L10 0L10 10L0 10z M2 2L2 8L8 8L8 2z"/></svg>`)); ok {
		t.Fatal("expected fallback for filled evenodd donut")
	}
	// The same holed path with no fill (stroke only) converts, since winding
	// does not affect an unfilled outline (inset so the stroke stays inside the
	// viewport).
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><path fill="none" stroke="black" fill-rule="evenodd" d="M1 1L9 1L9 9L1 9z M3 3L3 7L7 7L7 3z"/></svg>`)); !ok {
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
	// x beyond the viewBox would render outside the (non-clipping) group, so it
	// now falls back rather than emitting an externally positioned box.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="500" y="20" fill="red" font-size="10">Hi</text></svg>`)); ok {
		t.Fatal("expected fallback for text positioned outside the viewport")
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

func TestReviewBatchFallbacks(t *testing.T) {
	// Leaf element opacity still applies (multiplied into fill alpha).
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><rect width="10" height="10" fill="red" opacity="0.5"/></svg>`)
	if a := g.Geoms[0].Fill.Alpha; a < 0.49 || a > 0.51 {
		t.Fatalf("leaf opacity should apply, got alpha=%v", a)
	}
	// CSS display:none must hide the matched element (resolved style, not just attrs).
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><style>.h{display:none}</style>`+
		`<rect class="h" width="1" height="1" fill="red"/><rect width="1" height="1" fill="green"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("CSS display:none should hide element, got %#v", g.Geoms)
	}
	for name, svg := range map[string]string{
		"group opacity":       `<svg viewBox="0 0 10 10"><g opacity="0.5"><rect width="1" height="1" fill="red"/></g></svg>`,
		"unsupported css":     `<svg viewBox="0 0 10 10"><rect width="1" height="1" style="mix-blend-mode:multiply"/></svg>`,
		"dash array":          `<svg viewBox="0 0 10 10"><rect width="1" height="1" fill="none" stroke="black" stroke-width="1" stroke-dasharray="1 2"/></svg>`,
		"radial geometry":     `<svg viewBox="0 0 10 10"><defs><radialGradient id="g" cx="0" cy="0" r="50%"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></radialGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`,
		"use width":           `<svg viewBox="0 0 10 10"><defs><rect id="r" width="2" height="2"/></defs><use href="#r" width="5" height="5"/></svg>`,
		"symbol viewbox":      `<svg viewBox="0 0 10 10"><defs><symbol id="s" viewBox="0 0 4 4"><rect width="4" height="4"/></symbol></defs><use href="#s"/></svg>`,
		"text scale":          `<svg viewBox="0 0 100 100"><text x="1" y="1" transform="scale(2)" fill="red" font-size="10">Hi</text></svg>`,
		"mixed text ordering": `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10">A<tspan>B</tspan>C</text></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback (ok=false)", name)
		}
	}
}

func TestReviewBatch2(t *testing.T) {
	// Nested <svg> is rejected (own viewport unmodeled).
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><svg width="5" height="5"><rect width="1" height="1"/></svg></svg>`)); ok {
		t.Error("nested svg should fall back")
	}
	// viewBox + mismatched width/height aspect ratio -> fallback (letterboxing).
	if _, ok := Convert([]byte(`<svg width="200" height="100" viewBox="0 0 100 100"><rect width="1" height="1"/></svg>`)); ok {
		t.Error("viewBox+aspect mismatch should fall back")
	}
	// Non-default preserveAspectRatio -> fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10" preserveAspectRatio="none"><rect width="1" height="1"/></svg>`)); ok {
		t.Error("preserveAspectRatio none should fall back")
	}
	// visibility:hidden container with a visible descendant keeps the child.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><g visibility="hidden"><rect width="1" height="1" fill="red" visibility="visible"/></g></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "ff0000" {
		t.Fatalf("visibility override lost: %#v", g.Geoms)
	}
	// visibility:hidden leaf with no override is skipped.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><rect width="1" height="1" visibility="hidden"/><rect width="1" height="1" fill="green"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("visibility hidden leaf not skipped: %#v", g.Geoms)
	}
	// Fractional linear-gradient vector -> fallback; full-box vector converts.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><defs><linearGradient id="g" x1="0.5" x2="1"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)); ok {
		t.Error("partial linear gradient vector should fall back")
	}
	// Single-stop gradient is expanded to two stops (valid OOXML).
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="g"><stop offset="0" stop-color="red"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
	if gr := g.Geoms[0].Fill.Gradient; gr == nil || len(gr.Stops) != 2 {
		t.Fatalf("single stop should expand to two: %#v", g.Geoms[0].Fill.Gradient)
	}
	// Non-uniform scale on a stroked shape -> fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><rect width="2" height="2" fill="none" stroke="black" stroke-width="1" transform="scale(4,1)"/></svg>`)); ok {
		t.Error("non-uniform scaled stroke should fall back")
	}
	// CSS class selector is case-sensitive: .Hot must not match class="hot".
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><style>.Hot{fill:#00f}</style><rect class="hot" width="1" height="1" fill="green"/></svg>`)
	if g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf(".Hot should not match class=hot, got %s", g.Geoms[0].Fill.Color)
	}
	// Converted SVG text carries zero insets and preserves document order.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><rect width="1" height="1" fill="red"/><text x="1" y="20" fill="blue" font-size="10">Hi</text></svg>`)
	if len(g.Children) != 2 || g.Children[0].Geom == nil || g.Children[1].Text == nil {
		t.Fatalf("expected ordered geom then text children: %#v", g.Children)
	}
	if !g.Children[1].Text.NoInset {
		t.Error("svg text should use zero insets")
	}
}

func TestReviewBatch3(t *testing.T) {
	// Standalone <symbol> renders nothing; via <use> it renders.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><symbol id="s"><rect width="1" height="1" fill="red"/></symbol></svg>`)
	if len(g.Geoms) != 0 {
		t.Fatalf("standalone symbol should render nothing, got %d", len(g.Geoms))
	}
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><symbol id="s"><rect width="1" height="1" fill="red"/></symbol></defs><use href="#s"/></svg>`)
	if len(g.Geoms) != 1 {
		t.Fatalf("symbol via use should render, got %d", len(g.Geoms))
	}
	// display:none on the root renders nothing.
	g = mustConvert(t, `<svg viewBox="0 0 10 10" display="none"><rect width="1" height="1" fill="red"/></svg>`)
	if len(g.Geoms) != 0 {
		t.Fatalf("root display:none should render nothing, got %d", len(g.Geoms))
	}
	// A gradient-filled rect not covering the canvas gets tight bounds.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><defs><linearGradient id="lg"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect x="40" y="40" width="20" height="20" fill="url(#lg)"/></svg>`)
	gs := g.Geoms[0]
	if gs.X <= 0 || gs.W >= gs.PathW+1 && gs.W == round(100*emuPerUnit) {
		t.Fatalf("gradient shape should use tight bounds, got X=%d W=%d", gs.X, gs.W)
	}
	if gs.W != round(20*emuPerUnit) {
		t.Fatalf("gradient shape width should equal element width, got %d want %d", gs.W, round(20*emuPerUnit))
	}
	// Monotonic gradient offsets: a descending offset is clamped.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg"><stop offset="1" stop-color="red"/><stop offset="0" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`)
	st := g.Geoms[0].Fill.Gradient.Stops
	if st[1].Pos < st[0].Pos {
		t.Fatalf("stop offsets must be non-decreasing, got %v then %v", st[0].Pos, st[1].Pos)
	}
	// Text fill:none with a colored tspan still renders the tspan.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="1" y="20" fill="none" font-size="10"><tspan fill="red">Hi</tspan></text></svg>`)
	if len(g.Texts) != 1 || len(g.Texts[0].Paragraphs[0].Runs) != 1 || g.Texts[0].Paragraphs[0].Runs[0].Color != "ff0000" {
		t.Fatalf("fill:none text with colored tspan should render tspan: %#v", g.Texts)
	}
	// Whitespace between a text node and a tspan is preserved as a separator.
	g = mustConvert(t, `<svg viewBox="0 0 200 100"><text x="1" y="20" fill="blue" font-size="10">Hello <tspan>world</tspan></text></svg>`)
	runs := g.Texts[0].Paragraphs[0].Runs
	var joined string
	for _, r := range runs {
		joined += r.Text
	}
	if joined != "Hello world" {
		t.Fatalf("whitespace separator lost: %q", joined)
	}

	for name, svg := range map[string]string{
		"gradient under rotation":   `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)" transform="rotate(90)"/></svg>`,
		"partial gradient override": `<svg viewBox="0 0 10 10"><defs><linearGradient id="p" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient><linearGradient id="c" href="#p" x2="1"/></defs><rect width="10" height="10" fill="url(#c)"/></svg>`,
		"important":                 `<svg viewBox="0 0 10 10"><rect width="1" height="1" style="display:none !important"/></svg>`,
		"tspan positioned":          `<svg viewBox="0 0 100 100"><text x="1" y="20" fill="red" font-size="10"><tspan x="5">Hi</tspan></text></svg>`,
		"tspan nested":              `<svg viewBox="0 0 100 100"><text x="1" y="20" fill="red" font-size="10"><tspan><tspan>Hi</tspan></tspan></text></svg>`,
		"explicit miterlimit":       `<svg viewBox="0 0 10 10"><rect width="2" height="2" fill="none" stroke="black" stroke-width="1" stroke-linejoin="miter" stroke-miterlimit="10"/></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback", name)
		}
	}
}

func TestReviewBatch4(t *testing.T) {
	// Foreign-namespace element is not rendered.
	g := mustConvert(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:foo="urn:example" viewBox="0 0 10 10"><foo:rect x="0" y="0" width="1" height="1" fill="red"/><rect width="1" height="1" fill="green"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("foreign element should not render: %#v", g.Geoms)
	}
	// Whitespace-only text between tspans is order-significant -> fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10"><tspan>Hello</tspan> <tspan>world</tspan></text></svg>`)); ok {
		t.Error("whitespace between tspans should fall back")
	}
	// A hidden tspan after the visible content is skipped; the sibling renders.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10"><tspan>ok</tspan><tspan display="none">x</tspan></text></svg>`)
	if len(g.Texts) != 1 || len(g.Texts[0].Paragraphs[0].Runs) != 1 || g.Texts[0].Paragraphs[0].Runs[0].Text != "ok" {
		t.Fatalf("trailing display:none tspan should be skipped: %#v", g.Texts)
	}
	// A display:none tspan with text *before* visible text does not advance the
	// position (display:none is removed from layout), so the visible run stays
	// anchored and conversion succeeds.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10"><tspan display="none">x</tspan><tspan>ok</tspan></text></svg>`)
	if len(g.Texts[0].Paragraphs[0].Runs) != 1 || g.Texts[0].Paragraphs[0].Runs[0].Text != "ok" {
		t.Fatalf("display:none run before visible text should be dropped without advancing: %#v", g.Texts)
	}
	// A visibility:hidden tspan before visible text *does* advance, so it falls
	// back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10"><tspan visibility="hidden">x</tspan><tspan>ok</tspan></text></svg>`)); ok {
		t.Error("visibility:hidden text before visible text should fall back")
	}
	// Hidden text with a visibility:visible tspan still renders the child.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10" visibility="hidden"><tspan visibility="visible">seen</tspan></text></svg>`)
	if len(g.Texts) != 1 || len(g.Texts[0].Paragraphs[0].Runs) != 1 || g.Texts[0].Paragraphs[0].Runs[0].Text != "seen" {
		t.Fatalf("visible tspan under hidden text lost: %#v", g.Texts)
	}
	// Bezier bounds: a gradient-filled circle uses tight bounds ~= its bbox.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><defs><linearGradient id="lg"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><circle cx="50" cy="50" r="10" fill="url(#lg)"/></svg>`)
	gs := g.Geoms[0]
	if d := gs.W - round(20*emuPerUnit); d < -round(1*emuPerUnit) || d > round(1*emuPerUnit) {
		t.Fatalf("gradient circle bounds should be ~20 units wide, got W=%d", gs.W)
	}

	for name, svg := range map[string]string{
		"unsupported presentation attr": `<svg viewBox="0 0 10 10"><rect width="1" height="1" fill="red" vector-effect="non-scaling-stroke"/></svg>`,
		"font-weight attr":              `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10" font-weight="bold">Hi</text></svg>`,
		"zero-length gradient":          `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" x1="0.5" y1="0.5" x2="0.5" y2="0.5"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`,
		"opacity fill and stroke":       `<svg viewBox="0 0 10 10"><rect width="5" height="5" fill="red" stroke="blue" stroke-width="1" opacity="0.5"/></svg>`,
		"circle outside viewport":       `<svg viewBox="0 0 10 10"><circle cx="0" cy="5" r="3" fill="red"/></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback", name)
		}
	}
}

func TestReviewBatch5(t *testing.T) {
	// Gradient stop-color from a matching CSS rule overrides the presentation attr.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><style>.s{stop-color:#00ff00}</style><defs><linearGradient id="lg"><stop class="s" offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`)
	if g.Geoms[0].Fill.Gradient.Stops[0].Color != "00ff00" {
		t.Fatalf("CSS stop-color should override attr, got %s", g.Geoms[0].Fill.Gradient.Stops[0].Color)
	}

	for name, svg := range map[string]string{
		"xml:space preserve text":  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10" xml:space="preserve">a   b</text></svg>`,
		"text with stroke":         `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="none" stroke="red" stroke-width="1" font-size="10">Hi</text></svg>`,
		"tspan with stroke":        `<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10"><tspan stroke="blue" stroke-width="1">Hi</tspan></text></svg>`,
		"gradient axis reflection": `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect x="-5" width="5" height="5" fill="url(#lg)" transform="translate(10,0) scale(-1,1)"/></svg>`,
		"text outside viewport":    `<svg viewBox="0 0 100 100"><text x="1" y="200" fill="red" font-size="10">Hi</text></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback", name)
		}
	}
}

func TestReviewBatch6(t *testing.T) {
	// A gradient stop's currentColor resolves to the gradient's color context.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" color="#ff0000"><stop offset="0" stop-color="currentColor"/><stop offset="1" stop-color="#0000ff"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`)
	if g.Geoms[0].Fill.Gradient.Stops[0].Color != "ff0000" {
		t.Fatalf("stop currentColor should resolve to gradient color, got %s", g.Geoms[0].Fill.Gradient.Stops[0].Color)
	}
	// stroke-width:0 paints no stroke but the shape still converts.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><rect width="5" height="5" fill="red" stroke="black" stroke-width="0"/></svg>`)
	if g.Geoms[0].Stroke != nil {
		t.Fatalf("stroke-width:0 should yield no stroke")
	}

	for name, svg := range map[string]string{
		"path starts with L":    `<svg viewBox="0 0 10 10"><path d="L5 5" fill="red"/></svg>`,
		"zero scale stroked":    `<svg viewBox="0 0 10 10"><rect width="5" height="5" fill="none" stroke="black" stroke-width="1" transform="scale(0)"/></svg>`,
		"NaN opacity":           `<svg viewBox="0 0 10 10"><rect width="5" height="5" fill="red" opacity="NaN"/></svg>`,
		"NaN gradient coord":    `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" x1="NaN"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`,
		"negative stroke width": `<svg viewBox="0 0 10 10"><rect width="5" height="5" fill="none" stroke="black" stroke-width="-1"/></svg>`,
		"style media print":     `<svg viewBox="0 0 10 10"><style media="print">rect{fill:red}</style><rect width="1" height="1"/></svg>`,
		"style non-css type":    `<svg viewBox="0 0 10 10"><style type="text/less">rect{fill:red}</style><rect width="1" height="1"/></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback", name)
		}
	}
}

func TestReviewBatch7(t *testing.T) {
	// color:currentColor inherits the parent color; a descendant fill:currentColor
	// then resolves to that color rather than black.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><g color="#ff0000"><g color="currentColor"><rect width="1" height="1" fill="currentColor"/></g></g></svg>`)
	if g.Geoms[0].Fill.Color != "ff0000" {
		t.Fatalf("nested currentColor should resolve to inherited color, got %s", g.Geoms[0].Fill.Color)
	}
	// xml:space=preserve on an ancestor forces fallback for descendant text.
	if _, ok := Convert([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" xml:space="preserve"><g><text x="1" y="10" fill="red" font-size="10">a   b</text></g></svg>`)); ok {
		t.Error("inherited xml:space=preserve should fall back")
	}
	// An unsupported presentation attribute on a tspan forces fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="1" y="10" fill="red" font-size="10"><tspan font-weight="bold">Hi</tspan></text></svg>`)); ok {
		t.Error("tspan font-weight should fall back")
	}
}

func TestReviewBatch8(t *testing.T) {
	// visibility:inherit under a hidden parent stays hidden.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><g visibility="hidden"><rect width="1" height="1" fill="red" visibility="inherit"/></g><rect width="1" height="1" fill="green"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("visibility:inherit under hidden parent should stay hidden: %#v", g.Geoms)
	}
	// SVG text is emitted with no-wrap.
	g = mustConvert(t, `<svg viewBox="0 0 100 100"><text x="1" y="20" fill="red" font-size="10">Hi</text></svg>`)
	if !g.Texts[0].NoWrap {
		t.Fatal("SVG text should be no-wrap")
	}

	for name, svg := range map[string]string{
		"gradient color-interpolation": `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" color-interpolation="linearRGB"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`,
		"gradient coord out of range":  `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" x1="-100%" x2="200%"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`,
		"stop unsupported attr":        `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg"><stop offset="0" stop-color="red" color-interpolation="linearRGB"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`,
		"negative font-size":           `<svg viewBox="0 0 100 100"><text x="1" y="20" fill="red" font-size="-10">Hi</text></svg>`,
		"negative tspan font-size":     `<svg viewBox="0 0 100 100"><text x="1" y="20" fill="red" font-size="10"><tspan font-size="-5">Hi</tspan></text></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback", name)
		}
	}
}

func TestEmptyPolygonSkipped(t *testing.T) {
	// A polygon/polyline with no (or too few) points emits no geometry rather
	// than a malformed MoveTo-less path.
	for _, svg := range []string{
		`<svg viewBox="0 0 10 10"><polygon points="" fill="red"/></svg>`,
		`<svg viewBox="0 0 10 10"><polyline points="1,1" fill="none" stroke="red"/></svg>`,
	} {
		g, ok := Convert([]byte(svg))
		if !ok {
			t.Fatalf("expected conversion for %s", svg)
		}
		if len(g.Geoms) != 0 {
			t.Fatalf("expected no geometry for %s, got %d", svg, len(g.Geoms))
		}
	}
}

func TestReviewBatch9(t *testing.T) {
	for name, svg := range map[string]string{
		"operandless command":    `<svg viewBox="0 0 100 100"><path d="M0 0 L Z L20 0" fill="red"/></svg>`,
		"gradient external href": `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" href="ext.svg#g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`,
	} {
		if _, ok := Convert([]byte(svg)); ok {
			t.Errorf("%s: expected fallback", name)
		}
	}
	// An arc with identical endpoints emits no cubic (no visible dot).
	if cubs := arcToCubics(5, 5, 3, 3, 0, false, false, 5, 5); cubs != nil {
		t.Fatalf("identical-endpoint arc should emit no cubic, got %v", cubs)
	}
	// A crafted diamond <use> graph (exponential expansion, no shapes) is bounded.
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 10 10"><defs>`)
	b.WriteString(`<g id="g0"><rect width="1" height="1"/></g>`)
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, `<g id="g%d"><use href="#g%d"/><use href="#g%d"/></g>`, i, i-1, i-1)
	}
	b.WriteString(`</defs><use href="#g40"/></svg>`)
	if _, ok := Convert([]byte(b.String())); ok {
		t.Error("exponential <use> graph should hit the visit limit and fall back")
	}
}

func TestReviewBatch10(t *testing.T) {
	// Compound class selector .foo.bar is rejected (mis-matched as one class).
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><style>.foo.bar{fill:red}</style><rect class="foo" width="1" height="1"/></svg>`)); ok {
		t.Error("compound selector should fall back")
	}
	// xml-stylesheet PI forces fallback.
	if _, ok := Convert([]byte(`<?xml version="1.0"?><?xml-stylesheet href="s.css" type="text/css"?><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="1" height="1"/></svg>`)); ok {
		t.Error("xml-stylesheet PI should fall back")
	}
	// href takes precedence over xlink:href; an external href doesn't fall
	// through to a local xlink:href.
	if _, ok := Convert([]byte(`<svg xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10"><defs><linearGradient id="base" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/><linearGradient id="g" href="ext.svg#x" xlink:href="#base"/></svg>`)); ok {
		t.Error("external href should not fall through to xlink:href")
	}
	// A large stroke that would paint well outside the viewport falls back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><line x1="1" y1="5" x2="9" y2="5" stroke="black" stroke-width="1000"/></svg>`)); ok {
		t.Error("huge stroke crossing the viewport should fall back")
	}
	// An edge-aligned stroked border paints half its width outside the viewport,
	// which a PowerPoint group would show beyond the image rect; fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><rect x="0" y="0" width="10" height="10" fill="none" stroke="black" stroke-width="1" stroke-linejoin="round"/></svg>`)); ok {
		t.Error("edge-aligned stroked border should fall back (stroke overhang)")
	}
	// An inset border whose stroke stays within the viewport still converts.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><rect x="1" y="1" width="8" height="8" fill="none" stroke="black" stroke-width="1" stroke-linejoin="round"/></svg>`)); !ok {
		t.Error("inset border within the viewport should convert")
	}
	// Invisible text before visible text shifts position -> fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="1" y="10" fill="none" font-size="10">XX<tspan fill="red">Hi</tspan></text></svg>`)); ok {
		t.Error("invisible text before visible run should fall back")
	}
	// Whitespace across a run boundary collapses to a single separator and is
	// preserved on the run.
	g := mustConvert(t, `<svg viewBox="0 0 200 100"><text x="1" y="10" fill="blue" font-size="10">Hello <tspan> world</tspan></text></svg>`)
	runs := g.Texts[0].Paragraphs[0].Runs
	var joined string
	for _, r := range runs {
		joined += r.Text
	}
	if joined != "Hello world" {
		t.Fatalf("cross-run whitespace should collapse to one space, got %q", joined)
	}
}

func TestReviewBatch11(t *testing.T) {
	// Foreign-namespace root named "svg" is rejected (not an empty success).
	if _, ok := Convert([]byte(`<svg xmlns="urn:example"><rect width="1" height="1"/></svg>`)); ok {
		t.Error("foreign-namespace root should fall back")
	}
	// stop-color set on the gradient must not leak into a stop lacking one.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="lg" style="stop-color:#00ff00"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></linearGradient></defs><rect width="10" height="10" fill="url(#lg)"/></svg>`)
	if g.Geoms[0].Fill.Gradient.Stops[0].Color != "ff0000" {
		t.Fatalf("gradient stop-color leaked, got %s", g.Geoms[0].Fill.Gradient.Stops[0].Color)
	}
	// visibility:unset under a hidden parent stays hidden.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><g visibility="hidden"><rect width="1" height="1" fill="red" visibility="unset"/></g><rect width="1" height="1" fill="green"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("visibility:unset should inherit hidden: %#v", g.Geoms)
	}
	// A type selector is case-sensitive: RECT must not match <rect>.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><style>RECT{fill:#0000ff}</style><rect width="1" height="1" fill="green"/></svg>`)
	if g.Geoms[0].Fill.Color != "008000" {
		t.Fatalf("RECT should not match <rect>, got %s", g.Geoms[0].Fill.Color)
	}
	// camelCase type selector still matches the camelCase element.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><style>rect{fill:#00ff00}</style><rect width="1" height="1"/></svg>`)
	if g.Geoms[0].Fill.Color != "00ff00" {
		t.Fatalf("lowercase type selector should match, got %s", g.Geoms[0].Fill.Color)
	}
}

func TestReviewBatch12(t *testing.T) {
	// fill:initial restores the default (black), not "no fill".
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><rect width="1" height="1" fill="initial"/></svg>`)
	if g.Geoms[0].Fill.Kind != pptx.FillSolid || g.Geoms[0].Fill.Color != "000000" {
		t.Fatalf("fill:initial should be solid black, got %#v", g.Geoms[0].Fill)
	}
	// A symbol with display:none instantiated by <use> renders nothing.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><symbol id="s" display="none"><rect width="1" height="1" fill="red"/></symbol></defs><use href="#s"/></svg>`)
	if len(g.Geoms) != 0 {
		t.Fatalf("display:none symbol should render nothing, got %d", len(g.Geoms))
	}
	// A symbol with a clip-path (unsupported) instantiated by <use> falls back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><defs><symbol id="s" clip-path="url(#c)"><rect width="1" height="1"/></symbol></defs><use href="#s"/></svg>`)); ok {
		t.Error("symbol with clip-path should fall back")
	}
	// A whitespace-only tspan between visible runs is a word separator, not
	// dropped.
	g = mustConvert(t, `<svg viewBox="0 0 200 40"><text x="1" y="20" fill="blue" font-size="10"><tspan>Hello</tspan><tspan> </tspan><tspan>world</tspan></text></svg>`)
	var joined string
	for _, r := range g.Texts[0].Paragraphs[0].Runs {
		joined += r.Text
	}
	if joined != "Hello world" {
		t.Fatalf("whitespace-only tspan separator lost: %q", joined)
	}
	// A large tspan inside small parent text that would overflow the viewport
	// falls back (vertical bounds use the max run size).
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 20"><text x="1" y="18" fill="red" font-size="5"><tspan font-size="200">Big</tspan></text></svg>`)); ok {
		t.Error("oversized tspan overflowing the viewport should fall back")
	}
}

func TestReviewBatch19(t *testing.T) {
	// A gradient stop's currentColor inherits the color computed at the
	// gradient's own ancestors, not defaultStyle: here ancestor color="red".
	g := mustConvert(t, `<svg viewBox="0 0 10 10" color="red"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="currentColor"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
	if g.Geoms[0].Fill.Gradient.Stops[0].Color != "ff0000" {
		t.Fatalf("gradient currentColor should inherit ancestor red, got %#v", g.Geoms[0].Fill.Gradient.Stops[0])
	}
	// A stroke wider than the DrawingML a:ln@w limit (even inside a large
	// viewBox) can't be represented; fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 1000000 1000000"><line x1="1" y1="5" x2="9" y2="5" stroke="black" stroke-width="500000"/></svg>`)); ok {
		t.Error("stroke width beyond the DrawingML limit should fall back")
	}
	// A viewBox whose EMU extent exceeds ST_PositiveCoordinate can't be written;
	// fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 9000000000 9000000000"><rect width="1" height="1" fill="red"/></svg>`)); ok {
		t.Error("oversized viewBox should fall back")
	}
	// xml:base rebases href/url references; the converter can't honor it.
	if _, ok := Convert([]byte(`<svg xml:base="http://example.com/" viewBox="0 0 10 10"><rect width="1" height="1" fill="red"/></svg>`)); ok {
		t.Error("xml:base should force the fallback")
	}
	// A relative root height (1em) gives a real aspect (32x16) that differs from
	// a square viewBox, which needs letterboxing this model can't do; fall back.
	if _, ok := Convert([]byte(`<svg width="32px" height="1em" viewBox="0 0 32 32"><rect width="1" height="1" fill="red"/></svg>`)); ok {
		t.Error("mismatched em-based aspect should fall back")
	}
}

func TestReviewBatch18(t *testing.T) {
	// A sharp miter corner whose apex points past the viewport edge leaks
	// outside the non-clipping group; fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><polyline points="15,2 1,50 15,98" fill="none" stroke="black" stroke-width="2"/></svg>`)); ok {
		t.Error("sharp miter spike near the edge should fall back")
	}
	// The same corner comfortably inside the viewport still converts (the actual
	// miter apex, not a worst-case, is used).
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><polyline points="30,2 16,50 30,98" fill="none" stroke="black" stroke-width="2"/></svg>`)); !ok {
		t.Error("inset miter corner should convert")
	}
	// Under an SVG-namespaced root, an element that resets to no namespace
	// (xmlns="") is not SVG content and renders nothing.
	g := mustConvert(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect xmlns="" width="10" height="10" fill="red"/></svg>`)
	if len(g.Geoms) != 0 {
		t.Fatalf("xmlns=\"\" rect should not render, got %d geoms", len(g.Geoms))
	}
	// A prefixed svg:fill is not the unqualified presentation attribute, so it's
	// ignored and the default (black) fill applies.
	g = mustConvert(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="1" height="1" svg:fill="red"/></svg>`)
	if g.Geoms[0].Fill.Color != "000000" {
		t.Fatalf("prefixed svg:fill must be ignored (default black), got %#v", g.Geoms[0].Fill)
	}
	// stop-color:initial restores the initial value (black), not the red
	// presentation attribute; the gradient's first stop is black.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="red" style="stop-color:initial"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
	if g.Geoms[0].Fill.Kind != pptx.FillGradient || g.Geoms[0].Fill.Gradient.Stops[0].Color != "000000" {
		t.Fatalf("stop-color:initial should be black, got %#v", g.Geoms[0].Fill.Gradient)
	}
	// An invalid rgb() component (NaN) is not a valid color; the declaration is
	// rejected and the element falls back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><rect width="1" height="1" fill="rgb(NaN%,0%,0%)"/></svg>`)); ok {
		t.Error("rgb() with a NaN component should fall back")
	}
}

func TestReviewBatch17(t *testing.T) {
	// A DOCTYPE may carry ATTLIST defaults or an external DTD the parser doesn't
	// apply; converting could differ from a faithful render, so fall back.
	if _, ok := Convert([]byte(`<!DOCTYPE svg [ <!ATTLIST rect fill CDATA "red"> ]><svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`)); ok {
		t.Error("SVG with a DOCTYPE should fall back")
	}
	// An invalid selector (a class starting with a digit) is ignored by CSS, so
	// the stylesheet parse must fail rather than apply it to class="1".
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><style>.1{display:none}</style><rect class="1" width="10" height="10" fill="red"/></svg>`)); ok {
		t.Error("digit-leading class selector should fall back")
	}
	// A leading-hyphen-digit identifier is likewise invalid.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><style>.-1{display:none}</style><rect class="-1" width="10" height="10" fill="red"/></svg>`)); ok {
		t.Error("hyphen-digit class selector should fall back")
	}
	// A valid identifier selector still applies.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><style>.a{fill:red}</style><rect class="a" width="1" height="1"/></svg>`)
	if g.Geoms[0].Fill.Color != "ff0000" {
		t.Fatalf("valid class selector should apply, got %#v", g.Geoms[0].Fill)
	}
	// An edge-aligned stroke paints half its width outside the viewport, which a
	// PowerPoint group can't clip; fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><line x1="0" y1="5" x2="10" y2="5" stroke="black" stroke-width="1"/></svg>`)); ok {
		t.Error("edge-touching stroke should fall back (half-width overhang)")
	}
}

func TestReviewBatch16(t *testing.T) {
	// A known SVG element in the wrong case is not that element and must not be
	// converted into geometry/gradients.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><RECT width="10" height="10" fill="red"/></svg>`)); ok {
		t.Error("wrong-case <RECT> should fall back")
	}
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><defs><lineargradient id="g"><stop offset="0" stop-color="red"/></lineargradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)); ok {
		t.Error("wrong-case <lineargradient> should fall back")
	}
	// A known SVG attribute in the wrong case is unknown and must not activate.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><rect width="10" height="10" FILL="red"/></svg>`)); ok {
		t.Error("wrong-case FILL attribute should fall back")
	}
	// The correct case still converts.
	g := mustConvert(t, `<svg viewBox="0 0 10 10"><rect width="10" height="10" fill="red"/></svg>`)
	if g.Geoms[0].Fill.Color != "ff0000" {
		t.Fatalf("correct-case rect/fill should convert, got %#v", g.Geoms[0].Fill)
	}
	// A camelCase attribute (spreadMethod) still works in its canonical case,
	// but its wrong-case form is rejected.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="g" spreadMethod="pad" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
	if g.Geoms[0].Fill.Kind != pptx.FillGradient {
		t.Fatalf("canonical spreadMethod should convert to a gradient, got %#v", g.Geoms[0].Fill)
	}
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><defs><linearGradient id="g" SPREADMETHOD="pad" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)); ok {
		t.Error("wrong-case SPREADMETHOD should fall back")
	}
	// A quoted font-family containing a comma is one family, not split on the
	// comma.
	g = mustConvert(t, `<svg viewBox="0 0 200 40"><text x="1" y="20" fill="blue" font-size="10" font-family='"ACME, Inc.", sans-serif'>Hi</text></svg>`)
	if fam := g.Texts[0].Paragraphs[0].Runs[0].FontFamily; fam != "ACME, Inc." {
		t.Fatalf("quoted family with comma mis-parsed, got %q", fam)
	}
}

func TestReviewBatch15(t *testing.T) {
	// A font-size below the DrawingML 1pt minimum can't be represented.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="1" y="10" font-size="0.5">Hi</text></svg>`)); ok {
		t.Error("sub-1pt font size should fall back")
	}
	// A font-size above the 4000pt maximum can't be represented.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100000 100000"><text x="1" y="10" font-size="6000">Hi</text></svg>`)); ok {
		t.Error("oversized font size should fall back")
	}
	// A tspan font-size override outside the range also falls back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><text x="1" y="10" font-size="10"><tspan font-size="0.5">Hi</tspan></text></svg>`)); ok {
		t.Error("tspan sub-1pt font size should fall back")
	}
	// An invalid enumerated value (fill-rule/visibility) is a dropped CSS
	// declaration; treating it as a default could flip rendering, so fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><path d="M0 0h10v10h-10z" fill-rule="bogus"/></svg>`)); ok {
		t.Error("invalid fill-rule should fall back")
	}
	if _, ok := Convert([]byte(`<svg viewBox="0 0 10 10"><rect width="10" height="10" visibility="sometimes"/></svg>`)); ok {
		t.Error("invalid visibility should fall back")
	}
	// display:none removes a run from layout without advancing, so the
	// surrounding visible runs stay adjacent and conversion succeeds.
	g := mustConvert(t, `<svg viewBox="0 0 100 40"><text x="1" y="20" fill="blue" font-size="10"><tspan>A</tspan><tspan display="none">X</tspan><tspan>B</tspan></text></svg>`)
	var joined string
	for _, r := range g.Texts[0].Paragraphs[0].Runs {
		joined += r.Text
	}
	if joined != "AB" {
		t.Fatalf("display:none run should be dropped without advancing, got %q", joined)
	}
	// visibility:hidden whitespace still advances the position, so a later
	// visible run can't be faithfully placed: fall back.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 40"><text x="1" y="20" fill="blue" font-size="10"><tspan>A</tspan><tspan visibility="hidden"> </tspan><tspan>B</tspan></text></svg>`)); ok {
		t.Error("visibility:hidden whitespace separator should fall back")
	}
}

func TestReviewBatch14(t *testing.T) {
	// An explicit rx="0" disables rounding on that axis even when ry is set, so
	// the rect keeps square corners (no cubic segments).
	g := mustConvert(t, `<svg viewBox="0 0 20 20"><rect width="10" height="10" rx="0" ry="4"/></svg>`)
	for _, p := range g.Geoms[0].Paths {
		for _, cmd := range p.Cmds {
			if cmd.Verb == pptx.CubicTo {
				t.Fatalf("explicit rx=0 should keep square corners, got a cubic segment")
			}
		}
	}
	// An omitted rx still inherits ry (auto), producing rounded corners.
	g = mustConvert(t, `<svg viewBox="0 0 20 20"><rect width="10" height="10" ry="4"/></svg>`)
	var rounded bool
	for _, p := range g.Geoms[0].Paths {
		for _, cmd := range p.Cmds {
			if cmd.Verb == pptx.CubicTo {
				rounded = true
			}
		}
	}
	if !rounded {
		t.Fatalf("omitted rx should inherit ry and round the corners")
	}
	// A negative corner radius is invalid and forces the fallback.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 20 20"><rect width="10" height="10" rx="-4"/></svg>`)); ok {
		t.Error("negative rx should fall back")
	}
	// A comment-like sequence inside a quoted CSS string must not be stripped,
	// so the selector still matches and converts.
	g = mustConvert(t, `<svg viewBox="0 0 10 10"><style>rect{fill:red;font-family:"A/*B*/"}</style><rect width="1" height="1"/></svg>`)
	if g.Geoms[0].Fill.Color != "ff0000" {
		t.Fatalf("CSS comment inside a string must not break parsing, got %#v", g.Geoms[0].Fill)
	}
}

func TestReviewBatch13(t *testing.T) {
	// A whitespace-only invisible run advances the SVG text position, so a
	// later visible run must trigger the fallback rather than emit adjacent
	// text ("A B" collapsed to "AB").
	if _, ok := Convert([]byte(`<svg viewBox="0 0 100 40"><text x="1" y="20" fill="blue" font-size="10"><tspan>A</tspan><tspan fill="none"> </tspan><tspan>B</tspan></text></svg>`)); ok {
		t.Error("whitespace-only invisible run before a visible run should fall back")
	}
	// Text that fits well within the viewport still converts under the
	// full-em width upper bound.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 200 40"><text x="1" y="20" fill="blue" font-size="10">Hi</text></svg>`)); !ok {
		t.Error("short text within the viewport should still convert")
	}
	// Wide text near the right edge overflows under the full-em bound and falls
	// back instead of clipping.
	if _, ok := Convert([]byte(`<svg viewBox="0 0 50 20"><text x="1" y="15" fill="blue" font-size="10">WWWWW</text></svg>`)); ok {
		t.Error("wide text near the right edge should fall back")
	}
}
