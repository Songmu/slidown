// Package pptx provides a pure-Go writer for PowerPoint (.pptx / OOXML)
// presentations. It builds the Office Open XML package (a ZIP archive of XML
// parts) directly, without depending on any third-party Office libraries, so
// that slidown can remain MIT licensed.
//
// The package is intentionally low level: it knows how to assemble the fixed
// structural parts of a presentation (content types, relationships, the
// presentation part, a slide master, a slide layout and a theme) and how to
// emit individual slide parts. Higher level mapping from slidown's internal
// slide model is handled by the renderer that builds on top of this package.
package pptx

// English Metric Units (EMU) helpers. OOXML (and the Google Slides API) measure
// distances in EMUs, where 914400 EMU == 1 inch == 914400/72 points.
const (
	// EMUPerInch is the number of EMUs in one inch.
	EMUPerInch = 914400
	// EMUPerPoint is the number of EMUs in one typographic point.
	EMUPerPoint = 12700
	// EMUPerCentimeter is the number of EMUs in one centimeter.
	EMUPerCentimeter = 360000
)

// Inches converts a measurement in inches to EMUs.
func Inches(v float64) int64 { return int64(v * EMUPerInch) }

// Points converts a measurement in points to EMUs.
func Points(v float64) int64 { return int64(v * EMUPerPoint) }

// Default 16:9 slide dimensions in EMUs (13.333in x 7.5in), matching the
// PowerPoint "Widescreen" default.
const (
	DefaultSlideWidth  int64 = 12192000
	DefaultSlideHeight int64 = 6858000
)
