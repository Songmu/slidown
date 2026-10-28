package svgutil

import "testing"

func TestIsExplicitZeroLength(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{" \t\n", false},
		{"0", true},
		{"-0.0", true},
		{"\u00a0+0 \tPX\n", true},
		{"0%", true},
		{"0rEm", true},
		{"0vMiN", true},
		{"0Q", true},
		{"1px", false},
		{"0foo", false},
		{"0e", false},
		{"0px%", false},
		{"px", false},
		{"0p x", false},
		{"0e+10px", true},
		{"0x0p0px", true},
		{"0_0px", true},
		{"1e-9999px", true},
		{"1e9999px", false},
		{"NaN", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsExplicitZeroLength(tt.input); got != tt.want {
				t.Errorf("IsExplicitZeroLength(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
