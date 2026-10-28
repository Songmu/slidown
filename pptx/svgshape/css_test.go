package svgshape

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseCSS(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        []cssRule
		ok          bool
	}{
		{name: "missing declaration colon", input: `rect{fill red}`},
		{
			name: "comma remains in declaration value", input: `rect{fill:red,stroke:blue}`,
			want: []cssRule{{sels: []string{"rect"}, decl: style{"fill": "red,stroke:blue"}}}, ok: true,
		},
		{name: "empty list member", input: `rect, {fill:red}`},
		{name: "empty class name", input: `.{fill:red}`},
		{name: "empty id name", input: `#{fill:red}`},
		{name: "descendant selector", input: `g rect{fill:red}`},
		{name: "invalid bare identifier", input: `1rect{fill:red}`},
		{name: "compound type selector", input: `rect.hot{fill:red}`},
		{name: "lone hyphen", input: `-{fill:red}`},
		{name: "missing closing brace", input: `rect{fill:red`},
		{name: "at rule", input: `@media screen{fill:red}`},
		{name: "trailing text", input: `rect{fill:red} stray`},
		{
			name: "empty selector skips invalid body", input: ` {not a declaration} `,
			ok: true,
		},
		{
			name: "empty body preserves selectors", input: ` RECT, .Hot, #Icon { }`,
			want: []cssRule{{sels: []string{"RECT", ".Hot", "#Icon"}, decl: style{}}}, ok: true,
		},
		{
			name: "comment joins identifier", input: `re/* join */ct{fill:red}`,
			want: []cssRule{{sels: []string{"rect"}, decl: style{"fill": "red"}}}, ok: true,
		},
		{
			name: "unterminated trailing comment", input: `rect{fill:red}/* unfinished`,
			want: []cssRule{{sels: []string{"rect"}, decl: style{"fill": "red"}}}, ok: true,
		},
		{
			name: "escaped quote preserves comment text", input: `rect{font-family:'A\'/*B*/'}/* removed */`,
			want: []cssRule{{sels: []string{"rect"}, decl: style{"font-family": `'A\'/*B*/'`}}}, ok: true,
		},
		{name: "quoted brace still ends rule", input: `rect{font-family:"A}B"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCSS(tc.input)
			if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseCSS(%q) = (%#v, %v), want (%#v, %v)", tc.input, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestParseCSSLimits(t *testing.T) {
	selectorRule := strings.Repeat("rect,", maxCSSSelectors/2-1) + "rect{}"
	for _, tc := range []struct {
		name, input      string
		rules, selectors int
		ok               bool
	}{
		{"rules at limit", strings.Repeat("rect{}", maxCSSRules), maxCSSRules, maxCSSRules, true},
		{"rules over limit", strings.Repeat("rect{}", maxCSSRules+1), 0, 0, false},
		{"selectors at limit across rules", selectorRule + selectorRule, 2, maxCSSSelectors, true},
		{"selectors over limit across rules", selectorRule + selectorRule + "rect{}", 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules, ok := parseCSS(tc.input)
			if ok != tc.ok || len(rules) != tc.rules {
				t.Fatalf("parseCSS() returned %d rules, %v; want %d, %v", len(rules), ok, tc.rules, tc.ok)
			}
			selectors := 0
			for _, rule := range rules {
				selectors += len(rule.sels)
			}
			if selectors != tc.selectors {
				t.Fatalf("got %d selectors, want %d", selectors, tc.selectors)
			}
		})
	}
}
