package slidown

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/corona10/goimagehash"
)

// Incremental rebuild — change detection design
//
// When slidown rebuilds over an existing .pptx, it keeps the existing slide
// part verbatim for any slide whose source has not meaningfully changed, so that
// manual edits made in PowerPoint (and unchanged slides in general) survive.
// Deciding "has this slide's source changed?" reliably is the job of the
// per-slide fingerprint.
//
// Why a fingerprint instead of reverse-parsing the .pptx:
//   - The original (deck) approach reconstructed a slide model from the
//     generated file and compared it to the markdown-derived model. That
//     reverse parse is lossy (it cannot recover rich title formatting, run
//     underline, etc.), so genuine markdown changes could be misread as
//     "unchanged" and silently dropped.
//   - Instead, at generation time slidown embeds a fingerprint of the *source*
//     slide into the slide XML, and on rebuild compares the freshly computed
//     source fingerprint against the embedded one. The comparison is therefore
//     source-to-source and never depends on parsing the rendered output.
//
// What is compared, and how:
//   - Non-image content (layout, titles, subtitles, bodies, block quotes,
//     tables, speaker note, freeze/skip) is hashed and compared *exactly*. Any
//     change — including formatting-only changes such as bold/underline/
//     strikethrough — produces a different hash and forces a regenerate.
//   - Images are compared *perceptually and order-independently*: each image is
//     recorded by its perceptual hash (with a checksum fallback) plus its link,
//     and two slides' image sets are considered equal when every image has a
//     distinct visually-equivalent counterpart in the other set. As a result a
//     slide whose only difference is an image being recompressed, reordered or
//     repositioned counts as unchanged, and its existing slide part is kept
//     rather than regenerated. This mirrors deck's tolerant image handling.
//
// Consequences:
//   - Because comparison is source-to-source, an image that PowerPoint
//     re-compresses inside the .pptx never affects the decision — slidown never
//     reads the embedded image back.
//   - The fingerprint lives in the slide's extLst. If another tool strips it,
//     the slide simply looks "changed" and is regenerated (a safe degradation,
//     never silent data loss of a real change).

// imagePHashThreshold is the maximum perceptual-hash Hamming distance at which
// two images are still treated as the same content. It tolerates recompression
// and other lossy but visually-equivalent changes, mirroring deck's behaviour.
const imagePHashThreshold = 5

// Fingerprint returns the serialized slide signature embedded into the
// generated .pptx so an incremental rebuild can detect whether the source for a
// slide changed, without reverse-parsing the output.
//
// Non-image content is captured by an exact hash, while images are recorded by
// their perceptual hash (falling back to a content checksum when a perceptual
// hash cannot be computed). Image comparison is order-independent and tolerant
// of recompression, so reordering or recompressing images on a slide does not
// force a regenerate.
func (s *Slide) Fingerprint() string {
	if s == nil {
		return ""
	}
	b, err := json.Marshal(s.signature())
	if err != nil {
		return ""
	}
	return string(b)
}

// MatchesFingerprint reports whether this slide's source is equivalent to the
// one captured by a previously embedded fingerprint: non-image content must
// match exactly and the images must match as an order-independent, perceptually
// compared set.
func (s *Slide) MatchesFingerprint(stored string) bool {
	if s == nil || stored == "" {
		return false
	}
	var old slideSignature
	if err := json.Unmarshal([]byte(stored), &old); err != nil {
		return false
	}
	cur := s.signature()
	if cur.Content != old.Content {
		return false
	}
	return imageSetsEquivalent(cur.Images, old.Images)
}

type slideSignature struct {
	Content string           `json:"c"`
	Images  []imageSignature `json:"i,omitempty"`
}

type imageSignature struct {
	PHash    uint64 `json:"p,omitempty"`
	Checksum uint32 `json:"x,omitempty"` // fallback when no perceptual hash is available
	Link     string `json:"l,omitempty"`
}

func (s *Slide) signature() slideSignature {
	sig := slideSignature{Content: s.contentHash()}
	for _, img := range s.Images {
		var is imageSignature
		if img != nil {
			is.Link = img.link
			if ph, err := img.PHash(); err == nil {
				is.PHash = ph.GetHash()
			} else {
				is.Checksum = img.Checksum()
			}
		}
		sig.Images = append(sig.Images, is)
	}
	return sig
}

func (s *Slide) contentHash() string {
	b, err := json.Marshal(slideContent{
		Layout:         s.effectiveLayoutKey(),
		Freeze:         s.Freeze,
		Skip:           s.Skip,
		Titles:         s.Titles,
		TitleBodies:    s.TitleBodies,
		Subtitles:      s.Subtitles,
		SubtitleBodies: s.SubtitleBodies,
		Bodies:         s.Bodies,
		BlockQuotes:    s.BlockQuotes,
		Tables:         s.Tables,
		SpeakerNote:    s.SpeakerNote,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// titleSlotKey is the sentinel folded into the content fingerprint for a slide
// that occupies the title-layout slot only by the built-in first-slide default
// (i.e. it has no explicit or config-assigned layout). The leading NUL cannot
// collide with any authored or template layout name, so it distinguishes a
// default-title slide from a default-content one while leaving the fingerprint
// of every explicitly-laid or non-first slide byte-for-byte unchanged.
const titleSlotKey = "\x00title-slot"

// effectiveLayoutKey returns the layout value hashed into the fingerprint. It is
// the authored layout name, except that a slide taking the title-layout slot
// solely via the first-slide default (empty Layout, TitleSlot set) is
// represented by titleSlotKey so a change in that implicit default forces a
// re-render.
func (s *Slide) effectiveLayoutKey() string {
	if s.Layout == "" && s.TitleSlot {
		return titleSlotKey
	}
	return s.Layout
}

type slideContent struct {
	Layout         string
	Freeze         bool
	Skip           bool
	Titles         []string
	TitleBodies    []*Body
	Subtitles      []string
	SubtitleBodies []*Body
	Bodies         []*Body
	BlockQuotes    []*BlockQuote
	Tables         []*Table
	SpeakerNote    string
}

// imageSetsEquivalent reports whether two image signature sets describe the same
// images, independent of order. Each image in one set must have a distinct
// perceptually-equivalent (and same-linked) counterpart in the other.
func imageSetsEquivalent(a, b []imageSignature) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, ia := range a {
		matched := false
		for j, ib := range b {
			if used[j] {
				continue
			}
			if imageSignaturesEquivalent(ia, ib) {
				used[j] = true
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func imageSignaturesEquivalent(a, b imageSignature) bool {
	if a.Link != b.Link {
		return false
	}
	if a.PHash != 0 && b.PHash != 0 {
		ha := goimagehash.NewImageHash(a.PHash, goimagehash.PHash)
		hb := goimagehash.NewImageHash(b.PHash, goimagehash.PHash)
		d, err := ha.Distance(hb)
		if err != nil {
			return a.PHash == b.PHash
		}
		return d < imagePHashThreshold
	}
	// No perceptual hash on at least one side: fall back to exact equality.
	return a.PHash == b.PHash && a.Checksum == b.Checksum
}
