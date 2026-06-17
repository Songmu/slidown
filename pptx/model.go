package pptx

// Presentation is an in-memory representation of a deck that can be serialized
// to a .pptx package via WriteTo.
type Presentation struct {
	Width  int64
	Height int64
	Slides []*Slide
}

// New returns an empty Presentation using the default 16:9 dimensions.
func New() *Presentation {
	return &Presentation{
		Width:  DefaultSlideWidth,
		Height: DefaultSlideHeight,
	}
}

// AddSlide appends a slide and returns it for further configuration.
func (p *Presentation) AddSlide() *Slide {
	s := &Slide{}
	p.Slides = append(p.Slides, s)
	return s
}

// Slide is a single slide consisting of positioned shapes and optional speaker
// notes.
type Slide struct {
	Shapes []*Shape
	// Pictures are raster images placed on the slide.
	Pictures []*Picture
	// Note is the speaker note text for the slide.
	Note string
}

// AddShape appends a shape to the slide and returns it.
func (s *Slide) AddShape(sh *Shape) *Shape {
	s.Shapes = append(s.Shapes, sh)
	return sh
}

// AddPicture appends a picture to the slide and returns it.
func (s *Slide) AddPicture(p *Picture) *Picture {
	s.Pictures = append(s.Pictures, p)
	return p
}

// Picture is a raster image placed on a slide with explicit EMU geometry.
type Picture struct {
	Name string
	// Data is the raw encoded image (PNG/JPEG/GIF).
	Data []byte
	// Ext is the file extension without the dot: "png", "jpeg" or "gif".
	Ext string
	// Geometry in EMUs.
	X, Y, W, H int64
	// Link, when set, makes the picture a hyperlink to the given URL.
	Link string
}

// PlaceholderType enumerates the OOXML placeholder types relevant to slidown's
// title/subtitle/body mapping.
type PlaceholderType string

const (
	// PlaceholderNone marks a plain (non-placeholder) text box.
	PlaceholderNone PlaceholderType = ""
	// PlaceholderTitle is a standard title placeholder ("title").
	PlaceholderTitle PlaceholderType = "title"
	// PlaceholderCtrTitle is a centered title placeholder ("ctrTitle").
	PlaceholderCtrTitle PlaceholderType = "ctrTitle"
	// PlaceholderSubTitle is a subtitle placeholder ("subTitle").
	PlaceholderSubTitle PlaceholderType = "subTitle"
	// PlaceholderBody is a body placeholder ("body").
	PlaceholderBody PlaceholderType = "body"
)

// Shape is a text box (optionally a placeholder) positioned on a slide.
type Shape struct {
	Name        string
	Placeholder PlaceholderType
	// PlaceholderIdx is the placeholder index; only meaningful when Placeholder
	// is non-empty.
	PlaceholderIdx int
	// Geometry in EMUs. Used for non-placeholder shapes and as an explicit
	// override for placeholders.
	X, Y, W, H int64
	Paragraphs []*Paragraph
}

// Alignment enumerates horizontal paragraph alignment values.
type Alignment string

const (
	AlignNone   Alignment = ""
	AlignLeft   Alignment = "l"
	AlignCenter Alignment = "ctr"
	AlignRight  Alignment = "r"
)

// Paragraph is a single paragraph within a shape's text body.
type Paragraph struct {
	// Level is the indentation/nesting level (0-based).
	Level int
	// Bullet enables a bullet glyph for the paragraph.
	Bullet bool
	// Numbered, when true together with Bullet, renders an auto-numbered glyph.
	Numbered bool
	Align    Alignment
	Runs     []*Run
}

// Run is a styled span of text within a paragraph.
type Run struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	// Code renders the run using a monospace font.
	Code bool
	// Link, when set, makes the run a hyperlink to the given URL.
	Link string
	// FontSize in points; 0 means inherit from the placeholder/theme.
	FontSize float64
	// Color is an RRGGBB hex string (without '#'); empty means inherit.
	Color string
}
