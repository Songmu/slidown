package slidown

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/k1LoW/errors"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

type MIMEType string

const (
	MIMETypeImagePNG  MIMEType = "image/png"
	MIMETypeImageJPEG MIMEType = "image/jpeg"
	MIMETypeImageGIF  MIMEType = "image/gif"
	MIMETypeImageSVG  MIMEType = "image/svg+xml"
)

type Image struct {
	i            image.Image
	b            []byte // Raw image data
	mimeType     MIMEType
	url          string // URL if the image was fetched from a URL
	fromMarkdown bool
	checksum     uint32                 // Checksum for the image data
	pHash        *goimagehash.ImageHash // Perceptual hash for JPEG images
	modTime      time.Time              // Modification time of the image file, if applicable
	link         string                 // External link associated with the image
	svgIcon      *oksvg.SvgIcon
	width        int
	height       int
	dimensionsOK bool
}

func NewImage(pathOrURL string) (_ *Image, err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	var b io.Reader
	var modTime time.Time
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		if _, err := url.Parse(pathOrURL); err != nil {
			return nil, fmt.Errorf("invalid URL %s: %w", pathOrURL, err)
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		req, err := http.NewRequest("GET", pathOrURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch image from URL %s: %w", pathOrURL, err)
		}
		req.Header.Set("User-Agent", userAgent)
		res, err := client.Do(req) //nolint:gosec // The URL is provided by the user via Markdown content, not from an untrusted external source.
		if err != nil {
			return nil, fmt.Errorf("failed to fetch image from URL %s: %w", pathOrURL, err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch image from URL %s: status code %d", pathOrURL, res.StatusCode)
		}
		b = res.Body
	} else {
		fi, err := os.Stat(pathOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to stat image file %s: %w", pathOrURL, err)
		}
		modTime = fi.ModTime()
		file, err := os.Open(pathOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open image file %s: %w", pathOrURL, err)
		}
		defer file.Close()
		b = file
	}
	i, err := newImageFromBuffer(b)
	if err != nil {
		return nil, fmt.Errorf("failed to create image from buffer: %w", err)
	}
	i.url = pathOrURL
	i.modTime = modTime
	return i, nil
}

func NewImageFromMarkdown(pathOrURL string) (_ *Image, err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	i, err := NewImage(pathOrURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create image from path or URL: %w", err)
	}
	i.fromMarkdown = true
	return i, nil
}

func NewImageFromCodeBlock(r io.Reader) (_ *Image, err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	i, err := newImageFromBuffer(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create image from code block: %w", err)
	}
	i.fromMarkdown = true
	return i, nil
}

func newImageFromBuffer(r io.Reader) (_ *Image, err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	if isSVG(b) {
		return &Image{
			b:        b,
			mimeType: MIMETypeImageSVG,
		}, nil
	}
	_, mimeType, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	var mt MIMEType
	switch mimeType {
	case "png":
		mt = MIMETypeImagePNG
	case "jpeg":
		mt = MIMETypeImageJPEG
	case "gif":
		mt = MIMETypeImageGIF
	default:
		return nil, fmt.Errorf("unsupported image MIME type: %s", mimeType)
	}
	return &Image{
		b:        b,
		mimeType: mt,
	}, nil
}

// isSVG reports whether b is an SVG document by decoding XML tokens and
// requiring the first start element to be <svg> (in the SVG namespace when a
// namespace is declared). This avoids false positives from a stray "<svg"
// substring in a comment or an unrelated (e.g. HTML) document.
func isSVG(b []byte) bool {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if se, ok := tok.(xml.StartElement); ok {
			if !strings.EqualFold(se.Name.Local, "svg") {
				return false
			}
			return se.Name.Space == "" || se.Name.Space == "http://www.w3.org/2000/svg"
		}
	}
}

func (i *Image) SetLink(link string) {
	i.link = link
}

func (i *Image) Checksum() uint32 {
	if i == nil {
		return 0
	}
	if i.checksum == 0 {
		i.checksum = crc32.ChecksumIEEE(i.b)
	}
	return i.checksum
}

func (i *Image) IsSVG() bool {
	return i != nil && i.mimeType == MIMETypeImageSVG
}

