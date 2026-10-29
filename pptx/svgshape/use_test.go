package svgshape

import (
	"testing"

	"github.com/Songmu/slidown/pptx"
)

func TestConvertUseReferenceFallbacks(t *testing.T) {
	for name, content := range map[string]string{
		"missing href":             `<use/>`,
		"unresolved reference":     `<use href="#missing"/>`,
		"external overrides xlink": `<defs><rect id="r" width="1" height="1"/></defs><use href="other.svg#r" xlink:href="#r"/>`,
		"local overrides xlink":    `<defs><rect id="r" width="1" height="1"/></defs><use href="#missing" xlink:href="#r"/>`,
		"self cycle":               `<defs><use id="u" href="#u"/></defs><use href="#u"/>`,
		"symbol group cycle":       `<defs><symbol id="s"><use href="#g"/></symbol><g id="g"><use href="#s"/></g></defs><use href="#s"/>`,
		"invalid x":                `<defs><rect id="r" width="1" height="1"/></defs><use href="#r" x="bad"/>`,
		"invalid y":                `<defs><rect id="r" width="1" height="1"/></defs><use href="#r" y="10%"/>`,
		"use compositing opacity":  `<defs><rect id="r" width="1" height="1"/></defs><use href="#r" opacity=".5"/>`,
		"use invalid opacity":      `<defs><rect id="r" width="1" height="1"/></defs><use href="#r" opacity="NaN"/>`,
	} {
		t.Run(name, func(t *testing.T) {
			g, ok := Convert([]byte(`<svg xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 100 100">` + content + `</svg>`))
			if ok || g != nil {
				t.Fatalf("Convert() = (%v, %v), want whole-document fallback", g, ok)
			}
		})
	}
}

func TestConvertUseSymbolFallbacks(t *testing.T) {
	for name, symbol := range map[string]string{
		"invalid transform":          `<symbol id="s" transform="scale(bad)"/>`,
		"unsupported style":          `<symbol id="s" style="mix-blend-mode:multiply"/>`,
		"hidden unsupported style":   `<symbol id="s" display="none" style="mix-blend-mode:multiply"/>`,
		"hidden unsupported attr":    `<symbol id="s" display="none" vector-effect="non-scaling-stroke"/>`,
		"symbol compositing opacity": `<symbol id="s" opacity=".5"/>`,
		"symbol invalid opacity":     `<symbol id="s" opacity="NaN"/>`,
		"child failure after output": `<symbol id="s"><rect width="1" height="1"/><rect width="bad" height="1"/></symbol>`,
	} {
		t.Run(name, func(t *testing.T) {
			g, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><defs>` + symbol + `</defs><use href="#s"/></svg>`))
			if ok || g != nil {
				t.Fatalf("Convert() = (%v, %v), want whole-document fallback", g, ok)
			}
		})
	}
}

func TestConvertUseRepeatedSymbolTransforms(t *testing.T) {
	g := mustConvert(t, `<svg xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 100 100">
		<defs><symbol id="s" transform="scale(2)"><rect x="1" y="1" width="2" height="1" transform="translate(1,2)"/></symbol></defs>
		<use href="#s" xlink:href="#missing" x="5" y="7" transform="scale(3)" fill="red"/>
		<use xlink:href="#s" x="40" y="50" fill="blue"/>
	</svg>`)
	if len(g.Geoms) != 2 {
		t.Fatalf("got %d shapes, want two independent instances", len(g.Geoms))
	}
	for i, want := range []struct {
		x, y, w, h int64
		color      string
	}{
		{27, 39, 12, 6, "ff0000"},
		{44, 56, 4, 2, "0000ff"},
	} {
		shape := g.Geoms[i]
		minX, minY, maxX, maxY := bounds(shape.Paths[0])
		near(t, minX, want.x*9525)
		near(t, minY, want.y*9525)
		near(t, maxX-minX, want.w*9525)
		near(t, maxY-minY, want.h*9525)
		if shape.Fill.Color != want.color {
			t.Errorf("instance %d fill = %q, want %q", i, shape.Fill.Color, want.color)
		}
	}
}

func TestConvertUseHiddenSymbolSkipsRenderingChecks(t *testing.T) {
	// Attribute/style validation precedes display, but rendering checks do not.
	g := mustConvert(t, `<svg viewBox="0 0 100 100"><defs>
		<symbol id="s" display="none" transform="scale(bad)" opacity="NaN"><rect width="bad" height="1"/></symbol>
		</defs><use href="#s"/><rect x="3" y="4" width="1" height="2" fill="red"/></svg>`)
	if len(g.Geoms) != 1 || g.Geoms[0].Fill.Color != "ff0000" {
		t.Fatalf("hidden symbol must leave only the visible rectangle: %#v", g.Geoms)
	}
	minX, minY, maxX, maxY := bounds(g.Geoms[0].Paths[0])
	near(t, minX, 3*9525)
	near(t, minY, 4*9525)
	near(t, maxX-minX, 9525)
	near(t, maxY-minY, 2*9525)
}

func TestExpandUseResolutionCleanup(t *testing.T) {
	for _, tc := range []struct {
		name, definition string
		ok               bool
		shapes           int
	}{
		{"ordinary success", `<rect id="s" width="1" height="1"/>`, true, 1},
		{"symbol success", `<symbol id="s"><rect width="1" height="1"/></symbol>`, true, 1},
		{"hidden symbol", `<symbol id="s" display="none"/>`, true, 0},
		{"ordinary failure", `<rect id="s" width="bad" height="1"/>`, false, 0},
		{"symbol style failure", `<symbol id="s" style="mix-blend-mode:multiply"/>`, false, 0},
		{"partial symbol failure", `<symbol id="s"><rect width="1" height="1"/><rect width="bad" height="1"/></symbol>`, false, 1},
		{"nested cycle", `<symbol id="s"><use href="#other"/></symbol><g id="other"><use href="#s"/></g>`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := parseXML([]byte(`<svg viewBox="0 0 100 100"><defs>` + tc.definition + `</defs><use href="#s"/></svg>`))
			if err != nil {
				t.Fatal(err)
			}
			c := &conv{root: r, defs: map[string]*node{}, resolvingUse: map[string]bool{"outer": true}}
			if !c.initViewport() || !c.collectDefsAndCSS(r) {
				t.Fatal("cannot initialize converter")
			}
			g := &pptx.GroupShape{}
			for attempt := 1; attempt <= 2; attempt++ {
				if ok := c.expandUse(r.Children[1], defaultStyle(), identity(), g); ok != tc.ok {
					t.Fatalf("attempt %d: expandUse() = %v, want %v", attempt, ok, tc.ok)
				}
				if len(c.resolvingUse) != 1 || !c.resolvingUse["outer"] {
					t.Fatalf("attempt %d: resolvingUse = %v, want only outer reference", attempt, c.resolvingUse)
				}
				if len(g.Geoms) != attempt*tc.shapes {
					t.Fatalf("attempt %d: got %d shapes, want %d (no rollback)", attempt, len(g.Geoms), attempt*tc.shapes)
				}
			}
			c.resolvingUse["s"] = true
			if c.expandUse(r.Children[1], defaultStyle(), identity(), g) || !c.resolvingUse["s"] {
				t.Fatal("rejecting an already-active reference must not clear its owner's marker")
			}
		})
	}
}
