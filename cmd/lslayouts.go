package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/md"
	"github.com/Songmu/slidown/pptx"
	"github.com/spf13/cobra"
)

var lsLayoutsCmd = &cobra.Command{
	Use:   "ls-layouts TEMPLATE",
	Short: "list the slide layouts available in a .pptx/.potx template",
	Long: `ls-layouts prints the layout names available in a .pptx or .potx
template, so they can be referenced from a page configuration such as
<!-- {"layout":"..."} -->.

The argument may be a .pptx or .potx template directly, or a markdown deck file
whose template is resolved from the --template flag, its frontmatter, or the
config.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tmpl, err := resolveTemplate(args[0])
		if err != nil {
			return err
		}
		if tmpl == nil {
			cmd.Println("(built-in design)")
			cmd.Println("Title and Content")
			return nil
		}
		for _, l := range tmpl.Layouts {
			if l.Name != "" {
				cmd.Println(l.Name)
			}
		}
		return nil
	},
}

// resolveTemplate loads a template from a path that is either a PowerPoint
// template file (.pptx / .potx) or a markdown deck whose template is
// configured. It returns (nil, nil) when the argument is a markdown deck with
// no template (the built-in design is used).
func resolveTemplate(path string) (*pptx.Template, error) {
	if isTemplateFile(path) {
		tmpl, err := pptx.LoadTemplate(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load template %q: %w", path, err)
		}
		return tmpl, nil
	}

	cfg, err := config.Load(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	m, err := md.ParseFile(path, cfg)
	if err != nil {
		return nil, err
	}
	templatePath := lsLayoutsTemplate
	if templatePath == "" && m.Frontmatter != nil {
		templatePath = m.Frontmatter.Template
	}
	if templatePath == "" {
		return nil, nil
	}
	tmpl, err := pptx.LoadTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load template %q: %w", templatePath, err)
	}
	return tmpl, nil
}

var lsLayoutsTemplate string

// isTemplateFile reports whether path looks like a PowerPoint template that
// LoadTemplate can read directly (a .pptx presentation or .potx template).
func isTemplateFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".pptx" || ext == ".potx"
}

func init() {
	lsLayoutsCmd.Flags().StringVarP(&lsLayoutsTemplate, "template", "t", "", "path to a .pptx or .potx template (overrides the deck's configured template)")
	rootCmd.AddCommand(lsLayoutsCmd)
}
