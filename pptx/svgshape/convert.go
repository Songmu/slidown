package svgshape

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/Songmu/slidown/pptx"
)

const emuPerUnit = 9525.0

// Conversion resource limits guard against pathological or malicious SVGs
// (deeply nested groups, exponential <use> fan-out) exhausting stack/memory.
const (
	maxDepth  = 1000
	maxShapes = 100000
	// maxCommands bounds the total number of path commands across all shapes,
	// so a single huge <path> can't drive unbounded allocations or slide XML.
	maxCommands = 2000000
	// maxVisits bounds the total number of walk/use expansions so a small SVG
	// with a diamond/exponential <use> graph (whose nodes emit no shapes) can't
	// consume unbounded CPU.
	maxVisits = 1000000
	// maxGradientDepth bounds the gradient href reference chain so a long chain
	// can't exhaust the Go stack during recursive resolution.
	maxGradientDepth = 1000
	// maxCSSRules / maxCSSSelectors bound stylesheet size so matchedCSS (scanned
	// per element) can't cause CPU/memory exhaustion.
	maxCSSRules     = 10000
	maxCSSSelectors = 20000
	// maxCSSWork bounds the total selector-matching comparisons (rules/selectors
	// scanned per element) so a stylesheet crossed with many elements can't
	// exhaust CPU.
	maxCSSWork = 20000000
	// maxInputBytes caps the raw SVG size so pathological inputs are rejected
	// before any parsing/allocation work.
	maxInputBytes = 20 << 20 // 20 MiB
)

// errTooComplex is returned when parsing exceeds the resource limits above.
var errTooComplex = errors.New("svg too complex")

type node struct {
	Name string
	// rawName is the element's original (case-preserving) local name, used for
	// case-sensitive CSS type-selector matching (SVG/XML is case-sensitive).
	rawName  string
	Attrs    map[string]string
	Children []*node
	Text     string
	// textB accumulates character data during parsing to keep large/split text
	// nodes linear; it is materialized into Text on the element's end tag.
	textB *strings.Builder
	// foreign marks an element in a non-SVG namespace, which is not rendered.
	foreign bool
	// textAfterChild is set when character data appears after a child element
	// within this node (mixed content like <text>A<tspan/>C</text>), which the
	// parser flattens into Text and Children separately, losing order.
	textAfterChild bool
}

const (
	svgNS   = "http://www.w3.org/2000/svg"
	xlinkNS = "http://www.w3.org/1999/xlink"
	xmlNS   = "http://www.w3.org/XML/1998/namespace"
)

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
	cmdCount     int
	depth        int
	visits       int
	cssWork      int
	cssSelectors int
	resolvingUse map[string]bool
}

