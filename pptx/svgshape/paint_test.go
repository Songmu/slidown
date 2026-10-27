package svgshape

import (
	"reflect"
	"testing"

	"github.com/Songmu/slidown/pptx"
)

func TestPaintFillValidation(t *testing.T) {
	gradient := &pptx.Gradient{Stops: []pptx.GradientStop{{Color: "ff0000", Alpha: 1}}}
	c := &conv{gradients: map[string]*pptx.Gradient{"g": gradient}}
	for _, tc := range []struct {
		name  string
		props style
		want  pptx.Fill
		ok    bool
	}{
		{"invalid opacity before fill", style{"opacity": "NaN", "fill": "red"}, pptx.Fill{}, false},
		{"invalid fill opacity", style{"fill": "red", "fill-opacity": "NaN"}, pptx.Fill{}, false},
		{"invalid fill color", style{"fill": "not-a-color"}, pptx.Fill{}, false},
		{"missing gradient", style{"fill": "url(#missing)"}, pptx.Fill{}, false},
		{"external gradient", style{"fill": "url(other.svg#g)"}, pptx.Fill{}, false},
		{"gradient alpha", style{"fill": "url('#g')", "opacity": ".5", "fill-opacity": ".5"}, pptx.Fill{Kind: pptx.FillGradient, Gradient: gradient, Alpha: .25}, true},
		{"no fill skips fill opacity", style{"fill": "none", "fill-opacity": "NaN"}, pptx.Fill{}, true},
		{"empty fill skips fill opacity", style{"fill": "", "fill-opacity": "NaN"}, pptx.Fill{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := defaultStyle()
			for k, v := range tc.props {
				st[k] = v
			}
			fill, stroke, ok := c.paint(st, identity(), false)
			if ok != tc.ok || !reflect.DeepEqual(fill, tc.want) || stroke != nil {
				t.Fatalf("paint() = (%#v, %#v, %v), want (%#v, nil, %v)", fill, stroke, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestPaintStrokeValidation(t *testing.T) {
	c := &conv{}
	for _, tc := range []struct {
		name  string
		props style
		want  *pptx.Stroke
		ok    bool
	}{
		{"invalid color at zero width", style{"stroke": "not-a-color", "stroke-width": "0"}, nil, false},
		{"invalid opacity at zero width", style{"stroke-opacity": "NaN", "stroke-width": "0"}, nil, false},
		{"invalid width", style{"stroke-width": "bad"}, nil, false},
		{"nonfinite width", style{"stroke-width": "Inf"}, nil, false},
		{"scaled width overflow", style{"stroke-width": "1e308"}, nil, false},
		{"unit conversion overflow", style{"stroke-width": "1e308in"}, nil, false},
		{"invalid cap", style{"stroke-linecap": "bad"}, nil, false},
		{"invalid join", style{"stroke-linejoin": "bad"}, nil, false},
		{"invalid miter limit", style{"stroke-miterlimit": "bad"}, nil, false},
		{"explicit default miter", style{"stroke-miterlimit": "4"}, &pptx.Stroke{Color: "0000ff", Alpha: 1, Width: 9525, Cap: "flat", Join: "miter"}, true},
		{"empty join defaults to miter", style{"stroke-linejoin": ""}, &pptx.Stroke{Color: "0000ff", Alpha: 1, Width: 9525, Cap: "flat", Join: "miter"}, true},
		{"round join ignores miter limit", style{"stroke-linecap": "ROUND", "stroke-linejoin": "ROUND", "stroke-miterlimit": "bad"}, &pptx.Stroke{Color: "0000ff", Alpha: 1, Width: 9525, Cap: "rnd", Join: "round"}, true},
		{"bevel and square cap", style{"stroke-linecap": "square", "stroke-linejoin": "bevel", "stroke-dasharray": " none "}, &pptx.Stroke{Color: "0000ff", Alpha: 1, Width: 9525, Cap: "sq", Join: "bevel"}, true},
		{"stroke only opacity", style{"opacity": ".5", "stroke-opacity": ".5"}, &pptx.Stroke{Color: "0000ff", Alpha: .25, Width: 9525, Cap: "flat", Join: "miter"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := defaultStyle()
			st["fill"], st["stroke"] = "none", "blue"
			for k, v := range tc.props {
				st[k] = v
			}
			fill, stroke, ok := c.paint(st, identity(), false)
			if ok != tc.ok || fill != (pptx.Fill{}) || !reflect.DeepEqual(stroke, tc.want) {
				t.Fatalf("paint() = (%#v, %#v, %v), want (no fill, %#v, %v)", fill, stroke, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestPaintStageOrder(t *testing.T) {
	red := pptx.Fill{Kind: pptx.FillSolid, Color: "ff0000", Alpha: .5}
	for _, tc := range []struct {
		name          string
		props         style
		m             matrix
		forceFillNone bool
		wantFill      pptx.Fill
		wantStroke    *pptx.Stroke
		ok            bool
	}{
		{
			name: "zero width skips cap join miter dash and combined opacity",
			props: style{
				"stroke-width": "0", "stroke-linecap": "bad", "stroke-linejoin": "bad",
				"stroke-miterlimit": "bad", "stroke-dasharray": "1 2",
			},
			m: identity(), wantFill: red, ok: true,
		},
		{
			name: "zero width still validates transform", props: style{"stroke-width": "0"},
			m: matrix{a: 2, d: 1}, wantFill: red,
		},
		{
			name: "stroke failure keeps fill", props: style{"stroke-opacity": "NaN"},
			m: identity(), wantFill: red,
		},
		{
			name: "dash failure discards built stroke", props: style{"stroke-dasharray": "1 2"},
			m: identity(), wantFill: red,
		},
		{
			name: "combined opacity failure discards built stroke",
			m:    identity(), wantFill: red,
		},
		{
			name: "zero alpha fill still participates in combined opacity", props: style{"fill-opacity": "0"},
			m: identity(), wantFill: pptx.Fill{Kind: pptx.FillSolid, Color: "ff0000"},
		},
		{
			name: "absent stroke skips transform and stroke validation",
			props: style{
				"stroke": "none", "stroke-opacity": "NaN", "stroke-width": "-1",
				"stroke-linecap": "bad", "stroke-dasharray": "1 2",
			},
			m: matrix{a: 2, d: 1}, wantFill: red, ok: true,
		},
		{
			name: "forced no fill skips invalid fill", props: style{"fill": "url(#missing)", "fill-opacity": "NaN"},
			m: identity(), forceFillNone: true,
			wantStroke: &pptx.Stroke{Color: "0000ff", Alpha: .5, Width: 9525, Cap: "flat", Join: "miter"}, ok: true,
		},
		{
			name: "forced no fill still validates element opacity", props: style{"opacity": "NaN"},
			m: identity(), forceFillNone: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := defaultStyle()
			st["fill"], st["stroke"], st["opacity"] = "red", "blue", ".5"
			for k, v := range tc.props {
				st[k] = v
			}
			fill, stroke, ok := (&conv{}).paint(st, tc.m, tc.forceFillNone)
			if ok != tc.ok || !reflect.DeepEqual(fill, tc.wantFill) || !reflect.DeepEqual(stroke, tc.wantStroke) {
				t.Fatalf("paint() = (%#v, %#v, %v), want (%#v, %#v, %v)", fill, stroke, ok, tc.wantFill, tc.wantStroke, tc.ok)
			}
		})
	}
}
