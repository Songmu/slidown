/*
Copyright © 2025 Ken'ichiro Oyama <k1lowxb@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Songmu/slidown/config"
	"github.com/Songmu/slidown/version"
	"github.com/k1LoW/errors"
	"github.com/spf13/cobra"
)

var profile string

var rootCmd = &cobra.Command{
	Use:          "slidown",
	Short:        "slidown is a tool for creating PowerPoint presentations from Markdown",
	Long:         `slidown is a tool for creating PowerPoint (.pptx) presentations from Markdown.`,
	SilenceUsage: true,
	Version:      fmt.Sprintf("%s (rev:%s)", version.Version, version.Revision),
}

type errorData struct {
	StackTraces any       `json:"stack_traces"`
	CreatedAt   time.Time `json:"created_at"`
	Version     string    `json:"version"`
	Revision    string    `json:"revision"`
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrln(err)
		// Write a stack trace dump to the state directory for debugging.
		d := &errorData{
			StackTraces: errors.StackTraces(err),
			CreatedAt:   time.Now(),
			Version:     version.Version,
			Revision:    version.Revision,
		}
		if b, merr := json.Marshal(d); merr == nil {
			dumpPath := filepath.Join(config.StateHomePath(), "error.json")
			if werr := os.WriteFile(dumpPath, b, 0o600); werr != nil {
				rootCmd.Printf("failed to write error.json to %s: %v\n", dumpPath, werr)
			}
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "", "", "profile name")
}
