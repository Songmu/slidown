package pptx

import (
	"strings"
	"testing"
	"unsafe"
)

func TestEscapeXMLForbiddenControlsRemoved(t *testing.T) {
	var b strings.Builder
	for r := rune(0); r <= 0x1F; r++ {
		b.WriteRune(r)
	}
	got := escapeXML(b.String())
	want := "\t\n\r"
	if got != want {
		t.Fatalf("escapeXML() = %q, want %q", got, want)
	}
}

func TestEscapeXMLSpecialChars(t *testing.T) {
	const in = "&<>\"'"
	const want = "&amp;&lt;&gt;&quot;&apos;"
	if got := escapeXML(in); got != want {
		t.Fatalf("escapeXML() = %q, want %q", got, want)
	}
}

func TestIsForbiddenXMLRune(t *testing.T) {
	for _, r := range []rune{0xD800, 0xDFFF, 0xFFFE, 0xFFFF} {
		if !isForbiddenXMLRune(r) {
			t.Fatalf("rune %U should be forbidden", r)
		}
	}
}

func TestEscapeXMLFastPath(t *testing.T) {
	const in = "ASCII text 123"
	got := escapeXML(in)
	if got != in {
		t.Fatalf("escapeXML() = %q, want %q", got, in)
	}
	if unsafe.StringData(got) != unsafe.StringData(in) {
		t.Fatalf("escapeXML did not use fast path")
	}
}
