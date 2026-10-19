package helm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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

// manifestSpec holds the Deployment, Service, Certificate, and Issuer spec
// fields in one struct, since a manifest carries one kind's fields and the
// parser ignores the rest.
type manifestSpec struct {
	Template   *podTemplate   `yaml:"template"`
	Ports      []servicePort  `yaml:"ports"`
	Selector   map[string]any `yaml:"selector"`
	SecretName string         `yaml:"secretName"`
	DNSNames   []string       `yaml:"dnsNames"`
	IssuerRef  issuerRef      `yaml:"issuerRef"`
	IsCA       bool           `yaml:"isCA"`
	SelfSigned *emptySpec     `yaml:"selfSigned"`
	CA         *caSpec        `yaml:"ca"`
}

// issuerRef names the issuer that signs a Certificate.
type issuerRef struct {
	Name  string `yaml:"name"`
	Kind  string `yaml:"kind"`
	Group string `yaml:"group"`
}

// emptySpec marks an Issuer mode that carries no fields.
type emptySpec struct{}

// caSpec names the Secret holding the CA certificate an Issuer signs with.
type caSpec struct {
	SecretName string `yaml:"secretName"`
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

// volume is a pod volume and the ConfigMap or Secret it reads.
type volume struct {
	Name      string        `yaml:"name"`
	ConfigMap *configMapRef `yaml:"configMap"`
	Secret    *secretVolume `yaml:"secret"`
}

// configMapRef names the ConfigMap a volume reads.
type configMapRef struct {
	Name string `yaml:"name"`
}

// secretVolume names the Secret a volume reads.
type secretVolume struct {
	SecretName string `yaml:"secretName"`
}

// volumeMount attaches a volume at a container path.
type volumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readOnly"`
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

// TestChartVersionMatchesRepoVersion checks that the chart version and app
// version both equal the repo version, so the chart at a version runs the image
// at that version.
func TestChartVersionMatchesRepoVersion(t *testing.T) {
	var chart struct {
		Version    string `yaml:"version"`
		AppVersion string `yaml:"appVersion"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(chartDir(t), "Chart.yaml"))), &chart); err != nil {
		t.Fatalf("parse Chart.yaml: %v", err)
	}

	if chart.Version != version.Version {
		t.Errorf("chart version = %q, want the repo version %q", chart.Version, version.Version)
	}
	if chart.AppVersion != version.Version {
		t.Errorf("chart appVersion = %q, want the repo version %q", chart.AppVersion, version.Version)
	}
}

// TestChartImageDefaultsToRepoVersion checks that the chart's image values
// default to the service image at the repo version, so a caller installs the
// chart and runs the version its apps report.
func TestChartImageDefaultsToRepoVersion(t *testing.T) {
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

// TestServiceForwardsHTTPSPort checks that the Service exposes only port 443
// and forwards it to the container's named HTTPS port, so the chart serves no
// plaintext port.
func TestServiceForwardsHTTPSPort(t *testing.T) {
	svc, ok := findKind(renderChart(t), "Service")
	if !ok {
		t.Fatal("chart renders no Service")
	}

	ports := svc.Spec.Ports
	if len(ports) != 1 {
		t.Fatalf("Service forwards %d ports, want only HTTPS:\n%+v", len(ports), ports)
	}
	if got, want := ports[0].Port, 443; got != want {
		t.Errorf("Service port = %d, want %d", got, want)
	}
	if got, want := ports[0].TargetPort, "https"; got != want {
		t.Errorf("Service targetPort = %v, want the container port %q", got, want)
	}
}

// TestContainerExposesHTTPSPort checks that the container exposes only the
// HTTPS port, named https on 8443, and no plaintext port.
func TestContainerExposesHTTPSPort(t *testing.T) {
	dep, ok := findKind(renderChart(t), "Deployment")
	if !ok {
		t.Fatal("chart renders no Deployment")
	}

	ports := dep.Spec.Template.Spec.Containers[0].Ports
	if len(ports) != 1 {
		t.Fatalf("container exposes %d ports, want only HTTPS:\n%+v", len(ports), ports)
	}
	if got, want := ports[0].Name, "https"; got != want {
		t.Errorf("container port name = %q, want %q", got, want)
	}
	if got, want := ports[0].ContainerPort, 8443; got != want {
		t.Errorf("container port = %d, want %d", got, want)
	}
}

// TestChartRendersCertificateSignedByPrivateCA checks that the chart makes a
// service Certificate, the CA Certificate that signs it, and the self-signed and
// CA-backed Issuers that form the chain, so a default install issues a
// certificate from its own private CA.
func TestChartRendersCertificateSignedByPrivateCA(t *testing.T) {
	manifests := renderChart(t)

	certs, issuers := certificateAndIssuerManifests(manifests)
	caCert, ok := findManifest(certs, func(m manifest) bool { return m.Spec.IsCA })
	if !ok {
		t.Fatalf("chart renders no CA Certificate:\n%s", renderManifests(manifests))
	}
	serviceCert, ok := findManifest(certs, func(m manifest) bool { return !m.Spec.IsCA })
	if !ok {
		t.Fatalf("chart renders no service Certificate:\n%s", renderManifests(manifests))
	}
	selfSigned, ok := findManifest(issuers, func(m manifest) bool { return m.Spec.SelfSigned != nil })
	if !ok {
		t.Fatalf("chart renders no self-signed Issuer:\n%s", renderManifests(manifests))
	}
	caIssuer, ok := findManifest(issuers, func(m manifest) bool { return m.Spec.CA != nil })
	if !ok {
		t.Fatalf("chart renders no CA-backed Issuer:\n%s", renderManifests(manifests))
	}

	if got, want := caCert.Spec.IssuerRef.Name, selfSigned.Metadata.Name; got != want {
		t.Errorf("CA Certificate issuer = %q, want the self-signed Issuer %q", got, want)
	}
	if got, want := serviceCert.Spec.IssuerRef.Name, caIssuer.Metadata.Name; got != want {
		t.Errorf("service Certificate issuer = %q, want the CA-backed Issuer %q", got, want)
	}
	if got, want := caIssuer.Spec.CA.SecretName, caCert.Spec.SecretName; got != want {
		t.Errorf("CA Issuer reads Secret %q, want the CA Certificate Secret %q", got, want)
	}
	if serviceCert.Spec.SecretName == "" {
		t.Error("service Certificate writes no Secret")
	}
	if len(serviceCert.Spec.DNSNames) == 0 {
		t.Error("service Certificate covers no DNS name")
	}
}

// TestChartRequiresCertManager checks that the chart relies on cert-manager
// installed in the cluster: its default render includes Certificate and Issuer
// resources under the cert-manager API, and it carries no cert-manager subchart.
func TestChartRequiresCertManager(t *testing.T) {
	var chart struct {
		Dependencies []struct {
			Name string `yaml:"name"`
		} `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(chartDir(t), "Chart.yaml"))), &chart); err != nil {
		t.Fatalf("parse Chart.yaml: %v", err)
	}
	for _, dep := range chart.Dependencies {
		if dep.Name == "cert-manager" {
			t.Error("chart bundles cert-manager as a subchart, want the cluster to provide it")
		}
	}

	certs, issuers := certificateAndIssuerManifests(renderChart(t))
	if len(certs) == 0 {
		t.Error("default values render no Certificate")
	}
	if len(issuers) == 0 {
		t.Error("default values render no Issuer")
	}
	for _, m := range append(certs, issuers...) {
		if got, want := m.APIVersion, "cert-manager.io/v1"; got != want {
			t.Errorf("%s %s apiVersion = %q, want %q", m.Kind, m.Metadata.Name, got, want)
		}
	}
}

