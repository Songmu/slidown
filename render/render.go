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

	sl.Note = s.SpeakerNote
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
