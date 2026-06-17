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
	buildOutput              string
	buildCodeBlockToImageCmd string
	buildTemplate            string
)

var buildCmd = &cobra.Command{
	Use:   "build DECK_FILE",
	Short: "build a PowerPoint (.pptx) presentation from markdown",
	Long: `build generates a PowerPoint (.pptx) file from a markdown deck file.

The output path defaults to the input file name with a .pptx extension,
and can be overridden with the --output/-o flag.

A pre-existing output file is treated as the update target and reused as the
template when no explicit template is supplied.

A .pptx template (its theme, slide masters and layouts) can be supplied
with --template or the "template" frontmatter/config field.`,
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

		out := buildOutput
		if out == "" && m.Frontmatter != nil && m.Frontmatter.Output != "" {
			out = m.Frontmatter.Output
		}
		if out == "" {
			out = defaultOutputPath(f)
		}

		templatePath := buildTemplate
		useExistingAsTemplate := false
		if templatePath == "" && m.Frontmatter != nil {
			templatePath = m.Frontmatter.Template
		}
		if templatePath == "" {
			exists, err := pathExists(out)
			if err != nil {
				return fmt.Errorf("failed to inspect output path: %w", err)
			}
			if exists {
				templatePath = out
				useExistingAsTemplate = true
			}
		}

		slides, err := m.ToSlides(cmd.Context(), buildCodeBlockToImageCmd)
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

		updated, err := writePresentation(out, buf.Bytes(), slides, useExistingAsTemplate)
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

func writePresentation(out string, newPPTX []byte, sourceSlides slidown.Slides, allowNoOp bool) (bool, error) {
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

	// When the existing file is reused as the design template, preserve the
	// slides that should keep their existing content: slides whose source did
	// not change (so manual edits survive) and slides explicitly frozen via
	// configuration. Slides are matched to their existing counterpart by stable
	// key (falling back to position), so reuse and freeze survive inserts,
	// deletions and reordering. Change detection compares each slide's embedded
	// source fingerprint against the freshly computed one.
	if allowNoOp {
		if existing, err := pptx.ReadSlideMetas(out); err == nil {
			reuse := buildReuseMap(sourceSlides, existing)
			switch {
			case isIdentityReuse(reuse, len(sourceSlides), len(existing)):
				// Every slide is reused in place: keep the existing file as is.
				return true, nil
			case len(reuse) > 0:
				merged, err := pptx.MergeReusingUnchangedSlides(out, newPPTX, reuse)
				if err != nil {
					return false, err
				}
				if err := os.WriteFile(out, merged, 0o600); err != nil {
					return false, err
				}
				return true, nil
			}
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

// buildReuseMap matches source slides to slides in the existing presentation and
// returns a map of new 1-based position -> existing 1-based position for slides
// whose existing part should be kept. A source slide is matched to an existing
// slide by stable key when both carry one, otherwise positionally (same index)
// when neither does. A matched slide is reused when it is frozen or when its
// source fingerprint still matches the existing slide's embedded fingerprint.
//
// Matching by key means reuse and freeze survive inserts, deletions and
// reordering; an empty fingerprint (e.g. stripped by another editor) never
// matches, forcing a safe regenerate.
func buildReuseMap(source slidown.Slides, existing []pptx.SlideMeta) map[int]int {
	byKey := map[string]int{}
	for i, m := range existing {
		if m.Key != "" {
			byKey[m.Key] = i
		}
	}

	reuse := map[int]int{}
	usedOld := make([]bool, len(existing))
	for i := range source {
		s := source[i]
		if s == nil {
			continue
		}
		oldIdx := -1
		switch {
		case s.Key != "":
			if j, ok := byKey[s.Key]; ok {
				oldIdx = j
			}
		case i < len(existing) && existing[i].Key == "":
			// Keyless slides fall back to positional matching.
			oldIdx = i
		}
		if oldIdx < 0 || usedOld[oldIdx] {
			continue
		}
		if s.Freeze || s.MatchesFingerprint(existing[oldIdx].Fingerprint) {
			reuse[i+1] = oldIdx + 1
			usedOld[oldIdx] = true
		}
	}
	return reuse
}

// isIdentityReuse reports whether every slide is reused at its original
// position and the slide count is unchanged, in which case the existing file is
// already correct and need not be rewritten.
func isIdentityReuse(reuse map[int]int, sourceLen, existingLen int) bool {
	if sourceLen == 0 || sourceLen != existingLen || len(reuse) != sourceLen {
		return false
	}
	for newPos, oldPos := range reuse {
		if newPos != oldPos {
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
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "", "output .pptx file path (default: DECK_FILE with .pptx extension)")
	buildCmd.Flags().StringVarP(&buildCodeBlockToImageCmd, "code-block-to-image-command", "", "", "command to convert code blocks to images")
	buildCmd.Flags().StringVarP(&buildTemplate, "template", "t", "", "path to a .pptx template providing the design")
	rootCmd.AddCommand(buildCmd)
}
