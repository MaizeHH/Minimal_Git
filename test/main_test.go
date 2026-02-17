package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "gitre-*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	binName := "gitre-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath = filepath.Join(tempDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, "..")
	if err := buildCmd.Run(); err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func Test_Init(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gitre-*")
	defer os.RemoveAll(tempDir)

	runCommand(t, tempDir, "init")

	expectedPaths := []string{
		".gitre",
		".gitre/objects",
		".gitre/refs/heads",
		".gitre/index",
		".gitre/HEAD",
		".gitreignore",
	}

	for _, path := range expectedPaths {
		fullPath := filepath.Join(tempDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected path %s to exist, but it doesn't", path)
		}
	}
}

func Test_Add(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gitre-add-*")
	defer os.RemoveAll(tempDir)

	setupInit(t, tempDir)

	filesToAdd := []string{"file1.txt", "file2.txt"}
	setupAdd(t, tempDir, filesToAdd...)

	indexContent := readIndex(t, tempDir)
	for _, fileName := range filesToAdd {
		if !strings.Contains(indexContent, fileName) {
			t.Errorf("Index missing expected file: %s", fileName)
		}
	}

	setupAdd(t, tempDir, "file1.txt")
	indexContent = readIndex(t, tempDir)
	if count := strings.Count(indexContent, "file1.txt"); count > 1 {
		t.Errorf("Duplicate entry found for file1.txt")
	}

	subDir := "myfolder"
	subFile := filepath.Join(subDir, "nested.txt")
	os.Mkdir(filepath.Join(tempDir, subDir), 0755)
	os.WriteFile(filepath.Join(tempDir, subFile), []byte("nested"), 0644)

	runCommand(t, tempDir, "add", subDir)

	indexContent = readIndex(t, tempDir)
	if !strings.Contains(indexContent, "myfolder/nested.txt") {
		t.Errorf("Index missing nested file from subdirectory add")
	}

	os.WriteFile(filepath.Join(tempDir, "extra.txt"), []byte("extra"), 0644)
	runCommand(t, tempDir, "add", ".", "extra.txt")

	indexContent = readIndex(t, tempDir)
	if !strings.Contains(indexContent, "extra.txt") {
		t.Error("Index missing extra.txt after adding '.'")
	}
	if count := strings.Count(indexContent, "extra.txt"); count > 1 {
		t.Errorf("Deduplication failed for 'add . extra.txt'")
	}

	if strings.Contains(indexContent, ".gitre/") {
		t.Error("Error: .gitre internal files were added to index!")
	}
}

func Test_Commit(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gitre-commit-*")
	defer os.RemoveAll(tempDir)

	setupInit(t, tempDir)
	setupAdd(t, tempDir, "file1.txt", "file2.txt")
	runCommand(t, tempDir, "commit", "First commit message")

	refPath := filepath.Join(tempDir, ".gitre", "refs", "heads", "main")
	commitHashBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Commit failed: branch ref 'main' was not created: %v", err)
	}
	commitHash := string(commitHashBytes)

	if len(commitHash) == 0 {
		t.Error("Branch ref 'main' is empty; expected a commit hash")
	}

	folder := commitHash[:2]
	file := commitHash[2:]
	objectPath := filepath.Join(tempDir, ".gitre", "objects", folder, file)

	if _, err := os.Stat(objectPath); os.IsNotExist(err) {
		t.Fatalf("Commit object not found at %s", objectPath)
	}
}

