package slidown

import "testing"

func TestImageSignaturesEquivalent(t *testing.T) {
	for _, tt := range []struct {
		name string
		a, b imageSignature
		want bool
	}{
		{"distance four", imageSignature{PHash: 0x80}, imageSignature{PHash: 0x8f}, true},
		{"distance five", imageSignature{PHash: 0x80}, imageSignature{PHash: 0x9f}, false},
		{"changed link", imageSignature{PHash: 1, Link: "old"}, imageSignature{PHash: 1, Link: "new"}, false},
		{"changed SVG link", imageSignature{Digest: "svg", Link: "old"}, imageSignature{Digest: "svg", Link: "new"}, false},
		{"digest takes precedence over perceptual hash", imageSignature{Digest: "svg", PHash: 0x80}, imageSignature{Digest: "svg", PHash: 0x9f}, true},
		{"digest takes precedence over checksum", imageSignature{Digest: "svg", Checksum: 1}, imageSignature{Digest: "svg", Checksum: 2}, true},
		{"changed digest", imageSignature{Digest: "old", PHash: 1}, imageSignature{Digest: "new", PHash: 1}, false},
		{"missing digest", imageSignature{Digest: "svg", PHash: 1}, imageSignature{PHash: 1}, false},
		{"equal checksums", imageSignature{Checksum: 7}, imageSignature{Checksum: 7}, true},
		{"changed checksum", imageSignature{Checksum: 7}, imageSignature{Checksum: 8}, false},
		{"missing perceptual hash", imageSignature{PHash: 1, Checksum: 7}, imageSignature{Checksum: 7}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, pair := range [][2]imageSignature{{tt.a, tt.b}, {tt.b, tt.a}} {
				if got := imageSignaturesEquivalent(pair[0], pair[1]); got != tt.want {
					t.Errorf("imageSignaturesEquivalent(%+v, %+v) = %v, want %v", pair[0], pair[1], got, tt.want)
				}
			}
		})
	}
}

func TestImageSetsEquivalentDistinctMatches(t *testing.T) {
	for _, tt := range []struct {
		name string
		a, b []imageSignature
		want bool
	}{
		{"matching subset has different count", []imageSignature{{Checksum: 1}}, []imageSignature{{Checksum: 1}, {Checksum: 2}}, false},
		{"two distinct copies", []imageSignature{{Checksum: 1}, {Checksum: 1}}, []imageSignature{{Checksum: 1}, {Checksum: 1}}, true},
		{"counterpart cannot be reused", []imageSignature{{Checksum: 1}, {Checksum: 1}}, []imageSignature{{Checksum: 1}, {Checksum: 2}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := imageSetsEquivalent(tt.a, tt.b); got != tt.want {
				t.Errorf("imageSetsEquivalent(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
