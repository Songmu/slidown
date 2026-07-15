// Package render maps slidown's internal slide model (the slidown package types
// produced by the md parser) onto the pptx package's serializable model.
//
// The mapping mirrors deck's "how markdown maps to placeholders" rules so that
// slidown stays compatible with deck's Markdown semantics: the shallowest
// heading becomes the title, the next becomes the subtitle, and everything else
// becomes body content. Inline fragment styles (bold/italic/code/link/
// strikethrough/underline/...) are translated to OOXML run properties.
package render

import (
	"bytes"
	"encoding/xml"
	"errors"
	"image"
	"image/png"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/pptx"
	"github.com/Songmu/slidown/pptx/svgshape"
)

type converter struct {
	styles map[string]pptx.StyleSpec
}

// ToPresentation converts internal slides into a pptx.Presentation using the
// built-in default layout (title + body placeholders).
func ToPresentation(ss slidown.Slides) *pptx.Presentation {
	p := pptx.New()
	c := &converter{}
	for _, s := range ss {
		sl := c.renderSlide(p, s)
		sl.Hidden = s.Skip
	}
	return p
}

func (c *converter) renderSlide(p *pptx.Presentation, s *slidown.Slide) *pptx.Slide {
	sl := p.AddSlide()

	if title := c.titleParagraphs(s); len(title) > 0 {
		sl.AddShape(&pptx.Shape{
			Name:        "Title",
			Placeholder: pptx.PlaceholderTitle,
			Paragraphs:  title,
		})
	}

	body := c.bodyParagraphs(s)
	if len(body) > 0 {
		sl.AddShape(&pptx.Shape{
			Name:           "Content",
			Placeholder:    pptx.PlaceholderBody,
			PlaceholderIdx: 1,
			Paragraphs:     body,
		})
	}

	renderImagesAt(sl, s.Images, contentX, contentY, contentW, contentH, len(body) > 0)
	c.renderTablesAt(sl, s.Tables, contentX, contentY, contentW, nil, len(body) > 0 || len(s.Images) > 0)

	sl.Note = s.SpeakerNote
	sl.Fingerprint = s.Fingerprint()
	sl.Key = s.Key
	return sl
}

// renderTablesAt maps internal tables to pptx tables placed within the given
// region. When other content is present, tables are nudged toward the lower half.
func (c *converter) renderTablesAt(sl *pptx.Slide, tables []*slidown.Table, rx, ry, rw int64, style *pptx.TableStyleSpec, crowded bool) {
	if len(tables) == 0 {
		return
	}
	y := ry
	if crowded {
		y = ry + contentH/2
	}
	for _, t := range tables {
		if t == nil || len(t.Rows) == 0 {
			continue
		}
		pt := &pptx.Table{X: rx, Y: y, W: rw, Style: style}
		for _, row := range t.Rows {
			pr := &pptx.TableRow{Header: rowIsHeader(row)}
			for _, cell := range row.Cells {
				pc := &pptx.TableCell{Align: toAlign(cell.Alignment)}
				para := &pptx.Paragraph{}
				for _, f := range cell.Fragments {
					if f == nil {
						continue
					}
					para.Runs = append(para.Runs, c.convertFragment(f))
				}
				pc.Paragraphs = []*pptx.Paragraph{para}
				pr.Cells = append(pr.Cells, pc)
			}
			pt.Rows = append(pt.Rows, pr)
		}
		sl.AddTable(pt)
		// Advance y so multiple tables stack instead of overlapping.
		y += int64(len(pt.Rows))*pptxRowHeight + tableGap
	}
}

const (
	pptxRowHeight int64 = 370840
	tableGap      int64 = 182880 // 0.2 inch
)

func rowIsHeader(row *slidown.TableRow) bool {
	for _, c := range row.Cells {
		if c != nil && c.IsHeader {
			return true
		}
	}
	return false
}

