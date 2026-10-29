package svgshape

import (
	"strings"

	"github.com/Songmu/slidown/pptx"
)

// DrawingML a:rPr@sz must be 1..4000 pt (emitted as hundredths of a point); a
// font size outside this range can't be represented and forces the fallback.
const (
	minFontPt = 1.0
	maxFontPt = 4000.0
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
	if pt := fs * 0.75; pt < minFontPt || pt > maxFontPt {
		// Outside the DrawingML a:rPr@sz range; fall back rather than emit an
		// out-of-range (schema-invalid) size.
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
	boxY, boxW, boxH := textRunBounds(runs, fs, yemu)
	const tol = 16
	if xemu < -tol || boxY < -tol || boxY+boxH > c.chH+tol || xemu+boxW > c.chW+tol {
		return false
	}
	c.textCount++
	sh := &pptx.Shape{Name: shapeName(n, "Text", c.textCount), X: xemu, Y: boxY, W: boxW, H: boxH, NoInset: true, NoWrap: true, Paragraphs: []*pptx.Paragraph{{Align: align, Runs: runs}}}
	g.Texts = append(g.Texts, sh)
	g.Children = append(g.Children, pptx.GroupChild{Text: sh})
	return true
}

func textRunBounds(runs []*pptx.Run, fs float64, yemu int64) (boxY, boxW, boxH int64) {
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
	boxY = yemu - round(maxUnits*emuPerUnit)
	boxH = round(maxUnits * emuPerUnit * 1.5)
	boxW = round(estW)
	if boxW <= 0 {
		boxW = round(maxUnits * emuPerUnit)
	}
	return boxY, boxW, boxH
}

func (c *conv) textRuns(n *node, st style, pt float64, color, family string) ([]*pptx.Run, bool) {
	var stream textRunStream
	if !stream.emit(collapseSpace(n.Text), color, pt, family) {
		return nil, false
	}
	for _, ch := range n.Children {
		child, ok := c.tspanStyle(ch, n, st)
		if !ok {
			return nil, false
		}
		// display:none removes the run from layout entirely (no advance), so it
		// can be skipped without affecting the placement of later runs.
		if displayNone(child) {
			continue
		}
		// visibility:hidden renders nothing but still advances the text
		// position; any non-empty run (including a whitespace separator) means a
		// later visible run can't be faithfully placed, so mark it pending.
		if visibilityHidden(child) {
			if collapseSpace(ch.Text) != "" {
				stream.pendingInvisible = true
			}
			continue
		}
		run, ok := tspanRun(ch, child, pt, color, family)
		if !ok || !stream.emit(run.Text, run.Color, run.FontSize, run.FontFamily) {
			return nil, false
		}
	}
	return normalizeTextRuns(stream.runs), true
}

type textRunStream struct {
	runs []*pptx.Run
	// A skipped invisible (fill:none / hidden) run still advances the text
	// position in SVG. We can't reproduce that advance, so once a non-empty
	// invisible run appears, fall back if any later run is visible.
	pendingInvisible bool
}

func (s *textRunStream) emit(txt, col string, pt float64, family string) bool {
	if txt == "" {
		return true
	}
	if col == "" {
		// A whitespace-only invisible run still advances the SVG text
		// position, so treat any non-empty invisible run as pending; a
		// later visible run then triggers the native-image fallback.
		s.pendingInvisible = true
		return true
	}
	if s.pendingInvisible {
		return false
	}
	s.runs = append(s.runs, &pptx.Run{Text: txt, FontSize: pt, Color: col, FontFamily: family, FontAllScripts: family != ""})
	return true
}

func (c *conv) tspanStyle(ch, parent *node, st style) (style, bool) {
	// Names were lowercased during parsing, so a foreign-namespace or
	// wrong-case child could match "tspan"; such non-SVG elements can't be
	// faithfully placed, so fall back.
	if ch.foreign || ch.badCase {
		return nil, false
	}
	if ch.Name == "textpath" {
		return nil, false
	}
	if ch.Name != "tspan" {
		return nil, false
	}
	// Mixed content whose character data follows a child element loses
	// document order once flattened (e.g. <text>A<tspan>B</tspan>C</text>
	// would emit A,C,B); reject it rather than reorder.
	if ch.textAfterChild || parent.textAfterChild {
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
	return child, true
}

func tspanRun(n *node, st style, pt float64, color, family string) (pptx.Run, bool) {
	// Glyph strokes on a tspan can't be represented; fall back.
	if sv := resolvePaint(st, st.get("stroke")); sv != "none" && sv != "" {
		return pptx.Run{}, false
	}
	if fs := st.get("font-size"); fs != "" {
		f, ok := parseLength(fs, false)
		if !ok || f <= 0 {
			// Non-positive font size is invalid; fall back.
			return pptx.Run{}, false
		}
		pt = f * 0.75
		if pt < minFontPt || pt > maxFontPt {
			// Outside the DrawingML a:rPr@sz range; fall back.
			return pptx.Run{}, false
		}
	}
	// Runs carry no alpha, so a translucent tspan can't be represented:
	// fall back for the whole SVG.
	op, ok := parseUnit(st.get("opacity"), 1)
	if !ok {
		return pptx.Run{}, false
	}
	fo, ok := parseUnit(st.get("fill-opacity"), 1)
	if !ok {
		return pptx.Run{}, false
	}
	if op*fo < 1 {
		return pptx.Run{}, false
	}
	fillVal := resolvePaint(st, st.get("fill"))
	if fillVal == "none" {
		color = ""
	} else if fillVal != "" {
		color, ok = parseColor(fillVal)
		if !ok {
			return pptx.Run{}, false
		}
	}
	if ff := firstFamily(st.get("font-family")); ff != "" {
		family = ff
	}
	return pptx.Run{Text: collapseSpace(n.Text), Color: color, FontSize: pt, FontFamily: family}, true
}

func normalizeTextRuns(runs []*pptx.Run) []*pptx.Run {
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
	return kept
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
	// A quoted family name may itself contain commas (e.g. "ACME, Inc."), so a
	// leading string must be read up to its closing quote rather than split on
	// the first comma.
	if s[0] == '"' || s[0] == '\'' {
		if end := strings.IndexByte(s[1:], s[0]); end >= 0 {
			return s[1 : 1+end]
		}
		return "" // unterminated quote: not representable
	}
	p := strings.Split(s, ",")[0]
	return strings.TrimSpace(p)
}
