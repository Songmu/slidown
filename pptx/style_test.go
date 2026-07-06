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

func TestLoadStyleLayoutResolvesThemeSchemeAndSystemColors(t *testing.T) {
	const layoutPart = "ppt/slideLayouts/slideLayout9.xml"

	tmpl := &Template{
		designParts: map[string][]byte{
			relsPath(layoutPart): []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`),
			"ppt/slideMasters/slideMaster1.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
</p:sldMaster>`),
			relsPath("ppt/slideMasters/slideMaster1.xml"): []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`),
			"ppt/theme/theme1.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:themeElements>
    <a:clrScheme name="custom">
      <a:dk1><a:srgbClr val="101112"/></a:dk1>
      <a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="202122"/></a:dk2>
      <a:lt2><a:srgbClr val="EEEEEE"/></a:lt2>
      <a:accent1><a:srgbClr val="AA0001"/></a:accent1>
      <a:accent2><a:srgbClr val="AA0002"/></a:accent2>
      <a:accent3><a:sysClr val="windowText" lastClr="AA0003"/></a:accent3>
      <a:accent4><a:srgbClr val="AA0004"/></a:accent4>
      <a:accent5><a:srgbClr val="AA0005"/></a:accent5>
      <a:accent6><a:srgbClr val="AA0006"/></a:accent6>
      <a:hlink><a:srgbClr val="AA00AA"/></a:hlink>
      <a:folHlink><a:srgbClr val="00AA00"/></a:folHlink>
    </a:clrScheme>
  </a:themeElements>
</a:theme>`),
		},
	}

	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld name="style">
    <p:spTree>
      <p:sp>
        <p:txBody><a:p><a:r>
          <a:rPr>
            <a:solidFill><a:schemeClr val="accent1"/></a:solidFill>
            <a:highlight><a:schemeClr val="tx1"/></a:highlight>
          </a:rPr>
          <a:t>code</a:t>
        </a:r></a:p></p:txBody>
      </p:sp>
      <p:sp>
        <p:txBody><a:p><a:r>
          <a:rPr><a:solidFill><a:sysClr val="windowText" lastClr="ABCDEF"/></a:solidFill></a:rPr>
          <a:t>sys</a:t>
        </a:r></a:p></p:txBody>
      </p:sp>
      <p:graphicFrame>
        <a:graphic><a:graphicData><a:tbl>
          <a:tr>
            <a:tc>
              <a:txBody><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="accent3"/></a:solidFill></a:rPr><a:t>H1</a:t></a:r></a:p></a:txBody>
              <a:tcPr>
                <a:solidFill><a:schemeClr val="accent2"/></a:solidFill>
                <a:lnT w="100"><a:solidFill><a:srgbClr val="111111"/></a:solidFill></a:lnT>
                <a:lnL w="100"><a:solidFill><a:srgbClr val="222222"/></a:solidFill></a:lnL>
                <a:lnR w="100"><a:solidFill><a:schemeClr val="accent4"/></a:solidFill></a:lnR>
                <a:lnB w="100"><a:solidFill><a:srgbClr val="444444"/></a:solidFill></a:lnB>
              </a:tcPr>
            </a:tc>
            <a:tc><a:txBody><a:p><a:r><a:rPr/><a:t>H2</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>
          </a:tr>
          <a:tr>
            <a:tc><a:txBody><a:p><a:r><a:rPr/><a:t>D1</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>
            <a:tc><a:txBody><a:p><a:r><a:rPr/><a:t>D2</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>
          </a:tr>
        </a:tbl></a:graphicData></a:graphic>
      </p:graphicFrame>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`)

	tmpl.loadStyleLayoutPart(layoutPart, xmlData)

	if got := tmpl.SyntaxStyles["code"].Color; got != "AA0001" {
		t.Fatalf("code color = %q, want AA0001", got)
	}
	if got := tmpl.SyntaxStyles["code"].BgColor; got != "101112" {
		t.Fatalf("code highlight = %q, want mapped tx1->dk1 color 101112", got)
	}
	if got := tmpl.SyntaxStyles["sys"].Color; got != "ABCDEF" {
		t.Fatalf("sys color = %q, want sysClr lastClr", got)
	}
	if got := tmpl.TableStyle.HeaderFirstCol.BgColor; got != "AA0002" {
		t.Fatalf("table header fill = %q, want AA0002", got)
	}
	if got := tmpl.TableStyle.HeaderFirstCol.Color; got != "AA0003" {
		t.Fatalf("table header text = %q, want accent3 sysClr lastClr AA0003", got)
	}
	if got := tmpl.TableStyle.HeaderFirstColRight.Color; got != "AA0004" {
		t.Fatalf("table border color = %q, want AA0004", got)
	}
}