func toAlign(a string) pptx.Alignment {
	switch a {
	case "CENTER", "center", "ctr":
		return pptx.AlignCenter
	case "END", "RIGHT", "right", "r":
		return pptx.AlignRight
	case "START", "LEFT", "left", "l":
		return pptx.AlignLeft
	default:
		return pptx.AlignNone
	}
}

// Content region of the built-in layout's body placeholder, in EMUs.
const (
	contentX int64 = 838200
	contentY int64 = 1825625
	contentW int64 = 10515600
	contentH int64 = 4351338
	// emuPerPixel assumes a 96 DPI source image.
	emuPerPixel int64 = 9525
)

// renderImagesAt lays images out within the given region, in a single row,
// each fitted to its cell while preserving aspect ratio. When the slide also
// has body text, images are placed in the lower half to reduce overlap.
func renderImagesAt(sl *pptx.Slide, images []*slidown.Image, rx, ry, rw, rh int64, hasBody bool) {
	if len(images) == 0 {
		return
	}
	regionX, regionY, regionW, regionH := rx, ry, rw, rh
	if hasBody {
		regionY = ry + rh/2
		regionH = rh / 2
	}

	n := int64(len(images))
	const gap int64 = 91440 // 0.1 inch between cells
	cellW := (regionW - gap*(n-1)) / n

	for i, img := range images {
		if img == nil {
			continue
		}
		natW, natH, format, ok := imageNatEMU(img)
		if !ok {
			continue
		}

		// Scale to fit the cell (cellW x regionH) preserving aspect ratio.
		w, h := fit(natW, natH, cellW, regionH)
		cellX := regionX + int64(i)*(cellW+gap)
		x := cellX + (cellW-w)/2
		y := regionY + (regionH-h)/2

		if img.IsSVG() {
			// Prefer converting the SVG into native, editable PowerPoint shapes.
			// Fall back to embedding it as a native SVG picture when the
			// document uses features the converter can't faithfully reproduce.
			if g, converted := svgshape.Convert(img.Bytes()); converted {
				g.X, g.Y, g.W, g.H = x, y, w, h
				sl.AddGroup(g)
				continue
			}
			if pic := buildSVGPicture(img, x, y, w, h); pic != nil {
				sl.AddPicture(pic)
			}
			continue
		}

		sl.AddPicture(&pptx.Picture{
			Data: img.Bytes(),
			Ext:  imageExt(format),
			X:    x, Y: y, W: w, H: h,
		})
	}
}

// imageNatEMU returns the natural image dimensions in EMUs, handling both
// raster images (decoded via image.DecodeConfig) and SVG images (whose
// intrinsic size comes from the viewBox/width/height). format is the raster
// image format ("png"/"jpeg"/"gif") and is empty for SVG. ok is false when the
// dimensions cannot be determined.
func imageNatEMU(img *slidown.Image) (natW, natH int64, format string, ok bool) {
	if img == nil {
		return 0, 0, "", false
	}
	if img.IsSVG() {
		w, h, err := img.Dimensions()
		if err != nil || w == 0 || h == 0 {
			return 0, 0, "", false
		}
		return int64(w) * emuPerPixel, int64(h) * emuPerPixel, "", true
	}
	cfg, f, err := image.DecodeConfig(bytes.NewReader(img.Bytes()))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return 0, 0, "", false
	}
	return int64(cfg.Width) * emuPerPixel, int64(cfg.Height) * emuPerPixel, f, true
}

