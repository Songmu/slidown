package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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

	path := filepath.Join(t.TempDir(), "deck.pptx")
	if err := os.WriteFile(path, reorderPresentationOrder(t, buf.Bytes()), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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
	parts, order, err := readZipPartsFromBytes(in)
	if err != nil {
		t.Fatalf("readZipPartsFromBytes: %v", err)
	}
	parts["ppt/presentation.xml"] = reorderPresentationXMLForTest(t, parts["ppt/presentation.xml"])

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, name := range order {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fw.Write(parts[name]); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return out.Bytes()
}

func reorderPresentationXMLForTest(t *testing.T, presentationXML []byte) []byte {
	t.Helper()
	var p struct {
		SlideIDs []struct {
			RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sldIdLst>sldId"`
	}
	if err := xml.Unmarshal(presentationXML, &p); err != nil {
		t.Fatalf("xml.Unmarshal presentation.xml: %v", err)
	}
	if len(p.SlideIDs) < 2 {
		t.Fatalf("presentation.xml has less than 2 slides")
	}
	p.SlideIDs[0], p.SlideIDs[1] = p.SlideIDs[1], p.SlideIDs[0]

	var b strings.Builder
	b.WriteString("<p:sldIdLst>")
	for i, s := range p.SlideIDs {
		b.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="%s"/>`, 256+i, s.RelID))
	}
	b.WriteString("</p:sldIdLst>")

	re := regexp.MustCompile(`(?s)<p:sldIdLst>.*?</p:sldIdLst>`)
	updated := re.ReplaceAll(presentationXML, []byte(b.String()))
	if bytes.Equal(updated, presentationXML) {
		t.Fatalf("failed to rewrite sldIdLst in presentation.xml")
	}
	return updated
}
