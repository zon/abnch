package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// getConfigDeps returns deps for a get command that reads the CLI config with
// a capturing stdout. It stores no session and sets no service URL, so a
// command that reaches the service fails.
func getConfigDeps(t *testing.T, workDir string) (deps, *bytes.Buffer) {
	t.Helper()
	out := new(bytes.Buffer)
	return deps{
		homeDir: t.TempDir(),
		workDir: workDir,
		in:      strings.NewReader(""),
		out:     out,
		ctx:     context.Background(),
	}, out
}

// readModelViews parses the models a get command wrote as YAML.
func readModelViews(t *testing.T, out *bytes.Buffer) []modelView {
	t.Helper()
	var models []modelView
	if err := yaml.Unmarshal(out.Bytes(), &models); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	return models
}

func TestGetModelsListsModelsAsYAML(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n"+
			"  haiku:\n    provider: anthropic\n    name: claude-haiku\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"models"}, d); err != nil {
		t.Fatalf("get models = %v, want no error", err)
	}

	models := readModelViews(t, out)
	if len(models) != 2 {
		t.Fatalf("listed %d models, want 2:\n%s", len(models), out.String())
	}
	byStub := map[string]modelView{}
	for _, m := range models {
		byStub[m.Stub] = m
	}
	if got := byStub["sonnet"]; got.Provider != "anthropic" || got.Name != "claude-sonnet" || got.Variant != "high" {
		t.Errorf("sonnet = %+v, want the stored model", got)
	}
	if got := byStub["haiku"]; got.Provider != "anthropic" || got.Name != "claude-haiku" {
		t.Errorf("haiku = %+v, want the stored model", got)
	}
}

func TestGetModelsWithoutConfigIsEmpty(t *testing.T) {
	workDir := t.TempDir()
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"models"}, d); err != nil {
		t.Fatalf("get models = %v, want no error without a config", err)
	}
	if out.String() != "" {
		t.Errorf("output = %q, want nothing without a config", out.String())
	}
	if models := readModelViews(t, out); len(models) != 0 {
		t.Errorf("listed %d models, want none", len(models))
	}
}

func TestGetModelsNeedsNoRepoOrService(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	if err := runGet([]string{"models"}, d); err != nil {
		t.Fatalf("get models = %v, want no error without a repo or service", err)
	}
}

func TestGetModelPrintsModelAsYAML(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"url: https://bench.example.com\n"+
			"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"model", "sonnet"}, d); err != nil {
		t.Fatalf("get model = %v, want no error", err)
	}

	var got modelView
	if err := yaml.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if got.Stub != "sonnet" || got.Provider != "anthropic" || got.Name != "claude-sonnet" || got.Variant != "high" {
		t.Errorf("model = %+v, want the stored model with its stub", got)
	}
}

func TestGetModelOmitsVariantWhenAbsent(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"model", "sonnet"}, d); err != nil {
		t.Fatalf("get model = %v, want no error", err)
	}
	if strings.Contains(out.String(), "variant") {
		t.Errorf("output = %q, want no variant key", out.String())
	}
}

func TestGetModelFailsWhenModelMissing(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "models:\n  haiku:\n    provider: anthropic\n    name: claude-haiku\n")
	d, out := getConfigDeps(t, workDir)

	err := runGet([]string{"model", "sonnet"}, d)
	if err == nil {
		t.Fatal("get model = nil, want an error for a missing model")
	}
	if !strings.Contains(err.Error(), "sonnet") {
		t.Errorf("error = %q, want it to name the missing model", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing model", out.String())
	}
}

func TestGetModelReportsMissingModelOnStderr(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"get", "model", "sonnet"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing model")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing model", out)
	}
	if !strings.Contains(errOut, "sonnet") {
		t.Errorf("stderr = %q, want the missing model named", errOut)
	}
}

func TestGetModelFailsWithoutConfig(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	if err := runGet([]string{"model", "sonnet"}, d); err == nil {
		t.Fatal("get model = nil, want an error without a config")
	}
}
