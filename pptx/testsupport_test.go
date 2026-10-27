package pptx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Songmu/slidown/internal/testutil"
)

// unzipToMap reads the byte parts of a presentation.
func unzipToMap(t testing.TB, data []byte) map[string][]byte {
	t.Helper()
	parts, _ := testutil.ReadZIP(t, data)
	return parts
}

// unzipToStringMap reads parts for tests that assert on XML text.
func unzipToStringMap(t testing.TB, data []byte) map[string]string {
	t.Helper()
	return testutil.ReadZIPStrings(t, data)
}

// writeToParts serializes a presentation and returns its package parts as a
// name->contents string map, folding the WriteTo + unzip boilerplate.
func writeToParts(t testing.TB, p *Presentation) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return unzipToStringMap(t, buf.Bytes())
}

// writeTempPPTX writes pptx bytes to a fresh temporary file and returns its
// path, for APIs (e.g. MergeWithExisting, LoadTemplate) that take a file path.
func writeTempPPTX(t testing.TB, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deck.pptx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
