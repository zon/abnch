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
		if tk.Repo != "git@github.com:acme/widgets.git" {
			t.Errorf("task %s repo = %q, want the repo URL", tk.Stub, tk.Repo)
		}
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
	if tk.Stub != "fix-bug" || tk.Name != "Fix bug" || tk.Description != "Fix the bug" || tk.Repo != "git@github.com:acme/widgets.git" {
		t.Errorf("task = %+v, want the stored task with its repo URL", tk)
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

	err := runGet([]string{"tasks"}, d)
	if err == nil {
		t.Fatal("get tasks = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetTaskFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"task", "fix-bug"}, d)
	if err == nil {
		t.Fatal("get task = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
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

func TestGetTestsListsTestsAsYAML(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	fake.seedTest("acme", "widgets", "fix-bug", test{Stub: "basic", Name: "Basic", Version: "1"})
	fake.seedTest("acme", "widgets", "fix-bug", test{
		Stub: "smoke", Name: "Smoke", Version: "1",
		Schema: []schemaKey{{Stub: "correctness", Name: "Correctness"}},
	})
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"tests", "fix-bug"}, d); err != nil {
		t.Fatalf("get tests = %v, want no error", err)
	}

	var tests []test
	if err := yaml.Unmarshal(out.Bytes(), &tests); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(tests) != 2 {
		t.Fatalf("listed %d tests, want 2", len(tests))
	}
	stubs := map[string]test{}
	for _, tt := range tests {
		stubs[tt.Stub] = tt
	}
	if stubs["basic"].Name != "Basic" || stubs["basic"].Version != "1" {
		t.Errorf("basic = %+v, want the stored test", stubs["basic"])
	}
	if stubs["smoke"].Name != "Smoke" {
		t.Errorf("smoke = %+v, want the stored test", stubs["smoke"])
	}
	if len(stubs["smoke"].Schema) != 1 || stubs["smoke"].Schema[0].Stub != "correctness" {
		t.Errorf("smoke schema = %+v, want it listed", stubs["smoke"].Schema)
	}
	if !strings.Contains(out.String(), "stub: basic") {
		t.Errorf("output = %q, want YAML", out.String())
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks/fix-bug/tests") {
		t.Errorf("requests = %v, want the test list request", fake.methods())
	}
}

func TestGetTestsListsEmptyTaskAsEmptyList(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"tests", "fix-bug"}, d); err != nil {
		t.Fatalf("get tests = %v, want no error", err)
	}

	var tests []test
	if err := yaml.Unmarshal(out.Bytes(), &tests); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(tests) != 0 {
		t.Errorf("listed %d tests, want none", len(tests))
	}
}

func TestGetTestPrintsTestAsYAML(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	fake.seedTest("acme", "widgets", "fix-bug", test{
		Stub: "basic", Name: "Basic", Description: "The basic test", Version: "1",
		Schema: []schemaKey{{Stub: "correctness", Name: "Correctness"}},
	})
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"test", "fix-bug/basic"}, d); err != nil {
		t.Fatalf("get test = %v, want no error", err)
	}

	var tt test
	if err := yaml.Unmarshal(out.Bytes(), &tt); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if tt.Stub != "basic" || tt.Name != "Basic" || tt.Description != "The basic test" || tt.Version != "1" {
		t.Errorf("test = %+v, want the stored test", tt)
	}
	if len(tt.Schema) != 1 || tt.Schema[0].Stub != "correctness" || tt.Schema[0].Name != "Correctness" {
		t.Errorf("schema = %+v, want the stored schema", tt.Schema)
	}
	if !strings.Contains(out.String(), "stub: basic") {
		t.Errorf("output = %q, want YAML", out.String())
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks/fix-bug/tests/basic") {
		t.Errorf("requests = %v, want the test request", fake.methods())
	}
}

func TestGetTestFailsWhenTestMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"test", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get test = nil, want an error for a missing test")
	}
	if !strings.Contains(err.Error(), "basic") {
		t.Errorf("error = %q, want it to name the missing test", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing test", out.String())
	}
}

func TestGetTestReportsMissingTestOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d, _ := getDeps(t, workDir, fake.url)

	code, out, errOut := exitCode(t, []string{"get", "test", "fix-bug/basic"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing test")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing test", out)
	}
	if !strings.Contains(errOut, "basic") {
		t.Errorf("stderr = %q, want the missing test named", errOut)
	}
}

func TestGetTestsFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"tests", "fix-bug"}, d)
	if err == nil {
		t.Fatal("get tests = nil, want an error for a missing task")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the missing task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing task", out.String())
	}
}

func TestGetTestFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"test", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get test = nil, want an error for a missing task")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the missing task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing task", out.String())
	}
}

func TestGetTestsFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"tests", "fix-bug"}, d)
	if err == nil {
		t.Fatal("get tests = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetTestFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"test", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get test = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetTestFailsOnInvalidPath(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d, _ := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"test", "fix-bug"}, d); err == nil {
		t.Fatal("get test = nil, want an error for a path without a stub")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid path", fake.count())
	}
}

func TestGetTestsSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"tests", "fix-bug"}, d); err == nil {
		t.Fatal("get tests = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}

func TestGetTestSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"test", "fix-bug/basic"}, d); err == nil {
		t.Fatal("get test = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}
