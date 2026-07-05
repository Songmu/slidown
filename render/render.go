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

// ToPresentation converts internal slides into a pptx.Presentation using the
// built-in default layout (title + body placeholders).
func ToPresentation(ss slidown.Slides) *pptx.Presentation {
	p := pptx.New()
	for _, s := range ss {
		if s.Skip {
			continue
		}
		renderSlide(p, s)
	}
	return p
}

func renderSlide(p *pptx.Presentation, s *slidown.Slide) {
	sl := p.AddSlide()

	if title := titleParagraphs(s); len(title) > 0 {
		sl.AddShape(&pptx.Shape{
			Name:        "Title",
			Placeholder: pptx.PlaceholderTitle,
			Paragraphs:  title,
		})
	}

	body := bodyParagraphs(s)
	if len(body) > 0 {
		sl.AddShape(&pptx.Shape{
			Name:           "Content",
			Placeholder:    pptx.PlaceholderBody,
			PlaceholderIdx: 1,
			Paragraphs:     body,
		})
	}

	renderImagesAt(sl, s.Images, contentX, contentY, contentW, contentH, len(body) > 0)
	renderTablesAt(sl, s.Tables, contentX, contentY, contentW, len(body) > 0 || len(s.Images) > 0)

	sl.Note = s.SpeakerNote
	sl.Fingerprint = s.Fingerprint()
	sl.Key = s.Key
}

// renderTablesAt maps internal tables to pptx tables placed within the given
// region. When other content is present, tables are nudged toward the lower half.
func renderTablesAt(sl *pptx.Slide, tables []*slidown.Table, rx, ry, rw int64, crowded bool) {
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
		pt := &pptx.Table{X: rx, Y: y, W: rw}
		for _, row := range t.Rows {
			pr := &pptx.TableRow{Header: rowIsHeader(row)}
			for _, cell := range row.Cells {
				pc := &pptx.TableCell{Align: toAlign(cell.Alignment)}
				para := &pptx.Paragraph{}
				for _, f := range cell.Fragments {
					if f == nil {
						continue
					}
					para.Runs = append(para.Runs, convertFragment(f))
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
func titleParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	if len(s.TitleBodies) > 0 {
		var out []*pptx.Paragraph
		for _, b := range s.TitleBodies {
			out = append(out, convertBody(b)...)
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
func subtitleBoldParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	out := subtitleParagraphs(s)
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
func bodyParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	return append(subtitleBoldParagraphs(s), contentParagraphs(s)...)
}

// convertBlockQuote renders a block quote as italic, indented paragraphs so it
// is visually distinct within the body placeholder.
func convertBlockQuote(bq *slidown.BlockQuote) []*pptx.Paragraph {
	if bq == nil {
		return nil
	}
	var out []*pptx.Paragraph
	for _, para := range bq.Paragraphs {
		p := convertParagraph(para)
		p.Level += bq.Nesting + 1
		for _, r := range p.Runs {
			r.Italic = true
		}
		out = append(out, p)
	}
	return out
}

func convertBody(b *slidown.Body) []*pptx.Paragraph {
	if b == nil {
		return nil
	}
	out := make([]*pptx.Paragraph, 0, len(b.Paragraphs))
	for _, para := range b.Paragraphs {
		out = append(out, convertParagraph(para))
	}
	return out
}

func convertParagraph(p *slidown.Paragraph) *pptx.Paragraph {
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
		out.Runs = append(out.Runs, convertFragment(f))
	}
	return out
}

func convertFragment(f *slidown.Fragment) *pptx.Run {
	r := &pptx.Run{
		Text:   f.Value,
		Bold:   f.Bold,
		Italic: f.Italic,
		Code:   f.Code,
		Link:   f.Link,
	}
	applyStyleName(r, f.StyleName)
	return r
}

// applyStyleName mirrors the semantics of deck's default inline syntax styles
// (style.go) for the subset expressible as OOXML run properties.
func applyStyleName(r *pptx.Run, name string) {
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
	}
}
