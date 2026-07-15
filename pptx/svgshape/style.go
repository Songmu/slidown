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
				// Class/ID selectors are case-sensitive in SVG/XML; preserve
				// their spelling and compare exactly.
				sels = append(sels, sel)
			} else if isIdent(sel) {
				// Element-name selectors compare against the lowercased node name.
				sels = append(sels, strings.ToLower(sel))
			} else {
				return nil, false
			}
		}
		rules = append(rules, cssRule{sels: sels, decl: decl})
	}
	if strings.TrimSpace(s) != "" {
		return nil, false
	}
	return rules, true
}
func stripCSSComments(s string) string {
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+2+j+2:]
	}
}
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func (c *conv) resolveStyle(n *node, inherited style) (style, bool) {
	st := inherited.clone()
	// opacity and display are not inherited: opacity composites per
	// element/group, and display applies only to the element it is set on.
	st["opacity"] = "1"
	delete(st, "display")
	parentColor := inherited.get("color")
	for _, p := range paintProps {
		if v := n.Attrs[p]; v != "" {
			st[p] = v
		}
	}
	matched := c.matchedCSS(n)
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
	// Resolve the CSS-wide "inherit" keyword to the parent's value.
	for k, v := range st {
		if strings.EqualFold(v, "inherit") {
			st[k] = inherited.get(k)
		}
	}
	for k, v := range st {
		if k != "color" && strings.EqualFold(v, "currentColor") {
			st[k] = st["color"]
		}
	}
	return st, true
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
	return strings.ToLower(n.Name) == sel
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
