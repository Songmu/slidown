package render

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// findSoffice locates a LibreOffice binary, if available.
func findSoffice() string {
	for _, name := range []string{"soffice", "libreoffice"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func libreOfficeCommand(soffice string, args ...string) *exec.Cmd {
	cmd := exec.Command(soffice, args...)
	if os.Getenv("FONTCONFIG_FILE") != "" || os.Getenv("FONTCONFIG_PATH") != "" {
		return cmd
	}
	// Headless LibreOffice on macOS may miss the fontconfig installation on PATH.
	fcMatch, err := exec.LookPath("fc-match")
	if err != nil {
		return cmd
	}
	config := filepath.Join(filepath.Dir(filepath.Dir(fcMatch)), "etc", "fonts", "fonts.conf")
	if info, err := os.Stat(config); err == nil && !info.IsDir() {
		cmd.Env = append(os.Environ(), "FONTCONFIG_FILE="+config)
	}
	return cmd
}

func TestLibreOfficeCommandFontconfig(t *testing.T) {
	prefix := t.TempDir()
	binDir := filepath.Join(prefix, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	binary := "fc-match"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if err := os.WriteFile(filepath.Join(binDir, binary), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(prefix, "etc", "fonts", "fonts.conf")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("<fontconfig/>"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name       string
		path       string
		configFile string
		configPath string
		wantFile   string
	}{
		{name: "discover prefix", path: binDir, wantFile: config},
		{name: "preserve file override", path: binDir, configFile: "custom.conf", wantFile: "custom.conf"},
		{name: "preserve path override", path: binDir, configPath: "custom-fonts"},
		{name: "fontconfig unavailable", path: t.TempDir()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PATH", tt.path)
			t.Setenv("FONTCONFIG_FILE", tt.configFile)
			t.Setenv("FONTCONFIG_PATH", tt.configPath)
			cmd := libreOfficeCommand("soffice", "--headless")
			values := make(map[string]string)
			for _, entry := range cmd.Environ() {
				key, value, _ := strings.Cut(entry, "=")
				values[key] = value
			}
			if got := values["FONTCONFIG_FILE"]; got != tt.wantFile {
				t.Errorf("FONTCONFIG_FILE = %q, want %q", got, tt.wantFile)
			}
			if got := values["FONTCONFIG_PATH"]; got != tt.configPath {
				t.Errorf("FONTCONFIG_PATH = %q, want %q", got, tt.configPath)
			}
		})
	}
}

// TestE2ERendersWithLibreOffice is a visual end-to-end test: it builds a .pptx
// covering the main element types and verifies LibreOffice can open and convert
// it to a PDF. It is skipped under `go test -short` and when LibreOffice is not
// installed.
func TestE2ERendersWithLibreOffice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LibreOffice visual e2e test in short mode")
	}
	soffice := findSoffice()
	if soffice == "" {
		t.Skip("LibreOffice (soffice) not found; skipping visual e2e test")
	}

	const src = "# Title\n\n## Subtitle\n\n" +
		"- **bold** and *italic* and `code` and ~~strike~~\n" +
		"  - nested with a [link](https://example.com)\n\n" +
		"> a block quote\n\n" +
		"| A | B |\n| --- | ---: |\n| x | 1 |\n| y | 2 |\n\n" +
		"<!--\nspeaker note here\n-->\n"

	slides := markdownSlides(t, src)

	dir := t.TempDir()
	pptxPath := filepath.Join(dir, "e2e.pptx")
	if err := ToPresentation(slides).WriteFile(pptxPath); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Use an isolated user profile dir to avoid clashing with a running instance.
	profileArg := "-env:UserInstallation=file://" + filepath.Join(dir, "louser")
	cmd := libreOfficeCommand(soffice, "--headless", profileArg, "--convert-to", "pdf", "--outdir", dir, pptxPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("soffice conversion failed: %v\n%s", err, out)
	}

	pdfPath := filepath.Join(dir, "e2e.pdf")
	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("expected PDF output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("PDF output is empty")
	}
}
