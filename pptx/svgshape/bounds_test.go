package svgshape

import (
	"reflect"
	"testing"

	"github.com/Songmu/slidown/pptx"
)

func TestGeomPathBounds(t *testing.T) {
	tests := []struct {
		name  string
		paths []pptx.GeomPath
		want  [4]int64
		ok    bool
	}{
		{name: "empty"},
		{
			name: "empty paths and close",
			paths: []pptx.GeomPath{{}, {Cmds: []pptx.PathCmd{
				{Verb: pptx.ClosePath},
			}}},
		},
		{
			name: "single point",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: -4, Y: 7}}},
			}}},
			want: [4]int64{-4, 7, -4, 7}, ok: true,
		},
		{
			name: "line without move does not accumulate implicit origin",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 10, Y: 20}}},
			}}},
			want: [4]int64{10, 20, 10, 20}, ok: true,
		},
		{
			// The separate axis maxima occur at t=1/3 and t=2/3,
			// where 81*t*(1-t)^2 and 81*t^2*(1-t) both reach 12.
			name: "cubic internal extrema in both axes",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{}}},
				{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{{X: 27}, {Y: 27}, {}}},
			}}},
			want: [4]int64{0, 0, 12, 12}, ok: true,
		},
		{
			name: "cubic half EMU rounds away from zero",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{}}},
				{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{{X: 2, Y: -2}, {X: 2, Y: -2}, {}}},
			}}},
			want: [4]int64{0, -2, 2, 0}, ok: true,
		},
		{
			// x peaks at t=1/2; y reaches its minimum at t=2/3.
			name: "quadratic internal extrema in both axes",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{}}},
				{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: 12, Y: -12}, {Y: -6}}},
			}}},
			want: [4]int64{0, -8, 6, 0}, ok: true,
		},
		{
			name: "quadratic half EMU rounds away from zero",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{}}},
				{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: 5, Y: -5}, {}}},
			}}},
			want: [4]int64{0, -3, 3, 0}, ok: true,
		},
		{
			name: "close restores latest subpath before a curve",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 10, Y: 10}}},
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{}}},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 20, Y: 20}}},
				{Verb: pptx.ClosePath},
				{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: -20, Y: -20}, {}}},
			}}},
			want: [4]int64{-10, -10, 20, 20}, ok: true,
		},
		{
			name: "current point persists across paths",
			paths: []pptx.GeomPath{
				{Cmds: []pptx.PathCmd{{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 20, Y: 20}}}}},
				{Cmds: []pptx.PathCmd{{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: -20, Y: -20}, {X: 20, Y: 20}}}}},
			},
			want: [4]int64{0, 0, 20, 20}, ok: true,
		},
		{
			name: "subpath start persists across paths",
			paths: []pptx.GeomPath{
				{Cmds: []pptx.PathCmd{
					{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 20, Y: 20}}},
					{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{}}},
				}},
				{Cmds: []pptx.PathCmd{
					{Verb: pptx.ClosePath},
					{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: -20, Y: -20}, {X: 20, Y: 20}}},
				}},
			},
			want: [4]int64{0, 0, 20, 20}, ok: true,
		},
		{
			name: "insufficient points leave state unchanged and extra points are ignored",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 20, Y: 20}, {X: 999, Y: 999}}},
				{Verb: pptx.MoveTo},
				{Verb: pptx.LineTo},
				{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{{X: -999}, {Y: -999}}},
				{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: -999, Y: -999}}},
				{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: -20, Y: -20}, {X: 20, Y: 20}, {X: 999, Y: 999}}},
			}}},
			want: [4]int64{0, 0, 20, 20}, ok: true,
		},
		{
			name: "degenerate curves",
			paths: []pptx.GeomPath{{Cmds: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 3, Y: 4}}},
				{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{{X: 3, Y: 4}, {X: 3, Y: 4}, {X: 3, Y: 4}}},
				{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{{X: 3, Y: 4}, {X: 3, Y: 4}}},
			}}},
			want: [4]int64{3, 4, 3, 4}, ok: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x0, y0, x1, y1, ok := geomPathBounds(&pptx.GeomShape{Paths: tt.paths})
			if got := [4]int64{x0, y0, x1, y1}; got != tt.want || ok != tt.ok {
				t.Fatalf("geomPathBounds() = %v, %v; want %v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestConvertVerticalPath(t *testing.T) {
	tests := []struct {
		name, d string
		want    []pptx.PathCmd
	}{
		{
			name: "absolute repeated coordinates", d: "M2 3 V7 1",
			want: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 19050, Y: 28575}}},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 19050, Y: 66675}}},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 19050, Y: 9525}}},
			},
		},
		{
			name: "relative repeated coordinates after close", d: "M2 3 v4 -2 z v1",
			want: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 19050, Y: 28575}}},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 19050, Y: 66675}}},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 19050, Y: 47625}}},
				{Verb: pptx.ClosePath},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 19050, Y: 38100}}},
			},
		},
		{
			name: "fractional EMU rounding", d: "M2 3 V3.0002",
			want: []pptx.PathCmd{
				{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{{X: 19050, Y: 28575}}},
				{Verb: pptx.LineTo, Pts: []pptx.PathPoint{{X: 19050, Y: 28577}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := mustConvert(t, `<svg viewBox="0 0 100 100"><path d="`+tt.d+`" fill="none" stroke="black"/></svg>`)
			if got := g.Geoms[0].Paths[0].Cmds; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("commands = %#v; want %#v", got, tt.want)
			}
		})
	}
}

func TestMalformedVerticalPath(t *testing.T) {
	_, ok := Convert([]byte(`<svg viewBox="0 0 100 100"><path d="M2 3 V1e" fill="none" stroke="black"/></svg>`))
	if ok {
		t.Fatal("Convert() accepted a malformed vertical coordinate")
	}
}
