package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestSetConfigCreatesConfig(t *testing.T) {
	workDir := t.TempDir()
	d := commandDeps(t, workDir)

	code, _, errOut := exitCode(t, []string{"set", "config", "--url", "https://bench.example.com"}, d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing on success", errOut)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "https://bench.example.com" {
		t.Errorf("url = %q, want the given url", cfg.URL)
	}
	info, err := os.Stat(filepath.Join(workDir, configDir))
	if err != nil {
		t.Fatalf("config directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", configDir)
	}
}

func TestSetConfigUpdatesURLAndPreservesModelsAndTasks(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://old.example.com\n"+
		"models:\n"+
		"  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n"+
		"  haiku:\n    provider: anthropic\n    name: claude-haiku\n"+
		"tasks:\n"+
		"  fix-bug:\n    model: sonnet\n    agent:\n      versionCmd: ./version.sh\n      buildCmd: ./build.sh\n      runCmd: ./run.sh\n"+
		"  write-docs:\n    model: haiku\n    grader:\n      runCmd: ./grade.sh\n")
	before, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	d := commandDeps(t, workDir)

	code, _, errOut := exitCode(t, []string{"set", "config", "--url", "https://new.example.com"}, d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	after, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if after.URL != "https://new.example.com" {
		t.Errorf("url = %q, want the new url", after.URL)
	}
	if !reflect.DeepEqual(after.Models, before.Models) {
		t.Errorf("models = %+v, want the existing models kept as %+v", after.Models, before.Models)
	}
	if !reflect.DeepEqual(after.Tasks, before.Tasks) {
		t.Errorf("tasks = %+v, want the existing tasks kept as %+v", after.Tasks, before.Tasks)
	}
}

func TestGetConfigPrintsConfigAsYAML(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "# Service config\n"+
		"url: https://bench.example.com\n"+
		"models:\n"+
		"  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n"+
		"tasks:\n"+
		"  fix-bug:\n    model: sonnet\n    agent:\n      runCmd: ./run.sh\n    grader:\n      runCmd: ./grade.sh\n")
	want, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	d := commandDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"get", "config"}, d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing on success", errOut)
	}

	var got cliConfig
	if err := yaml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out)
	}
	if got.URL != want.URL {
		t.Errorf("url = %q, want %q", got.URL, want.URL)
	}
	if !reflect.DeepEqual(got.Models, want.Models) {
		t.Errorf("models = %+v, want %+v", got.Models, want.Models)
	}
	if !reflect.DeepEqual(got.Tasks, want.Tasks) {
		t.Errorf("tasks = %+v, want %+v", got.Tasks, want.Tasks)
	}
}

func TestGetConfigWithoutConfigPrintsNothing(t *testing.T) {
	workDir := t.TempDir()
	d := commandDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"get", "config"}, d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 without a config: %s", code, errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing without a config", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing without a config", errOut)
	}
}

func TestGetConfigWithoutConfigFilePrintsNothing(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, configDir), 0o700); err != nil {
		t.Fatal(err)
	}
	d := commandDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"get", "config"}, d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 without a config file: %s", code, errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing without a config file", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing without a config file", errOut)
	}
}

func TestSetConfigFailsWithoutURL(t *testing.T) {
	workDir := t.TempDir()
	d := commandDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"set", "config"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit without a url")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing without a url", out)
	}
	if !strings.Contains(errOut, "--url") {
		t.Errorf("stderr = %q, want it to name --url", errOut)
	}
	if _, err := os.Stat(configPath(workDir)); !os.IsNotExist(err) {
		t.Errorf("config was written without a url: stat = %v", err)
	}
}

func TestSetConfigFailsWithoutURLInExistingConfig(t *testing.T) {
	workDir := t.TempDir()
	body := "models:\n" +
		"  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n"
	writeCLIConfig(t, workDir, body)
	d := commandDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"set", "config"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit when the config has no url")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing without a url", out)
	}
	if !strings.Contains(errOut, "--url") {
		t.Errorf("stderr = %q, want it to name --url", errOut)
	}
	after, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("config = %q, want it unchanged as %q", after, body)
	}
}

func TestCommandsWithoutServiceURLNameSetConfig(t *testing.T) {
	commands := []struct {
		name string
		args []string
	}{
		{"get tasks", []string{"get", "tasks"}},
		{"login", []string{"login"}},
		{"logout", []string{"logout"}},
	}
	setups := []struct {
		name    string
		prepare func(t *testing.T, workDir string)
	}{
		{"no config", func(*testing.T, string) {}},
		{"config without url", func(t *testing.T, workDir string) {
			writeCLIConfig(t, workDir, "models: {}\n")
		}},
	}
	for _, setup := range setups {
		for _, command := range commands {
			t.Run(setup.name+"/"+command.name, func(t *testing.T) {
				workDir := t.TempDir()
				setup.prepare(t, workDir)
				d := commandDeps(t, workDir)

				code, out, errOut := exitCode(t, command.args, d)
				if code == 0 {
					t.Error("exit = 0, want a non-zero exit without a service url")
				}
				if out != "" {
					t.Errorf("stdout = %q, want nothing without a service url", out)
				}
				if !strings.Contains(errOut, "abnch set config --url <url>") {
					t.Errorf("stderr = %q, want it to name abnch set config --url <url>", errOut)
				}
			})
		}
	}
}

func TestSetConfigKeepsExistingURLWhenNotGiven(t *testing.T) {
	workDir := t.TempDir()
	body := "# Service config\n" +
		"url: https://bench.example.com  # primary\n" +
		"models:\n" +
		"  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n" +
		"tasks:\n" +
		"  fix-bug:\n    model: sonnet\n"
	writeCLIConfig(t, workDir, body)
	d := commandDeps(t, workDir)

	code, _, errOut := exitCode(t, []string{"set", "config"}, d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 with an existing url: %s", code, errOut)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing on success", errOut)
	}

	after, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("config = %q, want it unchanged as %q", after, body)
	}
	cfg, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "https://bench.example.com" {
		t.Errorf("url = %q, want the existing url kept", cfg.URL)
	}
}
