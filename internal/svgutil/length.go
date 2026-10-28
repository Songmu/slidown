package svgutil

import (
	"strconv"
	"strings"
)

// cssLengthUnits is the set of unit suffixes recognized on a length. A value
// with any other (unknown) suffix is an invalid declaration, not a zero.
var cssLengthUnits = map[string]bool{
	"": true, "%": true,
	"px": true, "pt": true, "pc": true, "mm": true, "cm": true, "in": true,
	"em": true, "rem": true, "ex": true, "ch": true, "q": true,
	"vw": true, "vh": true, "vmin": true, "vmax": true,
}

// IsExplicitZeroLength reports whether s is an explicit zero length: a unitless
// zero, zero percent, or zero in a recognized CSS length unit. An unknown or
// malformed unit (e.g. "0foo", "0e") returns false so callers can fall back to
// the viewBox/default size instead of dropping the SVG.
func IsExplicitZeroLength(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	n := len(s)
	for n > 0 {
		c := s[n-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '%' {
			n--
			continue
		}
		break
	}
	if !cssLengthUnits[strings.ToLower(strings.TrimSpace(s[n:]))] {
		return false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s[:n]), 64)
	return err == nil && v == 0
}
