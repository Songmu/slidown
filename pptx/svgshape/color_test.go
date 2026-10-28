package svgshape

import "testing"

func TestParseColor(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		ok          bool
	}{
		{" \tReBeCcApUrPlE\n", "663399", true},
		{"currentColor", "000000", true},
		{"transparent", "000000", true},
		{"#AbC", "aabbcc", true},
		{"#a0B1c2", "a0b1c2", true},
		{" RGB( +1, 2, 255 ) ", "0102ff", true},
		{"rgb(50%, .2%, 100%)", "8001ff", true},
		{"rgb(-1,256,+2)", "00ff02", true},
		{"", "", false},
		{"notacolor", "", false},
		{"#abcd", "", false},
		{"#00gg00", "", false},
		{"rgb(1,2)", "", false},
		{"rgb(1.0,2,3)", "", false},
		{"rgb(nan%,0,0)", "", false},
		{"rgb(inf%,0,0)", "", false},
		{"rgb(1e309%,0,0)", "", false},
		{"rgba(1,2,3,1)", "", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := parseColor(tc.input)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("parseColor(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.want, tc.ok)
			}
		})
	}
}