func (i *Image) Dimensions() (w, h int, err error) {
	if i == nil {
		return 0, 0, fmt.Errorf("image is nil")
	}
	if i.dimensionsOK {
		return i.width, i.height, nil
	}
	if i.IsSVG() {
		// An explicit zero width/height disables rendering; report a zero size
		// so the render pipeline skips the image entirely.
		if svgHasExplicitZeroSize(i.b) {
			i.width, i.height, i.dimensionsOK = 0, 0, true
			return 0, 0, nil
		}
		// Prefer the SVG's declared width/height (its intrinsic size); the
		// viewBox is only the coordinate window and may have a different aspect
		// ratio, which would mis-size the native fallback picture.
		var fw, fh float64
		if ew, eh, ok := svgExplicitSize(i.b); ok {
			fw, fh = ew, eh
		} else {
			icon, err := i.parseSVG()
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse SVG: %w", err)
			}
			fw, fh = icon.ViewBox.W, icon.ViewBox.H
			if fw <= 0 {
				fw = 300
			}
			if fh <= 0 {
				fh = 150
			}
		}
		w, h = capDimensions(fw, fh)
	} else {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(i.b))
		if err != nil {
			return 0, 0, fmt.Errorf("failed to decode image config: %w", err)
		}
		w, h = cfg.Width, cfg.Height
	}
	i.width, i.height, i.dimensionsOK = w, h, true
	return w, h, nil
}

// capDimensions rounds intrinsic SVG dimensions to ints, guarding NaN/Inf and
// non-positive values, and caps very large sizes with one shared scale factor
// so the aspect ratio is preserved.
func capDimensions(w, h float64) (int, int) {
	const maxDimension = 100000 // px
	if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
		w = 300
	}
	if math.IsNaN(h) || math.IsInf(h, 0) || h <= 0 {
		h = 150
	}
	// Scale both axes up together so the smaller sub-pixel dimension rounds to
	// at least 1px while preserving the aspect ratio (e.g. 0.1×100 -> 1×1000
	// rather than 1×100).
	if mn := math.Min(w, h); mn > 0 && mn < 1 {
		s := 1 / mn
		w *= s
		h *= s
	}
	if mx := math.Max(w, h); mx > maxDimension {
		f := maxDimension / mx
		w *= f
		h *= f
	}
	iw, ih := int(math.Round(w)), int(math.Round(h))
	if iw < 1 {
		iw = 1
	}
	if ih < 1 {
		ih = 1
	}
	return iw, ih
}

// svgRootAttr returns the raw value of an unqualified, case-correct attribute on
// the root <svg> element (empty when absent or when the root isn't an <svg>).
func svgRootAttr(b []byte, name string) string {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		se, isStart := tok.(xml.StartElement)
		if !isStart {
			continue
		}
		if !strings.EqualFold(se.Name.Local, "svg") {
			return ""
		}
		for _, a := range se.Attr {
			if a.Name.Space == "" && a.Name.Local == name {
				return a.Value
			}
		}
		return ""
	}
}

// svgHasExplicitZeroSize reports whether the root <svg> sets width or height to
// an explicit zero (including "0%"), which per SVG disables rendering.
func svgHasExplicitZeroSize(b []byte) bool {
	ws, hs, _ := svgRootSize(b)
	return isExplicitZeroLength(ws) || isExplicitZeroLength(hs)
}

// cssLengthUnits is the set of unit suffixes recognized on a length. A value
// with any other (unknown) suffix is an invalid declaration, not a zero.
var cssLengthUnits = map[string]bool{
	"": true, "%": true,
	"px": true, "pt": true, "pc": true, "mm": true, "cm": true, "in": true,
	"em": true, "rem": true, "ex": true, "ch": true, "q": true,
	"vw": true, "vh": true, "vmin": true, "vmax": true,
}

