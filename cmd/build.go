package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

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
		if templatePath == "" && m.Frontmatter != nil {
			templatePath = m.Frontmatter.Template
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

		if err := pres.WriteFile(out); err != nil {
			return fmt.Errorf("failed to write presentation: %w", err)
		}
		cmd.Printf("Wrote %s (%d slide(s))\n", out, len(slides))
		return nil
	},
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