// buildSVGPicture builds a native SVG picture: the raster PNG fallback lives in
// Data (rendered at roughly the placed size for crispness) while SVGData holds
// the original SVG so PowerPoint 2016+ renders the vector version. For SVGs that
// reference external/relative resources, SVGData is omitted (best-effort raster
// only). It returns nil only when the image's dimensions can't be resolved, or
// when rasterization fails and no native SVG can be embedded either.
func buildSVGPicture(img *slidown.Image, x, y, w, h int64) *pptx.Picture {
	natW, natH, _, ok := imageNatEMU(img)
	if !ok {
		return nil
	}
	// Rasterize at roughly 2x the on-slide size for crispness. Use the larger of
	// the width/height ratios so the PNG is not under-sized when one dimension is
	// the tighter bound after fit()'s rounding.
	ratio := 0.0
	if natW > 0 {
		if r := float64(w) / float64(natW); r > ratio {
			ratio = r
		}
	}
	if natH > 0 {
		if r := float64(h) / float64(natH); r > ratio {
			ratio = r
		}
	}
	scale := 2.0
	if ratio > 0 {
		scale = 2 * ratio
	}
	// An SVG that references external/relative resources (e.g.
	// <image href="asset.png">) can't be reproduced fully: the pure-Go raster
	// omits <image> content and the embedded native SVG can't resolve a
	// relocated relative path. Prefer showing the best-effort raster (which
	// still renders the SVG's own vector content) over dropping the image, but
	// don't embed the native SVG whose external reference would dangle.
	embedSVG := !svgReferencesExternalResource(img.Bytes())
	png, err := img.RasterPNG(scale)
	if err != nil || len(png) == 0 {
		if !embedSVG {
			// No native SVG and no usable raster: nothing meaningful to embed.
			return nil
		}
		// Keep the native SVG (modern PowerPoint renders it) with a 1x1
		// transparent PNG so older viewers get a valid, if blank, fallback.
		png = transparentPNG()
	}
	pic := &pptx.Picture{
		Data: png,
		Ext:  "png",
		X:    x, Y: y, W: w, H: h,
	}
	if embedSVG {
		pic.SVGData = img.Bytes()
	}
	return pic
}

// svgReferencesExternalResource reports whether the SVG references a resource
// the package can't resolve after embedding: a resource-bearing element
// (<image>, <use>, <feImage>) with an external href, or an external url(...)
// reference in any attribute value or <style> text. Fragment (#id) and data:
// URIs are self-contained and ignored. It tokenizes the XML so unrelated text
// or hrefs on other elements don't cause false positives.
func svgReferencesExternalResource(b []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(b))
	// Match isSVG's lenient tokenization so common HTML entities or
	// slightly-noncompliant XML don't abort the scan and mask an external
	// reference.
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	inStyle := false
	for {
		tok, err := dec.Token()
		if err != nil {
			// Clean end of input: the whole document was scanned and no
			// external reference was found.
			if errors.Is(err, io.EOF) {
				return false
			}
			// A malformed/partially-parseable document may hide an external
			// reference past the parse error; treat it conservatively as unsafe
			// to embed as a native SVG.
			return true
		}
		switch t := tok.(type) {
		case xml.ProcInst:
			// An xml-stylesheet PI can pull in an external stylesheet; treat a
			// non-self-contained href as an external reference.
			if strings.EqualFold(t.Target, "xml-stylesheet") {
				if href := piHref(string(t.Inst)); href == "" || isExternalRef(href) {
					return true
				}
			}
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			inStyle = name == "style"
			// Any element other than a navigational <a> can reference a
			// rendering resource via href (image, use, feImage, linear/radial
			// gradients, pattern, textPath, mpath, ...). An external href on any
			// of them cannot be resolved by PowerPoint or the raster fallback,
			// so treat it as unsafe to embed as native SVG.
			resource := name != "a"
			for _, a := range t.Attr {
				if resource && strings.EqualFold(a.Name.Local, "href") {
					if isExternalRef(a.Value) {
						return true
					}
				}
				if hasExternalStyleRef(a.Value) {
					return true
				}
			}
		case xml.CharData:
			if inStyle && hasExternalStyleRef(string(t)) {
				return true
			}
		case xml.Directive:
			// A DOCTYPE with an external DTD (SYSTEM/PUBLIC) or an internal
			// subset declaring external entities pulls in resources PowerPoint
			// can't resolve, so force the raster-only path.
			if directiveReferencesExternal(string(t)) {
				return true
			}
		case xml.EndElement:
			inStyle = false
		}
	}
}

