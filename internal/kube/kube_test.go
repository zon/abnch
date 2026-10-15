package kube

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestConfigMapManifest(t *testing.T) {
	manifest, err := configMapManifest("ralph-abnch", "abnch-config", map[string]string{
		"config.yaml": "url: http://localhost:8080\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		APIVersion string            `yaml:"apiVersion"`
		Kind       string            `yaml:"kind"`
		Metadata   map[string]string `yaml:"metadata"`
		Data       map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(manifest, &doc); err != nil {
		t.Fatal(err)
	}

	if doc.APIVersion != "v1" || doc.Kind != "ConfigMap" {
		t.Errorf("got %s %s, want v1 ConfigMap", doc.APIVersion, doc.Kind)
	}
	if doc.Metadata["name"] != "abnch-config" || doc.Metadata["namespace"] != "ralph-abnch" {
		t.Errorf("metadata = %#v, want abnch-config in ralph-abnch", doc.Metadata)
	}
	if doc.Data["config.yaml"] != "url: http://localhost:8080\n" {
		t.Errorf("data = %#v, want the config under its file name", doc.Data)
	}
}

func TestSecretManifestBase64EncodesData(t *testing.T) {
	manifest, err := secretManifest("ralph-abnch", "abnchd-config", map[string]string{
		"dev-issuer.key": "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Kind string            `yaml:"kind"`
		Type string            `yaml:"type"`
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(manifest, &doc); err != nil {
		t.Fatal(err)
	}

	if doc.Kind != "Secret" || doc.Type != "Opaque" {
		t.Errorf("got %s %s, want an Opaque Secret", doc.Kind, doc.Type)
	}
	raw, err := base64.StdEncoding.DecodeString(doc.Data["dev-issuer.key"])
	if err != nil {
		t.Fatalf("data is not base64: %v", err)
	}
	if string(raw) != "secret" {
		t.Errorf("decoded data = %q, want secret", raw)
	}
}

// TestApplyPipesManifestToKubectl replaces kubectl with a script that records
// its stdin and arguments, so the test observes what the client sends.
func TestApplyPipesManifestToKubectl(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "stdin")
	argsFile := filepath.Join(dir, "args")
	writeFakeKubectl(t, dir)

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("KUBE_CAPTURE", capture)
	t.Setenv("KUBE_ARGS", argsFile)

	if err := (Client{}).ApplyConfigMap("lab", "ralph-abnch", "abnch-config", map[string]string{"config.yaml": "x"}); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, capture); !strings.Contains(got, "kind: ConfigMap") {
		t.Errorf("stdin = %q, want the rendered ConfigMap", got)
	}
	args := readFile(t, argsFile)
	for _, want := range []string{"apply", "-f", "-", "--context", "lab"} {
		if !strings.Contains(args, want) {
			t.Errorf("args = %q, want %q", args, want)
		}
	}
}

func TestApplyReportsMissingKubectl(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := apply("", []byte("kind: ConfigMap\n"))
	if err == nil || !strings.Contains(err.Error(), "kubectl") {
		t.Fatalf("err = %v, want a missing kubectl error", err)
	}
}

// writeFakeKubectl writes a kubectl script that copies its stdin to the file
// named by KUBE_CAPTURE and its arguments to the file named by KUBE_ARGS.
func writeFakeKubectl(t *testing.T, dir string) {
	t.Helper()
	script := "#!/bin/sh\ncat > \"$KUBE_CAPTURE\"\nprintf '%s\\n' \"$@\" > \"$KUBE_ARGS\"\n"
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
