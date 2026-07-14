package svgshape

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/Songmu/slidown/pptx"
)

const emuPerUnit = 9525.0

type node struct {
	Name     string
	Attrs    map[string]string
	Children []*node
	Text     string
}

type conv struct {
	root         *node
	css          []cssRule
	defs         map[string]*node
	gradients    map[string]*pptx.Gradient
	vbMinX       float64
	vbMinY       float64
	vbW          float64
	vbH          float64
	chW          int64
	chH          int64
	geomCount    int
	textCount    int
	resolvingUse map[string]bool
}

// Convert parses svg and returns a native pptx group when the whole document is faithfully converted.
// Unsupported SVG features return ok=false so callers can fall back to embedding the SVG as an image.
func Convert(svg []byte) (g *pptx.GroupShape, ok bool) {
	r, err := parseXML(svg)
	if err != nil || r == nil || r.Name != "svg" {
		return nil, false
	}
	c := &conv{root: r, defs: map[string]*node{}, gradients: map[string]*pptx.Gradient{}, resolvingUse: map[string]bool{}}
	if !c.initViewport() || !c.collectDefsAndCSS(r) || !c.buildGradients() {
		return nil, false
	}
	g = &pptx.GroupShape{Name: "SVG", X: 0, Y: 0, W: c.chW, H: c.chH, ChX: 0, ChY: 0, ChW: c.chW, ChH: c.chH}
	st := defaultStyle()
	if !c.walk(r, st, identity(), g, true) {
		return nil, false
	}
	return g, true
}

func parseXML(data []byte) (*node, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	var stack []*node
	var root *node
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				return root, nil
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{Name: strings.ToLower(t.Name.Local), Attrs: map[string]string{}}
			for _, a := range t.Attr {
				key := strings.ToLower(a.Name.Local)
				if a.Name.Space != "" && key == "href" {
					key = strings.ToLower(a.Name.Space) + ":href"
				}
				n.Attrs[key] = strings.TrimSpace(a.Value)
			}
			if len(stack) == 0 {
				root = n
			} else {
				stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string([]byte(t))
			}
		}
	}
}

func (c *conv) initViewport() bool {
	if vb := c.root.Attrs["viewbox"]; vb != "" {
		n, ok := parseNumberList(vb)
		if !ok || len(n) != 4 || n[2] <= 0 || n[3] <= 0 {
			return false
		}
		c.vbMinX, c.vbMinY, c.vbW, c.vbH = n[0], n[1], n[2], n[3]
	} else {
		w, okw := parseLength(c.root.Attrs["width"], false)
		h, okh := parseLength(c.root.Attrs["height"], false)
		if !okw || !okh {
			w, h = 300, 150
		}
		if w <= 0 || h <= 0 {
			return false
		}
		c.vbW, c.vbH = w, h
	}
	c.chW, c.chH = round(c.vbW*emuPerUnit), round(c.vbH*emuPerUnit)
	return c.chW > 0 && c.chH > 0
}

func (c *conv) collectDefsAndCSS(n *node) bool {
	if n != c.root && isFallbackElement(n.Name) {
		return false
	}
	if n.Name == "style" {
		rules, ok := parseCSS(n.Text)
		if !ok {
			return false
		}
		c.css = append(c.css, rules...)
	}
	if id := n.Attrs["id"]; id != "" {
		c.defs[id] = n
	}
	for _, ch := range n.Children {
		if !c.collectDefsAndCSS(ch) {
			return false
		}
	}
	return true
}

