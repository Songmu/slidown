package pptx

import (
	"fmt"
	"math"
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

// mediaPart is a binary media file (e.g. an image) destined for ppt/media.
type mediaPart struct {
	name string // file name within ppt/media, e.g. "image1.png"
	data []byte
}

// renderSlide serializes a slide to its slide XML part and returns the XML plus
// any relationships and media parts it references. mediaIdx is a shared counter
// used to assign globally-unique media file names across all slides.
func renderSlide(s *Slide, mediaIdx *int) (xml string, rels []slideRel, media []mediaPart) {
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
	// Z-order is grouped by element type, not markdown insertion order: all
	// shapes, then pictures, then SVG shape groups, then tables. slidown lays
	// images out side by side (non-overlapping) and offers no way to stack them,
	// so this type-ordered drawing is intentional and does not cause overlap
	// issues. Revisit only if free-positioning/overlap support is added.
	for _, sh := range s.Shapes {
		b.WriteString(renderShape(sh, id, &relIdx, &rels))
		id++
	}
	for _, pic := range s.Pictures {
		b.WriteString(renderPicture(pic, id, &relIdx, &rels, mediaIdx, &media))
		id++
	}
	for _, g := range s.Groups {
		b.WriteString(renderGroup(g, &id, &relIdx, &rels))
	}
	for _, tbl := range s.Tables {
		b.WriteString(renderTable(tbl, id, &relIdx, &rels))
		id++
	}

	b.WriteString(`</p:spTree></p:cSld>`)
	b.WriteString(fingerprintExt(s.Fingerprint, s.Key))
	b.WriteString(`</p:sld>`)
	return b.String(), rels, media
}

// fingerprintNS / fingerprintURI identify slidown's per-slide source
// fingerprint extension embedded in the slide's extLst. slidown reads this back
// on an incremental rebuild to decide whether a slide's source changed (see
// Slide.Fingerprint in the root package) and to match slides by key across
// inserts/reordering. It is stored in the OOXML extension list so it is
// invisible to the presentation and preserved verbatim when an unchanged slide
// is reused. Tools that drop unknown extensions simply cause the affected slide
// to be regenerated, which is harmless.
const (
	fingerprintNS  = "https://github.com/Songmu/slidown/ns"
	fingerprintURI = "{6F2A3B40-5C7D-4E21-9A6B-1D3F8C0E7B92}"
	// shapeMetaURI identifies a per-shape slidown metadata extension carried
	// inside the shape's <p:nvPr>. Currently used to record a semantic Role
	// (e.g. "subTitle") on shapes whose underlying placeholder type is
	// repurposed (e.g. an ordinary body placeholder used as a subtitle target
	// via the layout's subtitle hint).
	shapeMetaURI = "{A3F7C812-9B4D-4E16-83CA-2D7F1E9B4C58}"
)

// fingerprintExt renders the slide-level extLst carrying the source fingerprint
// and optional stable key, or an empty string when neither is set. The key may
// be present without a fingerprint for slides that carry only an identity (e.g.
// a slide imported from another presentation that a later key-stamping pass
// tags so rebuilds can match it by key).
func fingerprintExt(fp, key string) string {
	if fp == "" && key == "" {
		return ""
	}
	attrs := ` xmlns:slidown="` + fingerprintNS + `"`
	if fp != "" {
		attrs += ` v="` + escapeXML(fp) + `"`
	}
	if key != "" {
		attrs += ` k="` + escapeXML(key) + `"`
	}
	return `<p:extLst><p:ext uri="` + fingerprintURI + `">` +
		`<slidown:fp` + attrs + `/>` +
		`</p:ext></p:extLst>`
}

// shapeMetaExt renders the per-shape slidown extension carrying the shape's
// semantic Role (e.g. "subTitle"), content fingerprint (fp), and optional
// stable shape key (sk). The fingerprint lets an incremental rebuild detect
// whether a shape's source changed so unchanged shapes can be preserved
// individually; the key lets non-placeholder text shapes be recognized across
// rebuilds. Returns an empty string when all attributes are empty so callers can
// drop it into the XML unconditionally.
func shapeMetaExt(role, fp, sk string) string {
	elem := shapeMetaElem(role, fp, sk)
	if elem == "" {
		return ""
	}
	return `<p:extLst><p:ext uri="` + shapeMetaURI + `">` + elem + `</p:ext></p:extLst>`
}

func shapeMetaElem(role, fp, sk string) string {
	if role == "" && fp == "" && sk == "" {
		return ""
	}
	attrs := ""
	if role != "" {
		attrs += ` role="` + escapeXML(role) + `"`
	}
	if fp != "" {
		attrs += ` fp="` + escapeXML(fp) + `"`
	}
	if sk != "" {
		attrs += ` sk="` + escapeXML(sk) + `"`
	}
	return `<slidown:shape xmlns:slidown="` + fingerprintNS + `"` + attrs + `/>`
}

func renderPicture(pic *Picture, id int, relIdx *int, rels *[]slideRel, mediaIdx *int, media *[]mediaPart) string {
	ext := pic.Ext
	if ext == "" {
		ext = "png"
	}
	*mediaIdx++
	fileName := fmt.Sprintf("image%d.%s", *mediaIdx, ext)
	*media = append(*media, mediaPart{name: fileName, data: pic.Data})

	*relIdx++
	embedID := fmt.Sprintf("rId%d", *relIdx)
	*rels = append(*rels, slideRel{
		id:     embedID,
		relTyp: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
		target: "../media/" + fileName,
	})
	blip := `<a:blip r:embed="` + embedID + `"/>`
	if pic.SVGData != nil {
		*mediaIdx++
		svgFileName := fmt.Sprintf("image%d.svg", *mediaIdx)
		*media = append(*media, mediaPart{name: svgFileName, data: pic.SVGData})

		*relIdx++
		svgEmbedID := fmt.Sprintf("rId%d", *relIdx)
		*rels = append(*rels, slideRel{
			id:     svgEmbedID,
			relTyp: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			target: "../media/" + svgFileName,
		})
		blip = `<a:blip r:embed="` + embedID + `"><a:extLst><a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">` +
			`<asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main" r:embed="` + svgEmbedID + `"/>` +
			`</a:ext></a:extLst></a:blip>`
	}

	var linkAttr string
	if pic.Link != "" {
		*relIdx++
		linkID := fmt.Sprintf("rId%d", *relIdx)
		*rels = append(*rels, slideRel{
			id:     linkID,
			relTyp: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink",
			target: pic.Link,
			mode:   "External",
		})
		linkAttr = fmt.Sprintf(`<a:hlinkClick r:id="%s"/>`, linkID)
	}

	name := pic.Name
	if name == "" {
		name = fmt.Sprintf("Picture %d", id)
	}

	return `<p:pic><p:nvPicPr>` +
		fmt.Sprintf(`<p:cNvPr id="%d" name="%s">`, id, escapeXML(name)) + linkAttr + `</p:cNvPr>` +
		`<p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr>` + picNvPr(pic) + `</p:nvPicPr>` +
		`<p:blipFill>` + blip + `<a:stretch><a:fillRect/></a:stretch></p:blipFill>` +
		`<p:spPr><a:xfrm><a:off x="` + itoa64(pic.X) + `" y="` + itoa64(pic.Y) + `"/>` +
		`<a:ext cx="` + itoa64(pic.W) + `" cy="` + itoa64(pic.H) + `"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`
}

func renderGroup(g *GroupShape, id *int, relIdx *int, rels *[]slideRel) string {
	groupID := *id
	*id++
	name := g.Name
	if name == "" {
		name = fmt.Sprintf("Group %d", groupID)
	}

	var b strings.Builder
	b.WriteString(`<p:grpSp><p:nvGrpSpPr>`)
	b.WriteString(fmt.Sprintf(`<p:cNvPr id="%d" name="%s"/>`, groupID, escapeXML(name)))
	b.WriteString(`<p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>`)
	b.WriteString(`<p:grpSpPr><a:xfrm>`)
	b.WriteString(`<a:off x="` + itoa64(g.X) + `" y="` + itoa64(g.Y) + `"/>`)
	b.WriteString(`<a:ext cx="` + itoa64(g.W) + `" cy="` + itoa64(g.H) + `"/>`)
	b.WriteString(`<a:chOff x="` + itoa64(g.ChX) + `" y="` + itoa64(g.ChY) + `"/>`)
	b.WriteString(`<a:chExt cx="` + itoa64(g.ChW) + `" cy="` + itoa64(g.ChH) + `"/>`)
	b.WriteString(`</a:xfrm></p:grpSpPr>`)
	if len(g.Children) > 0 {
		// Preferred path: render children in document order so geometry and
		// text z-order matches the source.
		for _, ch := range g.Children {
			switch {
			case ch.Geom != nil:
				b.WriteString(renderGeom(ch.Geom, *id))
				*id++
			case ch.Text != nil:
				b.WriteString(renderShape(ch.Text, *id, relIdx, rels))
				*id++
			}
		}
	} else {
		for _, gs := range g.Geoms {
			b.WriteString(renderGeom(gs, *id))
			*id++
		}
		for _, sh := range g.Texts {
			b.WriteString(renderShape(sh, *id, relIdx, rels))
			*id++
		}
	}
	b.WriteString(`</p:grpSp>`)
	return b.String()
}

func renderGeom(gs *GeomShape, id int) string {
	var b strings.Builder
	name := gs.Name
	if name == "" {
		name = fmt.Sprintf("Shape %d", id)
	}

	b.WriteString(`<p:sp><p:nvSpPr>`)
	b.WriteString(fmt.Sprintf(`<p:cNvPr id="%d" name="%s"/>`, id, escapeXML(name)))
	b.WriteString(`<p:cNvSpPr/><p:nvPr/></p:nvSpPr>`)
	b.WriteString(`<p:spPr>`)
	b.WriteString(`<a:xfrm><a:off x="` + itoa64(gs.X) + `" y="` + itoa64(gs.Y) + `"/>`)
	b.WriteString(`<a:ext cx="` + itoa64(gs.W) + `" cy="` + itoa64(gs.H) + `"/></a:xfrm>`)
	b.WriteString(`<a:custGeom><a:avLst/><a:gdLst/><a:ahLst/><a:cxnLst/>`)
	b.WriteString(`<a:rect l="0" t="0" r="` + itoa64(gs.PathW) + `" b="` + itoa64(gs.PathH) + `"/>`)
	b.WriteString(`<a:pathLst>`)
	for _, p := range gs.Paths {
		b.WriteString(`<a:path w="` + itoa64(gs.PathW) + `" h="` + itoa64(gs.PathH) + `">`)
		for _, cmd := range p.Cmds {
			b.WriteString(renderPathCmd(cmd))
		}
		b.WriteString(`</a:path>`)
	}
	b.WriteString(`</a:pathLst></a:custGeom>`)
	b.WriteString(renderFill(gs.Fill))
	if gs.Stroke != nil {
		b.WriteString(renderStroke(gs.Stroke))
	}
	b.WriteString(`</p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr/></a:p></p:txBody></p:sp>`)
	return b.String()
}

func renderPathCmd(cmd PathCmd) string {
	switch cmd.Verb {
	case MoveTo:
		if len(cmd.Pts) < 1 {
			return ""
		}
		return `<a:moveTo>` + renderPathPoint(cmd.Pts[0]) + `</a:moveTo>`
	case LineTo:
		if len(cmd.Pts) < 1 {
			return ""
		}
		return `<a:lnTo>` + renderPathPoint(cmd.Pts[0]) + `</a:lnTo>`
	case CubicTo:
		if len(cmd.Pts) < 3 {
			return ""
		}
		return `<a:cubicBezTo>` + renderPathPoint(cmd.Pts[0]) + renderPathPoint(cmd.Pts[1]) + renderPathPoint(cmd.Pts[2]) + `</a:cubicBezTo>`
	case QuadTo:
		if len(cmd.Pts) < 2 {
			return ""
		}
		return `<a:quadBezTo>` + renderPathPoint(cmd.Pts[0]) + renderPathPoint(cmd.Pts[1]) + `</a:quadBezTo>`
	case ClosePath:
		return `<a:close/>`
	default:
		return ""
	}
}

func renderPathPoint(p PathPoint) string {
	return `<a:pt x="` + itoa64(p.X) + `" y="` + itoa64(p.Y) + `"/>`
}

func renderFill(fill Fill) string {
	switch fill.Kind {
	case FillSolid:
		return `<a:solidFill><a:srgbClr val="` + escapeXML(fill.Color) + `">` + renderAlpha(fill.Alpha) + `</a:srgbClr></a:solidFill>`
	case FillGradient:
		if fill.Gradient == nil {
			return `<a:noFill/>`
		}
		return renderGradient(fill.Gradient, fill.Alpha)
	default:
		return `<a:noFill/>`
	}
}

func renderGradient(g *Gradient, overallAlpha float64) string {
	var b strings.Builder
	b.WriteString(`<a:gradFill><a:gsLst>`)
	for _, stop := range g.Stops {
		pos := stop.Pos
		if pos < 0 {
			pos = 0
		}
		if pos > 1 {
			pos = 1
		}
		alpha := stop.Alpha
		if overallAlpha < 1 {
			alpha *= overallAlpha
		}
		b.WriteString(fmt.Sprintf(`<a:gs pos="%d"><a:srgbClr val="%s">%s</a:srgbClr></a:gs>`,
			int(math.Round(pos*100000)), escapeXML(stop.Color), renderAlpha(alpha)))
	}
	b.WriteString(`</a:gsLst>`)
	switch g.Kind {
	case GradientRadial:
		b.WriteString(`<a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path>`)
	default:
		b.WriteString(fmt.Sprintf(`<a:lin ang="%d" scaled="1"/>`, normalizeAngle60000(g.Angle)))
	}
	b.WriteString(`</a:gradFill>`)
	return b.String()
}

// normalizeAngle60000 converts an angle in degrees to OOXML's 60000ths of a
// degree in the valid positive range [0, 21600000).
func normalizeAngle60000(deg float64) int {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return int(deg * 60000)
}

func renderStroke(st *Stroke) string {
	width := st.Width
	if width < 1 {
		width = 1
	}
	cap := st.Cap
	if cap == "" {
		cap = "flat"
	}
	join := st.Join
	if join == "" {
		join = "round"
	}

	var b strings.Builder
	b.WriteString(`<a:ln w="` + itoa64(width) + `" cap="` + escapeXML(cap) + `">`)
	b.WriteString(`<a:solidFill><a:srgbClr val="` + escapeXML(st.Color) + `">` + renderAlpha(st.Alpha) + `</a:srgbClr></a:solidFill>`)
	if st.Dash != "" {
		b.WriteString(`<a:prstDash val="` + escapeXML(st.Dash) + `"/>`)
	}
	switch join {
	case "bevel":
		b.WriteString(`<a:bevel/>`)
	case "miter":
		b.WriteString(`<a:miter lim="800000"/>`)
	default:
		b.WriteString(`<a:round/>`)
	}
	b.WriteString(`</a:ln>`)
	return b.String()
}

func renderAlpha(alpha float64) string {
	if alpha >= 1 {
		return ""
	}
	if alpha < 0 {
		alpha = 0
	}
	return fmt.Sprintf(`<a:alpha val="%d"/>`, int(math.Round(alpha*100000)))
}

// picNvPr returns the picture's <p:nvPr> element, embedding a <p:ph> binding
// when the picture fills a placeholder and using the self-closing empty form
// otherwise.
func picNvPr(pic *Picture) string {
	if !pic.isPlaceholder() {
		return `<p:nvPr/>`
	}
	ph := `<p:ph`
	if pic.Placeholder != PlaceholderNone {
		ph += fmt.Sprintf(` type="%s"`, pic.Placeholder)
	}
	if pic.PlaceholderIdx > 0 {
		ph += fmt.Sprintf(` idx="%d"`, pic.PlaceholderIdx)
	}
	return `<p:nvPr>` + ph + `/></p:nvPr>`
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
	if sh.isPlaceholder() {
		ph := `<p:ph`
		if sh.Placeholder != PlaceholderNone {
			ph += fmt.Sprintf(` type="%s"`, sh.Placeholder)
		}
		if sh.PlaceholderIdx > 0 {
			ph += fmt.Sprintf(` idx="%d"`, sh.PlaceholderIdx)
		}
		ph += `/>`
		b.WriteString(ph)
	}
	b.WriteString(shapeMetaExt(sh.Role, shapeFingerprint(sh), shapeSlotKey(sh)))
	b.WriteString(`</p:nvPr></p:nvSpPr>`)

	b.WriteString(`<p:spPr>`)
	if sh.W > 0 || sh.H > 0 || sh.X > 0 || sh.Y > 0 {
		b.WriteString(fmt.Sprintf(`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, sh.X, sh.Y, sh.W, sh.H))
	}
	if !sh.isPlaceholder() {
		b.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`)
	}
	b.WriteString(`</p:spPr>`)

	bodyPr := `<a:bodyPr/>`
	if sh.NoInset {
		bodyPr = `<a:bodyPr lIns="0" tIns="0" rIns="0" bIns="0"/>`
	}
	b.WriteString(`<p:txBody>` + bodyPr + `<a:lstStyle/>`)
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
		bullet = `<a:buFont typeface="Noto Sans" panose="020B0604020202020204" pitchFamily="34" charset="0"/><a:buChar char="&#8226;"/>`
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
	switch r.Baseline {
	case "super":
		rPr.WriteString(` baseline="30000"`)
	case "sub":
		rPr.WriteString(` baseline="-25000"`)
	}
	rPr.WriteString(`>`)

	var inner strings.Builder
	if r.Color != "" {
		inner.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, escapeXML(r.Color)))
	}
	if r.BgColor != "" {
		inner.WriteString(fmt.Sprintf(`<a:highlight><a:srgbClr val="%s"/></a:highlight>`, escapeXML(r.BgColor)))
	}
	// The CT_TextCharacterProperties schema requires latin/cs (the font) to
	// follow fill/highlight and precede hlinkClick, so emit fonts before Link.
	if r.FontFamily != "" {
		inner.WriteString(fmt.Sprintf(`<a:latin typeface="%s"/>`, escapeXML(r.FontFamily)))
		if r.Code {
			inner.WriteString(`<a:cs typeface="Noto Sans Mono"/>`)
		}
	} else if r.Code {
		inner.WriteString(`<a:latin typeface="Noto Sans Mono" pitchFamily="49" charset="0"/><a:cs typeface="Noto Sans Mono"/>`)
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

const defaultRowHeight int64 = 370840 // ~0.405 inch

// contentWidthEMU is the fallback table width (the built-in body width).
const contentWidthEMU int64 = 10515600

// renderTable serializes a table as a p:graphicFrame containing an a:tbl with
// explicit per-cell borders and header fill (self-contained, no table style
// part required).
func renderTable(t *Table, id int, relIdx *int, rels *[]slideRel) string {
	rows := t.Rows
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, r := range rows {
		if len(r.Cells) > cols {
			cols = len(r.Cells)
		}
	}
	if cols == 0 {
		return ""
	}

	width := t.W
	if width <= 0 {
		width = contentWidthEMU
	}
	colW := width / int64(cols)
	height := t.H
	if height <= 0 {
		height = int64(len(rows)) * defaultRowHeight
	}

	var b strings.Builder
	b.WriteString(`<p:graphicFrame><p:nvGraphicFramePr>`)
	b.WriteString(fmt.Sprintf(`<p:cNvPr id="%d" name="Table %d"/>`, id, id))
	b.WriteString(`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`)
	b.WriteString(fmt.Sprintf(`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, t.X, t.Y, width, height))
	b.WriteString(`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">`)
	b.WriteString(`<a:tbl><a:tblPr firstRow="1" bandRow="1"/><a:tblGrid>`)
	for i := 0; i < cols; i++ {
		w := colW
		if i == cols-1 {
			w = width - colW*int64(cols-1) // last column absorbs rounding
		}
		b.WriteString(fmt.Sprintf(`<a:gridCol w="%d"/>`, w))
	}
	b.WriteString(`</a:tblGrid>`)

	headerRows := make([]bool, len(rows))
	for i, r := range rows {
		headerRows[i] = r.Header
	}

	for rIdx, r := range rows {
		b.WriteString(fmt.Sprintf(`<a:tr h="%d">`, defaultRowHeight))
		for c := 0; c < cols; c++ {
			var cell *TableCell
			if c < len(r.Cells) {
				cell = r.Cells[c]
			}
			b.WriteString(renderTableCell(cell, r.Header, t.Style, rIdx, c, len(rows), cols, headerRows, relIdx, rels))
		}
		b.WriteString(`</a:tr>`)
	}
	b.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	return b.String()
}

func renderTableCell(cell *TableCell, header bool, style *TableStyleSpec, rowIdx, colIdx, nRows, nCols int, headerRows []bool, relIdx *int, rels *[]slideRel) string {
	var b strings.Builder
	b.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/>`)
	var paras []*Paragraph
	if cell != nil {
		paras = cell.Paragraphs
	}
	if len(paras) == 0 {
		b.WriteString(`<a:p><a:endParaRPr/></a:p>`)
	}
	align := AlignNone
	if cell != nil {
		align = cell.Align
	}
	var cellStyle TableCellStyleSpec
	if style != nil {
		cellStyle = tableRegionCellStyle(style, header, colIdx)
		if align == AlignNone {
			align = alignmentFromOOXML(cellStyle.HAlign)
		}
	}
	for _, p := range paras {
		if p.Align == AlignNone {
			p.Align = align
		}
		if style != nil {
			for _, run := range p.Runs {
				// TODO: apply the remaining deck table-cell text properties
				// (underline, strike, baseline, highlight/background color, font
				// size) once TableCellStyleSpec carries them.
				run.Bold = run.Bold || cellStyle.Bold
				run.Italic = run.Italic || cellStyle.Italic
				if cellStyle.Color != "" && run.Color == "" {
					run.Color = cellStyle.Color
				}
				if cellStyle.FontFamily != "" && run.FontFamily == "" && !run.Code {
					run.FontFamily = cellStyle.FontFamily
				}
			}
		} else if header {
			for _, run := range p.Runs {
				run.Bold = true
			}
		}
		b.WriteString(renderParagraph(p, relIdx, rels))
	}
	b.WriteString(`</a:txBody>`)

	if style != nil {
		lnL, lnR, lnT, lnB := tableCellBorders(style, rowIdx, colIdx, nRows, nCols, headerRows)
		b.WriteString(`<a:tcPr`)
		if cellStyle.VAlign != "" {
			b.WriteString(fmt.Sprintf(` anchor="%s"`, escapeXML(cellStyle.VAlign)))
		}
		b.WriteString(`>`)
		b.WriteString(renderTableBorder("lnL", lnL))
		b.WriteString(renderTableBorder("lnR", lnR))
		b.WriteString(renderTableBorder("lnT", lnT))
		b.WriteString(renderTableBorder("lnB", lnB))
		if cellStyle.BgColor != "" {
			b.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, escapeXML(cellStyle.BgColor)))
		}
		b.WriteString(`</a:tcPr></a:tc>`)
		return b.String()
	}

	// Cell properties: thin grey borders on all sides; header gets a light fill.
	const border = `<a:lnL w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnL>` +
		`<a:lnR w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnR>` +
		`<a:lnT w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnT>` +
		`<a:lnB w="6350"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:lnB>`
	b.WriteString(`<a:tcPr>` + border)
	if header {
		b.WriteString(`<a:solidFill><a:srgbClr val="D9E1F2"/></a:solidFill>`)
	}
	b.WriteString(`</a:tcPr></a:tc>`)
	return b.String()
}

func alignmentFromOOXML(algn string) Alignment {
	switch algn {
	case "":
		return AlignNone
	case string(AlignLeft):
		return AlignLeft
	case string(AlignCenter):
		return AlignCenter
	case string(AlignRight):
		return AlignRight
	default:
		return Alignment(algn)
	}
}

func tableRegionCellStyle(style *TableStyleSpec, header bool, colIdx int) TableCellStyleSpec {
	if header {
		if colIdx == 0 {
			return style.HeaderFirstCol
		}
		return style.HeaderOtherCols
	}
	if colIdx == 0 {
		return style.DataFirstCol
	}
	return style.DataOtherCols
}

func tableRegionRightBorder(style *TableStyleSpec, header bool, colIdx int) TableBorderSpec {
	if header {
		if colIdx == 0 {
			return style.HeaderFirstColRight
		}
		return style.HeaderOtherColRight
	}
	if colIdx == 0 {
		return style.DataFirstColRight
	}
	return style.DataOtherColRight
}

func tableRegionBottomBorder(style *TableStyleSpec, header bool, colIdx int) TableBorderSpec {
	if header {
		if colIdx == 0 {
			return style.HeaderFirstColBottom
		}
		return style.HeaderOtherColBottom
	}
	if colIdx == 0 {
		return style.DataFirstColBottom
	}
	return style.DataOtherColBottom
}

func tableCellBorders(style *TableStyleSpec, rowIdx, colIdx, nRows, nCols int, headerRows []bool) (lnL, lnR, lnT, lnB TableBorderSpec) {
	header := rowIdx >= 0 && rowIdx < len(headerRows) && headerRows[rowIdx]
	if colIdx == 0 {
		lnL = style.OuterVertical
	} else {
		lnL = tableRegionRightBorder(style, header, colIdx-1)
	}
	if colIdx == nCols-1 {
		lnR = style.OuterVertical
	} else {
		lnR = tableRegionRightBorder(style, header, colIdx)
	}
	if rowIdx == 0 {
		lnT = style.OuterHorizontal
	} else {
		aboveHeader := rowIdx-1 >= 0 && rowIdx-1 < len(headerRows) && headerRows[rowIdx-1]
		lnT = tableRegionBottomBorder(style, aboveHeader, colIdx)
	}
	if rowIdx == nRows-1 {
		lnB = style.OuterHorizontal
	} else {
		lnB = tableRegionBottomBorder(style, header, colIdx)
	}
	return lnL, lnR, lnT, lnB
}

func renderTableBorder(name string, spec TableBorderSpec) string {
	if spec.None {
		if spec.WidthEMU > 0 {
			return fmt.Sprintf(`<a:%s w="%d"><a:noFill/></a:%s>`, name, spec.WidthEMU, name)
		}
		return fmt.Sprintf(`<a:%s><a:noFill/></a:%s>`, name, name)
	}
	if spec.Color == "" {
		// A border width with no color would otherwise emit an invalid empty
		// srgbClr. Keep the width (fill inherited) or omit the border entirely.
		if spec.WidthEMU > 0 {
			return fmt.Sprintf(`<a:%s w="%d"/>`, name, spec.WidthEMU)
		}
		return ""
	}
	var attrs string
	if spec.WidthEMU > 0 {
		attrs = fmt.Sprintf(` w="%d"`, spec.WidthEMU)
	}
	var dash string
	if spec.Dash != "" {
		dash = fmt.Sprintf(`<a:prstDash val="%s"/>`, escapeXML(spec.Dash))
	}
	return fmt.Sprintf(`<a:%s%s><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:%s>`,
		name, attrs, escapeXML(spec.Color), dash, name)
}
