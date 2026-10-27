package svgshape

import (
	"math"
	"reflect"
	"testing"
)

func TestScanNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxCount int
		want     []float64
		ok       bool
	}{
		{"empty", "", 0, nil, true},
		{"separators", " ,\t\r\n,,", 0, nil, true},
		{"adjacent signs", "1-2+3", 0, []float64{1, -2, 3}, true},
		{"adjacent decimals", ".5.25 1. -2.", 0, []float64{.5, .25, 1, -2}, true},
		{"exponents", "1e2-2E-1+.3e+2", 0, []float64{100, -.2, 30}, true},
		{"incomplete exponent", "1e+", 0, nil, false},
		{"missing mantissa", ".e2", 0, nil, false},
		{"trailing sign", "1 +", 0, nil, false},
		{"trailing dot", "1 .", 0, nil, false},
		{"overflow", "1 1e309", 0, nil, false},
		{"underflow", "1e-4000", 0, []float64{0}, true},
		{"non-number", "NaN", 0, nil, false},
		{"byte whitespace", "\xa01,\x851", 0, []float64{1, 1}, true},
		{"utf8 whitespace", "1\u00a02", 0, nil, false},
		{"at limit", "1,2,", 2, []float64{1, 2}, true},
		{"over limit", "1,2,3", 2, nil, false},
		{"negative limit", "1 2", -1, []float64{1, 2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := scanNumbers(tt.input, tt.maxCount)
			if ok != tt.ok || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("scanNumbers(%q, %d) = %v, %v; want %v, %v",
					tt.input, tt.maxCount, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPathParserNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		start int
		want  float64
		end   int
		ok    bool
	}{
		{"empty", "", 0, 0, 0, false},
		{"separators", ", \t", 0, 0, 3, false},
		{"signed decimal", ", -.5+2", 0, -.5, 5, true},
		{"second decimal", "1.2.3", 0, 1.2, 3, true},
		{"exponent", "+.5E-2L", 0, .005, 6, true},
		{"incomplete exponent", "12e+", 0, 12, 2, true},
		{"empty exponent", "12E", 0, 12, 2, true},
		{"invalid exponent sign", "12e--2", 0, 12, 2, true},
		{"missing digits", " ,+.e2", 0, 0, 4, false},
		{"sign only", "-", 0, 0, 1, false},
		{"command", "M1", 0, 0, 0, false},
		{"nonzero cursor", "M-2e+3L", 1, -2000, 6, true},
		{"overflow", "-1e309L", 0, math.Inf(-1), 6, false},
		{"underflow", "-1e-4000L", 0, math.Copysign(0, -1), 8, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pathParser{s: tt.input, i: tt.start}
			got, ok := p.num()
			if ok != tt.ok || got != tt.want || math.Signbit(got) != math.Signbit(tt.want) || p.i != tt.end {
				t.Fatalf("num() = %v, %v at %d; want %v, %v at %d",
					got, ok, p.i, tt.want, tt.ok, tt.end)
			}
		})
	}
}
