package svgshape

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Songmu/slidown/pptx"
)

type style map[string]string

type cssRule struct {
	sels []string
	decl style
}

var inheritedProps = map[string]bool{"fill": true, "stroke": true, "opacity": true, "fill-opacity": true, "stroke-opacity": true, "stroke-width": true, "stroke-linecap": true, "stroke-linejoin": true, "stroke-dasharray": true, "stroke-miterlimit": true, "fill-rule": true, "font-size": true, "font-family": true, "text-anchor": true, "color": true, "display": true, "visibility": true, "xml:space": true}
var paintProps = []string{"fill", "stroke", "opacity", "fill-opacity", "stroke-opacity", "stroke-width", "stroke-linecap", "stroke-linejoin", "stroke-dasharray", "stroke-miterlimit", "fill-rule", "font-size", "font-family", "text-anchor", "color", "display", "visibility", "xml:space"}

func defaultStyle() style {
	return style{"fill": "black", "stroke": "none", "opacity": "1", "fill-opacity": "1", "stroke-opacity": "1", "stroke-width": "1", "stroke-linecap": "butt", "stroke-linejoin": "miter", "fill-rule": "nonzero", "font-size": "16", "text-anchor": "start", "color": "black"}
}
func (s style) clone() style {
	out := style{}
	for k, v := range s {
		out[k] = v
	}
	return out
}
func (s style) get(k string) string { return s[strings.ToLower(k)] }

func parseStyleDecl(s string) (style, bool) {
	out := style{}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, false
		}
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		// The !important cascade isn't modeled; reject rather than treat the
		// flagged value as a plain declaration (which would mis-handle e.g.
		// display:none !important).
		if strings.Contains(strings.ToLower(v), "!important") {
			return nil, false
		}
		if inheritedProps[k] || k == "stop-color" || k == "stop-opacity" {
			out[k] = v
			continue
		}
		// An unsupported declaration that affects rendering (e.g. display,
		// clip-path, filter, mix-blend-mode) would otherwise be silently
		// dropped and produce an altered image reported as a faithful
		// conversion. Reject so the whole SVG takes the native-picture fallback.
		return nil, false
	}
	return out, true
}

func parseCSS(s string) ([]cssRule, bool) {
	s = stripCSSComments(s)
	var rules []cssRule
	totalSels := 0
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i+1:], '}')
		if j < 0 {
			return nil, false
		}
		j += i + 1
		selText := strings.TrimSpace(s[:i])
		body := s[i+1 : j]
		s = s[j+1:]
		if selText == "" {
			continue
		}
		if strings.HasPrefix(selText, "@") {
			return nil, false
		}
		decl, ok := parseStyleDecl(body)
		if !ok {
			return nil, false
		}
		var sels []string
		for _, raw := range strings.Split(selText, ",") {
			sel := strings.TrimSpace(raw)
			if sel == "" {
				return nil, false
			}
			if strings.ContainsAny(sel, " >+~[:*") {
				return nil, false
			}
			if strings.HasPrefix(sel, ".") || strings.HasPrefix(sel, "#") {
				// Only a single simple class/ID selector is supported; a
				// compound like ".foo.bar" would be mis-matched as one name, so
				// reject anything but a bare identifier after the prefix.
				if !isIdent(sel[1:]) {
					return nil, false
				}
				// Class/ID selectors are case-sensitive in SVG/XML; preserve
				// their spelling and compare exactly.
				sels = append(sels, sel)
			} else if isIdent(sel) {
				// Element-name selectors are case-sensitive in SVG/XML; keep
				// their original spelling and compare exactly.
				sels = append(sels, sel)
			} else {
				return nil, false
			}
			totalSels++
			if totalSels > maxCSSSelectors {
				return nil, false
			}
		}
		rules = append(rules, cssRule{sels: sels, decl: decl})
		if len(rules) > maxCSSRules {
			return nil, false
		}
	}
	if strings.TrimSpace(s) != "" {
		return nil, false
	}
	return rules, true
}

// stripCSSComments removes /* ... */ comments, but not comment-like sequences
// that appear inside a quoted string (e.g. font-family:"A/*B*/"), so a valid
// stylesheet isn't silently rewritten into a different one.
func stripCSSComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	var quote byte // 0 when not inside a string, else the opening quote char
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return b.String() // unterminated comment: drop the rest
			}
			i += 2 + end + 1 // loop's i++ lands just past the closing */
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			// Always a valid identifier code point.
		case r == '-':
			// '-' is allowed; leading-hyphen constraints are checked below.
		case r >= '0' && r <= '9':
			// A CSS identifier can't start with a digit.
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	// A lone "-" and a leading "-<digit>" (e.g. "-1") are not valid identifier
	// starts, so such selectors trigger the document fallback.
	if s[0] == '-' && (len(s) == 1 || s[1] >= '0' && s[1] <= '9') {
		return false
	}
	return true
}

