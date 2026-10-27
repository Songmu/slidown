package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/Songmu/slidown"
	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/Songmu/slidown/render"
	"github.com/fswatcher/fswatcher"
	"github.com/spf13/cobra"
)

var (
	applyOutput              string
	applyCodeBlockToImageCmd string
	applyTemplate            string
	applyWatch               bool
	debounceWatchSignals     = debounceSignals
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
		if err := validateApplyFlags(cmd); err != nil {
			return err
		}
		ctx := cmd.Context()
		if applyWatch {
			var stop context.CancelFunc
			ctx, stop = signal.NotifyContext(ctx, os.Interrupt)
			defer stop()
		}
		applyOnce := func(runCtx context.Context) error {
			return applyDeck(runCtx, cmd, f)
		}
		return runApplyMode(ctx, cmd, f, applyWatch, applyOnce, watchApply)
	},
}

const applyWatchDebounce = 250 * time.Millisecond

func runApplyMode(
	ctx context.Context,
	cmd *cobra.Command,
	deckPath string,
	watch bool,
	apply func(context.Context) error,
	watchFn func(context.Context, *cobra.Command, string, func(context.Context) error) error,
) error {
	if err := apply(ctx); err != nil {
		if !watch {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		cmd.PrintErrln(err)
	}
	if !watch {
		return nil
	}
	return watchFn(ctx, cmd, deckPath, apply)
}

func validateApplyFlags(cmd *cobra.Command) error {
	if applyWatch && cmd.Flags().Changed("template") {
		return fmt.Errorf("--watch and --template cannot be used together: --template is only for initial generation. " +
			"To use watch mode with a template, set template in the config file")
	}
	return nil
}

func applyDeck(ctx context.Context, cmd *cobra.Command, f string) error {
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

	slides, err := m.ToSlides(ctx, applyCodeBlockToImageCmd)
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
}

func watchApply(ctx context.Context, cmd *cobra.Command, deckPath string, apply func(context.Context) error) error {
	// Ensure helper goroutines (e.g. debounceSignals) are canceled on any return path.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	absDeck, err := filepath.Abs(deckPath)
	if err != nil {
		return fmt.Errorf("failed to resolve deck path: %w", err)
	}
	canonicalDeck, err := canonicalizeWatchPath(absDeck)
	if err != nil {
		return fmt.Errorf("failed to canonicalize deck path: %w", err)
	}

	w, err := fswatcher.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer w.Close()

	watchDir := filepath.Dir(absDeck)
	if err := w.Add(watchDir, fswatcher.Write|fswatcher.Create|fswatcher.Rename|fswatcher.Remove); err != nil {
		return fmt.Errorf("failed to watch %q: %w", watchDir, err)
	}

	triggers := make(chan struct{}, 1)
	defer close(triggers)
	debounced := debounceWatchSignals(ctx, triggers, applyWatchDebounce)
	errorsCh := w.Errors

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isDeckFileEvent(ev.Name, canonicalDeck) {
				continue
			}
			select {
			case triggers <- struct{}{}:
			default:
			}
		case err, ok := <-errorsCh:
			if !ok {
				errorsCh = nil
				continue
			}
			cmd.PrintErrf("watch error: %v\n", err)
		case _, ok := <-debounced:
			if !ok {
				return nil
			}
			if err := apply(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				cmd.PrintErrln(err)
			}
		}
	}
}

