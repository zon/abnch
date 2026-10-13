package devconfig

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestJustfileHasConfigRecipe(t *testing.T) {
	root := repoRoot(t)
	content := readFile(t, filepath.Join(root, "Justfile"))
	if !strings.Contains(content, "config") {
		t.Fatalf("Justfile has no config recipe:\n%s", content)
	}

	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}
	out, err := exec.Command(just, "--justfile", filepath.Join(root, "Justfile"), "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("just --list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "config") {
		t.Errorf("just --list does not list config:\n%s", out)
	}
}

func TestJustfileDelegatesToAbnchctl(t *testing.T) {
	content := readFile(t, filepath.Join(repoRoot(t), "Justfile"))
	for _, want := range []string{"abnchctl set local-auth", "abnchctl check config"} {
		if !strings.Contains(content, want) {
			t.Errorf("Justfile does not run %q:\n%s", want, content)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
