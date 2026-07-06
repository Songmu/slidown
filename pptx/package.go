package pptx

// Package is a .pptx that has been read and inflated once. Callers that need
// several pieces of information from the same existing file during a single
// operation (e.g. an apply that reads slide metas, shape signatures and then
// merges) should open it once and reuse the parsed parts through the methods
// below, instead of calling the path-based helpers repeatedly and re-inflating
// the archive each time.
type Package struct {
	parts map[string][]byte
	order []string
}

// OpenPackage reads and inflates a .pptx from disk a single time.
func OpenPackage(path string) (*Package, error) {
	parts, order, err := readZipPartsFromPath(path)
	if err != nil {
		return nil, err
	}
	return &Package{parts: parts, order: order}, nil
}

// OpenPackageBytes inflates a .pptx from in-memory bytes.
func OpenPackageBytes(data []byte) (*Package, error) {
	parts, order, err := readZipPartsFromBytes(data)
	if err != nil {
		return nil, err
	}
	return &Package{parts: parts, order: order}, nil
}

// SlideMetasAndCoreTitle returns the per-slide slidown metadata (in slide
// order) and the deck title, mirroring ReadSlideMetasAndCoreTitle.
func (p *Package) SlideMetasAndCoreTitle() ([]SlideMeta, string) {
	return slideMetasAndCoreTitle(p.parts)
}

// ShapeSignaturesByPart returns the shape signatures of every slide keyed by
// its ZIP part name, mirroring ShapeSignaturesByPart.
func (p *Package) ShapeSignaturesByPart() map[string][]ShapeSignature {
	return shapeSignaturesByPart(p.parts)
}

// MergeWith merges newPPTX with this existing package, mirroring
// MergeWithExisting.
func (p *Package) MergeWith(newPPTX []byte) ([]byte, error) {
	return mergeWithExisting(p.parts, p.order, newPPTX)
}

// ReplaceCoreProps swaps in newPPTX's docProps/core.xml while keeping every
// other part of this existing package verbatim, mirroring ReplaceCoreProps.
func (p *Package) ReplaceCoreProps(newPPTX []byte) ([]byte, error) {
	return replaceCoreProps(p.parts, p.order, newPPTX)
}

// MergeReusingUnchangedSlides restores this package's slides named by reuse
// into newPPTX, mirroring MergeReusingUnchangedSlides.
func (p *Package) MergeReusingUnchangedSlides(newPPTX []byte, reuse map[int]string) ([]byte, error) {
	return mergeReusingUnchangedSlides(p.parts, newPPTX, reuse)
}

// MergeReusingUnchangedShapes restores unchanged text boxes from this package
// into the slides of pkg named by targets, mirroring
// MergeReusingUnchangedShapes.
func (p *Package) MergeReusingUnchangedShapes(pkg []byte, targets map[int]string) ([]byte, error) {
	return mergeReusingUnchangedShapes(pkg, p.parts, targets)
}