// Convert parses svg and returns a native pptx group when the whole document is faithfully converted.
// Unsupported SVG features return ok=false so callers can fall back to embedding the SVG as an image.
func Convert(svg []byte) (g *pptx.GroupShape, ok bool) {
	if len(svg) > maxInputBytes {
		return nil, false
	}
	r, err := parseXML(svg)
	if err != nil || r == nil || r.Name != "svg" || r.foreign {
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
	dec := xml.NewDecoder(bytes.NewReader(data))
	var stack []*node
	var root *node
	nodeCount := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return root, nil
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// Bound the tree so pathological inputs (deeply nested or huge
			// element counts) cannot allocate an enormous tree before Convert
			// aborts.
			if len(stack) >= maxDepth {
				return nil, errTooComplex
			}
			nodeCount++
			if nodeCount > maxShapes {
				return nil, errTooComplex
			}
			n := &node{Name: strings.ToLower(t.Name.Local), rawName: t.Name.Local, Attrs: map[string]string{}}
			// Elements in a foreign namespace are not SVG content and are not
			// rendered; mark them so the walk skips their subtree.
			n.foreign = t.Name.Space != "" && t.Name.Space != svgNS
			for _, a := range t.Attr {
				key := strings.ToLower(a.Name.Local)
				switch {
				case a.Name.Space == "" || a.Name.Space == svgNS:
					n.Attrs[key] = strings.TrimSpace(a.Value)
				case a.Name.Space == xlinkNS && key == "href":
					n.Attrs["xlink:href"] = strings.TrimSpace(a.Value)
				case a.Name.Space == xmlNS && key == "space":
					n.Attrs["xml:space"] = strings.TrimSpace(a.Value)
				default:
					// Foreign-namespaced attribute (e.g. an extension); ignore
					// rather than misinterpret it as an SVG presentation attr.
				}
			}
			if len(stack) == 0 {
				root = n
			} else {
				stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				n := stack[len(stack)-1]
				if n.textB != nil {
					n.Text = n.textB.String()
					n.textB = nil
				}
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				// Any character data after a child element is order-significant
				// in mixed content (including a whitespace separator between
				// tspans); flag it so text conversion falls back rather than
				// reordering.
				if len(top.Children) > 0 {
					top.textAfterChild = true
				}
				// Accumulate in a builder so a text node split into many small
				// CharData tokens (e.g. by comments/CDATA) stays linear rather
				// than quadratic.
				if top.textB == nil {
					top.textB = &strings.Builder{}
				}
				top.textB.Write(t)
			}
		case xml.ProcInst:
			// An xml-stylesheet PI can restyle the document; we can't evaluate
			// it, so fall back rather than convert with different output.
			if strings.EqualFold(t.Target, "xml-stylesheet") {
				return nil, errTooComplex
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
		// A viewBox combined with a root width/height of a different aspect
		// ratio (or a non-default preserveAspectRatio) needs the SVG viewport
		// transform / letterboxing, which isn't modeled; fall back so the
		// native SVG picture renders it faithfully.
		if pa := strings.TrimSpace(c.root.Attrs["preserveaspectratio"]); pa != "" &&
			!strings.EqualFold(pa, "xMidYMid meet") && !strings.EqualFold(pa, "xMidYMid") {
			return false
		}
		w, okw := parseLength(c.root.Attrs["width"], false)
		h, okh := parseLength(c.root.Attrs["height"], false)
		if okw && okh && w > 0 && h > 0 {
			// Compare aspect ratios (w/h vs vbW/vbH) with cross-multiplication.
			if math.Abs(w*c.vbH-h*c.vbW) > 1e-6*w*h {
				return false
			}
		}
	} else {
		// No viewBox: fall back per-dimension to the SVG spec defaults
		// (300x150) so a valid width/height isn't discarded when only the other
		// is missing/invalid. This matches Image.Dimensions()'s handling.
		w, okw := parseLength(c.root.Attrs["width"], false)
		h, okh := parseLength(c.root.Attrs["height"], false)
		if !okw || w <= 0 {
			w = 300
		}
		if !okh || h <= 0 {
			h = 150
		}
		c.vbW, c.vbH = w, h
	}
	c.chW, c.chH = round(c.vbW*emuPerUnit), round(c.vbH*emuPerUnit)
	return c.chW > 0 && c.chH > 0
}

func (c *conv) collectDefsAndCSS(n *node) bool {
	if n != c.root && n.foreign {
		// Foreign-namespace subtrees aren't SVG; don't interpret their <style>,
		// ids, or element names as SVG content.
		return true
	}
	if n != c.root && isFallbackElement(n.Name) {
		return false
	}
	if n.Name == "style" {
		// Only unconditional CSS stylesheets are applied. A non-CSS type or a
		// media query the converter can't evaluate would be applied
		// unconditionally here, so fall back instead.
		if ty := strings.TrimSpace(strings.ToLower(n.Attrs["type"])); ty != "" && ty != "text/css" {
			return false
		}
		if md := strings.TrimSpace(strings.ToLower(n.Attrs["media"])); md != "" && md != "all" && md != "screen" {
			return false
		}
		rules, ok := parseCSS(n.Text)
		if !ok {
			return false
		}
		// Bound the cumulative rule/selector counts across all <style> elements
		// (parseCSS's caps are per-stylesheet) so many small stylesheets can't
		// grow c.css without bound.
		sels := 0
		for _, r := range rules {
			sels += len(r.sels)
		}
		if len(c.css)+len(rules) > maxCSSRules || c.cssSelectors+sels > maxCSSSelectors {
			return false
		}
		c.cssSelectors += sels
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

// displayNone reports whether the resolved style sets display:none, which
// removes the element and its whole subtree.
func displayNone(st style) bool {
	return strings.EqualFold(strings.TrimSpace(st.get("display")), "none")
}

// visibilityHidden reports whether the resolved style hides the element via
// visibility:hidden/collapse. Unlike display:none, a descendant can override it
// with visibility:visible, so this is only applied per painted leaf.
func visibilityHidden(st style) bool {
	v := strings.ToLower(strings.TrimSpace(st.get("visibility")))
	return v == "hidden" || v == "collapse"
}

// containerOpacity reports the element's own opacity (0..1) when it is a
// container whose opacity would need group compositing that DrawingML custom
// geometry can't reproduce.
func containerOpacity(st style) (float64, bool) {
	op, ok := parseUnit(st.get("opacity"), 1)
	if !ok {
		return 0, false
	}
	return op, true
}

// tightenGradientBounds rebases a gradient-filled shape onto its own bounding
// box so a DrawingML gradient (which spans the shape's transform rectangle)
// matches SVG's objectBoundingBox extent. It shifts every path point so the
// bbox origin is 0,0 and sets the shape/path extent to the bbox size. Returns
// false when the bbox is degenerate.
func tightenGradientBounds(gs *pptx.GeomShape) bool {
	minX, minY, maxX, maxY, ok := geomPathBounds(gs)
	if !ok {
		return false
	}
	w, h := maxX-minX, maxY-minY
	if w <= 0 || h <= 0 {
		return false
	}
	for _, p := range gs.Paths {
		for _, cmd := range p.Cmds {
			for i := range cmd.Pts {
				cmd.Pts[i].X -= minX
				cmd.Pts[i].Y -= minY
			}
		}
	}
	gs.X, gs.Y, gs.W, gs.H = minX, minY, w, h
	gs.PathW, gs.PathH = w, h
	return true
}

// geomPathBounds returns the tight bounding box of a path in EMU, evaluating
// Bezier extrema (not just control points, which can lie outside the curve).
func geomPathBounds(gs *pptx.GeomShape) (minX, minY, maxX, maxY int64, ok bool) {
	first := true
	var cur, start pptx.PathPoint
	acc := func(p pptx.PathPoint) {
		if first {
			minX, minY, maxX, maxY = p.X, p.Y, p.X, p.Y
			first = false
			return
		}
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	accF := func(x, y float64) { acc(pptx.PathPoint{X: int64(math.Round(x)), Y: int64(math.Round(y))}) }
	for _, p := range gs.Paths {
		for _, cmd := range p.Cmds {
			switch cmd.Verb {
			case pptx.MoveTo:
				if len(cmd.Pts) >= 1 {
					cur = cmd.Pts[0]
					start = cur
					acc(cur)
				}
			case pptx.LineTo:
				if len(cmd.Pts) >= 1 {
					cur = cmd.Pts[0]
					acc(cur)
				}
			case pptx.ClosePath:
				// A subsequent segment resumes from the subpath's start point.
				cur = start
			case pptx.CubicTo:
				if len(cmd.Pts) >= 3 {
					p0, p1, p2, p3 := cur, cmd.Pts[0], cmd.Pts[1], cmd.Pts[2]
					acc(p3)
					for _, t := range cubicExtrema(float64(p0.X), float64(p1.X), float64(p2.X), float64(p3.X)) {
						accF(cubicAt(float64(p0.X), float64(p1.X), float64(p2.X), float64(p3.X), t), cubicAt(float64(p0.Y), float64(p1.Y), float64(p2.Y), float64(p3.Y), t))
					}
					for _, t := range cubicExtrema(float64(p0.Y), float64(p1.Y), float64(p2.Y), float64(p3.Y)) {
						accF(cubicAt(float64(p0.X), float64(p1.X), float64(p2.X), float64(p3.X), t), cubicAt(float64(p0.Y), float64(p1.Y), float64(p2.Y), float64(p3.Y), t))
					}
					cur = p3
				}
			case pptx.QuadTo:
				if len(cmd.Pts) >= 2 {
					p0, p1, p2 := cur, cmd.Pts[0], cmd.Pts[1]
					acc(p2)
					for _, t := range quadExtrema(float64(p0.X), float64(p1.X), float64(p2.X)) {
						accF(quadAt(float64(p0.X), float64(p1.X), float64(p2.X), t), quadAt(float64(p0.Y), float64(p1.Y), float64(p2.Y), t))
					}
					for _, t := range quadExtrema(float64(p0.Y), float64(p1.Y), float64(p2.Y)) {
						accF(quadAt(float64(p0.X), float64(p1.X), float64(p2.X), t), quadAt(float64(p0.Y), float64(p1.Y), float64(p2.Y), t))
					}
					cur = p2
				}
			}
		}
	}
	return minX, minY, maxX, maxY, !first
}

func cubicAt(p0, p1, p2, p3, t float64) float64 {
	u := 1 - t
	return u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
}

func cubicExtrema(p0, p1, p2, p3 float64) []float64 {
	// B'(t) = 3[(p1-p0)(1-t)^2 + 2(p2-p1)(1-t)t + (p3-p2)t^2]; solve a t^2+b t+c=0.
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 2 * (p0 - 2*p1 + p2)
	cc := p1 - p0
	var out []float64
	if math.Abs(a) < 1e-12 {
		if math.Abs(b) > 1e-12 {
			out = appendRoot(out, -cc/b)
		}
		return out
	}
	disc := b*b - 4*a*cc
	if disc < 0 {
		return out
	}
	sq := math.Sqrt(disc)
	out = appendRoot(out, (-b+sq)/(2*a))
	out = appendRoot(out, (-b-sq)/(2*a))
	return out
}

func quadAt(p0, p1, p2, t float64) float64 {
	u := 1 - t
	return u*u*p0 + 2*u*t*p1 + t*t*p2
}

func quadExtrema(p0, p1, p2 float64) []float64 {
	den := p0 - 2*p1 + p2
	if math.Abs(den) < 1e-12 {
		return nil
	}
	return appendRoot(nil, (p0-p1)/den)
}

func appendRoot(out []float64, t float64) []float64 {
	if t > 0 && t < 1 {
		return append(out, t)
	}
	return out
}

func (c *conv) walk(n *node, inherited style, m matrix, g *pptx.GroupShape, root bool) bool {
	c.depth++
	defer func() { c.depth-- }()
	c.visits++
	if c.depth > maxDepth || c.geomCount+c.textCount > maxShapes || c.visits > maxVisits {
		return false
	}
	if !root && isFallbackElement(n.Name) {
		return false
	}
	if !root && n.foreign {
		// Foreign-namespace content is not rendered by SVG.
		return true
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
	if displayNone(st) {
		// display:none removes the element and its whole subtree (including the
		// root <svg>, which then renders nothing).
		return true
	}
	if tr := n.Attrs["transform"]; tr != "" {
		tm, ok := parseTransform(tr)
		if !ok {
			return false
		}
		m = m.mul(tm)
	}

	switch n.Name {
	case "svg", "g":
		// A nested <svg> establishes its own viewport (x/y/width/height/viewBox/
		// preserveAspectRatio) that isn't modeled; only the root is handled.
		if n.Name == "svg" && !root {
			return false
		}
		// A container with opacity < 1 requires group compositing (its content
		// is flattened and blended as a whole), which flat custom-geometry
		// shapes can't reproduce; fall back to the native SVG picture.
		if op, ok := containerOpacity(st); !ok {
			return false
		} else if op < 1 {
			return false
		}
		// Keep walking visibility-hidden containers: a descendant may override
		// with visibility:visible.
		for _, ch := range n.Children {
			if !c.walk(ch, st, m, g, false) {
				return false
			}
		}
		return true
	case "symbol":
		// A <symbol> renders only when instantiated by <use>; encountered
		// directly in the tree it draws nothing.
		return true
	case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
		gp, forceFillNone, ok := c.geometry(n, m)
		if !ok {
			return false
		}
		if len(gp.Cmds) == 0 {
			return true
		}
		c.cmdCount += len(gp.Cmds)
		if c.cmdCount > maxCommands {
			return false
		}
		if visibilityHidden(st) {
			return true
		}
		fill, stroke, ok := c.paint(st, m, forceFillNone)
		if !ok {
			return false
		}
		// DrawingML custom geometry fills always use nonzero winding, so any
		// filled evenodd path (holes, or a self-intersecting subpath such as a
		// star) can mis-render; fall back to embedding the SVG as an image.
		evenOdd := strings.EqualFold(st.get("fill-rule"), "evenodd")
		if evenOdd && fill.Kind != pptx.FillNone {
			return false
		}
		// A DrawingML gradient spans the shape's own transform rectangle, so a
		// gradient-filled shape must use tight element bounds (not the whole
		// viewBox) to match SVG objectBoundingBox, and its direction can't be
		// rotated/skewed; fall back for rotation/skew.
		gs := &pptx.GeomShape{Name: shapeName(n, "Shape", c.geomCount+1), X: 0, Y: 0, W: c.chW, H: c.chH, PathW: c.chW, PathH: c.chH, Paths: []pptx.GeomPath{gp}, Fill: fill, Stroke: stroke, EvenOdd: evenOdd}
		// SVG clips painting to the root viewport by default, but a PowerPoint
		// group does not clip its children. Fall back when a shape's geometry
		// extends beyond the viewBox, and separately when a stroke would paint
		// substantially outside it. A thin stroke may overhang the edge by up to
		// ~2 user units (common for border shapes); a large stroke that would
		// paint far outside falls back so it isn't visible beyond the image.
		if bx0, by0, bx1, by1, ok := geomPathBounds(gs); ok {
			const tol = 16 // EMU, absorbs rounding of edge-aligned content
			if bx0 < -tol || by0 < -tol || bx1 > c.chW+tol || by1 > c.chH+tol {
				return false
			}
			if stroke != nil {
				se := stroke.Width / 2
				if stroke.Join == "miter" {
					// A miter join can extend up to miterlimit*half-width.
					se = stroke.Width * 2
				}
				maxOverhang := int64(2 * emuPerUnit)
				if bx0-se < -maxOverhang || by0-se < -maxOverhang ||
					bx1+se > c.chW+maxOverhang || by1+se > c.chH+maxOverhang {
					return false
				}
			}
		}
		if fill.Kind == pptx.FillGradient {
			// <a:lin> only expresses a direction over the shape's box, so a
			// rotation, skew or axis reflection baked into the geometry can't be
			// reflected in the gradient; fall back for those.
			if m.b != 0 || m.c != 0 || m.a < 0 || m.d < 0 {
				return false
			}
			if !tightenGradientBounds(gs) {
				return false
			}
		}
		c.geomCount++
		g.Geoms = append(g.Geoms, gs)
		g.Children = append(g.Children, pptx.GroupChild{Geom: gs})
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
		return parsePath(n.Attrs["d"], maxCommands-c.cmdCount, func(x, y float64) pptx.PathPoint { return c.point(m.apply(point{x, y})) })
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
		if len(pts) < 2 {
			// Fewer than two points can't form a line/polygon; emit no
			// geometry (rather than a MoveTo-less ClosePath) so the shape is
			// skipped as empty.
			return pptx.GeomPath{}, false, true
		}
		if len(pts) > maxCommands-c.cmdCount {
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
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
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

// allowedAttrs is the set of attribute local-names the converter understands
// (structural geometry + supported presentation attributes). Any other
// rendering-affecting attribute triggers the whole-SVG fallback rather than
// being silently ignored (e.g. vector-effect, font-weight, letter-spacing).
var allowedAttrs = map[string]bool{
	// structural / geometry
	"id": true, "class": true, "style": true, "transform": true,
	"x": true, "y": true, "width": true, "height": true,
	"cx": true, "cy": true, "r": true, "rx": true, "ry": true,
	"d": true, "points": true, "x1": true, "y1": true, "x2": true, "y2": true,
	"href": true, "xlink:href": true,
	"viewbox": true, "preserveaspectratio": true, "version": true,
	"baseprofile": true, "space": true, "lang": true, "overflow": true,
	"role": true, "focusable": true, "tabindex": true, "xml:space": true,
	// gradients
	"offset": true, "gradientunits": true, "gradienttransform": true,
	"spreadmethod": true, "fx": true, "fy": true, "fr": true,
}

// hasUnsupportedAttrs reports whether an element carries any attribute the
// converter can't faithfully honor. Supported style properties (inheritedProps,
// stop-color/stop-opacity) and structural attributes are allowed; xmlns/xml:*,
// aria-*, data-* and event handlers are harmless and ignored; everything else
// forces a fallback.
func hasUnsupportedAttrs(n *node) bool {
	for k := range n.Attrs {
		if allowedAttrs[k] || inheritedProps[k] || k == "stop-color" || k == "stop-opacity" {
			continue
		}
		if strings.HasPrefix(k, "xmlns") || strings.HasPrefix(k, "xml:") ||
			strings.HasPrefix(k, "aria-") || strings.HasPrefix(k, "data-") ||
			strings.HasPrefix(k, "on") {
			continue
		}
		return true
	}
	return false
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
