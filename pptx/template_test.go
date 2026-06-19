package pptx

import (
	"testing"
)

func TestParseLayoutExtractsPlaceholderNameAndPrompt(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="obj">
  <p:cSld name="Custom">
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="2" name="Title 1"/>
          <p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>
          <p:nvPr><p:ph type="title"/></p:nvPr>
        </p:nvSpPr>
        <p:spPr/>
        <p:txBody><a:bodyPr/><a:lstStyle/>
          <a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master title style</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="3" name="Subtitle 2"/>
          <p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>
          <p:nvPr><p:ph type="body" idx="1" hasCustomPrompt="1"/></p:nvPr>
        </p:nvSpPr>
        <p:spPr/>
        <p:txBody><a:bodyPr/><a:lstStyle/>
          <a:p><a:r><a:rPr lang="en-US"/><a:t>Add Subtitle here</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="4" name="Content Placeholder 3"/>
          <p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>
          <p:nvPr><p:ph type="body" idx="2"/></p:nvPr>
        </p:nvSpPr>
        <p:spPr/>
        <p:txBody><a:bodyPr/><a:lstStyle/>
          <a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit body</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`)

	li := parseLayout("ppt/slideLayouts/slideLayoutX.xml", xmlData)
	if li == nil {
		t.Fatal("parseLayout returned nil")
	}
	if got, want := len(li.Placeholders), 3; got != want {
		t.Fatalf("got %d placeholders, want %d", got, want)
	}
	cases := []struct {
		idx    int
		wantNm string
		wantPr string
	}{
		{0, "Title 1", "Click to edit Master title style"},
		{1, "Subtitle 2", "Add Subtitle here"},
		{2, "Content Placeholder 3", "Click to edit body"},
	}
	for _, c := range cases {
		ph := li.Placeholders[c.idx]
		if ph.Name != c.wantNm {
			t.Errorf("placeholder %d Name = %q, want %q", c.idx, ph.Name, c.wantNm)
		}
		if ph.Prompt != c.wantPr {
			t.Errorf("placeholder %d Prompt = %q, want %q", c.idx, ph.Prompt, c.wantPr)
		}
	}
}

func TestPlaceholderHasSubtitleHint(t *testing.T) {
	cases := []struct {
		desc   string
		name   string
		prompt string
		want   bool
	}{
		{"name lowercase", "subtitle 1", "", true},
		{"name TitleCase", "Subtitle 1", "", true},
		{"name UPPER", "SUBTITLE", "", true},
		{"prompt match", "Text Placeholder 2", "Please enter Subtitle text", true},
		{"prompt mixedcase", "Text Placeholder 2", "Edit subTITLE here", true},
		{"substring match", "AbcSubtitleDef", "", true},
		{"no match", "Text Placeholder 2", "Click to edit body", false},
		{"empty", "", "", false},
		{"title only (must not match)", "Title 1", "Click to edit Master title", false},
	}
	for _, c := range cases {
		ph := &PlaceholderInfo{Name: c.name, Prompt: c.prompt}
		if got := ph.HasSubtitleHint(); got != c.want {
			t.Errorf("%s: HasSubtitleHint(name=%q, prompt=%q) = %v, want %v",
				c.desc, c.name, c.prompt, got, c.want)
		}
	}
}