// piHref extracts the href pseudo-attribute value from an xml-stylesheet PI's
// instruction text (e.g. `type="text/css" href="theme.css"`).
func piHref(inst string) string {
	m := regexp.MustCompile(`(?i)href\s*=\s*(?:"([^"]*)"|'([^']*)')`).FindStringSubmatch(inst)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// directiveReferencesExternal reports whether an XML directive (e.g. a DOCTYPE)
// declares an external DTD or entity via a SYSTEM or PUBLIC identifier. The
// keywords are uppercase per the XML grammar, but a non-strict parser may accept
// other casings, so the match is case-insensitive; any match is treated
// conservatively as an external dependency.
func directiveReferencesExternal(d string) bool {
	u := strings.ToUpper(d)
	return strings.Contains(u, "SYSTEM") || strings.Contains(u, "PUBLIC")
}

// isExternalRef reports whether a reference target is external (not a #fragment
// or data: URI).
func isExternalRef(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && !strings.HasPrefix(v, "#") && !strings.HasPrefix(strings.ToLower(v), "data:")
}

// hasExternalStyleRef reports whether a style/attribute value references an
// external resource: a url(...) target or a string-form @import (which carries
// no url()). CSS escapes (e.g. \75 rl(...) for url(...)) are decoded first so an
// obfuscated reference can't bypass the scan.
func hasExternalStyleRef(s string) bool {
	if hasExternalStyleRefRaw(s) {
		return true
	}
	if dec := cssUnescape(s); dec != s {
		return hasExternalStyleRefRaw(dec)
	}
	return false
}

func hasExternalStyleRefRaw(s string) bool {
	if hasExternalURLRef(s) {
		return true
	}
	for _, m := range importStringRE.FindAllStringSubmatch(s, -1) {
		target := m[1]
		if target == "" {
			target = m[2]
		}
		if isExternalRef(target) {
			return true
		}
	}
	return false
}

// cssUnescape decodes CSS escape sequences: a backslash followed by 1-6 hex
// digits (with one optional trailing whitespace) yields that code point, and a
// backslash followed by any other character yields that literal character.
func cssUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		j := i
		for j < len(s) && j-i < 6 && isHexDigit(s[j]) {
			j++
		}
		if j > i {
			if v, err := strconv.ParseInt(s[i:j], 16, 32); err == nil {
				b.WriteRune(rune(v))
			}
			i = j
			if i < len(s) {
				switch s[i] {
				case ' ', '\t', '\n', '\r', '\f':
					i++
				}
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// importStringRE matches a string-form CSS import, e.g. @import "theme.css".
var importStringRE = regexp.MustCompile(`(?i)@import\s+(?:"([^"]*)"|'([^']*)')`)

// hasExternalURLRef reports whether s contains a url(...) reference whose target
// is external (not a #fragment or data: URI).
func hasExternalURLRef(s string) bool {
	// Scan the lowercased copy only: strings.ToLower can change a string's byte
	// length, so offsets into it must not be used to slice the original.
	lower := strings.ToLower(s)
	for {
		i := strings.Index(lower, "url(")
		if i < 0 {
			return false
		}
		rest := lower[i+4:]
		j := strings.IndexByte(rest, ')')
		if j < 0 {
			return false
		}
		target := strings.Trim(strings.TrimSpace(rest[:j]), "'\"")
		if target != "" && !strings.HasPrefix(target, "#") && !strings.HasPrefix(target, "data:") {
			return true
		}
		lower = rest[j+1:]
	}
}

// transparentPNG returns a 1x1 fully transparent PNG used as a raster fallback
// placeholder when rasterization fails but a native SVG is available.
func transparentPNG() []byte {
	m := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	_ = png.Encode(&buf, m)
	return buf.Bytes()
}

// distributeImagePlaceholders binds images to the layout's picture placeholders
// in visual order (top to bottom, then left to right), one image per
// placeholder, fitting each image into its placeholder region while preserving
// aspect ratio. Each emitted picture carries a <p:ph> element binding it to the
// layout placeholder. Images beyond the available placeholders — and images
// whose placeholder geometry cannot be resolved — are returned so the caller
// can lay them out with the default flow layout.
func distributeImagePlaceholders(sl *pptx.Slide, tmpl *pptx.Template, layout *pptx.LayoutInfo, images []*slidown.Image, picPHs []*pptx.PlaceholderInfo) []*slidown.Image {
	if len(images) == 0 || len(picPHs) == 0 || tmpl == nil {
		return images
	}
	ordered := orderPlaceholdersByPosition(tmpl, layout, picPHs)

	imgIdx := 0
	for _, ph := range ordered {
		if imgIdx >= len(images) {
			break
		}
		px, py, pw, ph2, ok := tmpl.EffectiveGeometry(layout, ph)
		if !ok {
			// Unusable placeholder: leave images for later placeholders or the
			// fallback flow layout.
			continue
		}
		img := images[imgIdx]
		imgIdx++
		if img == nil {
			continue
		}
		natW, natH, format, ok := imageNatEMU(img)
		if !ok {
			continue
		}
		w, h := fit(natW, natH, pw, ph2)
		x := px + (pw-w)/2
		y := py + (ph2-h)/2
		if img.IsSVG() {
			// Picture placeholders bind a native picture; SVGs are embedded as
			// native SVG images (with a raster fallback) rather than converted
			// to shapes, which would not fill the picture placeholder.
			pic := buildSVGPicture(img, x, y, w, h)
			if pic == nil {
				continue
			}
			pic.IsPlaceholder = true
			pic.Placeholder = pptx.PlaceholderType(ph.Type)
			pic.PlaceholderIdx = ph.Idx
			sl.AddPicture(pic)
			continue
		}
		sl.AddPicture(&pptx.Picture{
			Data:           img.Bytes(),
			Ext:            imageExt(format),
			X:              x,
			Y:              y,
			W:              w,
			H:              h,
			IsPlaceholder:  true,
			Placeholder:    pptx.PlaceholderType(ph.Type),
			PlaceholderIdx: ph.Idx,
		})
	}
	return images[imgIdx:]
}

// fit returns the largest (w,h) preserving the natural aspect ratio that fits
// within the bounding box (maxW,maxH).
func fit(natW, natH, maxW, maxH int64) (int64, int64) {
	if natW <= maxW && natH <= maxH {
		return natW, natH
	}
	// Compare aspect ratios with cross multiplication to avoid floats.
	if natW*maxH > maxW*natH {
		// width-bound
		return maxW, natH * maxW / natW
	}
	// height-bound
	return natW * maxH / natH, maxH
}

func imageExt(format string) string {
	switch format {
	case "jpeg":
		return "jpeg"
	case "gif":
		return "gif"
	default:
		return "png"
	}
}

// titleParagraphs produces the title placeholder paragraphs, preferring the
// rich TitleBodies when available and falling back to plain Titles.
func (c *converter) titleParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	if len(s.TitleBodies) > 0 {
		var out []*pptx.Paragraph
		for _, b := range s.TitleBodies {
			out = append(out, c.convertBody(b)...)
		}
		if len(out) > 0 {
			return out
		}
	}
	var out []*pptx.Paragraph
	for _, t := range s.Titles {
		out = append(out, &pptx.Paragraph{Runs: []*pptx.Run{{Text: t}}})
	}
	return out
}

// subtitleBoldParagraphs returns the subtitle content rendered as bold
// paragraphs, used when folding subtitles into a body placeholder because the
// layout has no dedicated subtitle slot.
func (c *converter) subtitleBoldParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	out := c.subtitleParagraphs(s)
	for _, para := range out {
		for _, r := range para.Runs {
			r.Bold = true
		}
	}
	return out
}

// bodyParagraphs collects subtitles and body content into the body placeholder.
// Because the built-in layout has no dedicated subtitle placeholder, subtitles
// are rendered as emphasized lead paragraphs so no content is lost.
func (c *converter) bodyParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	return append(c.subtitleBoldParagraphs(s), c.contentParagraphs(s)...)
}

