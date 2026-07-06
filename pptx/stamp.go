package pptx

import (
	"bytes"
	"regexp"
	"strings"
)

// fpElemRe matches the self-closing slidown fingerprint element embedded in a
// slide's extLst (see fingerprintExt). It is used to rewrite just the stable
// key (the k attribute) while preserving the source fingerprint (v).
var fpElemRe = regexp.MustCompile(`<slidown:fp\b[^>]*/>`)

// StampSlideKeys rewrites the stable key (the k attribute of the slidown
// fingerprint extension) on each slide of the given .pptx bytes so that it
// equals the key of the deck source page occupying the same visible position.
//
// keysByPos maps a 1-based visible slide position (presentation order) to the
// key that slide should carry: a non-empty key is written (adding the extension
// when the slide carries none, e.g. a slide pasted in from another
// presentation), and an empty key clears any key the slide currently has.
// Slides whose position is absent from keysByPos are left untouched.
//
// Treating the deck source as authoritative for keys keeps the embedded key in
// step with the Markdown after a page's key is renamed or removed (so no
// orphaned keys accumulate), and stamps a key onto imported slides so that
// subsequent rebuilds can match them by key rather than by fragile position.
// Only the extLst is touched, which the source fingerprint does not cover, so
// stamping never perturbs change detection and is idempotent.
func StampSlideKeys(pptxBytes []byte, keysByPos map[int]string) ([]byte, error) {
	if len(keysByPos) == 0 {
		return pptxBytes, nil
	}
	parts, order, err := readZipPartsFromBytes(pptxBytes)
	if err != nil {
		return nil, err
	}
	slideNames := slideNamesFromPresentationOrder(parts)
	if len(slideNames) == 0 {
		slideNames = slideNamesByFileName(parts)
	}
	changed := false
	for i, name := range slideNames {
		key, ok := keysByPos[i+1]
		if !ok {
			continue
		}
		cur := parts[name]
		next := stampSlideKey(cur, key)
		if !bytes.Equal(cur, next) {
			parts[name] = next
			changed = true
		}
	}
	if !changed {
		return pptxBytes, nil
	}
	return zipFromParts(order, parts)
}

// stampSlideKey returns slideXML with the slidown fingerprint extension's key
// set to key (cleared when key is empty). The source fingerprint (v) is
// preserved. When the slide has no fingerprint extension and key is non-empty,
// one is inserted; when clearing leaves the extension with neither v nor k, the
// extension (and an emptied slide-level extLst) is removed.
func stampSlideKey(slideXML []byte, key string) []byte {
	s := string(slideXML)
	if loc := fpElemRe.FindStringIndex(s); loc != nil {
		elem := s[loc[0]:loc[1]]
		rawV := rawAttrValue(elem, "v")
		newElem := buildFPElem(rawV, key)
		if newElem == "" {
			return []byte(removeFPExt(s))
		}
		if newElem == elem {
			return slideXML
		}
		return []byte(s[:loc[0]] + newElem + s[loc[1]:])
	}
	if key == "" {
		return slideXML
	}
	return []byte(insertFPExt(s, buildFPElem("", key)))
}

// buildFPElem renders the slidown fingerprint element for the given raw
// (already XML-escaped) fingerprint value and unescaped key, matching the
// format produced by fingerprintExt. It returns "" when both are empty.
func buildFPElem(rawV, key string) string {
	if rawV == "" && key == "" {
		return ""
	}
	attrs := ` xmlns:slidown="` + fingerprintNS + `"`
	if rawV != "" {
		attrs += ` v="` + rawV + `"`
	}
	if key != "" {
		attrs += ` k="` + escapeXML(key) + `"`
	}
	return `<slidown:fp` + attrs + `/>`
}

// insertFPExt inserts a slidown fingerprint extension carrying fpElem into the
// slide's XML. It reuses an existing slide-level extLst (the one immediately
// preceding </p:sld>) when present, otherwise it adds a fresh extLst as the
// last child of p:sld.
func insertFPExt(s, fpElem string) string {
	ext := `<p:ext uri="` + fingerprintURI + `">` + fpElem + `</p:ext>`
	sldClose := strings.LastIndex(s, "</p:sld>")
	if sldClose < 0 {
		return s
	}
	before := strings.TrimRight(s[:sldClose], " \t\r\n")
	if strings.HasSuffix(before, "</p:extLst>") {
		insertAt := strings.LastIndex(s[:sldClose], "</p:extLst>")
		return s[:insertAt] + ext + s[insertAt:]
	}
	return s[:sldClose] + `<p:extLst>` + ext + `</p:extLst>` + s[sldClose:]
}

// removeFPExt deletes the slidown fingerprint extension (identified by its URI)
// from the slide XML, and drops the slide-level extLst if it becomes empty.
func removeFPExt(s string) string {
	open := `<p:ext uri="` + fingerprintURI + `"`
	idx := strings.Index(s, open)
	if idx < 0 {
		return fpElemRe.ReplaceAllString(s, "")
	}
	rel := strings.Index(s[idx:], `</p:ext>`)
	if rel < 0 {
		return fpElemRe.ReplaceAllString(s, "")
	}
	end := idx + rel + len(`</p:ext>`)
	s = s[:idx] + s[end:]
	return strings.ReplaceAll(s, `<p:extLst></p:extLst>`, "")
}

// rawAttrValue returns the raw (still XML-escaped) value of the named attribute
// in a single XML element string, or "" when the attribute is absent.
func rawAttrValue(elem, name string) string {
	marker := " " + name + `="`
	idx := strings.Index(elem, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	end := strings.IndexByte(elem[start:], '"')
	if end < 0 {
		return ""
	}
	return elem[start : start+end]
}
