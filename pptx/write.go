package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
)

// part is a single text file entry within the .pptx ZIP package.
type part struct {
	name string
	data string
}

// binPart is a single binary file entry within the .pptx ZIP package.
type binPart struct {
	name string
	data []byte
}

// WriteTo serializes the presentation as an OOXML (.pptx) package to w.
func (p *Presentation) WriteTo(w io.Writer) (int64, error) {
	if p.Template != nil {
		return p.writeWithTemplate(w)
	}
	width, height := p.Width, p.Height
	if width == 0 {
		width = DefaultSlideWidth
	}
	if height == 0 {
		height = DefaultSlideHeight
	}
	n := len(p.Slides)

	// Determine which slides carry speaker notes.
	var notesSlideNums []int
	for i, s := range p.Slides {
		if s.Note != "" {
			notesSlideNums = append(notesSlideNums, i+1)
		}
	}
	hasNotes := len(notesSlideNums) > 0

	parts := []part{
		{"[Content_Types].xml", contentTypes(n, notesSlideNums)},
		{"_rels/.rels", rootRels},
		{"docProps/core.xml", coreProps},
		{"docProps/app.xml", appProps},
		{"ppt/presentation.xml", presentation(width, height, n, hasNotes)},
		{"ppt/_rels/presentation.xml.rels", presentationRels(n, hasNotes)},
		{"ppt/presProps.xml", presProps},
		{"ppt/viewProps.xml", viewProps},
		{"ppt/theme/theme1.xml", theme1},
		{"ppt/slideMasters/slideMaster1.xml", slideMaster1()},
		{"ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMaster1Rels},
		{"ppt/slideLayouts/slideLayout1.xml", slideLayout1()},
		{"ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayout1Rels},
	}
	if hasNotes {
		parts = append(parts,
			part{"ppt/notesMasters/notesMaster1.xml", notesMaster1()},
			part{"ppt/notesMasters/_rels/notesMaster1.xml.rels", notesMaster1Rels},
		)
	}

	mediaIdx := 0
	var binParts []binPart
	for i, s := range p.Slides {
		xml, rels, media := renderSlide(s, &mediaIdx)
		parts = append(parts, part{fmt.Sprintf("ppt/slides/slide%d.xml", i+1), xml})
		// Every slide part needs a layout relationship; other rels follow.
		layoutRel := slideRel{
			id:     "rId1",
			relTyp: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout",
			target: "../slideLayouts/slideLayout1.xml",
		}
		all := append([]slideRel{layoutRel}, rels...)
		if s.Note != "" {
			all = append(all, slideRel{
				id:     "rId500",
				relTyp: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide",
				target: fmt.Sprintf("../notesSlides/notesSlide%d.xml", i+1),
			})
			parts = append(parts,
				part{fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", i+1), notesSlideXML(s.Note)},
				part{fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", i+1), notesSlideRels(i + 1)},
			)
		}
		parts = append(parts, part{
			fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1),
			slideRelsXML(all),
		})
		for _, m := range media {
			binParts = append(binParts, binPart{"ppt/media/" + m.name, m.data})
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, pt := range parts {
		fw, err := zw.Create(pt.name)
		if err != nil {
			return 0, err
		}
		if _, err := io.WriteString(fw, pt.data); err != nil {
			return 0, err
		}
	}
	for _, bp := range binParts {
		fw, err := zw.Create(bp.name)
		if err != nil {
			return 0, err
		}
		if _, err := fw.Write(bp.data); err != nil {
			return 0, err
		}
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	nn, err := w.Write(buf.Bytes())
	return int64(nn), err
}

// WriteFile serializes the presentation to the given path.
func (p *Presentation) WriteFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := p.WriteTo(f); err != nil {
		return err
	}
	return f.Close()
}
