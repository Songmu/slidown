package render

import (
	"fmt"
	"os"
	"sort"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/pptx"
)

// ToPresentationWithTemplate converts internal slides into a pptx.Presentation
// that uses the given template's design (theme, masters, layouts). Each slide is
// mapped onto a template layout and its placeholders.
func ToPresentationWithTemplate(ss slidown.Slides, tmpl *pptx.Template) *pptx.Presentation {
	p := pptx.New()
	p.Template = tmpl
	c := &converter{styles: tmpl.SyntaxStyles}
	firstRendered := true
	for _, s := range ss {
		if s.Skip {
			continue
		}
		c.renderSlideWithLayout(p, s, tmpl, firstRendered)
		firstRendered = false
	}
	return p
}

func (c *converter) renderSlideWithLayout(p *pptx.Presentation, s *slidown.Slide, tmpl *pptx.Template, first bool) {
	layout := resolveLayout(s, tmpl, first)
	sl := p.AddSlide()
	sl.LayoutName = layout.Name

	titlePH, subSlots, bodyPHs, picPHs := classifyPlaceholdersWithPics(layout)

	// Title
	if titlePH != nil {
		if title := c.titleParagraphs(s); len(title) > 0 {
			sl.AddShape(&pptx.Shape{
				Name:           "Title",
				IsPlaceholder:  true,
				Placeholder:    pptx.PlaceholderType(titlePH.Type),
				PlaceholderIdx: titlePH.Idx,
				Paragraphs:     title,
			})
		}
	}

	// Subtitles: distributed across every subtitle-capable placeholder (real
	// subTitle placeholders and hint-promoted body placeholders), ordered by
	// their visual position on the layout.
	orderSubtitleSlots(tmpl, layout, subSlots)
	c.distributeSubtitleContent(sl, s, subSlots)

	hasBody := c.distributeBodyContent(sl, s, len(subSlots) > 0, bodyPHs)

	// Position images/tables using the layout's body geometry when available.
	rx, ry, rw, rh := contentX, contentY, contentW, contentH
	if x, y, w, h, ok := layout.BodyGeometry(); ok {
		rx, ry, rw, rh = x, y, w, h
	}
	// Bind as many images as possible to the layout's picture placeholders; any
	// remaining images fall back to the flow layout below.
	rest := distributeImagePlaceholders(sl, tmpl, layout, s.Images, picPHs)
	renderImagesAt(sl, rest, rx, ry, rw, rh, hasBody)
	// Only flow-laid images (rest) occupy the body region; images bound to
	// picture placeholders sit in their own layout slots, so they must not push
	// tables into the lower half.
	c.renderTablesAt(sl, s.Tables, rx, ry, rw, tmpl.TableStyle, hasBody || len(rest) > 0)

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
func (c *converter) distributeBodyContent(sl *pptx.Slide, s *slidown.Slide, hasSubSlot bool, bodyPHs []*pptx.PlaceholderInfo) bool {
	if len(bodyPHs) == 0 {
		return false
	}

	if len(bodyPHs) == 1 {
		// Single placeholder: original behaviour — all content concatenated.
		var bodyParas []*pptx.Paragraph
		if hasSubSlot {
			bodyParas = c.contentParagraphs(s)
		} else {
			bodyParas = c.bodyParagraphs(s) // subtitles folded in (emphasized)
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
		if i == 0 && !hasSubSlot {
			paras = append(paras, c.subtitleBoldParagraphs(s)...)
		}

		// Each placeholder gets one body section; the last one absorbs any
		// remaining sections so no content is silently dropped.
		if i < len(s.Bodies) {
			if isLast {
				for _, b := range s.Bodies[i:] {
					paras = append(paras, c.convertBody(b)...)
				}
			} else {
				paras = append(paras, c.convertBody(s.Bodies[i])...)
			}
		}

		// Block quotes go into the last placeholder.
		if isLast {
			for _, bq := range s.BlockQuotes {
				paras = append(paras, c.convertBlockQuote(bq)...)
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
		fmt.Fprintf(os.Stderr, "warning: layout %q not found in template; falling back to a default layout\n", s.Layout)
	}
	if first {
		return tmpl.TitleLayout()
	}
	return tmpl.ContentLayout()
}

// orderPlaceholdersByPosition returns the placeholders sorted by their visual
// position on the layout (top to bottom, then left to right), resolving each
// placeholder's geometry from the layout shape or the slide master it inherits
// from. When any placeholder's geometry cannot be resolved the original
// shape-tree order is preserved, so ordering never partially applies.
func orderPlaceholdersByPosition(tmpl *pptx.Template, layout *pptx.LayoutInfo, phs []*pptx.PlaceholderInfo) []*pptx.PlaceholderInfo {
	if tmpl == nil || len(phs) < 2 {
		return phs
	}
	type positioned struct {
		ph   *pptx.PlaceholderInfo
		x, y int64
	}
	items := make([]positioned, len(phs))
	for i, ph := range phs {
		x, y, _, _, ok := tmpl.EffectiveGeometry(layout, ph)
		if !ok {
			return phs
		}
		items[i] = positioned{ph: ph, x: x, y: y}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y != items[j].y {
			return items[i].y < items[j].y
		}
		return items[i].x < items[j].x
	})
	ordered := make([]*pptx.PlaceholderInfo, len(items))
	for i := range items {
		ordered[i] = items[i].ph
	}
	return ordered
}

// subtitleSlot is a layout placeholder that receives subtitle content: either a
// real <p:ph type="subTitle"/> or a body placeholder promoted via a subtitle
// hint. fromHint records the latter so the emitted shape can carry a slidown
// role marker while keeping its underlying "body" placeholder type.
type subtitleSlot struct {
	ph       *pptx.PlaceholderInfo
	fromHint bool
}

// classifyPlaceholders picks the title, subtitle and body placeholders from a
// layout. A placeholder becomes a subtitle slot when it is either:
//  1. A real <p:ph type="subTitle"/> placeholder, or
//  2. A body-shaped placeholder whose display name or prompt text contains
//     "subtitle" (case insensitive). This lets users opt an ordinary text
//     placeholder into the subtitle role via PowerPoint's Selection Pane or
//     slide-master prompt edit, without XML editing.
//
// All matching placeholders are returned (in the layout's shape-tree order) so
// multiple subtitles can be distributed across multiple slots. Hint-promoted
// slots are removed from the body pool; the rest are returned as bodies.
func classifyPlaceholders(l *pptx.LayoutInfo) (title *pptx.PlaceholderInfo, subs []subtitleSlot, bodies []*pptx.PlaceholderInfo) {
	title, subs, bodies, _ = classifyPlaceholdersWithPics(l)
	return title, subs, bodies
}

// classifyPlaceholdersWithPics is classifyPlaceholders extended to also return
// picture placeholders (<p:ph type="pic"/>, "clipArt" or "media") in layout
// shape-tree order. Picture placeholders receive images; they are never treated
// as body or subtitle slots.
func classifyPlaceholdersWithPics(l *pptx.LayoutInfo) (title *pptx.PlaceholderInfo, subs []subtitleSlot, bodies []*pptx.PlaceholderInfo, pics []*pptx.PlaceholderInfo) {
	for _, ph := range l.Placeholders {
		switch ph.Type {
		case "ctrTitle", "title":
			if title == nil {
				title = ph
			}
		case "subTitle":
			subs = append(subs, subtitleSlot{ph: ph})
		case "pic", "clipArt", "media":
			pics = append(pics, ph)
		case "body", "obj", "tx", "":
			if ph.HasSubtitleHint() {
				subs = append(subs, subtitleSlot{ph: ph, fromHint: true})
			} else {
				bodies = append(bodies, ph)
			}
		}
	}
	return title, subs, bodies, pics
}

// orderSubtitleSlots reorders the subtitle slots by their visual position on the
// layout (top to bottom, then left to right), resolving each placeholder's
// geometry from the layout shape or, when it declares none, from the slide
// master it inherits from. When any slot's geometry cannot be resolved the
// original shape-tree order is preserved, so ordering never partially applies.
func orderSubtitleSlots(tmpl *pptx.Template, layout *pptx.LayoutInfo, slots []subtitleSlot) {
	if tmpl == nil || len(slots) < 2 {
		return
	}
	type positioned struct {
		slot subtitleSlot
		x, y int64
	}
	items := make([]positioned, len(slots))
	for i, sl := range slots {
		x, y, _, _, ok := tmpl.EffectiveGeometry(layout, sl.ph)
		if !ok {
			return
		}
		items[i] = positioned{slot: sl, x: x, y: y}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y != items[j].y {
			return items[i].y < items[j].y
		}
		return items[i].x < items[j].x
	})
	for i := range items {
		slots[i] = items[i].slot
	}
}

// distributeSubtitleContent places subtitle content into the subtitle slots. Each
// subtitle (one per source heading at the subtitle level) is a paragraph group;
// groups are distributed one-per-slot in order, and the last slot absorbs any
// remaining groups so no content is dropped. Hint-promoted slots keep their
// underlying "body" placeholder type but carry a slidown role marker so
// incremental updates can identify the subtitle target by intent.
func (c *converter) distributeSubtitleContent(sl *pptx.Slide, s *slidown.Slide, slots []subtitleSlot) {
	groups := c.subtitleGroups(s)
	if len(groups) == 0 || len(slots) == 0 {
		return
	}
	for i, slot := range slots {
		isLast := i == len(slots)-1
		var paras []*pptx.Paragraph
		if i < len(groups) {
			if isLast {
				for _, g := range groups[i:] {
					paras = append(paras, g...)
				}
			} else {
				paras = append(paras, groups[i]...)
			}
		}
		if len(paras) == 0 {
			continue
		}
		role := ""
		if slot.fromHint {
			role = "subTitle"
		}
		sl.AddShape(&pptx.Shape{
			Name:           "Subtitle",
			IsPlaceholder:  true,
			Placeholder:    pptx.PlaceholderType(slot.ph.Type),
			PlaceholderIdx: slot.ph.Idx,
			Role:           role,
			Paragraphs:     paras,
		})
	}
}

// subtitleGroups returns the subtitle content grouped one entry per source
// subtitle heading, preserving inline formatting when available.
func (c *converter) subtitleGroups(s *slidown.Slide) [][]*pptx.Paragraph {
	var groups [][]*pptx.Paragraph
	if len(s.SubtitleBodies) > 0 {
		for _, b := range s.SubtitleBodies {
			if g := c.convertBody(b); len(g) > 0 {
				groups = append(groups, g)
			}
		}
		return groups
	}
	for _, st := range s.Subtitles {
		groups = append(groups, []*pptx.Paragraph{{Runs: []*pptx.Run{{Text: st}}}})
	}
	return groups
}

// subtitleParagraphs renders all subtitle content flattened into a single
// paragraph list (without the emphasis that bodyParagraphs applies when folding
// subtitles into the body). Used for the fold-into-body fallback.
func (c *converter) subtitleParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	var out []*pptx.Paragraph
	for _, g := range c.subtitleGroups(s) {
		out = append(out, g...)
	}
	return out
}

// contentParagraphs renders body content and block quotes (no subtitles).
func (c *converter) contentParagraphs(s *slidown.Slide) []*pptx.Paragraph {
	var out []*pptx.Paragraph
	for _, b := range s.Bodies {
		out = append(out, c.convertBody(b)...)
	}
	for _, bq := range s.BlockQuotes {
		out = append(out, c.convertBlockQuote(bq)...)
	}
	return out
}