// TestChartUsesExistingIssuer checks that disabling the chart's issuer and naming
// an existing issuer makes the service Certificate reference it and renders no
// private CA resources.
func TestChartUsesExistingIssuer(t *testing.T) {
	manifests := renderChart(t,
		"--set", "tls.createIssuer=false",
		"--set", "tls.issuerRef.name=existing-issuer",
		"--set", "tls.issuerRef.kind=ClusterIssuer",
	)

	certs, issuers := certificateAndIssuerManifests(manifests)
	if len(issuers) != 0 {
		t.Errorf("chart renders %d Issuers with an existing issuer:\n%s", len(issuers), renderManifests(manifests))
	}
	if len(certs) != 1 {
		t.Fatalf("chart renders %d Certificates with an existing issuer, want the service Certificate only:\n%s",
			len(certs), renderManifests(manifests))
	}
	cert := certs[0]
	if cert.Spec.IsCA {
		t.Error("chart renders a CA Certificate with an existing issuer")
	}
	if got, want := cert.Spec.IssuerRef.Name, "existing-issuer"; got != want {
		t.Errorf("Certificate issuer = %q, want %q", got, want)
	}
	if got, want := cert.Spec.IssuerRef.Kind, "ClusterIssuer"; got != want {
		t.Errorf("Certificate issuer kind = %q, want %q", got, want)
	}
	if got, want := cert.Spec.IssuerRef.Group, "cert-manager.io"; got != want {
		t.Errorf("Certificate issuer group = %q, want %q", got, want)
	}
}

