package pptx

import (
	"encoding/xml"
	"strconv"
	"strings"
)

const styleLayoutName = "style"

// StyleSpec captures a custom inline style parsed from a template's style
// layout. Baseline is "", "super", or "sub".
type StyleSpec struct {
	Bold, Italic, Underline, Strike bool
	Color, BgColor, FontFamily      string
	Baseline                        string
}

type TableBorderSpec struct {
	Color    string
	WidthEMU int64
	Dash     string
	None     bool
}

// TableCellStyleSpec holds the styling for one table region. Only bold, italic,
// text color and font family are currently supported for cell text.
// TODO: extend table cell text styling to full deck parity by also honoring
// underline, strikethrough, superscript/subscript baseline, highlight/background
// color and font size (all already expressible on pptx.Run).
type TableCellStyleSpec struct {
	BgColor           string
	Bold, Italic      bool
	Color, FontFamily string
	HAlign, VAlign    string
}

type TableStyleSpec struct {
	HeaderFirstCol  TableCellStyleSpec
	HeaderOtherCols TableCellStyleSpec
	DataFirstCol    TableCellStyleSpec
	DataOtherCols   TableCellStyleSpec

	OuterHorizontal TableBorderSpec
	OuterVertical   TableBorderSpec

	HeaderFirstColRight  TableBorderSpec
	HeaderFirstColBottom TableBorderSpec
	HeaderOtherColRight  TableBorderSpec
	HeaderOtherColBottom TableBorderSpec
	DataFirstColRight    TableBorderSpec
	DataFirstColBottom   TableBorderSpec
	DataOtherColRight    TableBorderSpec
	DataOtherColBottom   TableBorderSpec
}

func (t *Template) loadStyleLayout(data []byte) {
	t.loadStyleLayoutPart("", data)
}

func (t *Template) loadStyleLayoutPart(layoutPart string, data []byte) {
	var l xmlSldLayout
	if err := xml.Unmarshal(data, &l); err != nil {
		return
	}
	resolveScheme := t.styleColorResolver(layoutPart, l)
	if t.SyntaxStyles == nil {
		t.SyntaxStyles = map[string]StyleSpec{}
	}
	for _, sp := range l.CSld.SpTree.Sp {
		keyword := strings.TrimSpace(shapeText(sp))
		if keyword == "" {
			continue
		}
		rPr, ok := firstRunPr(sp.TxBody.Paras)
		if !ok {
			continue
		}
		t.SyntaxStyles[keyword] = styleSpecFromRunPr(rPr, resolveScheme)
	}
	t.TableStyle = parseTableStyle(l.CSld.SpTree.GraphicFrame, resolveScheme)
}

func shapeText(sp xmlSp) string {
	var b strings.Builder
	for pi, p := range sp.TxBody.Paras {
		if pi > 0 {
			b.WriteByte('\n')
		}
		for _, r := range p.Runs {
			b.WriteString(r.T)
		}
	}
	return b.String()
}

func firstRunPr(paras []xmlPara) (xmlRunPr, bool) {
	for _, p := range paras {
		for _, r := range p.Runs {
			return r.RPr, true
		}
	}
	return xmlRunPr{}, false
}

func styleSpecFromRunPr(rPr xmlRunPr, resolveScheme func(string) string) StyleSpec {
	spec := StyleSpec{
		Bold:       boolAttr(rPr.B),
		Italic:     boolAttr(rPr.I),
		Underline:  underlineAttr(rPr.U),
		Strike:     strikeAttr(rPr.Strike),
		Color:      colorValue(rPr.SolidFill.SRGBClr, rPr.SolidFill.SchemeClr, rPr.SolidFill.SysClr, resolveScheme),
		BgColor:    colorValue(rPr.Highlight.SRGBClr, rPr.Highlight.SchemeClr, rPr.Highlight.SysClr, resolveScheme),
		FontFamily: rPr.Latin.Typeface,
	}
	if n, err := strconv.Atoi(strings.TrimSpace(rPr.Baseline)); err == nil {
		switch {
		case n > 0:
			spec.Baseline = "super"
		case n < 0:
			spec.Baseline = "sub"
		}
	}
	return spec
}

