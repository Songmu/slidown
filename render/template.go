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

	titlePH, subPH, subFromHint, bodyPH := classifyPlaceholders(layout)

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
			// When the layout exposes no real subTitle placeholder but a body
			// placeholder hints at being one (HasSubtitleHint), keep the
			// underlying OOXML type ("body") so the slide stays
			// schema-compatible with the layout, and mark the shape with a
			// slidown Role so future incremental shape updates can identify
			// the subtitle target by intent.
			role := ""
			if subFromHint {
				role = "subTitle"
			}
			sl.AddShape(&pptx.Shape{
				Name:           "Subtitle",
				IsPlaceholder:  true,
				Placeholder:    pptx.PlaceholderType(subPH.Type),
				PlaceholderIdx: subPH.Idx,
				Role:           role,
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
// layout. Preference order for the subtitle slot:
//   1. A real <p:ph type="subTitle"/> placeholder.
//   2. The first body-shaped placeholder whose display name or prompt text
//      contains "subtitle" (case insensitive). This lets users opt an ordinary
//      text placeholder into the subtitle role via PowerPoint's Selection Pane
//      or slide-master prompt edit, without XML editing.
//
// subFromHint reports whether the returned sub came from the hint fallback.
// The hint-promoted placeholder is removed from the body candidate pool, so
// any remaining body placeholder is used as the body slot.
func classifyPlaceholders(l *pptx.LayoutInfo) (title, sub *pptx.PlaceholderInfo, subFromHint bool, body *pptx.PlaceholderInfo) {
	var bodyCandidates []*pptx.PlaceholderInfo
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
			bodyCandidates = append(bodyCandidates, ph)
		}
	}
	if sub == nil {
		for i, ph := range bodyCandidates {
			if ph.HasSubtitleHint() {
				sub = ph
				subFromHint = true
				bodyCandidates = append(bodyCandidates[:i], bodyCandidates[i+1:]...)
				break
			}
		}
	}
	if len(bodyCandidates) > 0 {
		body = bodyCandidates[0]
	}
	return title, sub, subFromHint, body
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
