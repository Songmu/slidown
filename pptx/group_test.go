package pptx

import (
	"fmt"
	"strings"
	"testing"
)

func TestContentTypesIncludesSVGDefault(t *testing.T) {
	out := contentTypes(1, nil)
	if !strings.Contains(out, `<Default Extension="svg" ContentType="image/svg+xml"/>`) {
		t.Errorf("expected svg content type default, got: %s", out)
	}
}

func TestRenderGeomSolidFill(t *testing.T) {
	gs := &GeomShape{
		Name:  "Curve",
		X:     10,
		Y:     20,
		W:     300,
		H:     400,
		PathW: 1000,
		PathH: 1000,
		Paths: []GeomPath{{
			Cmds: []PathCmd{
				{Verb: MoveTo, Pts: []PathPoint{{X: 0, Y: 0}}},
				{Verb: CubicTo, Pts: []PathPoint{{X: 10, Y: 20}, {X: 30, Y: 40}, {X: 50, Y: 60}}},
				{Verb: ClosePath},
			},
		}},
		Fill:   Fill{Kind: FillSolid, Color: "ff0000", Alpha: 0.5},
		Stroke: &Stroke{Color: "0000ff", Alpha: 0.75, Width: 0, Cap: "rnd", Dash: "dash"},
	}
	out := renderGeom(gs, 7)

	for _, want := range []string{
		`<a:custGeom>`,
		`<a:moveTo><a:pt x="0" y="0"/></a:moveTo>`,
		`<a:cubicBezTo><a:pt x="10" y="20"/><a:pt x="30" y="40"/><a:pt x="50" y="60"/></a:cubicBezTo>`,
		`<a:solidFill><a:srgbClr val="ff0000"><a:alpha val="50000"/></a:srgbClr></a:solidFill>`,
		`<a:ln w="1" cap="rnd">`,
		`<a:prstDash val="dash"/>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in: %s", want, out)
		}
	}
}

func TestRenderGeomGradientFill(t *testing.T) {
	gs := &GeomShape{
		PathW: 100,
		PathH: 100,
		Paths: []GeomPath{{
			Cmds: []PathCmd{
				{Verb: MoveTo, Pts: []PathPoint{{X: 0, Y: 0}}},
				{Verb: LineTo, Pts: []PathPoint{{X: 100, Y: 0}}},
			},
		}},
		Fill: Fill{Kind: FillGradient, Gradient: &Gradient{
			Kind:  GradientLinear,
			Angle: 45,
			Stops: []GradientStop{
				{Pos: 0, Color: "ff0000", Alpha: 1},
				{Pos: 1, Color: "00ff00", Alpha: 0.25},
			},
		}},
	}
	out := renderGeom(gs, 8)

	if !strings.Contains(out, `<a:gradFill><a:gsLst>`) {
		t.Errorf("expected gradient fill, got: %s", out)
	}
	if got := strings.Count(out, `<a:gs `); got != 2 {
		t.Errorf("expected 2 gradient stops, got %d in: %s", got, out)
	}
	if !strings.Contains(out, `<a:lin ang="2700000" scaled="1"/>`) {
		t.Errorf("expected linear angle, got: %s", out)
	}
}

func TestRenderGroup(t *testing.T) {
	g := &GroupShape{
		Name: "Group",
		X:    1, Y: 2, W: 3, H: 4,
		ChX: 5, ChY: 6, ChW: 7, ChH: 8,
		Geoms: []*GeomShape{{
			Name:  "Geom",
			PathW: 10,
			PathH: 10,
			Paths: []GeomPath{{Cmds: []PathCmd{{Verb: MoveTo, Pts: []PathPoint{{X: 0, Y: 0}}}}}},
			Fill:  Fill{Kind: FillNone},
		}},
		Texts: []*Shape{{Name: "Text", X: 9, Y: 10, W: 11, H: 12}},
	}
	var rels []slideRel
	id, relIdx := 20, 1
	out := renderGroup(g, &id, &relIdx, &rels)

	for _, want := range []string{
		`<p:grpSp>`,
		`<a:off x="1" y="2"/>`,
		`<a:ext cx="3" cy="4"/>`,
		`<a:chOff x="5" y="6"/>`,
		`<a:chExt cx="7" cy="8"/>`,
		`<p:cNvPr id="20" name="Group"/>`,
		`<p:cNvPr id="21" name="Geom"/>`,
		`<p:cNvPr id="22" name="Text"/>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in: %s", want, out)
		}
	}
	if id != 23 {
		t.Errorf("id after renderGroup = %d, want 23", id)
	}
	for i := 20; i <= 22; i++ {
		if strings.Count(out, fmt.Sprintf(`id="%d"`, i)) != 1 {
			t.Errorf("expected id %d exactly once in: %s", i, out)
		}
	}
}

func TestRenderSlideIncludesGroups(t *testing.T) {
	s := &Slide{Groups: []*GroupShape{{
		Name: "Group",
		W:    100,
		H:    100,
		ChW:  100,
		ChH:  100,
		Geoms: []*GeomShape{{
			Name:  "Geom",
			PathW: 10,
			PathH: 10,
			Paths: []GeomPath{{Cmds: []PathCmd{{Verb: MoveTo, Pts: []PathPoint{{X: 0, Y: 0}}}}}},
			Fill:  Fill{Kind: FillNone},
		}},
	}}}
	mediaIdx := 0
	out, _, _ := renderSlide(s, &mediaIdx)

	if !strings.Contains(out, `<p:grpSp>`) || !strings.Contains(out, `<a:custGeom>`) {
		t.Errorf("expected rendered group and custom geometry in slide XML, got: %s", out)
	}
}