func (c *conv) resolveStyle(n *node, inherited style) (style, bool) {
	st := inherited.clone()
	// opacity and display are not inherited: opacity composites per
	// element/group, and display applies only to the element it is set on.
	// stop-color/stop-opacity are not inherited either and must not leak from a
	// parent (e.g. a gradient) into each <stop>.
	st["opacity"] = "1"
	delete(st, "display")
	delete(st, "stop-color")
	delete(st, "stop-opacity")
	parentColor := inherited.get("color")
	for _, p := range paintProps {
		if v := n.Attrs[p]; v != "" {
			st[p] = v
		}
	}
	matched := c.matchedCSS(n)
	if c.cssWork > maxCSSWork {
		// Bound the total selector-matching work across the whole document so a
		// large stylesheet crossed with many elements can't exhaust CPU.
		return nil, false
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].spec < matched[j].spec })
	for _, r := range matched {
		for k, v := range r.decl {
			st[k] = v
		}
	}
	if inline := n.Attrs["style"]; inline != "" {
		decl, ok := parseStyleDecl(inline)
		if !ok {
			return nil, false
		}
		for k, v := range decl {
			st[k] = v
		}
	}
	// color:currentColor means "inherit the parent's color"; resolve it first so
	// other paint properties referencing currentColor pick up the right value.
	if strings.EqualFold(st["color"], "currentColor") {
		st["color"] = parentColor
	}
	// Resolve the CSS-wide keywords. "inherit" always takes the parent value.
	// "unset"/"revert" take the parent value for inherited properties but the
	// initial value for non-inherited ones (opacity/display/stop-color/
	// stop-opacity). "initial" always restores the property's initial value.
	setInitial := func(k string) {
		if v, ok := initialStyleValue[k]; ok {
			st[k] = v
		} else if dv := defaultStyle().get(k); dv != "" {
			st[k] = dv
		} else {
			delete(st, k)
		}
	}
	for k, v := range st {
		switch strings.ToLower(v) {
		case "inherit":
			st[k] = inherited.get(k)
		case "unset", "revert":
			if nonInheritedProps[k] {
				setInitial(k)
			} else {
				st[k] = inherited.get(k)
			}
		case "initial":
			setInitial(k)
		}
	}
	for k, v := range st {
		if k != "color" && strings.EqualFold(v, "currentColor") {
			st[k] = st["color"]
		}
	}
	// Enumerated properties with an unrecognized value are invalid declarations.
	// CSS would ignore them (keeping the inherited value), so treating the raw
	// value as a default here could flip hole-filling (fill-rule) or reveal
	// hidden geometry (visibility). Reject so the document takes the faithful
	// native-image fallback instead.
	for prop, allowed := range enumStyleValues {
		if v, ok := st[prop]; ok && !allowed[strings.ToLower(strings.TrimSpace(v))] {
			return nil, false
		}
	}
	return st, true
}

// nonInheritedProps are the properties reset per element rather than inherited,
// so "unset"/"revert" resolve them to their initial value (not the parent's).
var nonInheritedProps = map[string]bool{
	"opacity": true, "display": true, "stop-color": true, "stop-opacity": true,
}

// initialStyleValue defines CSS initial values for properties whose initial
// value isn't captured by defaultStyle (the gradient stop properties).
var initialStyleValue = map[string]string{
	"stop-color":   "black",
	"stop-opacity": "1",
}

// enumStyleValues lists the valid keyword values for the enumerated properties
// the converter relies on for correct rendering.
var enumStyleValues = map[string]map[string]bool{
	"fill-rule":  {"nonzero": true, "evenodd": true},
	"visibility": {"visible": true, "hidden": true, "collapse": true},
}

// matchedRule pairs a rule's declarations with the specificity of the most
// specific selector in the rule that actually matched the element.
type matchedRule struct {
	spec int
	decl style
}

func (c *conv) matchedCSS(n *node) []matchedRule {
	var out []matchedRule
	for _, r := range c.css {
		best := -1
		for _, sel := range r.sels {
			c.cssWork++
			if matchSelector(n, sel) {
				if s := selSpec(sel); s > best {
					best = s
				}
			}
		}
		if best >= 0 {
			out = append(out, matchedRule{spec: best, decl: r.decl})
		}
	}
	return out
}
func matchSelector(n *node, sel string) bool {
	if strings.HasPrefix(sel, "#") {
		return n.Attrs["id"] == sel[1:]
	}
	if strings.HasPrefix(sel, ".") {
		for _, cl := range strings.Fields(n.Attrs["class"]) {
			if cl == sel[1:] {
				return true
			}
		}
		return false
	}
	// Type selector: case-sensitive match against the element's original name.
	return n.rawName == sel
}

