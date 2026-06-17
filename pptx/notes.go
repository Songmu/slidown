package pptx

import "fmt"

// notesMaster1 is a minimal notes master defining the notes body text style.
func notesMaster1() string {
	return xmlDecl +
		`<p:notesMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder 1"/>` +
		`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="685800" y="4343400"/><a:ext cx="5486400" cy="3886200"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr/></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:notesStyle>` +
		notesStyleLevels() +
		`</p:notesStyle>` +
		`</p:notesMaster>`
}

func notesStyleLevels() string {
	indents := []int{0, 457200, 914400, 1371600, 1828800, 2286000, 2743200, 3200400, 3657600}
	var out string
	for i := 0; i < 9; i++ {
		out += `<a:lvl` + itoa(i+1) + `pPr marL="` + itoa(indents[i]) + `" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1">` +
			`<a:defRPr sz="1200" kern="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl` + itoa(i+1) + `pPr>`
	}
	return out
}

const notesMaster1Rels = xmlDecl +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
	`</Relationships>`

// notesSlideXML builds a notes slide part carrying the given note text.
func notesSlideXML(note string) string {
	return xmlDecl +
		`<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder 1"/>` +
		`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` +
		notesParagraphs(note) +
		`</p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:overrideClrMapping bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr>` +
		`</p:notes>`
}

func notesParagraphs(note string) string {
	if note == "" {
		return `<a:p><a:endParaRPr/></a:p>`
	}
	var out string
	start := 0
	for i := 0; i <= len(note); i++ {
		if i == len(note) || note[i] == '\n' {
			line := note[start:i]
			out += `<a:p><a:r><a:rPr lang="en-US"/><a:t>` + escapeXML(line) + `</a:t></a:r></a:p>`
			start = i + 1
		}
	}
	return out
}

// notesSlideRels builds the .rels for a notes slide, linking it to its slide
// and the notes master.
func notesSlideRels(slideNum int) string {
	return xmlDecl +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		fmt.Sprintf(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="../slides/slide%d.xml"/>`, slideNum) +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="../notesMasters/notesMaster1.xml"/>` +
		`</Relationships>`
}
