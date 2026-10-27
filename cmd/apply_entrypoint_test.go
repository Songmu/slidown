package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Songmu/slidown/pptx"
	"github.com/spf13/cobra"
)

func TestApplyDeckOutputSelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		flag, front bool
	}{
		{"default", false, false},
		{"frontmatter", false, true},
		{"flag overrides frontmatter", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateApplyForTest(t)
			dir := t.TempDir()
			deck := filepath.Join(dir, "deck.md")
			want := filepath.Join(dir, "deck.pptx")
			source := "---\ntitle: Deck & title\n"
			if tc.front {
				want = filepath.Join(dir, "frontmatter.pptx")
				source += fmt.Sprintf("output: %q\n", want)
			}
			if tc.flag {
				want = filepath.Join(dir, "flag.pptx")
				applyOutput = want
			}
			source += "---\n# First\n\nbody\n"
			writeApplyInputForTest(t, deck, source)
			var output bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&output)
			if err := applyDeck(context.Background(), cmd, deck); err != nil {
				t.Fatal(err)
			}
			if got, expected := output.String(), fmt.Sprintf("Wrote %s (1 slide(s))\n", want); got != expected {
				t.Fatalf("output = %q, want %q", got, expected)
			}
			if title := pptx.ReadCoreTitle(want); title != "Deck & title" {
				t.Fatalf("core title = %q, want %q", title, "Deck & title")
			}
			for _, name := range []string{"deck.pptx", "frontmatter.pptx", "flag.pptx"} {
				candidate := filepath.Join(dir, name)
				if candidate != want {
					if _, err := os.Stat(candidate); !os.IsNotExist(err) {
						t.Errorf("unexpected output %s: %v", candidate, err)
					}
				}
			}
		})
	}
}

func TestApplyDeckTemplateSelection(t *testing.T) {
	template := buildTemplateFileForTest(t)
	master := zipRawPartsForTest(t, template)["ppt/slideMasters/slideMaster1.xml"]
	master = bytes.Replace(master, []byte("</p:sldMaster>"), []byte("<!-- selected template --></p:sldMaster>"), 1)
	injectZipPartForTest(t, template, "ppt/slideMasters/slideMaster1.xml", master)
	for _, tc := range []struct {
		name, flag, config string
	}{
		{"config template", "", template},
		{"flag overrides missing config template", template, filepath.Join(t.TempDir(), "missing.pptx")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateApplyForTest(t)
			dir := t.TempDir()
			deck := filepath.Join(dir, "deck.md")
			applyOutput = filepath.Join(dir, "deck.pptx")
			applyTemplate = tc.flag
			configPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "slidown", "config.yml")
			writeApplyInputForTest(t, configPath, fmt.Sprintf("template: %q\n", tc.config))
			writeApplyInputForTest(t, deck, "# First\n")
			cmd := &cobra.Command{}
			var output bytes.Buffer
			cmd.SetOut(&output)
			if err := applyDeck(context.Background(), cmd, deck); err != nil {
				t.Fatal(err)
			}
			got := zipRawPartsForTest(t, applyOutput)
			want := zipRawPartsForTest(t, template)
			if !bytes.Equal(got["ppt/slideMasters/slideMaster1.xml"], want["ppt/slideMasters/slideMaster1.xml"]) {
				t.Fatal("did not preserve the selected template's slide master")
			}
			applyTemplate = ""
			writeApplyInputForTest(t, configPath, "template: missing-on-update.pptx\n")
			output.Reset()
			if err := applyDeck(context.Background(), cmd, deck); err != nil {
				t.Fatalf("update should reuse the existing output instead of the config template: %v", err)
			}
			if expected := fmt.Sprintf("Updated %s (1 slide(s))\n", applyOutput); output.String() != expected {
				t.Fatalf("output = %q, want %q", output.String(), expected)
			}
		})
	}
}

func TestApplyDeckErrors(t *testing.T) {
	for _, tc := range []struct {
		name, source, config, template, command, want string
		missingDeck, missingOutputDir                 bool
	}{
		{name: "config", source: "# Title\n", config: "breaks: [", want: "failed to load config:"},
		{name: "missing deck", missingDeck: true, want: "missing.md"},
		{name: "conversion", source: "# Title\n\n```go\ncode\n```\n", command: "slidown-test-command-that-does-not-exist", want: "failed to convert markdown to slides:"},
		{name: "template", source: "# Title\n", template: "missing.pptx", want: "failed to load template:"},
		{name: "write", source: "# Title\n", missingOutputDir: true, want: "failed to write presentation:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateApplyForTest(t)
			dir := t.TempDir()
			deck := filepath.Join(dir, "missing.md")
			applyOutput = filepath.Join(dir, "deck.pptx")
			if !tc.missingDeck {
				writeApplyInputForTest(t, deck, tc.source)
			}
			if tc.config != "" {
				writeApplyInputForTest(t, filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "slidown", "config.yml"), tc.config)
			}
			if tc.missingOutputDir {
				applyOutput = filepath.Join(dir, "missing", "deck.pptx")
			}
			if tc.template != "" {
				applyTemplate = filepath.Join(dir, tc.template)
			}
			applyCodeBlockToImageCmd = tc.command
			var output bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&output)
			err := applyDeck(context.Background(), cmd, deck)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if output.Len() != 0 {
				t.Fatalf("reported success after failure: %q", output.String())
			}
			if _, err := os.Stat(applyOutput); !os.IsNotExist(err) {
				t.Fatalf("failed apply left an output file: %v", err)
			}
		})
	}
}

func writeApplyInputForTest(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
