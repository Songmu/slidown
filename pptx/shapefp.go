package pptx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

// shapeSlotKey returns the stable per-slide identity of a text shape, used to
// match a shape to its counterpart in an existing slide during an incremental
// shape-level rebuild. It combines the shape's effective semantic type (its
// slidown Role when set, otherwise its OOXML placeholder type) with the
// placeholder index, so a title, a body and each additional body/subtitle slot
// get distinct, position-independent keys.
//
// Only placeholders get a slot key: a non-placeholder text box has no stable
// identity across rebuilds (its index is meaningless), so it returns "" and is
// never spliced, falling back to safe regeneration.
func shapeSlotKey(sh *Shape) string {
	if !sh.isPlaceholder() {
		return ""
	}
	return slotKey(effectiveType(sh.Role, string(sh.Placeholder)), sh.PlaceholderIdx)
}

// effectiveType prefers the slidown role (e.g. "subTitle") over the raw OOXML
// placeholder type so a body placeholder repurposed as a subtitle keeps a
// stable identity across rebuilds.
func effectiveType(role, phType string) string {
	if role != "" {
		return role
	}
	return phType
}

func slotKey(effType string, idx int) string {
	return effType + "#" + strconv.Itoa(idx)
}

// shapeFingerprint returns a hash of a shape's semantic text content: its slot
// key plus every paragraph and run, excluding geometry (x/y/w/h) and the shape
// name. Geometry is deliberately omitted so that manual repositioning in
// PowerPoint does not count as a content change, letting an incremental rebuild
// keep the manually-edited shape when its source text is unchanged.
//
// The fingerprint is embedded in the generated slide (see shapeMetaExt) and
// compared on rebuild. Any change to the text or its inline styling yields a
// different hash and forces the shape to be regenerated.
func shapeFingerprint(sh *Shape) string {
	if sh == nil {
		return ""
	}
	sig := shapeSig{Slot: shapeSlotKey(sh)}
	for _, p := range sh.Paragraphs {
		if p == nil {
			continue
		}
		ps := paraSig{
			Level:    p.Level,
			Bullet:   p.Bullet,
			Numbered: p.Numbered,
			Align:    string(p.Align),
		}
		for _, r := range p.Runs {
			if r == nil {
				continue
			}
			ps.Runs = append(ps.Runs, runSig{
				Text:       r.Text,
				Bold:       r.Bold,
				Italic:     r.Italic,
				Underline:  r.Underline,
				Strike:     r.Strike,
				Code:       r.Code,
				Link:       r.Link,
				FontSize:   r.FontSize,
				Color:      r.Color,
				BgColor:    r.BgColor,
				FontFamily: r.FontFamily,
				Baseline:   r.Baseline,
			})
		}
		sig.Paras = append(sig.Paras, ps)
	}
	b, err := json.Marshal(sig)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type shapeSig struct {
	Slot  string    `json:"s"`
	Paras []paraSig `json:"p,omitempty"`
}

type paraSig struct {
	Level    int      `json:"l,omitempty"`
	Bullet   bool     `json:"b,omitempty"`
	Numbered bool     `json:"n,omitempty"`
	Align    string   `json:"a,omitempty"`
	Runs     []runSig `json:"r,omitempty"`
}

type runSig struct {
	Text       string  `json:"t"`
	Bold       bool    `json:"b,omitempty"`
	Italic     bool    `json:"i,omitempty"`
	Underline  bool    `json:"u,omitempty"`
	Strike     bool    `json:"s,omitempty"`
	Code       bool    `json:"c,omitempty"`
	Link       string  `json:"lk,omitempty"`
	FontSize   float64 `json:"fs,omitempty"`
	Color      string  `json:"co,omitempty"`
	BgColor    string  `json:"bg,omitempty"`
	FontFamily string  `json:"ff,omitempty"`
	Baseline   string  `json:"bl,omitempty"`
}
