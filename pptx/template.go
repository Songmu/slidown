package pptx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Template is a loaded .pptx used as the design base for generated slides. It
// carries the template's design parts (theme, slide master(s), slide layouts
// and related parts) verbatim and exposes the layouts so the renderer can map
// content onto their placeholders.
type Template struct {
	// designParts holds the verbatim parts copied from the template, keyed by
	// part name (e.g. "ppt/theme/theme1.xml").
	designParts map[string][]byte
	// masterParts lists slide master part names in a stable order.
	masterParts []string
	// notesMasterPart / handoutMasterPart are the template's notes and handout
	// master part names, when present. They are carried over verbatim and wired
	// into the generated presentation so their themes stay referenced (avoiding
	// orphan parts that PowerPoint strips on repair) and so notes and handouts
	// added later inherit the template's design.
	notesMasterPart   string
	handoutMasterPart string
	// themePart is the theme part referenced from the presentation (the theme
	// used by the slide master).
	themePart string
	// presPropsPart / viewPropsPart / tableStylesPart are optional.
	presPropsPart   string
	viewPropsPart   string
	tableStylesPart string
	// Layouts are the parsed slide layouts available in the template.
	Layouts []*LayoutInfo
	// partTypes maps design part names to their content types (for emitting
	// [Content_Types].xml overrides).
	partTypes map[string]string
	// defaultTypes maps file-extension Default declarations preserved from the
	// template's [Content_Types].xml (e.g. "emf" -> "image/x-emf"). PowerPoint
	// flags the package as needing repair if a media part (copied verbatim from
	// the template) has neither a Default nor an Override for its extension.
	defaultTypes map[string]string
	// slideSize from the template presentation, in EMUs (0 if not found).
	width, height int64
}

// LayoutInfo describes a single slide layout from the template.
type LayoutInfo struct {
	PartName     string
	Name         string
	Type         string // sldLayout @type, e.g. "title", "obj", "tx", "blank"
	Placeholders []*PlaceholderInfo
}

// PlaceholderInfo describes a placeholder within a layout.
type PlaceholderInfo struct {
	Type string // "title", "ctrTitle", "subTitle", "body", "" (default body)
	// Name is the shape's display name from the layout (the <p:cNvPr name="...">
	// attribute, e.g. "Subtitle 1" or "Content Placeholder 2"). Available for
	// heuristics like HasSubtitleHint.
	Name string
	// Prompt is the concatenated prompt text stored on the layout placeholder
	// (<a:t> within <p:txBody>). PowerPoint persists user-edited prompts here
	// when <p:ph hasCustomPrompt="1"/>. Used by HasSubtitleHint.
	Prompt  string
	Idx     int
	HasGeom bool
	X, Y    int64
	W, H    int64
}

// HasSubtitleHint reports whether this placeholder's display name or prompt
// text contains the substring "subtitle" (case insensitive). It lets users
// repurpose an ordinary text placeholder as a subtitle target without resorting
// to XML editing: editing the placeholder name (via PowerPoint's Selection
// Pane) or its prompt text in the slide master view is enough to opt in.
func (p *PlaceholderInfo) HasSubtitleHint() bool {
	const needle = "subtitle"
	return strings.Contains(strings.ToLower(p.Name), needle) ||
		strings.Contains(strings.ToLower(p.Prompt), needle)
}

// --- XML parsing structs (namespace-agnostic via local names) ---

