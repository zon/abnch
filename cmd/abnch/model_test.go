package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// configModelFile is a model entry in the CLI config as a test reads it.
type configModelFile struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
	Variant  string `yaml:"variant"`
}

// readConfigModels reads the models in the CLI config under workDir.
func readConfigModels(t *testing.T, workDir string) map[string]configModelFile {
	t.Helper()
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Models map[string]configModelFile `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg.Models
}

// setModelDeps returns deps for a set model command run in an isolated home.
func setModelDeps(t *testing.T, workDir string) deps {
	t.Helper()
	return deps{
		homeDir: t.TempDir(),
		workDir: workDir,
		in:      strings.NewReader(""),
		out:     io.Discard,
		ctx:     context.Background(),
	}
}

// runSetModel runs `abnch set model` with the given arguments.
func runSetModel(args []string, d deps) error {
	return runCLI(append([]string{"set", "model"}, args...), d, func(int) {})
}

func TestSetModelRecordsProviderNameAndVariant(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet", "--variant", "high"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	models := readConfigModels(t, workDir)
	got, ok := models["claude-sonnet"]
	if !ok {
		t.Fatalf("models = %v, want the model under its derived stub", models)
	}
	if got.Provider != "anthropic" || got.Name != "claude-sonnet" || got.Variant != "high" {
		t.Errorf("model = %+v, want provider anthropic, name claude-sonnet, variant high", got)
	}
}

func TestSetModelUsesGivenStub(t *testing.T) {
	workDir := t.TempDir()
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet", "--stub", "sonnet"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	models := readConfigModels(t, workDir)
	if _, ok := models["sonnet"]; !ok {
		t.Fatalf("models = %v, want the model under sonnet", models)
	}
	if _, ok := models["claude-sonnet"]; ok {
		t.Errorf("models = %v, want no model under the derived stub", models)
	}
}

func TestSetModelUpdatesExistingModel(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n")
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet", "--stub", "sonnet", "--variant", "low"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	got := readConfigModels(t, workDir)["sonnet"]
	if got.Variant != "low" {
		t.Errorf("variant = %q, want low", got.Variant)
	}
	if got.Provider != "anthropic" || got.Name != "claude-sonnet" {
		t.Errorf("model = %+v, want name and provider unchanged", got)
	}
}

func TestSetModelKeepsVariantWhenNotGiven(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n")
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet", "--stub", "sonnet"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	got := readConfigModels(t, workDir)["sonnet"]
	if got.Variant != "high" {
		t.Errorf("variant = %q, want the existing variant kept", got.Variant)
	}
}

func TestSetModelOmitsVariantWhenNotGiven(t *testing.T) {
	workDir := t.TempDir()
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "variant") {
		t.Errorf("config = %q, want no variant key", data)
	}
}

func TestSetModelFailsWithoutProvider(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setModelDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"set", "model", "claude-sonnet"}, d)

	if code == 0 {
		t.Error("exit = 0, want a non-zero exit without a provider")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing provider", out)
	}
	if !strings.Contains(errOut, "provider") {
		t.Errorf("stderr = %q, want the missing provider reported", errOut)
	}
	if models := readConfigModels(t, workDir); len(models) != 0 {
		t.Errorf("models = %v, want no model recorded", models)
	}
}

func TestSetModelFailsOnEmptyProvider(t *testing.T) {
	workDir := t.TempDir()
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"/claude-sonnet"}, d); err == nil {
		t.Fatal("set model = nil, want an error for an empty provider")
	}
	if _, err := os.Stat(configPath(workDir)); !os.IsNotExist(err) {
		t.Errorf("config was written for an invalid argument: stat = %v", err)
	}
}

func TestSetModelFailsWithoutName(t *testing.T) {
	workDir := t.TempDir()
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/"}, d); err == nil {
		t.Fatal("set model = nil, want an error for a missing name")
	}
}

func TestSetModelCreatesConfig(t *testing.T) {
	workDir := t.TempDir()
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	got := readConfigModels(t, workDir)["claude-sonnet"]
	if got.Provider != "anthropic" || got.Name != "claude-sonnet" {
		t.Errorf("model = %+v, want the recorded model", got)
	}
}

func TestSetModelKeepsOtherConfig(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n"+
		"models:\n  haiku:\n    provider: anthropic\n    name: claude-haiku\n"+
		"tasks:\n  fix-bug:\n    model: haiku\n")
	d := setModelDeps(t, workDir)

	if err := runSetModel([]string{"anthropic/claude-sonnet", "--stub", "sonnet"}, d); err != nil {
		t.Fatalf("set model = %v, want no error", err)
	}

	models := readConfigModels(t, workDir)
	if got := models["haiku"]; got.Name != "claude-haiku" {
		t.Errorf("haiku = %+v, want the existing model kept", got)
	}
	if got := models["sonnet"]; got.Name != "claude-sonnet" {
		t.Errorf("sonnet = %+v, want the new model", got)
	}
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fix-bug") {
		t.Errorf("config = %q, want the tasks section kept", data)
	}
}
