package helm

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// chartPath is the chart the delivery workflow lints, packages, and publishes.
const chartPath = "deploy/helm/abnchd"

// chartRegistry is the OCI repository the delivery workflow publishes the chart
// to, as the Deployment Design names it.
const chartRegistry = "oci://ghcr.io/zon/abnch/charts"

// workflowStep is one step of a GitHub Actions job.
type workflowStep struct {
	Name            string `yaml:"name"`
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
	Jobs map[string]workflowJob `yaml:"jobs"`
}

// TestWorkflowPublishesChartAfterImage checks that the delivery workflow lints,
// packages, and publishes the chart only after it pushes the service image.
func TestWorkflowPublishesChartAfterImage(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	imagePush, ok := indexOf(steps, pushesServiceImage)
	if !ok {
		t.Fatalf("no step pushes the service image:\n%s", renderSteps(steps))
	}
	lint, ok := indexOf(steps, lintsChart)
	if !ok {
		t.Fatalf("no step lints the chart at %s:\n%s", chartPath, renderSteps(steps))
	}
	pack, ok := indexOf(steps, packagesChart)
	if !ok {
		t.Fatalf("no step packages the chart at %s:\n%s", chartPath, renderSteps(steps))
	}
	publish, ok := indexOf(steps, publishesChart)
	if !ok {
		t.Fatalf("no step publishes the chart:\n%s", renderSteps(steps))
	}

	if lint > pack || pack > publish {
		t.Errorf("pipeline lints at %d, packages at %d, publishes at %d, want lint then package then publish", lint, pack, publish)
	}
	for _, i := range []int{lint, pack, publish} {
		if i < imagePush {
			t.Errorf("step %q runs at %d before the image push at %d", steps[i].Name, i, imagePush)
		}
	}
}

// TestWorkflowPublishesChartAtRepoVersion checks that the workflow publishes
// abnchd to the chart registry at the repo version.
func TestWorkflowPublishesChartAtRepoVersion(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	pack, ok := indexOf(steps, packagesChart)
	if !ok {
		t.Fatalf("no step packages the chart:\n%s", renderSteps(steps))
	}
	if !strings.Contains(steps[pack].Run, "steps.version.outputs.value") {
		t.Errorf("package step does not package the chart at the repo version:\n%s", steps[pack].Run)
	}

	publish, ok := indexOf(steps, publishesChart)
	if !ok {
		t.Fatalf("no step publishes the chart:\n%s", renderSteps(steps))
	}
	run := steps[publish].Run
	if !strings.Contains(run, chartRegistry) {
		t.Errorf("publish step does not push to %s:\n%s", chartRegistry, run)
	}
	for _, want := range []string{
		"dist/abnchd-${{ steps.version.outputs.value }}.tgz",
		"steps.version.outputs.value",
	} {
		if !strings.Contains(run, want) {
			t.Errorf("publish step does not publish abnchd at the repo version (%q missing):\n%s", want, run)
		}
	}
}

// TestWorkflowLogsInToChartRegistry checks that the workflow logs Helm in to the
// registry with the job's own token before it publishes the chart.
func TestWorkflowLogsInToChartRegistry(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	publish, ok := indexOf(steps, publishesChart)
	if !ok {
		t.Fatalf("no step publishes the chart:\n%s", renderSteps(steps))
	}
	run := steps[publish].Run
	login := strings.Index(run, "helm registry login ghcr.io")
	push := strings.Index(run, "helm push")
	if login < 0 {
		t.Fatalf("publish step does not log Helm in to ghcr.io:\n%s", run)
	}
	if push >= 0 && login > push {
		t.Errorf("publish step logs in after it pushes:\n%s", run)
	}
	if !strings.Contains(run, "secrets.GITHUB_TOKEN") || !strings.Contains(run, "github.actor") {
		t.Errorf("publish step does not log in with the job's own token:\n%s", run)
	}
}

