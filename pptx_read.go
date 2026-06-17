package deck

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Songmu/slidown/pptx"
)

// ReadSlidesFromPPTX reads a slidown-authored .pptx and reconstructs the slide
// model plus its template metadata. It only supports the subset emitted by the
// current writer (text placeholders, images, tables, speaker notes, and the
// built-in/template layouts used by slidown).
func ReadSlidesFromPPTX(pptxPath string) (Slides, *pptx.Template, error) {
	tmpl, err := pptx.LoadTemplate(pptxPath)
	if err != nil {
		return nil, nil, err
	}

	parts, err := readZipParts(pptxPath)
	if err != nil {
		return nil, nil, err
	}

	slideNames := make([]string, 0)
	for name := range parts {
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") {
			slideNames = append(slideNames, name)
		}
	}
	sort.Slice(slideNames, func(i, j int) bool {
		return slideIndexFromName(slideNames[i]) < slideIndexFromName(slideNames[j])
	})

	slides := make(Slides, 0, len(slideNames))
	for _, slideName := range slideNames {
		idx := slideIndexFromName(slideName)
		slide, err := readSlide(parts, tmpl, idx)
		if err != nil {
			return nil, nil, err
		}
		slides = append(slides, slide)
	}
	return slides, tmpl, nil
}

func readSlide(parts map[string][]byte, tmpl *pptx.Template, idx int) (*Slide, error) {
	slideName := fmt.Sprintf("ppt/slides/slide%d.xml", idx)
	slideRelsName := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", idx)
	slideXML, ok := parts[slideName]
	if !ok {
		return nil, fmt.Errorf("missing slide part %q", slideName)
	}
	relMap := map[string]string{}
	if relsXML, ok := parts[slideRelsName]; ok {
		relMap = parseRels(relsXML)
	}

	var s xmlSlide
	if err := xml.Unmarshal(slideXML, &s); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", slideName, err)
	}

	slide := &Slide{}

	if layoutTarget := relMap["rId1"]; layoutTarget != "" {
		layoutPart := path.Clean(path.Join("ppt/slides", layoutTarget))
		for _, l := range tmpl.Layouts {
			if l.PartName == layoutPart {
				slide.Layout = l.Name
				break
			}
		}
	}

	for _, sp := range s.CSld.SpTree.Sp {
		ph := sp.NvSpPr.NvPr.Ph
		if ph == nil {
			continue
		}
		switch ph.Type {
		case "title", "ctrTitle":
			text := extractTextRuns(sp.TxBody.Paras, relMap)
			if text != "" {
				slide.Titles = append(slide.Titles, text)
			}
		case "subTitle":
			text := extractTextRuns(sp.TxBody.Paras, relMap)
			if text != "" {
				slide.Subtitles = append(slide.Subtitles, text)
			}
		case "body":
			b := &Body{Paragraphs: parseParagraphs(sp.TxBody.Paras, relMap)}
			if len(b.Paragraphs) > 0 {
				slide.Bodies = append(slide.Bodies, b)
			}
		}
	}

	for _, pic := range s.CSld.SpTree.Pic {
		embedID := pic.BlipFill.Blip.Embed
		target := relMap[embedID]
		if target == "" {
			continue
		}
		partName := path.Clean(path.Join("ppt/slides", target))
		data, ok := parts[partName]
		if !ok {
			continue
		}
		img, err := newImageFromBytes(data)
		if err != nil {
			continue
		}
		if linkID := pic.NvPicPr.CNvPr.HlinkClick.ID; linkID != "" {
			if url := relMap[linkID]; url != "" {
				img.SetLink(url)
			}
		}
		slide.Images = append(slide.Images, img)
	}

	for _, gf := range s.CSld.SpTree.GraphicFrame {
		if gf.Graphic.GraphicData.Tbl == nil {
			continue
		}
		tbl := parseTable(gf.Graphic.GraphicData.Tbl, relMap)
		if tbl != nil {
			slide.Tables = append(slide.Tables, tbl)
		}
	}

	// Speaker notes live in notesSlides. They are linked from the slide rels.
	if noteTarget := relMap["rId500"]; noteTarget != "" {
		notePart := path.Clean(path.Join("ppt/slides", noteTarget))
		if b, ok := parts[notePart]; ok {
			slide.SpeakerNote = parseNotes(b)
		}
	}

	// Heuristic blockquote recovery from italic, indented body paragraphs.
	slide.Bodies, slide.BlockQuotes = splitBlockQuotes(slide.Bodies)
	return slide, nil
}

