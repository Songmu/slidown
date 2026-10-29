package svgshape

import (
	"fmt"
	"testing"
)

func TestConvertTextRuneWidth(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		width      int64
	}{
		{"accented", "é", 95250},
		{"CJK", "漢字", 190500},
		{"emoji", "🙂", 95250},
		{"combining_mark", "e\u0301", 190500},
		{"mixed", "A漢🙂", 285750},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := mustConvert(t, fmt.Sprintf(`<svg viewBox="0 0 40 40"><text x="1" y="20" font-size="10">%s</text></svg>`, tt.text))
			sh := g.Texts[0]
			if sh.W != tt.width {
				t.Fatalf("width = %d, want %d", sh.W, tt.width)
			}
			if got := sh.Paragraphs[0].Runs[0].Text; got != tt.text {
				t.Fatalf("text = %q, want %q", got, tt.text)
			}
		})
	}
}

func TestConvertTextBoundsTolerance(t *testing.T) {
	for _, edge := range []string{"left", "top", "right", "bottom"} {
		for _, excess := range []int{15, 16, 17} {
			t.Run(fmt.Sprintf("%s_%d_EMU", edge, excess), func(t *testing.T) {
				x, y := 10.0, 20.0
				offset := float64(excess) / emuPerUnit
				switch edge {
				case "left":
					x = -offset
				case "top":
					y = 10 - offset
				case "right":
					x = 90 + offset
				case "bottom":
					y = 95 + offset
				}
				svg := fmt.Sprintf(`<svg viewBox="0 0 100 100"><text x="%.12f" y="%.12f" font-size="10">A</text></svg>`, x, y)
				g, ok := Convert([]byte(svg))
				if want := excess <= 16; ok != want {
					t.Fatalf("Convert() ok = %v, want %v", ok, want)
				}
				if ok && (g == nil || len(g.Texts) != 1) {
					t.Fatalf("expected one text shape, got %#v", g)
				}
			})
		}
	}
}

func TestConvertTextRunBoundsRounding(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		y, w, h    int64
	}{
		{
			name: "largest_child_run",
			text: `<text x="6.87506" y="36.62506" transform="translate(.125 .375)" font-size="10.00004">A<tspan font-size="7.00004">漢</tspan><tspan font-size="13.00004">🙂</tspan></text>`,
			y:    161926, w: 285751, h: 185738,
		},
		{
			name: "parent_size_floor",
			text: `<text x="6.87506" y="36.62506" transform="translate(.125 .375)" font-size="20.00004"><tspan font-size="7.00004">漢</tspan><tspan font-size="13.00004">🙂</tspan></text>`,
			y:    95251, w: 190501, h: 285751,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Each run contributes a fractional EMU; round only after summing
			// widths, and round the baseline and maximum run size separately.
			g := mustConvert(t, `<svg viewBox="5 7 200 100"><rect x="5" y="7" width="1" height="1"/>`+tt.text+`<rect x="5" y="7" width="1" height="1"/></svg>`)
			if len(g.Texts) != 1 {
				t.Fatalf("expected one text shape, got %d", len(g.Texts))
			}
			sh := g.Texts[0]
			if sh.X != 19051 || sh.Y != tt.y || sh.W != tt.w || sh.H != tt.h {
				t.Fatalf("bounds = (%d, %d, %d, %d), want (19051, %d, %d, %d)", sh.X, sh.Y, sh.W, sh.H, tt.y, tt.w, tt.h)
			}
			if sh.Name != "Text 1" || !sh.NoInset || !sh.NoWrap {
				t.Fatalf("unexpected text properties: %#v", sh)
			}
			if len(g.Children) != 3 || g.Children[0].Geom == nil || g.Children[1].Text != sh || g.Children[2].Geom == nil {
				t.Fatalf("text must remain between its geometry siblings: %#v", g.Children)
			}
		})
	}
}
