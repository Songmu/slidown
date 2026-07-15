package svgshape

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/Songmu/slidown/pptx"
)

type pathMapper func(x, y float64) pptx.PathPoint

func parsePath(d string, budget int, mapPt pathMapper) (pptx.GeomPath, bool, bool) {
	p := &pathParser{s: d}
	var cmds []pptx.PathCmd
	var cmd byte
	cur, start := point{}, point{}
	var lastC, lastQ point
	lastWasC, lastWasQ := false, false
	started := false
	for p.more() {
		if len(cmds) > budget {
			return pptx.GeomPath{}, false, false
		}
		startIdx := p.i
		consumedCmd := false
		if p.isCommand() {
			cmd = p.nextCommand()
			consumedCmd = true
			// A valid path must begin with a moveto command.
			if !started && cmd != 'M' && cmd != 'm' {
				return pptx.GeomPath{}, false, false
			}
			started = true
			// Every command except close must be followed by operands; an
			// explicit command with no number is a malformed path.
			if cmd != 'Z' && cmd != 'z' && !p.hasNumber() {
				return pptx.GeomPath{}, false, false
			}
		} else if cmd == 0 {
			return pptx.GeomPath{}, false, false
		}
		switch cmd {
		case 'M', 'm':
			x, y, ok := p.two()
			if !ok {
				return pptx.GeomPath{}, false, false
			}
			if cmd == 'm' {
				x += cur.x
				y += cur.y
			}
			cur = point{x, y}
			start = cur
			cmds = append(cmds, pptx.PathCmd{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{mapPt(cur.x, cur.y)}})
			cmd = toggle(cmd, 'L', 'l')
			lastWasC, lastWasQ = false, false
		case 'L', 'l':
			for p.hasNumber() && len(cmds) <= budget {
				x, y, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 'l' {
					x += cur.x
					y += cur.y
				}
				cur = point{x, y}
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(cur.x, cur.y)}})
			}
			lastWasC, lastWasQ = false, false
		case 'H', 'h':
			for p.hasNumber() && len(cmds) <= budget {
				x, ok := p.num()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 'h' {
					x += cur.x
				}
				cur.x = x
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(cur.x, cur.y)}})
			}
			lastWasC, lastWasQ = false, false
		case 'V', 'v':
			for p.hasNumber() && len(cmds) <= budget {
				y, ok := p.num()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 'v' {
					y += cur.y
				}
				cur.y = y
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(cur.x, cur.y)}})
			}
			lastWasC, lastWasQ = false, false
		case 'C', 'c':
			for p.hasNumber() && len(cmds) <= budget {
				x1, y1, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				x2, y2, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				x, y, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 'c' {
					x1 += cur.x
					y1 += cur.y
					x2 += cur.x
					y2 += cur.y
					x += cur.x
					y += cur.y
				}
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(x1, y1), mapPt(x2, y2), mapPt(x, y)}})
				cur = point{x, y}
				lastC = point{x2, y2}
				lastWasC = true
				lastWasQ = false
			}
		case 'S', 's':
			for p.hasNumber() && len(cmds) <= budget {
				c1 := cur
				if lastWasC {
					c1 = point{2*cur.x - lastC.x, 2*cur.y - lastC.y}
				}
				x2, y2, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				x, y, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 's' {
					x2 += cur.x
					y2 += cur.y
					x += cur.x
					y += cur.y
				}
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(c1.x, c1.y), mapPt(x2, y2), mapPt(x, y)}})
				cur = point{x, y}
				lastC = point{x2, y2}
				lastWasC = true
				lastWasQ = false
			}
		case 'Q', 'q':
			for p.hasNumber() && len(cmds) <= budget {
				x1, y1, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				x, y, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 'q' {
					x1 += cur.x
					y1 += cur.y
					x += cur.x
					y += cur.y
				}
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{mapPt(x1, y1), mapPt(x, y)}})
				cur = point{x, y}
				lastQ = point{x1, y1}
				lastWasQ = true
				lastWasC = false
			}
		case 'T', 't':
			for p.hasNumber() && len(cmds) <= budget {
				c := cur
				if lastWasQ {
					c = point{2*cur.x - lastQ.x, 2*cur.y - lastQ.y}
				}
				x, y, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 't' {
					x += cur.x
					y += cur.y
				}
				cmds = append(cmds, pptx.PathCmd{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{mapPt(c.x, c.y), mapPt(x, y)}})
				cur = point{x, y}
				lastQ = c
				lastWasQ = true
				lastWasC = false
			}
		case 'A', 'a':
			for p.hasNumber() && len(cmds) <= budget {
				rx, ok := p.num()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				ry, ok := p.num()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				rot, ok := p.num()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				laf, ok := p.flag()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				sf, ok := p.flag()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				x, y, ok := p.two()
				if !ok {
					return pptx.GeomPath{}, false, false
				}
				if cmd == 'a' {
					x += cur.x
					y += cur.y
				}
				cubs := arcToCubics(cur.x, cur.y, rx, ry, rot, laf, sf, x, y)
				for _, cb := range cubs {
					cmds = append(cmds, pptx.PathCmd{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(cb[0].x, cb[0].y), mapPt(cb[1].x, cb[1].y), mapPt(cb[2].x, cb[2].y)}})
				}
				cur = point{x, y}
				lastWasC, lastWasQ = false, false
			}
		case 'Z', 'z':
			cmds = append(cmds, pptx.PathCmd{Verb: pptx.ClosePath})
			cur = start
			lastWasC, lastWasQ = false, false
		default:
			return pptx.GeomPath{}, false, false
		}
		// Guard against non-progressing iterations (e.g. a persisting command
		// such as "Z" followed by stray tokens, or an operand-less command that
		// consumed nothing): without progress the loop would spin forever.
		if !consumedCmd && p.i == startIdx {
			return pptx.GeomPath{}, false, false
		}
	}
	if len(cmds) > budget {
		return pptx.GeomPath{}, false, false
	}
	return pptx.GeomPath{Cmds: cmds}, false, true
}
func toggle(cmd, abs, rel byte) byte {
	if cmd >= 'a' && cmd <= 'z' {
		return rel
	}
	return abs
}