func (c *conv) walk(n *node, inherited style, m matrix, g *pptx.GroupShape, root bool) bool {
	if !root && isFallbackElement(n.Name) {
		return false
	}
	if !root && n.Name == "style" {
		return true
	}
	if !root && (n.Name == "title" || n.Name == "desc" || n.Name == "metadata") {
		return true
	}
	if !root && n.Name == "defs" {
		return true
	}
	if hasUnsupportedAttrs(n) {
		return false
	}

	st, ok := c.resolveStyle(n, inherited)
	if !ok {
		return false
	}
	if tr := n.Attrs["transform"]; tr != "" {
		tm, ok := parseTransform(tr)
		if !ok {
			return false
		}
		m = m.mul(tm)
	}

	switch n.Name {
	case "svg", "g", "symbol":
		for _, ch := range n.Children {
			if !c.walk(ch, st, m, g, false) {
				return false
			}
		}
		return true
	case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
		gp, forceFillNone, ok := c.geometry(n, m)
		if !ok {
			return false
		}
		if len(gp.Cmds) == 0 {
			return true
		}
		fill, stroke, ok := c.paint(st, m, forceFillNone)
		if !ok {
			return false
		}
		c.geomCount++
		g.Geoms = append(g.Geoms, &pptx.GeomShape{Name: shapeName(n, "Shape", c.geomCount), X: 0, Y: 0, W: c.chW, H: c.chH, PathW: c.chW, PathH: c.chH, Paths: []pptx.GeomPath{gp}, Fill: fill, Stroke: stroke, EvenOdd: strings.EqualFold(st.get("fill-rule"), "evenodd")})
		return true
	case "use":
		return c.expandUse(n, st, m, g)
	case "text":
		return c.text(n, st, m, g)
	default:
		if root {
			return true
		}
		return false
	}
}

func (c *conv) geometry(n *node, m matrix) (pptx.GeomPath, bool, bool) {
	switch n.Name {
	case "path":
		return parsePath(n.Attrs["d"], func(x, y float64) pptx.PathPoint { return c.point(m.apply(point{x, y})) })
	case "rect":
		x, ok1 := attrLen(n, "x", 0)
		y, ok2 := attrLen(n, "y", 0)
		w, ok3 := attrLen(n, "width", 0)
		h, ok4 := attrLen(n, "height", 0)
		if !ok1 || !ok2 || !ok3 || !ok4 || w < 0 || h < 0 {
			return pptx.GeomPath{}, false, false
		}
		rx, ok5 := attrLen(n, "rx", 0)
		ry, ok6 := attrLen(n, "ry", 0)
		if !ok5 || !ok6 {
			return pptx.GeomPath{}, false, false
		}
		return rectPath(x, y, w, h, rx, ry, func(x, y float64) pptx.PathPoint { return c.point(m.apply(point{x, y})) }), false, true
	case "circle":
		cx, ok1 := attrLen(n, "cx", 0)
		cy, ok2 := attrLen(n, "cy", 0)
		r, ok3 := attrLen(n, "r", 0)
		if !ok1 || !ok2 || !ok3 || r < 0 {
			return pptx.GeomPath{}, false, false
		}
		return ellipsePath(cx, cy, r, r, func(x, y float64) pptx.PathPoint { return c.point(m.apply(point{x, y})) }), false, true
	case "ellipse":
		cx, ok1 := attrLen(n, "cx", 0)
		cy, ok2 := attrLen(n, "cy", 0)
		rx, ok3 := attrLen(n, "rx", 0)
		ry, ok4 := attrLen(n, "ry", 0)
		if !ok1 || !ok2 || !ok3 || !ok4 || rx < 0 || ry < 0 {
			return pptx.GeomPath{}, false, false
		}
		return ellipsePath(cx, cy, rx, ry, func(x, y float64) pptx.PathPoint { return c.point(m.apply(point{x, y})) }), false, true
	case "line":
		x1, ok1 := attrLen(n, "x1", 0)
		y1, ok2 := attrLen(n, "y1", 0)
		x2, ok3 := attrLen(n, "x2", 0)
		y2, ok4 := attrLen(n, "y2", 0)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return pptx.GeomPath{}, false, false
		}
		return pptx.GeomPath{Cmds: []pptx.PathCmd{{Verb: pptx.MoveTo, Pts: []pptx.PathPoint{c.point(m.apply(point{x1, y1}))}}, {Verb: pptx.LineTo, Pts: []pptx.PathPoint{c.point(m.apply(point{x2, y2}))}}}}, true, true
	case "polyline", "polygon":
		pts, ok := parsePoints(n.Attrs["points"])
		if !ok {
			return pptx.GeomPath{}, false, false
		}
		cmds := []pptx.PathCmd{}
		for i, p := range pts {
			verb := pptx.LineTo
			if i == 0 {
				verb = pptx.MoveTo
			}
			cmds = append(cmds, pptx.PathCmd{Verb: verb, Pts: []pptx.PathPoint{c.point(m.apply(p))}})
		}
		if n.Name == "polygon" {
			cmds = append(cmds, pptx.PathCmd{Verb: pptx.ClosePath})
		}
		return pptx.GeomPath{Cmds: cmds}, false, true
	}
	return pptx.GeomPath{}, false, false
}

