package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// serviceRequest records one request the CLI sent to the fake service.
type serviceRequest struct {
	method string
	path   string
	body   string
}

// fakeService is an in-memory repo and task service that records every request,
// so a test drives the CLI without a database.
type fakeService struct {
	mu       sync.Mutex
	repos    map[string]string
	tasks    map[string]task
	tests    map[string]test
	data     map[string]string
	results  map[string]store.Result
	scores   map[string]store.Score
	requests []serviceRequest
	url      string
}

// newFakeService starts a fake service and returns it.
func newFakeService(t *testing.T) *fakeService {
	t.Helper()
	f := &fakeService{repos: map[string]string{}, tasks: map[string]task{}, tests: map[string]test{}, data: map[string]string{}, results: map[string]store.Result{}, scores: map[string]store.Score{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	f.url = srv.URL
	return f
}

// ServeHTTP serves the repo and task API from the fake service's maps.
func (f *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, serviceRequest{method: r.Method, path: r.URL.Path, body: string(body)})

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == http.MethodGet && len(segments) == 3 && segments[0] == "repos":
		f.serveGetRepo(w, segments[1], segments[2])
	case r.Method == http.MethodPut && len(segments) == 3 && segments[0] == "repos":
		f.servePutRepo(w, segments[1], segments[2], body)
	case r.Method == http.MethodGet && len(segments) == 4 && segments[0] == "repos" && segments[3] == "tasks":
		f.serveListTasks(w, segments[1], segments[2])
	case r.Method == http.MethodGet && len(segments) == 5 && segments[0] == "repos" && segments[3] == "tasks":
		f.serveGetTask(w, segments[1], segments[2], segments[4])
	case r.Method == http.MethodPut && len(segments) == 5 && segments[0] == "repos" && segments[3] == "tasks":
		f.servePutTask(w, segments[1], segments[2], segments[4], body)
	case r.Method == http.MethodPost && len(segments) == 6 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "move":
		f.serveMoveTask(w, segments[1], segments[2], segments[4], body)
	case r.Method == http.MethodGet && len(segments) == 6 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests":
		f.serveListTests(w, segments[1], segments[2], segments[4])
	case r.Method == http.MethodGet && len(segments) == 7 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests":
		f.serveGetTest(w, segments[1], segments[2], segments[4], segments[6])
	case r.Method == http.MethodPut && len(segments) == 7 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests":
		f.servePutTest(w, segments[1], segments[2], segments[4], segments[6], body)
	case r.Method == http.MethodPost && len(segments) == 8 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests" && segments[7] == "move":
		f.serveMoveTest(w, segments[1], segments[2], segments[4], segments[6], body)
	case r.Method == http.MethodGet && len(segments) == 8 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests" && segments[7] == "data":
		f.serveListData(w, segments[1], segments[2], segments[4], segments[6])
	case r.Method == http.MethodGet && len(segments) == 9 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests" && segments[7] == "data":
		f.serveGetData(w, segments[1], segments[2], segments[4], segments[6], segments[8])
	case r.Method == http.MethodPost && len(segments) == 8 && segments[0] == "repos" && segments[3] == "tasks" && segments[5] == "tests" && segments[7] == "data":
		f.serveAddData(w, segments[1], segments[2], segments[4], segments[6], body)
	case r.Method == http.MethodGet && len(segments) == 4 && segments[0] == "repos" && segments[3] == "results":
		f.serveListResults(w, segments[1], segments[2])
	case r.Method == http.MethodGet && len(segments) == 5 && segments[0] == "repos" && segments[3] == "results":
		f.serveGetResult(w, segments[1], segments[2], segments[4])
	case r.Method == http.MethodPost && len(segments) == 4 && segments[0] == "repos" && segments[3] == "results":
		f.serveAddResult(w, segments[1], segments[2], body)
	case r.Method == http.MethodPost && len(segments) == 4 && segments[0] == "repos" && segments[3] == "scores":
		f.serveAddScore(w, segments[1], segments[2], body)
	default:
		httpx.WriteError(w, http.StatusNotFound, "route not found")
	}
}

func (f *fakeService) serveGetRepo(w http.ResponseWriter, org, repo string) {
	rawURL, ok := f.repos[org+"/"+repo]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"org": org, "stub": repo, "url": rawURL})
}