type pathParser struct {
	s string
	i int
}

func (p *pathParser) skip() {
	for p.i < len(p.s) && (unicode.IsSpace(rune(p.s[p.i])) || p.s[p.i] == ',') {
		p.i++
	}
}
func (p *pathParser) more() bool { p.skip(); return p.i < len(p.s) }
func (p *pathParser) isCommand() bool {
	p.skip()
	if p.i >= len(p.s) {
		return false
	}
	c := p.s[p.i]
	return strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(c))
}
func (p *pathParser) nextCommand() byte { c := p.s[p.i]; p.i++; return c }
func (p *pathParser) hasNumber() bool {
	p.skip()
	if p.i >= len(p.s) {
		return false
	}
	c := p.s[p.i]
	return c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9')
}
func (p *pathParser) num() (float64, bool) {
	p.skip()
	start := p.i
	if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		p.i++
	}
	digits := false
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
		digits = true
	}
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			digits = true
		}
	}
	if !digits {
		return 0, false
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		j := p.i + 1
		if j < len(p.s) && (p.s[j] == '+' || p.s[j] == '-') {
			j++
		}
		k := j
		for k < len(p.s) && p.s[k] >= '0' && p.s[k] <= '9' {
			k++
		}
		if k > j {
			p.i = k
		}
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 64)
	return v, err == nil
}
func (p *pathParser) two() (float64, float64, bool) {
	x, ok := p.num()
	if !ok {
		return 0, 0, false
	}
	y, ok := p.num()
	return x, y, ok
}
func (p *pathParser) flag() (bool, bool) {
	p.skip()
	if p.i >= len(p.s) {
		return false, false
	}
	c := p.s[p.i]
	if c != '0' && c != '1' {
		return false, false
	}
	p.i++
	return c == '1', true
}

