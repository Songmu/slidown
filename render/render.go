// Package render maps slidown's internal slide model (the deck package types
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

	deck "github.com/Songmu/slidown"
	"github.com/Songmu/slidown/pptx"
)

// ToPresentation converts internal slides into a pptx.Presentation using the
// built-in default layout (title + body placeholders).
func ToPresentation(ss deck.Slides) *pptx.Presentation {
	p := pptx.New()
	for _, s := range ss {
		if s.Skip {
			continue
		}
		renderSlide(p, s)
	}
	return p
}

func renderSlide(p *pptx.Presentation, s *deck.Slide) {
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

	renderImages(sl, s.Images, len(body) > 0)
	renderTables(sl, s.Tables, len(body) > 0 || len(s.Images) > 0)

	sl.Note = s.SpeakerNote
}

// renderTables maps internal tables to pptx tables placed in the content area.
// When other content is present, the table is nudged toward the lower half.
func renderTables(sl *pptx.Slide, tables []*deck.Table, crowded bool) {
	if len(tables) == 0 {
		return
	}
	y := contentY
	if crowded {
		y = contentY + contentH/2
	}
	for _, t := range tables {
		if t == nil || len(t.Rows) == 0 {
			continue
		}
		pt := &pptx.Table{X: contentX, Y: y, W: contentW}
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

func rowIsHeader(row *deck.TableRow) bool {
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

// renderImages lays images out within the content region, in a single row,
// each fitted to its cell while preserving aspect ratio. When the slide also
// has body text, images are placed in the lower half to reduce overlap.
func renderImages(sl *pptx.Slide, images []*deck.Image, hasBody bool) {
	if len(images) == 0 {
		return
	}
	regionX, regionY, regionW, regionH := contentX, contentY, contentW, contentH
	if hasBody {
		regionY = contentY + contentH/2
		regionH = contentH / 2
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
func titleParagraphs(s *deck.Slide) []*pptx.Paragraph {
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

// bodyParagraphs collects subtitles and body content into the body placeholder.
// Because the built-in layout has no dedicated subtitle placeholder, subtitles
// are rendered as emphasized lead paragraphs so no content is lost.
func bodyParagraphs(s *deck.Slide) []*pptx.Paragraph {
	var out []*pptx.Paragraph

	switch {
	case len(s.SubtitleBodies) > 0:
		for _, b := range s.SubtitleBodies {
			for _, para := range convertBody(b) {
				for _, r := range para.Runs {
					r.Bold = true
				}
				out = append(out, para)
			}
		}
	default:
		for _, st := range s.Subtitles {
			out = append(out, &pptx.Paragraph{Runs: []*pptx.Run{{Text: st, Bold: true}}})
		}
	}

	for _, b := range s.Bodies {
		out = append(out, convertBody(b)...)
	}
	return out
}

func convertBody(b *deck.Body) []*pptx.Paragraph {
	if b == nil {
		return nil
	}
	out := make([]*pptx.Paragraph, 0, len(b.Paragraphs))
	for _, para := range b.Paragraphs {
		out = append(out, convertParagraph(para))
	}
	return out
}

func convertParagraph(p *deck.Paragraph) *pptx.Paragraph {
	out := &pptx.Paragraph{Level: p.Nesting}
	switch p.Bullet {
	case deck.BulletDash:
		out.Bullet = true
	case deck.BulletNumbered:
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

func convertFragment(f *deck.Fragment) *pptx.Run {
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