func (f *fakeService) servePutRepo(w http.ResponseWriter, org, repo string, body []byte) {
	var req repoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid repo body")
		return
	}
	key := org + "/" + repo
	_, existed := f.repos[key]
	f.repos[key] = req.URL
	status := http.StatusOK
	if !existed {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, map[string]string{"org": org, "stub": repo, "url": req.URL})
}

// serveListTasks writes the repo's tasks as a JSON array, or the service error
// response when the repo does not exist.
func (f *fakeService) serveListTasks(w http.ResponseWriter, org, repo string) {
	rawURL, ok := f.repos[org+"/"+repo]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	tasks := make([]task, 0)
	for key, t := range f.tasks {
		if strings.HasPrefix(key, org+"/"+repo+"/") {
			t.Repo = rawURL
			tasks = append(tasks, t)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, tasks)
}

func (f *fakeService) serveGetTask(w http.ResponseWriter, org, repo, stub string) {
	rawURL, ok := f.repos[org+"/"+repo]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	t, ok := f.tasks[org+"/"+repo+"/"+stub]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "task not found")
		return
	}
	t.Repo = rawURL
	httpx.WriteJSON(w, http.StatusOK, t)
}

func (f *fakeService) servePutTask(w http.ResponseWriter, org, repo, stub string, body []byte) {
	rawURL, ok := f.repos[org+"/"+repo]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	var req taskBody
	if err := json.Unmarshal(body, &req); err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid task body")
		return
	}
	key := org + "/" + repo + "/" + stub
	t, existed := f.tasks[key]
	if !existed {
		if req.Name == "" || req.Description == "" {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "task name and description are required")
			return
		}
		t = task{Stub: stub, Repo: repo}
	}
	if req.Name != "" {
		t.Name = req.Name
	}
	if req.Description != "" {
		t.Description = req.Description
	}
	f.tasks[key] = t
	t.Repo = rawURL
	status := http.StatusOK
	if !existed {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, t)
}

func (f *fakeService) serveMoveTask(w http.ResponseWriter, org, repo, stub string, body []byte) {
	rawURL, ok := f.repos[org+"/"+repo]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	var req moveTaskRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Stub == "" {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid task body")
		return
	}
	key := org + "/" + repo + "/" + stub
	t, ok := f.tasks[key]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "task not found")
		return
	}
	newKey := org + "/" + repo + "/" + req.Stub
	if _, taken := f.tasks[newKey]; taken {
		httpx.WriteError(w, http.StatusConflict, "task stub is taken")
		return
	}
	delete(f.tasks, key)
	t.Stub = req.Stub
	f.tasks[newKey] = t
	t.Repo = rawURL
	httpx.WriteJSON(w, http.StatusOK, t)
}

// seedRepo adds a repo to the fake service.
func (f *fakeService) seedRepo(org, repo, rawURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[org+"/"+repo] = rawURL
}

// seedTask adds a task to the fake service.
func (f *fakeService) seedTask(org string, t task) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks[org+"/"+t.Repo+"/"+t.Stub] = t
}

// repoURL returns the URL stored for a repo, or "" when it is missing.
func (f *fakeService) repoURL(org, repo string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.repos[org+"/"+repo]
}

// task returns a stored task and whether it exists.
func (f *fakeService) task(org, repo, stub string) (task, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[org+"/"+repo+"/"+stub]
	return t, ok
}

// methods returns the recorded requests as "METHOD path" strings.
func (f *fakeService) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.requests))
	for _, req := range f.requests {
		out = append(out, req.method+" "+req.path)
	}
	return out
}