type xmlOff struct {
	X string `xml:"x,attr"`
	Y string `xml:"y,attr"`
}
type xmlExt struct {
	Cx string `xml:"cx,attr"`
	Cy string `xml:"cy,attr"`
}
type xmlXfrm struct {
	Off xmlOff `xml:"off"`
	Ext xmlExt `xml:"ext"`
}
type xmlPh struct {
	Type            string `xml:"type,attr"`
	Idx             string `xml:"idx,attr"`
	HasCustomPrompt string `xml:"hasCustomPrompt,attr"`
}
type xmlCNvPr struct {
	Name string `xml:"name,attr"`
}
type xmlRun struct {
	T string `xml:"t"`
}
type xmlPara struct {
	Runs []xmlRun `xml:"r"`
}
type xmlSp struct {
	NvSpPr struct {
		CNvPr xmlCNvPr `xml:"cNvPr"`
		NvPr  struct {
			Ph *xmlPh `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvSpPr"`
	SpPr struct {
		Xfrm *xmlXfrm `xml:"xfrm"`
	} `xml:"spPr"`
	TxBody struct {
		Paras []xmlPara `xml:"p"`
	} `xml:"txBody"`
}
type xmlSldLayout struct {
	Type string `xml:"type,attr"`
	CSld struct {
		Name   string `xml:"name,attr"`
		SpTree struct {
			Sp []xmlSp `xml:"sp"`
		} `xml:"spTree"`
	} `xml:"cSld"`
}

type xmlContentTypes struct {
	Defaults []struct {
		Extension   string `xml:"Extension,attr"`
		ContentType string `xml:"ContentType,attr"`
	} `xml:"Default"`
	Overrides []struct {
		PartName    string `xml:"PartName,attr"`
		ContentType string `xml:"ContentType,attr"`
	} `xml:"Override"`
}

type xmlPresentation struct {
	SldSz struct {
		Cx string `xml:"cx,attr"`
		Cy string `xml:"cy,attr"`
	} `xml:"sldSz"`
}

// LoadTemplate reads a PowerPoint template package (a .pptx presentation or
// .potx template) and extracts its reusable design parts. Both formats share
// the same OOXML structure for theme, slide masters and slide layouts, so the
// loader treats them interchangeably.
func LoadTemplate(path string) (*Template, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open template %q: %w", path, err)
	}
	defer zr.Close()

	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to read template part %q: %w", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		parts[f.Name] = b
	}

	ct, ok := parts["[Content_Types].xml"]
	if !ok {
		return nil, fmt.Errorf("template %q is missing [Content_Types].xml", path)
	}
	var types xmlContentTypes
	if err := xml.Unmarshal(ct, &types); err != nil {
		return nil, fmt.Errorf("failed to parse template content types: %w", err)
	}

	t := &Template{designParts: map[string][]byte{}, partTypes: map[string]string{}, defaultTypes: map[string]string{}}

	// Preserve the template's Default extension declarations so verbatim-copied
	// media (emf, svg, wdp, ...) still has a declared content type in the
	// emitted [Content_Types].xml.
	for _, d := range types.Defaults {
		ext := strings.ToLower(d.Extension)
		if ext == "" || d.ContentType == "" {
			continue
		}
		t.defaultTypes[ext] = d.ContentType
	}

	// Classify overrides into design parts we carry over.
	for _, o := range types.Overrides {
		name := strings.TrimPrefix(o.PartName, "/")
		switch {
		case strings.Contains(o.ContentType, "theme+xml"):
			if t.themePart == "" {
				t.themePart = name
			}
			t.copyPart(parts, name)
			t.copyRels(parts, name)
			t.partTypes[name] = o.ContentType
		case strings.Contains(o.ContentType, "slideMaster+xml"):
			t.masterParts = append(t.masterParts, name)
			t.copyPart(parts, name)
			t.copyRels(parts, name)
			t.partTypes[name] = o.ContentType
		case strings.Contains(o.ContentType, "notesMaster+xml"):
			t.notesMasterPart = name
			t.copyPart(parts, name)
			t.copyRels(parts, name)
			t.partTypes[name] = o.ContentType
		case strings.Contains(o.ContentType, "handoutMaster+xml"):
			t.handoutMasterPart = name
			t.copyPart(parts, name)
			t.copyRels(parts, name)
			t.partTypes[name] = o.ContentType
		case strings.Contains(o.ContentType, "slideLayout+xml"):
			t.copyPart(parts, name)
			t.copyRels(parts, name)
			t.partTypes[name] = o.ContentType
			if li := parseLayout(name, parts[name]); li != nil {
				t.Layouts = append(t.Layouts, li)
			}
		case strings.Contains(o.ContentType, "presProps+xml"):
			t.presPropsPart = name
			t.copyPart(parts, name)
			t.partTypes[name] = o.ContentType
		case strings.Contains(o.ContentType, "viewProps+xml"):
			t.viewPropsPart = name
			t.copyPart(parts, name)
			t.partTypes[name] = o.ContentType
		case strings.Contains(o.ContentType, "tableStyles+xml"):
			t.tableStylesPart = name
			t.copyPart(parts, name)
			t.partTypes[name] = o.ContentType
		}
	}

	// Copy only ppt/media/* parts that are actually referenced by the design
	// .rels files collected above (theme, masters, layouts). Copying the whole
	// ppt/media tree would accumulate orphaned slide-level media across
	// incremental rebuilds when an existing output is reused as a template.
	for media := range referencedDesignMedia(t.designParts) {
		if data, ok := parts[media]; ok {
			t.designParts[media] = data
		}
	}

	if len(t.masterParts) == 0 {
		return nil, fmt.Errorf("template %q has no slide master", path)
	}
	if len(t.Layouts) == 0 {
		return nil, fmt.Errorf("template %q has no slide layouts", path)
	}
	sort.Slice(t.masterParts, func(i, j int) bool { return t.masterParts[i] < t.masterParts[j] })
	sort.Slice(t.Layouts, func(i, j int) bool { return t.Layouts[i].PartName < t.Layouts[j].PartName })

	// The presentation's theme relationship must point at the same theme the
	// slide master uses, not merely the first theme override encountered. Derive
	// it from the master's rels so notes/handout masters (which carry their own
	// themes) can't be mistaken for the presentation theme.
	if theme := t.themeReferencedBy(t.masterParts[0]); theme != "" {
		t.themePart = theme
	}

	if pres, ok := parts["ppt/presentation.xml"]; ok {
		var xp xmlPresentation
		if err := xml.Unmarshal(pres, &xp); err == nil {
			t.width = atoi64(xp.SldSz.Cx)
			t.height = atoi64(xp.SldSz.Cy)
		}
	}

	return t, nil
}

