package svgshape

import (
	"math"
	"strings"
)

type point struct{ x, y float64 }
type matrix struct{ a, b, c, d, e, f float64 }

func identity() matrix               { return matrix{a: 1, d: 1} }
func (m matrix) apply(p point) point { return point{m.a*p.x + m.c*p.y + m.e, m.b*p.x + m.d*p.y + m.f} }
func (m matrix) mul(n matrix) matrix {
	return matrix{a: m.a*n.a + m.c*n.b, b: m.b*n.a + m.d*n.b, c: m.a*n.c + m.c*n.d, d: m.b*n.c + m.d*n.d, e: m.a*n.e + m.c*n.f + m.e, f: m.b*n.e + m.d*n.f + m.f}
}
func (m matrix) avgScale() float64 {
	det := m.a*m.d - m.b*m.c
	if det < 0 {
		det = -det
	}
	if det == 0 {
		return 1
	}
	return math.Sqrt(det)
}

// isSimilarity reports whether the linear part is a similarity transform
// (uniform scale plus rotation/reflection), i.e. it maps circles to circles.
// Non-similarity transforms (non-uniform scale, skew) stretch a stroke outline
// anisotropically, which a single uniform stroke width can't reproduce.
func (m matrix) isSimilarity() bool {
	orthogonal := math.Abs(m.a*m.c+m.b*m.d) < 1e-9
	equalLen := math.Abs((m.a*m.a+m.b*m.b)-(m.c*m.c+m.d*m.d)) < 1e-9
	return orthogonal && equalLen
}

// isTranslateOnly reports whether the matrix is a pure translation (no scale,
// rotation or skew). Text conversion only translates the anchor point, so any
// other component would misplace/mis-size glyphs.
func (m matrix) isTranslateOnly() bool {
	return math.Abs(m.a-1) < 1e-9 && math.Abs(m.d-1) < 1e-9 &&
		math.Abs(m.b) < 1e-9 && math.Abs(m.c) < 1e-9
}

func parseTransform(s string) (matrix, bool) {
	res := identity()
	s = strings.TrimSpace(s)
	for s != "" {
		s = strings.TrimSpace(s)
		i := strings.IndexByte(s, '(')
		if i < 0 {
			return matrix{}, false
		}
		name := strings.ToLower(strings.TrimSpace(s[:i]))
		j := findCloseParen(s, i)
		if j < 0 {
			return matrix{}, false
		}
		nums, ok := scanNumbers(s[i+1 : j])
		if !ok {
			return matrix{}, false
		}
		var tm matrix
		switch name {
		case "matrix":
			if len(nums) != 6 {
				return matrix{}, false
			}
			tm = matrix{nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]}
		case "translate":
			if len(nums) < 1 || len(nums) > 2 {
				return matrix{}, false
			}
			ty := 0.0
			if len(nums) == 2 {
				ty = nums[1]
			}
			tm = matrix{a: 1, d: 1, e: nums[0], f: ty}
		case "scale":
			if len(nums) < 1 || len(nums) > 2 {
				return matrix{}, false
			}
			sy := nums[0]
			if len(nums) == 2 {
				sy = nums[1]
			}
			tm = matrix{a: nums[0], d: sy}
		case "rotate":
			if len(nums) != 1 && len(nums) != 3 {
				return matrix{}, false
			}
			a := nums[0] * math.Pi / 180
			r := matrix{a: math.Cos(a), b: math.Sin(a), c: -math.Sin(a), d: math.Cos(a)}
			if len(nums) == 3 {
				tm = matrix{a: 1, d: 1, e: nums[1], f: nums[2]}.mul(r).mul(matrix{a: 1, d: 1, e: -nums[1], f: -nums[2]})
			} else {
				tm = r
			}
		case "skewx":
			if len(nums) != 1 {
				return matrix{}, false
			}
			tm = matrix{a: 1, d: 1, c: math.Tan(nums[0] * math.Pi / 180)}
		case "skewy":
			if len(nums) != 1 {
				return matrix{}, false
			}
			tm = matrix{a: 1, d: 1, b: math.Tan(nums[0] * math.Pi / 180)}
		default:
			return matrix{}, false
		}
		res = res.mul(tm)
		s = strings.TrimSpace(s[j+1:])
		if strings.HasPrefix(s, ",") {
			s = strings.TrimSpace(s[1:])
		}
	}
	return res, true
}
func findCloseParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		}
		if s[i] == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
