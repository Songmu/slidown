package pptx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// unzipToStringMap unzips a .pptx into a name->contents map with string values,
// for tests that assert on XML text. It builds on unzipToMap.
func unzipToStringMap(t *testing.T, data []byte) map[string]string {
	t.Helper()
	raw := unzipToMap(t, data)
	parts := make(map[string]string, len(raw))
	for name, b := range raw {
		parts[name] = string(b)
	}
	return parts
}

// writeToParts serializes a presentation and returns its package parts as a
// name->contents string map, folding the WriteTo + unzip boilerplate.
func writeToParts(t *testing.T, p *Presentation) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return unzipToStringMap(t, buf.Bytes())
}

// writeTempPPTX writes pptx bytes to a fresh temporary file and returns its
// path, for APIs (e.g. MergeWithExisting, LoadTemplate) that take a file path.
func writeTempPPTX(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deck.pptx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
