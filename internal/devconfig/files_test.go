package devconfig

import (
	"os"
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