// TestChartRequiresIssuerNameWithExistingIssuer checks that disabling the chart's
// issuer without naming the existing issuer fails the render with an error that
// names the required issuer setting.
func TestChartRequiresIssuerNameWithExistingIssuer(t *testing.T) {
	out, err := renderChartOutput(t, "--set", "tls.createIssuer=false")
	if err == nil {
		t.Fatalf("helm template accepted a missing issuer name:\n%s", out)
	}
	if !strings.Contains(string(out), "tls.issuerRef.name") {
		t.Errorf("helm template error does not name tls.issuerRef.name:\n%s", out)
	}
}

// TestServiceCertificateCoversConfiguredDNSNames checks that the service
// Certificate covers the names in tls.dnsNames, so a deployment can serve the
// public host names it configures.
func TestServiceCertificateCoversConfiguredDNSNames(t *testing.T) {
	manifests := renderChart(t,
		"--set", "tls.dnsNames[0]=abnch.example.com",
		"--set", "tls.dnsNames[1]=api.example.com",
	)

	cert := serviceCertificate(t, manifests)
	want := []string{"abnch.example.com", "api.example.com"}
	if !slices.Equal(cert.Spec.DNSNames, want) {
		t.Errorf("service Certificate covers %v, want %v", cert.Spec.DNSNames, want)
	}
}

// TestServiceCertificateDefaultsToServiceClusterDNSName checks that an empty
// tls.dnsNames covers the Service's cluster DNS name, so a default install
// issues a certificate for the Service it exposes.
func TestServiceCertificateDefaultsToServiceClusterDNSName(t *testing.T) {
	const namespace = "abnch-test"
	manifests := renderChart(t, "--namespace", namespace)

	svc, ok := findKind(manifests, "Service")
	if !ok {
		t.Fatalf("chart renders no Service:\n%s", renderManifests(manifests))
	}
	cert := serviceCertificate(t, manifests)

	for _, want := range []string{
		fmt.Sprintf("%s.%s.svc", svc.Metadata.Name, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", svc.Metadata.Name, namespace),
	} {
		if !slices.Contains(cert.Spec.DNSNames, want) {
			t.Errorf("service Certificate covers %v, want it to include the Service's cluster DNS name %q",
				cert.Spec.DNSNames, want)
		}
	}
}

// TestServiceCertificateSecretIsMounted checks that the service Certificate
// writes the Secret the Deployment mounts, so the certificate files reach the
// service.
func TestServiceCertificateSecretIsMounted(t *testing.T) {
	manifests := renderChart(t)
	cert := serviceCertificate(t, manifests)

	dep, ok := findKind(manifests, "Deployment")
	if !ok {
		t.Fatalf("chart renders no Deployment:\n%s", renderManifests(manifests))
	}
	c := dep.Spec.Template.Spec.Containers[0]

	volumeName := ""
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Secret != nil && v.Secret.SecretName == cert.Spec.SecretName {
			volumeName = v.Name
		}
	}
	if volumeName == "" {
		t.Fatalf("Deployment mounts no Secret %q written by the service Certificate:\n%+v",
			cert.Spec.SecretName, dep.Spec.Template.Spec.Volumes)
	}

	for _, m := range c.VolumeMounts {
		if m.Name != volumeName {
			continue
		}
		if m.MountPath != "/etc/abnchd/tls" {
			t.Errorf("certificate Secret mounts at %q, want /etc/abnchd/tls", m.MountPath)
		}
		if !m.ReadOnly {
			t.Error("certificate Secret mounts read-write, want read-only")
		}
		return
	}
	t.Errorf("container mounts no volume %q for the certificate Secret:\n%+v", volumeName, c.VolumeMounts)
}

