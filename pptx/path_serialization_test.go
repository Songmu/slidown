package pptx

import (
	"strings"
	"testing"
)

func TestGroupPathSerialization(t *testing.T) {
	tests := []struct {
		name string
		cmds []PathCmd
		want string
	}{
		{
			name: "curve coordinates and command order",
			cmds: []PathCmd{
				{Verb: MoveTo, Pts: []PathPoint{{X: 10, Y: 20}}},
				{Verb: QuadTo, Pts: []PathPoint{{X: -30, Y: 40}, {X: 50, Y: 60}}},
				{Verb: CubicTo, Pts: []PathPoint{{X: 70, Y: -80}, {X: 90, Y: 100}, {X: 110, Y: 120}}},
				{Verb: LineTo, Pts: []PathPoint{{X: 130, Y: 140}}},
				{Verb: ClosePath},
			},
			want: `<a:moveTo><a:pt x="10" y="20"/></a:moveTo>` +
				`<a:quadBezTo><a:pt x="-30" y="40"/><a:pt x="50" y="60"/></a:quadBezTo>` +
				`<a:cubicBezTo><a:pt x="70" y="-80"/><a:pt x="90" y="100"/><a:pt x="110" y="120"/></a:cubicBezTo>` +
				`<a:lnTo><a:pt x="130" y="140"/></a:lnTo><a:close/>`,
		},
		{
			name: "incomplete and unknown commands are omitted",
			cmds: []PathCmd{
				{Verb: MoveTo},
				{Verb: MoveTo, Pts: []PathPoint{{X: 10, Y: 20}}},
				{Verb: LineTo},
				{Verb: QuadTo, Pts: []PathPoint{{X: 999, Y: 999}}},
				{Verb: CubicTo, Pts: []PathPoint{{X: 888, Y: 888}, {X: 777, Y: 777}}},
				{Verb: PathVerb(99), Pts: []PathPoint{{X: 666, Y: 666}}},
				{Verb: LineTo, Pts: []PathPoint{{X: 30, Y: 40}}},
				{Verb: ClosePath},
			},
			want: `<a:moveTo><a:pt x="10" y="20"/></a:moveTo>` +
				`<a:lnTo><a:pt x="30" y="40"/></a:lnTo><a:close/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			p.AddSlide().AddGroup(&GroupShape{
				Name: "Path group",
				W:    200, H: 300, ChW: 200, ChH: 300,
				Geoms: []*GeomShape{{
					Name:  "Path",
					W:     200,
					H:     300,
					PathW: 200,
					PathH: 300,
					Paths: []GeomPath{{Cmds: tt.cmds}},
					Fill:  Fill{Kind: FillNone},
				}},
			})
			slide := writeToParts(t, p)["ppt/slides/slide1.xml"]
			if err := assertXMLWellFormed(slide); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(slide, `<p:grpSp>`) {
				t.Fatal("slide is missing the geometry group")
			}
			start, end := strings.Index(slide, `<a:pathLst>`), strings.Index(slide, `</a:pathLst>`)
			if start < 0 || end < start {
				t.Fatalf("slide is missing a complete path list: %s", slide)
			}
			got := slide[start : end+len(`</a:pathLst>`)]
			want := `<a:pathLst><a:path w="200" h="300">` + tt.want + `</a:path></a:pathLst>`
			if got != want {
				t.Errorf("path XML = %s; want %s", got, want)
			}
		})
	}
}
