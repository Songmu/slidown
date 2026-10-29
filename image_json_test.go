package slidown

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestImageJSONRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name, path string
		mime       MIMEType
		w, h       int
	}{
		{"PNG", "testdata/test.png", MIMETypeImagePNG, 400, 400},
		{"JPEG", "testdata/test.jpeg", MIMETypeImageJPEG, 400, 400},
		{"GIF", "testdata/test.gif", MIMETypeImageGIF, 400, 400},
		{"SVG", "", MIMETypeImageSVG, 100, 50},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var img *Image
			raw := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"></svg>`)
			var err error
			if tt.path != "" {
				raw, err = os.ReadFile(tt.path)
				if err != nil {
					t.Fatal(err)
				}
				img, err = NewImageFromMarkdown(tt.path)
			} else {
				img, err = NewImageFromCodeBlock(bytes.NewReader(raw))
			}
			if err != nil {
				t.Fatal(err)
			}
			img.SetLink("https://example.com/image")
			data, err := json.Marshal(img)
			if err != nil {
				t.Fatal(err)
			}
			var got Image
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			for _, candidate := range []*Image{img, &got} {
				if !strings.HasPrefix(candidate.String(), "data:"+string(tt.mime)+";base64,") {
					t.Errorf("String() is missing the %s data URI prefix", tt.mime)
				}
				if len(candidate.Bytes()) == 0 {
					t.Error("Bytes() returned empty data")
				}
			}
			assertImageJSONData(t, &got, img)
			if !bytes.Equal(got.Bytes(), raw) {
				t.Error("roundtrip bytes differ from the original fixture")
			}
			if got.mimeType != tt.mime || got.IsSVG() != (tt.mime == MIMETypeImageSVG) {
				t.Errorf("image type = %q, SVG = %v; want %q", got.mimeType, got.IsSVG(), tt.mime)
			}
			if w, h, err := got.Dimensions(); err != nil || w != tt.w || h != tt.h {
				t.Errorf("Dimensions() = %dx%d, %v; want %dx%d", w, h, err, tt.w, tt.h)
			}
		})
	}
}

func assertImageJSONData(t *testing.T, got, want *Image) {
	t.Helper()
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Error("JSON replacement changed image bytes")
	}
	if got.mimeType != want.mimeType || got.IsSVG() != want.IsSVG() {
		t.Errorf("image type = %q, want %q", got.mimeType, want.mimeType)
	}
	if got.url != want.url || got.fromMarkdown != want.fromMarkdown ||
		!got.modTime.Equal(want.modTime) || got.link != want.link {
		t.Errorf("JSON metadata = (%q, %v, %v, %q), want (%q, %v, %v, %q)",
			got.url, got.fromMarkdown, got.modTime, got.link,
			want.url, want.fromMarkdown, want.modTime, want.link)
	}
}

func TestImageJSONErrors(t *testing.T) {
	dataURI := func(mime MIMEType, data []byte) string {
		return "data:" + string(mime) + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	for _, tt := range []struct {
		name, data, prefix string
		rawJSON            bool
		checkCause         func(error) bool
	}{
		{"malformed JSON", `{"Data":`, "failed to unmarshal image data: ", true, func(err error) bool {
			var cause *json.SyntaxError
			return errors.As(err, &cause)
		}},
		{"wrong JSON field type", `{"Data":1}`, "failed to unmarshal image data: ", true, func(err error) bool {
			var cause *json.UnmarshalTypeError
			return errors.As(err, &cause)
		}},
		{"missing data URI prefix", "image/png;base64,AAAA", "invalid image data: ", false, nil},
		{"missing base64 separator", "data:image/png,AAAA", "invalid image data: ", false, nil},
		{"invalid base64", "data:image/png;base64,!", "failed to decode base64 image data: ", false, func(err error) bool {
			var cause base64.CorruptInputError
			return errors.As(err, &cause)
		}},
		{"invalid raster", dataURI(MIMETypeImagePNG, []byte("not an image")), "failed to decode image: ", false, func(err error) bool {
			return errors.Is(err, image.ErrFormat)
		}},
		{"invalid SVG root", dataURI(MIMETypeImageSVG, []byte(`<html><svg/></html>`)), "invalid SVG image data", false, nil},
		{"MIME mismatch", dataURI(MIMETypeImageJPEG, dummyPNG(t).Bytes()), "image MIME type mismatch: expected image/jpeg, got png", false, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.data)
			if !tt.rawJSON {
				var err error
				data, err = json.Marshal(map[string]string{"Data": tt.data})
				if err != nil {
					t.Fatal(err)
				}
			}
			var got Image
			// Call directly so malformed JSON reaches the custom decoder.
			err := got.UnmarshalJSON(data)
			if err == nil || !strings.HasPrefix(err.Error(), tt.prefix) {
				t.Fatalf("UnmarshalJSON() = %v, want %q prefix", err, tt.prefix)
			}
			if tt.checkCause != nil && !tt.checkCause(err) {
				t.Errorf("error lost expected wrapped cause: %v", err)
			}
		})
	}
}

func TestImageJSONReplacesPopulatedReceiver(t *testing.T) {
	const rectSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 16"><rect width="8" height="16" fill="red"/></svg>`
	const triangleSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 48"><path d="M0 0 L24 48 L0 48Z" fill="blue"/></svg>`
	for _, tt := range []struct {
		name, source, destination string
	}{
		{"raster to SVG", "testdata/test.png", rectSVG},
		{"SVG to SVG", rectSVG, triangleSVG},
		{"SVG to raster", triangleSVG, "testdata/test.jpeg"},
		{"raster to raster", "testdata/test.jpeg", dummyPNG(t).String()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			load := func(source string) *Image {
				t.Helper()
				var img *Image
				var err error
				if strings.HasPrefix(source, "testdata/") {
					img, err = NewImageFromMarkdown(source)
				} else {
					img, err = newImageFromBuffer(strings.NewReader(source))
				}
				if err != nil {
					t.Fatal(err)
				}
				return img
			}
			receiver := load(tt.source)
			receiver.SetLink("https://example.com/old")
			old := observeImageJSON(t, receiver)
			want := load(tt.destination)
			expected := observeImageJSON(t, want)
			if old.width == expected.width && old.height == expected.height {
				t.Fatal("replacement fixture must distinguish cached dimensions from the previous image")
			}
			if old.checksum == expected.checksum || old.pHash == expected.pHash || bytes.Equal(old.pixels, expected.pixels) {
				t.Fatal("replacement fixture must distinguish content caches from the previous image")
			}
			data, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, receiver); err != nil {
				t.Fatal(err)
			}
			assertImageJSONData(t, receiver, want)
			if got := observeImageJSON(t, receiver); !reflect.DeepEqual(got, expected) {
				t.Error("reused receiver dimensions, decoded pixels, checksum or PHash differ from a fresh image")
			}
		})
	}
}

type imageJSONObservation struct {
	width, height int
	pixels        []byte
	checksum      uint32
	pHash         uint64
}

func observeImageJSON(t *testing.T, img *Image) imageJSONObservation {
	t.Helper()
	w, h, err := img.Dimensions()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := img.Image()
	if err != nil {
		t.Fatal(err)
	}
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, decoded); err != nil {
		t.Fatal(err)
	}
	ph, err := img.PHash()
	if err != nil {
		t.Fatal(err)
	}
	return imageJSONObservation{w, h, pixels.Bytes(), img.Checksum(), ph.GetHash()}
}
