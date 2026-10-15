package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/httpx"
)

// serveGetTest writes one test as JSON, or the service error response when the
// repo, task, or test does not exist.
func (f *fakeService) serveGetTest(w http.ResponseWriter, org, repo, task, stub string) {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	if _, ok := f.tasks[org+"/"+repo+"/"+task]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "task not found")
		return
	}
	t, ok := f.tests[org+"/"+repo+"/"+task+"/"+stub]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "test not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

// servePutTest creates or updates a test, mirroring the service's merge and
// validation. It answers 201 when it creates a test and 200 when it updates one.
func (f *fakeService) servePutTest(w http.ResponseWriter, org, repo, task, stub string, body []byte) {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	if _, ok := f.tasks[org+"/"+repo+"/"+task]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "task not found")
		return
	}
	var req testBody
	if err := json.Unmarshal(body, &req); err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid test body")
		return
	}
	key := org + "/" + repo + "/" + task + "/" + stub
	t, existed := f.tests[key]
	if !existed {
		if req.Name == "" || req.Version == "" {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "test name and version are required")
			return
		}
		t = test{Stub: stub}
	}
	if req.Name != "" {
		t.Name = req.Name
	}
	if req.Description != "" {
		t.Description = req.Description
	}
	if req.Version != "" {
		t.Version = req.Version
	}
	if req.Schema != nil {
		t.Schema = req.Schema
	}
	f.tests[key] = t
	status := http.StatusOK
	if !existed {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, t)
}

// seedTest adds a test to the fake service.
func (f *fakeService) seedTest(org, repo, task string, t test) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tests[org+"/"+repo+"/"+task+"/"+t.Stub] = t
}

// test returns a stored test and whether it exists.
func (f *fakeService) test(org, repo, task, stub string) (test, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tests[org+"/"+repo+"/"+task+"/"+stub]
	return t, ok
}

// setTestDeps returns deps for a set test command with a stored session, the
// service URL in the CLI config, and the given stdin body.
func setTestDeps(t *testing.T, workDir, serviceURL, stdin string) deps {
	t.Helper()
	home := t.TempDir()
	if err := devconfig.StoreSession(home, serviceURL, "session-token"); err != nil {
		t.Fatal(err)
	}
	writeCLIConfig(t, workDir, "url: "+serviceURL+"\n")
	return deps{
		homeDir: home,
		workDir: workDir,
		in:      strings.NewReader(stdin),
		out:     io.Discard,
		ctx:     context.Background(),
	}
}

// runSetTest runs `abnch set test` with the given arguments.
func runSetTest(args []string, d deps) error {
	return runCLI(append([]string{"set", "test"}, args...), d, func(int) {})
}

// seedTestTask seeds the repo and task a set test command needs.
func seedTestTask(fake *fakeService) {
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
}

func TestSetTestCreatesTest(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d := setTestDeps(t, workDir, fake.url, "")

	if err := runSetTest([]string{"fix-bug/basic", "--name", "Basic", "--version", "1"}, d); err != nil {
		t.Fatalf("set test = %v, want no error", err)
	}

	got, ok := fake.test("acme", "widgets", "fix-bug", "basic")
	if !ok {
		t.Fatal("test was not created")
	}
	if got.Stub != "basic" || got.Name != "Basic" || got.Version != "1" {
		t.Errorf("test = %+v, want the option values", got)
	}
	if !contains(fake.methods(), "PUT /repos/acme/widgets/tasks/fix-bug/tests/basic") {
		t.Errorf("requests = %v, want the test write", fake.methods())
	}
}

