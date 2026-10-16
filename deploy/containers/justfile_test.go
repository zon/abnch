package containers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestJustfileHasContainerRecipe checks that the Justfile offers the container
// recipe that builds and scans the service image.
func TestJustfileHasContainerRecipe(t *testing.T) {
	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}

	out, err := exec.Command(just, "--justfile", filepath.Join(repoRoot(t), "Justfile"), "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("just --list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "container") {
		t.Errorf("just --list does not list the container recipe:\n%s", out)
	}
}

// TestJustContainerBuildsServiceImage checks that the container recipe builds
// the service image from the Containerfile the repo ships.
func TestJustContainerBuildsServiceImage(t *testing.T) {
	steps := containerRecipeSteps(t)
	if len(steps) < 2 {
		t.Fatalf("container recipe runs %d steps, want a build and a scan:\n%q", len(steps), steps)
	}

	build := steps[0]
	if !strings.Contains(build, "build") {
		t.Errorf("container recipe step %q does not build an image", build)
	}
	if !strings.Contains(build, filepath.Join("deploy", "containers", "Containerfile")) {
		t.Errorf("container recipe step %q does not use the service Containerfile", build)
	}
	if builtImage(build) == "" {
		t.Errorf("container recipe step %q tags no image for the scan", build)
	}
}

// TestJustContainerScansForHighAndCritical checks that the container recipe
// scans the image with Trivy, limited to high and critical vulnerabilities, and
// fails the recipe when it finds one.
func TestJustContainerScansForHighAndCritical(t *testing.T) {
	steps := containerRecipeSteps(t)
	if len(steps) < 2 {
		t.Fatalf("container recipe runs %d steps, want a scan after the build:\n%q", len(steps), steps)
	}

	scan := steps[len(steps)-1]
	if !strings.Contains(scan, "trivy") || !strings.Contains(scan, "image") {
		t.Errorf("container recipe step %q does not scan the image with Trivy", scan)
	}
	if !strings.Contains(scan, "--severity") || !strings.Contains(scan, "HIGH") || !strings.Contains(scan, "CRITICAL") {
		t.Errorf("container recipe step %q does not limit the scan to high and critical vulnerabilities", scan)
	}
	if !strings.Contains(scan, "--exit-code 1") {
		t.Errorf("container recipe step %q does not fail on a found vulnerability", scan)
	}
}

// TestJustContainerScansTheImageItBuilds checks that the scan reads the image
// the build step produced, rather than another tag.
func TestJustContainerScansTheImageItBuilds(t *testing.T) {
	steps := containerRecipeSteps(t)
	if len(steps) < 2 {
		t.Fatalf("container recipe runs %d steps, want a build and a scan:\n%q", len(steps), steps)
	}

	build, scan := steps[0], steps[len(steps)-1]
	image := builtImage(build)
	if image == "" {
		t.Fatalf("container recipe build step %q tags no image", build)
	}
	if !strings.Contains(scan, image) {
		t.Errorf("container recipe scans %q but builds %q", scan, image)
	}
}

// TestJustContainerRunsBuildThenScan runs the real container recipe with stub
// podman and trivy tools, so the build and scan stay offline, and checks that
// the recipe builds the image before scanning it.
func TestJustContainerRunsBuildThenScan(t *testing.T) {
	output, calls, err := runContainerRecipe(t, 0)
	if err != nil {
		t.Fatalf("just container = %v, want success for a clean scan:\n%s\n%s", err, output, calls)
	}

	build := strings.Index(calls, "podman build")
	scan := strings.Index(calls, "trivy image")
	if build < 0 {
		t.Fatalf("container recipe did not build the service image:\n%s", calls)
	}
	if scan < 0 {
		t.Fatalf("container recipe did not scan the image with Trivy:\n%s", calls)
	}
	if build > scan {
		t.Errorf("container recipe scans before it builds:\n%s", calls)
	}
}

// TestJustContainerFailsWhenScanFindsVulnerability runs the real container
// recipe with a trivy stub that reports a found vulnerability, and checks that
// the recipe fails rather than passing the image along.
func TestJustContainerFailsWhenScanFindsVulnerability(t *testing.T) {
	output, calls, err := runContainerRecipe(t, 1)
	if err == nil {
		t.Fatalf("just container = nil, want a non-zero exit when the scan fails:\n%s\n%s", output, calls)
	}
	if !strings.Contains(calls, "trivy image") {
		t.Errorf("container recipe failed before it scanned the image:\n%s", calls)
	}
}

// runContainerRecipe runs the real container recipe with stub podman and trivy
// tools on PATH. The trivy stub exits with scanExit, standing in for the scan
// result, and both stubs record their arguments. It returns just's combined
// output, the recorded calls, and just's exit result.
func runContainerRecipe(t *testing.T, scanExit int) (output, calls string, err error) {
	t.Helper()
	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}

	stubDir := t.TempDir()
	recordDir := t.TempDir()
	writeStub(t, filepath.Join(stubDir, "podman"), "podman", recordDir, 0)
	writeStub(t, filepath.Join(stubDir, "trivy"), "trivy", recordDir, scanExit)

	cmd := exec.Command(just, "--justfile", filepath.Join(repoRoot(t), "Justfile"), "container")
	cmd.Env = envWithPath(stubDir)
	out, err := cmd.CombinedOutput()

	recorded, readErr := os.ReadFile(filepath.Join(recordDir, "calls"))
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(out), string(recorded), err
}

// writeStub writes a shim at path that records its arguments to a calls file in
// recordDir and exits with code.
func writeStub(t *testing.T, path, name, recordDir string, code int) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\necho %s \"$@\" >> %s\nexit %d\n",
		name, filepath.Join(recordDir, "calls"), code)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// envWithPath returns the process environment with PATH prefixed by dir.
func envWithPath(dir string) []string {
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "PATH=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// containerRecipeSteps renders the container recipe's commands without running
// them.
func containerRecipeSteps(t *testing.T) []string {
	t.Helper()
	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}

	out, err := exec.Command(just, "--justfile", filepath.Join(repoRoot(t), "Justfile"), "--dry-run", "container").CombinedOutput()
	if err != nil {
		t.Fatalf("just --dry-run container: %v\n%s", err, out)
	}

	var steps []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			steps = append(steps, line)
		}
	}
	return steps
}

// builtImage returns the tag a build command passes to its -t flag.
func builtImage(build string) string {
	fields := strings.Fields(build)
	for i, field := range fields {
		if field == "-t" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}