func parseTableStyle(frames []xmlGraphicFrame, resolveScheme func(string) string) *TableStyleSpec {
	for _, frame := range frames {
		tbl := frame.Graphic.GraphicData.Tbl
		if len(tbl.Rows) != 2 {
			continue
		}
		if len(tbl.Rows[0].Cells) != 2 || len(tbl.Rows[1].Cells) != 2 {
			continue
		}
		c00 := tbl.Rows[0].Cells[0]
		c01 := tbl.Rows[0].Cells[1]
		c10 := tbl.Rows[1].Cells[0]
		c11 := tbl.Rows[1].Cells[1]
		return &TableStyleSpec{
			HeaderFirstCol:  tableCellStyleSpec(c00, resolveScheme),
			HeaderOtherCols: tableCellStyleSpec(c01, resolveScheme),
			DataFirstCol:    tableCellStyleSpec(c10, resolveScheme),
			DataOtherCols:   tableCellStyleSpec(c11, resolveScheme),

			OuterHorizontal: lineSpec(c00.TcPr.LnT, resolveScheme),
			OuterVertical:   lineSpec(c00.TcPr.LnL, resolveScheme),

			HeaderFirstColRight:  lineSpec(c00.TcPr.LnR, resolveScheme),
			HeaderFirstColBottom: lineSpec(c00.TcPr.LnB, resolveScheme),
			HeaderOtherColRight:  lineSpec(c01.TcPr.LnR, resolveScheme),
			HeaderOtherColBottom: lineSpec(c01.TcPr.LnB, resolveScheme),
			DataFirstColRight:    lineSpec(c10.TcPr.LnR, resolveScheme),
			DataFirstColBottom:   lineSpec(c10.TcPr.LnB, resolveScheme),
			DataOtherColRight:    lineSpec(c11.TcPr.LnR, resolveScheme),
			DataOtherColBottom:   lineSpec(c11.TcPr.LnB, resolveScheme),
		}
	}
	return nil
}

func tableCellStyleSpec(cell xmlTableCell, resolveScheme func(string) string) TableCellStyleSpec {
	spec := TableCellStyleSpec{
		BgColor: colorValue(cell.TcPr.SolidFill.SRGBClr, cell.TcPr.SolidFill.SchemeClr, cell.TcPr.SolidFill.SysClr, resolveScheme),
		VAlign:  cell.TcPr.Anchor,
	}
	for _, p := range cell.TxBody.Paras {
		if spec.HAlign == "" {
			spec.HAlign = p.PPr.Algn
		}
		for _, r := range p.Runs {
			// TODO: parse the remaining run properties (underline, strike,
			// baseline, highlight/background color, font size) for full deck
			// parity on table cell text styling.
			spec.Bold = boolAttr(r.RPr.B)
			spec.Italic = boolAttr(r.RPr.I)
			spec.Color = colorValue(r.RPr.SolidFill.SRGBClr, r.RPr.SolidFill.SchemeClr, r.RPr.SolidFill.SysClr, resolveScheme)
			spec.FontFamily = r.RPr.Latin.Typeface
			return spec
		}
	}
	return spec
}

func lineSpec(line xmlLine, resolveScheme func(string) string) TableBorderSpec {
	return TableBorderSpec{
		Color:    colorValue(line.SolidFill.SRGBClr, line.SolidFill.SchemeClr, line.SolidFill.SysClr, resolveScheme),
		WidthEMU: atoi64(line.W),
		Dash:     line.PrstDash.Val,
		None:     line.NoFill != nil,
	}
}

func colorValue(srgb xmlSRGBClr, scheme xmlSchemeClr, sys xmlSysClr, resolveScheme func(string) string) string {
	if v := strings.TrimSpace(srgb.Val); v != "" {
		return v
	}
	if v := strings.TrimSpace(sys.LastClr); v != "" {
		return v
	}
	if resolveScheme != nil {
		// NOTE: scheme color transforms (tint/shade/lum*/sat*) are currently
		// ignored and we resolve to the base scheme slot color only.
		// TODO: Apply DrawingML color transforms to scheme colors.
		if v := resolveScheme(strings.TrimSpace(scheme.Val)); v != "" {
			return v
		}
	}
	return ""
}

func (t *Template) styleColorResolver(layoutPart string, layout xmlSldLayout) func(string) string {
	if layoutPart == "" {
		return nil
	}
	masterPart := t.masterOf(layoutPart)
	if masterPart == "" {
		return nil
	}
	masterClrMap := parseMasterClrMap(t.designParts[masterPart])
	if ovr := layout.ClrMapOvr.OverrideClrMapping; ovr != nil {
		masterClrMap = *ovr
	}
	themePart := t.themeReferencedBy(masterPart)
	if themePart == "" {
		themePart = t.themePart
	}
	colors := parseThemeSchemeColors(t.designParts[themePart])
	if len(colors) == 0 {
		return nil
	}
	return func(slot string) string {
		mapped := applyClrMap(slot, masterClrMap)
		return colors[mapped]
	}
}

