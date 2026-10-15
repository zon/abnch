package devconfig

import (
	"os"
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

// TestJustServiceLoadsLocalServiceConfig checks that the service recipe starts
// abnchd against the local service config at `.abnchd/config.yaml`.
func TestJustServiceLoadsLocalServiceConfig(t *testing.T) {
	steps := nonEmptyLines(dryRunService(t))
	if len(steps) != 1 {
		t.Fatalf("service recipe runs %d steps, want the one abnchd command:\n%q", len(steps), steps)
	}
	if !strings.Contains(steps[0], "./cmd/abnchd") {
		t.Errorf("service recipe step %q does not drive the abnchd CLI", steps[0])
	}
	if !strings.Contains(steps[0], ".abnchd/config.yaml") {
		t.Errorf("service recipe step %q does not load .abnchd/config.yaml", steps[0])
	}
}

// postgresBlock stands in for the Postgres details a human or an agent writes
// into the local service config before running the recipe.
const postgresBlock = "postgres:\n" +
	"  host: db\n" +
	"  port: 5432\n" +
	"  dbname: abnch\n" +
	"  user: admin\n" +
	"  password: pw\n" +
	"  sslmode: disable\n" +
	"  timeZone: UTC\n"

// TestJustConfigSetsUpLocalService runs the real config recipe over a
// hand-written Postgres config and checks that both steps pass.
func TestJustConfigSetsUpLocalService(t *testing.T) {
	out, err := runConfigRecipe(t, postgresBlock)
	if err != nil {
		t.Fatalf("just config = %v, want success:\n%s", err, out)
	}
	if !strings.Contains(out, "is ready") {
		t.Errorf("just config did not confirm the config:\n%s", out)
	}
}

// TestJustConfigFailsWhenSetLocalAuthFails runs the config recipe with a
// malformed local service config, so set local-auth fails to read it. The
// recipe must exit non-zero, report the failure, and never reach check config.
func TestJustConfigFailsWhenSetLocalAuthFails(t *testing.T) {
	out, err := runConfigRecipe(t, "postgres: [\n")
	if err == nil {
		t.Fatalf("just config = nil, want a non-zero exit when set local-auth fails:\n%s", out)
	}
	if !strings.Contains(out, "Recipe `config` failed") {
		t.Errorf("just config does not report the failing recipe:\n%s", out)
	}
	if strings.Contains(out, "is ready") {
		t.Errorf("config ran check config after set local-auth failed:\n%s", out)
	}
}

// TestJustConfigFailsWhenCheckConfigFails runs the config recipe with no
// Postgres block, so set local-auth succeeds but check config fails. The recipe
// must exit non-zero and report it.
func TestJustConfigFailsWhenCheckConfigFails(t *testing.T) {
	out, err := runConfigRecipe(t, "")
	if err == nil {
		t.Fatalf("just config = nil, want a non-zero exit when check config fails:\n%s", out)
	}
	if !strings.Contains(out, "postgres config is missing") {
		t.Errorf("just config does not report the check config failure:\n%s", out)
	}
	if !strings.Contains(out, "Recipe `config` failed") {
		t.Errorf("just config does not report the failing recipe:\n%s", out)
	}
}

// runConfigRecipe runs the real config recipe end to end and returns just's
// combined output. It seeds seed as the local service config, then builds
// abnchctl and substitutes a stub `go` that runs that binary from an isolated
// repo root and HOME, so the recipe's writes stay in temp directories.
func runConfigRecipe(t *testing.T, seed string) (string, error) {
	t.Helper()

	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not installed")
	}

	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "abnchctl")
	build := exec.Command(goTool, "build", "-o", bin, "./cmd/abnchctl")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build abnchctl: %v\n%s", err, out)
	}

	work := t.TempDir()
	if seed != "" {
		if err := writeConfigFile(ConfigPath(work), seed); err != nil {
			t.Fatal(err)
		}
	}
	stubDir := t.TempDir()
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"run\" ]; then shift; fi\n" +
		"shift\n" +
		"cd '" + work + "' || exit 1\n" +
		"exec '" + bin + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(stubDir, "go"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(just, "--justfile", filepath.Join(root, "Justfile"), "config")
	cmd.Env = envWith(map[string]string{
		"PATH": stubDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME": t.TempDir(),
	})
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// envWith returns the process environment with each override replacing any
// existing value.
func envWith(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		key := kv[:strings.IndexByte(kv, '=')]
		if _, ok := overrides[key]; ok {
			continue
		}
		env = append(env, kv)
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

// dryRunConfig renders the config recipe's commands without running them.
func dryRunConfig(t *testing.T, args ...string) string {
	t.Helper()
	return dryRun(t, "config", args...)
}

// dryRunService renders the service recipe's commands without running them.
func dryRunService(t *testing.T) string {
	t.Helper()
	return dryRun(t, "service")
}

// dryRun renders a recipe's commands without running them.
func dryRun(t *testing.T, recipe string, args ...string) string {
	t.Helper()
	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}
	cmdArgs := append([]string{"--justfile", filepath.Join(repoRoot(t), "Justfile"), "--dry-run", recipe}, args...)
	out, err := exec.Command(just, cmdArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("just --dry-run %s: %v\n%s", recipe, err, out)
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
