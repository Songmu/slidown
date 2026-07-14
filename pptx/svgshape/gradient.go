package svgshape

import (
	"math"
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
	seen[id] = true
	n := c.defs[id]
	if n == nil || (n.Name != "lineargradient" && n.Name != "radialgradient") {
		return nil, false
	}
	if gt := n.Attrs["gradienttransform"]; gt != "" {
		return nil, false
	}
	if sp := n.Attrs["spreadmethod"]; sp != "" && strings.ToLower(sp) != "pad" {
		return nil, false
	}
	var parent *pptx.Gradient
	if href := hrefID(n); href != "" {
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
		x1 := 0.0
		y1 := 0.0
		x2 := 1.0
		y2 := 0.0
		var ok bool
		if n.Attrs["x1"] != "" {
			x1, ok = parseGradCoord(n.Attrs["x1"])
			if !ok {
				return nil, false
			}
		}
		if n.Attrs["y1"] != "" {
			y1, ok = parseGradCoord(n.Attrs["y1"])
			if !ok {
				return nil, false
			}
		}
		if n.Attrs["x2"] != "" {
			x2, ok = parseGradCoord(n.Attrs["x2"])
			if !ok {
				return nil, false
			}
		}
		if n.Attrs["y2"] != "" {
			y2, ok = parseGradCoord(n.Attrs["y2"])
			if !ok {
				return nil, false
			}
		}
		gr.Angle = math.Atan2(y2-y1, x2-x1) * 180 / math.Pi
		if gr.Angle < 0 {
			gr.Angle += 360
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
	c.gradients[id] = gr
	return gr, true
}
func parseGradCoord(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		v, ok := parseUnit(s, 0)
		return v, ok
	}
	return parseLength(s, false)
}
func (c *conv) gradientStops(n *node) ([]pptx.GradientStop, bool) {
	var out []pptx.GradientStop
	for _, ch := range n.Children {
		if ch.Name != "stop" {
			continue
		}
		st, ok := c.resolveStyle(ch, defaultStyle())
		if !ok {
			return nil, false
		}
		off, ok := parseOffset(ch.Attrs["offset"])
		if !ok {
			return nil, false
		}
		color := ch.Attrs["stop-color"]
		if color == "" {
			color = st.get("stop-color")
		}
		if color == "" {
			color = "black"
		}
		op := ch.Attrs["stop-opacity"]
		if op == "" {
			op = st.get("stop-opacity")
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
	if v := n.Attrs["href"]; v != "" && strings.HasPrefix(v, "#") {
		return v[1:]
	}
	for k, v := range n.Attrs {
		if strings.HasSuffix(k, ":href") && strings.HasPrefix(v, "#") {
			return v[1:]
		}
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
	return c.walk(ref, st, m, g, false)
}
