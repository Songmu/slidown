package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
)

// MergeReusingUnchangedSlides builds the output package from the freshly
// generated newPPTX, but restores the existing file's slide for each entry in
// reuse, a map of new 1-based slide position -> existing slide part name
// (e.g. "ppt/slides/slide3.xml"). Using the part name directly avoids the
// filename-vs-visible-position confusion that arises when slides have been
// reordered in PowerPoint (sldIdLst order ≠ filename order).
// The restored slide (its XML, relationships, any notes slide it references and
// the media those parts use) is copied to the new position, rewriting the
// notes<->slide cross references when the position changes.
//
// This preserves slides whose source did not change (or that are frozen) even
// when other slides are inserted, deleted or reordered, identifying them by
// their stable key rather than by position.
func MergeReusingUnchangedSlides(existingPath string, newPPTX []byte, reuse map[int]string) ([]byte, error) {
	oldParts, _, err := readZipPartsFromPath(existingPath)
	if err != nil {
		return nil, err
	}
	return mergeReusingUnchangedSlides(oldParts, newPPTX, reuse)
}

// mergeReusingUnchangedSlides implements MergeReusingUnchangedSlides on
// already-parsed existing parts. It only reads oldParts, so the caller may keep
// reusing the parsed package afterwards.
func mergeReusingUnchangedSlides(oldParts map[string][]byte, newPPTX []byte, reuse map[int]string) ([]byte, error) {
	newParts, newOrder, err := readZipPartsFromBytes(newPPTX)
	if err != nil {
		return nil, err
	}

	pkg := slideReusePackage{
		oldParts:        oldParts,
		parts:           maps.Clone(newParts),
		order:           append([]string(nil), newOrder...),
		neededOverrides: map[string]string{},
	}
	media := reuseMediaAllocator{
		oldParts: oldParts,
		parts:    pkg.parts,
		nextIdx:  mediaMaxIndex(pkg.parts),
	}

	// Iterate in sorted order so that addPart appends to `order` in a
	// deterministic sequence regardless of map iteration order.
	newPositions := slices.Sorted(maps.Keys(reuse))
	for _, newPos := range newPositions {
		oldSlideName := strings.TrimPrefix(reuse[newPos], "/")
		// Only reuse standard slide parts we can safely remap.
		if !strings.HasPrefix(oldSlideName, "ppt/slides/slide") || !strings.HasSuffix(oldSlideName, ".xml") {
			continue
		}
		newSlideName := fmt.Sprintf("ppt/slides/slide%d.xml", newPos)
		// Skip if newPos is out of range (<=0 or not present in the new PPTX);
		// otherwise we would create an orphan part not referenced by presentation.xml.
		if _, ok := newParts[newSlideName]; !ok {
			continue
		}
		// Derive the numeric part of the old slide name for rewriting
		// cross-references in rels (e.g. notesSlide back-links).
		oldPos := slideNumFromName(oldSlideName)
		if oldPos <= 0 {
			continue
		}
		oldSlide, ok := oldParts[oldSlideName]
		if !ok {
			// The existing file does not carry this slide; leave the freshly
			// generated one in place.
			continue
		}
		pkg.addPart(newSlideName, oldSlide)

		oldRels, ok := oldParts[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", oldPos)]
		if !ok {
			continue
		}

		mediaRename := media.renames(oldRels)
		newRels := pkg.restoreRelationships(oldRels, oldPos, newPos, mediaRename)
		pkg.addPart(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", newPos), newRels)
	}

	// Ensure [Content_Types].xml declares the restored notes parts; otherwise
	// they fall back to the default application/xml type and PowerPoint reports
	// the package as corrupt.
	if ct, ok := pkg.parts["[Content_Types].xml"]; ok && len(pkg.neededOverrides) > 0 {
		pkg.parts["[Content_Types].xml"] = ensureContentTypeOverrides(ct, pkg.neededOverrides)
	}

	return zipFromParts(pkg.order, pkg.parts)
}

type slideReusePackage struct {
	oldParts        map[string][]byte
	parts           map[string][]byte
	order           []string
	neededOverrides map[string]string
}

func (p *slideReusePackage) addPart(name string, data []byte) {
	if _, ok := p.parts[name]; !ok {
		p.order = append(p.order, name)
	}
	p.parts[name] = data
}

func (p *slideReusePackage) restoreRelationships(oldRels []byte, oldPos, newPos int, mediaRename map[string]string) []byte {
	newRels := oldRels
	for oldResolved, newResolved := range mediaRename {
		oldTarget := "../media/" + path.Base(oldResolved)
		newTarget := "../media/" + path.Base(newResolved)
		newRels = []byte(strings.ReplaceAll(string(newRels), oldTarget, newTarget))
	}
	for _, target := range relTargets(oldRels) {
		resolved := path.Clean(path.Join("ppt/slides", target))
		switch {
		case strings.HasPrefix(resolved, "ppt/media/"):
			p.restoreMedia(resolved, mediaRename)
		case strings.HasPrefix(resolved, "ppt/notesSlides/"):
			if newNotes := p.restoreNotes(resolved, oldPos, newPos); newNotes != "" {
				// Notes part numbers need not match their owning slide's number.
				newRels = rewriteRef(newRels, target, "../notesSlides/"+path.Base(newNotes))
			}
		}
	}
	return newRels
}

func (p *slideReusePackage) restoreMedia(resolved string, mediaRename map[string]string) {
	data, ok := p.oldParts[resolved]
	if !ok {
		return
	}
	destName := resolved
	if newName, ok := mediaRename[resolved]; ok {
		destName = newName
	}
	// Identical bytes at the same name are already present and need no action.
	if _, exists := p.parts[destName]; !exists {
		p.addPart(destName, data)
	}
}

func (p *slideReusePackage) restoreNotes(resolved string, oldPos, newPos int) string {
	oldNotes, ok := p.oldParts[resolved]
	if !ok {
		return ""
	}
	newNotes := fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", newPos)
	p.addPart(newNotes, oldNotes)
	p.neededOverrides[newNotes] = ctNotesSlide

	oldNotesRels, ok := p.oldParts[relsPath(resolved)]
	if !ok {
		return newNotes
	}
	// The notes slide's back-reference to its slide must follow the move too.
	newNotesRels := rewriteRef(oldNotesRels,
		fmt.Sprintf("slide%d.xml", oldPos),
		fmt.Sprintf("slide%d.xml", newPos))
	p.addPart(relsPath(newNotes), newNotesRels)
	p.restoreNotesMasters(oldNotesRels)
	return newNotes
}

func (p *slideReusePackage) restoreNotesMasters(oldNotesRels []byte) {
	// A regenerated package without notes may be missing these dependencies.
	for _, target := range relTargets(oldNotesRels) {
		master := path.Clean(path.Join("ppt/notesSlides", target))
		if !strings.HasPrefix(master, "ppt/notesMasters/") {
			continue
		}
		if data, ok := p.oldParts[master]; ok {
			p.addPart(master, data)
			p.neededOverrides[master] = ctNotesMaster
		}
		if rels, ok := p.oldParts[relsPath(master)]; ok {
			p.addPart(relsPath(master), rels)
		}
	}
}

type reuseMediaAllocator struct {
	oldParts map[string][]byte
	parts    map[string][]byte
	nextIdx  int
}

// renames allocates collision-free media names before relationships are written.
func (m *reuseMediaAllocator) renames(oldRels []byte) map[string]string {
	renames := map[string]string{}
	reserved := map[string]bool{}
	for _, target := range relTargets(oldRels) {
		resolved := path.Clean(path.Join("ppt/slides", target))
		if !strings.HasPrefix(resolved, "ppt/media/") {
			continue
		}
		oldData, ok := m.oldParts[resolved]
		if !ok {
			continue
		}
		if existing, exists := m.parts[resolved]; exists && !bytes.Equal(existing, oldData) {
			name := m.allocate(path.Ext(resolved), reserved)
			renames[resolved] = name
			reserved[name] = true
		}
	}
	return renames
}

func (m *reuseMediaAllocator) allocate(ext string, reserved map[string]bool) string {
	for {
		m.nextIdx++
		candidate := fmt.Sprintf("ppt/media/image%d%s", m.nextIdx, ext)
		if _, exists := m.parts[candidate]; exists {
			continue
		}
		if _, exists := m.oldParts[candidate]; exists {
			continue
		}
		if reserved[candidate] {
			continue
		}
		return candidate
	}
}

// rewriteRef replaces occurrences of the oldRef path segment with newRef in a
// relationships part. The references are full file names (e.g. "slide3.xml"),
// so replacement is unambiguous; when oldRef == newRef it is a no-op.
func rewriteRef(data []byte, oldRef, newRef string) []byte {
	if oldRef == newRef {
		return data
	}
	return []byte(strings.ReplaceAll(string(data), oldRef, newRef))
}

const (
	ctNotesSlide  = "application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"
	ctNotesMaster = "application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"
)

// ensureContentTypeOverrides injects an <Override> for each part that is not
// already declared in the [Content_Types].xml document.
func ensureContentTypeOverrides(contentTypes []byte, overrides map[string]string) []byte {
	s := string(contentTypes)
	parts := slices.Sorted(maps.Keys(overrides))
	var inject strings.Builder
	for _, part := range parts {
		ct := overrides[part]
		decl := fmt.Sprintf(`PartName="/%s"`, part)
		if strings.Contains(s, decl) {
			continue
		}
		inject.WriteString(fmt.Sprintf(`<Override PartName="/%s" ContentType="%s"/>`, part, ct))
	}
	if inject.Len() == 0 {
		return contentTypes
	}
	if idx := strings.LastIndex(s, "</Types>"); idx >= 0 {
		return []byte(s[:idx] + inject.String() + s[idx:])
	}
	return contentTypes
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

// mediaMaxIndex returns the highest imageN index already present in parts so
// fresh names can be allocated starting above that watermark.
func mediaMaxIndex(parts map[string][]byte) int {
	max := 0
	for name := range parts {
		if !strings.HasPrefix(name, "ppt/media/image") {
			continue
		}
		base := strings.TrimPrefix(path.Base(name), "image")
		if dot := strings.IndexByte(base, '.'); dot >= 0 {
			base = base[:dot]
		}
		if v := atoi(base); v > max {
			max = v
		}
	}
	return max
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
