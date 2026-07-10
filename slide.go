package slidown

import "strings"

type Slides []*Slide

type Slide struct {
	Layout string `json:"layout"`
	Freeze bool   `json:"freeze,omitempty"`
	Skip   bool   `json:"skip,omitempty"`
	// Key is an opaque, stable per-page identifier (from the markdown page
	// config) used to match a slide to its existing counterpart across inserts,
	// deletions and reordering during an incremental rebuild. It is identity,
	// not content, so it is excluded from the content fingerprint.
	Key string `json:"-"`
	// TitleSlot reports that the slide occupies the deck's title-layout slot
	// (the first rendered slide) under the built-in first-slide layout default.
	// It only matters when Layout is empty: it is folded into the content
	// fingerprint so a slide that moves into or out of the first position — and
	// therefore switches between the title and content default layout — is
	// re-rendered instead of being reused verbatim with a stale layout. It is
	// identity-adjacent, not authored content, so it is excluded from JSON.
	TitleSlot      bool          `json:"-"`
	Titles         []string      `json:"titles,omitempty"`
	TitleBodies    []*Body       `json:"title_bodies,omitempty"`
	Subtitles      []string      `json:"subtitles,omitempty"`
	SubtitleBodies []*Body       `json:"subtitle_bodies,omitempty"`
	Bodies         []*Body       `json:"bodies,omitempty"`
	Images         []*Image      `json:"images,omitempty"`
	BlockQuotes    []*BlockQuote `json:"block_quotes,omitempty"`
	Tables         []*Table      `json:"tables,omitempty"`
	SpeakerNote    string        `json:"speaker_note,omitempty"`
}

// Body represents the content body of a slide.
type Body struct {
	Paragraphs []*Paragraph `json:"paragraphs,omitempty"`
}

// Paragraph represents a paragraph within a slide body.
type Paragraph struct {
	Fragments []*Fragment `json:"fragments,omitempty"`
	Bullet    Bullet      `json:"bullet,omitempty"`
	Nesting   int         `json:"nesting,omitempty"`
}

// Fragment represents a text fragment within a paragraph.
type Fragment struct {
	Value     string `json:"value"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Link      string `json:"link,omitempty"`
	Code      bool   `json:"code,omitempty"`
	StyleName string `json:"style_name,omitempty"`
}

type BlockQuote struct {
	Paragraphs []*Paragraph `json:"paragraphs,omitempty"`
	Nesting    int          `json:"nesting,omitempty"`
}

type Table struct {
	Rows []*TableRow `json:"rows,omitempty"`
}

type TableRow struct {
	Cells []*TableCell `json:"cells,omitempty"`
}

type TableCell struct {
	Fragments []*Fragment `json:"content,omitempty"`
	Alignment string      `json:"alignment,omitempty"`
	IsHeader  bool        `json:"is_header,omitempty"`
}

// Bullet represents the type of bullet point for a paragraph.
type Bullet string

// StyleName constants for inline-syntax styles shared with the markdown parser.
const (
	// StyleDel is the style name applied to GFM strikethrough (`~~`) and the
	// HTML <del> element.
	StyleDel = "del"
)

// Bullet constants for different bullet point types.
const (
	BulletNone     Bullet = ""
	BulletDash     Bullet = "-"
	BulletNumbered Bullet = "1"
)

func (b *Body) String() string {
	var result strings.Builder
	for i, paragraph := range b.Paragraphs {
		if i > 0 && b.Paragraphs[i-1].Bullet != BulletNone && paragraph.Bullet == BulletNone {
			result.WriteString("\n")
		}
		result.WriteString(paragraph.String())
		switch {
		case paragraph.Bullet != BulletNone:
			result.WriteString("\n")
		case i == len(b.Paragraphs)-1:
			result.WriteString("\n")
		default:
			result.WriteString("\n\n")
		}
	}
	return result.String()
}

func (p *Paragraph) String() string {
	if p == nil {
		return ""
	}
	var result strings.Builder
	result.WriteString(strings.Repeat("  ", p.Nesting))
	switch p.Bullet {
	case BulletDash:
		result.WriteString("- ")
	case BulletNumbered:
		result.WriteString("1. ")
	}
	for _, fragment := range p.Fragments {
		if fragment == nil {
			continue
		}
		result.WriteString(fragment.Value)
	}
	return result.String()
}

func (b *BlockQuote) String() string {
	if b == nil {
		return ""
	}
	quotes := strings.Repeat("> ", b.Nesting+1)
	var result strings.Builder
	for i, paragraph := range b.Paragraphs {
		result.WriteString(quotes)
		if i > 0 && b.Paragraphs[i-1].Bullet != BulletNone && paragraph.Bullet == BulletNone {
			result.WriteString("\n")
			result.WriteString(quotes)
		}
		result.WriteString(paragraph.String())
		switch {
		case paragraph.Bullet != BulletNone:
			result.WriteString("\n")
		case i == len(b.Paragraphs)-1:
			result.WriteString("\n")
		default:
			result.WriteString("\n")
			result.WriteString(quotes)
			result.WriteString("\n")
		}
	}
	return result.String()
}

func (f *Fragment) StylesEqual(other *Fragment) bool {
	if f == nil || other == nil {
		return f == other
	}
	return f.Bold == other.Bold &&
		f.Italic == other.Italic &&
		f.Link == other.Link &&
		f.Code == other.Code &&
		f.StyleName == other.StyleName
}