// TestWorkflowPublishesNoChartAfterFailedScan checks that a failed image scan
// stops the pipeline before the image push and the chart publish.
func TestWorkflowPublishesNoChartAfterFailedScan(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	scan, ok := indexOf(steps, scansServiceImage)
	if !ok {
		t.Fatalf("no step scans the service image:\n%s", renderSteps(steps))
	}
	imagePush, ok := indexOf(steps, pushesServiceImage)
	if !ok {
		t.Fatalf("no step pushes the service image:\n%s", renderSteps(steps))
	}
	publish, ok := indexOf(steps, publishesChart)
	if !ok {
		t.Fatalf("no step publishes the chart:\n%s", renderSteps(steps))
	}

	if scan > imagePush || imagePush > publish {
		t.Errorf("pipeline scans at %d, pushes the image at %d, publishes the chart at %d, want scan then push then publish",
			scan, imagePush, publish)
	}
	for _, i := range []int{imagePush, publish} {
		if ignoresFailure(steps[i]) {
			t.Errorf("step %q can run after a failed scan", steps[i].Name)
		}
	}
}

// TestWorkflowSkipsChartAlreadyPublished checks that the workflow asks the
// registry for abnchd at the repo version and pushes the chart only when it is
// absent, so a published chart version never changes.
func TestWorkflowSkipsChartAlreadyPublished(t *testing.T) {
	_, steps := deliveryWorkflow(t)

	publish, ok := indexOf(steps, publishesChart)
	if !ok {
		t.Fatalf("no step publishes the chart:\n%s", renderSteps(steps))
	}
	lines := strings.Split(steps[publish].Run, "\n")

	condition, end, ok := skipWhenChartPresent(lines)
	if !ok {
		t.Fatalf("publish step does not skip the chart when the registry already holds it:\n%s", steps[publish].Run)
	}
	chart := chartRegistry + "/abnchd"
	if !strings.Contains(condition, chart) {
		t.Errorf("skip condition does not check %s:\n%s", chart, condition)
	}
	if !strings.Contains(condition, "steps.version.outputs.value") {
		t.Errorf("skip condition does not check the repo version:\n%s", condition)
	}

	push := lineContaining(lines, "helm push")
	if push < 0 {
		t.Fatalf("no step pushes the chart:\n%s", steps[publish].Run)
	}
	if push < end {
		t.Errorf("publish step pushes the chart at line %d, before the registry check ends at line %d:\n%s",
			push, end, steps[publish].Run)
	}
}

// skipWhenChartPresent returns the condition and closing line of the check that
// skips the publish when the registry already holds the chart.
func skipWhenChartPresent(lines []string) (string, int, bool) {
	for i, line := range lines {
		condition := strings.TrimSpace(line)
		if !strings.HasPrefix(condition, "if ") || !strings.Contains(condition, "helm show chart") {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) != "fi" {
				continue
			}
			return condition, j, strings.Contains(strings.Join(lines[i+1:j], "\n"), "exit 0")
		}
	}
	return "", -1, false
}

// deliveryWorkflow returns the steps of the job that builds the service image.
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

// lintsChart reports whether a step lints the chart.
func lintsChart(s workflowStep) bool {
	return strings.Contains(s.Run, "helm lint") && strings.Contains(s.Run, chartPath)
}

// packagesChart reports whether a step packages the chart.
func packagesChart(s workflowStep) bool {
	return strings.Contains(s.Run, "helm package") && strings.Contains(s.Run, chartPath)
}

// publishesChart reports whether a step publishes the chart.
func publishesChart(s workflowStep) bool {
	return strings.Contains(s.Run, "helm push")
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

// lineContaining returns the first line containing substr.
func lineContaining(lines []string, substr string) int {
	for i, line := range lines {
		if strings.Contains(line, substr) {
			return i
		}
	}
	return -1
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

// repoRoot is the module root above the test directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(testDir(t), "..", ".."))
}
