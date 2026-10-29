package svgshape

import (
	"fmt"
	"math"
	"testing"

	"github.com/Songmu/slidown/pptx"
)

func TestConvertCurveCoordinates(t *testing.T) {
	tests := []struct {
		name, d string
		verbs   []pptx.PathVerb
		points  [][]point
	}{
		{
			name:  "relative quadratics advance the origin",
			d:     "M10 20 q2 4 6 8 -1 -2 4 2",
			verbs: []pptx.PathVerb{pptx.MoveTo, pptx.QuadTo, pptx.QuadTo},
			points: [][]point{
				{{10, 20}}, {{12, 24}, {16, 28}}, {{15, 26}, {20, 30}},
			},
		},
		{
			name:  "relative quarter circle and following line",
			d:     "M20 20 a10 10 0 0 1 10 10 l2 3",
			verbs: []pptx.PathVerb{pptx.MoveTo, pptx.CubicTo, pptx.LineTo},
			// A quarter-circle cubic uses k = 0.5522847498 times the radius.
			points: [][]point{
				{{20, 20}}, {{25.522847498, 20}, {30, 24.477152502}, {30, 30}}, {{32, 33}},
			},
		},
		{
			name:  "zero horizontal radius becomes a straight cubic",
			d:     "M20 20 a0 10 0 0 1 10 10",
			verbs: []pptx.PathVerb{pptx.MoveTo, pptx.CubicTo},
			points: [][]point{
				{{20, 20}}, {{20, 20}, {30, 30}, {30, 30}},
			},
		},
		{
			name:  "zero vertical radius becomes a straight cubic",
			d:     "M20 20 A10 0 0 0 0 30 30",
			verbs: []pptx.PathVerb{pptx.MoveTo, pptx.CubicTo},
			points: [][]point{
				{{20, 20}}, {{20, 20}, {30, 30}, {30, 30}},
			},
		},
		{
			name:  "undersized negative radii expand to a semicircle",
			d:     "M20 30 A-5 -5 0 0 1 40 30",
			verbs: []pptx.PathVerb{pptx.MoveTo, pptx.CubicTo, pptx.CubicTo},
			// The chord is 20 units, so corrected radii are 10, centered at (30,30).
			points: [][]point{
				{{20, 30}},
				{{20, 24.477152502}, {24.477152502, 20}, {30, 20}},
				{{35.522847498, 20}, {40, 24.477152502}, {40, 30}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := mustConvert(t, `<svg viewBox="0 0 100 100"><path d="`+tt.d+`" fill="none" stroke="black"/></svg>`)
			cmds := g.Geoms[0].Paths[0].Cmds
			if len(cmds) != len(tt.verbs) {
				t.Fatalf("got %d commands, want %d: %#v", len(cmds), len(tt.verbs), cmds)
			}
			for i, cmd := range cmds {
				if cmd.Verb != tt.verbs[i] || len(cmd.Pts) != len(tt.points[i]) {
					t.Fatalf("command %d = %#v; want verb %v and %d points", i, cmd, tt.verbs[i], len(tt.points[i]))
				}
				for j, p := range cmd.Pts {
					want := tt.points[i][j]
					t.Run(fmt.Sprintf("command_%d_point_%d", i, j), func(t *testing.T) {
						near(t, p.X, int64(math.Round(want.x*9525)))
						near(t, p.Y, int64(math.Round(want.y*9525)))
					})
				}
			}
		})
	}
}

func TestMalformedCurvePaths(t *testing.T) {
	paths := map[string]string{
		"quadratic incomplete control": "M10 10 q1",
		"quadratic incomplete end":     "M10 10 q1 2 3",
		"arc malformed radius x":       "M10 10 A+ 10 0 0 1 20 20",
		"arc missing radius y":         "M10 10 A10",
		"arc malformed rotation":       "M10 10 A10 10 - 0 1 20 20",
		"arc invalid large flag":       "M10 10 A10 10 0 2 1 20 20",
		"arc invalid sweep flag":       "M10 10 A10 10 0 0 2 20 20",
		"arc incomplete end":           "M10 10 A10 10 0 0 1 20",
	}
	for name, d := range paths {
		paths[name] = `<svg viewBox="0 0 100 100"><path d="` + d + `"/></svg>`
	}
	testConvertFallbacks(t, paths)
}
