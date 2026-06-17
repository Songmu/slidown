package pptx

// theme1 is a standard, self-contained Office theme (color, font and format
// schemes). It is used as slidown's built-in default design.
const theme1 = xmlDecl +
	`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="slidown">` +
	`<a:themeElements>` +
	`<a:clrScheme name="slidown">` +
	`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
	`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
	`<a:dk2><a:srgbClr val="44546A"/></a:dk2>` +
	`<a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>` +
	`<a:accent1><a:srgbClr val="4472C4"/></a:accent1>` +
	`<a:accent2><a:srgbClr val="ED7D31"/></a:accent2>` +
	`<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3>` +
	`<a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
	`<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5>` +
	`<a:accent6><a:srgbClr val="70AD47"/></a:accent6>` +
	`<a:hlink><a:srgbClr val="0563C1"/></a:hlink>` +
	`<a:folHlink><a:srgbClr val="954F72"/></a:folHlink>` +
	`</a:clrScheme>` +
	`<a:fontScheme name="slidown">` +
	`<a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
	`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>` +
	`</a:fontScheme>` +
	`<a:fmtScheme name="slidown">` +
	`<a:fillStyleLst>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:gradFill rotWithShape="1"><a:gsLst>` +
	`<a:gs pos="0"><a:schemeClr val="phClr"><a:lumMod val="110000"/><a:satMod val="105000"/><a:tint val="67000"/></a:schemeClr></a:gs>` +
	`<a:gs pos="50000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="103000"/><a:tint val="73000"/></a:schemeClr></a:gs>` +
	`<a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="109000"/><a:tint val="81000"/></a:schemeClr></a:gs>` +
	`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
	`<a:gradFill rotWithShape="1"><a:gsLst>` +
	`<a:gs pos="0"><a:schemeClr val="phClr"><a:satMod val="103000"/><a:lumMod val="102000"/><a:tint val="94000"/></a:schemeClr></a:gs>` +
	`<a:gs pos="50000"><a:schemeClr val="phClr"><a:satMod val="110000"/><a:lumMod val="100000"/><a:shade val="100000"/></a:schemeClr></a:gs>` +
	`<a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="99000"/><a:satMod val="120000"/><a:shade val="78000"/></a:schemeClr></a:gs>` +
	`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
	`</a:fillStyleLst>` +
	`<a:lnStyleLst>` +
	`<a:ln w="6350" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
	`<a:ln w="12700" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
	`<a:ln w="19050" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
	`</a:lnStyleLst>` +
	`<a:effectStyleLst>` +
	`<a:effectStyle><a:effectLst/></a:effectStyle>` +
	`<a:effectStyle><a:effectLst/></a:effectStyle>` +
	`<a:effectStyle><a:effectLst/></a:effectStyle>` +
	`</a:effectStyleLst>` +
	`<a:bgFillStyleLst>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"><a:tint val="95000"/><a:satMod val="170000"/></a:schemeClr></a:solidFill>` +
	`<a:gradFill rotWithShape="1"><a:gsLst>` +
	`<a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="93000"/><a:satMod val="150000"/><a:shade val="98000"/><a:lumMod val="102000"/></a:schemeClr></a:gs>` +
	`<a:gs pos="50000"><a:schemeClr val="phClr"><a:tint val="98000"/><a:satMod val="130000"/><a:shade val="90000"/><a:lumMod val="103000"/></a:schemeClr></a:gs>` +
	`<a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="63000"/><a:satMod val="120000"/></a:schemeClr></a:gs>` +
	`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
	`</a:bgFillStyleLst>` +
	`</a:fmtScheme>` +
	`</a:themeElements>` +
	`</a:theme>`

// slideMaster1 defines the master slide, its color map, the layout list and the
// inherited text styles for title/body/other.
func slideMaster1() string {
	return xmlDecl +
		`<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld>` +
		`<p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		masterPlaceholder("2", "Title Placeholder 1", "title", "", 838200, 365125, 10515600, 1325563) +
		masterPlaceholder("3", "Text Placeholder 2", "body", "1", 838200, 1825625, 10515600, 4351338) +
		`</p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>` +
		`<p:txStyles>` +
		`<p:titleStyle>` +
		`<a:lvl1pPr algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">` +
		`<a:spcBef><a:spcPct val="0"/></a:spcBef><a:buNone/>` +
		`<a:defRPr sz="4400" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mj-lt"/></a:defRPr></a:lvl1pPr>` +
		`</p:titleStyle>` +
		`<p:bodyStyle>` +
		bodyStyleLevels() +
		`</p:bodyStyle>` +
		`<p:otherStyle>` +
		`<a:defPPr><a:defRPr lang="en-US"/></a:defPPr>` +
		`<a:lvl1pPr marL="0" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr>` +
		`</p:otherStyle>` +
		`</p:txStyles>` +
		`</p:sldMaster>`
}

