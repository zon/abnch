package containers

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// serviceImage is the registry path the delivery workflow publishes to, as the
// Deployment Design names it.
const serviceImage = "ghcr.io/zon/abnch/abnchd"

// workflowStep is one step of a GitHub Actions job.
type workflowStep struct {
	Name            string `yaml:"name"`
	Uses            string `yaml:"uses"`
	Run             string `yaml:"run"`
	If              string `yaml:"if"`
	ContinueOnError bool   `yaml:"continue-on-error"`
}

// workflowJob is one job of a GitHub Actions workflow.
type workflowJob struct {
	Steps []workflowStep `yaml:"steps"`
}

// workflow is the part of a GitHub Actions workflow the delivery tests read.
type workflow struct {
	Env  map[string]string      `yaml:"env"`
	Jobs map[string]workflowJob `yaml:"jobs"`
}

// TestWorkflowDeliversServiceImage checks that one workflow builds the service
// image, scans it with Trivy, and pushes it, in that order.
func TestWorkflowDeliversServiceImage(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	build, ok := indexOf(steps, buildsServiceImage)
	if !ok {
		t.Fatalf("no step builds the service image from the Containerfile:\n%s", renderSteps(steps))
	}
	scan, ok := indexOf(steps, scansServiceImage)
	if !ok {
		t.Fatalf("no step scans the service image with Trivy:\n%s", renderSteps(steps))
	}
	push, ok := indexOf(steps, pushesServiceImage)
	if !ok {
		t.Fatalf("no step pushes the service image:\n%s", renderSteps(steps))
	}

	if build > scan || scan > push {
		t.Errorf("pipeline runs build at %d, scan at %d, push at %d, want build then scan then push", build, scan, push)
	}
	for _, i := range []int{build, scan, push} {
		if !referencesImage(steps[i].Run) {
			t.Errorf("step %q does not use the workflow image %s:\n%s", steps[i].Name, serviceImage, steps[i].Run)
		}
	}
}

// TestWorkflowStopsOnHighOrCritical checks that the scan limits itself to high
// and critical vulnerabilities and fails the pipeline when it finds one.
func TestWorkflowStopsOnHighOrCritical(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	scan, ok := indexOf(steps, scansServiceImage)
	if !ok {
		t.Fatalf("no step scans the service image with Trivy:\n%s", renderSteps(steps))
	}
	run := steps[scan].Run
	if !strings.Contains(run, "--severity") || !strings.Contains(run, "HIGH") || !strings.Contains(run, "CRITICAL") {
		t.Errorf("scan step does not limit the scan to high and critical vulnerabilities:\n%s", run)
	}
	if !strings.Contains(run, "--exit-code 1") {
		t.Errorf("scan step does not fail on a found vulnerability:\n%s", run)
	}
}

// TestWorkflowPushesToServiceImageRegistry checks that the workflow publishes
// the service image to the registry path the Deployment Design names.
func TestWorkflowPushesToServiceImageRegistry(t *testing.T) {
	wf, steps := deliveryWorkflow(t)

	if got := wf.Env["IMAGE"]; got != serviceImage {
		t.Errorf("workflow image is %q, want %q", got, serviceImage)
	}

	push, ok := indexOf(steps, pushesServiceImage)
	if !ok {
		t.Fatalf("no step pushes the service image:\n%s", renderSteps(steps))
	}
	if !strings.Contains(steps[push].Run, "push") {
		t.Errorf("push step %q does not push the image:\n%s", steps[push].Name, steps[push].Run)
	}
}

// TestWorkflowPushesOnlyAfterCleanScan checks that a failing scan stops the
// pipeline before the push, so a vulnerable image never reaches the registry.
func TestWorkflowPushesOnlyAfterCleanScan(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	scan, ok := indexOf(steps, scansServiceImage)
	if !ok {
		t.Fatalf("no step scans the service image with Trivy:\n%s", renderSteps(steps))
	}
	push, ok := indexOf(steps, pushesServiceImage)
	if !ok {
		t.Fatalf("no step pushes the service image:\n%s", renderSteps(steps))
	}
	if scan > push {
		t.Errorf("pipeline pushes at %d before it scans at %d", push, scan)
	}
	if ignoresFailure(steps[scan]) {
		t.Errorf("scan step %q ignores a found vulnerability, so the push still runs", steps[scan].Name)
	}
	if ignoresFailure(steps[push]) {
		t.Errorf("push step %q can run after a failed scan", steps[push].Name)
	}
}

// deliveryWorkflow returns the one workflow that builds the service image and
// the steps of the job that does so.
func deliveryWorkflow(t *testing.T) (workflow, []workflowStep) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	paths, err := filepath.Glob(filepath.Join(dir, "*.y*ml"))
	if err != nil {
		t.Fatal(err)
	}

	var (
		wf    workflow
		steps []workflowStep
	)
	for _, path := range paths {
		var candidate workflow
		if err := yaml.Unmarshal([]byte(readFile(t, path)), &candidate); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, job := range candidate.Jobs {
			if _, ok := indexOf(job.Steps, buildsServiceImage); !ok {
				continue
			}
			if steps != nil {
				t.Fatalf("more than one job builds the service image; keep the delivery pipeline in one job")
			}
			wf, steps = candidate, job.Steps
		}
	}
	if steps == nil {
		t.Fatalf("no workflow in %s builds the service image from the Containerfile", dir)
	}
	return wf, steps
}

// buildsServiceImage reports whether a step builds the service image from the
// Containerfile the repo ships.
func buildsServiceImage(s workflowStep) bool {
	return strings.Contains(s.Run, "build") &&
		strings.Contains(s.Run, filepath.Join("deploy", "containers", "Containerfile"))
}

// scansServiceImage reports whether a step scans an image with Trivy.
func scansServiceImage(s workflowStep) bool {
	return strings.Contains(s.Run, "trivy image")
}

// pushesServiceImage reports whether a step pushes the image.
func pushesServiceImage(s workflowStep) bool {
	return strings.Contains(s.Run, "podman push")
}

// referencesImage reports whether a command publishes or reads the image named
// by the workflow's IMAGE variable.
func referencesImage(run string) bool {
	return strings.Contains(run, "$IMAGE") || strings.Contains(run, "${IMAGE}")
}

// ignoresFailure reports whether a step keeps the pipeline going when an
// earlier step fails, or runs even after a failure.
func ignoresFailure(s workflowStep) bool {
	if s.ContinueOnError {
		return true
	}
	condition := strings.ReplaceAll(s.If, " ", "")
	return strings.Contains(condition, "always()") || strings.Contains(condition, "failure()")
}

// indexOf returns the position of the first step matching pred.
func indexOf(steps []workflowStep, pred func(workflowStep) bool) (int, bool) {
	for i, s := range steps {
		if pred(s) {
			return i, true
		}
	}
	return -1, false
}

// renderSteps names each step with its position, for failure output.
func renderSteps(steps []workflowStep) string {
	var b strings.Builder
	for i, s := range steps {
		fmt.Fprintf(&b, "%d: %s\n", i, s.Name)
	}
	return b.String()
}
