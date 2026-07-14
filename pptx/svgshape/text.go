package svgshape

import (
	"strings"

	"github.com/Songmu/slidown/pptx"
)

func (c *conv) text(n *node, st style, m matrix, g *pptx.GroupShape) bool {
	// Only the anchor point is transformed; font size and box dimensions are
	// not, so fall back for any non-translation transform (scale/rotate/skew).
	if !m.isTranslateOnly() {
		return false
	}
	x, ok1 := attrLen(n, "x", 0)
	y, ok2 := attrLen(n, "y", 0)
	if !ok1 || !ok2 {
		return false
	}
	p := m.apply(point{x, y})
	xemu := round((p.x - c.vbMinX) * emuPerUnit)
	yemu := round((p.y - c.vbMinY) * emuPerUnit)
	fs, ok := parseLength(st.get("font-size"), false)
	if !ok {
		return false
	}
	// The text box starts at x and extends rightward, which only matches
	// text-anchor:start. middle/end would need the box geometry computed from
	// the anchor, so fall back rather than mis-anchor the text.
	switch strings.ToLower(st.get("text-anchor")) {
	case "start", "":
	default:
		return false
	}
	align := pptx.AlignLeft
	fillVal := resolvePaint(st, st.get("fill"))
	if fillVal == "none" || fillVal == "" {
		// Text with no visible fill is invisible; render nothing rather than
		// emitting opaque text.
		return true
	}
	// pptx runs carry no alpha, so any translucent text cannot be represented
	// faithfully: fall back to the native SVG picture for the whole document.
	op, ok := parseUnit(st.get("opacity"), 1)
	if !ok {
		return false
	}
	fo, ok := parseUnit(st.get("fill-opacity"), 1)
	if !ok {
		return false
	}
	if op*fo < 1 {
		return false
	}
	color, ok := parseColor(fillVal)
	if !ok {
		return false
	}
	family := firstFamily(st.get("font-family"))
	runs, ok := c.textRuns(n, st, fs*0.75, color, family)
	if !ok {
		return false
	}
	if len(runs) == 0 {
		return true
	}
	// Clamp the text box width so an x beyond the viewBox cannot produce a
	// negative (invalid) OOXML extent.
	w := c.chW - xemu
	if minW := round(fs * emuPerUnit); w < minW {
		w = minW
	}
	c.textCount++
	sh := &pptx.Shape{Name: shapeName(n, "Text", c.textCount), X: xemu, Y: yemu - round(fs*emuPerUnit), W: w, H: round(fs * emuPerUnit * 1.5), NoInset: true, Paragraphs: []*pptx.Paragraph{{Align: align, Runs: runs}}}
	g.Texts = append(g.Texts, sh)
	g.Children = append(g.Children, pptx.GroupChild{Text: sh})
	return true
}
func (c *conv) textRuns(n *node, st style, pt float64, color, family string) ([]*pptx.Run, bool) {
	var runs []*pptx.Run
	add := func(txt string) {
		if txt != "" {
			runs = append(runs, &pptx.Run{Text: txt, FontSize: pt, Color: color, FontFamily: family})
		}
	}
	add(strings.TrimSpace(n.Text))
	for _, ch := range n.Children {
		if ch.Name == "textpath" {
			return nil, false
		}
		if ch.Name != "tspan" {
			return nil, false
		}
		// Mixed content whose character data follows a child element loses
		// document order once flattened (e.g. <text>A<tspan>B</tspan>C</text>
		// would emit A,C,B); reject it rather than reorder.
		if ch.textAfterChild || n.textAfterChild {
			return nil, false
		}
		child, ok := c.resolveStyle(ch, st)
		if !ok {
			return nil, false
		}
		cpt := pt
		if fs := child.get("font-size"); fs != "" {
			f, ok := parseLength(fs, false)
			if !ok {
				return nil, false
			}
			cpt = f * 0.75
		}
		// Runs carry no alpha, so a translucent tspan can't be represented:
		// fall back for the whole SVG.
		cop, ok := parseUnit(child.get("opacity"), 1)
		if !ok {
			return nil, false
		}
		cfo, ok := parseUnit(child.get("fill-opacity"), 1)
		if !ok {
			return nil, false
		}
		if cop*cfo < 1 {
			return nil, false
		}
		col := color
		fillVal := resolvePaint(child, child.get("fill"))
		if fillVal == "none" {
			// A no-fill tspan is invisible; skip it rather than emitting it
			// with the inherited color.
			continue
		}
		if fillVal != "" {
			var ok bool
			col, ok = parseColor(fillVal)
			if !ok {
				return nil, false
			}
		}
		fam := family
		if ff := firstFamily(child.get("font-family")); ff != "" {
			fam = ff
		}
		txt := strings.TrimSpace(ch.Text)
		if txt != "" {
			runs = append(runs, &pptx.Run{Text: txt, FontSize: cpt, Color: col, FontFamily: fam})
		}
	}
	return runs, true
}
func firstFamily(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	p := strings.Split(s, ",")[0]
	return strings.Trim(strings.TrimSpace(p), "'\"")
}