// isExplicitZeroLength reports whether s is an explicit zero length: a unitless
// zero, zero percent, or zero in a recognized CSS length unit. An unknown or
// malformed unit (e.g. "0foo", "0e") is an invalid declaration and returns
// false so the viewBox/default size is used instead of dropping the SVG.
func isExplicitZeroLength(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	n := len(s)
	for n > 0 {
		c := s[n-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '%' {
			n--
			continue
		}
		break
	}
	if !cssLengthUnits[strings.ToLower(strings.TrimSpace(s[n:]))] {
		return false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s[:n]), 64)
	return err == nil && v == 0
}

// svgExplicitSize returns the SVG's intrinsic width and height in px-equivalent
// user units. It resolves the root width/height (supporting px and the standard
// absolute units in/cm/mm/pt/pc) and, when those are missing or percentages,
// falls back to the viewBox dimensions. Returns ok=false when neither yields a
// usable size.
func svgExplicitSize(b []byte) (w, h float64, ok bool) {
	ws, hs, vb := svgRootSize(b)
	wv, okw := parseCSSLength(ws)
	hv, okh := parseCSSLength(hs)
	if okw && okh && wv > 0 && hv > 0 {
		return wv, hv, true
	}
	vw, vh, okvb := parseViewBoxWH(vb)
	// When only one dimension is declared, derive the other from the viewBox
	// aspect ratio so the SVG keeps its declared size.
	if okw && wv > 0 && okvb {
		return wv, wv * vh / vw, true
	}
	if okh && hv > 0 && okvb {
		return vw * hv / vh, hv, true
	}
	if okvb {
		return vw, vh, true
	}
	// No viewBox but one dimension is declared: keep it and default the missing
	// one to the SVG spec default (300x150) rather than discarding the size.
	if okw && wv > 0 {
		return wv, 150, true
	}
	if okh && hv > 0 {
		return 300, hv, true
	}
	return 0, 0, false
}

// svgRootSize returns the raw width/height/viewBox attribute strings of the
// root <svg> element (empty when absent or when the root isn't an <svg>).
func svgRootSize(b []byte) (width, height, viewBox string) {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", "", ""
		}
		se, isStart := tok.(xml.StartElement)
		if !isStart {
			continue
		}
		if !strings.EqualFold(se.Name.Local, "svg") {
			return "", "", ""
		}
		for _, a := range se.Attr {
			// Only unqualified, case-correct SVG sizing attributes affect the
			// intrinsic size. A namespaced attribute (e.g. foo:width) shares the
			// same Name.Local and must not be mistaken for the SVG attribute.
			if a.Name.Space != "" {
				continue
			}
			switch a.Name.Local {
			case "width":
				width = a.Value
			case "height":
				height = a.Value
			case "viewBox":
				viewBox = a.Value
			}
		}
		return width, height, viewBox
	}
}

