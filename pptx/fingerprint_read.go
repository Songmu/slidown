package pptx

import (
	"encoding/xml"
	"sort"
	"strconv"
	"strings"
)

// SlideMeta is the slidown metadata embedded in a generated slide: its source
// fingerprint and optional stable key.
type SlideMeta struct {
	Fingerprint string
	Key         string
}

// ReadSlideMetas reads the per-slide slidown metadata embedded in an existing
// .pptx, returned in slide order. A slide without the metadata (e.g. one
// re-saved by another tool that dropped the extension) yields a zero SlideMeta
// at its position.
func ReadSlideMetas(path string) ([]SlideMeta, error) {
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

	metas := make([]SlideMeta, 0, len(slides))
	for _, s := range slides {
		metas = append(metas, parseSlideMeta(parts[s.name]))
	}
	return metas, nil
}

func slideNumFromName(name string) int {
	base := name[strings.LastIndex(name, "/")+1:]
	base = strings.TrimPrefix(base, "slide")
	base = strings.TrimSuffix(base, ".xml")
	n, _ := strconv.Atoi(base)
	return n
}

func parseSlideMeta(slideXML []byte) SlideMeta {
	var s struct {
		ExtLst struct {
			Ext []struct {
				FP struct {
					V string `xml:"v,attr"`
					K string `xml:"k,attr"`
				} `xml:"fp"`
			} `xml:"ext"`
		} `xml:"extLst"`
	}
	if err := xml.Unmarshal(slideXML, &s); err != nil {
		return SlideMeta{}
	}
	for _, e := range s.ExtLst.Ext {
		if e.FP.V != "" {
			return SlideMeta{Fingerprint: e.FP.V, Key: e.FP.K}
		}
	}
	return SlideMeta{}
}