func (t *Template) copyPart(parts map[string][]byte, name string) {
	if b, ok := parts[name]; ok {
		t.designParts[name] = b
	}
}

// copyRels copies the .rels file associated with a part, if present.
func (t *Template) copyRels(parts map[string][]byte, name string) {
	rels := relsPath(name)
	if b, ok := parts[rels]; ok {
		t.designParts[rels] = b
	}
}

// themeReferencedBy returns the theme part name (e.g. "ppt/theme/theme1.xml")
// referenced by the given design part's already-copied .rels, or "" when none
// is found.
func (t *Template) themeReferencedBy(partName string) string {
	relsXML, ok := t.designParts[relsPath(partName)]
	if !ok {
		return ""
	}
	base := partName
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[:i]
	}
	for _, target := range relTargets(relsXML) {
		resolved := path.Clean(path.Join(base, target))
		if strings.HasPrefix(resolved, "ppt/theme/") {
			return resolved
		}
	}
	return ""
}

// relsPath returns the relationships part path for a given part path.
func relsPath(part string) string {
	i := strings.LastIndex(part, "/")
	if i < 0 {
		return "_rels/" + part + ".rels"
	}
	return part[:i] + "/_rels/" + part[i+1:] + ".rels"
}

// referencedDesignMedia returns the set of ppt/media/* part names that are
// directly referenced by any .rels file already collected in designParts
// (theme, slide masters, slide layouts). It is used to restrict media copying
// to only design-related assets, so that orphaned slide-level media from a
// previous build does not accumulate when an existing .pptx is reused as a
// template.
func referencedDesignMedia(designParts map[string][]byte) map[string]struct{} {
	refs := make(map[string]struct{})
	for name, data := range designParts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		dir := relsOwnerDir(name)
		for _, target := range relTargets(data) {
			resolved := path.Clean(path.Join(dir, target))
			if strings.HasPrefix(resolved, "ppt/media/") {
				refs[resolved] = struct{}{}
			}
		}
	}
	return refs
}

