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

func TestJustConfigDelegatesEveryStepToAbnchctl(t *testing.T) {
	steps := nonEmptyLines(dryRunConfig(t))
	if len(steps) != 2 {
		t.Fatalf("config recipe runs %d steps, want the two abnchctl commands:\n%q", len(steps), steps)
	}
	for _, step := range steps {
		if !strings.Contains(step, "./cmd/abnchctl") {
			t.Errorf("config recipe step %q does not drive the abnchctl CLI", step)
		}
	}
	if !strings.Contains(steps[0], "set local-auth") {
		t.Errorf("config recipe first step = %q, want set local-auth", steps[0])
	}
	if !strings.Contains(steps[1], "check config") {
		t.Errorf("config recipe second step = %q, want check config", steps[1])
	}
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestJustConfigDefaultsToPort8080(t *testing.T) {
	out := dryRunConfig(t)
	if !strings.Contains(out, "abnchctl set local-auth --port 8080") {
		t.Errorf("just config does not default to port 8080:\n%s", out)
	}
}

func TestJustConfigForwardsGivenPort(t *testing.T) {
	out := dryRunConfig(t, "9090")
	if !strings.Contains(out, "abnchctl set local-auth --port 9090") {
		t.Errorf("just config does not forward the given port:\n%s", out)
	}
}

// dryRunConfig renders the config recipe's commands without running them.
func dryRunConfig(t *testing.T, args ...string) string {
	t.Helper()
	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}
	cmdArgs := append([]string{"--justfile", filepath.Join(repoRoot(t), "Justfile"), "--dry-run", "config"}, args...)
	out, err := exec.Command(just, cmdArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("just --dry-run config: %v\n%s", err, out)
	}
	return string(out)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
