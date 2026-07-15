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
	// xml:space="preserve" (inherited) keeps runs of whitespace verbatim; the
	// converter collapses whitespace, so fall back to preserve fidelity.
	if strings.EqualFold(st.get("xml:space"), "preserve") {
		return false
	}
	// PowerPoint text runs can't render a glyph stroke, so any visible stroke on
	// the text forces the native-image fallback.
	if sv := resolvePaint(st, st.get("stroke")); sv != "none" && sv != "" {
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
	hasTspan := false
	for _, ch := range n.Children {
		if ch.Name == "tspan" {
			hasTspan = true
		}
	}
	if (fillVal == "none" || fillVal == "") && !hasTspan {
		// Text with no visible fill and no children is invisible; render
		// nothing rather than emitting opaque text.
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
	// color is empty when the element itself has no paintable fill; child runs
	// with their own fill may still render.
	var color string
	if fillVal != "none" && fillVal != "" {
		color, ok = parseColor(fillVal)
		if !ok {
			return false
		}
	}
	// The text element's own visibility hides its direct text, but a descendant
	// tspan may override it, so this is applied per run rather than skipping the
	// whole element.
	if visibilityHidden(st) {
		color = ""
	}
	family := firstFamily(st.get("font-family"))
	runs, ok := c.textRuns(n, st, fs*0.75, color, family)
	if !ok {
		return false
	}
	if len(runs) == 0 {
		return true
	}
	// A PowerPoint group doesn't clip its children, so a text box positioned
	// outside the viewport would render outside the placed image. Fall back
	// rather than clamp when the box origin/extent leaves the viewBox.
	boxY := yemu - round(fs*emuPerUnit)
	boxH := round(fs * emuPerUnit * 1.5)
	w := c.chW - xemu
	const tol = 16
	if xemu < -tol || xemu > c.chW+tol || boxY < -tol || boxY+boxH > c.chH+tol || w <= 0 {
		return false
	}
	c.textCount++
	sh := &pptx.Shape{Name: shapeName(n, "Text", c.textCount), X: xemu, Y: boxY, W: w, H: boxH, NoInset: true, Paragraphs: []*pptx.Paragraph{{Align: align, Runs: runs}}}
	g.Texts = append(g.Texts, sh)
	g.Children = append(g.Children, pptx.GroupChild{Text: sh})
	return true
}
func (c *conv) textRuns(n *node, st style, pt float64, color, family string) ([]*pptx.Run, bool) {
	var runs []*pptx.Run
	add := func(txt, col string) {
		// col == "" means no paintable fill; skip such runs.
		if txt != "" && col != "" {
			runs = append(runs, &pptx.Run{Text: txt, FontSize: pt, Color: col, FontFamily: family})
		}
	}
	add(collapseSpace(n.Text), color)
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
		// Positioned or nested tspans (x/y/dx/dy/rotate, or child elements)
		// change layout in ways not modeled here; fall back.
		for _, a := range []string{"x", "y", "dx", "dy", "rotate"} {
			if ch.Attrs[a] != "" {
				return nil, false
			}
		}
		if len(ch.Children) > 0 {
			return nil, false
		}
		// Validate the tspan's attributes the same way as any element so an
		// unsupported presentation attribute (e.g. font-weight) forces fallback.
		if hasUnsupportedAttrs(ch) {
			return nil, false
		}
		child, ok := c.resolveStyle(ch, st)
		if !ok {
			return nil, false
		}
		// xml:space="preserve" (inherited) changes whitespace handling.
		if strings.EqualFold(child.get("xml:space"), "preserve") {
			return nil, false
		}
		// A tspan hidden via display/visibility renders nothing.
		if displayNone(child) || visibilityHidden(child) {
			continue
		}
		// Glyph strokes on a tspan can't be represented; fall back.
		if sv := resolvePaint(child, child.get("stroke")); sv != "none" && sv != "" {
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
			// A no-fill tspan is invisible; skip it.
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
		txt := collapseSpace(ch.Text)
		if txt != "" && col != "" {
			runs = append(runs, &pptx.Run{Text: txt, FontSize: cpt, Color: col, FontFamily: fam})
		}
	}
	// Trim the leading/trailing whitespace of the whole text stream while
	// preserving separators between runs.
	if len(runs) > 0 {
		runs[0].Text = strings.TrimLeft(runs[0].Text, " ")
		last := runs[len(runs)-1]
		last.Text = strings.TrimRight(last.Text, " ")
	}
	// Drop any runs emptied by trimming.
	kept := runs[:0]
	for _, r := range runs {
		if r.Text != "" {
			kept = append(kept, r)
		}
	}
	return kept, true
}

// collapseSpace collapses runs of XML whitespace to a single space without
// trimming the ends, so separators between adjacent runs (e.g. "Hello " before
// a <tspan>) are preserved.
func collapseSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		if space && b.Len() == 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	if space && b.Len() > 0 {
		b.WriteByte(' ')
	}
	return b.String()
}
func firstFamily(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	p := strings.Split(s, ",")[0]
	return strings.Trim(strings.TrimSpace(p), "'\"")
}
