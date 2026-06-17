package pptx

import (
	"encoding/xml"
	"sort"
	"strconv"
	"strings"
)

// ReadSlideFingerprints reads the per-slide source fingerprints embedded by
// slidown from an existing .pptx, returned in slide order. A slide without a
// fingerprint (e.g. one re-saved by another tool that dropped the extension)
// yields an empty string at its position.
func ReadSlideFingerprints(path string) ([]string, error) {
	parts, _, err := readZipPartsFromPath(path)
	if err != nil {
		return nil, err
	}

	type idxName struct {
		idx  int
		name string
	}
	var slides []idxName
	for name := range parts {
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") {
			slides = append(slides, idxName{idx: slideNumFromName(name), name: name})
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].idx < slides[j].idx })

	fps := make([]string, 0, len(slides))
	for _, s := range slides {
		fps = append(fps, parseFingerprint(parts[s.name]))
	}
	return fps, nil
}

func slideNumFromName(name string) int {
	base := name[strings.LastIndex(name, "/")+1:]
	base = strings.TrimPrefix(base, "slide")
	base = strings.TrimSuffix(base, ".xml")
	n, _ := strconv.Atoi(base)
	return n
}

func parseFingerprint(slideXML []byte) string {
	var s struct {
		ExtLst struct {
			Ext []struct {
				FP struct {
					V string `xml:"v,attr"`
				} `xml:"fp"`
			} `xml:"ext"`
		} `xml:"extLst"`
	}
	if err := xml.Unmarshal(slideXML, &s); err != nil {
		return ""
	}
	for _, e := range s.ExtLst.Ext {
		if e.FP.V != "" {
			return e.FP.V
		}
	}
	return ""
}