// count returns how many requests the fake service received.
func (f *fakeService) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// contains reports whether values holds want.
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// setTaskDeps returns deps for a set task command with a stored session, the
// service URL in the CLI config, and the given stdin body.
func setTaskDeps(t *testing.T, workDir, serviceURL, stdin string) deps {
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

// runSetTask runs `abnch set task` with the given arguments.
func runSetTask(args []string, d deps) error {
	return runCLI(append([]string{"set", "task"}, args...), d, func(int) {})
}

// forbiddenService answers every request with the service's 403 error.
func forbiddenService(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden: reader cannot write")
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestSetTaskCreatesTask(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--name", "Fix bug", "--description", "Fix the bug"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}

	if got := fake.repoURL("acme", "widgets"); got != "git@github.com:acme/widgets.git" {
		t.Errorf("repo url = %q, want the origin remote", got)
	}
	tk, ok := fake.task("acme", "widgets", "fix-bug")
	if !ok {
		t.Fatal("task was not created")
	}
	if tk.Name != "Fix bug" || tk.Description != "Fix the bug" {
		t.Errorf("task = %+v, want the option values", tk)
	}
}

func TestSetTaskCreatesTaskFromStdin(t *testing.T) {
	bodies := map[string]string{
		"yaml": "name: Fix bug\ndescription: Fix the bug\n",
		"json": `{"name":"Fix bug","description":"Fix the bug"}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			workDir := t.TempDir()
			addRemote(t, workDir, "git@github.com:acme/widgets.git")
			fake := newFakeService(t)
			d := setTaskDeps(t, workDir, fake.url, body)

			if err := runSetTask([]string{"fix-bug"}, d); err != nil {
				t.Fatalf("set task = %v, want no error", err)
			}
			tk, ok := fake.task("acme", "widgets", "fix-bug")
			if !ok {
				t.Fatal("task was not created")
			}
			if tk.Name != "Fix bug" || tk.Description != "Fix the bug" {
				t.Errorf("task = %+v, want the stdin values", tk)
			}
		})
	}
}

func TestSetTaskUpdatesFromStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "old"})
	d := setTaskDeps(t, workDir, fake.url, "description: new\n")

	if err := runSetTask([]string{"fix-bug"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}
	tk, _ := fake.task("acme", "widgets", "fix-bug")
	if tk.Name != "Fix bug" {
		t.Errorf("name = %q, want it unchanged", tk.Name)
	}
	if tk.Description != "new" {
		t.Errorf("description = %q, want the stdin value", tk.Description)
	}
	if contains(fake.methods(), "PUT /repos/acme/widgets") {
		t.Error("the CLI wrote a repo that already exists")
	}
}

func TestSetTaskMergesOptionsOverStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d := setTaskDeps(t, workDir, fake.url, "name: stdin name\ndescription: stdin description\n")

	if err := runSetTask([]string{"fix-bug", "--name", "flag name"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}
	tk, _ := fake.task("acme", "widgets", "fix-bug")
	if tk.Name != "flag name" {
		t.Errorf("name = %q, want the option value", tk.Name)
	}
	if tk.Description != "stdin description" {
		t.Errorf("description = %q, want the stdin value", tk.Description)
	}
}

func TestSetTaskReusesExistingRepo(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--name", "Fix bug", "--description", "Fix the bug"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}
	if contains(fake.methods(), "PUT /repos/acme/widgets") {
		t.Error("the CLI wrote a repo that already exists")
	}
	if _, ok := fake.task("acme", "widgets", "fix-bug"); !ok {
		t.Error("task was not created in the existing repo")
	}
}

func TestSetTaskMovesTask(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--move", "fix-typo"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}

	if _, ok := fake.task("acme", "widgets", "fix-bug"); ok {
		t.Error("the old stub still holds a task")
	}
	moved, ok := fake.task("acme", "widgets", "fix-typo")
	if !ok {
		t.Fatal("the task was not renamed")
	}
	if moved.Name != "Fix bug" || moved.Description != "Fix the bug" {
		t.Errorf("task = %+v, want the original fields", moved)
	}
}

func TestSetTaskMoveFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--move", "fix-typo"}, d); err == nil {
		t.Fatal("set task = nil, want an error for a missing task")
	}
	if _, ok := fake.task("acme", "widgets", "fix-typo"); ok {
		t.Error("a task was created by a move of a missing task")
	}
}

func TestSetTaskMoveFailsWhenStubTaken(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "one"})
	fake.seedTask("acme", task{Stub: "fix-typo", Repo: "widgets", Name: "Fix typo", Description: "two"})
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--move", "fix-typo"}, d); err == nil {
		t.Fatal("set task = nil, want an error when the new stub is taken")
	}
	if _, ok := fake.task("acme", "widgets", "fix-bug"); !ok {
		t.Error("the moved task was changed")
	}
	if _, ok := fake.task("acme", "widgets", "fix-typo"); !ok {
		t.Error("the task holding the new stub was changed")
	}
}

func TestSetTaskFailsWithoutRequiredField(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d := setTaskDeps(t, workDir, fake.url, "description: only\n")

	err := runSetTask([]string{"fix-bug"}, d)
	if err == nil {
		t.Fatal("set task = nil, want an error for a missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error = %q, want it to name the missing field", err)
	}
	if _, ok := fake.task("acme", "widgets", "fix-bug"); ok {
		t.Error("a task was created without a required field")
	}
	if got := fake.repoURL("acme", "widgets"); got != "" {
		t.Errorf("repo url = %q, want no repo created", got)
	}
}

func TestReadTaskBodyIgnoresTerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	body, err := readTaskBody(f)
	if err != nil {
		t.Fatalf("readTaskBody = %v, want no error", err)
	}
	if body != (taskBody{}) {
		t.Errorf("body = %+v, want an empty body", body)
	}
}

// cliConfigFile is the CLI config as a test reads it, allowing keys the
// production type does not model.
type cliConfigFile struct {
	URL   string `yaml:"url"`
	Tasks map[string]struct {
		Model string `yaml:"model"`
	} `yaml:"tasks"`
}

// readCLIConfig reads the CLI config under workDir.
func readCLIConfig(t *testing.T, workDir string) cliConfigFile {
	t.Helper()
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	var cfg cliConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSetTaskRecordsDefaultModel(t *testing.T) {
	workDir := t.TempDir()
	fake := newFakeService(t)
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--model", "sonnet"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}

	cfg := readCLIConfig(t, workDir)
	if got := cfg.Tasks["fix-bug"].Model; got != "sonnet" {
		t.Errorf("task model = %q, want sonnet", got)
	}
	if got := cfg.URL; got != fake.url {
		t.Errorf("config url = %q, want it kept", got)
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for a model-only set", fake.count())
	}
}

func TestSetTaskModelLeavesServiceTaskUnchanged(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	d := setTaskDeps(t, workDir, fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--model", "sonnet"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}

	tk, ok := fake.task("acme", "widgets", "fix-bug")
	if !ok {
		t.Fatal("the task was removed by a model-only set")
	}
	if tk.Name != "Fix bug" || tk.Description != "Fix the bug" {
		t.Errorf("task = %+v, want it unchanged", tk)
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for a model-only set", fake.count())
	}
}

func TestSetTaskModelCreatesConfig(t *testing.T) {
	workDir := t.TempDir()
	d := deps{homeDir: t.TempDir(), workDir: workDir, in: strings.NewReader(""), out: io.Discard, ctx: context.Background()}

	if err := runSetTask([]string{"fix-bug", "--model", "sonnet"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}

	cfg := readCLIConfig(t, workDir)
	if got := cfg.Tasks["fix-bug"].Model; got != "sonnet" {
		t.Errorf("task model = %q, want sonnet", got)
	}
}

func TestSetTaskModelKeepsOtherConfig(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "url: https://bench.example.com\n"+
		"models:\n  sonnet:\n    provider: anthropic\n"+
		"tasks:\n  other:\n    model: haiku\n")
	d := deps{homeDir: t.TempDir(), workDir: workDir, in: strings.NewReader(""), out: io.Discard, ctx: context.Background()}

	if err := runSetTask([]string{"fix-bug", "--model", "sonnet"}, d); err != nil {
		t.Fatalf("set task = %v, want no error", err)
	}

	cfg := readCLIConfig(t, workDir)
	if got := cfg.Tasks["other"].Model; got != "haiku" {
		t.Errorf("other task model = %q, want it kept", got)
	}
	if got := cfg.Tasks["fix-bug"].Model; got != "sonnet" {
		t.Errorf("task model = %q, want sonnet", got)
	}
	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "anthropic") {
		t.Errorf("config = %q, want the models section kept", data)
	}
}

func TestSetTaskFailsOnInvalidStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d := setTaskDeps(t, workDir, fake.url, "name: [\n")

	if err := runSetTask([]string{"fix-bug"}, d); err == nil {
		t.Fatal("set task = nil, want an error for an invalid body")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid body", fake.count())
	}
}

func TestSetTaskSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d := setTaskDeps(t, workDir, service, "")

	err := runSetTask([]string{"fix-bug", "--name", "Fix bug", "--description", "Fix the bug"}, d)
	if err == nil {
		t.Fatal("set task = nil, want an error when the service rejects the write")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestSetTaskSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d := setTaskDeps(t, t.TempDir(), fake.url, "")

	if err := runSetTask([]string{"fix-bug", "--name", "Fix bug", "--description", "Fix the bug"}, d); err == nil {
		t.Fatal("set task = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}
