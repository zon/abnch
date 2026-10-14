package main

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/zon/abnch/internal/devconfig"
)

// getDeps returns deps for a get command with a stored session, the service URL
// in the CLI config, and a buffer that captures the command output.
func getDeps(t *testing.T, workDir, serviceURL string) (deps, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	if err := devconfig.StoreSession(home, serviceURL, "session-token"); err != nil {
		t.Fatal(err)
	}
	writeCLIConfig(t, workDir, "url: "+serviceURL+"\n")
	out := new(bytes.Buffer)
	return deps{
		homeDir: home,
		workDir: workDir,
		in:      strings.NewReader(""),
		out:     out,
		ctx:     context.Background(),
	}, out
}

// runGet runs `abnch get` with the given arguments.
func runGet(args []string, d deps) error {
	return runCLI(append([]string{"get"}, args...), d, func(int) {})
}

func TestGetTasksListsTasksAsYAML(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	fake.seedTask("acme", task{Stub: "add-test", Repo: "widgets", Name: "Add test", Description: "Add a test"})
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"tasks"}, d); err != nil {
		t.Fatalf("get tasks = %v, want no error", err)
	}

	var tasks []task
	if err := yaml.Unmarshal(out.Bytes(), &tasks); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(tasks) != 2 {
		t.Fatalf("listed %d tasks, want 2", len(tasks))
	}
	stubs := map[string]bool{}
	for _, tk := range tasks {
		stubs[tk.Stub] = true
	}
	if !stubs["fix-bug"] || !stubs["add-test"] {
		t.Errorf("listed %v, want both tasks", stubs)
	}
	if !strings.Contains(out.String(), "stub: fix-bug") {
		t.Errorf("output = %q, want YAML", out.String())
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks") {
		t.Errorf("requests = %v, want the task list request", fake.methods())
	}
}

func TestGetTasksListsEmptyRepoAsEmptyList(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"tasks"}, d); err != nil {
		t.Fatalf("get tasks = %v, want no error", err)
	}

	var tasks []task
	if err := yaml.Unmarshal(out.Bytes(), &tasks); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(tasks) != 0 {
		t.Errorf("listed %d tasks, want none", len(tasks))
	}
}

func TestGetTaskPrintsTaskAsYAML(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"task", "fix-bug"}, d); err != nil {
		t.Fatalf("get task = %v, want no error", err)
	}

	var tk task
	if err := yaml.Unmarshal(out.Bytes(), &tk); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if tk.Stub != "fix-bug" || tk.Name != "Fix bug" || tk.Description != "Fix the bug" || tk.Repo != "widgets" {
		t.Errorf("task = %+v, want the stored task", tk)
	}
	if !strings.Contains(out.String(), "stub: fix-bug") {
		t.Errorf("output = %q, want YAML", out.String())
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks/fix-bug") {
		t.Errorf("requests = %v, want the task request", fake.methods())
	}
}

func TestGetTaskFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"task", "fix-bug"}, d); err == nil {
		t.Fatal("get task = nil, want an error for a missing task")
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing task", out.String())
	}
}

func TestGetTasksFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"tasks"}, d); err == nil {
		t.Fatal("get tasks = nil, want an error for a missing repo")
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetTasksSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"tasks"}, d); err == nil {
		t.Fatal("get tasks = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}
