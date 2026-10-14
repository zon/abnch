package devconfig

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreAddsMissingLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, gitignoreName)
	if err := os.WriteFile(path, []byte("bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignore(path, ConfigFileName, KeyFileName); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	for _, want := range []string{"bin/", ConfigFileName, KeyFileName} {
		if !strings.Contains(got, want) {
			t.Errorf(".gitignore missing %q:\n%s", want, got)
		}
	}
}

func TestEnsureGitignoreIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, gitignoreName)

	if err := EnsureGitignore(path, ConfigFileName, KeyFileName); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGitignore(path, ConfigFileName, KeyFileName); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, path)
	if strings.Count(got, ConfigFileName) != 1 {
		t.Errorf("%s appears %d times, want 1:\n%s", ConfigFileName, strings.Count(got, ConfigFileName), got)
	}
	if strings.Count(got, KeyFileName) != 1 {
		t.Errorf("%s appears %d times, want 1:\n%s", KeyFileName, strings.Count(got, KeyFileName), got)
	}
}

// TestRepoGitignoreExcludesGeneratedFiles pins the checked-in .gitignore, so a
// fresh checkout ignores the generated config and key before the first setup
// run ever writes them.
func TestRepoGitignoreExcludesGeneratedFiles(t *testing.T) {
	content := readFile(t, filepath.Join(repoRoot(t), gitignoreName))
	for _, name := range []string{ConfigFileName, KeyFileName} {
		if !hasGitignoreLine(content, name) {
			t.Errorf("checked-in .gitignore does not exclude %s:\n%s", name, content)
		}
	}
}

// TestSetLocalAuthExcludesGeneratedFilesFromGit runs the setup in a fresh
// repository and asks git itself whether the written config and key are
// ignored, so an entry git does not honor fails even when the text appears in
// .gitignore.
func TestSetLocalAuthExcludesGeneratedFilesFromGit(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}

	repo := t.TempDir()
	if _, err := runGit(t, git, repo, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	if _, err := SetLocalAuth(Options{
		Port:     8080,
		RepoRoot: repo,
		HomeDir:  t.TempDir(),
		In:       strings.NewReader("db\n5432\nabnch\nadmin\npw\n\n\n"),
		Out:      &bytes.Buffer{},
	}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{ConfigFileName, KeyFileName} {
		if _, err := os.Stat(filepath.Join(repo, name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
		if _, err := runGit(t, git, repo, "check-ignore", name); err != nil {
			t.Errorf("git does not exclude %s: %v", name, err)
		}
	}

	status, err := runGit(t, git, repo, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{ConfigFileName, KeyFileName} {
		if strings.Contains(status, name) {
			t.Errorf("git status lists the generated %s:\n%s", name, status)
		}
	}
}

// hasGitignoreLine reports whether content holds the exact line, ignoring
// surrounding whitespace.
func hasGitignoreLine(content, line string) bool {
	for _, candidate := range strings.Split(content, "\n") {
		if strings.TrimSpace(candidate) == line {
			return true
		}
	}
	return false
}

// runGit runs a git command in dir and returns its combined output.
func runGit(t *testing.T, git, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestEnsureDevKeyKeepsExistingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), KeyFileName)
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureDevKey(path); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "keep" {
		t.Errorf("key = %q, want the existing key", got)
	}
}

func TestEnsureDevKeyWritesPrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), KeyFileName)
	if err := EnsureDevKey(path); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "BEGIN PRIVATE KEY") {
		t.Errorf("key is not a PEM private key:\n%s", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
