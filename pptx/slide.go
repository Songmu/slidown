package pptx

import (
	"fmt"
	"strings"
)

// slideRel represents a relationship referenced from a slide part (currently
// only hyperlinks).
type slideRel struct {
	id     string
	relTyp string
	target string
	mode   string // "External" for hyperlinks
}

// renderSlide serializes a slide to its slide XML part and returns the XML plus
// any relationships it references.
func renderSlide(s *Slide) (xml string, rels []slideRel) {
	var b strings.Builder
	b.WriteString(xmlDecl)
	b.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`)
	b.WriteString(`<p:cSld><p:spTree>`)
	b.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	b.WriteString(`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)

	relIdx := 1 // rId1 is reserved for the slide layout relationship
	id := 2     // shape ids; 1 is the group
	for _, sh := range s.Shapes {
		b.WriteString(renderShape(sh, id, &relIdx, &rels))
		id++
	}

	b.WriteString(`</p:spTree></p:cSld>`)
	b.WriteString(`<p:clrMapOvr><a:overrideClrMapping bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr>`)
	b.WriteString(`</p:sld>`)
	return b.String(), rels
}

func renderShape(sh *Shape, id int, relIdx *int, rels *[]slideRel) string {
	var b strings.Builder
	name := sh.Name
	if name == "" {
		name = fmt.Sprintf("Shape %d", id)
	}
	b.WriteString(`<p:sp><p:nvSpPr>`)
	b.WriteString(fmt.Sprintf(`<p:cNvPr id="%d" name="%s"/>`, id, escapeXML(name)))
	b.WriteString(`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>`)
	b.WriteString(`<p:nvPr>`)
	if sh.Placeholder != PlaceholderNone {
		ph := fmt.Sprintf(`<p:ph type="%s"`, sh.Placeholder)
		if sh.PlaceholderIdx > 0 {
			ph += fmt.Sprintf(` idx="%d"`, sh.PlaceholderIdx)
		}
		ph += `/>`
		b.WriteString(ph)
	}
	b.WriteString(`</p:nvPr></p:nvSpPr>`)

	b.WriteString(`<p:spPr>`)
	if sh.W > 0 || sh.H > 0 || sh.X > 0 || sh.Y > 0 {
		b.WriteString(fmt.Sprintf(`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, sh.X, sh.Y, sh.W, sh.H))
	}
	if sh.Placeholder == PlaceholderNone {
		b.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`)
	}
	b.WriteString(`</p:spPr>`)

	b.WriteString(`<p:txBody><a:bodyPr/><a:lstStyle/>`)
	if len(sh.Paragraphs) == 0 {
		b.WriteString(`<a:p><a:endParaRPr/></a:p>`)
	}
	for _, p := range sh.Paragraphs {
		b.WriteString(renderParagraph(p, relIdx, rels))
	}
	b.WriteString(`</p:txBody></p:sp>`)
	return b.String()
}

func renderParagraph(p *Paragraph, relIdx *int, rels *[]slideRel) string {
	var b strings.Builder
	b.WriteString(`<a:p>`)
	// Paragraph properties.
	var pPr strings.Builder
	if p.Level > 0 {
		pPr.WriteString(fmt.Sprintf(` lvl="%d"`, p.Level))
	}
	if p.Align != AlignNone {
		pPr.WriteString(fmt.Sprintf(` algn="%s"`, p.Align))
	}
	var bullet string
	switch {
	case p.Bullet && p.Numbered:
		bullet = `<a:buFont typeface="+mj-lt"/><a:buAutoNum type="arabicPeriod"/>`
	case p.Bullet:
		bullet = `<a:buFont typeface="Arial" panose="020B0604020202020204" pitchFamily="34" charset="0"/><a:buChar char="&#8226;"/>`
	default:
		bullet = `<a:buNone/>`
	}
	b.WriteString(`<a:pPr` + pPr.String() + `>` + bullet + `</a:pPr>`)

	for _, r := range p.Runs {
		b.WriteString(renderRun(r, relIdx, rels))
	}
	b.WriteString(`</a:p>`)
	return b.String()
}

func renderRun(r *Run, relIdx *int, rels *[]slideRel) string {
	var rPr strings.Builder
	rPr.WriteString(`<a:rPr lang="en-US"`)
	if r.Bold {
		rPr.WriteString(` b="1"`)
	}
	if r.Italic {
		rPr.WriteString(` i="1"`)
	}
	if r.Underline {
		rPr.WriteString(` u="sng"`)
	}
	if r.Strike {
		rPr.WriteString(` strike="sngStrike"`)
	}
	if r.FontSize > 0 {
		rPr.WriteString(fmt.Sprintf(` sz="%d"`, int(r.FontSize*100)))
	}
	rPr.WriteString(`>`)

	var inner strings.Builder
	if r.Color != "" {
		inner.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, escapeXML(r.Color)))
	}
	if r.Link != "" {
		*relIdx++
		rid := fmt.Sprintf("rId%d", *relIdx)
		*rels = append(*rels, slideRel{
			id:     rid,
			relTyp: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink",
			target: r.Link,
			mode:   "External",
		})
		inner.WriteString(fmt.Sprintf(`<a:hlinkClick xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="%s"/>`, rid))
	}
	if r.Code {
		inner.WriteString(`<a:latin typeface="Consolas" pitchFamily="49" charset="0"/><a:cs typeface="Consolas"/>`)
	}

	hasInner := inner.Len() > 0
	var rPrStr string
	if hasInner {
		rPrStr = rPr.String() + inner.String() + `</a:rPr>`
	} else {
		// self-close: replace trailing '>' with '/>'
		s := rPr.String()
		rPrStr = s[:len(s)-1] + `/>`
	}
	return `<a:r>` + rPrStr + `<a:t>` + escapeXML(r.Text) + `</a:t></a:r>`
}

// slideRelsXML builds the slide's .rels part from its relationships.
func slideRelsXML(rels []slideRel) string {
	var b strings.Builder
	b.WriteString(xmlDecl)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, rel := range rels {
		if rel.mode != "" {
			b.WriteString(fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s" TargetMode="%s"/>`,
				rel.id, rel.relTyp, escapeXML(rel.target), rel.mode))
		} else {
			b.WriteString(fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"/>`,
				rel.id, rel.relTyp, escapeXML(rel.target)))
		}
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}
