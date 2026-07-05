package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/Songmu/slidown/render"
	"github.com/spf13/cobra"
)

var (
	applyOutput              string
	applyCodeBlockToImageCmd string
	applyTemplate            string
)

var applyCmd = &cobra.Command{
	Use:   "apply DECK_FILE",
	Short: "apply a markdown deck to a PowerPoint (.pptx) presentation",
	Long: `apply generates a PowerPoint (.pptx) file from a markdown deck file.

The output path defaults to the input file name with a .pptx extension,
and can be overridden with the --output/-o flag.

When the output file does not yet exist it is created. A .pptx or .potx
template (its theme, slide masters and layouts) may seed a newly created file
via the --template flag or the "template" config field; a template can only be
supplied when creating a new file.

When the output file already exists it is updated in place, reusing itself as
the template. Passing --template while the output already exists is an error:
choose a different --output, or remove the existing file first.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := args[0]

		cfg, err := config.Load(profile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		m, err := md.ParseFile(f, cfg)
		if err != nil {
			return err
		}

		out := applyOutput
		if out == "" && m.Frontmatter != nil && m.Frontmatter.Output != "" {
			out = m.Frontmatter.Output
		}
		if out == "" {
			out = defaultOutputPath(f)
		}

		var cfgTemplate string
		if cfg != nil {
			cfgTemplate = cfg.Template
		}
		templatePath, err := resolveApplyTemplate(out, applyTemplate, cfgTemplate)
		if err != nil {
			return err
		}

		slides, err := m.ToSlides(cmd.Context(), applyCodeBlockToImageCmd)
		if err != nil {
			return fmt.Errorf("failed to convert markdown to slides: %w", err)
		}

		var pres *pptx.Presentation
		if templatePath != "" {
			tmpl, err := pptx.LoadTemplate(templatePath)
			if err != nil {
				return fmt.Errorf("failed to load template: %w", err)
			}
			pres = render.ToPresentationWithTemplate(slides, tmpl)
		} else {
			pres = render.ToPresentation(slides)
		}
		if m.Frontmatter != nil {
			pres.Title = m.Frontmatter.Title
		}

		var buf bytes.Buffer
		if _, err := pres.WriteTo(&buf); err != nil {
			return fmt.Errorf("failed to write presentation: %w", err)
		}

		updated, err := writePresentation(out, buf.Bytes(), slides, pres.Title)
		if err != nil {
			return fmt.Errorf("failed to write presentation: %w", err)
		}
		if updated {
			cmd.Printf("Updated %s (%d slide(s))\n", out, len(slides))
		} else {
			cmd.Printf("Wrote %s (%d slide(s))\n", out, len(slides))
		}
		return nil
	},
}

// resolveApplyTemplate decides which template (if any) apply should use, given
// the resolved output path, the --template flag value and the config template.
//
// A template may only seed a newly created file:
//   - flagTemplate set + output exists -> error (choose -o or remove the file)
//   - flagTemplate set + new output    -> create from flagTemplate
//   - output exists                    -> reuse the output as its own template
//   - cfgTemplate set + new output     -> create from cfgTemplate
//   - otherwise                        -> "" (built-in design)
func resolveApplyTemplate(out, flagTemplate, cfgTemplate string) (string, error) {
	outExists, err := pathExists(out)
	if err != nil {
		return "", fmt.Errorf("failed to inspect output path: %w", err)
	}
	switch {
	case flagTemplate != "":
		if outExists {
			return "", fmt.Errorf("output %q already exists; --template can only be used when creating a new file. "+
				"Specify a different output with -o, or remove the existing file first", out)
		}
		return flagTemplate, nil
	case outExists:
		// Update in place: reuse the existing presentation as its own template.
		return out, nil
	case cfgTemplate != "":
		return cfgTemplate, nil
	default:
		return "", nil
	}
}

func writePresentation(out string, newPPTX []byte, sourceSlides slidown.Slides, desiredTitle string) (bool, error) {
	exists, err := pathExists(out)
	if err != nil {
		return false, err
	}
	if !exists {
		if err := os.WriteFile(out, newPPTX, 0o600); err != nil {
			return false, err
		}
		return false, nil
	}

	// When the output file already exists, preserve the slides that should keep
	// their existing content: slides whose source did not change (so manual
	// edits survive) and slides explicitly frozen via configuration. Slides are
	// matched to their existing counterpart by stable key (falling back to
	// position), so reuse and freeze survive inserts, deletions and reordering.
	// Change detection compares each slide's embedded source fingerprint against
	// the freshly computed one.
	//
	// Reuse copies a slide's .rels verbatim from the existing package, which may
	// reference layouts by file name (e.g. ../slideLayouts/slideLayout7.xml).
	// This is safe here because an existing output is always rebuilt using
	// itself as the template (apply refuses --template when the output already
	// exists), so its design parts are unchanged.
	if existing, existingTitle, err := pptx.ReadSlideMetasAndCoreTitle(out); err == nil {
		// Per-shape signatures for the freshly generated package and the existing
		// file feed the shape-level similarity gate. Read best-effort: on failure
		// the maps are nil, the gate sees zero overlap and no shape-level merge
		// is attempted (safe fallback to whole-slide regeneration).
		newSigs, _ := pptx.ShapeSignaturesByPosition(newPPTX)
		oldSigs, _ := pptx.ShapeSignaturesByPart(out)
		reuse, shapeMerge := alignSlides(sourceSlides, existing, newSigs, oldSigs)
		switch {
		case isIdentityReuse(reuse, existing, len(sourceSlides)):
			// Every slide is reused in place. The file is already correct unless
			// a deck-level property that the per-slide fingerprints do not cover
			// changed — currently the title in docProps/core.xml. When only the
			// title changed, swap in the freshly generated core.xml while keeping
			// every other part (slides, media, customXml, …) verbatim.
			if existingTitle == desiredTitle {
				return true, nil
			}
			merged, err := pptx.ReplaceCoreProps(out, newPPTX)
			if err != nil {
				return false, err
			}
			if err := os.WriteFile(out, merged, 0o600); err != nil {
				return false, err
			}
			return true, nil
		case len(reuse) > 0 || len(shapeMerge) > 0:
			var merged []byte
			if len(reuse) > 0 {
				merged, err = pptx.MergeReusingUnchangedSlides(out, newPPTX, reuse)
			} else {
				merged, err = pptx.MergeWithExisting(out, newPPTX)
			}
			if err != nil {
				return false, err
			}
			merged, err = pptx.MergeReusingUnchangedShapes(merged, out, shapeMerge)
			if err != nil {
				return false, err
			}
			if err := os.WriteFile(out, merged, 0o600); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	merged, err := pptx.MergeWithExisting(out, newPPTX)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(out, merged, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// shapeMergeMinOverlap is the minimum fraction of shapes (matched by slot key
// and per-shape fingerprint) that a changed source slide must share with its
// paired existing slide before a shape-level merge is attempted. Below this the
// pair is treated as two different slides and the slide is fully regenerated.
const shapeMergeMinOverlap = 0.5

// alignSlides matches source slides to slides in the existing presentation and
// classifies each matched pair as either a whole-slide reuse or a shape-level
// merge. It returns two maps of new 1-based position -> existing slide part name
// (e.g. "ppt/slides/slide3.xml"): reuse for slides kept verbatim (unchanged or
// frozen), and shapeMerge for changed slides confidently paired with their
// prior version, where unchanged text boxes should be preserved individually.
//
// Matching is robust without stable page keys:
//  1. Anchor pairs are established first by key, then by exact fingerprint
//     (unchanged slides), which survive inserts, deletions and reordering.
//  2. Remaining slides are aligned in order within each anchor-bounded segment,
//     so a shape can only ever be inherited from a positionally plausible
//     neighbour, never from a distant look-alike slide.
//  3. A changed pair is only offered for shape-level merge when the two slides
//     are clearly the same slide lightly edited (shape overlap >= threshold);
//     otherwise it is regenerated. This bounds any residual mis-pairing to a
//     cosmetic, non-destructive effect on identical-text shapes.
//
// Using the part name (rather than a position integer) avoids the
// filename-vs-visible-position mismatch that arises when slides have been
// reordered in PowerPoint (sldIdLst order ≠ filename order).
func alignSlides(source slidown.Slides, existing []pptx.SlideMeta,
	newSigs map[int][]pptx.ShapeSignature, oldSigs map[string][]pptx.ShapeSignature,
) (map[int]string, map[int]string) {
	n := len(source)
	m := len(existing)
	pairOld := make([]int, n)
	for i := range pairOld {
		pairOld[i] = -1
	}
	oldUsed := make([]bool, m)
	isAnchorTarget := make([]bool, m)

	// Phase 1a: anchor by stable key (unique within a deck).
	oldByKey := map[string]int{}
	for j, e := range existing {
		if e.Key == "" {
			continue
		}
		if _, seen := oldByKey[e.Key]; seen {
			oldByKey[e.Key] = -1 // ambiguous duplicate key: never anchor on it
		} else {
			oldByKey[e.Key] = j
		}
	}
	for i := range source {
		s := source[i]
		if s == nil || s.Key == "" {
			continue
		}
		j, ok := oldByKey[s.Key]
		if !ok || j < 0 || oldUsed[j] {
			continue
		}
		pairOld[i] = j
		oldUsed[j] = true
		isAnchorTarget[j] = true
	}

	// Phase 1b: anchor unchanged slides by fingerprint, preferring the nearest
	// still-unused existing slide so anchors stay locally ordered.
	for i := range source {
		s := source[i]
		if s == nil || pairOld[i] >= 0 {
			continue
		}
		best := -1
		for j := 0; j < m; j++ {
			if oldUsed[j] || !s.MatchesFingerprint(existing[j].Fingerprint) {
				continue
			}
			if best < 0 || abs(j-i) < abs(best-i) {
				best = j
			}
		}
		if best >= 0 {
			pairOld[i] = best
			oldUsed[best] = true
			isAnchorTarget[best] = true
		}
	}

	// Phase 2: align the remaining slides in order within anchor-bounded
	// segments. A two-pointer walk never pairs across an anchor target, so
	// mis-pairing is confined to a single gap between unchanged slides.
	ei := 0
	for i := range source {
		s := source[i]
		if s == nil {
			continue
		}
		if pairOld[i] >= 0 {
			if pairOld[i]+1 > ei {
				ei = pairOld[i] + 1
			}
			continue
		}
		for ei < m && oldUsed[ei] && !isAnchorTarget[ei] {
			ei++
		}
		if ei < m && !oldUsed[ei] && !isAnchorTarget[ei] {
			pairOld[i] = ei
			oldUsed[ei] = true
			ei++
		}
	}

	// Phase 3: classify each pair as whole-slide reuse or shape-level merge.
	reuse := map[int]string{}
	shapeMerge := map[int]string{}
	for i := range source {
		s := source[i]
		if s == nil || pairOld[i] < 0 {
			continue
		}
		j := pairOld[i]
		pos := i + 1
		part := existing[j].PartName
		if s.Freeze || s.MatchesFingerprint(existing[j].Fingerprint) {
			reuse[pos] = part
			continue
		}
		newSig := newSigs[pos]
		if len(newSig) == 0 {
			continue
		}
		if pptx.ShapeOverlap(newSig, oldSigs[part]) >= shapeMergeMinOverlap {
			shapeMerge[pos] = part
		}
	}
	return reuse, shapeMerge
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// isIdentityReuse reports whether every slide is reused at its original visible
// position and the slide count is unchanged, in which case the existing file is
// already correct and need not be rewritten. The existing slice is needed to
// compare part names rather than position integers, since sldIdLst order may
// differ from filename order after a PowerPoint reorder.
func isIdentityReuse(reuse map[int]string, existing []pptx.SlideMeta, sourceLen int) bool {
	if sourceLen == 0 || sourceLen != len(existing) || len(reuse) != sourceLen {
		return false
	}
	for newPos, oldPartName := range reuse {
		if newPos < 1 || newPos > len(existing) {
			return false
		}
		if oldPartName != existing[newPos-1].PartName {
			return false
		}
	}
	return true
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// defaultOutputPath derives the output .pptx path from the input markdown path.
func defaultOutputPath(input string) string {
	base := strings.TrimSuffix(input, filepath.Ext(input))
	return base + ".pptx"
}

func init() {
	applyCmd.Flags().StringVarP(&applyOutput, "output", "o", "", "output .pptx file path (default: DECK_FILE with .pptx extension)")
	applyCmd.Flags().StringVarP(&applyCodeBlockToImageCmd, "code-block-to-image-command", "", "", "command to convert code blocks to images")
	applyCmd.Flags().StringVarP(&applyTemplate, "template", "t", "", "path to a .pptx or .potx template providing the design")
	rootCmd.AddCommand(applyCmd)
}
