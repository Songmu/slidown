package svgshape

import (
	"math"
	"strconv"
	"strings"

	"github.com/Songmu/slidown/pptx"
)

func (c *conv) buildGradients() bool {
	for id, n := range c.defs {
		if n.Name == "lineargradient" || n.Name == "radialgradient" {
			if _, ok := c.gradient(id, map[string]bool{}); !ok {
				return false
			}
		}
	}
	return true
}

func (c *conv) gradient(id string, seen map[string]bool) (*pptx.Gradient, bool) {
	if g := c.gradients[id]; g != nil {
		return g, true
	}
	if seen[id] {
		return nil, false
	}
	// seen accumulates every id in the current reference chain, so its size
	// bounds an acyclic href chain that could otherwise recurse deeply enough to
	// exhaust the Go stack.
	if len(seen) > maxGradientDepth {
		return nil, false
	}
	seen[id] = true
	n := c.defs[id]
	if n == nil || (n.Name != "lineargradient" && n.Name != "radialgradient") {
		return nil, false
	}
	// Gradient definitions live under <defs> and never reach walk's attribute
	// validation, so check here that they carry no rendering-affecting
	// attributes the converter can't honor (e.g. color-interpolation).
	if hasUnsupportedAttrs(n) {
		return nil, false
	}
	if gt := n.Attrs["gradienttransform"]; gt != "" {
		return nil, false
	}
	if sp := n.Attrs["spreadmethod"]; sp != "" && strings.ToLower(sp) != "pad" {
		return nil, false
	}
	var parent *pptx.Gradient
	if n.Attrs["href"] != "" || n.Attrs["xlink:href"] != "" {
		// A present href that doesn't resolve to a local #id references an
		// external gradient we can't inherit from; fall back rather than
		// silently using default coordinates.
		href := hrefID(n)
		if href == "" {
			return nil, false
		}
		pg, ok := c.gradient(href, seen)
		if !ok {
			return nil, false
		}
		parent = pg
	}
	gr := &pptx.Gradient{Kind: pptx.GradientLinear}
	if n.Name == "radialgradient" {
		gr.Kind = pptx.GradientRadial
	}
	if parent != nil {
		gr.Kind = parent.Kind
		gr.Angle = parent.Angle
		gr.Stops = append([]pptx.GradientStop(nil), parent.Stops...)
	}
	if n.Name == "lineargradient" {
		gr.Kind = pptx.GradientLinear
		if gu := n.Attrs["gradientunits"]; gu != "" && strings.ToLower(gu) != "objectboundingbox" {
			return nil, false
		}
		// DrawingML's <a:lin> expresses only a direction and always spans the
		// whole bounding box, so only a full-box vector (endpoints on the 0/1
		// edges) is representable. Recompute the angle only when this gradient
		// declares its own vector; otherwise keep the inherited (parent) angle.
		hasCoord := n.Attrs["x1"] != "" || n.Attrs["y1"] != "" || n.Attrs["x2"] != "" || n.Attrs["y2"] != ""
		if hasCoord && parent != nil {
			// A partial coordinate override must inherit the unspecified
			// endpoints from the referenced gradient; that merge isn't modeled,
			// so fall back conservatively.
			return nil, false
		}
		if hasCoord || parent == nil {
			coords := map[string]float64{"x1": 0, "y1": 0, "x2": 1, "y2": 0}
			for k := range coords {
				if s := n.Attrs[k]; s != "" {
					v, ok := parseGradCoord(s)
					if !ok {
						return nil, false
					}
					coords[k] = v
				}
			}
			// Reject partial vectors: each endpoint coordinate must sit on a box
			// edge (0 or 1) so the gradient covers the full box.
			for _, v := range coords {
				if math.Abs(v) > 1e-9 && math.Abs(v-1) > 1e-9 {
					return nil, false
				}
			}
			// A zero-length gradient vector paints a solid last-stop color; the
			// direction-only <a:lin> can't express that, so fall back.
			if math.Abs(coords["x2"]-coords["x1"]) < 1e-9 && math.Abs(coords["y2"]-coords["y1"]) < 1e-9 {
				return nil, false
			}
			gr.Angle = math.Atan2(coords["y2"]-coords["y1"], coords["x2"]-coords["x1"]) * 180 / math.Pi
			if gr.Angle < 0 {
				gr.Angle += 360
			}
		}
	} else {
		gr.Kind = pptx.GradientRadial
		// The writer always emits a centered radial fill, so reject any custom
		// radial geometry or non-default gradientUnits rather than silently
		// rendering a different gradient.
		for _, a := range []string{"cx", "cy", "r", "fx", "fy", "fr"} {
			if n.Attrs[a] != "" {
				return nil, false
			}
		}
		if gu := n.Attrs["gradientunits"]; gu != "" && strings.ToLower(gu) != "objectboundingbox" {
			return nil, false
		}
	}
	stops, ok := c.gradientStops(n)
	if !ok {
		return nil, false
	}
	if len(stops) > 0 {
		gr.Stops = stops
	}
	if len(gr.Stops) == 0 {
		return nil, false
	}
	if len(gr.Stops) == 1 {
		// DrawingML requires at least two gradient stops; a single SVG stop is
		// visually a solid color, so duplicate it at both ends.
		s := gr.Stops[0]
		gr.Stops = []pptx.GradientStop{{Pos: 0, Color: s.Color, Alpha: s.Alpha}, {Pos: 1, Color: s.Color, Alpha: s.Alpha}}
	}
	c.gradients[id] = gr
	return gr, true
}
func parseGradCoord(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		// Parse the raw percentage without clamping so the representability
		// check can reject out-of-range coordinates like -100% or 200%.
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v / 100, true
	}
	return parseLength(s, false)
}
func (c *conv) gradientStops(n *node) ([]pptx.GradientStop, bool) {
	var out []pptx.GradientStop
	prevOff := 0.0
	// Resolve the gradient element's own style so a stop's currentColor picks up
	// the gradient's color context instead of defaulting to black.
	base, ok := c.resolveStyle(n, defaultStyle())
	if !ok {
		return nil, false
	}
	for _, ch := range n.Children {
		if ch.Name != "stop" {
			continue
		}
		if hasUnsupportedAttrs(ch) {
			return nil, false
		}
		st, ok := c.resolveStyle(ch, base)
		if !ok {
			return nil, false
		}
		off, ok := parseOffset(ch.Attrs["offset"])
		if !ok {
			return nil, false
		}
		// SVG clamps each offset to be >= the previous stop's offset.
		if off < prevOff {
			off = prevOff
		}
		prevOff = off
		// CSS cascade: a resolved style value (matched stylesheet rule or inline
		// style) overrides the presentation attribute.
		color := ch.Attrs["stop-color"]
		if v := st.get("stop-color"); v != "" {
			color = v
		}
		if color == "" {
			color = "black"
		}
		if strings.EqualFold(strings.TrimSpace(color), "currentcolor") {
			color = st.get("color")
		}
		op := ch.Attrs["stop-opacity"]
		if v := st.get("stop-opacity"); v != "" {
			op = v
		}
		alpha, ok := parseUnit(op, 1)
		if !ok {
			return nil, false
		}
		var col string
		if strings.EqualFold(strings.TrimSpace(color), "transparent") {
			// transparent means fully transparent regardless of stop-opacity.
			col, alpha = "000000", 0
		} else {
			col, ok = parseColor(color)
			if !ok {
				return nil, false
			}
		}
		out = append(out, pptx.GradientStop{Pos: off, Color: col, Alpha: alpha})
	}
	return out, true
}
func parseOffset(s string) (float64, bool) {
	if strings.TrimSpace(s) == "" {
		return 0, false
	}
	return parseUnit(s, 0)
}
func hrefID(n *node) string {
	// SVG2 href takes precedence over legacy xlink:href. When href is present
	// but not a local #fragment, it references an external target we can't
	// resolve, so return "" (the caller then falls back) rather than falling
	// through to xlink:href.
	if v := n.Attrs["href"]; v != "" {
		if strings.HasPrefix(v, "#") {
			return v[1:]
		}
		return ""
	}
	if v := n.Attrs["xlink:href"]; v != "" && strings.HasPrefix(v, "#") {
		return v[1:]
	}
	return ""
}

