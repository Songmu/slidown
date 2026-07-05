package pptx

import "testing"

func TestLoadStyleLayoutParsesSyntaxStylesAndTableStyle(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld name="style">
    <p:spTree>
      <p:sp>
        <p:txBody><a:p><a:r>
          <a:rPr b="1" i="true" u="sng" strike="sngStrike" baseline="-25000">
            <a:solidFill><a:srgbClr val="112233"/></a:solidFill>
            <a:highlight><a:srgbClr val="FFEEDD"/></a:highlight>
            <a:latin typeface="Menlo"/>
          </a:rPr>
          <a:t>code</a:t>
        </a:r></a:p></p:txBody>
      </p:sp>
      <p:sp>
        <p:txBody><a:p><a:r><a:rPr baseline="30000"/><a:t>sup</a:t></a:r></a:p></p:txBody>
      </p:sp>
      <p:sp>
        <p:txBody><a:p><a:r><a:rPr b="1"/><a:t>   </a:t></a:r></a:p></p:txBody>
      </p:sp>
      <p:graphicFrame>
        <a:graphic><a:graphicData><a:tbl>
          <a:tr>
            <a:tc>
              <a:txBody><a:p><a:pPr algn="ctr"/><a:r><a:rPr b="1"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:latin typeface="Arial"/></a:rPr><a:t>H1</a:t></a:r></a:p></a:txBody>
              <a:tcPr anchor="ctr">
                <a:solidFill><a:srgbClr val="AAAAAA"/></a:solidFill>
                <a:lnL w="100"><a:solidFill><a:srgbClr val="111111"/></a:solidFill><a:prstDash val="solid"/></a:lnL>
                <a:lnT w="200"><a:solidFill><a:srgbClr val="222222"/></a:solidFill></a:lnT>
                <a:lnR w="300"><a:solidFill><a:srgbClr val="333333"/></a:solidFill></a:lnR>
                <a:lnB w="400"><a:noFill/></a:lnB>
              </a:tcPr>
            </a:tc>
            <a:tc>
              <a:txBody><a:p><a:pPr algn="r"/><a:r><a:rPr i="1"><a:solidFill><a:srgbClr val="666666"/></a:solidFill><a:latin typeface="Calibri"/></a:rPr><a:t>H2</a:t></a:r></a:p></a:txBody>
              <a:tcPr anchor="t">
                <a:solidFill><a:srgbClr val="BBBBBB"/></a:solidFill>
                <a:lnR w="500"><a:solidFill><a:srgbClr val="444444"/></a:solidFill><a:prstDash val="dash"/></a:lnR>
                <a:lnB w="600"><a:solidFill><a:srgbClr val="555555"/></a:solidFill></a:lnB>
              </a:tcPr>
            </a:tc>
          </a:tr>
          <a:tr>
            <a:tc>
              <a:txBody><a:p><a:r><a:rPr/><a:t>D1</a:t></a:r></a:p></a:txBody>
              <a:tcPr><a:lnR w="700"><a:noFill/></a:lnR><a:lnB w="800"><a:solidFill><a:srgbClr val="777777"/></a:solidFill></a:lnB></a:tcPr>
            </a:tc>
            <a:tc>
              <a:txBody><a:p><a:r><a:rPr/><a:t>D2</a:t></a:r></a:p></a:txBody>
              <a:tcPr><a:lnR w="900"><a:solidFill><a:srgbClr val="888888"/></a:solidFill></a:lnR><a:lnB w="1000"><a:solidFill><a:srgbClr val="999999"/></a:solidFill></a:lnB></a:tcPr>
            </a:tc>
          </a:tr>
        </a:tbl></a:graphicData></a:graphic>
      </p:graphicFrame>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`)

	tmpl := &Template{}
	tmpl.loadStyleLayout(xmlData)

	code, ok := tmpl.SyntaxStyles["code"]
	if !ok {
		t.Fatal("SyntaxStyles missing code")
	}
	if !code.Bold || !code.Italic || !code.Underline || !code.Strike {
		t.Errorf("code bool styles = %+v, want all true", code)
	}
	if code.Color != "112233" || code.BgColor != "FFEEDD" || code.FontFamily != "Menlo" || code.Baseline != "sub" {
		t.Errorf("code style = %+v, want colors/font/baseline parsed", code)
	}
	if got := tmpl.SyntaxStyles["sup"].Baseline; got != "super" {
		t.Errorf("sup baseline = %q, want super", got)
	}
	if _, ok := tmpl.SyntaxStyles[""]; ok {
		t.Error("empty keyword should be skipped")
	}

	ts := tmpl.TableStyle
	if ts == nil {
		t.Fatal("TableStyle is nil")
	}
	if ts.HeaderFirstCol.BgColor != "AAAAAA" || !ts.HeaderFirstCol.Bold || ts.HeaderFirstCol.Color != "FFFFFF" || ts.HeaderFirstCol.FontFamily != "Arial" || ts.HeaderFirstCol.HAlign != "ctr" || ts.HeaderFirstCol.VAlign != "ctr" {
		t.Errorf("HeaderFirstCol = %+v, want parsed fill/text/alignment", ts.HeaderFirstCol)
	}
	if !ts.HeaderOtherCols.Italic || ts.HeaderOtherCols.HAlign != "r" || ts.HeaderOtherCols.VAlign != "t" {
		t.Errorf("HeaderOtherCols = %+v, want italic and alignment", ts.HeaderOtherCols)
	}
	if ts.OuterHorizontal.Color != "222222" || ts.OuterHorizontal.WidthEMU != 200 {
		t.Errorf("OuterHorizontal = %+v, want top border from cell[0,0]", ts.OuterHorizontal)
	}
	if ts.OuterVertical.Color != "111111" || ts.OuterVertical.WidthEMU != 100 || ts.OuterVertical.Dash != "solid" {
		t.Errorf("OuterVertical = %+v, want left border from cell[0,0]", ts.OuterVertical)
	}
	if ts.HeaderFirstColRight.Color != "333333" || ts.HeaderFirstColBottom.WidthEMU != 400 || !ts.HeaderFirstColBottom.None {
		t.Errorf("header first-col borders = right %+v bottom %+v", ts.HeaderFirstColRight, ts.HeaderFirstColBottom)
	}
	if ts.HeaderOtherColRight.Color != "444444" || ts.HeaderOtherColRight.Dash != "dash" || ts.HeaderOtherColBottom.Color != "555555" {
		t.Errorf("header other-col borders = right %+v bottom %+v", ts.HeaderOtherColRight, ts.HeaderOtherColBottom)
	}
	if !ts.DataFirstColRight.None || ts.DataFirstColBottom.Color != "777777" || ts.DataOtherColRight.Color != "888888" || ts.DataOtherColBottom.Color != "999999" {
		t.Errorf("data borders = first right %+v first bottom %+v other right %+v other bottom %+v", ts.DataFirstColRight, ts.DataFirstColBottom, ts.DataOtherColRight, ts.DataOtherColBottom)
	}
}

func TestParseTableStyleRequiresTwoByTwoTable(t *testing.T) {
	xmlData := []byte(`<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld name="style"><p:spTree><p:graphicFrame><a:graphic><a:graphicData><a:tbl>
    <a:tr><a:tc><a:txBody/></a:tc></a:tr>
  </a:tbl></a:graphicData></a:graphic></p:graphicFrame></p:spTree></p:cSld>
</p:sldLayout>`)
	tmpl := &Template{}
	tmpl.loadStyleLayout(xmlData)
	if tmpl.TableStyle != nil {
		t.Fatalf("TableStyle = %+v, want nil for non-2x2 table", tmpl.TableStyle)
	}
}