// selSpec returns the specificity of a single selector: id (100) > class (10) >
// type (1).
func selSpec(sel string) int {
	if strings.HasPrefix(sel, "#") {
		return 100
	}
	if strings.HasPrefix(sel, ".") {
		return 10
	}
	return 1
}

// resolvePaint normalizes a fill/stroke paint value: currentColor resolves to
// the inherited color, and transparent maps to no paint ("none").
func resolvePaint(st style, v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "currentcolor":
		return st.get("color")
	case "transparent":
		return "none"
	}
	return v
}

func (c *conv) paint(st style, m matrix, forceFillNone bool) (pptx.Fill, *pptx.Stroke, bool) {
	op, ok := parseUnit(st.get("opacity"), 1)
	if !ok {
		return pptx.Fill{}, nil, false
	}
	fill := pptx.Fill{Kind: pptx.FillNone}
	fillVal := resolvePaint(st, st.get("fill"))
	if !forceFillNone && fillVal != "none" && fillVal != "" {
		fo, ok := parseUnit(st.get("fill-opacity"), 1)
		if !ok {
			return fill, nil, false
		}
		if strings.HasPrefix(fillVal, "url(") {
			id := urlID(fillVal)
			gr := c.gradients[id]
			if id == "" || gr == nil {
				return fill, nil, false
			}
			fill = pptx.Fill{Kind: pptx.FillGradient, Alpha: op * fo, Gradient: gr}
		} else {
			col, ok := parseColor(fillVal)
			if !ok {
				return fill, nil, false
			}
			fill = pptx.Fill{Kind: pptx.FillSolid, Color: col, Alpha: op * fo}
		}
	}
	var stroke *pptx.Stroke
	strokeVal := resolvePaint(st, st.get("stroke"))
	if strokeVal != "none" && strokeVal != "" {
		// A non-similarity transform (non-uniform scale or skew) stretches the
		// stroke outline anisotropically, which a single uniform stroke width
		// can't reproduce, so fall back to the native SVG picture.
		if !m.isSimilarity() {
			return fill, nil, false
		}
		col, ok := parseColor(strokeVal)
		if !ok {
			return fill, nil, false
		}
		so, ok := parseUnit(st.get("stroke-opacity"), 1)
		if !ok {
			return fill, nil, false
		}
		w, ok := parseLength(st.get("stroke-width"), false)
		if !ok {
			return fill, nil, false
		}
		if w == 0 {
			// stroke-width:0 paints no stroke.
			return fill, nil, true
		}
		if w < 0 {
			// A negative stroke width is invalid; fall back.
			return fill, nil, false
		}
		cap := map[string]string{"butt": "flat", "round": "rnd", "square": "sq"}[strings.ToLower(st.get("stroke-linecap"))]
		if cap == "" {
			return fill, nil, false
		}
		join := strings.ToLower(st.get("stroke-linejoin"))
		if join == "" {
			join = "miter"
		}
		if join != "round" && join != "bevel" && join != "miter" {
			return fill, nil, false
		}
		// The writer emits SVG's default miter limit (4); an explicit different
		// limit isn't modeled, so fall back for miter joins that set one.
		if join == "miter" {
			if ml := strings.TrimSpace(st.get("stroke-miterlimit")); ml != "" {
				v, ok := parseLength(ml, false)
				if !ok || math.Abs(v-4) > 1e-9 {
					return fill, nil, false
				}
			}
		}
		stroke = &pptx.Stroke{Color: col, Alpha: op * so, Width: round(w * emuPerUnit * m.avgScale()), Cap: cap, Join: join}
		// Arbitrary dash patterns can't be mapped to a single OOXML preset
		// faithfully, so fall back to the native SVG picture rather than
		// silently rendering a different dash.
		if da := strings.TrimSpace(st.get("stroke-dasharray")); da != "" && da != "none" {
			return fill, nil, false
		}
	}
	// Element opacity < 1 composites the fill and stroke together, but this
	// model multiplies it into each independently. That only matches when at
	// most one of fill/stroke is present; otherwise fall back.
	if op < 1 && fill.Kind != pptx.FillNone && stroke != nil {
		return fill, nil, false
	}
	return fill, stroke, true
}
func parseUnit(s string, def float64) (float64, bool) {
	if strings.TrimSpace(s) == "" {
		return def, true
	}
	if strings.HasSuffix(strings.TrimSpace(s), "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return math.Max(0, math.Min(1, v/100)), true
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return math.Max(0, math.Min(1, v)), true
}
func urlID(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "url(") || !strings.HasSuffix(s, ")") {
		return ""
	}
	inner := strings.Trim(strings.TrimSpace(s[4:len(s)-1]), "'\"")
	if strings.HasPrefix(inner, "#") {
		return inner[1:]
	}
	return ""
}