func (c *conv) expandUse(n *node, st style, m matrix, g *pptx.GroupShape) bool {
	id := hrefID(n)
	if id == "" {
		return false
	}
	ref := c.defs[id]
	if ref == nil {
		return false
	}
	if c.resolvingUse[id] {
		return false
	}
	// <use> compositing opacity and viewport remapping (width/height plus a
	// referenced <symbol viewBox>) are not modeled; fall back when present.
	if op, ok := containerOpacity(st); !ok || op < 1 {
		return false
	}
	if n.Attrs["width"] != "" || n.Attrs["height"] != "" {
		return false
	}
	if ref.Name == "symbol" && ref.Attrs["viewbox"] != "" {
		return false
	}
	x, ok1 := attrLen(n, "x", 0)
	y, ok2 := attrLen(n, "y", 0)
	if !ok1 || !ok2 {
		return false
	}
	m = m.mul(matrix{a: 1, d: 1, e: x, f: y})
	c.resolvingUse[id] = true
	defer delete(c.resolvingUse, id)
	if ref.Name == "symbol" {
		// A symbol renders only through <use>; walk its children directly since
		// the symbol element itself is skipped during a normal tree walk. Apply
		// the same checks walk() would (foreign/attrs/display/transform/opacity).
		if ref.foreign || hasUnsupportedAttrs(ref) {
			return false
		}
		cst, ok := c.resolveStyle(ref, st)
		if !ok {
			return false
		}
		if displayNone(cst) {
			return true
		}
		if tr := ref.Attrs["transform"]; tr != "" {
			tm, ok := parseTransform(tr)
			if !ok {
				return false
			}
			m = m.mul(tm)
		}
		if op, ok := containerOpacity(cst); !ok || op < 1 {
			return false
		}
		for _, ch := range ref.Children {
			if !c.walk(ch, cst, m, g, false) {
				return false
			}
		}
		return true
	}
	return c.walk(ref, st, m, g, false)
}
