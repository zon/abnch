package helm

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/service"
	"github.com/zon/abnch/internal/version"
)

// serviceImage is the registry path the delivery workflow publishes to and the
// chart runs by default.
const serviceImage = "ghcr.io/zon/abnch/abnchd"

// objectMeta is the subset of object metadata the tests read.
type objectMeta struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

// manifest is the union of the Deployment, Service, and ConfigMap fields the
// tests read.
type manifest struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   objectMeta        `yaml:"metadata"`
	Data       map[string]string `yaml:"data"`
	Spec       manifestSpec      `yaml:"spec"`
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
	Volumes    []volume    `yaml:"volumes"`
}

// container is the subset of a container the tests read.
type container struct {
	Name         string          `yaml:"name"`
	Image        string          `yaml:"image"`
	Args         []string        `yaml:"args"`
	Env          []envVar        `yaml:"env"`
	Ports        []containerPort `yaml:"ports"`
	VolumeMounts []volumeMount   `yaml:"volumeMounts"`
}

// envVar is one container environment variable and where its value comes from.
type envVar struct {
	Name      string        `yaml:"name"`
	ValueFrom *envValueFrom `yaml:"valueFrom"`
}

// envValueFrom names the source of an environment variable.
type envValueFrom struct {
	SecretKeyRef *secretKeyRef `yaml:"secretKeyRef"`
}

// secretKeyRef reads one key from a Kubernetes Secret.
type secretKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

// volume is a pod volume and the ConfigMap it reads.
type volume struct {
	Name      string        `yaml:"name"`
	ConfigMap *configMapRef `yaml:"configMap"`
}

// configMapRef names the ConfigMap a volume reads.
type configMapRef struct {
	Name string `yaml:"name"`
}

// volumeMount attaches a volume at a container path.
type volumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
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

// TestChartImageDefaultsToRepoVersion checks that the chart's image values
// default to the service image at the repo version, so a caller installs the
// chart and runs the version its apps report.
func TestChartImageDefaultsToRepoVersion(t *testing.T) {
	var chart struct {
		AppVersion string `yaml:"appVersion"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(chartDir(t), "Chart.yaml"))), &chart); err != nil {
		t.Fatalf("parse Chart.yaml: %v", err)
	}
	if chart.AppVersion != version.Version {
		t.Errorf("chart appVersion = %q, want the repo version %q", chart.AppVersion, version.Version)
	}

	var values struct {
		Image struct {
			Repository string `yaml:"repository"`
			Tag        string `yaml:"tag"`
		} `yaml:"image"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(chartDir(t), "values.yaml"))), &values); err != nil {
		t.Fatalf("parse values.yaml: %v", err)
	}
	if values.Image.Repository != serviceImage {
		t.Errorf("values image.repository = %q, want %q", values.Image.Repository, serviceImage)
	}

	dep, ok := findKind(renderChart(t), "Deployment")
	if !ok {
		t.Fatalf("chart renders no Deployment")
	}
	if got, want := dep.Spec.Template.Spec.Containers[0].Image, serviceImage+":"+version.Version; got != want {
		t.Errorf("default container image = %q, want %q", got, want)
	}
}

