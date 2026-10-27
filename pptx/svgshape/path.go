package svgshape

import (
	"math"
	"strings"

	"github.com/Songmu/slidown/pptx"
)

type pathMapper func(x, y float64) pptx.PathPoint

type pathState struct {
	parser             *pathParser
	cmds               []pptx.PathCmd
	cmd                byte
	cur, start         point
	lastC, lastQ       point
	lastWasC, lastWasQ bool
	started            bool
	budget             int
	mapPt              pathMapper
}

func parsePath(d string, budget int, mapPt pathMapper) (pptx.GeomPath, bool, bool) {
	s := &pathState{parser: &pathParser{s: d}, budget: budget, mapPt: mapPt}
	if !s.parse() {
		return pptx.GeomPath{}, false, false
	}
	return pptx.GeomPath{Cmds: s.cmds}, false, true
}

func (s *pathState) parse() bool {
	for s.parser.more() {
		if len(s.cmds) > s.budget {
			return false
		}
		startIdx := s.parser.i
		consumedCmd := false
		if s.parser.isCommand() {
			s.cmd = s.parser.nextCommand()
			consumedCmd = true
			// A valid path must begin with a moveto command.
			if !s.started && s.cmd != 'M' && s.cmd != 'm' {
				return false
			}
			s.started = true
			// Every command except close must be followed by operands; an
			// explicit command with no number is a malformed path.
			if s.cmd != 'Z' && s.cmd != 'z' && !s.parser.hasNumber() {
				return false
			}
		} else if s.cmd == 0 {
			return false
		}
		if !s.parseCommand() {
			return false
		}

		// Guard against non-progressing iterations (e.g. a persisting command
		// such as "Z" followed by stray tokens, or an operand-less command that
		// consumed nothing): without progress the loop would spin forever.
		if !consumedCmd && s.parser.i == startIdx {
			return false
		}
	}
	return len(s.cmds) <= s.budget
}

func (s *pathState) parseCommand() bool {
	switch s.cmd {
	case 'M', 'm':
		return s.moveTo()
	case 'L', 'l':
		return s.lineTo()
	case 'H', 'h':
		return s.horizontalTo()
	case 'V', 'v':
		return s.verticalTo()
	case 'C', 'c':
		return s.cubicTo()
	case 'S', 's':
		return s.smoothCubicTo()
	case 'Q', 'q':
		return s.quadraticTo()
	case 'T', 't':
		return s.smoothQuadraticTo()
	case 'A', 'a':
		return s.arcTo()
	case 'Z', 'z':
		return s.closePath()
	default:
		return false
	}
}

func (s *pathState) moveTo() bool {
	x, y, ok := s.parser.two()
	if !ok {
		return false
	}
	if s.cmd == 'm' {
		x += s.cur.x
		y += s.cur.y
	}
	s.cur = point{x, y}
	s.start = s.cur
	s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{s.mapPt(s.cur.x, s.cur.y)}})
	s.cmd = toggle(s.cmd, 'L', 'l')
	s.lastWasC, s.lastWasQ = false, false
	return true
}

func (s *pathState) lineTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		x, y, ok := s.parser.two()
		if !ok {
			return false
		}
		if s.cmd == 'l' {
			x += s.cur.x
			y += s.cur.y
		}
		s.cur = point{x, y}
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.LineTo, Pts: []pptx.PathPoint{s.mapPt(s.cur.x, s.cur.y)}})
	}
	s.lastWasC, s.lastWasQ = false, false
	return true
}

func (s *pathState) horizontalTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		x, ok := s.parser.num()
		if !ok {
			return false
		}
		if s.cmd == 'h' {
			x += s.cur.x
		}
		s.cur.x = x
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.LineTo, Pts: []pptx.PathPoint{s.mapPt(s.cur.x, s.cur.y)}})
	}
	s.lastWasC, s.lastWasQ = false, false
	return true
}

func (s *pathState) verticalTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		y, ok := s.parser.num()
		if !ok {
			return false
		}
		if s.cmd == 'v' {
			y += s.cur.y
		}
		s.cur.y = y
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.LineTo, Pts: []pptx.PathPoint{s.mapPt(s.cur.x, s.cur.y)}})
	}
	s.lastWasC, s.lastWasQ = false, false
	return true
}

