package testutil

import (
	"bytes"
	"slices"
	"testing"
)

func TestRewriteZIPPreservesOrderAndBytes(t *testing.T) {
	parts := map[string][]byte{
		"z.xml": []byte("<original/>"),
		"a.bin": {0, 255, 128, 0},
	}
	parts, order := ReadZIP(t, WriteZIP(t, parts, []string{"z.xml", "a.bin"}))
	parts["z.xml"] = []byte("<changed/>")
	parts["new-b.xml"] = []byte("b")
	parts["new-a.xml"] = []byte("a")
	got, gotOrder := ReadZIP(t, WriteZIP(t, parts, order))
	if !slices.Equal(gotOrder, []string{"z.xml", "a.bin", "new-a.xml", "new-b.xml"}) {
		t.Fatalf("rewritten order = %v", gotOrder)
	}
	for name, want := range parts {
		if !bytes.Equal(got[name], want) {
			t.Errorf("%s = %v, want %v", name, got[name], want)
		}
	}
}

func TestWriteZIPOmitsDeletedAndDuplicateEntries(t *testing.T) {
	data := WriteZIP(t, map[string][]byte{"keep": []byte("text")}, []string{"deleted", "keep", "keep"})
	_, order := ReadZIP(t, data)
	if !slices.Equal(order, []string{"keep"}) {
		t.Fatalf("entry order = %v, want [keep]", order)
	}
	if got := ReadZIPStrings(t, data)["keep"]; got != "text" {
		t.Fatalf("keep = %q, want text", got)
	}
}
