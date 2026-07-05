package pptx

import "testing"

func TestParseGeom(t *testing.T) {
	master := xmlDecl + `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>` +
		`<p:sp><p:nvSpPr><p:cNvPr name="Title"/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm></p:spPr></p:sp>` +
		`<p:sp><p:nvSpPr><p:cNvPr name="Body"/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="500" y="600"/><a:ext cx="700" cy="800"/></a:xfrm></p:spPr></p:sp>` +
		// A placeholder without xfrm must be skipped.
		`<p:sp><p:nvSpPr><p:cNvPr name="NoGeom"/><p:nvPr><p:ph type="body" idx="2"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr/></p:sp>` +
		`</p:spTree></p:cSld></p:sldMaster>`

	geoms := parseGeom([]byte(master))
	if len(geoms) != 2 {
		t.Fatalf("parseGeom returned %d entries, want 2: %+v", len(geoms), geoms)
	}
	if g := geoms[0]; g.x != 100 || g.y != 200 || g.w != 300 || g.h != 400 {
		t.Errorf("title geom (idx 0) = %+v, want {100 200 300 400}", g)
	}
	if g := geoms[1]; g.x != 500 || g.y != 600 || g.w != 700 || g.h != 800 {
		t.Errorf("body geom (idx 1) = %+v, want {500 600 700 800}", g)
	}
	if _, ok := geoms[2]; ok {
		t.Errorf("idx 2 (no xfrm) should be absent, got %+v", geoms[2])
	}
}

func TestEffectiveGeometry(t *testing.T) {
	const masterPart = "ppt/slideMasters/slideMaster1.xml"
	tmpl := &Template{
		masterGeoms: map[string]map[int]phGeom{
			masterPart: {1: {x: 10, y: 20, w: 30, h: 40}},
		},
	}
	layout := &LayoutInfo{MasterPart: masterPart}

	// Explicit layout geometry wins over master inheritance.
	own := &PlaceholderInfo{Idx: 1, HasGeom: true, X: 1, Y: 2, W: 3, H: 4}
	if x, y, w, h, ok := tmpl.EffectiveGeometry(layout, own); !ok || x != 1 || y != 2 || w != 3 || h != 4 {
		t.Errorf("own geometry = (%d,%d,%d,%d,%v), want (1,2,3,4,true)", x, y, w, h, ok)
	}

	// No layout geometry: inherit from the master by placeholder idx.
	inherited := &PlaceholderInfo{Idx: 1}
	if x, y, w, h, ok := tmpl.EffectiveGeometry(layout, inherited); !ok || x != 10 || y != 20 || w != 30 || h != 40 {
		t.Errorf("inherited geometry = (%d,%d,%d,%d,%v), want (10,20,30,40,true)", x, y, w, h, ok)
	}

	// Unknown idx and unknown master both resolve to not-ok.
	if _, _, _, _, ok := tmpl.EffectiveGeometry(layout, &PlaceholderInfo{Idx: 99}); ok {
		t.Errorf("unknown idx should not resolve")
	}
	if _, _, _, _, ok := tmpl.EffectiveGeometry(&LayoutInfo{MasterPart: "missing"}, inherited); ok {
		t.Errorf("unknown master should not resolve")
	}
	if _, _, _, _, ok := tmpl.EffectiveGeometry(layout, nil); ok {
		t.Errorf("nil placeholder should not resolve")
	}
}
