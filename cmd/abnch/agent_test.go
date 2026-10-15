package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// configAgentFile is an agent entry in the CLI config as a test reads it.
type configAgentFile struct {
	VersionCmd string `yaml:"versionCmd"`
	BuildCmd   string `yaml:"buildCmd"`
	RunCmd     string `yaml:"runCmd"`
}

// readConfigAgent reads the agent recorded for the task under workDir and
// whether it exists.
func readConfigAgent(t *testing.T, workDir, stub string) (configAgentFile, bool) {
	t.Helper()
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Tasks map[string]struct {
			Agent *configAgentFile `yaml:"agent"`
		} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	agent := cfg.Tasks[stub].Agent
	if agent == nil {
		return configAgentFile{}, false
	}
	return *agent, true
}

// setAgentDeps returns deps for a set agent command run in an isolated home.
func setAgentDeps(t *testing.T, workDir string) deps {
	t.Helper()
	return deps{
		homeDir: t.TempDir(),
		workDir: workDir,
		in:      strings.NewReader(""),
		out:     io.Discard,
		ctx:     context.Background(),
	}
}

// runSetAgent runs `abnch set agent` with the given arguments.
func runSetAgent(args []string, d deps) error {
	return runCLI(append([]string{"set", "agent"}, args...), d, func(int) {})
}

func TestSetAgentRecordsCommands(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setAgentDeps(t, workDir)

	if err := runSetAgent([]string{
		"fix-bug", "--version-cmd", "./version.sh", "--build-cmd", "./build.sh", "--run-cmd", "./run.sh",
	}, d); err != nil {
		t.Fatalf("set agent = %v, want no error", err)
	}

	got, ok := readConfigAgent(t, workDir, "fix-bug")
	if !ok {
		t.Fatal("agent was not recorded")
	}
	if got.VersionCmd != "./version.sh" || got.BuildCmd != "./build.sh" || got.RunCmd != "./run.sh" {
		t.Errorf("agent = %+v, want all three commands", got)
	}
}

func TestSetAgentRecordsRunCommandAlone(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setAgentDeps(t, workDir)

	if err := runSetAgent([]string{"fix-bug", "--run-cmd", "./run.sh"}, d); err != nil {
		t.Fatalf("set agent = %v, want no error", err)
	}

	got, ok := readConfigAgent(t, workDir, "fix-bug")
	if !ok {
		t.Fatal("agent was not recorded")
	}
	if got.RunCmd != "./run.sh" {
		t.Errorf("run command = %q, want ./run.sh", got.RunCmd)
	}
}

func TestSetAgentMergesExistingAgent(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    agent:\n      runCmd: ./run.sh\n      buildCmd: ./build.sh\n")
	d := setAgentDeps(t, workDir)

	if err := runSetAgent([]string{"fix-bug", "--version-cmd", "./version.sh"}, d); err != nil {
		t.Fatalf("set agent = %v, want no error", err)
	}

	got, _ := readConfigAgent(t, workDir, "fix-bug")
	if got.VersionCmd != "./version.sh" {
		t.Errorf("version command = %q, want the new one", got.VersionCmd)
	}
	if got.RunCmd != "./run.sh" {
		t.Errorf("run command = %q, want the existing one kept", got.RunCmd)
	}
	if got.BuildCmd != "./build.sh" {
		t.Errorf("build command = %q, want the existing one kept", got.BuildCmd)
	}
}

func TestSetAgentFailsWithoutRunCommand(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setAgentDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"set", "agent", "fix-bug", "--build-cmd", "./build.sh"}, d)

	if code == 0 {
		t.Error("exit = 0, want a non-zero exit without a run command")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing when the run command is missing", out)
	}
	if !strings.Contains(errOut, "run command") {
		t.Errorf("stderr = %q, want the missing run command reported", errOut)
	}
	if _, ok := readConfigAgent(t, workDir, "fix-bug"); ok {
		t.Error("an agent was recorded without a run command")
	}
}

func TestSetAgentFailsWithoutRunCommandOnUpdate(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    model: sonnet\n")
	d := setAgentDeps(t, workDir)

	if err := runSetAgent([]string{"fix-bug", "--version-cmd", "./version.sh"}, d); err == nil {
		t.Fatal("set agent = nil, want an error when the agent has no run command")
	}
	if _, ok := readConfigAgent(t, workDir, "fix-bug"); ok {
		t.Error("an agent was recorded without a run command")
	}
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sonnet") {
		t.Errorf("config = %q, want the existing task kept", data)
	}
}

func TestSetAgentCreatesConfig(t *testing.T) {
	workDir := t.TempDir()
	d := setAgentDeps(t, workDir)

	if err := runSetAgent([]string{"fix-bug", "--run-cmd", "./run.sh"}, d); err != nil {
		t.Fatalf("set agent = %v, want no error", err)
	}

	got, ok := readConfigAgent(t, workDir, "fix-bug")
	if !ok {
		t.Fatal("agent was not recorded")
	}
	if got.RunCmd != "./run.sh" {
		t.Errorf("run command = %q, want ./run.sh", got.RunCmd)
	}
}

func TestSetAgentKeepsOtherConfig(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n"+
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n"+
		"tasks:\n  other:\n    model: haiku\n    agent:\n      runCmd: ./other.sh\n")
	d := setAgentDeps(t, workDir)

	if err := runSetAgent([]string{"fix-bug", "--run-cmd", "./run.sh"}, d); err != nil {
		t.Fatalf("set agent = %v, want no error", err)
	}

	other, ok := readConfigAgent(t, workDir, "other")
	if !ok || other.RunCmd != "./other.sh" {
		t.Errorf("other agent = %+v, want the existing one kept", other)
	}
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "anthropic") {
		t.Errorf("config = %q, want the models section kept", data)
	}
	cfg := readCLIConfig(t, workDir)
	if got := cfg.URL; got != "https://bench.example.com" {
		t.Errorf("config url = %q, want it kept", got)
	}
	if got := cfg.Tasks["other"].Model; got != "haiku" {
		t.Errorf("other task model = %q, want it kept", got)
	}
}