// TestChartImageRepositoryIsValue checks that the image repository follows a
// chart value, so a caller installs the image from another registry.
func TestChartImageRepositoryIsValue(t *testing.T) {
	dep, ok := findKind(renderChart(t, "--set", "image.repository=registry.example.com/abnchd"), "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment with an overridden repository")
	}
	if got, want := dep.Spec.Template.Spec.Containers[0].Image, "registry.example.com/abnchd:"+version.Version; got != want {
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

// TestChartRendersServiceConfig checks that the chart renders its values into a
// service config the service reads: the url, Postgres connection, and the OIDC
// and workload auth settings all come through.
func TestChartRendersServiceConfig(t *testing.T) {
	manifests := renderChart(t,
		"--set", "config.url=http://abnch.example.com:8080",
		"--set", "config.postgres.host=db",
		"--set", "config.postgres.port=5433",
		"--set", "config.postgres.dbname=abnch",
		"--set", "config.postgres.user=admin",
		"--set", "config.postgres.sslmode=require",
		"--set", "config.postgres.timeZone=America/New_York",
		"--set", "config.oidc.issuer=https://id.example.com",
		"--set", "config.oidc.clientId=abnch",
		"--set", "config.oidc.redirectUrl=http://abnch.example.com:8080/callback",
		"--set", "config.oidc.scopes={openid,email}",
		"--set", "config.oidc.roleClaim=groups",
		"--set", "config.oidc.roleMap.admin=Admin",
		"--set", "config.oauth.issuers[0].issuer=https://k8s.example.com",
		"--set", "config.oauth.issuers[0].audience=abnch",
		"--set", "config.oauth.issuers[0].roleClaim=sub",
		"--set", "config.oauth.issuers[0].roleMap.build=Service",
	)

	cfg := renderedConfig(t, manifests)

	if got, want := cfg.URL, "http://abnch.example.com:8080"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
	pg := cfg.Postgres
	if pg.Host != "db" || pg.Port != 5433 || pg.DBName != "abnch" || pg.User != "admin" ||
		pg.SSLMode != "require" || pg.TimeZone != "America/New_York" {
		t.Errorf("postgres = %+v, want the chart values", pg)
	}
	oidc := cfg.OIDC
	if oidc.Issuer != "https://id.example.com" || oidc.ClientID != "abnch" ||
		oidc.RedirectURL != "http://abnch.example.com:8080/callback" ||
		oidc.RoleClaim != "groups" || oidc.RoleMap["admin"] != "Admin" {
		t.Errorf("oidc = %+v, want the chart values", oidc)
	}
	if pg.Password != "" {
		t.Errorf("postgres.password = %q, want the secret to supply it, not the values", pg.Password)
	}
	if oidc.ClientSecret != "" {
		t.Errorf("oidc.clientSecret = %q, want the secret to supply it, not the values", oidc.ClientSecret)
	}
	if got, want := strings.Join(oidc.Scopes, ","), "openid,email"; got != want {
		t.Errorf("oidc.scopes = %q, want %q", got, want)
	}
	if len(cfg.OAuth.Issuers) != 1 {
		t.Fatalf("oauth.issuers = %+v, want one issuer", cfg.OAuth.Issuers)
	}
	iss := cfg.OAuth.Issuers[0]
	if iss.Issuer != "https://k8s.example.com" || iss.Audience != "abnch" ||
		iss.RoleClaim != "sub" || iss.RoleMap["build"] != "Service" {
		t.Errorf("oauth issuer = %+v, want the chart values", iss)
	}
}

// TestChartValuesHoldNoSecretValues checks that the chart values carry the
// secret's name and keys rather than the Postgres password and OIDC client
// secret themselves.
func TestChartValuesHoldNoSecretValues(t *testing.T) {
	var values struct {
		Secrets struct {
			Name                string `yaml:"name"`
			PostgresPasswordKey string `yaml:"postgresPasswordKey"`
			OIDCClientSecretKey string `yaml:"oidcClientSecretKey"`
		} `yaml:"secrets"`
		Config struct {
			Postgres struct {
				Password string `yaml:"password"`
			} `yaml:"postgres"`
			OIDC struct {
				ClientSecret string `yaml:"clientSecret"`
			} `yaml:"oidc"`
		} `yaml:"config"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(chartDir(t), "values.yaml"))), &values); err != nil {
		t.Fatalf("parse values.yaml: %v", err)
	}

	if values.Secrets.PostgresPasswordKey == "" || values.Secrets.OIDCClientSecretKey == "" {
		t.Errorf("values name no secret keys, so the chart cannot read the secret: %+v", values.Secrets)
	}
	if values.Config.Postgres.Password != "" {
		t.Errorf("values hold postgres.password %q, want them to name the secret key instead", values.Config.Postgres.Password)
	}
	if values.Config.OIDC.ClientSecret != "" {
		t.Errorf("values hold oidc.clientSecret %q, want them to name the secret key instead", values.Config.OIDC.ClientSecret)
	}
}

// TestDeploymentInjectsSecretConfig checks that the Deployment reads the
// Postgres password and OIDC client secret from a Kubernetes Secret the chart
// values name, so the chart never carries those values.
func TestDeploymentInjectsSecretConfig(t *testing.T) {
	dep, ok := findKind(renderChart(t,
		"--set", "secrets.name=abnchd-secret",
		"--set", "secrets.postgresPasswordKey=pg-pass",
		"--set", "secrets.oidcClientSecretKey=oidc-secret",
	), "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment")
	}
	env := dep.Spec.Template.Spec.Containers[0].Env

	pg, ok := secretRef(env, service.PostgresPasswordEnv)
	if !ok {
		t.Fatalf("container has no %s env, so the Postgres password never comes from the secret:\n%+v",
			service.PostgresPasswordEnv, env)
	}
	if pg.Name != "abnchd-secret" || pg.Key != "pg-pass" {
		t.Errorf("%s reads %s/%s, want abnchd-secret/pg-pass", service.PostgresPasswordEnv, pg.Name, pg.Key)
	}

	oidc, ok := secretRef(env, service.OIDCClientSecretEnv)
	if !ok {
		t.Fatalf("container has no %s env, so the OIDC client secret never comes from the secret:\n%+v",
			service.OIDCClientSecretEnv, env)
	}
	if oidc.Name != "abnchd-secret" || oidc.Key != "oidc-secret" {
		t.Errorf("%s reads %s/%s, want abnchd-secret/oidc-secret", service.OIDCClientSecretEnv, oidc.Name, oidc.Key)
	}
}

// TestDeploymentSecretDefaultsToReleaseName checks that the secret name defaults
// to the release's object name, so a caller reads the release's secret without
// naming it.
func TestDeploymentSecretDefaultsToReleaseName(t *testing.T) {
	dep, ok := findKind(renderChart(t), "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment")
	}

	pg, ok := secretRef(dep.Spec.Template.Spec.Containers[0].Env, service.PostgresPasswordEnv)
	if !ok {
		t.Fatalf("container has no %s env", service.PostgresPasswordEnv)
	}
	if pg.Name != dep.Metadata.Name {
		t.Errorf("secret name = %q, want the release name %q", pg.Name, dep.Metadata.Name)
	}
}

// TestDeploymentSecretSuppliesServiceConfig checks that the secret the
// Deployment reads reaches the config the service runs with, so the Postgres
// password and OIDC client secret come from the secret.
func TestDeploymentSecretSuppliesServiceConfig(t *testing.T) {
	manifests := renderChart(t,
		"--set", "config.url=http://abnch.example.com:8080",
		"--set", "config.postgres.host=db",
		"--set", "config.postgres.dbname=abnch",
		"--set", "config.postgres.user=admin",
		"--set", "config.postgres.sslmode=require",
		"--set", "config.postgres.timeZone=UTC",
		"--set", "config.oidc.issuer=https://id.example.com",
		"--set", "config.oidc.clientId=abnch",
		"--set", "secrets.name=abnchd-secret",
		"--set", "secrets.postgresPasswordKey=pg-pass",
		"--set", "secrets.oidcClientSecretKey=oidc-secret",
	)

	cfg := renderedConfig(t, manifests)
	if cfg.Postgres.Password != "" || cfg.OIDC.ClientSecret != "" {
		t.Fatalf("rendered config holds secret values: postgres.password=%q oidc.clientSecret=%q",
			cfg.Postgres.Password, cfg.OIDC.ClientSecret)
	}

	dep, ok := findKind(manifests, "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment")
	}
	getenv := secretEnv(dep.Spec.Template.Spec.Containers[0].Env, map[string]string{
		"abnchd-secret/pg-pass":     "pg-secret",
		"abnchd-secret/oidc-secret": "oidc-secret",
	})

	cfg = service.ApplySecretEnv(cfg, func(name string) string { return getenv[name] })
	if cfg.Postgres.Password != "pg-secret" {
		t.Errorf("postgres.password = %q, want the secret value", cfg.Postgres.Password)
	}
	if cfg.OIDC.ClientSecret != "oidc-secret" {
		t.Errorf("oidc.clientSecret = %q, want the secret value", cfg.OIDC.ClientSecret)
	}
	if err := devconfig.CheckPostgres(cfg.Postgres); err != nil {
		t.Errorf("service config is not usable with the secret values: %v", err)
	}
}

// secretRef returns the environment entry that reads a secret key.
func secretRef(env []envVar, name string) (secretKeyRef, bool) {
	for _, e := range env {
		if e.Name == name && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			return *e.ValueFrom.SecretKeyRef, true
		}
	}
	return secretKeyRef{}, false
}

// secretEnv resolves each secret-backed environment variable to the value the
// Secret holds at the named key, as Kubernetes does at pod start.
func secretEnv(env []envVar, secret map[string]string) map[string]string {
	out := map[string]string{}
	for _, e := range env {
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := e.ValueFrom.SecretKeyRef
		out[e.Name] = secret[ref.Name+"/"+ref.Key]
	}
	return out
}

// TestDeploymentReadsRenderedConfig checks that the Deployment mounts the config
// the chart renders and starts abnchd with that file's path, so the service runs
// from the chart values.
func TestDeploymentReadsRenderedConfig(t *testing.T) {
	manifests := renderChart(t)

	dep, ok := findKind(manifests, "Deployment")
	if !ok {
		t.Fatalf("chart renders no Deployment:\n%s", renderManifests(manifests))
	}
	cm, ok := findKind(manifests, "ConfigMap")
	if !ok {
		t.Fatalf("chart renders no ConfigMap:\n%s", renderManifests(manifests))
	}
	if cm.Data["config.yaml"] == "" {
		t.Fatalf("ConfigMap %s holds no config.yaml:\n%s", cm.Metadata.Name, renderManifests(manifests))
	}

	c := dep.Spec.Template.Spec.Containers[0]
	if len(c.Args) == 0 {
		t.Fatalf("container runs no args, so it never names the config path:\n%s", renderManifests(manifests))
	}

	configPath := c.Args[0]
	if path.Base(configPath) != "config.yaml" {
		t.Errorf("container config argument = %q, want the rendered config.yaml", configPath)
	}

	mountPath, ok := configMountPath(dep.Spec.Template.Spec.Volumes, c.VolumeMounts, cm.Metadata.Name, configPath)
	if !ok {
		t.Errorf("container mounts no volume from ConfigMap %s at %s:\n%+v",
			cm.Metadata.Name, configPath, c.VolumeMounts)
	}
	if mountPath != "" && configPath != path.Join(mountPath, "config.yaml") {
		t.Errorf("container reads %q, want the config ConfigMap mounted at %q", configPath, mountPath)
	}
}

// renderedConfig returns the service config the chart renders from its values.
func renderedConfig(t *testing.T, manifests []manifest) devconfig.Config {
	t.Helper()
	cm, ok := findKind(manifests, "ConfigMap")
	if !ok {
		t.Fatalf("chart renders no ConfigMap:\n%s", renderManifests(manifests))
	}
	cfg, err := devconfig.UnmarshalConfig([]byte(cm.Data["config.yaml"]))
	if err != nil {
		t.Fatalf("rendered config is not valid: %v\n%s", err, cm.Data["config.yaml"])
	}
	return cfg
}

// configMountPath returns the mount path of the volume that reads configMap and
// holds the given config path, so a caller checks the container reads the
// rendered config.
func configMountPath(volumes []volume, mounts []volumeMount, configMap, configPath string) (string, bool) {
	byName := make(map[string]volume, len(volumes))
	for _, v := range volumes {
		byName[v.Name] = v
	}
	for _, m := range mounts {
		v, ok := byName[m.Name]
		if !ok || v.ConfigMap == nil || v.ConfigMap.Name != configMap {
			continue
		}
		if strings.HasPrefix(configPath, m.MountPath+"/") {
			return m.MountPath, true
		}
	}
	return "", false
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