func splitBlockQuotes(bodies []*Body) ([]*Body, []*BlockQuote) {
	var outBodies []*Body
	var outQuotes []*BlockQuote
	for _, body := range bodies {
		if body == nil {
			continue
		}
		var current *BlockQuote
		var keepBody *Body
		for _, p := range body.Paragraphs {
			if isLikelyBlockQuoteParagraph(p) {
				if current == nil || current.Nesting != p.Nesting {
					current = &BlockQuote{Nesting: p.Nesting}
					outQuotes = append(outQuotes, current)
				}
				current.Paragraphs = append(current.Paragraphs, p)
				continue
			}
			current = nil
			if keepBody == nil {
				keepBody = &Body{}
			}
			keepBody.Paragraphs = append(keepBody.Paragraphs, p)
		}
		if keepBody != nil && len(keepBody.Paragraphs) > 0 {
			outBodies = append(outBodies, keepBody)
		}
	}
	return outBodies, outQuotes
}

func isLikelyBlockQuoteParagraph(p *Paragraph) bool {
	if p == nil || p.Bullet != BulletNone || p.Nesting == 0 {
		return false
	}
	if len(p.Fragments) == 0 {
		return false
	}
	for _, f := range p.Fragments {
		if f == nil || !f.Italic {
			return false
		}
	}
	return true
}

