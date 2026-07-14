package svgshape

import (
	"strings"

	"github.com/Songmu/slidown/pptx"
)

func (c *conv) text(n *node, st style, m matrix, g *pptx.GroupShape) bool {
	if m.hasRotationOrSkew() {
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
	align := pptx.AlignLeft
	switch strings.ToLower(st.get("text-anchor")) {
	case "middle":
		align = pptx.AlignCenter
	case "end":
		align = pptx.AlignRight
	case "start", "":
		align = pptx.AlignLeft
	default:
		return false
	}
	fill := st.get("fill")
	if fill == "" || fill == "none" {
		fill = st.get("color")
	}
	color, ok := parseColor(fill)
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
	c.textCount++
	g.Texts = append(g.Texts, &pptx.Shape{Name: shapeName(n, "Text", c.textCount), X: xemu, Y: yemu - round(fs*emuPerUnit), W: c.chW - xemu, H: round(fs * emuPerUnit * 1.5), Paragraphs: []*pptx.Paragraph{{Align: align, Runs: runs}}})
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
		col := color
		fill := child.get("fill")
		if fill != "" && fill != "none" {
			var ok bool
			col, ok = parseColor(fill)
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