func parseMasterClrMap(data []byte) xmlClrMap {
	var master struct {
		ClrMap xmlClrMap `xml:"clrMap"`
	}
	if err := xml.Unmarshal(data, &master); err != nil {
		return xmlClrMap{}
	}
	return master.ClrMap
}

func parseThemeSchemeColors(data []byte) map[string]string {
	var theme struct {
		ThemeElements struct {
			ClrScheme struct {
				Dk1      xmlSolidFill `xml:"dk1"`
				Lt1      xmlSolidFill `xml:"lt1"`
				Dk2      xmlSolidFill `xml:"dk2"`
				Lt2      xmlSolidFill `xml:"lt2"`
				Accent1  xmlSolidFill `xml:"accent1"`
				Accent2  xmlSolidFill `xml:"accent2"`
				Accent3  xmlSolidFill `xml:"accent3"`
				Accent4  xmlSolidFill `xml:"accent4"`
				Accent5  xmlSolidFill `xml:"accent5"`
				Accent6  xmlSolidFill `xml:"accent6"`
				Hlink    xmlSolidFill `xml:"hlink"`
				FolHlink xmlSolidFill `xml:"folHlink"`
			} `xml:"clrScheme"`
		} `xml:"themeElements"`
	}
	if err := xml.Unmarshal(data, &theme); err != nil {
		return nil
	}
	c := theme.ThemeElements.ClrScheme
	return map[string]string{
		"dk1":      colorValue(c.Dk1.SRGBClr, c.Dk1.SchemeClr, c.Dk1.SysClr, nil),
		"lt1":      colorValue(c.Lt1.SRGBClr, c.Lt1.SchemeClr, c.Lt1.SysClr, nil),
		"dk2":      colorValue(c.Dk2.SRGBClr, c.Dk2.SchemeClr, c.Dk2.SysClr, nil),
		"lt2":      colorValue(c.Lt2.SRGBClr, c.Lt2.SchemeClr, c.Lt2.SysClr, nil),
		"accent1":  colorValue(c.Accent1.SRGBClr, c.Accent1.SchemeClr, c.Accent1.SysClr, nil),
		"accent2":  colorValue(c.Accent2.SRGBClr, c.Accent2.SchemeClr, c.Accent2.SysClr, nil),
		"accent3":  colorValue(c.Accent3.SRGBClr, c.Accent3.SchemeClr, c.Accent3.SysClr, nil),
		"accent4":  colorValue(c.Accent4.SRGBClr, c.Accent4.SchemeClr, c.Accent4.SysClr, nil),
		"accent5":  colorValue(c.Accent5.SRGBClr, c.Accent5.SchemeClr, c.Accent5.SysClr, nil),
		"accent6":  colorValue(c.Accent6.SRGBClr, c.Accent6.SchemeClr, c.Accent6.SysClr, nil),
		"hlink":    colorValue(c.Hlink.SRGBClr, c.Hlink.SchemeClr, c.Hlink.SysClr, nil),
		"folHlink": colorValue(c.FolHlink.SRGBClr, c.FolHlink.SchemeClr, c.FolHlink.SysClr, nil),
	}
}

func applyClrMap(name string, m xmlClrMap) string {
	switch name {
	case "bg1":
		return pickClrMap(name, m.Bg1)
	case "tx1":
		return pickClrMap(name, m.Tx1)
	case "bg2":
		return pickClrMap(name, m.Bg2)
	case "tx2":
		return pickClrMap(name, m.Tx2)
	case "accent1":
		return pickClrMap(name, m.Accent1)
	case "accent2":
		return pickClrMap(name, m.Accent2)
	case "accent3":
		return pickClrMap(name, m.Accent3)
	case "accent4":
		return pickClrMap(name, m.Accent4)
	case "accent5":
		return pickClrMap(name, m.Accent5)
	case "accent6":
		return pickClrMap(name, m.Accent6)
	case "hlink":
		return pickClrMap(name, m.Hlink)
	case "folHlink":
		return pickClrMap(name, m.FolHlink)
	default:
		return name
	}
}

func pickClrMap(name, mapped string) string {
	if strings.TrimSpace(mapped) == "" {
		return name
	}
	return mapped
}

func boolAttr(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "1" || v == "true" || v == "t"
}

func underlineAttr(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v != "" && v != "none"
}

func strikeAttr(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v != "" && v != "nostrike"
}
