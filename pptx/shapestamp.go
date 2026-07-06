package pptx

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
)

var shapeElemRe = regexp.MustCompile(`<slidown:shape\b[^>]*/>`)

// emptyNvPrRe matches a self-closing empty <p:nvPr/>, tolerating whitespace
// before the slash (e.g. "<p:nvPr />") as produced by some XML serializers.
var emptyNvPrRe = regexp.MustCompile(`<p:nvPr\s*/>`)

// StampShapeKeys stamps stable keys on keyless non-placeholder top-level shapes
// in every slide part. Stamping is idempotent and only touches shape metadata
// extensions, which are excluded from source fingerprints.
func StampShapeKeys(pptxBytes []byte) ([]byte, error) {
	parts, order, err := readZipPartsFromBytes(pptxBytes)
	if err != nil {
		return nil, err
	}
	slideNames := slideNamesFromPresentationOrder(parts)
	if len(slideNames) == 0 {
		slideNames = slideNamesByFileName(parts)
	}
	changed := false
	for _, name := range slideNames {
		cur := parts[name]
		next := stampSlideShapeKeys(cur)
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

func stampSlideShapeKeys(slideXML []byte) []byte {
	shapes, _, err := parseSlideShapes(slideXML)
	if err != nil {
		return slideXML
	}
	used := map[string]bool{}
	for _, s := range shapes {
		if s.key != "" {
			used[s.key] = true
		}
	}
	type replacement struct {
		start, end int
		data       []byte
	}
	var repls []replacement
	for i, s := range shapes {
		if s.slotKey != "" || s.key != "" {
			continue
		}
		key := uniqueShapeStampKey(s, i, used)
		data, changed := stampShapeKey(s.raw, key)
		if !changed {
			continue
		}
		repls = append(repls, replacement{start: s.start, end: s.end, data: data})
		used[key] = true
	}
	if len(repls) == 0 {
		return slideXML
	}
	var out bytes.Buffer
	pos := 0
	for _, r := range repls {
		if r.start < pos {
			continue
		}
		out.Write(slideXML[pos:r.start])
		out.Write(r.data)
		pos = r.end
	}
	out.Write(slideXML[pos:])
	return out.Bytes()
}

func uniqueShapeStampKey(s shapeInfo, index int, used map[string]bool) string {
	base := "shape#" + s.cNvPrID
	if s.cNvPrID == "" {
		base = "shape#" + strconv.Itoa(index+1)
	}
	key := base
	n := 2
	for used[key] {
		key = base + "-" + strconv.Itoa(n)
		n++
	}
	return key
}

func stampShapeKey(shapeXML []byte, key string) ([]byte, bool) {
	s := string(shapeXML)
	if loc := shapeElemRe.FindStringIndex(s); loc != nil {
		elem := s[loc[0]:loc[1]]
		if rawAttrValue(elem, "sk") != "" {
			return shapeXML, false
		}
		newElem := strings.TrimSuffix(elem, "/>") + ` sk="` + escapeXML(key) + `"/>`
		if newElem == elem {
			return shapeXML, false
		}
		return []byte(s[:loc[0]] + newElem + s[loc[1]:]), true
	}

	ext := `<p:ext uri="` + shapeMetaURI + `">` + shapeMetaElem("", "", key) + `</p:ext>`
	if loc := emptyNvPrRe.FindStringIndex(s); loc != nil {
		return []byte(s[:loc[0]] + `<p:nvPr><p:extLst>` + ext + `</p:extLst></p:nvPr>` + s[loc[1]:]), true
	}
	nvClose := strings.Index(s, `</p:nvPr>`)
	if nvClose < 0 {
		return shapeXML, false
	}
	prefix := s[:nvClose]
	if strings.HasSuffix(strings.TrimSpace(prefix), `</p:extLst>`) {
		insertAt := strings.LastIndex(prefix, `</p:extLst>`)
		if insertAt >= 0 {
			return []byte(s[:insertAt] + ext + s[insertAt:]), true
		}
	}
	return []byte(s[:nvClose] + `<p:extLst>` + ext + `</p:extLst>` + s[nvClose:]), true
}
