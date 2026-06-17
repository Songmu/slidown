package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	deck "github.com/Songmu/slidown"
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

func writePresentation(out string, newPPTX []byte, sourceSlides deck.Slides, allowNoOp bool) (bool, error) {
	exists, err := pathExists(out)
	if err != nil {
		return false, err
	}
	if exists {
		if allowNoOp {
			existingSlides, _, err := deck.ReadSlidesFromPPTX(out)
			if err == nil && slidesEquivalentForUpdate(sourceSlides, existingSlides) {
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
	if err := os.WriteFile(out, newPPTX, 0o600); err != nil {
		return false, err
	}
	return false, nil
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

func slidesEquivalentForUpdate(source, generated deck.Slides) bool {
	if len(source) != len(generated) {
		return false
	}
	normalized := make(deck.Slides, len(generated))
	for i, slide := range generated {
		if slide == nil {
			normalized[i] = nil
			continue
		}
		cp := *slide
		if source[i] != nil && source[i].Layout == "" {
			cp.Layout = ""
		}
		normalized[i] = &cp
	}
	return source.Equal(normalized)
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
