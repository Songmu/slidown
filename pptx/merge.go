package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// regeneratedPartPrefixes enumerates the package namespaces that slidown fully
// regenerates from the deck source and the configured template on every apply:
// slides, slide layouts/masters, notes and handout masters, themes and media,
// plus the loose template-hash sentinel written by older versions. An old-only
// part under one of these prefixes (i.e. present in the existing file but not in
// the freshly generated package) is stale — typically an orphan slide, notes
// slide, layout or media part from a shrunk or restructured deck, or the legacy
// ppt/slidownMeta part written by older versions.
// Carrying it over would leave the part undeclared in the regenerated
// [Content_Types].xml and unreferenced by the new master and presentation, which
// PowerPoint reports as unreadable content and strips during a repair. The
// freshly generated package is authoritative for the presentation's structure,
// so these old-only parts must be dropped.
var regeneratedPartPrefixes = []string{
	"ppt/slides/",
	"ppt/slideLayouts/",
	"ppt/slideMasters/",
	"ppt/notesSlides/",
	"ppt/notesMasters/",
	"ppt/handoutMasters/",
	"ppt/theme/",
	"ppt/media/",
	"ppt/slidownMeta",
}

// isRegeneratedPart reports whether name belongs to a namespace that slidown
// regenerates in full on every apply (see regeneratedPartPrefixes).
func isRegeneratedPart(name string) bool {
	for _, prefix := range regeneratedPartPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// MergeWithExisting merges a newly generated pptx package with the contents of
// an existing pptx file. Entries that are unchanged keep their previous payloads
// and any old entries missing from the new package are preserved, except for
// parts under the namespaces slidown fully regenerates (see isRegeneratedPart):
// those old-only parts are stale design/content leftovers and are dropped so
// they cannot accumulate across rebuilds and corrupt the package. This
// keeps the update path stable for slidown-generated decks without requiring a
// fragile reverse parser.
func MergeWithExisting(existingPath string, newPPTX []byte) ([]byte, error) {
	oldParts, oldOrder, err := readZipPartsFromPath(existingPath)
	if err != nil {
		return nil, err
	}
	newParts, newOrder, err := readZipPartsFromBytes(newPPTX)
	if err != nil {
		return nil, err
	}

	merged := make(map[string][]byte, len(oldParts)+len(newParts))
	for name, data := range newParts {
		merged[name] = data
	}
	for name, data := range oldParts {
		if _, ok := merged[name]; ok {
			continue
		}
		if isRegeneratedPart(name) {
			continue
		}
		merged[name] = data
	}

	// Preserve the ordering from the new package and append any extra old
	// entries in lexical order for deterministic output. Old-only parts are
	// added here exactly once, and only when they were kept in `merged` above.
	extras := make([]string, 0, len(oldOrder))
	for _, name := range oldOrder {
		if _, ok := newParts[name]; ok {
			continue
		}
		if _, ok := merged[name]; !ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	newOrder = append(newOrder, extras...)

	return writeZipParts(newOrder, merged)
}

// ReplaceCoreProps returns the existing pptx package with its
// docProps/core.xml replaced by the one from the freshly generated newPPTX,
// preserving every other part verbatim (slides, media, customXml, thumbnails,
// notes, …) and the original part order. It applies a deck-level metadata
// change (e.g. a new title) when every slide is otherwise reused unchanged, so
// the update touches as little of the package as possible.
func ReplaceCoreProps(existingPath string, newPPTX []byte) ([]byte, error) {
	oldParts, oldOrder, err := readZipPartsFromPath(existingPath)
	if err != nil {
		return nil, err
	}
	newParts, _, err := readZipPartsFromBytes(newPPTX)
	if err != nil {
		return nil, err
	}
	const coreName = "docProps/core.xml"
	newCore, ok := newParts[coreName]
	if !ok {
		// The generated package has no core.xml (should not happen); leave the
		// existing package unchanged.
		return writeZipParts(oldOrder, oldParts)
	}
	order := oldOrder
	if _, existed := oldParts[coreName]; !existed {
		order = append(append([]string(nil), oldOrder...), coreName)
	}
	oldParts[coreName] = newCore
	return writeZipParts(order, oldParts)
}

// writeZipParts serializes parts as a ZIP, emitting entries in order (skipping
// names absent from parts) so callers control the part ordering.
func writeZipParts(order []string, parts map[string][]byte) ([]byte, error) {
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

func readZipPartsFromPath(path string) (map[string][]byte, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open pptx %q: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat pptx %q: %w", path, err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read pptx %q: %w", path, err)
	}
	return readZipParts(bytes.NewReader(b), info.Size())
}

func readZipPartsFromBytes(b []byte) (map[string][]byte, []string, error) {
	return readZipParts(bytes.NewReader(b), int64(len(b)))
}

func readZipParts(r io.ReaderAt, size int64) (map[string][]byte, []string, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, nil, err
	}
	parts := make(map[string][]byte, len(zr.File))
	order := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, err
		}
		parts[f.Name] = data
		order = append(order, f.Name)
	}
	return parts, order, nil
}
