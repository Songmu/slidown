package pptx

import (
	"strings"
	"testing"
)

func TestRenderShapeEmitsRoleMarker(t *testing.T) {
	sh := &Shape{
		Placeholder:    PlaceholderBody,
		PlaceholderIdx: 1,
		Role:           "subTitle",
		Paragraphs:     []*Paragraph{{Runs: []*Run{{Text: "副題"}}}},
	}
	var rels []slideRel
	relIdx := 1
	out := renderShape(sh, 2, &relIdx, &rels)

	if !strings.Contains(out, `<p:ph type="body" idx="1"/>`) {
		t.Errorf("expected body placeholder type to be preserved, got: %s", out)
	}
	if !strings.Contains(out, `uri="`+shapeMetaURI+`"`) {
		t.Errorf("expected shape meta extension with uri %s, got: %s", shapeMetaURI, out)
	}
	if !strings.Contains(out, `role="subTitle"`) {
		t.Errorf("expected role=\"subTitle\" attribute, got: %s", out)
	}
	// The extension must live inside <p:nvPr>, not at the shape's top level
	// (where <p:extLst> would be invalid per the OOXML schema for p:sp).
	phIdx := strings.Index(out, `<p:ph`)
	extIdx := strings.Index(out, `<p:extLst>`)
	nvprCloseIdx := strings.Index(out, `</p:nvPr>`)
	if phIdx < 0 || extIdx < 0 || nvprCloseIdx < 0 {
		t.Fatalf("missing expected fragments in: %s", out)
	}
	if !(phIdx < extIdx && extIdx < nvprCloseIdx) {
		t.Errorf("extLst not positioned after <p:ph> and before </p:nvPr>: %s", out)
	}
}

func TestRenderShapeOmitsRoleMarkerWhenEmpty(t *testing.T) {
	sh := &Shape{
		Placeholder:    PlaceholderBody,
		PlaceholderIdx: 1,
		Paragraphs:     []*Paragraph{{Runs: []*Run{{Text: "body"}}}},
	}
	var rels []slideRel
	relIdx := 1
	out := renderShape(sh, 2, &relIdx, &rels)
	if strings.Contains(out, `<p:extLst>`) {
		t.Errorf("did not expect extLst for Role=\"\" shape, got: %s", out)
	}
}

