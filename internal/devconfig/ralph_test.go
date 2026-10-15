package devconfig

import (
	"strings"
	"testing"
)

const (
	ralphSeed          = "mode: remote\nworkflow:\n  context: lab\n  namespace: ralph-abnch\n"
	ralphServiceConfig = "url: http://localhost:8080\npostgres:\n  password: s3cret\n"
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
	if secret.data[ConfigFileName] != ralphServiceConfig {
		t.Errorf("secret %s = %q, want the local service config", ConfigFileName, secret.data[ConfigFileName])
	}
	if secret.data[KeyFileName] != ralphDevKey {
		t.Errorf("secret %s = %q, want the dev key", KeyFileName, secret.data[KeyFileName])
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