func debounceSignals(ctx context.Context, in <-chan struct{}, delay time.Duration) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		defer close(out)

		var timer *time.Timer
		var timerC <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case _, ok := <-in:
				if !ok {
					if timer != nil {
						timer.Stop()
					}
					return
				}
				if timer == nil {
					timer = time.NewTimer(delay)
				} else {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(delay)
				}
				timerC = timer.C
			case <-timerC:
				timerC = nil
				select {
				case out <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func canonicalizeWatchPath(path string) (string, error) {
	return fswatcher.Canonicalize(path)
}

func isDeckFileEvent(eventName, canonicalDeckPath string) bool {
	return eventName == canonicalDeckPath
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
	// keysByPos maps each output slide's 1-based visible position to the stable
	// key of the deck page that produced it. It drives the final stamping pass
	// that makes the deck source authoritative for embedded keys (see
	// pptx.StampSlideKeys): keys renamed or removed in the Markdown are updated
	// or cleared, and slides imported from other presentations are tagged so a
	// later rebuild can match them by key rather than by position.
	keysByPos := sourceKeysByPosition(sourceSlides)

	exists, err := pathExists(out)
	if err != nil {
		return false, err
	}
	if !exists {
		stamped, err := stampPresentationKeys(newPPTX, keysByPos)
		if err != nil {
			return false, err
		}
		if err := os.WriteFile(out, stamped, 0o600); err != nil {
			return false, err
		}
		return false, nil
	}

	existingBytes, err := os.ReadFile(out)
	if err != nil {
		return false, err
	}
	existingStamped, err := pptx.StampShapeKeys(existingBytes)
	if err != nil {
		return false, err
	}

	final, err := reconcilePresentation(existingStamped, newPPTX, sourceSlides, desiredTitle)
	if err != nil {
		return false, err
	}

	if final == nil {
		final, err = pptx.MergeWithExisting(out, newPPTX)
		if err != nil {
			return false, err
		}
	}

	final, err = stampPresentationKeys(final, keysByPos)
	if err != nil {
		return false, err
	}
	if bytes.Equal(final, existingBytes) {
		return true, nil
	}
	if err := os.WriteFile(out, final, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

func stampPresentationKeys(data []byte, keysByPos map[int]string) ([]byte, error) {
	stamped, err := pptx.StampSlideKeys(data, keysByPos)
	if err != nil {
		return nil, err
	}
	return pptx.StampShapeKeys(stamped)
}

// reconcilePresentation returns nil when whole-file merging is needed.
// Reuse is safe because updates render against the existing output's design,
// so copied slide relationships still reference the same layouts.
func reconcilePresentation(existingStamped, newPPTX []byte, sourceSlides slidown.Slides, desiredTitle string) ([]byte, error) {
	pkg, err := pptx.OpenPackageBytes(existingStamped)
	if err != nil {
		return nil, nil
	}
	existing, existingTitle := pkg.SlideMetasAndCoreTitle()
	// Unreadable new signatures give zero overlap and disable shape reuse.
	newSigs, _ := pptx.ShapeSignaturesByPosition(newPPTX)
	oldSigs := pkg.ShapeSignaturesByPart()
	reuse, shapeMerge := alignSlides(sourceSlides, existing, newSigs, oldSigs)
	if isIdentityReuse(reuse, existing, len(renderedSlides(sourceSlides))) {
		// Core properties are not covered by per-slide fingerprints. Keys are
		// stamped by the caller even when every slide is reused in place.
		if existingTitle == desiredTitle {
			return existingStamped, nil
		}
		return pkg.ReplaceCoreProps(newPPTX)
	}
	if len(reuse) == 0 && len(shapeMerge) == 0 {
		return nil, nil
	}

	var final []byte
	if len(reuse) > 0 {
		final, err = pkg.MergeReusingUnchangedSlides(newPPTX, reuse)
	} else {
		final, err = pkg.MergeWith(newPPTX)
	}
	if err != nil {
		return nil, err
	}
	return pkg.MergeReusingUnchangedShapes(final, shapeMerge)
}

// sourceKeysByPosition maps each rendered deck slide's 1-based output position
// to its stable key, for the key-stamping pass. Positions with no key map to
// the empty string, which clears any key the slide at that position carries.
func sourceKeysByPosition(source slidown.Slides) map[int]string {
	rendered := renderedSlides(source)
	keys := make(map[int]string, len(rendered))
	for i, s := range rendered {
		keys[i+1] = s.Key
	}
	return keys
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
	// Only slides that are actually rendered occupy a position in the output
	// package. Skipped source slides are still rendered (as hidden), so only
	// defensively nil source slides emit no slide; align on the rendered
	// subsequence to keep 1-based positions in step with ppt/slides/slideN.xml
	// and the shape-signature maps.
	rendered := renderedSlides(source)
	a := slideAlignment{
		rendered:       rendered,
		existing:       existing,
		pairOld:        make([]int, len(rendered)),
		keyed:          make([]bool, len(rendered)),
		oldUsed:        make([]bool, len(existing)),
		isAnchorTarget: make([]bool, len(existing)),
	}
	for i := range a.pairOld {
		a.pairOld[i] = -1
	}

	a.anchorByKey()
	a.anchorByFingerprint()
	// Detect reordering before positional pairs are added: only anchors are
	// evidence that positional shape-level merging is unreliable.
	reordered := anchorsReordered(a.pairOld)
	a.pairPositionally()
	return a.classify(newSigs, oldSigs, reordered)
}

type slideAlignment struct {
	rendered       []*slidown.Slide
	existing       []pptx.SlideMeta
	pairOld        []int
	keyed          []bool
	oldUsed        []bool
	isAnchorTarget []bool
}

// anchorByKey anchors by stable key (unique within a deck).
func (a *slideAlignment) anchorByKey() {
	oldByKey := map[string]int{}
	for j, e := range a.existing {
		if e.Key == "" {
			continue
		}
		if _, seen := oldByKey[e.Key]; seen {
			oldByKey[e.Key] = -1 // ambiguous duplicate key: never anchor on it
		} else {
			oldByKey[e.Key] = j
		}
	}
	for i, s := range a.rendered {
		if s.Key == "" {
			continue
		}
		j, ok := oldByKey[s.Key]
		if !ok || j < 0 || a.oldUsed[j] {
			continue
		}
		a.pairOld[i] = j
		a.keyed[i] = true
		a.oldUsed[j] = true
		a.isAnchorTarget[j] = true
	}
}

// anchorByFingerprint anchors unchanged slides, preferring the nearest
// still-unused existing slide so anchors stay locally ordered.
func (a *slideAlignment) anchorByFingerprint() {
	for i := range a.rendered {
		if a.pairOld[i] >= 0 {
			continue
		}
		best := a.nearestFingerprint(i)
		if best >= 0 {
			a.pairOld[i] = best
			a.oldUsed[best] = true
			a.isAnchorTarget[best] = true
		}
	}
}

func (a *slideAlignment) nearestFingerprint(i int) int {
	best := -1
	for j, e := range a.existing {
		if a.oldUsed[j] || !a.rendered[i].MatchesFingerprint(e.Fingerprint) {
			continue
		}
		if best < 0 || abs(j-i) < abs(best-i) {
			best = j
		}
	}
	return best
}

// pairPositionally aligns the remaining slides in order within anchor-bounded
// segments. A two-pointer walk never pairs across an anchor target, so
// mis-pairing is confined to a single gap between unchanged slides. Existing
// slides whose stable key is still present in the source are reserved for key
// matching: they are never claimed positionally, so a keyless source can't
// accidentally reuse (or freeze onto) a keyed page that was merely reordered
// out of this segment. A slide with an orphaned key (one no longer present in
// the source) may be claimed only by a keyed source slide — a likely key rename —
// so its slide is re-paired and later re-stamped, while a keyless source (e.g.
// a frozen page after an unrelated keyed slide was deleted) still skips it and
// keeps its own positional counterpart.
func (a *slideAlignment) pairPositionally() {
	sourceKeySet := make(map[string]bool)
	for _, s := range a.rendered {
		if s.Key != "" {
			sourceKeySet[s.Key] = true
		}
	}

	m := len(a.existing)
	ei := 0
	for i := range a.rendered {
		if a.pairOld[i] >= 0 {
			if a.pairOld[i]+1 > ei {
				ei = a.pairOld[i] + 1
			}
			continue
		}
		srcKeyed := a.rendered[i].Key != ""
		for ei < m && !a.isAnchorTarget[ei] && (a.oldUsed[ei] || keyBlocksPositional(a.existing[ei].Key, srcKeyed, sourceKeySet)) {
			ei++
		}
		if ei < m && !a.oldUsed[ei] && !a.isAnchorTarget[ei] && !keyBlocksPositional(a.existing[ei].Key, srcKeyed, sourceKeySet) {
			a.pairOld[i] = ei
			a.oldUsed[ei] = true
			ei++
		}
	}
}

// classify separates whole-slide reuse from shape-level merge. When anchors
// were reordered, shape-level merge is restricted to key-anchored pairs,
// whose identity is certain.
func (a *slideAlignment) classify(newSigs map[int][]pptx.ShapeSignature,
	oldSigs map[string][]pptx.ShapeSignature, reordered bool,
) (map[int]string, map[int]string) {
	reuse := map[int]string{}
	shapeMerge := map[int]string{}
	for i, s := range a.rendered {
		if a.pairOld[i] < 0 {
			continue
		}
		j := a.pairOld[i]
		pos := i + 1
		part := a.existing[j].PartName
		if s.Freeze || s.MatchesFingerprint(a.existing[j].Fingerprint) {
			reuse[pos] = part
			continue
		}
		// Changed slide: only merge shapes when the pairing is trustworthy
		// (key-anchored, or no reordering was detected) and the two slides are
		// clearly the same slide lightly edited.
		if reordered && !a.keyed[i] {
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

// keyBlocksPositional reports whether an existing slide's stable key prevents it
// from being claimed positionally in Phase 2. A key still present in the source
// (sourceKeys) is always reserved for key anchoring. An orphaned key (absent
// from the source) blocks only keyless source slides: a keyed source slide (a
// likely key rename) may re-pair with the slide, but a keyless page must not, so
// deleting a keyed slide can't divert a neighbouring keyless (frozen) page onto
// the removed slide's content.
func keyBlocksPositional(existingKey string, srcKeyed bool, sourceKeys map[string]bool) bool {
	if existingKey == "" {
		return false
	}
	if sourceKeys[existingKey] {
		return true
	}
	return !srcKeyed
}

// renderedSlides returns the source slides that produce an output slide, in
// order — i.e. excluding defensively nil slides — so their index matches the
// 1-based position of ppt/slides/slideN.xml in the package. Skipped slides are
// still rendered (as hidden slides), so they are retained here.
func renderedSlides(source slidown.Slides) []*slidown.Slide {
	out := make([]*slidown.Slide, 0, len(source))
	for _, s := range source {
		if s == nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

// anchorsReordered reports whether the established anchor pairs map to existing
// slides in a non-increasing order, which signals a reorder between the source
// and the existing presentation.
func anchorsReordered(pairOld []int) bool {
	last := -1
	for _, j := range pairOld {
		if j < 0 {
			continue
		}
		if j < last {
			return true
		}
		last = j
	}
	return false
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
	applyCmd.Flags().StringVarP(&applyTemplate, "template", "t", "", "path to a .pptx or .potx template providing the design (initial generation only)")
	applyCmd.Flags().BoolVarP(&applyWatch, "watch", "w", false, "watch the deck file and re-apply on changes")
	rootCmd.AddCommand(applyCmd)
}
