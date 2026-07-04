package pptx

import (
	"bytes"
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