func TestSetTestCreatesTestFromStdin(t *testing.T) {
	bodies := map[string]string{
		"yaml": "name: Basic\nversion: \"1\"\nschema:\n  - stub: correctness\n    name: Correctness\n",
		"json": `{"name":"Basic","version":"1","schema":[{"stub":"correctness","name":"Correctness"}]}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			workDir := t.TempDir()
			addRemote(t, workDir, "git@github.com:acme/widgets.git")
			fake := newFakeService(t)
			seedTestTask(fake)
			d := setTestDeps(t, workDir, fake.url, body)

			if err := runSetTest([]string{"fix-bug/basic"}, d); err != nil {
				t.Fatalf("set test = %v, want no error", err)
			}
			got, ok := fake.test("acme", "widgets", "fix-bug", "basic")
			if !ok {
				t.Fatal("test was not created")
			}
			if got.Name != "Basic" || got.Version != "1" {
				t.Errorf("test = %+v, want the stdin values", got)
			}
			if len(got.Schema) != 1 || got.Schema[0].Stub != "correctness" || got.Schema[0].Name != "Correctness" {
				t.Errorf("schema = %+v, want the stdin schema", got.Schema)
			}
		})
	}
}

func TestSetTestUpdatesFromStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	fake.seedTest("acme", "widgets", "fix-bug", test{
		Stub: "basic", Name: "Basic", Version: "1", Schema: []schemaKey{{Stub: "old", Name: "Old"}},
	})
	d := setTestDeps(t, workDir, fake.url, "schema:\n  - stub: new\n    name: New\n")

	if err := runSetTest([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("set test = %v, want no error", err)
	}

	got, _ := fake.test("acme", "widgets", "fix-bug", "basic")
	if got.Name != "Basic" || got.Version != "1" {
		t.Errorf("test = %+v, want the name and version unchanged", got)
	}
	if len(got.Schema) != 1 || got.Schema[0].Stub != "new" {
		t.Errorf("schema = %+v, want the stdin schema", got.Schema)
	}
	if !contains(fake.methods(), "PUT /repos/acme/widgets/tasks/fix-bug/tests/basic") {
		t.Errorf("requests = %v, want the test write", fake.methods())
	}
}

func TestSetTestUpdateKeepsSchema(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	fake.seedTest("acme", "widgets", "fix-bug", test{
		Stub: "basic", Name: "Basic", Version: "1", Schema: []schemaKey{{Stub: "correctness", Name: "Correctness"}},
	})
	d := setTestDeps(t, workDir, fake.url, "")

	if err := runSetTest([]string{"fix-bug/basic", "--version", "2"}, d); err != nil {
		t.Fatalf("set test = %v, want no error", err)
	}

	got, _ := fake.test("acme", "widgets", "fix-bug", "basic")
	if got.Version != "2" {
		t.Errorf("version = %q, want the option value", got.Version)
	}
	if len(got.Schema) != 1 || got.Schema[0].Stub != "correctness" {
		t.Errorf("schema = %+v, want the stored schema kept", got.Schema)
	}
}

func TestSetTestMergesOptionsOverStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d := setTestDeps(t, workDir, fake.url, "name: stdin name\nversion: \"1\"\ndescription: stdin description\n")

	if err := runSetTest([]string{"fix-bug/basic", "--name", "flag name"}, d); err != nil {
		t.Fatalf("set test = %v, want no error", err)
	}
	got, _ := fake.test("acme", "widgets", "fix-bug", "basic")
	if got.Name != "flag name" {
		t.Errorf("name = %q, want the option value", got.Name)
	}
	if got.Description != "stdin description" {
		t.Errorf("description = %q, want the stdin value", got.Description)
	}
	if got.Version != "1" {
		t.Errorf("version = %q, want the stdin value", got.Version)
	}
}

func TestSetTestFailsWithoutRequiredField(t *testing.T) {
	tests := map[string]struct {
		body string
		args []string
		want string
	}{
		"missing name": {
			body: "description: only\nversion: \"1\"\n",
			args: []string{"fix-bug/basic"},
			want: "name",
		},
		"missing version": {
			body: "name: Basic\n",
			args: []string{"fix-bug/basic"},
			want: "version",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			workDir := t.TempDir()
			addRemote(t, workDir, "git@github.com:acme/widgets.git")
			fake := newFakeService(t)
			seedTestTask(fake)
			d := setTestDeps(t, workDir, fake.url, tc.body)

			err := runSetTest(tc.args, d)
			if err == nil {
				t.Fatal("set test = nil, want an error for a missing required field")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name the missing %s", err, tc.want)
			}
			if _, ok := fake.test("acme", "widgets", "fix-bug", "basic"); ok {
				t.Error("a test was created without a required field")
			}
		})
	}
}

func TestSetTestFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d := setTestDeps(t, workDir, fake.url, "")

	if err := runSetTest([]string{"fix-bug/basic", "--name", "Basic", "--version", "1"}, d); err == nil {
		t.Fatal("set test = nil, want an error for a missing task")
	} else if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the missing task", err)
	}
	if _, ok := fake.test("acme", "widgets", "fix-bug", "basic"); ok {
		t.Error("a test was created under a missing task")
	}
	if _, ok := fake.task("acme", "widgets", "fix-bug"); ok {
		t.Error("a task was created by a set test")
	}
}

func TestSetTestFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d := setTestDeps(t, workDir, fake.url, "")

	err := runSetTest([]string{"fix-bug/basic", "--name", "Basic", "--version", "1"}, d)
	if err == nil {
		t.Fatal("set test = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo reported as unregistered", err)
	}
	if got := fake.repoURL("acme", "widgets"); got != "" {
		t.Errorf("repo url = %q, want no repo created", got)
	}
}

func TestSetTestFailsOnInvalidStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d := setTestDeps(t, workDir, fake.url, "schema: [\n")

	if err := runSetTest([]string{"fix-bug/basic", "--name", "Basic", "--version", "1"}, d); err == nil {
		t.Fatal("set test = nil, want an error for an invalid body")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid body", fake.count())
	}
}

func TestSetTestFailsOnInvalidPath(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedTestTask(fake)
	d := setTestDeps(t, workDir, fake.url, "")

	if err := runSetTest([]string{"fix-bug", "--name", "Basic", "--version", "1"}, d); err == nil {
		t.Fatal("set test = nil, want an error for a path without a stub")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid path", fake.count())
	}
}

func TestSetTestSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d := setTestDeps(t, workDir, service, "")

	err := runSetTest([]string{"fix-bug/basic", "--name", "Basic", "--version", "1"}, d)
	if err == nil {
		t.Fatal("set test = nil, want an error when the service rejects the write")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestSetTestSendsNoRequestOutsideGitRepo(t *testing.T) {
	fake := newFakeService(t)
	d := setTestDeps(t, t.TempDir(), fake.url, "")

	if err := runSetTest([]string{"fix-bug/basic", "--name", "Basic", "--version", "1"}, d); err == nil {
		t.Fatal("set test = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}

func TestSetTestReportsMissingTaskOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d := setTestDeps(t, workDir, fake.url, "")

	code, out, errOut := exitCode(t, []string{"set", "test", "fix-bug/basic", "--name", "Basic", "--version", "1"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing task")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing on failure", out)
	}
	if !strings.Contains(errOut, "fix-bug") {
		t.Errorf("stderr = %q, want the missing task named", errOut)
	}
}

func TestReadTestBodyIgnoresTerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	body, err := readTestBody(f)
	if err != nil {
		t.Fatalf("readTestBody = %v, want no error", err)
	}
	if body.Name != "" || body.Version != "" || body.Schema != nil {
		t.Errorf("body = %+v, want an empty body", body)
	}
}
