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

var inheritedProps = map[string]bool{"fill": true, "stroke": true, "opacity": true, "fill-opacity": true, "stroke-opacity": true, "stroke-width": true, "stroke-linecap": true, "stroke-linejoin": true, "stroke-dasharray": true, "fill-rule": true, "font-size": true, "font-family": true, "text-anchor": true, "color": true}
var paintProps = []string{"fill", "stroke", "opacity", "fill-opacity", "stroke-opacity", "stroke-width", "stroke-linecap", "stroke-linejoin", "stroke-dasharray", "fill-rule", "font-size", "font-family", "text-anchor", "color"}

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
		if inheritedProps[k] || k == "stop-color" || k == "stop-opacity" {
			out[k] = v
		}
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
			if strings.HasPrefix(sel, ".") || strings.HasPrefix(sel, "#") || isIdent(sel) {
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
	for k, v := range st {
		if strings.EqualFold(v, "currentColor") {
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
		return strings.ToLower(n.Attrs["id"]) == sel[1:]
	}
	if strings.HasPrefix(sel, ".") {
		for _, cl := range strings.Fields(n.Attrs["class"]) {
			if strings.ToLower(cl) == sel[1:] {
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
		stroke = &pptx.Stroke{Color: col, Alpha: op * so, Width: round(w * emuPerUnit * m.avgScale()), Cap: cap, Join: join}
		if da := strings.TrimSpace(st.get("stroke-dasharray")); da != "" && da != "none" {
			stroke.Dash = "dash"
		}
	}
	return fill, stroke, true
}
func parseUnit(s string, def float64) (float64, bool) {
	if strings.TrimSpace(s) == "" {
		return def, true
	}
	if strings.HasSuffix(strings.TrimSpace(s), "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"), 64)
		return math.Max(0, math.Min(1, v/100)), err == nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
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
