package slidown

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	"strings"
	"time"
	"unicode"

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

func isSVG(b []byte) bool {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	b = bytes.TrimLeftFunc(b, unicode.IsSpace)
	if len(b) == 0 {
		return false
	}
	limit := min(len(b), 1024)
	prefix := bytes.ToLower(b[:limit])
	if hasSVGTag(prefix) {
		return true
	}
	if bytes.HasPrefix(prefix, []byte("<?xml")) {
		return hasSVGTag(prefix)
	}
	return bytes.Contains(prefix, []byte("http://www.w3.org/2000/svg")) && hasSVGTag(prefix)
}

func hasSVGTag(b []byte) bool {
	idx := bytes.Index(b, []byte("<svg"))
	if idx < 0 {
		return false
	}
	next := idx + len("<svg")
	return next == len(b) || b[next] == '>' || b[next] == '/' || unicode.IsSpace(rune(b[next]))
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
		icon, err := i.parseSVG()
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse SVG: %w", err)
		}
		w = roundDimension(icon.ViewBox.W, 300)
		h = roundDimension(icon.ViewBox.H, 150)
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

func roundDimension(v, fallback float64) int {
	if v <= 0 {
		v = fallback
	}
	n := int(math.Round(v))
	if n < 1 {
		return 1
	}
	return n
}

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
	w := clampRasterDimension(float64(intrinsicW) * scale)
	h := clampRasterDimension(float64(intrinsicH) * scale)
	if icon.ViewBox.W <= 0 {
		icon.ViewBox.W = float64(intrinsicW)
	}
	if icon.ViewBox.H <= 0 {
		icon.ViewBox.H = float64(intrinsicH)
	}
	icon.SetTarget(0, 0, float64(w), float64(h))
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, rgba, rgba.Bounds())
	raster := rasterx.NewDasher(w, h, scanner)
	icon.Draw(raster, 1.0)
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, fmt.Errorf("failed to encode rasterized SVG: %w", err)
	}
	return buf.Bytes(), nil
}

func clampRasterDimension(v float64) int {
	const maxDimension = 4096
	n := int(math.Round(v))
	if n < 1 {
		return 1
	}
	if n > maxDimension {
		return maxDimension
	}
	return n
}

func (i *Image) parseSVG() (*oksvg.SvgIcon, error) {
	if i.svgIcon != nil {
		return i.svgIcon, nil
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(i.b))
	if err != nil {
		return nil, err
	}
	i.svgIcon = icon
	return icon, nil
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