func (c *conv) point(p point) pptx.PathPoint {
	return pptx.PathPoint{X: round((p.x - c.vbMinX) * emuPerUnit), Y: round((p.y - c.vbMinY) * emuPerUnit)}
}

func attrLen(n *node, name string, def float64) (float64, bool) {
	if v := n.Attrs[name]; v != "" {
		return parseLength(v, false)
	}
	return def, true
}
func parseLength(s string, allowPercent bool) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "%") {
		return 0, allowPercent
	}
	units := []string{"px", "pt", "pc", "mm", "cm", "in"}
	unit := ""
	for _, u := range units {
		if strings.HasSuffix(strings.ToLower(s), u) {
			unit = u
			s = strings.TrimSpace(s[:len(s)-len(u)])
			break
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	switch unit {
	case "pt":
		v *= 96.0 / 72.0
	case "pc":
		v *= 16
	case "mm":
		v *= 96.0 / 25.4
	case "cm":
		v *= 96.0 / 2.54
	case "in":
		v *= 96
	}
	return v, true
}
func round(v float64) int64 { return int64(math.Round(v)) }

func shapeName(n *node, prefix string, i int) string {
	if id := n.Attrs["id"]; id != "" {
		return id
	}
	return prefix + " " + strconv.Itoa(i)
}
func isFallbackElement(name string) bool {
	return name == "clippath" || name == "mask" || name == "pattern" || name == "filter" || name == "image" || name == "foreignobject" || name == "textpath" || name == "switch" || name == "set" || strings.HasPrefix(name, "animate")
}
func hasUnsupportedAttrs(n *node) bool {
	_, f := n.Attrs["filter"]
	if f {
		return true
	}
	if _, m := n.Attrs["marker"]; m {
		return true
	}
	return n.Attrs["marker-start"] != "" || n.Attrs["marker-mid"] != "" || n.Attrs["marker-end"] != ""
}

func parseNumberList(s string) ([]float64, bool) { return scanNumbers(s) }
func parsePoints(s string) ([]point, bool) {
	nums, ok := scanNumbers(s)
	if !ok || len(nums)%2 != 0 {
		return nil, false
	}
	pts := make([]point, 0, len(nums)/2)
	for i := 0; i < len(nums); i += 2 {
		pts = append(pts, point{nums[i], nums[i+1]})
	}
	return pts, true
}

func scanNumbers(s string) ([]float64, bool) {
	var out []float64
	i := 0
	for i < len(s) {
		for i < len(s) && (unicode.IsSpace(rune(s[i])) || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		if s[i] == '+' || s[i] == '-' {
			i++
		}
		dot := false
		for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
			if s[i] == '.' {
				if dot {
					break
				}
				dot = true
			}
			i++
		}
		if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
			j := i + 1
			if j < len(s) && (s[j] == '+' || s[j] == '-') {
				j++
			}
			k := j
			for k < len(s) && s[k] >= '0' && s[k] <= '9' {
				k++
			}
			if k > j {
				i = k
			}
		}
		if start == i {
			return nil, false
		}
		v, err := strconv.ParseFloat(s[start:i], 64)
		if err != nil {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}
