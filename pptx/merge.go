package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
)

// MergeWithExisting merges a newly generated pptx package with the contents of
// an existing pptx file. Entries that are unchanged keep their previous payloads
// and any old entries missing from the new package are preserved. This keeps the
// update path stable for slidown-generated decks without requiring a fragile
// reverse parser.
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
		if _, ok := merged[name]; !ok {
			merged[name] = data
		}
	}

	// Preserve the ordering from the new package and append any extra old
	// entries in lexical order for deterministic output. Old-only parts are
	// added here exactly once.
	extras := make([]string, 0, len(oldOrder))
	for _, name := range oldOrder {
		if _, ok := newParts[name]; ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	newOrder = append(newOrder, extras...)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range newOrder {
		data, ok := merged[name]
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
