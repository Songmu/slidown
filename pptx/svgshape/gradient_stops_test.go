package svgshape

import (
	"reflect"
	"testing"

	"github.com/Songmu/slidown/pptx"
)

func TestConvertGradientStopOpacityCascade(t *testing.T) {
	for _, tt := range []struct {
		name, inline string
		want         float64
	}{
		{"stylesheet_over_attribute", "", .25},
		{"inline_over_stylesheet", ` style="stop-opacity:75%"`, .75},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := mustConvert(t, `<svg viewBox="0 0 10 10"><style>.s{stop-opacity:25%}</style><defs><linearGradient id="g" stop-opacity=".1"><stop class="s" offset="0" stop-color="red" stop-opacity=".5"`+tt.inline+`/><stop offset="1"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
			want := []pptx.GradientStop{
				{Pos: 0, Color: "ff0000", Alpha: tt.want},
				{Pos: 1, Color: "000000", Alpha: 1},
			}
			if got := g.Geoms[0].Fill.Gradient.Stops; !reflect.DeepEqual(got, want) {
				t.Fatalf("stops = %#v, want %#v", got, want)
			}
		})
	}
}

func TestConvertGradientStopInvalidValues(t *testing.T) {
	for _, tt := range []struct {
		name, attrs string
	}{
		{"missing_offset", `stop-color="red"`},
		{"invalid_offset", `offset="bad" stop-color="red"`},
		{"nonfinite_offset", `offset="NaN" stop-color="red"`},
		{"invalid_color", `offset="0" stop-color="not-a-color"`},
		{"invalid_opacity", `offset="0" stop-color="red" stop-opacity="bad"`},
		{"nonfinite_opacity", `offset="0" stop-color="red" stop-opacity="Inf"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svg := `<svg viewBox="0 0 10 10"><defs><linearGradient id="g"><stop ` + tt.attrs + `/><stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`
			if _, ok := Convert([]byte(svg)); ok {
				t.Fatal("invalid gradient stop should fall back")
			}
		})
	}
}

func TestConvertGradientStopsSkipNonSVGStops(t *testing.T) {
	for _, tt := range []struct {
		name, child string
	}{
		{"non_stop", `<desc offset="bad" stop-color="not-a-color"/>`},
		{"foreign_stop", `<stop xmlns="urn:other" offset="bad" stop-color="not-a-color"/>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := mustConvert(t, `<svg viewBox="0 0 10 10"><defs><linearGradient id="g"><stop offset=".25" stop-color="red"/>`+tt.child+`<stop offset=".75" stop-color="blue"/></linearGradient></defs><rect width="10" height="10" fill="url(#g)"/></svg>`)
			want := []pptx.GradientStop{
				{Pos: .25, Color: "ff0000", Alpha: 1},
				{Pos: .75, Color: "0000ff", Alpha: 1},
			}
			if got := g.Geoms[0].Fill.Gradient.Stops; !reflect.DeepEqual(got, want) {
				t.Fatalf("stops = %#v, want %#v", got, want)
			}
		})
	}
}
