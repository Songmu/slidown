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
	if !ok || fs <= 0 {
		// A non-positive font size is invalid and would emit a negative OOXML
		// extent; fall back.
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
	// A PowerPoint group doesn't clip its children, so a text box positioned or
	// sized outside the viewport would render outside the placed image. SVG text
	// is single-line (no wrap); estimate its extent so it can't wrap or overflow
	// the viewport, and fall back otherwise.
	//
	// Vertical bounds use the largest run size (a tspan may be larger than the
	// parent), so a big child can't overflow the box.
	maxUnits := fs
	var estW float64
	for _, r := range runs {
		runUnits := fs
		if r.FontSize > 0 {
			runUnits = r.FontSize / 0.75 // pt back to user units
		}
		if runUnits > maxUnits {
			maxUnits = runUnits
		}
		// Estimate width per run using its own size with a full-em (1.0em)
		// per-character upper bound, since the widest glyphs (e.g. "W",
		// full-width/CJK characters) approach one em; this keeps the extent an
		// over-estimate so near-edge text falls back instead of overflowing.
		estW += float64(len([]rune(r.Text))) * runUnits * emuPerUnit
	}
	boxY := yemu - round(maxUnits*emuPerUnit)
	boxH := round(maxUnits * emuPerUnit * 1.5)
	const tol = 16
	est := round(estW)
	if est <= 0 {
		est = round(maxUnits * emuPerUnit)
	}
	if xemu < -tol || boxY < -tol || boxY+boxH > c.chH+tol || xemu+est > c.chW+tol {
		return false
	}
	c.textCount++
	sh := &pptx.Shape{Name: shapeName(n, "Text", c.textCount), X: xemu, Y: boxY, W: est, H: boxH, NoInset: true, NoWrap: true, Paragraphs: []*pptx.Paragraph{{Align: align, Runs: runs}}}
	g.Texts = append(g.Texts, sh)
	g.Children = append(g.Children, pptx.GroupChild{Text: sh})
	return true
}
func (c *conv) textRuns(n *node, st style, pt float64, color, family string) ([]*pptx.Run, bool) {
	var runs []*pptx.Run
	// A skipped invisible (fill:none / hidden) run still advances the text
	// position in SVG. We can't reproduce that advance, so once a non-empty
	// invisible run appears, fall back if any later run is visible.
	pendingInvisible := false
	emit := func(txt, col string, fpt float64, fam string) bool {
		if strings.TrimSpace(txt) == "" && txt == "" {
			return true
		}
		if col == "" {
			// A whitespace-only invisible run still advances the SVG text
			// position, so treat any non-empty invisible run as pending; a
			// later visible run then triggers the native-image fallback.
			pendingInvisible = true
			return true
		}
		if pendingInvisible {
			return false
		}
		if txt != "" {
			runs = append(runs, &pptx.Run{Text: txt, FontSize: fpt, Color: col, FontFamily: fam})
		}
		return true
	}
	if !emit(collapseSpace(n.Text), color, pt, family) {
		return nil, false
	}
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
		// A tspan hidden via display/visibility renders nothing but still
		// advances the text position.
		if displayNone(child) || visibilityHidden(child) {
			if strings.TrimSpace(collapseSpace(ch.Text)) != "" {
				pendingInvisible = true
			}
			continue
		}
		// Glyph strokes on a tspan can't be represented; fall back.
		if sv := resolvePaint(child, child.get("stroke")); sv != "none" && sv != "" {
			return nil, false
		}
		cpt := pt
		if fs := child.get("font-size"); fs != "" {
			f, ok := parseLength(fs, false)
			if !ok || f <= 0 {
				// Non-positive font size is invalid; fall back.
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
			col = ""
		} else if fillVal != "" {
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
		if !emit(collapseSpace(ch.Text), col, cpt, fam) {
			return nil, false
		}
	}
	// Collapse whitespace across run boundaries so a trailing space on one run
	// plus a leading space on the next don't become a double space.
	for i := 0; i+1 < len(runs); i++ {
		if strings.HasSuffix(runs[i].Text, " ") {
			runs[i+1].Text = strings.TrimLeft(runs[i+1].Text, " ")
		}
	}
	// Trim the leading/trailing whitespace of the whole text stream while
	// preserving separators between runs.
	if len(runs) > 0 {
		runs[0].Text = strings.TrimLeft(runs[0].Text, " ")
		last := runs[len(runs)-1]
		last.Text = strings.TrimRight(last.Text, " ")
	}
	// Drop runs emptied by trimming and preserve boundary whitespace so
	// PowerPoint keeps run separators.
	kept := runs[:0]
	for _, r := range runs {
		if r.Text == "" {
			continue
		}
		if strings.HasPrefix(r.Text, " ") || strings.HasSuffix(r.Text, " ") {
			r.PreserveSpace = true
		}
		kept = append(kept, r)
	}
	return kept, true
}

// collapseSpace collapses runs of XML whitespace to a single space without
// trimming the ends, so separators between adjacent runs (e.g. "Hello " before
// a <tspan>) are preserved.
func collapseSpace(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	if space {
		// A trailing space, or an all-whitespace node (which is a word
		// separator between runs), collapses to a single space.
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
