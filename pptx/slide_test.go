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

func TestRenderShapeOmitsRoleAttrWhenEmpty(t *testing.T) {
	sh := &Shape{
		Placeholder:    PlaceholderBody,
		PlaceholderIdx: 1,
		Paragraphs:     []*Paragraph{{Runs: []*Run{{Text: "body"}}}},
	}
	var rels []slideRel
	relIdx := 1
	out := renderShape(sh, 2, &relIdx, &rels)
	// The shape carries no role, but the meta extension is still emitted to
	// carry the per-shape fingerprint used by incremental shape-level rebuilds.
	if strings.Contains(out, `role=`) {
		t.Errorf("did not expect a role attribute for Role=\"\" shape, got: %s", out)
	}
	if !strings.Contains(out, `<slidown:shape`) || !strings.Contains(out, ` fp="`) {
		t.Errorf("expected shape meta extension carrying fp, got: %s", out)
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
			{Cells: []*TableCell{
				tableCell("d4"),
				tableCell("d5"),
				tableCell("d6"),
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
	if len(cells) != 9 {
		t.Fatalf("expected 9 cells, got %d", len(cells))
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
			`<a:lnB w="6666"><a:solidFill><a:srgbClr val="606060"/></a:solidFill></a:lnB>`,
			`<a:solidFill><a:srgbClr val="00AA00"/></a:solidFill>`,
		},
		2: {
			`<a:lnR w="2222"><a:solidFill><a:srgbClr val="202020"/></a:solidFill></a:lnR>`,
		},
		3: {
			`<a:lnR w="7777"><a:solidFill><a:srgbClr val="707070"/></a:solidFill></a:lnR>`,
			`<a:lnT w="4444"><a:solidFill><a:srgbClr val="404040"/></a:solidFill></a:lnT>`,
			`<a:lnB w="8888"><a:solidFill><a:srgbClr val="808080"/></a:solidFill></a:lnB>`,
			`<a:solidFill><a:srgbClr val="0000AA"/></a:solidFill>`,
		},
		4: {
			`<a:lnL w="7777"><a:solidFill><a:srgbClr val="707070"/></a:solidFill></a:lnL>`,
			`<a:lnT w="6666"><a:solidFill><a:srgbClr val="606060"/></a:solidFill></a:lnT>`,
			`<a:lnR w="9999"><a:solidFill><a:srgbClr val="909090"/></a:solidFill></a:lnR>`,
			`<a:lnB w="1010"><a:solidFill><a:srgbClr val="A0A0A0"/></a:solidFill></a:lnB>`,
			`<a:solidFill><a:srgbClr val="AAAAAA"/></a:solidFill>`,
		},
		6: {
			`<a:lnT w="8888"><a:solidFill><a:srgbClr val="808080"/></a:solidFill></a:lnT>`,
			`<a:lnB w="1111"><a:solidFill><a:srgbClr val="101010"/></a:solidFill></a:lnB>`,
		},
		7: {
			`<a:lnT w="1010"><a:solidFill><a:srgbClr val="A0A0A0"/></a:solidFill></a:lnT>`,
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

func TestRenderTableStyledPreservesInlineEmphasis(t *testing.T) {
	boldRunCell := func(text string) *TableCell {
		return &TableCell{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: text, Bold: true}}}}}
	}
	tbl := &Table{
		Rows: []*TableRow{
			{Header: true, Cells: []*TableCell{tableCell("h1"), tableCell("h2")}},
			// A data cell whose run is inline-bold, under a region style with
			// Bold:false. The style must not strip the inline bold.
			{Cells: []*TableCell{boldRunCell("d1"), tableCell("d2")}},
		},
		Style: &TableStyleSpec{
			HeaderFirstCol:  TableCellStyleSpec{Bold: true},
			HeaderOtherCols: TableCellStyleSpec{Bold: true},
			DataFirstCol:    TableCellStyleSpec{}, // Bold:false
			DataOtherCols:   TableCellStyleSpec{},
		},
	}
	var rels []slideRel
	relIdx := 1
	cells := tableCellXMLs(renderTable(tbl, 2, &relIdx, &rels))
	// cell index 2 is the first data cell (row 1, col 0).
	if !strings.Contains(cells[2], `b="1"`) {
		t.Errorf("inline bold in data cell was stripped by region style: %s", cells[2])
	}
	// Header cells become bold from the region style even without inline bold.
	if !strings.Contains(cells[0], `b="1"`) {
		t.Errorf("header cell should be bold from region style: %s", cells[0])
	}
}

func TestRenderTableCellMutatesDefaultsInPlace(t *testing.T) {
	tests := []struct {
		name      string
		style     *TableStyleSpec
		header    bool
		cellAlign Alignment
		wantAlign Alignment
		wantRuns  []Run
	}{
		{
			name: "region defaults preserve inline overrides",
			style: &TableStyleSpec{DataFirstCol: TableCellStyleSpec{
				Bold: true, Color: "112233", FontFamily: "Aptos", HAlign: "ctr",
			}},
			wantAlign: AlignCenter,
			wantRuns: []Run{
				{Bold: true, Color: "112233", FontFamily: "Aptos"},
				{Bold: true, Italic: true, Color: "AABBCC", FontFamily: "Custom"},
				{Bold: true, Code: true, Color: "112233"},
			},
		},
		{
			name: "cell alignment overrides region and style adds italic",
			style: &TableStyleSpec{HeaderFirstCol: TableCellStyleSpec{
				Italic: true, HAlign: "ctr",
			}},
			header: true, cellAlign: AlignRight, wantAlign: AlignRight,
			wantRuns: []Run{
				{Italic: true},
				{Bold: true, Italic: true, Color: "AABBCC", FontFamily: "Custom"},
				{Italic: true, Code: true},
			},
		},
		{
			name:   "legacy header defaults",
			header: true, cellAlign: AlignCenter, wantAlign: AlignCenter,
			wantRuns: []Run{
				{Bold: true},
				{Bold: true, Italic: true, Color: "AABBCC", FontFamily: "Custom"},
				{Bold: true, Code: true},
			},
		},
		{
			name:   "empty style suppresses legacy header bold",
			header: true, style: &TableStyleSpec{},
			wantRuns: []Run{
				{},
				{Bold: true, Italic: true, Color: "AABBCC", FontFamily: "Custom"},
				{Code: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := []*Run{
				{},
				{Bold: true, Italic: true, Color: "AABBCC", FontFamily: "Custom"},
				{Code: true},
			}
			p := &Paragraph{Runs: runs}
			explicit := &Paragraph{Align: AlignLeft}
			cell := &TableCell{Align: tt.cellAlign, Paragraphs: []*Paragraph{p, explicit}}
			relIdx := 1
			var rels []slideRel
			renderTableCell(cell, tt.header, tt.style, 0, 0, 1, 1, []bool{tt.header}, &relIdx, &rels)
			if cell.Paragraphs[0] != p || p.Align != tt.wantAlign || explicit.Align != AlignLeft {
				t.Errorf("paragraph defaults or identity changed: %+v, %+v", p, explicit)
			}
			for i, run := range runs {
				if p.Runs[i] != run || *run != tt.wantRuns[i] {
					t.Errorf("run %d = %+v, want %+v in place", i, *run, tt.wantRuns[i])
				}
			}
		})
	}
}

func TestRenderTableCellEmptyParagraphsExactXML(t *testing.T) {
	style := &TableStyleSpec{
		HeaderFirstCol:  TableCellStyleSpec{VAlign: "ctr", BgColor: "AABBCC"},
		OuterVertical:   TableBorderSpec{None: true},
		OuterHorizontal: TableBorderSpec{Color: "112233", WidthEMU: 1200, Dash: "dash"},
	}
	want := `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr/></a:p></a:txBody>` +
		`<a:tcPr anchor="ctr"><a:lnL><a:noFill/></a:lnL><a:lnR><a:noFill/></a:lnR>` +
		`<a:lnT w="1200"><a:solidFill><a:srgbClr val="112233"/></a:solidFill><a:prstDash val="dash"/></a:lnT>` +
		`<a:lnB w="1200"><a:solidFill><a:srgbClr val="112233"/></a:solidFill><a:prstDash val="dash"/></a:lnB>` +
		`<a:solidFill><a:srgbClr val="AABBCC"/></a:solidFill></a:tcPr></a:tc>`
	for _, cell := range []*TableCell{nil, {}, {Paragraphs: []*Paragraph{}}} {
		relIdx := 7
		var rels []slideRel
		got := renderTableCell(cell, true, style, 0, 0, 1, 1, []bool{true}, &relIdx, &rels)
		if got != want {
			t.Errorf("cell %+v XML = %s, want %s", cell, got, want)
		}
		if relIdx != 7 || rels != nil {
			t.Errorf("empty cell changed relationships: %d, %+v", relIdx, rels)
		}
	}
}

func TestRenderTableBorderWidthOnlyNoColor(t *testing.T) {
	got := renderTableBorder("lnL", TableBorderSpec{WidthEMU: 1234})
	if strings.Contains(got, `srgbClr`) {
		t.Errorf("width-only border must not emit a color: %s", got)
	}
	if got != `<a:lnL w="1234"/>` {
		t.Errorf("unexpected width-only border: %s", got)
	}
	if empty := renderTableBorder("lnR", TableBorderSpec{}); empty != "" {
		t.Errorf("empty spec should render nothing, got %s", empty)
	}
}