func parseTable(tbl *xmlTable, relMap map[string]string) *Table {
	if tbl == nil || len(tbl.Rows) == 0 {
		return nil
	}
	out := &Table{Rows: make([]*TableRow, 0, len(tbl.Rows))}
	for i, r := range tbl.Rows {
		row := &TableRow{Cells: make([]*TableCell, 0, len(r.Cells))}
		for _, c := range r.Cells {
			cell := &TableCell{Alignment: parseAlignment(c.TxBody.Paras), Fragments: parseFragments(c.TxBody.Paras, relMap)}
			if i == 0 {
				cell.IsHeader = true
			}
			row.Cells = append(row.Cells, cell)
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}

func parseAlignment(paras []xmlParagraph) string {
	for _, p := range paras {
		switch strings.ToLower(strings.TrimSpace(p.PPr.Algn)) {
		case "ctr":
			return "ctr"
		case "r":
			return "r"
		case "l":
			return "l"
		}
	}
	return ""
}

func parseNotes(b []byte) string {
	var n xmlNotes
	if err := xml.Unmarshal(b, &n); err != nil {
		return ""
	}
	var lines []string
	for _, p := range n.CSld.SpTree.Sp {
		if p.TxBody == nil {
			continue
		}
		for _, para := range p.TxBody.Paras {
			text := extractTextRuns([]xmlParagraph{para}, map[string]string{})
			if text != "" {
				lines = append(lines, text)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func parseFragments(paras []xmlParagraph, relMap map[string]string) []*Fragment {
	var out []*Fragment
	for _, p := range paras {
		for _, r := range p.Runs {
			frag := &Fragment{
				Value:     strings.TrimSuffix(r.Text, "\n"),
				Bold:      strings.TrimSpace(r.RPr.B) == "1",
				Italic:    strings.TrimSpace(r.RPr.I) == "1",
				Link:      relMap[r.RPr.HlinkClick.ID],
				Code:      isCodeRun(r.RPr),
			}
			if strings.TrimSpace(r.RPr.Strike) != "" {
				frag.StyleName = StyleDel
			}
			if frag.Value != "" {
				out = append(out, frag)
			}
		}
	}
	return out
}

func extractTextRuns(paras []xmlParagraph, relMap map[string]string) string {
	var b strings.Builder
	for _, p := range paras {
		for _, r := range p.Runs {
			if r.Text != "" {
				b.WriteString(r.Text)
			}
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(b.String(), "\v", "\n"))
}

func parseParagraphs(paras []xmlParagraph, relMap map[string]string) []*Paragraph {
	var out []*Paragraph
	var current *Paragraph
	for _, p := range paras {
		if current != nil && len(current.Fragments) > 0 {
			out = append(out, current)
		}
		current = &Paragraph{
			Nesting: parseInt(p.PPr.Lvl),
		}
		switch {
		case p.PPr.BuAutoNum != nil:
			current.Bullet = BulletNumbered
		case p.PPr.BuChar != nil:
			current.Bullet = BulletDash
		default:
			current.Bullet = BulletNone
		}
		current.Fragments = parseFragments([]xmlParagraph{p}, relMap)
		if len(current.Fragments) == 0 {
			current = nil
		}
	}
	if current != nil && len(current.Fragments) > 0 {
		out = append(out, current)
	}
	return out
}

func isCodeRun(r xmlRunPr) bool {
	return strings.Contains(strings.ToLower(r.Latin.Typeface), "consolas") ||
		strings.Contains(strings.ToLower(r.Cs.Typeface), "consolas")
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func slideIndexFromName(name string) int {
	base := path.Base(name)
	base = strings.TrimPrefix(base, "slide")
	base = strings.TrimSuffix(base, ".xml")
	return parseInt(base)
}

func readZipParts(pptxPath string) (map[string][]byte, error) {
	f, err := os.Open(pptxPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return nil, err
	}
	parts := make(map[string][]byte, len(zr.File))
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		parts[zf.Name] = b
	}
	return parts, nil
}

func parseRels(b []byte) map[string]string {
	var rels xmlRelationships
	if err := xml.Unmarshal(b, &rels); err != nil {
		return map[string]string{}
	}
	m := make(map[string]string, len(rels.Relationships))
	for _, r := range rels.Relationships {
		m[r.ID] = r.Target
	}
	return m
}

func newImageFromBytes(data []byte) (*Image, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var mt MIMEType
	switch format {
	case "png":
		mt = MIMETypeImagePNG
	case "jpeg":
		mt = MIMETypeImageJPEG
	case "gif":
		mt = MIMETypeImageGIF
	default:
		return nil, fmt.Errorf("unsupported image format %q", format)
	}
	return &Image{b: data, mimeType: mt}, nil
}

type xmlSlide struct {
	CSld struct {
		SpTree struct {
			Sp           []xmlShape        `xml:"sp"`
			Pic          []xmlPic          `xml:"pic"`
			GraphicFrame []xmlGraphicFrame  `xml:"graphicFrame"`
		} `xml:"spTree"`
	} `xml:"cSld"`
}

type xmlNotes struct {
	CSld struct {
		SpTree struct {
			Sp []xmlNotesShape `xml:"sp"`
		} `xml:"spTree"`
	} `xml:"cSld"`
}

type xmlNotesShape struct {
	TxBody *xmlTxBody `xml:"txBody"`
}

type xmlShape struct {
	NvSpPr struct {
		CNvPr struct {
			Name string `xml:"name,attr"`
		} `xml:"cNvPr"`
		NvPr struct {
			Ph *xmlPh `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvSpPr"`
	TxBody *xmlTxBody `xml:"txBody"`
}

type xmlPic struct {
	NvPicPr struct {
		CNvPr struct {
			Name       string       `xml:"name,attr"`
			HlinkClick xmlHlinkClick `xml:"hlinkClick"`
		} `xml:"cNvPr"`
	} `xml:"nvPicPr"`
	BlipFill struct {
		Blip struct {
			Embed string `xml:"embed,attr"`
		} `xml:"blip"`
	} `xml:"blipFill"`
	SpPr struct {
		Xfrm *xmlXfrm `xml:"xfrm"`
	} `xml:"spPr"`
}

type xmlGraphicFrame struct {
	Graphic struct {
		GraphicData struct {
			Tbl *xmlTable `xml:"tbl"`
		} `xml:"graphicData"`
	} `xml:"graphic"`
	SpPr struct {
		Xfrm *xmlXfrm `xml:"xfrm"`
	} `xml:"spPr"`
}

type xmlTable struct {
	Rows []xmlTableRow `xml:"tr"`
}

type xmlTableRow struct {
	Cells []xmlTableCell `xml:"tc"`
}

type xmlTableCell struct {
	TxBody xmlTxBody `xml:"txBody"`
}

type xmlTxBody struct {
	Paras []xmlParagraph `xml:"p"`
}

type xmlParagraph struct {
	PPr  xmlPPr      `xml:"pPr"`
	Runs []xmlRun    `xml:"r"`
}

type xmlPPr struct {
	Lvl       string `xml:"lvl,attr"`
	Algn      string `xml:"algn,attr"`
	BuNone    *struct{} `xml:"buNone"`
	BuChar    *struct{} `xml:"buChar"`
	BuAutoNum *struct{} `xml:"buAutoNum"`
}

type xmlRun struct {
	RPr xmlRunPr `xml:"rPr"`
	Text string  `xml:"t"`
}

type xmlRunPr struct {
	B          string        `xml:"b,attr"`
	I          string        `xml:"i,attr"`
	U          string        `xml:"u,attr"`
	Strike     string        `xml:"strike,attr"`
	Latin      xmlTypeface   `xml:"latin"`
	Cs         xmlTypeface   `xml:"cs"`
	HlinkClick  xmlHlinkClick `xml:"hlinkClick"`
}

type xmlPh struct {
	Type string `xml:"type,attr"`
	Idx  string `xml:"idx,attr"`
}

type xmlXfrm struct {
	Off struct {
		X string `xml:"x,attr"`
		Y string `xml:"y,attr"`
	} `xml:"off"`
	Ext struct {
		Cx string `xml:"cx,attr"`
		Cy string `xml:"cy,attr"`
	} `xml:"ext"`
}

type xmlTypeface struct {
	Typeface string `xml:"typeface,attr"`
}

type xmlHlinkClick struct {
	ID string `xml:"id,attr"`
}

type xmlRelationships struct {
	Relationships []xmlRelationship `xml:"Relationship"`
}

type xmlRelationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}