// bodyStyleLevels emits nine indentation levels for the body text style.
func bodyStyleLevels() string {
	indents := []int{0, 457200, 914400, 1371600, 1828800, 2286000, 2743200, 3200400, 3657600}
	sizes := []int{2800, 2400, 2000, 1800, 1800, 1800, 1800, 1800, 1800}
	var out string
	for i := 0; i < 9; i++ {
		out += `<a:lvl` + itoa(i+1) + `pPr marL="` + itoa(indents[i]) + `" indent="-285750" algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">` +
			`<a:spcBef><a:spcPct val="20000"/></a:spcBef>` +
			`<a:buFont typeface="Arial" panose="020B0604020202020204" pitchFamily="34" charset="0"/>` +
			`<a:buChar char="&#8226;"/>` +
			`<a:defRPr sz="` + itoa(sizes[i]) + `" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl` + itoa(i+1) + `pPr>`
	}
	return out
}

// masterPlaceholder builds a placeholder shape definition for the master.
func masterPlaceholder(id, name, phType, idx string, x, y, w, h int64) string {
	ph := `<p:ph type="` + phType + `"`
	if idx != "" {
		ph += ` idx="` + idx + `"`
	}
	ph += `/>`
	return `<p:sp><p:nvSpPr>` +
		`<p:cNvPr id="` + id + `" name="` + name + `"/>` +
		`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr>` + ph + `</p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="` + itoa64(x) + `" y="` + itoa64(y) + `"/><a:ext cx="` + itoa64(w) + `" cy="` + itoa64(h) + `"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr/></a:p></p:txBody></p:sp>`
}

const slideMaster1Rels = xmlDecl +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
	`</Relationships>`

// slideLayout1 is the built-in "Title and Content" layout.
func slideLayout1() string {
	return xmlDecl +
		`<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="obj" preserve="1">` +
		`<p:cSld name="Title and Content">` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		layoutPlaceholder("2", "Title 1", "title", "", 0, 838200, 365125, 10515600, 1325563) +
		layoutPlaceholder("3", "Content Placeholder 2", "body", "1", 1, 838200, 1825625, 10515600, 4351338) +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:overrideClrMapping bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

// layoutPlaceholder builds a placeholder shape definition for a layout.
func layoutPlaceholder(id, name, phType, idx string, _ int, x, y, w, h int64) string {
	ph := `<p:ph type="` + phType + `"`
	if idx != "" {
		ph += ` idx="` + idx + `"`
	}
	ph += `/>`
	return `<p:sp><p:nvSpPr>` +
		`<p:cNvPr id="` + id + `" name="` + name + `"/>` +
		`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr>` + ph + `</p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="` + itoa64(x) + `" y="` + itoa64(y) + `"/><a:ext cx="` + itoa64(w) + `" cy="` + itoa64(h) + `"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr/></a:p></p:txBody></p:sp>`
}

const slideLayout1Rels = xmlDecl +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
	`</Relationships>`
