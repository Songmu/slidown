package svgshape

import (
	"strconv"
	"unicode"
)

func skipNumberSeparators(s string, i int) int {
	for i < len(s) && (unicode.IsSpace(rune(s[i])) || s[i] == ',') {
		i++
	}
	return i
}

// scanNumber returns the cursor even on failure. Incomplete exponents are left
// unconsumed; a sign or decimal point without digits is still consumed.
func scanNumber(s string, i int) (float64, int, bool) {
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	integerStart := i
	i = scanDigits(s, i)
	digits := i > integerStart
	if i < len(s) && s[i] == '.' {
		i++
		fractionStart := i
		i = scanDigits(s, i)
		digits = digits || i > fractionStart
	}
	if !digits {
		return 0, i, false
	}
	i = scanExponent(s, i)
	v, err := strconv.ParseFloat(s[start:i], 64)
	return v, i, err == nil
}

func scanDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

func scanExponent(s string, i int) int {
	if i >= len(s) || (s[i] != 'e' && s[i] != 'E') {
		return i
	}
	j := i + 1
	if j < len(s) && (s[j] == '+' || s[j] == '-') {
		j++
	}
	end := scanDigits(s, j)
	if end == j {
		return i
	}
	return end
}
