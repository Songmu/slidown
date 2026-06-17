package render

import (
	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/pptx"
)

// ToPresentationWithTemplate converts internal slides into a pptx.Presentation
// that uses the given template's design (theme, masters, layouts). Each slide is
// mapped onto a template layout and its placeholders.
func ToPresentationWithTemplate(ss slidown.Slides, tmpl *pptx.Template) *pptx.Presentation {
	p := pptx.New()
	p.Template = tmpl
	for i, s := range ss {
		if s.Skip {
			continue
		}
		renderSlideWithLayout(p, s, tmpl, i == 0)
	}
	return p
}

func renderSlideWithLayout(p *pptx.Presentation, s *slidown.Slide, tmpl *pptx.Template, first bool) {
	layout := resolveLayout(s, tmpl, first)
	sl := p.AddSlide()
	sl.LayoutName = layout.Name

	titlePH, subPH, bodyPH := classifyPlaceholders(layout)

	// Title
	if titlePH != nil {
		if title := titleParagraphs(s); len(title) > 0 {
			sl.AddShape(&pptx.Shape{
				Name:           "Title",
				IsPlaceholder:  true,
				Placeholder:    pptx.PlaceholderType(titlePH.Type),
				PlaceholderIdx: titlePH.Idx,
				Paragraphs:     title,
			})
		}
	}

	// Body content (bodies + block quotes), and subtitles either in their own
	// placeholder or folded into the body when none exists.
	var bodyParas []*pptx.Paragraph
	if subPH != nil {
		if sub := subtitleParagraphs(s); len(sub) > 0 {
			sl.AddShape(&pptx.Shape{
				Name:           "Subtitle",
				IsPlaceholder:  true,
				Placeholder:    pptx.PlaceholderType(subPH.Type),
				PlaceholderIdx: subPH.Idx,
				Paragraphs:     sub,
			})
		}
		bodyParas = contentParagraphs(s)
	} else {
		bodyParas = bodyParagraphs(s) // subtitles folded in (emphasized)
	}

	if bodyPH != nil && len(bodyParas) > 0 {
		sl.AddShape(&pptx.Shape{
			Name:           "Content",
			IsPlaceholder:  true,
			Placeholder:    pptx.PlaceholderType(bodyPH.Type),
			PlaceholderIdx: bodyPH.Idx,
			Paragraphs:     bodyParas,
		})
	}

	// Position images/tables using the layout's body geometry when available.
	rx, ry, rw, rh := contentX, contentY, contentW, contentH
	if x, y, w, h, ok := layout.BodyGeometry(); ok {
		rx, ry, rw, rh = x, y, w, h
	}
	hasBody := bodyPH != nil && len(bodyParas) > 0
	renderImagesAt(sl, s.Images, rx, ry, rw, rh, hasBody)
	renderTablesAt(sl, s.Tables, rx, ry, rw, hasBody || len(s.Images) > 0)

	sl.Note = s.SpeakerNote
	sl.Fingerprint = s.Fingerprint()
	sl.Key = s.Key
}

// resolveLayout selects a template layout for the slide: by explicit name if it
// matches, otherwise a title layout for the first slide and a content layout for
// the rest.
func resolveLayout(s *slidown.Slide, tmpl *pptx.Template, first bool) *pptx.LayoutInfo {
	if s.Layout != "" {
		if l := tmpl.LayoutByName(s.Layout); l != nil {
			return l
		}
	}
	if first {
		return tmpl.TitleLayout()
	}
	return tmpl.ContentLayout()
}

// classifyPlaceholders picks the title, subtitle and body placeholders from a
// layout.
func classifyPlaceholders(l *pptx.LayoutInfo) (title, sub, body *pptx.PlaceholderInfo) {
	for _, ph := range l.Placeholders {
		switch ph.Type {
		case "ctrTitle", "title":
			if title == nil {
				title = ph
			}
		case "subTitle":
			if sub == nil {
				sub = ph
			}
		case "body", "obj", "tx", "":
			if body == nil {
				body = ph
			}
		}
	}
	return title, sub, body
}

// subtitleParagraphs renders only the subtitle content (without the emphasis
// that bodyParagraphs applies when folding subtitles into the body).
func subtitleParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	var out []*pptx.Paragraph
	if len(s.SubtitleBodies) > 0 {
		for _, b := range s.SubtitleBodies {
			out = append(out, convertBody(b)...)
		}
		return out
	}
	for _, st := range s.Subtitles {
		out = append(out, &pptx.Paragraph{Runs: []*pptx.Run{{Text: st}}})
	}
	return out
}

// contentParagraphs renders body content and block quotes (no subtitles).
func contentParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	var out []*pptx.Paragraph
	for _, b := range s.Bodies {
		out = append(out, convertBody(b)...)
	}
	for _, bq := range s.BlockQuotes {
		out = append(out, convertBlockQuote(bq)...)
	}
	return out
}
