package pptx

// Presentation is an in-memory representation of a deck that can be serialized
// to a .pptx package via WriteTo.
type Presentation struct {
	Width  int64
	Height int64
	Slides []*Slide
	// Title is the presentation title written to the document metadata
	// (docProps/core.xml). Empty leaves the title blank.
	Title string
	// Template, when set, supplies the design (theme, masters, layouts) and the
	// generated slides reference its layouts instead of the built-in one.
	Template *Template
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
	// Groups are grouped custom-geometry shapes and text boxes placed on the slide.
	Groups []*GroupShape
	// Tables are tables placed on the slide.
	Tables []*Table
	// LayoutName is the template layout name this slide should use. Ignored in
	// built-in (non-template) mode.
	LayoutName string
	// Note is the speaker note text for the slide.
	Note string
	// Fingerprint, when set, is embedded in the slide XML as a slidown
	// extension so an incremental rebuild can detect whether the source
	// content for this slide changed. Empty omits it.
	Fingerprint string
	// Key is the stable per-slide identity (from the markdown page config),
	// embedded alongside the fingerprint so a rebuild can match slides across
	// inserts, deletions and reordering. Empty omits it.
	Key string
	// Hidden marks the slide as skipped in the presentation: it is still
	// generated as a slide part but its sldId entry carries show="0" so
	// PowerPoint hides it during a slideshow.
	Hidden bool
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

// AddGroup appends a group shape to the slide and returns it.
func (s *Slide) AddGroup(g *GroupShape) *GroupShape {
	s.Groups = append(s.Groups, g)
	return g
}

// AddTable appends a table to the slide and returns it.
func (s *Slide) AddTable(t *Table) *Table {
	s.Tables = append(s.Tables, t)
	return t
}

// Picture is a raster image placed on a slide with explicit EMU geometry.
type Picture struct {
	Name string
	// Data is the raw encoded image (PNG/JPEG/GIF).
	Data []byte
	// SVGData, when set, embeds a native SVG alongside Data as the raster
	// fallback via an asvg:svgBlip extension.
	SVGData []byte
	// Ext is the file extension without the dot: "png", "jpeg" or "gif".
	Ext string
	// Geometry in EMUs.
	X, Y, W, H int64
	// Link, when set, makes the picture a hyperlink to the given URL.
	Link string
	// Placeholder, when set (together with IsPlaceholder), binds this picture to
	// a layout picture placeholder by emitting a <p:ph> element under the
	// picture's <p:nvPr>. Typically PlaceholderPic.
	Placeholder PlaceholderType
	// IsPlaceholder marks this picture as filling a placeholder even when
	// Placeholder is the empty type. When true a <p:ph> element is emitted.
	IsPlaceholder bool
	// PlaceholderIdx is the placeholder index; only meaningful for placeholders.
	PlaceholderIdx int
}

// isPlaceholder reports whether the picture should emit a placeholder element.
func (p *Picture) isPlaceholder() bool {
	return p.IsPlaceholder || p.Placeholder != PlaceholderNone
}

// FillKind enumerates fill types for custom-geometry shapes.
type FillKind int

const (
	// FillNone means the shape has no fill.
	FillNone FillKind = iota
	// FillSolid means the shape uses a single solid color fill.
	FillSolid
	// FillGradient means the shape uses a gradient fill.
	FillGradient
)

// GradientKind enumerates gradient fill types.
type GradientKind int

const (
	// GradientLinear is a linear gradient.
	GradientLinear GradientKind = iota
	// GradientRadial is a radial gradient.
	GradientRadial
)

// GradientStop is one color stop in a gradient fill.
type GradientStop struct {
	Pos   float64 // 0..1 position along the gradient
	Color string  // RRGGBB hex, no '#'
	Alpha float64 // 0..1 (1 = opaque)
}

// Gradient describes a linear or radial gradient fill.
type Gradient struct {
	Kind  GradientKind
	Angle float64 // degrees clockwise from 3 o'clock; used for linear
	Stops []GradientStop
}

// Fill describes the fill style for a custom-geometry shape.
type Fill struct {
	Kind     FillKind
	Color    string    // RRGGBB for FillSolid
	Alpha    float64   // 0..1 for FillSolid
	Gradient *Gradient // for FillGradient
}

// Stroke describes the outline style for a custom-geometry shape.
type Stroke struct {
	Color string  // RRGGBB
	Alpha float64 // 0..1
	Width int64   // EMU; 0 renders a hairline-safe minimum
	Cap   string  // "rnd", "sq", "flat"; empty => "flat"
	Join  string  // "round", "bevel", "miter"; empty => "round"
	Dash  string  // OOXML preset dash val (e.g. "dash", "sysDot"); empty => solid
}

// PathVerb enumerates supported path commands for custom geometry.
type PathVerb int

const (
	// MoveTo moves the current point to one point.
	MoveTo PathVerb = iota
	// LineTo draws a line to one point.
	LineTo
	// CubicTo draws a cubic Bezier curve using control1, control2 and end points.
	CubicTo
	// QuadTo draws a quadratic Bezier curve using control and end points.
	QuadTo
	// ClosePath closes the current subpath.
	ClosePath
)

// PathPoint is a point in the GeomShape's path coordinate space.
type PathPoint struct{ X, Y int64 }

// PathCmd is one command in a custom-geometry path.
type PathCmd struct {
	Verb PathVerb
	Pts  []PathPoint
}

// GeomPath is a sequence of path commands.
type GeomPath struct {
	Cmds []PathCmd
}

// GeomShape is a single custom-geometry shape inside a group.
type GeomShape struct {
	Name string
	// Placement of this shape within the parent group's child coordinate space (EMU).
	X, Y, W, H int64
	// Path coordinate space size (the <a:path w=.. h=..>). Path points use this space.
	PathW, PathH int64
	Paths        []GeomPath
	Fill         Fill
	Stroke       *Stroke // nil => no outline
	// EvenOdd records the source SVG fill rule. PowerPoint custom geometry uses
	// nonzero winding, so writers preserve the field but do not emit XML for it.
	EvenOdd bool
}

// GroupShape is a <p:grpSp> containing geometry shapes and optional text boxes.
type GroupShape struct {
	Name string
	// Placement on the slide (EMU).
	X, Y, W, H int64
	// Child coordinate space: chOff and chExt. Child shapes (Geoms/Texts) are
	// positioned in this space; PowerPoint scales child space (ChW x ChH) into
	// the group's on-slide W x H.
	ChX, ChY, ChW, ChH int64
	Geoms              []*GeomShape
	Texts              []*Shape
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
	// PlaceholderPic is a picture placeholder ("pic").
	PlaceholderPic PlaceholderType = "pic"
	// PlaceholderClipArt is a clip-art picture placeholder ("clipArt").
	PlaceholderClipArt PlaceholderType = "clipArt"
	// PlaceholderMedia is a media placeholder ("media").
	PlaceholderMedia PlaceholderType = "media"
)

// Shape is a text box (optionally a placeholder) positioned on a slide.
type Shape struct {
	Name        string
	Placeholder PlaceholderType
	// IsPlaceholder marks this shape as a placeholder even when Placeholder is
	// the empty (default body) type. When false, Placeholder != "" also implies
	// a placeholder.
	IsPlaceholder bool
	// PlaceholderIdx is the placeholder index; only meaningful for placeholders.
	PlaceholderIdx int
	// Role, when set, records slidown's semantic role for this shape (e.g.
	// "subTitle") independent of its underlying OOXML Placeholder type. It is
	// emitted as a slidown extension under <p:nvPr> so future incremental shape
	// updates can match shapes by intent even when the underlying placeholder
	// type is the generic "body". Unknown to PowerPoint; ignored on read by
	// tools that don't recognise the extension.
	Role string
	// Geometry in EMUs. Used for non-placeholder shapes and as an explicit
	// override for placeholders.
	X, Y, W, H int64
	Paragraphs []*Paragraph
}

// isPlaceholder reports whether the shape should emit a placeholder element.
func (s *Shape) isPlaceholder() bool {
	return s.IsPlaceholder || s.Placeholder != PlaceholderNone
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
	// BgColor is an RRGGBB hex string (without '#'); empty means no highlight.
	BgColor string
	// FontFamily is an explicit latin typeface; empty means inherit.
	FontFamily string
	// Baseline is "", "super", or "sub"; non-empty values render superscript/subscript.
	Baseline string
}

// Table is a simple grid table positioned on a slide.
type Table struct {
	// Geometry in EMUs. Height may be 0 to let it be derived from rows.
	X, Y, W, H int64
	Rows       []*TableRow
	// Style, when non-nil, overrides the default table styling (cell fills,
	// text styles, alignment and borders) parsed from a template's style
	// layout. Nil preserves the built-in hardcoded styling.
	Style *TableStyleSpec
}

// TableRow is a single row of table cells.
type TableRow struct {
	Cells  []*TableCell
	Header bool
}

// TableCell is a single table cell.
type TableCell struct {
	Paragraphs []*Paragraph
	Align      Alignment
}
