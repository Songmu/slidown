package md

import (
	"reflect"
	"testing"

	"github.com/Songmu/slidown"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

func TestToFragmentsInlineStyleAndSeed(t *testing.T) {
	source := []byte(`<span class="outer">*<i>inner</i>* tail<br>reset</span> <https://example.com>`)
	doc := newParser().Parser().Parse(text.NewReader(source))
	seed := slidown.Fragment{Bold: true, Italic: true, Code: true, Link: "seed", StyleName: "seed"}
	frags, images, err := toFragments("", source, doc.FirstChild(), seed)
	if err != nil {
		t.Fatal(err)
	}
	want := []*fragment{
		{Fragment: &slidown.Fragment{Value: "inner", Bold: true, Italic: true, Code: true, Link: "seed", StyleName: "outer"}},
		{Fragment: &slidown.Fragment{Value: " tail", Bold: true, Italic: true, Code: true, Link: "seed", StyleName: "outer"}},
		{Fragment: &slidown.Fragment{Value: "\n", StyleName: "outer"}},
		{Fragment: &slidown.Fragment{Value: "reset", Bold: true, Italic: true, Code: true, Link: "seed"}},
		{Fragment: &slidown.Fragment{Value: " ", Bold: true, Italic: true, Code: true, Link: "seed"}},
		{Fragment: &slidown.Fragment{Value: "https://example.com", Link: "https://example.com"}},
	}
	if !reflect.DeepEqual(frags, want) || len(images) != 0 {
		t.Errorf("toFragments() = %#v, %v; want %#v, no images", frags, images, want)
	}
}

func TestToFragmentsFirstDecoratedChild(t *testing.T) {
	for _, node := range []ast.Node{ast.NewCodeSpan(), east.NewStrikethrough()} {
		t.Run(node.Kind().String(), func(t *testing.T) {
			source := []byte("firstsecond")
			parent := ast.NewParagraph()
			first := ast.NewTextSegment(text.NewSegment(0, 5))
			first.SetSoftLineBreak(true)
			node.AppendChild(node, first)
			node.AppendChild(node, ast.NewTextSegment(text.NewSegment(5, len(source))))
			parent.AppendChild(parent, node)
			frags, _, err := toFragments("", source, parent, slidown.Fragment{Bold: true})
			if err != nil {
				t.Fatal(err)
			}
			want := &fragment{SoftLineBreak: true, Fragment: &slidown.Fragment{Value: "first", Bold: true}}
			switch node.(type) {
			case *ast.CodeSpan:
				want.Code = true
			case *east.Strikethrough:
				want.StyleName = slidown.StyleDel
			}
			if len(frags) != 1 || !reflect.DeepEqual(frags[0], want) {
				t.Errorf("toFragments() = %#v, want only %#v", frags, want)
			}
		})
	}
}

func TestToFragmentsEmptyTextUpdatesSoftLineBreak(t *testing.T) {
	source := []byte("first")
	parent := ast.NewParagraph()
	parent.AppendChild(parent, ast.NewTextSegment(text.NewSegment(0, len(source))))
	empty := ast.NewTextSegment(text.NewSegment(len(source), len(source)))
	empty.SetSoftLineBreak(true)
	parent.AppendChild(parent, empty)
	frags, _, err := toFragments("", source, parent, slidown.Fragment{})
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 || frags[0].Value != "first" || !frags[0].SoftLineBreak {
		t.Errorf("empty text did not update the preceding fragment: %#v", frags)
	}
}
