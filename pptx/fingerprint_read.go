package pptx

import (
	"encoding/xml"
	"path"
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

	slideNames := slideNamesFromPresentationOrder(parts)
	if len(slideNames) == 0 {
		slideNames = slideNamesByFileName(parts)
	}

	metas := make([]SlideMeta, 0, len(slideNames))
	for _, name := range slideNames {
		metas = append(metas, parseSlideMeta(parts[name]))
	}
	return metas, nil
}

func slideNamesFromPresentationOrder(parts map[string][]byte) []string {
	presentationXML, ok := parts["ppt/presentation.xml"]
	if !ok {
		return nil
	}
	presentationRelsXML, ok := parts["ppt/_rels/presentation.xml.rels"]
	if !ok {
		return nil
	}

	var p struct {
		SlideIDs []struct {
			RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sldIdLst>sldId"`
	}
	if err := xml.Unmarshal(presentationXML, &p); err != nil {
		return nil
	}
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(presentationRelsXML, &rels); err != nil {
		return nil
	}

	targetByID := make(map[string]string, len(rels.Rels))
	for _, r := range rels.Rels {
		if r.ID == "" || r.Target == "" || !strings.HasSuffix(r.Type, "/slide") {
			continue
		}
		if _, exists := targetByID[r.ID]; exists {
			return nil
		}
		targetByID[r.ID] = r.Target
	}

	slideNames := make([]string, 0, len(p.SlideIDs))
	for _, s := range p.SlideIDs {
		target, ok := targetByID[s.RelID]
		if !ok {
			return nil
		}
		slideName := strings.TrimPrefix(target, "/")
		slideName = path.Clean(slideName)
		if !strings.HasPrefix(target, "/") && !strings.HasPrefix(slideName, "ppt/") {
			slideName = path.Clean(path.Join("ppt", slideName))
		}
		if _, ok := parts[slideName]; !ok {
			return nil
		}
		slideNames = append(slideNames, slideName)
	}
	return slideNames
}

func slideNamesByFileName(parts map[string][]byte) []string {
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

	names := make([]string, 0, len(slides))
	for _, s := range slides {
		names = append(names, s.name)
	}
	return names
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
