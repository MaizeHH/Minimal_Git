package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func Test_Init(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitre-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	binaryName := "gitre-test"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "..")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, string(buildOutput))
	}

	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command 'init' failed: %v\nOutput: %s", err, string(output))
	}

	repoPath := filepath.Join(tempDir, ".gitre")
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Errorf("Expected .gitre directory was not created at %s", repoPath)
	}

	headFile := filepath.Join(repoPath, "HEAD")
	if _, err := os.Stat(headFile); os.IsNotExist(err) {
		t.Errorf("Expected HEAD file was not created")
	}
}