// convertBlockQuote renders a block quote as italic, indented paragraphs so it
// is visually distinct within the body placeholder.
func (c *converter) convertBlockQuote(bq *slidown.BlockQuote) []*pptx.Paragraph {
	if bq == nil {
		return nil
	}
	var out []*pptx.Paragraph
	for _, para := range bq.Paragraphs {
		p := c.convertParagraph(para)
		p.Level += bq.Nesting + 1
		for _, r := range p.Runs {
			if spec, ok := c.styles["blockquote"]; ok {
				mergeStyleSpec(r, spec)
			} else {
				r.Italic = true
			}
		}
		out = append(out, p)
	}
	return out
}

func (c *converter) convertBody(b *slidown.Body) []*pptx.Paragraph {
	if b == nil {
		return nil
	}
	out := make([]*pptx.Paragraph, 0, len(b.Paragraphs))
	for _, para := range b.Paragraphs {
		out = append(out, c.convertParagraph(para))
	}
	return out
}

func (c *converter) convertParagraph(p *slidown.Paragraph) *pptx.Paragraph {
	out := &pptx.Paragraph{Level: p.Nesting}
	switch p.Bullet {
	case slidown.BulletDash:
		out.Bullet = true
	case slidown.BulletNumbered:
		out.Bullet = true
		out.Numbered = true
	}
	for _, f := range p.Fragments {
		if f == nil {
			continue
		}
		out.Runs = append(out.Runs, c.convertFragment(f))
	}
	return out
}