// serviceCertificate returns the rendered Certificate for the service, which is
// the Certificate that is not a CA.
func serviceCertificate(t *testing.T, manifests []manifest) manifest {
	t.Helper()
	certs, _ := certificateAndIssuerManifests(manifests)
	cert, ok := findManifest(certs, func(m manifest) bool { return !m.Spec.IsCA })
	if !ok {
		t.Fatalf("chart renders no service Certificate:\n%s", renderManifests(manifests))
	}
	return cert
}

// certificateAndIssuerManifests returns the Certificates and Issuers the chart
// renders.
func certificateAndIssuerManifests(manifests []manifest) ([]manifest, []manifest) {
	var certs, issuers []manifest
	for _, m := range manifests {
		switch m.Kind {
		case "Certificate":
			certs = append(certs, m)
		case "Issuer":
			issuers = append(issuers, m)
		}
	}
	return certs, issuers
}

// findManifest returns the first manifest matching pred.
func findManifest(manifests []manifest, pred func(manifest) bool) (manifest, bool) {
	for _, m := range manifests {
		if pred(m) {
			return m, true
		}
	}
	return manifest{}, false
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

// TestChartConfiguresHTTPS checks that the rendered config serves HTTPS with
// the certificate files the Deployment mounts, so a default install serves the
// service over TLS.
func TestChartConfiguresHTTPS(t *testing.T) {
	manifests := renderChart(t)
	cfg := renderedConfig(t, manifests)

	if !strings.HasPrefix(cfg.URL, "https://") {
		t.Errorf("url = %q, want an https url", cfg.URL)
	}
	if got, want := cfg.Listen, "0.0.0.0:8443"; got != want {
		t.Errorf("listen = %q, want %q", got, want)
	}
	if cfg.TLS == nil {
		t.Fatalf("config has no tls block:\n%s", renderManifests(manifests))
	}
	if got, want := cfg.TLS.CertFile, "/etc/abnchd/tls/tls.crt"; got != want {
		t.Errorf("tls.certFile = %q, want %q", got, want)
	}
	if got, want := cfg.TLS.KeyFile, "/etc/abnchd/tls/tls.key"; got != want {
		t.Errorf("tls.keyFile = %q, want %q", got, want)
	}

	dep, ok := findKind(manifests, "Deployment")
	if !ok {
		t.Fatalf("chart renders no Deployment:\n%s", renderManifests(manifests))
	}
	c := dep.Spec.Template.Spec.Containers[0]
	for _, file := range []string{cfg.TLS.CertFile, cfg.TLS.KeyFile} {
		if !mountedAt(c.VolumeMounts, path.Dir(file), "/etc/abnchd/tls") {
			t.Errorf("tls file %q is not under the certificate mount:\n%+v", file, c.VolumeMounts)
		}
	}
}

// mountedAt reports whether a volume mount covers dir and sits at wantPath.
func mountedAt(mounts []volumeMount, dir, wantPath string) bool {
	for _, m := range mounts {
		if m.MountPath == wantPath && (dir == m.MountPath || strings.HasPrefix(dir, m.MountPath+"/")) {
			return true
		}
	}
	return false
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
	if strings.Contains(string(out), "[ERROR]") {
		t.Errorf("helm lint reported an error:\n%s", out)
	}
}

// dns1123Subdomain is the name form Kubernetes accepts for object metadata.
var dns1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// TestChartRendersValidManifests checks that the chart renders valid Kubernetes
// manifests with its default values: every document names a kind and API version
// the cluster serves, carries a DNS-1123 name, and holds the fields its kind
// requires.
func TestChartRendersValidManifests(t *testing.T) {
	manifests := renderChart(t)
	if len(manifests) == 0 {
		t.Fatal("chart renders no manifests with its default values")
	}

	kinds := map[string]string{
		"Deployment":  "apps/v1",
		"Service":     "v1",
		"ConfigMap":   "v1",
		"Certificate": "cert-manager.io/v1",
		"Issuer":      "cert-manager.io/v1",
	}
	seen := map[string]bool{}
	for _, m := range manifests {
		apiVersion, known := kinds[m.Kind]
		if !known {
			t.Errorf("chart renders unknown kind %q", m.Kind)
			continue
		}
		seen[m.Kind] = true

		if m.APIVersion != apiVersion {
			t.Errorf("%s apiVersion = %q, want %q", m.Kind, m.APIVersion, apiVersion)
		}
		if !dns1123Subdomain.MatchString(m.Metadata.Name) {
			t.Errorf("%s name = %q, want a DNS-1123 subdomain", m.Kind, m.Metadata.Name)
		}

		switch m.Kind {
		case "Deployment":
			checkValidDeployment(t, m)
		case "Service":
			checkValidService(t, m)
		case "ConfigMap":
			if len(m.Data) == 0 {
				t.Errorf("ConfigMap %s holds no data", m.Metadata.Name)
			}
		case "Certificate":
			checkValidCertificate(t, m)
		case "Issuer":
			checkValidIssuer(t, m)
		}
	}

	for kind := range kinds {
		if !seen[kind] {
			t.Errorf("chart renders no %s with its default values", kind)
		}
	}
}

// checkValidDeployment checks the fields a Deployment requires to schedule pods.
func checkValidDeployment(t *testing.T, m manifest) {
	t.Helper()
	if len(m.Spec.Selector) == 0 {
		t.Errorf("Deployment %s selects no pods", m.Metadata.Name)
	}
	if m.Spec.Template == nil {
		t.Errorf("Deployment %s has no pod template", m.Metadata.Name)
		return
	}
	if len(m.Spec.Template.Spec.Containers) == 0 {
		t.Errorf("Deployment %s runs no containers", m.Metadata.Name)
		return
	}
	for _, c := range m.Spec.Template.Spec.Containers {
		if c.Name == "" || c.Image == "" {
			t.Errorf("Deployment %s container = %+v, want a name and image", m.Metadata.Name, c)
		}
	}
}

// checkValidService checks the fields a Service requires to reach pods.
func checkValidService(t *testing.T, m manifest) {
	t.Helper()
	if len(m.Spec.Selector) == 0 {
		t.Errorf("Service %s selects no pods", m.Metadata.Name)
	}
	if len(m.Spec.Ports) == 0 {
		t.Errorf("Service %s forwards no ports", m.Metadata.Name)
		return
	}
	for _, p := range m.Spec.Ports {
		if p.Port < 1 || p.Port > 65535 {
			t.Errorf("Service %s port = %d, want 1-65535", m.Metadata.Name, p.Port)
		}
	}
}

// checkValidCertificate checks the fields a Certificate requires to issue.
func checkValidCertificate(t *testing.T, m manifest) {
	t.Helper()
	if m.Spec.SecretName == "" {
		t.Errorf("Certificate %s writes no Secret", m.Metadata.Name)
	}
	if m.Spec.IssuerRef.Name == "" {
		t.Errorf("Certificate %s names no issuer", m.Metadata.Name)
	}
	if !m.Spec.IsCA && len(m.Spec.DNSNames) == 0 {
		t.Errorf("Certificate %s covers no DNS name", m.Metadata.Name)
	}
}

// checkValidIssuer checks that an Issuer names the mode it signs with.
func checkValidIssuer(t *testing.T, m manifest) {
	t.Helper()
	if m.Spec.SelfSigned == nil && m.Spec.CA == nil {
		t.Errorf("Issuer %s names neither a self-signed nor a CA signer", m.Metadata.Name)
	}
	if m.Spec.CA != nil && m.Spec.CA.SecretName == "" {
		t.Errorf("Issuer %s names no CA Secret", m.Metadata.Name)
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
	out, err := renderChartOutput(t, args...)
	if err != nil {
		t.Fatalf("helm template: %v\n%s", err, out)
	}
	return parseManifests(t, out)
}

// renderChartOutput runs `helm template` with the given arguments and returns
// its output, skipping when helm is not installed.
func renderChartOutput(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm is not installed")
	}

	cmd := exec.Command(helm, append([]string{"template", "abnchd", chartDir(t)}, args...)...)
	return cmd.CombinedOutput()
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