func Test_Checkout(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gitre-checkout-*")
	defer os.RemoveAll(tempDir)

	setupInit(t, tempDir)
	setupAdd(t, tempDir, "mainfile.txt")
	runCommand(t, tempDir, "commit", "Main commit")

	newBranch := "new_branch"
	cmd := exec.Command(binPath, "checkout", newBranch)
	cmd.Dir = tempDir

	cmd.Stdin = strings.NewReader("1\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Checkout command failed: %v\nOutput: %s", err, string(out))
	}

	headContent, _ := os.ReadFile(filepath.Join(tempDir, ".gitre", "HEAD"))
	expectedHead := "ref: refs/heads/" + newBranch
	if string(headContent) != expectedHead {
		t.Errorf("HEAD not updated correctly. Got: %s, Want: %s", string(headContent), expectedHead)
	}

	refPath := filepath.Join(tempDir, ".gitre", "refs", "heads", newBranch)
	if _, err := os.Stat(refPath); os.IsNotExist(err) {
		t.Errorf("New branch ref file was not created at %s", refPath)
	}
}

func Test_Log(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gitre-log-*")
	defer os.RemoveAll(tempDir)

	setupInit(t, tempDir)
	setupAdd(t, tempDir, "file1.txt")
	runCommand(t, tempDir, "commit", "Initial commit")

	refPath := filepath.Join(tempDir, ".gitre", "refs", "heads", "main")
	hash1, _ := os.ReadFile(refPath)

	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("data"), 0644)
	runCommand(t, tempDir, "add", "file2.txt")
	runCommand(t, tempDir, "commit", "Second commit")

	hash2, _ := os.ReadFile(refPath)

	output := runCommand(t, tempDir, "log")
	if !strings.Contains(output, string(hash1)) {
		t.Errorf("Log missing first commit hash: %s", string(hash1))
	}
	if !strings.Contains(output, string(hash2)) {
		t.Errorf("Log missing second commit hash: %s", string(hash2))
	}
}

func Test_Status(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "gitre-status-*")
	defer os.RemoveAll(tempDir)

	setupInit(t, tempDir)

	os.WriteFile(filepath.Join(tempDir, "to-be-modified.txt"), []byte("v1"), 0644)
	os.WriteFile(filepath.Join(tempDir, "to-be-deleted.txt"), []byte("bye"), 0644)
	runCommand(t, tempDir, "add", "to-be-modified.txt", "to-be-deleted.txt")
	runCommand(t, tempDir, "commit", "Initial commit")

	os.WriteFile(filepath.Join(tempDir, "to-be-modified.txt"), []byte("v2"), 0644)
	runCommand(t, tempDir, "add", "to-be-modified.txt")

	os.WriteFile(filepath.Join(tempDir, "new-file.txt"), []byte("hello"), 0644)
	runCommand(t, tempDir, "add", "new-file.txt")

	os.WriteFile(filepath.Join(tempDir, "untracked.txt"), []byte("mystery"), 0644)

	output := runCommand(t, tempDir, "status")

	if !strings.Contains(output, "new-file.txt") {
		t.Error("Status should show new-file.txt in New files")
	}
	if !strings.Contains(output, "to-be-modified.txt") {
		t.Error("Status should show to-be-modified.txt in Modified (Staging)")
	}

	if !strings.Contains(output, "untracked.txt") {
		t.Error("Status should show untracked.txt in Untracked")
	}
}

func runCommand(t *testing.T, dir string, name string, args ...string) string {
	cmd := exec.Command(binPath, append([]string{name}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command '%s %v' failed: %v\nOutput: %s", name, args, err, string(out))
	}
	return string(out)
}

func setupInit(t *testing.T, dir string) {
	runCommand(t, dir, "init")
	if _, err := os.Stat(filepath.Join(dir, ".gitre")); os.IsNotExist(err) {
		t.Fatal(".gitre folder was not created")
	}
}

func setupAdd(t *testing.T, dir string, filenames ...string) {
	for _, f := range filenames {
		os.WriteFile(filepath.Join(dir, f), []byte("content"), 0644)
	}
	runCommand(t, dir, "add", filenames...)
}

func readIndex(t *testing.T, tempDir string) string {
	b, err := os.ReadFile(filepath.Join(tempDir, ".gitre", "index"))
	if err != nil {
		t.Fatalf("Failed to read index: %v", err)
	}
	return string(b)
}
