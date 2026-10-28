package testutil

import (
	"bytes"
	"testing"
)

func TestSwapFirstTwoSlideIDs(t *testing.T) {
	const prefix = `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:keep/>`
	const suffix = `<p:keepAfter/></p:presentation>`
	input := prefix + `<p:sldIdLst>
<p:sldId id="400" r:id="rId1"/>
<p:sldId id="500" r:id="rId2"/>
<p:sldId id="600" r:id="rId3"/>
</p:sldIdLst>` + suffix
	want := prefix + `<p:sldIdLst><p:sldId id="256" r:id="rId2"/><p:sldId id="257" r:id="rId1"/><p:sldId id="258" r:id="rId3"/></p:sldIdLst>` + suffix
	if got := SwapFirstTwoSlideIDs(t, []byte(input)); !bytes.Equal(got, []byte(want)) {
		t.Fatalf("rewritten XML = %s, want %s", got, want)
	}
}
