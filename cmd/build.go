package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/render"
	"github.com/spf13/cobra"
)

var (
	buildOutput              string
	buildCodeBlockToImageCmd string
)

var buildCmd = &cobra.Command{
	Use:   "build DECK_FILE",
	Short: "build a PowerPoint (.pptx) presentation from markdown",
	Long: `build generates a PowerPoint (.pptx) file from a markdown deck file.

The output path defaults to the input file name with a .pptx extension,
and can be overridden with the --output/-o flag.`,
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
		if out == "" {
			out = defaultOutputPath(f)
		}

		slides, err := m.ToSlides(cmd.Context(), buildCodeBlockToImageCmd)
		if err != nil {
			return fmt.Errorf("failed to convert markdown to slides: %w", err)
		}

		if err := render.ToPresentation(slides).WriteFile(out); err != nil {
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
	rootCmd.AddCommand(buildCmd)
}
