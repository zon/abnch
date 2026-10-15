package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// resultAPI is one result as the service API returns it. The fake service
// serves it so a get result command reads the same shape the service writes.
type resultAPI struct {
	Ref          string          `json:"ref"`
	Task         string          `json:"task"`
	Test         string          `json:"test"`
	Data         string          `json:"data"`
	JSON         json.RawMessage `json:"json"`
	Logs         string          `json:"logs"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	TokenCost    float64         `json:"tokenCost"`
	Duration     float64         `json:"duration"`
	Local        string          `json:"local"`
}

// seedResult stores a result in the fake service and returns its ref.
func (f *fakeService) seedResult(r store.Result) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref := store.ResultRef(r)
	r.Ref = ref
	f.results[r.Org+"/"+r.Repo+"/"+ref] = r
	return ref
}

// serveListResults writes the repo's result refs as a JSON array, or the service
// error response when the repo does not exist.
func (f *fakeService) serveListResults(w http.ResponseWriter, org, repo string) {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	prefix := org + "/" + repo + "/"
	refs := make([]string, 0)
	for key := range f.results {
		if strings.HasPrefix(key, prefix) {
			refs = append(refs, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(refs)
	httpx.WriteJSON(w, http.StatusOK, refs)
}

// serveGetResult writes one result as JSON, or the service error response when
// the repo or result does not exist.
func (f *fakeService) serveGetResult(w http.ResponseWriter, org, repo, ref string) {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	res, ok := f.results[org+"/"+repo+"/"+ref]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "result not found")
		return
	}
	body := json.RawMessage(res.JSON)
	if len(body) == 0 {
		body = json.RawMessage("null")
	}
	httpx.WriteJSON(w, http.StatusOK, resultAPI{
		Ref: res.Ref, Task: res.Task, Test: res.Test, Data: res.Data,
		JSON: body, Logs: res.Logs,
		InputTokens: res.InputTokens, OutputTokens: res.OutputTokens,
		TokenCost: res.TokenCost, Duration: res.Duration, Local: res.Local,
	})
}

// seedWidgetResults seeds the repo `acme/widgets` with the given results and
// returns their refs.
func seedWidgetResults(t *testing.T, fake *fakeService, results ...store.Result) []string {
	t.Helper()
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	refs := make([]string, 0, len(results))
	for _, r := range results {
		r.Org, r.Repo = "acme", "widgets"
		refs = append(refs, fake.seedResult(r))
	}
	return refs
}

func TestGetResultsListsResultsAsYAML(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	refs := seedWidgetResults(t, fake,
		store.Result{Task: "fix-bug", Test: "basic", Data: "aaaa", JSON: `{"answer":"hi"}`},
		store.Result{Task: "fix-bug", Test: "basic", Data: "bbbb", JSON: `{"answer":"bye"}`},
	)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"results"}, d); err != nil {
		t.Fatalf("get results = %v, want no error", err)
	}

	var listed []string
	if err := yaml.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(listed) != len(refs) || !contains(listed, refs[0]) || !contains(listed, refs[1]) {
		t.Errorf("listed %v, want the refs %v", listed, refs)
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/results") {
		t.Errorf("requests = %v, want the result list request", fake.methods())
	}
}

func TestGetResultsListsOnlyTheRepoResults(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	want := seedWidgetResults(t, fake,
		store.Result{Task: "fix-bug", Test: "basic", Data: "aaaa", JSON: `{"answer":"hi"}`},
	)[0]
	fake.seedResult(store.Result{Org: "acme", Repo: "gadgets", Task: "fix-bug", Test: "basic", Data: "aaaa", JSON: `{"answer":"hi"}`})
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"results"}, d); err != nil {
		t.Fatalf("get results = %v, want no error", err)
	}

	var listed []string
	if err := yaml.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(listed) != 1 || listed[0] != want {
		t.Errorf("listed %v, want only %s", listed, want)
	}
}

func TestGetResultsListsEmptyRepoAsEmptyList(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"results"}, d); err != nil {
		t.Fatalf("get results = %v, want no error", err)
	}

	var listed []string
	if err := yaml.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(listed) != 0 {
		t.Errorf("listed %v, want none", listed)
	}
}

func TestGetResultsFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"results"}, d)
	if err == nil {
		t.Fatal("get results = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetResultsSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := getDeps(t, workDir, service)

	err := runGet([]string{"results"}, d)
	if err == nil {
		t.Fatal("get results = nil, want an error when the service rejects the read")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestGetResultsSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"results"}, d); err == nil {
		t.Fatal("get results = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}

func TestGetResultPrintsResultAsJSON(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedWidgetResults(t, fake, store.Result{
		Task: "fix-bug", Test: "basic", Data: "aaaa",
		JSON: `{"answer":"hi"}`, Logs: "worked\n",
		InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 0.5, Local: "main",
	})[0]
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"result", ref}, d); err != nil {
		t.Fatalf("get result = %v, want no error", err)
	}

	var got struct {
		Ref          string            `json:"ref"`
		Task         string            `json:"task"`
		Test         string            `json:"test"`
		Data         string            `json:"data"`
		JSON         map[string]string `json:"json"`
		Logs         string            `json:"logs"`
		InputTokens  int               `json:"inputTokens"`
		OutputTokens int               `json:"outputTokens"`
		TokenCost    float64           `json:"tokenCost"`
		Duration     float64           `json:"duration"`
		Local        string            `json:"local"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got.Ref != ref || got.Task != "fix-bug" || got.Test != "basic" || got.Data != "aaaa" {
		t.Errorf("result = %+v, want the stored result", got)
	}
	if got.JSON["answer"] != "hi" {
		t.Errorf("json = %v, want the result JSON", got.JSON)
	}
	if got.Logs != "worked\n" || got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 || got.Duration != 0.5 || got.Local != "main" {
		t.Errorf("result = %+v, want the stored logs and metrics", got)
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/results/"+ref) {
		t.Errorf("requests = %v, want the result request", fake.methods())
	}
}

func TestGetResultFailsWhenResultMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"result", "ghost"}, d)
	if err == nil {
		t.Fatal("get result = nil, want an error for a missing result")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing result", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing result", out.String())
	}
}

func TestGetResultReportsMissingResultOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, _ := getDeps(t, workDir, fake.url)

	code, out, errOut := exitCode(t, []string{"get", "result", "ghost"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing result")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing result", out)
	}
	if !strings.Contains(errOut, "ghost") {
		t.Errorf("stderr = %q, want the missing result named", errOut)
	}
}

func TestGetResultFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"result", "ghost"}, d)
	if err == nil {
		t.Fatal("get result = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetResultSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := getDeps(t, workDir, service)

	err := runGet([]string{"result", "ghost"}, d)
	if err == nil {
		t.Fatal("get result = nil, want an error when the service rejects the read")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestGetResultSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"result", "ghost"}, d); err == nil {
		t.Fatal("get result = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}
