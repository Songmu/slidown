package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// MergeReusingUnchangedSlides builds the output package from the freshly
// generated newPPTX, but restores the existing file's parts verbatim for the
// given 1-based slide positions (the slide XML, its relationships, any notes
// slide it references and the media those parts use).
//
// This preserves manual edits to slides whose source content did not change,
// mirroring deck's behaviour of leaving unchanged slides untouched. The caller
// must guarantee that the slide count is identical between the existing file and
// newPPTX and that positions are stable; otherwise reuse is unsafe and
// MergeWithExisting should be used instead.
func MergeReusingUnchangedSlides(existingPath string, newPPTX []byte, reuseSlideNums []int) ([]byte, error) {
	oldParts, _, err := readZipPartsFromPath(existingPath)
	if err != nil {
		return nil, err
	}
	newParts, newOrder, err := readZipPartsFromBytes(newPPTX)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte, len(newParts))
	for name, data := range newParts {
		result[name] = data
	}
	order := append([]string(nil), newOrder...)

	addPart := func(name string, data []byte) {
		if _, ok := result[name]; !ok {
			order = append(order, name)
		}
		result[name] = data
	}

	for _, num := range reuseSlideNums {
		slideName := fmt.Sprintf("ppt/slides/slide%d.xml", num)
		relsName := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", num)

		oldSlide, ok := oldParts[slideName]
		if !ok {
			// The existing file does not carry this slide part; leave the freshly
			// generated one in place.
			continue
		}
		addPart(slideName, oldSlide)

		oldRels, ok := oldParts[relsName]
		if !ok {
			continue
		}
		addPart(relsName, oldRels)

		for _, target := range relTargets(oldRels) {
			resolved := path.Clean(path.Join("ppt/slides", target))
			switch {
			case strings.HasPrefix(resolved, "ppt/media/"):
				if data, ok := oldParts[resolved]; ok {
					addPart(resolved, data)
				}
			case strings.HasPrefix(resolved, "ppt/notesSlides/"):
				if data, ok := oldParts[resolved]; ok {
					addPart(resolved, data)
				}
				notesRels := relsPath(resolved)
				if data, ok := oldParts[notesRels]; ok {
					addPart(notesRels, data)
				}
			}
		}
	}

	return zipFromParts(order, result)
}

// relTargets returns the (internal) relationship targets declared in a .rels
// part, skipping external relationships such as hyperlinks.
func relTargets(relsXML []byte) []string {
	var rels struct {
		Relationships []struct {
			Target     string `xml:"Target,attr"`
			TargetMode string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(relsXML, &rels); err != nil {
		return nil
	}
	var targets []string
	for _, r := range rels.Relationships {
		if strings.EqualFold(r.TargetMode, "External") {
			continue
		}
		if r.Target == "" {
			continue
		}
		targets = append(targets, r.Target)
	}
	return targets
}

// zipFromParts serializes parts into a .pptx ZIP archive following the given
// name order.
func zipFromParts(order []string, parts map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		data, ok := parts[name]
		if !ok {
			continue
		}
		fw, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
