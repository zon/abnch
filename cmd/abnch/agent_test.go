package main

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGetAgentPrintsAgentAsYAML(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"url: https://bench.example.com\n"+
			"tasks:\n  fix-bug:\n    agent:\n      name: claude-code\n      stub: claude\n      commit: abc123\n      branch: main\n      version: 1.2.3\n      uncommitted: true\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"agent", "fix-bug"}, d); err != nil {
		t.Fatalf("get agent = %v, want no error", err)
	}

	var got configAgent
	if err := yaml.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if got.Name != "claude-code" || got.Stub != "claude" || got.Commit != "abc123" ||
		got.Branch != "main" || got.Version != "1.2.3" || !got.Uncommitted {
		t.Errorf("agent = %+v, want the stored record", got)
	}
}

func TestGetAgentOmitsAbsentVersion(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    agent:\n      name: claude-code\n      stub: claude\n      commit: abc123\n      branch: main\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"agent", "fix-bug"}, d); err != nil {
		t.Fatalf("get agent = %v, want no error", err)
	}
	if strings.Contains(out.String(), "version:") {
		t.Errorf("output = %q, want an absent version omitted", out.String())
	}
}

func TestGetAgentFailsWhenTaskHasNoAgent(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    model: sonnet\n")
	d, out := getConfigDeps(t, workDir)

	err := runGet([]string{"agent", "fix-bug"}, d)
	if err == nil {
		t.Fatal("get agent = nil, want an error when the task has no agent")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing agent", out.String())
	}
}

func TestGetAgentReportsMissingAgentOnStderr(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"get", "agent", "fix-bug"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing agent")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing agent", out)
	}
	if !strings.Contains(errOut, "fix-bug") {
		t.Errorf("stderr = %q, want the task named", errOut)
	}
}

func TestGetAgentFailsWithoutConfig(t *testing.T) {
	workDir := t.TempDir()
	d, _ := getConfigDeps(t, workDir)

	if err := runGet([]string{"agent", "fix-bug"}, d); err == nil {
		t.Fatal("get agent = nil, want an error without a config")
	}
}
