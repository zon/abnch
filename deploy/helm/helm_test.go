package helm

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// serviceImage is the registry path the delivery workflow publishes to and the
// chart runs by default.
const serviceImage = "ghcr.io/zon/abnch/abnchd"

// objectMeta is the subset of object metadata the tests read.
type objectMeta struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

// manifest is the union of the Deployment and Service fields the tests read.
type manifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   objectMeta   `yaml:"metadata"`
	Spec       manifestSpec `yaml:"spec"`
}

// manifestSpec holds the Deployment and Service spec fields in one struct, since
// a manifest carries one kind's fields and the parser ignores the rest.
type manifestSpec struct {
	Template *podTemplate   `yaml:"template"`
	Ports    []servicePort  `yaml:"ports"`
	Selector map[string]any `yaml:"selector"`
}

// podTemplate is a Deployment's pod template.
type podTemplate struct {
	Metadata objectMeta `yaml:"metadata"`
	Spec     podSpec    `yaml:"spec"`
}

// podSpec is the subset of a pod spec the tests read.
type podSpec struct {
	Containers []container `yaml:"containers"`
}

// container is the subset of a container the tests read.
type container struct {
	Name  string          `yaml:"name"`
	Image string          `yaml:"image"`
	Ports []containerPort `yaml:"ports"`
}

// containerPort is one port a container exposes.
type containerPort struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
}

// servicePort is one port a Service forwards.
type servicePort struct {
	Port       int `yaml:"port"`
	TargetPort any `yaml:"targetPort"`
}

// TestChartManifest checks the chart names itself and carries a version, so helm
// recognizes it as an installable application chart.
func TestChartManifest(t *testing.T) {
	var chart struct {
		APIVersion string `yaml:"apiVersion"`
		Name       string `yaml:"name"`
		Version    string `yaml:"version"`
		AppVersion string `yaml:"appVersion"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(chartDir(t), "Chart.yaml"))), &chart); err != nil {
		t.Fatalf("parse Chart.yaml: %v", err)
	}

	if chart.APIVersion != "v2" {
		t.Errorf("apiVersion = %q, want v2", chart.APIVersion)
	}
	if chart.Name != "abnchd" {
		t.Errorf("name = %q, want abnchd", chart.Name)
	}
	if chart.Version == "" {
		t.Error("chart names no version")
	}
	if chart.AppVersion == "" {
		t.Error("chart names no appVersion")
	}
}

// TestChartInstallsDeploymentAndService checks that the chart renders a
// Deployment running a container and a Service that reaches it, so a caller
// installs the service into a cluster without more input than the chart.
func TestChartInstallsDeploymentAndService(t *testing.T) {
	manifests := renderChart(t)

	dep, ok := findKind(manifests, "Deployment")
	if !ok {
		t.Fatalf("chart renders no Deployment:\n%s", renderManifests(manifests))
	}
	if dep.APIVersion != "apps/v1" {
		t.Errorf("Deployment apiVersion = %q, want apps/v1", dep.APIVersion)
	}
	if dep.Spec.Template == nil || len(dep.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("Deployment runs no container:\n%s", renderManifests(manifests))
	}

	if _, ok := findKind(manifests, "Service"); !ok {
		t.Errorf("chart renders no Service:\n%s", renderManifests(manifests))
	}
}

// TestDeploymentRunsServiceImage checks that the Deployment runs the service
// image the delivery workflow publishes, and that the image tag follows a chart
// value so a caller installs another version.
func TestDeploymentRunsServiceImage(t *testing.T) {
	dep, ok := findKind(renderChart(t), "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment")
	}
	c := dep.Spec.Template.Spec.Containers[0]
	if !strings.HasPrefix(c.Image, serviceImage+":") {
		t.Errorf("container image = %q, want the service image %s", c.Image, serviceImage)
	}

	dep, ok = findKind(renderChart(t, "--set", "image.tag=v9.9.9"), "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment with an overridden tag")
	}
	if got, want := dep.Spec.Template.Spec.Containers[0].Image, serviceImage+":v9.9.9"; got != want {
		t.Errorf("container image = %q, want %q", got, want)
	}
}

// TestServiceSelectsDeploymentPods checks that the Service selects the pods the
// Deployment runs and forwards to the port the container exposes, so the
// Service reaches the service.
func TestServiceSelectsDeploymentPods(t *testing.T) {
	manifests := renderChart(t)

	dep, ok := findKind(manifests, "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment")
	}
	svc, ok := findKind(manifests, "Service")
	if !ok {
		t.Fatal("chart renders no Service")
	}

	podLabels := dep.Spec.Template.Metadata.Labels
	if len(svc.Spec.Selector) == 0 {
		t.Fatalf("Service selects no pods:\n%s", renderManifests(manifests))
	}
	for key, value := range svc.Spec.Selector {
		want, _ := value.(string)
		if got := podLabels[key]; got != want {
			t.Errorf("Service selects %s=%q, but the pod template carries %q", key, want, got)
		}
	}

	ports := svc.Spec.Ports
	if len(ports) == 0 {
		t.Fatalf("Service forwards no port:\n%s", renderManifests(manifests))
	}
	if ports[0].Port == 0 {
		t.Errorf("Service forwards port %d:\n%s", ports[0].Port, renderManifests(manifests))
	}
	if target := containerPortName(dep.Spec.Template.Spec.Containers[0]); target != "" && target != ports[0].TargetPort {
		t.Errorf("Service targetPort = %v, want the container port %q", ports[0].TargetPort, target)
	}
}

// TestChartLints checks that helm accepts the chart with its default values.
func TestChartLints(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed")
	}

	out, err := exec.Command(helm, "lint", chartDir(t)).CombinedOutput()
	if err != nil {
		t.Fatalf("helm lint: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "Error") {
		t.Errorf("helm lint reported an error:\n%s", out)
	}
}

// containerPortName returns the name of a container's first port.
func containerPortName(c container) string {
	if len(c.Ports) == 0 {
		return ""
	}
	return c.Ports[0].Name
}

// renderChart renders the chart with `helm template`, skipping when helm is not
// installed, and returns the parsed manifests.
func renderChart(t *testing.T, args ...string) []manifest {
	t.Helper()
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed")
	}

	cmd := exec.Command(helm, append([]string{"template", "abnchd", chartDir(t)}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm template: %v\n%s", err, out)
	}
	return parseManifests(t, out)
}

// parseManifests decodes a stream of YAML documents, ignoring empty ones.
func parseManifests(t *testing.T, data []byte) []manifest {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var manifests []manifest
	for {
		var m manifest
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("parse helm output: %v", err)
		}
		if m.Kind != "" {
			manifests = append(manifests, m)
		}
	}
	return manifests
}

// findKind returns the first manifest of the given kind.
func findKind(manifests []manifest, kind string) (manifest, bool) {
	for _, m := range manifests {
		if m.Kind == kind {
			return m, true
		}
	}
	return manifest{}, false
}

// renderManifests names each rendered manifest, for failure output.
func renderManifests(manifests []manifest) string {
	var b strings.Builder
	for _, m := range manifests {
		b.WriteString(m.Kind + " " + m.Metadata.Name + "\n")
	}
	return b.String()
}

// chartDir is the chart under test, beside this test file.
func chartDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testDir(t), "abnchd")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// testDir is the directory holding this test and the chart.
func testDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Dir(file)
}
