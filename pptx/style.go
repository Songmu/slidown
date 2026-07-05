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
	var l xmlSldLayout
	if err := xml.Unmarshal(data, &l); err != nil {
		return
	}
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
		t.SyntaxStyles[keyword] = styleSpecFromRunPr(rPr)
	}
	t.TableStyle = parseTableStyle(l.CSld.SpTree.GraphicFrame)
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

func styleSpecFromRunPr(rPr xmlRunPr) StyleSpec {
	spec := StyleSpec{
		Bold:       boolAttr(rPr.B),
		Italic:     boolAttr(rPr.I),
		Underline:  underlineAttr(rPr.U),
		Strike:     strikeAttr(rPr.Strike),
		Color:      rPr.SolidFill.SRGBClr.Val,
		BgColor:    rPr.Highlight.SRGBClr.Val,
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

func parseTableStyle(frames []xmlGraphicFrame) *TableStyleSpec {
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
			HeaderFirstCol:  tableCellStyleSpec(c00),
			HeaderOtherCols: tableCellStyleSpec(c01),
			DataFirstCol:    tableCellStyleSpec(c10),
			DataOtherCols:   tableCellStyleSpec(c11),

			OuterHorizontal: lineSpec(c00.TcPr.LnT),
			OuterVertical:   lineSpec(c00.TcPr.LnL),

			HeaderFirstColRight:  lineSpec(c00.TcPr.LnR),
			HeaderFirstColBottom: lineSpec(c00.TcPr.LnB),
			HeaderOtherColRight:  lineSpec(c01.TcPr.LnR),
			HeaderOtherColBottom: lineSpec(c01.TcPr.LnB),
			DataFirstColRight:    lineSpec(c10.TcPr.LnR),
			DataFirstColBottom:   lineSpec(c10.TcPr.LnB),
			DataOtherColRight:    lineSpec(c11.TcPr.LnR),
			DataOtherColBottom:   lineSpec(c11.TcPr.LnB),
		}
	}
	return nil
}

func tableCellStyleSpec(cell xmlTableCell) TableCellStyleSpec {
	spec := TableCellStyleSpec{
		BgColor: cell.TcPr.SolidFill.SRGBClr.Val,
		VAlign:  cell.TcPr.Anchor,
	}
	for _, p := range cell.TxBody.Paras {
		if spec.HAlign == "" {
			spec.HAlign = p.PPr.Algn
		}
		for _, r := range p.Runs {
			spec.Bold = boolAttr(r.RPr.B)
			spec.Italic = boolAttr(r.RPr.I)
			spec.Color = r.RPr.SolidFill.SRGBClr.Val
			spec.FontFamily = r.RPr.Latin.Typeface
			return spec
		}
	}
	return spec
}

func lineSpec(line xmlLine) TableBorderSpec {
	return TableBorderSpec{
		Color:    line.SolidFill.SRGBClr.Val,
		WidthEMU: atoi64(line.W),
		Dash:     line.PrstDash.Val,
		None:     line.NoFill != nil,
	}
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
