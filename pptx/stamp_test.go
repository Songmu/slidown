package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// slideWithMeta builds a minimal slide XML carrying the slidown fingerprint
// extension for the given fingerprint and key (via fingerprintExt).
func slideWithMeta(fp, key string) string {
	return `<p:sld><p:cSld><p:spTree></p:spTree></p:cSld>` + fingerprintExt(fp, key) + `</p:sld>`
}

func TestStampSlideKeyAddsKeyPreservingFingerprint(t *testing.T) {
	got := string(stampSlideKey([]byte(slideWithMeta("fp-1", "")), "overview"))
	if !strings.Contains(got, `k="overview"`) {
		t.Errorf("key not stamped: %s", got)
	}
	if !strings.Contains(got, `v="fp-1"`) {
		t.Errorf("fingerprint not preserved: %s", got)
	}
}

func TestStampSlideKeyUpdatesExistingKey(t *testing.T) {
	got := string(stampSlideKey([]byte(slideWithMeta("fp-1", "old")), "new"))
	if strings.Contains(got, `k="old"`) {
		t.Errorf("old key not replaced: %s", got)
	}
	if !strings.Contains(got, `k="new"`) || !strings.Contains(got, `v="fp-1"`) {
		t.Errorf("key not updated or fingerprint lost: %s", got)
	}
}

func TestStampSlideKeyClearsKeyKeepingFingerprint(t *testing.T) {
	got := string(stampSlideKey([]byte(slideWithMeta("fp-1", "old")), ""))
	if strings.Contains(got, "k=") {
		t.Errorf("key not cleared: %s", got)
	}
	if !strings.Contains(got, `v="fp-1"`) {
		t.Errorf("fingerprint lost while clearing key: %s", got)
	}
}

func TestStampSlideKeyInsertsExtForImportedSlide(t *testing.T) {
	imported := `<p:sld><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`
	got := string(stampSlideKey([]byte(imported), "imported"))
	if !strings.Contains(got, fingerprintURI) || !strings.Contains(got, `k="imported"`) {
		t.Errorf("fp extension not inserted: %s", got)
	}
	if strings.Contains(got, "v=") {
		t.Errorf("imported slide should carry no fingerprint: %s", got)
	}
	// The inserted extLst must be the last child of p:sld.
	if !strings.HasSuffix(got, `</p:extLst></p:sld>`) {
		t.Errorf("extLst not placed as last child of p:sld: %s", got)
	}
}

func TestStampSlideKeyImportedSlideEmptyKeyNoop(t *testing.T) {
	imported := `<p:sld><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`
	got := stampSlideKey([]byte(imported), "")
	if string(got) != imported {
		t.Errorf("empty key on imported slide should be a no-op: %s", got)
	}
}

func TestStampSlideKeyReusesForeignExtLst(t *testing.T) {
	slide := `<p:sld><p:cSld><p:spTree></p:spTree></p:cSld>` +
		`<p:extLst><p:ext uri="{OTHER}"><foo/></p:ext></p:extLst></p:sld>`
	got := string(stampSlideKey([]byte(slide), "k"))
	if strings.Count(got, "<p:extLst>") != 1 {
		t.Errorf("should reuse the existing extLst, not add a second one: %s", got)
	}
	if !strings.Contains(got, "{OTHER}") || !strings.Contains(got, `k="k"`) {
		t.Errorf("foreign ext lost or key not added: %s", got)
	}
}

func TestStampSlideKeyRemovesEmptiedExt(t *testing.T) {
	// A slide previously stamped with a key-only extension gets its key cleared.
	got := string(stampSlideKey([]byte(slideWithMeta("", "only")), ""))
	if strings.Contains(got, fingerprintURI) || strings.Contains(got, "slidown:fp") {
		t.Errorf("emptied fp extension should be removed: %s", got)
	}
	if strings.Contains(got, "<p:extLst></p:extLst>") {
		t.Errorf("emptied extLst should be removed: %s", got)
	}
}

func TestStampSlideKeysEndToEnd(t *testing.T) {
	in := twoSlidePresentationWithFingerprints(t, "fp-1", "fp-2")

	out, err := StampSlideKeys(in, map[int]string{1: "intro", 2: ""})
	if err != nil {
		t.Fatalf("StampSlideKeys: %v", err)
	}

	parts, _, err := readZipPartsFromBytes(out)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes: %v", err)
	}
	metas, _ := slideMetasAndCoreTitle(parts)
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}
	if metas[0].Key != "intro" || metas[0].Fingerprint != "fp-1" {
		t.Errorf("slide 1 meta = %+v, want key=intro fingerprint=fp-1", metas[0])
	}
	if metas[1].Key != "" || metas[1].Fingerprint != "fp-2" {
		t.Errorf("slide 2 meta = %+v, want key='' fingerprint=fp-2", metas[1])
	}
}

func TestStampSlideKeysIdempotent(t *testing.T) {
	in := twoSlidePresentationWithFingerprints(t, "fp-1", "fp-2")
	keys := map[int]string{1: "intro", 2: "outro"}

	once, err := StampSlideKeys(in, keys)
	if err != nil {
		t.Fatalf("first stamp: %v", err)
	}
	twice, err := StampSlideKeys(once, keys)
	if err != nil {
		t.Fatalf("second stamp: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Error("stamping twice with the same keys changed the bytes")
	}
}
