package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// configGraderFile is a grader entry in the CLI config as a test reads it.
type configGraderFile struct {
	VersionCmd string `yaml:"versionCmd"`
	BuildCmd   string `yaml:"buildCmd"`
	RunCmd     string `yaml:"runCmd"`
}

// readConfigGrader reads the grader recorded for the task under workDir and
// whether it exists.
func readConfigGrader(t *testing.T, workDir, stub string) (configGraderFile, bool) {
	t.Helper()
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Tasks map[string]struct {
			Grader *configGraderFile `yaml:"grader"`
		} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	grader := cfg.Tasks[stub].Grader
	if grader == nil {
		return configGraderFile{}, false
	}
	return *grader, true
}

// setGraderDeps returns deps for a set grader command run in an isolated home.
func setGraderDeps(t *testing.T, workDir string) deps {
	t.Helper()
	return deps{
		homeDir: t.TempDir(),
		workDir: workDir,
		in:      strings.NewReader(""),
		out:     io.Discard,
		ctx:     context.Background(),
	}
}

// runSetGrader runs `abnch set grader` with the given arguments.
func runSetGrader(args []string, d deps) error {
	return runCLI(append([]string{"set", "grader"}, args...), d, func(int) {})
}

func TestSetGraderRecordsCommands(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{
		"fix-bug", "--version-cmd", "./version.sh", "--build-cmd", "./build.sh", "--run-cmd", "./run.sh",
	}, d); err != nil {
		t.Fatalf("set grader = %v, want no error", err)
	}

	got, ok := readConfigGrader(t, workDir, "fix-bug")
	if !ok {
		t.Fatal("grader was not recorded")
	}
	if got.VersionCmd != "./version.sh" || got.BuildCmd != "./build.sh" || got.RunCmd != "./run.sh" {
		t.Errorf("grader = %+v, want all three commands", got)
	}
}

func TestSetGraderRecordsRunCommandAlone(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{"fix-bug", "--run-cmd", "./run.sh"}, d); err != nil {
		t.Fatalf("set grader = %v, want no error", err)
	}

	got, ok := readConfigGrader(t, workDir, "fix-bug")
	if !ok {
		t.Fatal("grader was not recorded")
	}
	if got.RunCmd != "./run.sh" {
		t.Errorf("run command = %q, want ./run.sh", got.RunCmd)
	}
}

func TestSetGraderMergesExistingGrader(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    grader:\n      runCmd: ./run.sh\n      buildCmd: ./build.sh\n")
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{"fix-bug", "--version-cmd", "./version.sh"}, d); err != nil {
		t.Fatalf("set grader = %v, want no error", err)
	}

	got, _ := readConfigGrader(t, workDir, "fix-bug")
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

func TestSetGraderFailsWithoutRunCommand(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n")
	d := setGraderDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"set", "grader", "fix-bug", "--build-cmd", "./build.sh"}, d)

	if code == 0 {
		t.Error("exit = 0, want a non-zero exit without a run command")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing when the run command is missing", out)
	}
	if !strings.Contains(errOut, "run command") {
		t.Errorf("stderr = %q, want the missing run command reported", errOut)
	}
	if _, ok := readConfigGrader(t, workDir, "fix-bug"); ok {
		t.Error("a grader was recorded without a run command")
	}
}

func TestSetGraderFailsWithoutRunCommandOnUpdate(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    model: sonnet\n")
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{"fix-bug", "--version-cmd", "./version.sh"}, d); err == nil {
		t.Fatal("set grader = nil, want an error when the grader has no run command")
	}
	if _, ok := readConfigGrader(t, workDir, "fix-bug"); ok {
		t.Error("a grader was recorded without a run command")
	}
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sonnet") {
		t.Errorf("config = %q, want the existing task kept", data)
	}
}

func TestSetGraderCreatesConfig(t *testing.T) {
	workDir := t.TempDir()
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{"fix-bug", "--run-cmd", "./run.sh"}, d); err != nil {
		t.Fatalf("set grader = %v, want no error", err)
	}

	got, ok := readConfigGrader(t, workDir, "fix-bug")
	if !ok {
		t.Fatal("grader was not recorded")
	}
	if got.RunCmd != "./run.sh" {
		t.Errorf("run command = %q, want ./run.sh", got.RunCmd)
	}
}

func TestSetGraderKeepsOtherConfig(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n"+
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n"+
		"tasks:\n  other:\n    model: haiku\n    grader:\n      runCmd: ./other.sh\n")
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{"fix-bug", "--run-cmd", "./run.sh"}, d); err != nil {
		t.Fatalf("set grader = %v, want no error", err)
	}

	other, ok := readConfigGrader(t, workDir, "other")
	if !ok || other.RunCmd != "./other.sh" {
		t.Errorf("other grader = %+v, want the existing one kept", other)
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

func TestSetGraderLeavesAgentAlone(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    agent:\n      runCmd: ./agent.sh\n")
	d := setGraderDeps(t, workDir)

	if err := runSetGrader([]string{"fix-bug", "--run-cmd", "./grade.sh"}, d); err != nil {
		t.Fatalf("set grader = %v, want no error", err)
	}

	agent, ok := readConfigAgent(t, workDir, "fix-bug")
	if !ok || agent.RunCmd != "./agent.sh" {
		t.Errorf("agent = %+v, want the existing one kept", agent)
	}
	grader, _ := readConfigGrader(t, workDir, "fix-bug")
	if grader.RunCmd != "./grade.sh" {
		t.Errorf("grader run command = %q, want ./grade.sh", grader.RunCmd)
	}
}

func TestGetGraderPrintsGraderAsYAML(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"url: https://bench.example.com\n"+
			"tasks:\n  fix-bug:\n    grader:\n      versionCmd: ./version.sh\n      buildCmd: ./build.sh\n      runCmd: ./run.sh\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"grader", "fix-bug"}, d); err != nil {
		t.Fatalf("get grader = %v, want no error", err)
	}

	var got configGraderFile
	if err := yaml.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if got.VersionCmd != "./version.sh" || got.BuildCmd != "./build.sh" || got.RunCmd != "./run.sh" {
		t.Errorf("grader = %+v, want the stored commands", got)
	}
}

func TestGetGraderOmitsAbsentCommands(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    grader:\n      runCmd: ./run.sh\n")
	d, out := getConfigDeps(t, workDir)

	if err := runGet([]string{"grader", "fix-bug"}, d); err != nil {
		t.Fatalf("get grader = %v, want no error", err)
	}
	if strings.Contains(out.String(), "versionCmd") || strings.Contains(out.String(), "buildCmd") {
		t.Errorf("output = %q, want the commands that are not set omitted", out.String())
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