// parseViewBoxWH extracts the width and height from a viewBox="minX minY w h".
func parseViewBoxWH(s string) (w, h float64, ok bool) {
	f := strings.FieldsFunc(strings.TrimSpace(s), func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' })
	if len(f) != 4 {
		return 0, 0, false
	}
	wv, err1 := strconv.ParseFloat(f[2], 64)
	hv, err2 := strconv.ParseFloat(f[3], 64)
	if err1 != nil || err2 != nil || wv <= 0 || hv <= 0 {
		return 0, 0, false
	}
	return wv, hv, true
}

// cssUnitPx maps the standard absolute CSS/SVG length units to px (96 dpi).
var cssUnitPx = map[string]float64{"px": 1, "in": 96, "cm": 96 / 2.54, "mm": 96 / 25.4, "pt": 96.0 / 72, "pc": 16}

// parseCSSLength parses an SVG length with an optional absolute unit (px, in,
// cm, mm, pt, pc) into px. Percentages and unknown units are rejected.
func parseCSSLength(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasSuffix(s, "%") {
		return 0, false
	}
	mul := 1.0
	if len(s) > 2 {
		if m, ok := cssUnitPx[strings.ToLower(s[len(s)-2:])]; ok {
			mul = m
			s = s[:len(s)-2]
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return v * mul, true
}

// RasterPNG renders the image to PNG bytes. The scale factor applies only to
// SVGs (rendered at scale × their intrinsic size); for raster inputs (PNG/JPEG/
// GIF) scale is ignored and the source is re-encoded as PNG at its native size.
// For SVGs it is a best-effort raster produced by the pure-Go oksvg rasterizer,
// which does not support every SVG feature (notably filter, clipPath, mask,
// embedded <image>, foreignObject and <text>); such parts may be omitted. It is
// intended only as a compatibility fallback for viewers that can't render the
// embedded native SVG (which PowerPoint 2016+ uses as the primary rendering).
func (i *Image) RasterPNG(scale float64) ([]byte, error) {
	if i == nil {
		return nil, fmt.Errorf("image is nil")
	}
	if !i.IsSVG() {
		// Contract: always return PNG-encoded bytes. Re-encode raster sources
		// (which may be JPEG/GIF) rather than returning their original bytes.
		img, err := i.Image()
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to encode image as PNG: %w", err)
		}
		return buf.Bytes(), nil
	}
	if scale <= 0 {
		scale = 1
	}
	icon, err := i.parseSVG()
	if err != nil {
		return nil, fmt.Errorf("failed to parse SVG: %w", err)
	}
	intrinsicW, intrinsicH, err := i.Dimensions()
	if err != nil {
		return nil, err
	}
	w := float64(intrinsicW) * scale
	h := float64(intrinsicH) * scale
	// Apply a single shared downscale so the raster keeps the SVG's aspect
	// ratio even when only one axis exceeds the cap (avoids stretching the
	// fallback in older viewers).
	const maxRasterDimension = 4096
	if mx := math.Max(w, h); mx > maxRasterDimension {
		f := maxRasterDimension / mx
		w *= f
		h *= f
	}
	iw, ih := int(math.Round(w)), int(math.Round(h))
	if iw < 1 {
		iw = 1
	}
	if ih < 1 {
		ih = 1
	}
	if icon.ViewBox.W <= 0 {
		icon.ViewBox.W = float64(intrinsicW)
	}
	if icon.ViewBox.H <= 0 {
		icon.ViewBox.H = float64(intrinsicH)
	}
	// oksvg's SetTarget applies independent X/Y scales and ignores
	// preserveAspectRatio, which would stretch content when the viewport and
	// viewBox aspect ratios differ. Apply the root preserveAspectRatio
	// alignment and meet/slice scale so the raster matches SVG semantics.
	setRasterTarget(icon, iw, ih, i.b)
	rgba := image.NewRGBA(image.Rect(0, 0, iw, ih))
	scanner := rasterx.NewScannerGV(iw, ih, rgba, rgba.Bounds())
	raster := rasterx.NewDasher(iw, ih, scanner)
	icon.Draw(raster, 1.0)
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, fmt.Errorf("failed to encode rasterized SVG: %w", err)
	}
	return buf.Bytes(), nil
}

// maxSVGRasterBytes bounds the raw SVG size passed to the oksvg rasterizer so a
// pathological input can't drive uncapped parsing/allocation.
const maxSVGRasterBytes = 20 << 20 // 20 MiB

// setRasterTarget maps the icon's viewBox onto an iw×ih canvas honoring the
// root preserveAspectRatio (alignment + meet/slice). preserveAspectRatio="none"
// keeps the independent-scale (stretch) behavior.
func setRasterTarget(icon *oksvg.SvgIcon, iw, ih int, raw []byte) {
	vbW, vbH := icon.ViewBox.W, icon.ViewBox.H
	fx, fy, none, slice := parsePreserveAspect(svgRootAttr(raw, "preserveAspectRatio"))
	if vbW <= 0 || vbH <= 0 || none {
		icon.SetTarget(0, 0, float64(iw), float64(ih))
		return
	}
	sx, sy := float64(iw)/vbW, float64(ih)/vbH
	s := math.Min(sx, sy)
	if slice {
		s = math.Max(sx, sy)
	}
	drawW, drawH := vbW*s, vbH*s
	offX := (float64(iw) - drawW) * fx
	offY := (float64(ih) - drawH) * fy
	icon.SetTarget(offX, offY, drawW, drawH)
}

// parsePreserveAspect parses an SVG preserveAspectRatio value into alignment
// fractions (0=min, 0.5=mid, 1=max) plus none/slice flags. The default is
// "xMidYMid meet" (centered, scale-to-fit).
func parsePreserveAspect(s string) (fx, fy float64, none, slice bool) {
	fx, fy = 0.5, 0.5
	f := strings.Fields(s)
	if len(f) > 0 && strings.EqualFold(f[0], "defer") {
		f = f[1:]
	}
	if len(f) == 0 {
		return
	}
	if strings.EqualFold(f[0], "none") {
		none = true
		return
	}
	a := strings.ToLower(f[0])
	switch {
	case strings.HasPrefix(a, "xmin"):
		fx = 0
	case strings.HasPrefix(a, "xmax"):
		fx = 1
	}
	switch {
	case strings.HasSuffix(a, "ymin"):
		fy = 0
	case strings.HasSuffix(a, "ymax"):
		fy = 1
	}
	if len(f) > 1 && strings.EqualFold(f[1], "slice") {
		slice = true
	}
	return
}

func (i *Image) parseSVG() (*oksvg.SvgIcon, error) {
	if i.svgIcon != nil {
		return i.svgIcon, nil
	}
	if len(i.b) > maxSVGRasterBytes {
		return nil, fmt.Errorf("svg too large to rasterize: %d bytes", len(i.b))
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(i.b))
	if err != nil {
		// oksvg rejects non-pixel root width/height units (e.g. in, %); retry
		// with those attributes normalized so the raster fallback still works.
		if norm, ok := normalizeSVGRootSize(i.b); ok {
			if icon2, err2 := oksvg.ReadIconStream(bytes.NewReader(norm)); err2 == nil {
				i.svgIcon = icon2
				return icon2, nil
			}
		}
		return nil, err
	}
	i.svgIcon = icon
	return icon, nil
}

// asciiLowerBytes lowercases only ASCII A-Z, preserving byte length so offsets
// into the result map 1:1 onto the input (unlike bytes.ToLower for multi-byte
// UTF-8 characters).
func asciiLowerBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

// svgRootSizeAttr matches a width= or height= attribute on the root svg tag.
// The leading whitespace (group 1) keeps it from matching hyphenated names like
// stroke-width; values may be single- or double-quoted.
var svgRootSizeAttr = regexp.MustCompile(`(?i)(\s)(width|height)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// normalizeSVGRootSize rewrites the root <svg> element's width/height values to
// plain pixel numbers (resolving absolute units) or drops them (for
// percentages/unknown units so oksvg falls back to the viewBox). Returns
// ok=false when there is no root <svg> tag to adjust.
func normalizeSVGRootSize(b []byte) ([]byte, bool) {
	// ASCII-only fold: bytes.ToLower can change UTF-8 byte lengths, which would
	// make the offsets computed here invalid for slicing the original b.
	lower := asciiLowerBytes(b)
	// Single-pass scan for the first "<svg" start tag that isn't inside an XML
	// comment (a prolog comment may contain the literal text "<svg>").
	start := -1
	for i := 0; i < len(lower); {
		if bytes.HasPrefix(lower[i:], []byte("<!--")) {
			j := bytes.Index(lower[i+4:], []byte("-->"))
			if j < 0 {
				break // unterminated comment: no real root before it ends
			}
			i += 4 + j + 3
			continue
		}
		if bytes.HasPrefix(lower[i:], []byte("<svg")) {
			start = i
			break
		}
		i++
	}
	if start < 0 {
		return nil, false
	}
	end := tagEnd(b, start)
	if end < 0 {
		return nil, false
	}
	tag := b[start : end+1]
	newTag := svgRootSizeAttr.ReplaceAllFunc(tag, func(m []byte) []byte {
		sub := svgRootSizeAttr.FindSubmatch(m)
		ws := string(sub[1])
		name := string(sub[2])
		val := string(sub[3])
		if val == "" {
			val = string(sub[4])
		}
		if px, ok := parseCSSLength(val); ok {
			return []byte(fmt.Sprintf(`%s%s="%g"`, ws, name, px))
		}
		return []byte(ws) // drop percentage/unknown-unit sizes, keep the space
	})
	out := make([]byte, 0, len(b))
	out = append(out, b[:start]...)
	out = append(out, newTag...)
	out = append(out, b[end+1:]...)
	return out, true
}

// tagEnd returns the index of the '>' that closes the start tag beginning at
// start, ignoring any '>' inside a single- or double-quoted attribute value
// (which is valid XML). Returns -1 if no unquoted '>' is found.
func tagEnd(b []byte, start int) int {
	var quote byte
	for i := start; i < len(b); i++ {
		c := b[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return i
		}
	}
	return -1
}

func (i *Image) Image() (image.Image, error) {
	if i == nil {
		return nil, fmt.Errorf("image is nil")
	}
	if i.i == nil {
		b := i.b
		if i.IsSVG() {
			pngBytes, err := i.RasterPNG(1)
			if err != nil {
				return nil, err
			}
			b = pngBytes
		}
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %w", err)
		}
		i.i = img
	}
	return i.i, nil
}

func (i *Image) PHash() (_ *goimagehash.ImageHash, err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	if i == nil {
		return nil, fmt.Errorf("image is nil")
	}
	if i.i == nil {
		if _, err := i.Image(); err != nil {
			return nil, err
		}
	}
	if i.pHash == nil {
		pHash, err := goimagehash.PerceptionHash(i.i)
		if err != nil {
			return nil, fmt.Errorf("failed to compute perceptual hash: %w", err)
		}
		i.pHash = pHash
	}
	return i.pHash, nil
}

func (i *Image) String() string {
	if i == nil {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(i.b)
	return fmt.Sprintf("data:%s;base64,%s", i.mimeType, encoded)
}

func (i *Image) Bytes() []byte {
	if i == nil {
		return nil
	}
	return i.b
}

// internalImage is a subset of `Image` that excludes state and other elements, containing the minimum
// data required to reproduce the `Image`. It is used for `json.Marshal/Unmarshal` and caching purposes.
type internalImage struct {
	Data         string
	URL          string
	FromMarkdown bool
	ModTime      time.Time
	Link         string
}

// MarshalJSON and UnmarshalJSON are defined for cloning data and for similarity comparisons of `slide` structures.
func (i *Image) MarshalJSON() (_ []byte, err error) {
	return json.Marshal(i.toInternal())
}

func (i *Image) UnmarshalJSON(data []byte) (err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	var iimg internalImage
	if err := json.Unmarshal(data, &iimg); err != nil {
		return fmt.Errorf("failed to unmarshal image data: %w", err)
	}
	return iimg.toImage(i)
}

func (i *Image) toInternal() *internalImage {
	return &internalImage{
		Data:         i.String(),
		URL:          i.url,
		FromMarkdown: i.fromMarkdown,
		ModTime:      i.modTime,
		Link:         i.link,
	}
}

func (iimg *internalImage) toImage(i *Image) error {
	i.url = iimg.URL
	i.fromMarkdown = iimg.FromMarkdown
	i.modTime = iimg.ModTime
	i.link = iimg.Link
	i.i = nil
	i.checksum = 0
	i.pHash = nil
	i.svgIcon = nil
	i.width = 0
	i.height = 0
	i.dimensionsOK = false

	data := []byte(iimg.Data)
	if !bytes.HasPrefix(data, []byte(`data:`)) {
		return fmt.Errorf("invalid image data: %s", data)
	}
	splitted := bytes.Split(bytes.TrimPrefix(data, []byte(`data:`)), []byte(";base64,"))
	if len(splitted) != 2 {
		return fmt.Errorf("invalid image data: %s", data)
	}
	i.mimeType = MIMEType(splitted[0])
	decoded, err := base64.StdEncoding.DecodeString(string(splitted[1]))
	if err != nil {
		return fmt.Errorf("failed to decode base64 image data: %w", err)
	}
	if i.mimeType == MIMETypeImageSVG {
		if !isSVG(decoded) {
			return fmt.Errorf("invalid SVG image data")
		}
		i.b = decoded
		return nil
	}
	_, mimeType, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}
	if string(i.mimeType) != fmt.Sprintf("image/%s", mimeType) {
		return fmt.Errorf("image MIME type mismatch: expected %s, got %s", i.mimeType, mimeType)
	}
	i.b = decoded
	return nil
}
