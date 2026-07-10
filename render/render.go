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
	"image"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/pptx"
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
		data := img.Bytes()
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width == 0 || cfg.Height == 0 {
			continue
		}
		natW := int64(cfg.Width) * emuPerPixel
		natH := int64(cfg.Height) * emuPerPixel

		// Scale to fit the cell (cellW x regionH) preserving aspect ratio.
		w, h := fit(natW, natH, cellW, regionH)
		cellX := regionX + int64(i)*(cellW+gap)
		x := cellX + (cellW-w)/2
		y := regionY + (regionH-h)/2

		sl.AddPicture(&pptx.Picture{
			Data: data,
			Ext:  imageExt(format),
			X:    x, Y: y, W: w, H: h,
		})
	}
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
		data := img.Bytes()
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width == 0 || cfg.Height == 0 {
			continue
		}
		natW := int64(cfg.Width) * emuPerPixel
		natH := int64(cfg.Height) * emuPerPixel
		w, h := fit(natW, natH, pw, ph2)
		x := px + (pw-w)/2
		y := py + (ph2-h)/2
		sl.AddPicture(&pptx.Picture{
			Data:           data,
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
