package svgshape

import (
	"math"
	"testing"
)

func TestParseTransform(t *testing.T) {
	tests := []struct {
		name, input string
		want        matrix
	}{
		{"empty", " \t", identity()},
		{"matrix", "matrix(1 2 3 4 5 6)", matrix{1, 2, 3, 4, 5, 6}},
		{"translate default", "translate(2)", matrix{a: 1, d: 1, e: 2}},
		{"translate adjacent signs", "translate(2-3)", matrix{a: 1, d: 1, e: 2, f: -3}},
		{"scale default", "scale(-2)", matrix{a: -2, d: -2}},
		{"scale axes", "scale(2 3)", matrix{a: 2, d: 3}},
		{"rotate origin", "rotate(90)", matrix{b: 1, c: -1}},
		{"rotate center", "rotate(90 2 3)", matrix{b: 1, c: -1, e: 5, f: 1}},
		{"skew x", "skewX(45)", matrix{a: 1, c: 1, d: 1}},
		{"skew y", "skewY(45)", matrix{a: 1, b: 1, d: 1}},
		{"composition", "translate(10 20) scale(2 3)", matrix{a: 2, d: 3, e: 10, f: 20}},
		{"reverse composition", "scale(2 3) translate(10 20)", matrix{a: 2, d: 3, e: 20, f: 60}},
		{"permissive list", " TrAnSlAtE (,,2,,3,)scale(2), ", matrix{a: 2, d: 2, e: 2, f: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTransform(tt.input)
			if !ok {
				t.Fatalf("parseTransform(%q) failed", tt.input)
			}
			gv := [...]float64{got.a, got.b, got.c, got.d, got.e, got.f}
			wv := [...]float64{tt.want.a, tt.want.b, tt.want.c, tt.want.d, tt.want.e, tt.want.f}
			for i := range gv {
				if math.IsNaN(gv[i]) || math.Abs(gv[i]-wv[i]) > 1e-12 {
					t.Fatalf("parseTransform(%q) = %+v; want %+v", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestParseTransformRejects(t *testing.T) {
	for _, input := range []string{
		"matrix()", "matrix(1 2 3 4 5)", "matrix(1 2 3 4 5 6 7)",
		"translate()", "translate(1 2 3)", "scale()", "scale(1 2 3)",
		"rotate()", "rotate(1 2)", "rotate(1 2 3 4)",
		"skewX()", "skewX(1 2)", "skewY()", "skewY(1 2)",
		"unknown(1)", "translate", "translate(1", "translate((1))",
		"translate(1e)", "translate(1e309)", "translate(1) scale()",
		"translate(1),,", ",translate(1)", "translate(1) junk",
	} {
		t.Run(input, func(t *testing.T) {
			if got, ok := parseTransform(input); ok || got != (matrix{}) {
				t.Fatalf("parseTransform(%q) = %+v, %v; want zero matrix, false", input, got, ok)
			}
		})
	}
}
