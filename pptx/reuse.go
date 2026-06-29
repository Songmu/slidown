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

	// Part name -> content type for restored notes parts that may be missing
	// from the regenerated [Content_Types].xml.
	neededOverrides := map[string]string{}

	// nextMediaIdx tracks the next free imageN index to allocate when a reused
	// slide's media name collides with an already-present part.
	nextMediaIdx := mediaMaxIndex(result)

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
		addPart(newSlideName, oldSlide)

		oldRels, ok := oldParts[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", oldPos)]
		if !ok {
			continue
		}

		// First pass: build a rename map for any media parts whose name is
		// already taken in result by different bytes.  We must compute this
		// before writing the rels so the rels can reference the new names.
		mediaRename := map[string]string{} // old resolved path -> new resolved path
		for _, target := range relTargets(oldRels) {
			resolved := path.Clean(path.Join("ppt/slides", target))
			if !strings.HasPrefix(resolved, "ppt/media/") {
				continue
			}
			oldData, ok := oldParts[resolved]
			if !ok {
				continue
			}
			if existing, exists := result[resolved]; exists && !bytes.Equal(existing, oldData) {
				// Collision: the name is already used for different content.
				// Allocate a fresh name that is guaranteed to be absent.
				ext := path.Ext(resolved)
				for {
					nextMediaIdx++
					candidate := fmt.Sprintf("ppt/media/image%d%s", nextMediaIdx, ext)
					if _, exists := result[candidate]; exists {
						continue
					}
					if _, exists := oldParts[candidate]; exists {
						continue
					}
					// Avoid allocating the same candidate for multiple renames in this pass.
					alreadyAllocated := false
					for _, v := range mediaRename {
						if v == candidate {
							alreadyAllocated = true
							break
						}
					}
					if alreadyAllocated {
						continue
					}
					mediaRename[resolved] = candidate
					break
				}
			}
		}

		// When the slide moves, its reference to the notes slide in the rels
		// must be renumbered to the new position.  Any media renames are applied
		// here too so the rels stay consistent with the actual part names.
		newRels := rewriteRef(oldRels,
			fmt.Sprintf("notesSlide%d.xml", oldPos),
			fmt.Sprintf("notesSlide%d.xml", newPos))
		for oldResolved, newResolved := range mediaRename {
			oldTarget := "../media/" + path.Base(oldResolved)
			newTarget := "../media/" + path.Base(newResolved)
			newRels = []byte(strings.ReplaceAll(string(newRels), oldTarget, newTarget))
		}
		addPart(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", newPos), newRels)

		for _, target := range relTargets(oldRels) {
			resolved := path.Clean(path.Join("ppt/slides", target))
			switch {
			case strings.HasPrefix(resolved, "ppt/media/"):
				data, ok := oldParts[resolved]
				if !ok {
					continue
				}
				destName := resolved
				if newName, ok := mediaRename[resolved]; ok {
					destName = newName
				}
				// Only add if the destination slot is still empty; identical
				// bytes at the same name are already present and need no action.
				if _, exists := result[destName]; !exists {
					order = append(order, destName)
					result[destName] = data
				}
			case strings.HasPrefix(resolved, "ppt/notesSlides/"):
				oldNotes, ok := oldParts[resolved]
				if !ok {
					continue
				}
				newNotes := fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", newPos)
				addPart(newNotes, oldNotes)
				neededOverrides[newNotes] = ctNotesSlide

				oldNotesRels, ok := oldParts[relsPath(resolved)]
				if !ok {
					continue
				}
				// The notes slide's back-reference to its slide must follow the
				// move too.
				newNotesRels := rewriteRef(oldNotesRels,
					fmt.Sprintf("slide%d.xml", oldPos),
					fmt.Sprintf("slide%d.xml", newPos))
				addPart(relsPath(newNotes), newNotesRels)

				// Restore the notes master the notes slide depends on, so the
				// reused note does not leave a dangling relationship when the
				// regenerated package has no notes of its own.
				for _, nt := range relTargets(oldNotesRels) {
					master := path.Clean(path.Join("ppt/notesSlides", nt))
					if !strings.HasPrefix(master, "ppt/notesMasters/") {
						continue
					}
					if mdata, ok := oldParts[master]; ok {
						addPart(master, mdata)
						neededOverrides[master] = ctNotesMaster
					}
					if mrels, ok := oldParts[relsPath(master)]; ok {
						addPart(relsPath(master), mrels)
					}
				}
			}
		}
	}

	// Ensure [Content_Types].xml declares the restored notes parts; otherwise
	// they fall back to the default application/xml type and PowerPoint reports
	// the package as corrupt.
	if ct, ok := result["[Content_Types].xml"]; ok && len(neededOverrides) > 0 {
		result["[Content_Types].xml"] = ensureContentTypeOverrides(ct, neededOverrides)
	}

	return zipFromParts(order, result)
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