func TestRenderRunExtendedCharacterProperties(t *testing.T) {
	tests := []struct {
		name         string
		run          *Run
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "highlight font family and superscript",
			run: &Run{
				Text:       "styled",
				BgColor:    "FFFF00",
				FontFamily: "Aptos",
				Baseline:   "super",
			},
			wantContains: []string{
				`baseline="30000"`,
				`<a:highlight><a:srgbClr val="FFFF00"/></a:highlight>`,
				`<a:latin typeface="Aptos"/>`,
			},
		},
		{
			name: "subscript",
			run: &Run{
				Text:     "sub",
				Baseline: "sub",
			},
			wantContains: []string{`baseline="-25000"`},
		},
		{
			name: "plain run unchanged",
			run:  &Run{Text: "plain"},
			wantContains: []string{
				`<a:r><a:rPr lang="en-US"/><a:t>plain</a:t></a:r>`,
			},
			wantMissing: []string{
				`baseline=`,
				`<a:highlight>`,
				`<a:latin`,
			},
		},
		{
			name: "explicit font family overrides code latin font",
			run: &Run{
				Text:       "code",
				Code:       true,
				FontFamily: "Courier New",
			},
			wantContains: []string{
				`<a:latin typeface="Courier New"/>`,
				`<a:cs typeface="Noto Sans Mono"/>`,
			},
			wantMissing: []string{
				`<a:latin typeface="Noto Sans Mono"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rels []slideRel
			relIdx := 1
			out := renderRun(tt.run, &relIdx, &rels)
			for _, want := range tt.wantContains {
				if !strings.Contains(out, want) {
					t.Errorf("expected output to contain %q, got: %s", want, out)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(out, missing) {
					t.Errorf("expected output not to contain %q, got: %s", missing, out)
				}
			}
		})
	}
}

func TestRenderTableStyledCells(t *testing.T) {
	tbl := &Table{
		Rows: []*TableRow{
			{Header: true, Cells: []*TableCell{
				tableCell("h1"),
				tableCell("h2"),
				tableCell("h3"),
			}},
			{Cells: []*TableCell{
				tableCell("d1"),
				tableCell("d2"),
				tableCell("d3"),
			}},
		},
		Style: &TableStyleSpec{
			HeaderFirstCol:  TableCellStyleSpec{BgColor: "AA0000", Bold: true, Color: "111111", FontFamily: "Aptos", HAlign: "ctr", VAlign: "ctr"},
			HeaderOtherCols: TableCellStyleSpec{BgColor: "00AA00", Italic: true, HAlign: "r", VAlign: "b"},
			DataFirstCol:    TableCellStyleSpec{BgColor: "0000AA"},
			DataOtherCols:   TableCellStyleSpec{BgColor: "AAAAAA"},

			OuterHorizontal: TableBorderSpec{Color: "101010", WidthEMU: 1111},
			OuterVertical:   TableBorderSpec{Color: "202020", WidthEMU: 2222},

			HeaderFirstColRight:  TableBorderSpec{Color: "303030", WidthEMU: 3333},
			HeaderFirstColBottom: TableBorderSpec{Color: "404040", WidthEMU: 4444},
			HeaderOtherColRight:  TableBorderSpec{Color: "505050", WidthEMU: 5555},
			HeaderOtherColBottom: TableBorderSpec{Color: "606060", WidthEMU: 6666},
			DataFirstColRight:    TableBorderSpec{Color: "707070", WidthEMU: 7777},
			DataFirstColBottom:   TableBorderSpec{Color: "808080", WidthEMU: 8888},
			DataOtherColRight:    TableBorderSpec{Color: "909090", WidthEMU: 9999},
			DataOtherColBottom:   TableBorderSpec{Color: "A0A0A0", WidthEMU: 1010},
		},
	}
	var rels []slideRel
	relIdx := 1
	cells := tableCellXMLs(renderTable(tbl, 2, &relIdx, &rels))
	if len(cells) != 6 {
		t.Fatalf("expected 6 cells, got %d", len(cells))
	}

	wantInCell := map[int][]string{
		0: {
			`<a:pPr algn="ctr"><a:buNone/></a:pPr>`,
			`<a:rPr lang="en-US" b="1"><a:solidFill><a:srgbClr val="111111"/></a:solidFill><a:latin typeface="Aptos"/></a:rPr>`,
			`<a:tcPr anchor="ctr">`,
			`<a:lnL w="2222"><a:solidFill><a:srgbClr val="202020"/></a:solidFill></a:lnL>`,
			`<a:lnR w="3333"><a:solidFill><a:srgbClr val="303030"/></a:solidFill></a:lnR>`,
			`<a:lnT w="1111"><a:solidFill><a:srgbClr val="101010"/></a:solidFill></a:lnT>`,
			`<a:lnB w="4444"><a:solidFill><a:srgbClr val="404040"/></a:solidFill></a:lnB>`,
			`<a:solidFill><a:srgbClr val="AA0000"/></a:solidFill>`,
		},
		1: {
			`<a:pPr algn="r"><a:buNone/></a:pPr>`,
			`<a:rPr lang="en-US" i="1"/>`,
			`<a:tcPr anchor="b">`,
			`<a:lnL w="3333"><a:solidFill><a:srgbClr val="303030"/></a:solidFill></a:lnL>`,
			`<a:lnR w="5555"><a:solidFill><a:srgbClr val="505050"/></a:solidFill></a:lnR>`,
			`<a:solidFill><a:srgbClr val="00AA00"/></a:solidFill>`,
		},
		2: {
			`<a:lnR w="2222"><a:solidFill><a:srgbClr val="202020"/></a:solidFill></a:lnR>`,
		},
		4: {
			`<a:lnT w="6666"><a:solidFill><a:srgbClr val="606060"/></a:solidFill></a:lnT>`,
			`<a:lnR w="9999"><a:solidFill><a:srgbClr val="909090"/></a:solidFill></a:lnR>`,
			`<a:solidFill><a:srgbClr val="AAAAAA"/></a:solidFill>`,
		},
	}
	for idx, wants := range wantInCell {
		for _, want := range wants {
			if !strings.Contains(cells[idx], want) {
				t.Errorf("cell %d: expected %q in %s", idx, want, cells[idx])
			}
		}
	}
}

func TestRenderTableNilStyleKeepsLegacyOutput(t *testing.T) {
	tbl := &Table{Rows: []*TableRow{
		{Header: true, Cells: []*TableCell{tableCell("h")}},
		{Cells: []*TableCell{tableCell("d")}},
	}}
	var rels []slideRel
	relIdx := 1
	cells := tableCellXMLs(renderTable(tbl, 2, &relIdx, &rels))
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
	legacyBorder := `<a:lnL w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnL><a:lnR w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnR><a:lnT w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnT><a:lnB w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnB>`
	if !strings.Contains(cells[0], `<a:tcPr>`+legacyBorder+`<a:solidFill><a:srgbClr val="D9E1F2"/></a:solidFill></a:tcPr>`) {
		t.Errorf("header cell did not keep legacy border and fill: %s", cells[0])
	}
	if !strings.Contains(cells[1], `<a:tcPr>`+legacyBorder+`</a:tcPr>`) {
		t.Errorf("data cell did not keep legacy border without fill: %s", cells[1])
	}
	if !strings.Contains(cells[0], `<a:rPr lang="en-US" b="1"/>`) {
		t.Errorf("header cell did not keep legacy bold: %s", cells[0])
	}
}

func tableCell(text string) *TableCell {
	return &TableCell{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: text}}}}}
}

func tableCellXMLs(out string) []string {
	parts := strings.Split(out, `<a:tc>`)
	cells := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		end := strings.Index(part, `</a:tc>`)
		if end < 0 {
			continue
		}
		cells = append(cells, `<a:tc>`+part[:end]+`</a:tc>`)
	}
	return cells
}
