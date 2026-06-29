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

	titlePH, subPH, subFromHint, bodyPHs := classifyPlaceholders(layout)

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
	}

	hasBody := distributeBodyContent(sl, s, subPH, bodyPHs)

	// Position images/tables using the layout's body geometry when available.
	rx, ry, rw, rh := contentX, contentY, contentW, contentH
	if x, y, w, h, ok := layout.BodyGeometry(); ok {
		rx, ry, rw, rh = x, y, w, h
	}
	renderImagesAt(sl, s.Images, rx, ry, rw, rh, hasBody)
	renderTablesAt(sl, s.Tables, rx, ry, rw, hasBody || len(s.Images) > 0)

	sl.Note = s.SpeakerNote
	sl.Fingerprint = s.Fingerprint()
	sl.Key = s.Key
}

// distributeBodyContent places body paragraphs into the available body
// placeholders and reports whether any body content was added.
//
// When there is only one body placeholder, all content is concatenated into it —
// the original behaviour. When the layout declares no body placeholders, no body
// content is rendered. With multiple body placeholders, body groups (separated
// by an intra-slide thematic break in the markdown source) are distributed
// one-per-placeholder; the last placeholder absorbs any overflow bodies and
// all block quotes.
func distributeBodyContent(sl *pptx.Slide, s *slidown.Slide, subPH *pptx.PlaceholderInfo, bodyPHs []*pptx.PlaceholderInfo) bool {
	if len(bodyPHs) == 0 {
		return false
	}

	if len(bodyPHs) == 1 {
		// Single placeholder: original behaviour — all content concatenated.
		var bodyParas []*pptx.Paragraph
		if subPH != nil {
			bodyParas = contentParagraphs(s)
		} else {
			bodyParas = bodyParagraphs(s) // subtitles folded in (emphasized)
		}
		if len(bodyParas) == 0 {
			return false
		}
		sl.AddShape(&pptx.Shape{
			Name:           "Content",
			IsPlaceholder:  true,
			Placeholder:    pptx.PlaceholderType(bodyPHs[0].Type),
			PlaceholderIdx: bodyPHs[0].Idx,
			Paragraphs:     bodyParas,
		})
		return true
	}

	// Multiple placeholders: distribute s.Bodies one per placeholder.
	hasBody := false
	for i, ph := range bodyPHs {
		isLast := i == len(bodyPHs)-1
		var paras []*pptx.Paragraph

		// When there is no subtitle placeholder, fold subtitles (as bold
		// runs) into the first body placeholder — matching the single-
		// placeholder behaviour.
		if i == 0 && subPH == nil {
			paras = append(paras, subtitleBoldParagraphs(s)...)
		}

		// Each placeholder gets one body section; the last one absorbs any
		// remaining sections so no content is silently dropped.
		if i < len(s.Bodies) {
			if isLast {
				for _, b := range s.Bodies[i:] {
					paras = append(paras, convertBody(b)...)
				}
			} else {
				paras = append(paras, convertBody(s.Bodies[i])...)
			}
		}

		// Block quotes go into the last placeholder.
		if isLast {
			for _, bq := range s.BlockQuotes {
				paras = append(paras, convertBlockQuote(bq)...)
			}
		}

		if len(paras) > 0 {
			hasBody = true
			sl.AddShape(&pptx.Shape{
				Name:           "Content",
				IsPlaceholder:  true,
				Placeholder:    pptx.PlaceholderType(ph.Type),
				PlaceholderIdx: ph.Idx,
				Paragraphs:     paras,
			})
		}
	}
	return hasBody
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
//  1. A real <p:ph type="subTitle"/> placeholder.
//  2. The first body-shaped placeholder whose display name or prompt text
//     contains "subtitle" (case insensitive). This lets users opt an ordinary
//     text placeholder into the subtitle role via PowerPoint's Selection Pane
//     or slide-master prompt edit, without XML editing.
//
// subFromHint reports whether the returned sub came from the hint fallback.
// The hint-promoted placeholder is removed from the body candidate pool, so
// any remaining body placeholders are returned as bodies.
func classifyPlaceholders(l *pptx.LayoutInfo) (title, sub *pptx.PlaceholderInfo, subFromHint bool, bodies []*pptx.PlaceholderInfo) {
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
	bodies = bodyCandidates
	return title, sub, subFromHint, bodies
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
