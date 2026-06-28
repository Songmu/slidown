package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteWritesErrorDumpOnFreshStateHome(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		rootCmd.SetArgs([]string{"apply", "does-not-exist.md"})
		Execute()
		return
	}

	tmpDir := t.TempDir()
	stateHome := filepath.Join(tmpDir, "state")
	helper := exec.Command(os.Args[0], "-test.run=TestExecuteWritesErrorDumpOnFreshStateHome")
	helper.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"XDG_STATE_HOME="+stateHome,
	)

	var stdout, stderr bytes.Buffer
	helper.Stdout = &stdout
	helper.Stderr = &stderr

	err := helper.Run()
	if err == nil {
		t.Fatal("expected command failure")
	}
	if !strings.Contains(stderr.String(), "does-not-exist.md") {
		t.Fatalf("stderr should include the real error, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "failed to write error.json") {
		t.Fatalf("stderr should not include dump write failure, got %q", stderr.String())
	}
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("expected exit error, got %v", err)
	}

	dumpPath := filepath.Join(stateHome, "slidown", "error.json")
	b, readErr := os.ReadFile(dumpPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%s): %v", dumpPath, readErr)
	}

	var errorDump errorData
	if unmarshalErr := json.Unmarshal(b, &errorDump); unmarshalErr != nil {
		t.Fatalf("json.Unmarshal(error.json): %v", unmarshalErr)
	}
	if errorDump.CreatedAt.IsZero() {
		t.Fatal("error dump should include created_at")
	}
	if errorDump.StackTraces == nil {
		t.Fatal("error dump should include stack_traces")
	}
}
