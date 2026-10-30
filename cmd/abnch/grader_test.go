package main

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGetGraderPrintsGraderAsYAML(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"url: https://bench.example.com\n"+
			"tasks:\n  fix-bug:\n    grader:\n      name: llm-judge\n      stub: judge\n      commit: def456\n      branch: main\n      version: 2.0.0\n      uncommitted: true\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"grader", "fix-bug"}, d); err != nil {
		t.Fatalf("get grader = %v, want no error", err)
	}

	var got configGrader
	if err := yaml.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if got.Name != "llm-judge" || got.Stub != "judge" || got.Commit != "def456" ||
		got.Branch != "main" || got.Version != "2.0.0" || !got.Uncommitted {
		t.Errorf("grader = %+v, want the stored record", got)
	}
}

func TestGetGraderOmitsAbsentVersion(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    grader:\n      name: llm-judge\n      stub: judge\n      commit: def456\n      branch: main\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"grader", "fix-bug"}, d); err != nil {
		t.Fatalf("get grader = %v, want no error", err)
	}
	if strings.Contains(out.String(), "version:") {
		t.Errorf("output = %q, want an absent version omitted", out.String())
	}
}

func TestGetGraderFailsWhenTaskHasNoGrader(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    model: sonnet\n")
	d, out := getConfigDeps(t, workDir)

	err := runGet([]string{"grader", "fix-bug"}, d)
	if err == nil {
		t.Fatal("get grader = nil, want an error when the task has no grader")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing grader", out.String())
	}
}

func TestGetGraderReportsMissingGraderOnStderr(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"get", "grader", "fix-bug"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing grader")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing grader", out)
	}
	if !strings.Contains(errOut, "fix-bug") {
		t.Errorf("stderr = %q, want the task named", errOut)
	}
}

func TestGetGraderFailsWithoutConfig(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	if err := runGet([]string{"grader", "fix-bug"}, d); err == nil {
		t.Fatal("get grader = nil, want an error without a config")
	}
}
