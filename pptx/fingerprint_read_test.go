package pptx

import (
	"bytes"
	"testing"

	"github.com/Songmu/slidown/internal/testutil"
)

func TestReadSlideMetasFollowsPresentationSlideOrder(t *testing.T) {
	p := New()
	s1 := p.AddSlide()
	s1.Fingerprint = "fp-1"
	s2 := p.AddSlide()
	s2.Fingerprint = "fp-2"

	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	path := writeTempPPTX(t, reorderPresentationOrder(t, buf.Bytes()))

	metas, err := ReadSlideMetas(path)
	if err != nil {
		t.Fatalf("ReadSlideMetas: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}
	if got, want := metas[0].Fingerprint, "fp-2"; got != want {
		t.Fatalf("first meta fingerprint = %q, want %q", got, want)
	}
	if got, want := metas[1].Fingerprint, "fp-1"; got != want {
		t.Fatalf("second meta fingerprint = %q, want %q", got, want)
	}
}

func reorderPresentationOrder(t *testing.T, in []byte) []byte {
	t.Helper()
	parts, order := testutil.ReadZIP(t, in)
	parts["ppt/presentation.xml"] = testutil.SwapFirstTwoSlideIDs(t, parts["ppt/presentation.xml"])
	return testutil.WriteZIP(t, parts, order)
}
