package devconfig

import (
	"os"
	"strings"
	"testing"
)

const (
	ralphSeed          = "mode: remote\nworkflow:\n  context: lab\n  namespace: ralph-abnch\n"
	ralphServiceConfig = "url: http://localhost:8080\npostgres:\n  host: avocado\n  port: 30543\n  password: s3cret\n"
	ralphDevKey        = "-----BEGIN PRIVATE KEY-----\nkey\n"
	ralphCLIConfig     = "url: http://localhost:8080\n"
)

func TestSetRalphPublishesLocalFiles(t *testing.T) {
	repo := t.TempDir()
	seedRalphRepo(t, repo)
	applier := &fakeApplier{}

	namespace, err := SetRalph(RalphOptions{RepoRoot: repo, Applier: applier})
	if err != nil {
		t.Fatal(err)
	}
	if namespace != "ralph-abnch" {
		t.Errorf("namespace = %q, want the configured ralph-abnch", namespace)
	}

	if len(applier.secrets) != 1 {
		t.Fatalf("applied %d secrets, want 1", len(applier.secrets))
	}
	secret := applier.secrets[0]
	if secret.name != RalphSecretName {
		t.Errorf("secret name = %q, want %q", secret.name, RalphSecretName)
	}
	if secret.namespace != "ralph-abnch" || secret.context != "lab" {
		t.Errorf("secret target = %q/%q, want lab/ralph-abnch", secret.context, secret.namespace)
	}
	published, err := UnmarshalConfig([]byte(secret.data[ConfigFileName]))
	if err != nil {
		t.Fatal(err)
	}
	if published.Postgres.Host != RalphPostgresHost || published.Postgres.Port != RalphPostgresPort {
		t.Errorf("published postgres = %s:%d, want %s:%d",
			published.Postgres.Host, published.Postgres.Port, RalphPostgresHost, RalphPostgresPort)
	}
	if published.Postgres.Password != "s3cret" {
		t.Errorf("published postgres password = %q, want it kept", published.Postgres.Password)
	}
	if secret.data[KeyFileName] != ralphDevKey {
		t.Errorf("secret %s = %q, want the dev key", KeyFileName, secret.data[KeyFileName])
	}

	local, err := os.ReadFile(ConfigPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if string(local) != ralphServiceConfig {
		t.Errorf("local config = %q, want it unchanged", local)
	}

	if len(applier.configMaps) != 1 {
		t.Fatalf("applied %d configmaps, want 1", len(applier.configMaps))
	}
	configMap := applier.configMaps[0]
	if configMap.name != RalphConfigMapName {
		t.Errorf("configmap name = %q, want %q", configMap.name, RalphConfigMapName)
	}
	if configMap.data[CLIConfigFileName] != ralphCLIConfig {
		t.Errorf("configmap %s = %q, want the CLI config", CLIConfigFileName, configMap.data[CLIConfigFileName])
	}
}

func TestSetRalphFlagsOverrideTarget(t *testing.T) {
	repo := t.TempDir()
	seedRalphRepo(t, repo)
	applier := &fakeApplier{}

	namespace, err := SetRalph(RalphOptions{
		RepoRoot:  repo,
		Context:   "canary",
		Namespace: "staging",
		Applier:   applier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if namespace != "staging" {
		t.Errorf("namespace = %q, want the staging flag", namespace)
	}
	if len(applier.secrets) != 1 || applier.secrets[0].context != "canary" || applier.secrets[0].namespace != "staging" {
		t.Errorf("secret target = %#v, want canary/staging", applier.secrets)
	}
}

func TestSetRalphPostgresFlagsOverride(t *testing.T) {
	repo := t.TempDir()
	seedRalphRepo(t, repo)
	applier := &fakeApplier{}

	_, err := SetRalph(RalphOptions{
		RepoRoot:     repo,
		PostgresHost: "db.internal",
		PostgresPort: 5433,
		Applier:      applier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(applier.secrets) != 1 {
		t.Fatalf("applied %d secrets, want 1", len(applier.secrets))
	}
	published, err := UnmarshalConfig([]byte(applier.secrets[0].data[ConfigFileName]))
	if err != nil {
		t.Fatal(err)
	}
	if published.Postgres.Host != "db.internal" || published.Postgres.Port != 5433 {
		t.Errorf("published postgres = %s:%d, want db.internal:5433",
			published.Postgres.Host, published.Postgres.Port)
	}
}

func TestSetRalphRequiresNamespace(t *testing.T) {
	repo := t.TempDir()
	seedRalphRepo(t, repo)
	// Drop the configured namespace so only a flag could supply one.
	writeRalphConfig(t, repo, "mode: remote\nworkflow:\n  context: lab\n")

	_, err := SetRalph(RalphOptions{RepoRoot: repo, Applier: &fakeApplier{}})
	if err == nil {
		t.Fatal("SetRalph = nil, want an error for a missing namespace")
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("error = %q, want it to name the missing namespace", err)
	}
}

func TestSetRalphFailsWithoutRalphConfig(t *testing.T) {
	repo := t.TempDir()
	if err := writeConfigFile(ConfigPath(repo), ralphServiceConfig); err != nil {
		t.Fatal(err)
	}

	_, err := SetRalph(RalphOptions{RepoRoot: repo, Applier: &fakeApplier{}})
	if err == nil {
		t.Fatal("SetRalph = nil, want an error for a missing .ralph/config.yaml")
	}
	if !strings.Contains(err.Error(), RalphConfigFileName) {
		t.Errorf("error = %q, want it to name the missing config", err)
	}
}

// fakeApplier records what SetRalph publishes.
type fakeApplier struct {
	secrets    []applied
	configMaps []applied
}

type applied struct {
	context   string
	namespace string
	name      string
	data      map[string]string
}

func (f *fakeApplier) ApplySecret(context, namespace, name string, data map[string]string) error {
	f.secrets = append(f.secrets, applied{context, namespace, name, data})
	return nil
}

func (f *fakeApplier) ApplyConfigMap(context, namespace, name string, data map[string]string) error {
	f.configMaps = append(f.configMaps, applied{context, namespace, name, data})
	return nil
}

// seedRalphRepo writes the local files the command publishes and a Ralph config
// that targets the ralph-abnch namespace.
func seedRalphRepo(t *testing.T, repo string) {
	t.Helper()
	writeRalphFiles(t, repo)
	writeRalphConfig(t, repo, ralphSeed)
}

func writeRalphFiles(t *testing.T, repo string) {
	t.Helper()
	files := map[string]string{
		ConfigPath(repo):          ralphServiceConfig,
		KeyPath(ConfigPath(repo)): ralphDevKey,
		CLIConfigPath(repo):       ralphCLIConfig,
	}
	for path, content := range files {
		if err := writeConfigFile(path, content); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRalphConfig(t *testing.T, repo, content string) {
	t.Helper()
	if err := writeConfigFile(RalphConfigPath(repo), content); err != nil {
		t.Fatal(err)
	}
}
