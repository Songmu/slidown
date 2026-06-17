package pptx

import "fmt"

// contentTypes returns the [Content_Types].xml part. slideCount overrides are
// declared individually because each slide is its own part. notesSlideNums
// lists the slide numbers (1-based) that have an associated notes slide.
func contentTypes(slideCount int, notesSlideNums []int) string {
	var b []byte
	b = append(b, []byte(xmlDecl+
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="xml" ContentType="application/xml"/>`+
		`<Default Extension="png" ContentType="image/png"/>`+
		`<Default Extension="jpeg" ContentType="image/jpeg"/>`+
		`<Default Extension="jpg" ContentType="image/jpeg"/>`+
		`<Default Extension="gif" ContentType="image/gif"/>`+
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`+
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`+
		`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`+
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`+
		`<Override PartName="/ppt/presProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"/>`+
		`<Override PartName="/ppt/viewProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"/>`+
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`+
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)...)
	for i := 1; i <= slideCount; i++ {
		b = append(b, []byte(fmt.Sprintf(
			`<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i))...)
	}
	if len(notesSlideNums) > 0 {
		b = append(b, []byte(`<Override PartName="/ppt/notesMasters/notesMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"/>`)...)
		for _, n := range notesSlideNums {
			b = append(b, []byte(fmt.Sprintf(
				`<Override PartName="/ppt/notesSlides/notesSlide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/>`, n))...)
		}
	}
	b = append(b, []byte(`</Types>`)...)
	return string(b)
}

const xmlDecl = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// rootRels is the package-level _rels/.rels part.
const rootRels = xmlDecl +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
	`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
	`</Relationships>`

// presentation builds ppt/presentation.xml referencing the master and slides.
func presentation(width, height int64, slideCount int, hasNotes bool) string {
	var b []byte
	b = append(b, []byte(xmlDecl+
		`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" `+
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" `+
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" saveSubsetFonts="1">`+
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`)...)
	if hasNotes {
		b = append(b, []byte(`<p:notesMasterIdLst><p:notesMasterId r:id="rId902"/></p:notesMasterIdLst>`)...)
	}
	b = append(b, []byte(`<p:sldIdLst>`)...)
	// Slide relationship ids start after master(rId1) and theme(rId2).
	for i := 0; i < slideCount; i++ {
		b = append(b, []byte(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, 3+i))...)
	}
	b = append(b, []byte(fmt.Sprintf(
		`</p:sldIdLst><p:sldSz cx="%d" cy="%d"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
		width, height))...)
	return string(b)
}

// presentationRels builds ppt/_rels/presentation.xml.rels.
func presentationRels(slideCount int, hasNotes bool) string {
	var b []byte
	b = append(b, []byte(xmlDecl+
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`+
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`)...)
	for i := 0; i < slideCount; i++ {
		b = append(b, []byte(fmt.Sprintf(
			`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`,
			3+i, i+1))...)
	}
	b = append(b, []byte(`<Relationship Id="rId900" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps" Target="presProps.xml"/>`+
		`<Relationship Id="rId901" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps" Target="viewProps.xml"/>`)...)
	if hasNotes {
		b = append(b, []byte(`<Relationship Id="rId902" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="notesMasters/notesMaster1.xml"/>`)...)
	}
	b = append(b, []byte(`</Relationships>`)...)
	return string(b)
}

const presProps = xmlDecl +
	`<p:presentationPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`

const viewProps = xmlDecl +
	`<p:viewPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`

const coreProps = xmlDecl +
	`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
	`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" ` +
	`xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
	`<dc:title></dc:title><cp:revision>1</cp:revision></cp:coreProperties>`

const appProps = xmlDecl +
	`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" ` +
	`xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
	`<Application>slidown</Application></Properties>`
