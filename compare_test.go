package deck

import (
	"reflect"
	"testing"
)

func TestBlockQuotesEqualDoesNotMutateInputs(t *testing.T) {
	a := []*BlockQuote{{Nesting: 2}, {Nesting: 1}}
	b := []*BlockQuote{{Nesting: 1}, {Nesting: 2}}
	wantA := append([]*BlockQuote(nil), a...)
	wantB := append([]*BlockQuote(nil), b...)

	_ = blockQuotesEqual(a, b)

	if !reflect.DeepEqual(a, wantA) {
		t.Fatalf("left input was mutated: got %#v want %#v", a, wantA)
	}
	if !reflect.DeepEqual(b, wantB) {
		t.Fatalf("right input was mutated: got %#v want %#v", b, wantB)
	}
}
