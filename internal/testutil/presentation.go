package testutil

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// SwapFirstTwoSlideIDs swaps the first two slide relationships in presentation
// XML, renumbering slide IDs while preserving all XML outside the slide list.
func SwapFirstTwoSlideIDs(t testing.TB, presentationXML []byte) []byte {
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