func rectPath(x, y, w, h, rx, ry float64, mapPt pathMapper) pptx.GeomPath {
	if w == 0 || h == 0 {
		return pptx.GeomPath{}
	}
	if rx > w/2 {
		rx = w / 2
	}
	if ry > h/2 {
		ry = h / 2
	}
	if rx <= 0 || ry <= 0 {
		return pptx.GeomPath{Cmds: []pptx.PathCmd{{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{mapPt(x, y)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x+w, y)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x+w, y+h)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x, y+h)}}, {Verb: pptx.ClosePath}}}
	}
	k := 0.5522847498307936
	cmds := []pptx.PathCmd{{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{mapPt(x+rx, y)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x+w-rx, y)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(x+w-rx+k*rx, y), mapPt(x+w, y+ry-k*ry), mapPt(x+w, y+ry)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x+w, y+h-ry)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(x+w, y+h-ry+k*ry), mapPt(x+w-rx+k*rx, y+h), mapPt(x+w-rx, y+h)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x+rx, y+h)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(x+rx-k*rx, y+h), mapPt(x, y+h-ry+k*ry), mapPt(x, y+h-ry)}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{mapPt(x, y+ry)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(x, y+ry-k*ry), mapPt(x+rx-k*rx, y), mapPt(x+rx, y)}}, {Verb: pptx.ClosePath}}
	return pptx.GeomPath{Cmds: cmds}
}
func ellipsePath(cx, cy, rx, ry float64, mapPt pathMapper) pptx.GeomPath {
	if rx == 0 || ry == 0 {
		return pptx.GeomPath{}
	}
	k := 0.5522847498307936
	return pptx.GeomPath{Cmds: []pptx.PathCmd{{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{mapPt(cx+rx, cy)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(cx+rx, cy+k*ry), mapPt(cx+k*rx, cy+ry), mapPt(cx, cy+ry)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(cx-k*rx, cy+ry), mapPt(cx-rx, cy+k*ry), mapPt(cx-rx, cy)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(cx-rx, cy-k*ry), mapPt(cx-k*rx, cy-ry), mapPt(cx, cy-ry)}}, {Verb: pptx.CubicTo, Pts: []pptx.PathPoint{mapPt(cx+k*rx, cy-ry), mapPt(cx+rx, cy-k*ry), mapPt(cx+rx, cy)}}, {Verb: pptx.ClosePath}}}
}

func arcToCubics(x1, y1, rx, ry, phi float64, large, sweep bool, x2, y2 float64) [][3]point {
	if x1 == x2 && y1 == y2 {
		// Identical endpoints: SVG omits the arc segment entirely (emitting a
		// zero-length cubic would show a dot under round/square caps).
		return nil
	}
	if rx == 0 || ry == 0 {
		return [][3]point{{{x1, y1}, {x2, y2}, {x2, y2}}}
	}
	rx = math.Abs(rx)
	ry = math.Abs(ry)
	ph := phi * math.Pi / 180
	cos, sin := math.Cos(ph), math.Sin(ph)
	dx, dy := (x1-x2)/2, (y1-y2)/2
	x1p := cos*dx + sin*dy
	y1p := -sin*dx + cos*dy
	lam := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lam > 1 {
		s := math.Sqrt(lam)
		rx *= s
		ry *= s
	}
	sign := 1.0
	if large == sweep {
		sign = -1
	}
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	coef := 0.0
	if den != 0 {
		coef = sign * math.Sqrt(math.Max(0, (rx*rx*ry*ry-den)/den))
	}
	cxp := coef * rx * y1p / ry
	cyp := -coef * ry * x1p / rx
	cx := cos*cxp - sin*cyp + (x1+x2)/2
	cy := sin*cxp + cos*cyp + (y1+y2)/2
	v1 := point{(x1p - cxp) / rx, (y1p - cyp) / ry}
	v2 := point{(-x1p - cxp) / rx, (-y1p - cyp) / ry}
	theta := math.Atan2(v1.y, v1.x)
	delta := angleBetween(v1, v2)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	segs := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	out := make([][3]point, 0, segs)
	for i := 0; i < segs; i++ {
		a1 := theta + delta*float64(i)/float64(segs)
		a2 := theta + delta*float64(i+1)/float64(segs)
		out = append(out, arcSeg(cx, cy, rx, ry, ph, a1, a2))
	}
	return out
}
func angleBetween(u, v point) float64 { return math.Atan2(u.x*v.y-u.y*v.x, u.x*v.x+u.y*v.y) }
func arcSeg(cx, cy, rx, ry, phi, a1, a2 float64) [3]point {
	t := 4.0 / 3.0 * math.Tan((a2-a1)/4)
	p1 := point{math.Cos(a1) - t*math.Sin(a1), math.Sin(a1) + t*math.Cos(a1)}
	p2 := point{math.Cos(a2) + t*math.Sin(a2), math.Sin(a2) - t*math.Cos(a2)}
	p3 := point{math.Cos(a2), math.Sin(a2)}
	cos, sin := math.Cos(phi), math.Sin(phi)
	tr := func(p point) point {
		// Scale the unit-circle point by the radii first, then rotate by phi.
		sx, sy := rx*p.x, ry*p.y
		return point{cx + cos*sx - sin*sy, cy + sin*sx + cos*sy}
	}
	return [3]point{tr(p1), tr(p2), tr(p3)}
}
