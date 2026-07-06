package pptx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Namespace URIs used when parsing slide XML for shape-level incremental reuse.
const (
	nsPresentationML = "http://schemas.openxmlformats.org/presentationml/2006/main"
	nsRelationships  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

var (
	xnSp       = xml.Name{Space: nsPresentationML, Local: "sp"}
	xnSpTree   = xml.Name{Space: nsPresentationML, Local: "spTree"}
	xnCNvPr    = xml.Name{Space: nsPresentationML, Local: "cNvPr"}
	xnPh       = xml.Name{Space: nsPresentationML, Local: "ph"}
	xnSlidownS = xml.Name{Space: fingerprintNS, Local: "shape"}
)

// shapeInfo describes one top-level <p:sp> in a slide: its stable slot key, the
// slidown per-shape fingerprint, whether it references any relationships (which
// makes it ineligible for cross-slide splicing), its cNvPr id, and the raw XML
// byte range it occupies within the slide document.
type shapeInfo struct {
	slotKey string
	key     string
	fp      string
	hasRels bool
	cNvPrID string
	start   int
	end     int
	raw     []byte
}

// parseSlideShapes walks a slide XML document and returns the ordered top-level
// shapes (direct children of <p:spTree>) together with the root element's
// attributes (used for namespace reconciliation). Shapes nested inside group
// shapes are intentionally ignored; only spliceable top-level text boxes are
// returned.
func parseSlideShapes(slideXML []byte) (shapes []shapeInfo, rootAttrs []xml.Attr, err error) {
	dec := xml.NewDecoder(bytes.NewReader(slideXML))
	var stack []xml.Name
	var prev int64
	firstElem := true
	capturing := false
	depthAtSp := 0
	var cur shapeInfo
	var phType, phIdx, role, sk string
	var sawPh bool

	finalize := func(end int64) {
		cur.end = int(end)
		cur.raw = slideXML[cur.start:cur.end]
		// Placeholders keep their stable slot key, while non-placeholder text
		// boxes can be keyed by a stamped shape key (sk) so they can be preserved
		// across rebuilds even when absent from freshly generated slide XML.
		if sawPh {
			cur.slotKey = slotKey(effectiveType(role, phType), atoi(phIdx))
		}
		cur.key = cur.slotKey
		if cur.key == "" {
			cur.key = sk
		}
		// Namespace-aware detection above is authoritative; the substring scan is
		// a conservative backstop (a false positive only skips an optimisation).
		cur.hasRels = cur.hasRels || rawHasRels(cur.raw)
		shapes = append(shapes, cur)
	}

	for {
		tok, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, e
		}
		off := dec.InputOffset()
		switch t := tok.(type) {
		case xml.StartElement:
			if firstElem {
				rootAttrs = append([]xml.Attr(nil), t.Attr...)
				firstElem = false
			}
			parentIsSpTree := len(stack) > 0 && stack[len(stack)-1] == xnSpTree
			stack = append(stack, t.Name)
			switch {
			case !capturing && t.Name == xnSp && parentIsSpTree:
				capturing = true
				depthAtSp = len(stack)
				cur = shapeInfo{start: int(prev)}
				phType, phIdx, role, sk = "", "", "", ""
				sawPh = false
			case capturing:
				for _, a := range t.Attr {
					if a.Name.Space == nsRelationships {
						cur.hasRels = true
					}
				}
				switch t.Name {
				case xnCNvPr:
					if cur.cNvPrID == "" {
						cur.cNvPrID = attrValue(t.Attr, "id")
					}
				case xnPh:
					sawPh = true
					if phType == "" {
						phType = attrValue(t.Attr, "type")
					}
					if phIdx == "" {
						phIdx = attrValue(t.Attr, "idx")
					}
				case xnSlidownS:
					if role == "" {
						role = attrValue(t.Attr, "role")
					}
					if cur.fp == "" {
						cur.fp = attrValue(t.Attr, "fp")
					}
					if sk == "" {
						sk = attrValue(t.Attr, "sk")
					}
				}
			}
		case xml.EndElement:
			if capturing && t.Name == xnSp && len(stack) == depthAtSp {
				finalize(off)
				capturing = false
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
		prev = off
	}
	return shapes, rootAttrs, nil
}

// attrValue returns the value of the first attribute with the given local name,
// ignoring namespace (the attributes read here — id, type, idx, role, fp — are
// all unprefixed).
func attrValue(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// rawHasRels reports whether a shape's raw XML references any package
// relationship (hyperlink, embedded media, etc.). Detection is deliberately
// broad: a false positive only makes the shape ineligible for splicing (a safe
// fallback to regeneration), never a corruption.
func rawHasRels(raw []byte) bool {
	s := string(raw)
	for _, marker := range []string{"r:id=", "r:embed=", "r:link=", "hlinkClick", "hlinkHover", "hlinkMouseOver", "<a:blip"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

var cNvPrIDRe = regexp.MustCompile(`(<[A-Za-z0-9]*:?cNvPr\b[^>]*?\bid\s*=\s*["'])[^"']*(["'])`)

// rewriteCNvPrID replaces the id attribute on a shape's first cNvPr element so a
// spliced-in existing shape adopts the id assigned to it in the new slide,
// keeping shape ids unique within the slide. It rewrites only the first cNvPr
// (the shape's own non-visual id) and reports whether the rewrite succeeded, so
// a caller can refuse to splice a shape whose id could not be normalised.
func rewriteCNvPrID(raw []byte, newID string) ([]byte, bool) {
	if newID == "" {
		return raw, false
	}
	loc := cNvPrIDRe.FindSubmatchIndex(raw)
	if loc == nil {
		return raw, false
	}
	var out bytes.Buffer
	out.Write(raw[:loc[3]]) // up to and including group 1 (…id=")
	out.WriteString(newID)  // new id value
	out.Write(raw[loc[4]:]) // from group 2 (closing quote) to end
	return out.Bytes(), true
}

// mergeSlideShapes rebuilds newSld (the freshly generated slide, which is
// self-consistent for content, relationships and media) by splicing in the
// existing slide's shape for every top-level text box whose slot key and
// per-shape fingerprint are unchanged. This preserves manual edits (position,
// size, formatting) made in PowerPoint on shapes whose source did not change,
// while regenerated shapes and non-shape content (pictures, tables) come from
// the new slide.
//
// Only relationship-free shapes are spliced on both sides, so no relationship
// or media remapping is needed. Namespace declarations present on the existing
// slide root are merged into the new slide root so any exotic namespaces used
// inside a spliced shape stay resolvable.
func mergeSlideShapes(newSld, oldSld []byte) ([]byte, bool) {
	newShapes, newRoot, err := parseSlideShapes(newSld)
	if err != nil {
		return newSld, false
	}
	oldShapes, oldRoot, err := parseSlideShapes(oldSld)
	if err != nil {
		return newSld, false
	}

	// Index rel-free existing shapes by slot key, dropping any slot key that is
	// ambiguous (appears more than once) so a merge never picks the wrong one.
	oldByKey := map[string]shapeInfo{}
	ambiguous := map[string]bool{}
	for _, s := range oldShapes {
		if s.hasRels || s.key == "" {
			continue
		}
		if _, seen := oldByKey[s.key]; seen {
			ambiguous[s.key] = true
			continue
		}
		oldByKey[s.key] = s
	}

	type replacement struct {
		start, end int
		data       []byte
	}
	var repls []replacement
	used := map[string]bool{}
	newKeys := map[string]bool{}
	for _, ns := range newShapes {
		if ns.key != "" {
			newKeys[ns.key] = true
		}
	}
	for _, ns := range newShapes {
		if ns.hasRels || ns.fp == "" || ns.key == "" {
			continue
		}
		if ambiguous[ns.key] || used[ns.key] {
			continue
		}
		os, ok := oldByKey[ns.key]
		if !ok || os.fp == "" || os.fp != ns.fp {
			continue
		}
		data, ok := rewriteCNvPrID(os.raw, ns.cNvPrID)
		if !ok {
			continue // cannot normalise the shape id; keep the new shape
		}
		repls = append(repls, replacement{
			start: ns.start,
			end:   ns.end,
			data:  data,
		})
		used[ns.key] = true
	}

	maxID, haveID := maxShapeID(newShapes)
	var carry []byte
	for _, os := range oldShapes {
		if os.hasRels || os.key == "" || ambiguous[os.key] || newKeys[os.key] || used[os.key] {
			continue
		}
		if !haveID {
			continue
		}
		maxID++
		data, ok := rewriteCNvPrID(os.raw, strconv.Itoa(maxID))
		if !ok {
			continue
		}
		carry = append(carry, data...)
	}
	if len(repls) == 0 && len(carry) == 0 {
		return newSld, false
	}

	sort.Slice(repls, func(i, j int) bool { return repls[i].start < repls[j].start })
	var out bytes.Buffer
	pos := 0
	for _, r := range repls {
		if r.start < pos {
			continue // overlapping (shouldn't happen); skip defensively
		}
		out.Write(newSld[pos:r.start])
		out.Write(r.data)
		pos = r.end
	}
	out.Write(newSld[pos:])

	merged := out.Bytes()
	if len(carry) > 0 {
		withCarry, ok := appendShapesToSpTree(merged, carry)
		if ok {
			merged = withCarry
		}
	}
	return unionRootNamespaces(merged, newRoot, oldRoot), true
}

func maxShapeID(shapes []shapeInfo) (int, bool) {
	maxID := 0
	ok := false
	for _, s := range shapes {
		id, err := strconv.Atoi(s.cNvPrID)
		if err != nil {
			continue
		}
		if !ok || id > maxID {
			maxID = id
			ok = true
		}
	}
	return maxID, ok
}

func appendShapesToSpTree(slideXML, shapes []byte) ([]byte, bool) {
	idx := bytes.Index(slideXML, []byte(`</p:spTree>`))
	if idx < 0 {
		return slideXML, false
	}
	var out bytes.Buffer
	out.Write(slideXML[:idx])
	out.Write(shapes)
	out.Write(slideXML[idx:])
	return out.Bytes(), true
}

// unionRootNamespaces injects into the slide root any xmlns declaration present
// on the existing slide root but missing on the new slide root, so namespaces
// referenced inside a spliced existing shape remain declared.
func unionRootNamespaces(slideXML []byte, newRoot, oldRoot []xml.Attr) []byte {
	have := map[string]bool{}
	for _, a := range newRoot {
		if a.Name.Space == "xmlns" {
			have[a.Name.Local] = true
		} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
			have[""] = true
		}
	}
	var inject strings.Builder
	for _, a := range oldRoot {
		switch {
		case a.Name.Space == "xmlns":
			if !have[a.Name.Local] {
				have[a.Name.Local] = true
				inject.WriteString(fmt.Sprintf(` xmlns:%s="%s"`, a.Name.Local, escapeXML(a.Value)))
			}
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			if !have[""] {
				have[""] = true
				inject.WriteString(fmt.Sprintf(` xmlns="%s"`, escapeXML(a.Value)))
			}
		}
	}
	if inject.Len() == 0 {
		return slideXML
	}
	// Insert the declarations just before the '>' that closes the root start tag.
	idx := bytes.IndexByte(slideXML, '>')
	if idx < 0 {
		return slideXML
	}
	// Guard against a self-closing or PI '>'; the root <p:sld ...> start tag is
	// a normal element, but be safe if the first '>' closes the XML declaration.
	if idx > 0 && slideXML[idx-1] == '?' {
		next := bytes.IndexByte(slideXML[idx+1:], '>')
		if next < 0 {
			return slideXML
		}
		idx = idx + 1 + next
	}
	var out bytes.Buffer
	out.Write(slideXML[:idx])
	out.WriteString(inject.String())
	out.Write(slideXML[idx:])
	return out.Bytes()
}

// ShapeSignature is a shape's slot key and per-shape fingerprint, used by the
// slide aligner to gauge how similar two slides are before merging shapes.
type ShapeSignature struct {
	SlotKey string
	FP      string
}

func shapeSignatures(slideXML []byte) []ShapeSignature {
	shapes, _, err := parseSlideShapes(slideXML)
	if err != nil {
		return nil
	}
	sigs := make([]ShapeSignature, 0, len(shapes))
	for _, s := range shapes {
		if s.fp == "" || s.slotKey == "" {
			continue
		}
		sigs = append(sigs, ShapeSignature{SlotKey: s.slotKey, FP: s.fp})
	}
	return sigs
}

// ShapeSignaturesByPosition returns, for a .pptx package, the shape signatures
// of every slide keyed by its 1-based position in presentation order.
func ShapeSignaturesByPosition(pkg []byte) (map[int][]ShapeSignature, error) {
	parts, _, err := readZipPartsFromBytes(pkg)
	if err != nil {
		return nil, err
	}
	names := slideNamesFromPresentationOrder(parts)
	if len(names) == 0 {
		names = slideNamesByFileName(parts)
	}
	out := make(map[int][]ShapeSignature, len(names))
	for i, name := range names {
		out[i+1] = shapeSignatures(parts[name])
	}
	return out, nil
}

// ShapeSignaturesByPart returns the shape signatures of every slide in an
// on-disk .pptx keyed by the slide's ZIP part name (e.g. ppt/slides/slide3.xml).
func ShapeSignaturesByPart(path string) (map[string][]ShapeSignature, error) {
	parts, _, err := readZipPartsFromPath(path)
	if err != nil {
		return nil, err
	}
	return shapeSignaturesByPart(parts), nil
}

// shapeSignaturesByPart implements ShapeSignaturesByPart on already-parsed
// package parts, so a caller that has read the .pptx once can reuse the parts.
func shapeSignaturesByPart(parts map[string][]byte) map[string][]ShapeSignature {
	out := map[string][]ShapeSignature{}
	for name, data := range parts {
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") {
			out[name] = shapeSignatures(data)
		}
	}
	return out
}

// ShapeOverlap returns the fraction of shapes shared by two slides, matching by
// (slot key, fingerprint), over the larger shape count. It is 1.0 when the
// slides have identical shape sets and 0 when they share nothing. The aligner
// uses it as a confidence gate before merging shapes between a source slide and
// its existing counterpart.
func ShapeOverlap(a, b []ShapeSignature) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	larger := len(a)
	if len(b) > larger {
		larger = len(b)
	}
	if larger == 0 {
		return 0
	}
	used := make([]bool, len(b))
	matched := 0
	for _, sa := range a {
		for j, sb := range b {
			if used[j] {
				continue
			}
			if sa.SlotKey == sb.SlotKey && sa.FP == sb.FP {
				used[j] = true
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(larger)
}

// MergeReusingUnchangedShapes rewrites, within pkg, the slides named by targets
// (a map of new 1-based slide position -> existing slide part name) so that each
// unchanged text box is restored from the existing slide while the rest of the
// slide (changed shapes, pictures, tables, relationships, media) stays as freshly
// generated. Slides not in targets are left untouched.
func MergeReusingUnchangedShapes(pkg []byte, existingPath string, targets map[int]string) ([]byte, error) {
	if len(targets) == 0 {
		return pkg, nil
	}
	oldParts, _, err := readZipPartsFromPath(existingPath)
	if err != nil {
		return nil, err
	}
	return mergeReusingUnchangedShapes(pkg, oldParts, targets)
}

// mergeReusingUnchangedShapes implements MergeReusingUnchangedShapes on
// already-parsed existing parts. It only reads oldParts, so the caller may keep
// reusing the parsed package afterwards.
func mergeReusingUnchangedShapes(pkg []byte, oldParts map[string][]byte, targets map[int]string) ([]byte, error) {
	if len(targets) == 0 {
		return pkg, nil
	}
	parts, order, err := readZipPartsFromBytes(pkg)
	if err != nil {
		return nil, err
	}
	anyChange := false
	for newPos, oldName := range targets {
		newName := fmt.Sprintf("ppt/slides/slide%d.xml", newPos)
		newSld, ok := parts[newName]
		if !ok {
			continue
		}
		oldSld, ok := oldParts[strings.TrimPrefix(oldName, "/")]
		if !ok {
			continue
		}
		merged, changed := mergeSlideShapes(newSld, oldSld)
		if changed {
			parts[newName] = merged
			anyChange = true
		}
	}
	if !anyChange {
		return pkg, nil
	}
	return zipFromParts(order, parts)
}