// relsOwnerDir returns the directory that conceptually "owns" a .rels file,
// which is the base for resolving relative relationship targets.
// For example, "ppt/slideMasters/_rels/sm1.xml.rels" -> "ppt/slideMasters".
// For root-level rels like "_rels/.rels" it returns "".
func relsOwnerDir(relsName string) string {
	if i := strings.LastIndex(relsName, "/_rels/"); i >= 0 {
		return relsName[:i]
	}
	return ""
}

func parseLayout(partName string, data []byte) *LayoutInfo {
	var l xmlSldLayout
	if err := xml.Unmarshal(data, &l); err != nil {
		return nil
	}
	li := &LayoutInfo{PartName: partName, Name: l.CSld.Name, Type: l.Type}
	for _, sp := range l.CSld.SpTree.Sp {
		if sp.NvSpPr.NvPr.Ph == nil {
			continue
		}
		ph := &PlaceholderInfo{
			Type:   sp.NvSpPr.NvPr.Ph.Type,
			Idx:    atoi(sp.NvSpPr.NvPr.Ph.Idx),
			Name:   sp.NvSpPr.CNvPr.Name,
			Prompt: collectPromptText(sp),
		}
		if sp.SpPr.Xfrm != nil {
			x := atoi64(sp.SpPr.Xfrm.Off.X)
			y := atoi64(sp.SpPr.Xfrm.Off.Y)
			w := atoi64(sp.SpPr.Xfrm.Ext.Cx)
			h := atoi64(sp.SpPr.Xfrm.Ext.Cy)
			if w > 0 && h > 0 {
				ph.HasGeom = true
				ph.X, ph.Y, ph.W, ph.H = x, y, w, h
			}
		}
		li.Placeholders = append(li.Placeholders, ph)
	}
	return li
}

// collectPromptText concatenates the placeholder's prompt run text. Only used
// for heuristics (HasSubtitleHint), so we just join all <a:t> children with
// spaces; structural details (paragraphs, runs) are irrelevant for matching.
func collectPromptText(sp xmlSp) string {
	var b strings.Builder
	for _, p := range sp.TxBody.Paras {
		for _, r := range p.Runs {
			if r.T == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(r.T)
		}
	}
	return b.String()
}

// --- Layout selection helpers ---

// LayoutByName returns the layout whose (case-insensitive) name matches, or nil.
func (t *Template) LayoutByName(name string) *LayoutInfo {
	if name == "" {
		return nil
	}
	for _, l := range t.Layouts {
		if strings.EqualFold(l.Name, name) {
			return l
		}
	}
	return nil
}

// TitleLayout returns the best layout for a title slide.
func (t *Template) TitleLayout() *LayoutInfo {
	for _, l := range t.Layouts {
		if l.Type == "title" {
			return l
		}
	}
	for _, l := range t.Layouts {
		if l.hasPlaceholder("ctrTitle") {
			return l
		}
	}
	return t.ContentLayout()
}

// ContentLayout returns the best layout for a content slide (title + body).
func (t *Template) ContentLayout() *LayoutInfo {
	for _, l := range t.Layouts {
		if l.Type == "obj" || l.Type == "tx" {
			return l
		}
	}
	for _, l := range t.Layouts {
		if l.hasPlaceholder("title") && l.hasBodyPlaceholder() {
			return l
		}
	}
	return t.Layouts[0]
}

func (l *LayoutInfo) hasPlaceholder(typ string) bool {
	for _, p := range l.Placeholders {
		if p.Type == typ {
			return true
		}
	}
	return false
}

func (l *LayoutInfo) hasBodyPlaceholder() bool {
	for _, p := range l.Placeholders {
		if p.Type == "body" || p.Type == "" || p.Type == "obj" {
			return true
		}
	}
	return false
}

// BodyGeometry returns the geometry of the layout's first body placeholder, or
// false if none has explicit geometry.
func (l *LayoutInfo) BodyGeometry() (x, y, w, h int64, ok bool) {
	for _, p := range l.Placeholders {
		if (p.Type == "body" || p.Type == "" || p.Type == "obj") && p.HasGeom {
			return p.X, p.Y, p.W, p.H, true
		}
	}
	return 0, 0, 0, 0, false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