func (s *pathState) cubicTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		x1, y1, ok := s.parser.two()
		if !ok {
			return false
		}
		x2, y2, ok := s.parser.two()
		if !ok {
			return false
		}
		x, y, ok := s.parser.two()
		if !ok {
			return false
		}
		if s.cmd == 'c' {
			x1 += s.cur.x
			y1 += s.cur.y
			x2 += s.cur.x
			y2 += s.cur.y
			x += s.cur.x
			y += s.cur.y
		}
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{s.mapPt(x1, y1), s.mapPt(x2, y2), s.mapPt(x, y)}})
		s.cur = point{x, y}
		s.lastC = point{x2, y2}
		s.lastWasC = true
		s.lastWasQ = false
	}
	return true
}

func (s *pathState) smoothCubicTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		c1 := s.cur
		if s.lastWasC {
			c1 = point{2*s.cur.x - s.lastC.x, 2*s.cur.y - s.lastC.y}
		}
		x2, y2, ok := s.parser.two()
		if !ok {
			return false
		}
		x, y, ok := s.parser.two()
		if !ok {
			return false
		}
		if s.cmd == 's' {
			x2 += s.cur.x
			y2 += s.cur.y
			x += s.cur.x
			y += s.cur.y
		}
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{s.mapPt(c1.x, c1.y), s.mapPt(x2, y2), s.mapPt(x, y)}})
		s.cur = point{x, y}
		s.lastC = point{x2, y2}
		s.lastWasC = true
		s.lastWasQ = false
	}
	return true
}

func (s *pathState) quadraticTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		x1, y1, ok := s.parser.two()
		if !ok {
			return false
		}
		x, y, ok := s.parser.two()
		if !ok {
			return false
		}
		if s.cmd == 'q' {
			x1 += s.cur.x
			y1 += s.cur.y
			x += s.cur.x
			y += s.cur.y
		}
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{s.mapPt(x1, y1), s.mapPt(x, y)}})
		s.cur = point{x, y}
		s.lastQ = point{x1, y1}
		s.lastWasQ = true
		s.lastWasC = false
	}
	return true
}

func (s *pathState) smoothQuadraticTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		c := s.cur
		if s.lastWasQ {
			c = point{2*s.cur.x - s.lastQ.x, 2*s.cur.y - s.lastQ.y}
		}
		x, y, ok := s.parser.two()
		if !ok {
			return false
		}
		if s.cmd == 't' {
			x += s.cur.x
			y += s.cur.y
		}
		s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.QuadTo, Pts: []pptx.PathPoint{s.mapPt(c.x, c.y), s.mapPt(x, y)}})
		s.cur = point{x, y}
		s.lastQ = c
		s.lastWasQ = true
		s.lastWasC = false
	}
	return true
}

func (s *pathState) arcTo() bool {
	for s.parser.hasNumber() && len(s.cmds) <= s.budget {
		rx, ok := s.parser.num()
		if !ok {
			return false
		}
		ry, ok := s.parser.num()
		if !ok {
			return false
		}
		rot, ok := s.parser.num()
		if !ok {
			return false
		}
		laf, ok := s.parser.flag()
		if !ok {
			return false
		}
		sf, ok := s.parser.flag()
		if !ok {
			return false
		}
		x, y, ok := s.parser.two()
		if !ok {
			return false
		}
		if s.cmd == 'a' {
			x += s.cur.x
			y += s.cur.y
		}
		cubs := arcToCubics(s.cur.x, s.cur.y, rx, ry, rot, laf, sf, x, y)
		for _, cb := range cubs {
			s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.CubicTo, Pts: []pptx.PathPoint{s.mapPt(cb[0].x, cb[0].y), s.mapPt(cb[1].x, cb[1].y), s.mapPt(cb[2].x, cb[2].y)}})
		}
		s.cur = point{x, y}
		s.lastWasC, s.lastWasQ = false, false
	}
	return true
}

func (s *pathState) closePath() bool {
	s.cmds = append(s.cmds, pptx.PathCmd{Verb: pptx.ClosePath})
	s.cur = s.start
	s.lastWasC, s.lastWasQ = false, false
	return true
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
	p.i = skipNumberSeparators(p.s, p.i)
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
	v, end, ok := scanNumber(p.s, p.i)
	p.i = end
	return v, ok
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