func (c *converter) convertFragment(f *slidown.Fragment) *pptx.Run {
	r := &pptx.Run{
		Text: f.Value,
		Link: f.Link,
	}
	if f.Code {
		c.applyStyleName(r, "code")
	}
	if f.Bold {
		c.applyStyleName(r, "bold")
	}
	if f.Italic {
		c.applyStyleName(r, "italic")
	}
	if f.Link != "" {
		c.applyStyleName(r, "link")
	}
	c.applyStyleName(r, f.StyleName)
	return r
}

// applyStyleName mirrors the semantics of deck's default inline syntax styles
// (style.go) for the subset expressible as OOXML run properties.
func (c *converter) applyStyleName(r *pptx.Run, name string) {
	if spec, ok := c.styles[name]; ok {
		applyStyleSpec(r, spec)
		return
	}
	switch name {
	case "", "link":
		// nothing extra
	case "bold", "strong":
		r.Bold = true
	case "italic", "em", "var":
		r.Italic = true
	case "del", "s":
		r.Strike = true
	case "u":
		r.Underline = true
	case "code", "kbd", "samp":
		r.Code = true
	case "sup":
		r.Baseline = "super"
	case "sub":
		r.Baseline = "sub"
	}
}

func applyStyleSpec(r *pptx.Run, s pptx.StyleSpec) {
	r.Bold = s.Bold
	r.Italic = s.Italic
	r.Underline = s.Underline
	r.Strike = s.Strike
	r.Color = s.Color
	r.BgColor = s.BgColor
	r.FontFamily = s.FontFamily
	r.Baseline = s.Baseline
	r.Code = false
}

// mergeStyleSpec applies a block-level base style (e.g. blockquote) to a run
// without clobbering inline formatting already present on it: booleans are
// OR-ed in and string properties are only filled when the run has none. This
// mirrors deck, where a block base style is applied first and inline styles
// override it, so inline emphasis inside the block is preserved.
func mergeStyleSpec(r *pptx.Run, s pptx.StyleSpec) {
	r.Bold = r.Bold || s.Bold
	r.Italic = r.Italic || s.Italic
	r.Underline = r.Underline || s.Underline
	r.Strike = r.Strike || s.Strike
	if r.Color == "" {
		r.Color = s.Color
	}
	if r.BgColor == "" {
		r.BgColor = s.BgColor
	}
	if r.FontFamily == "" && !r.Code {
		r.FontFamily = s.FontFamily
	}
	if r.Baseline == "" {
		r.Baseline = s.Baseline
	}
}
