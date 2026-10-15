package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// serveAddData stores the body's JSON as a datum and returns its ref, mirroring
// the service's content-addressed upsert: the same JSON returns the existing ref
// as 200, and new JSON returns 201. A malformed body, or a missing repo, task,
// or test, writes the service error response.
func (f *fakeService) serveAddData(w http.ResponseWriter, org, repo, task, test string, body []byte) {
	if !f.requireDataTest(w, org, repo, task, test) {
		return
	}
	if !json.Valid(body) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid datum body")
		return
	}
	ref := store.DatumRef(body)
	key := org + "/" + repo + "/" + task + "/" + test + "/" + ref
	_, existed := f.data[key]
	f.data[key] = string(body)
	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, dataRef{Ref: ref})
}

// requireDataTest writes the service error response and reports false when the
// repo, task, or test a data request names does not exist.
func (f *fakeService) requireDataTest(w http.ResponseWriter, org, repo, task, test string) bool {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return false
	}
	if _, ok := f.tasks[org+"/"+repo+"/"+task]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "task not found")
		return false
	}
	if _, ok := f.tests[org+"/"+repo+"/"+task+"/"+test]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "test not found")
		return false
	}
	return true
}

// serveListData writes a test's data refs as a JSON array, ordered by ref.
func (f *fakeService) serveListData(w http.ResponseWriter, org, repo, task, test string) {
	if !f.requireDataTest(w, org, repo, task, test) {
		return
	}
	prefix := org + "/" + repo + "/" + task + "/" + test + "/"
	refs := make([]string, 0)
	for key := range f.data {
		if strings.HasPrefix(key, prefix) {
			refs = append(refs, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(refs)
	httpx.WriteJSON(w, http.StatusOK, refs)
}

// serveGetData writes one datum's JSON unchanged, or the service error response
// when the repo, task, test, or datum does not exist.
func (f *fakeService) serveGetData(w http.ResponseWriter, org, repo, task, test, ref string) {
	if !f.requireDataTest(w, org, repo, task, test) {
		return
	}
	body, ok := f.data[org+"/"+repo+"/"+task+"/"+test+"/"+ref]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "datum not found")
		return
	}
	httpx.WriteRawJSON(w, http.StatusOK, []byte(body))
}

// seedDatum stores the body's JSON as a datum in the fake service and returns
// its ref.
func (f *fakeService) seedDatum(org, repo, task, test, body string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref := store.DatumRef([]byte(body))
	f.data[org+"/"+repo+"/"+task+"/"+test+"/"+ref] = body
	return ref
}

// datum returns the JSON stored for a ref and whether it exists.
func (f *fakeService) datum(org, repo, task, test, ref string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, ok := f.data[org+"/"+repo+"/"+task+"/"+test+"/"+ref]
	return body, ok
}

// addDataDeps returns deps for an add data command with a stored session, the
// service URL in the CLI config, and a buffer that captures the command output.
func addDataDeps(t *testing.T, workDir, serviceURL, stdin string) (deps, *bytes.Buffer) {
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
		in:      strings.NewReader(stdin),
		out:     out,
		ctx:     context.Background(),
	}, out
}

// runAdd runs `abnch add` with the given arguments.
func runAdd(args []string, d deps) error {
	return runCLI(append([]string{"add"}, args...), d, func(int) {})
}

// seedDataTest seeds the repo, task, and test an add data command needs.
func seedDataTest(fake *fakeService) {
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	fake.seedTest("acme", "widgets", "fix-bug", test{Stub: "basic", Name: "Basic", Version: "1"})
}

func TestAddDataFromStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"hi"}`
	d, out := addDataDeps(t, workDir, fake.url, body)

	if err := runAdd([]string{"data", "fix-bug/basic"}, d); err != nil {
		t.Fatalf("add data = %v, want no error", err)
	}

	ref := store.DatumRef([]byte(body))
	if got := strings.TrimSpace(out.String()); got != ref {
		t.Errorf("ref = %q, want %q", got, ref)
	}
	stored, ok := fake.datum("acme", "widgets", "fix-bug", "basic", ref)
	if !ok {
		t.Fatal("the datum was not stored")
	}
	if stored != body {
		t.Errorf("stored datum = %q, want the stdin JSON", stored)
	}
	if !contains(fake.methods(), "POST /repos/acme/widgets/tasks/fix-bug/tests/basic/data") {
		t.Errorf("requests = %v, want the data write", fake.methods())
	}
}

func TestAddDataFromFile(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"from file"}`
	if err := os.WriteFile(filepath.Join(workDir, "case.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	d, out := addDataDeps(t, workDir, fake.url, "")

	if err := runAdd([]string{"data", "fix-bug/basic", "--file", "case.json"}, d); err != nil {
		t.Fatalf("add data = %v, want no error", err)
	}

	ref := store.DatumRef([]byte(body))
	if got := strings.TrimSpace(out.String()); got != ref {
		t.Errorf("ref = %q, want %q", got, ref)
	}
	if _, ok := fake.datum("acme", "widgets", "fix-bug", "basic", ref); !ok {
		t.Error("the datum was not stored")
	}
}

func TestAddDataSameJSONPrintsSameRef(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"hi"}`
	d, out := addDataDeps(t, workDir, fake.url, body)

	if err := runAdd([]string{"data", "fix-bug/basic"}, d); err != nil {
		t.Fatalf("add data = %v, want no error", err)
	}
	d.in = strings.NewReader(body)
	if err := runAdd([]string{"data", "fix-bug/basic"}, d); err != nil {
		t.Fatalf("second add data = %v, want no error", err)
	}

	ref := store.DatumRef([]byte(body))
	lines := strings.Fields(out.String())
	if len(lines) != 2 || lines[0] != ref || lines[1] != ref {
		t.Errorf("refs = %q, want the same %q twice", out.String(), ref)
	}
}

func TestAddDataFailsOnInvalidJSON(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, out := addDataDeps(t, workDir, fake.url, `{"prompt":`)

	err := runAdd([]string{"data", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("add data = nil, want an error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("error = %q, want it to report invalid JSON", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for invalid JSON", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for invalid JSON", fake.count())
	}
}

func TestAddDataReportsInvalidJSONOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := addDataDeps(t, workDir, fake.url, `{"prompt":`)

	code, out, errOut := exitCode(t, []string{"add", "data", "fix-bug/basic"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for invalid JSON")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for invalid JSON", out)
	}
	if !strings.Contains(errOut, "invalid JSON") {
		t.Errorf("stderr = %q, want the invalid JSON reported", errOut)
	}
}

func TestAddDataFailsWhenFileUnreadable(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := addDataDeps(t, workDir, fake.url, "")

	if err := runAdd([]string{"data", "fix-bug/basic", "--file", "missing.json"}, d); err == nil {
		t.Fatal("add data = nil, want an error for a missing file")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for a missing file", fake.count())
	}
}

func TestAddDataFailsWhenTestMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	d, out := addDataDeps(t, workDir, fake.url, `{"prompt":"hi"}`)

	err := runAdd([]string{"data", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("add data = nil, want an error for a missing test")
	}
	if !strings.Contains(err.Error(), "basic") {
		t.Errorf("error = %q, want it to name the missing test", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing test", out.String())
	}
}

func TestAddDataReportsMissingTestOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	d, _ := addDataDeps(t, workDir, fake.url, `{"prompt":"hi"}`)

	code, out, errOut := exitCode(t, []string{"add", "data", "fix-bug/basic"}, d)
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

func TestAddDataFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := addDataDeps(t, workDir, fake.url, `{"prompt":"hi"}`)

	err := runAdd([]string{"data", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("add data = nil, want an error for a missing task")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the missing task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing task", out.String())
	}
}

func TestAddDataFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := addDataDeps(t, workDir, fake.url, `{"prompt":"hi"}`)

	err := runAdd([]string{"data", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("add data = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestAddDataFailsOnInvalidPath(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := addDataDeps(t, workDir, fake.url, `{"prompt":"hi"}`)

	if err := runAdd([]string{"data", "fix-bug"}, d); err == nil {
		t.Fatal("add data = nil, want an error for a path without a test stub")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid path", fake.count())
	}
}

func TestAddDataSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := addDataDeps(t, workDir, service, `{"prompt":"hi"}`)

	err := runAdd([]string{"data", "fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("add data = nil, want an error when the service rejects the write")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestAddDataSendsNoRequestOutsideGitRepo(t *testing.T) {
	fake := newFakeService(t)
	d, _ := addDataDeps(t, t.TempDir(), fake.url, `{"prompt":"hi"}`)

	if err := runAdd([]string{"data", "fix-bug/basic"}, d); err == nil {
		t.Fatal("add data = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}

// runGetData runs `abnch get data` with the given arguments.
func runGetData(args []string, d deps) error {
	return runCLI(append([]string{"get", "data"}, args...), d, func(int) {})
}

func TestGetDataListsRefs(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	first := fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	second := fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"bye"}`)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGetData([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("get data = %v, want no error", err)
	}

	refs := strings.Fields(out.String())
	if len(refs) != 2 || !contains(refs, first) || !contains(refs, second) {
		t.Errorf("listed %q, want both refs %s and %s", out.String(), first, second)
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data") {
		t.Errorf("requests = %v, want the data list request", fake.methods())
	}
}

func TestGetDataListsOnlyThatTestsData(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedTest("acme", "widgets", "fix-bug", test{Stub: "smoke", Name: "Smoke", Version: "1"})
	want := fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	fake.seedDatum("acme", "widgets", "fix-bug", "smoke", `{"prompt":"other"}`)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGetData([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("get data = %v, want no error", err)
	}

	refs := strings.Fields(out.String())
	if len(refs) != 1 || refs[0] != want {
		t.Errorf("listed %q, want only %s", out.String(), want)
	}
}

func TestGetDataListsEmptyTestAsNothing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGetData([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("get data = %v, want no error", err)
	}

	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("output = %q, want nothing for a test with no data", out.String())
	}
}

func TestGetDataPrintsDatumAsJSON(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"hi"}`
	ref := fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGetData([]string{"fix-bug/basic/" + ref}, d); err != nil {
		t.Fatalf("get data = %v, want no error", err)
	}

	if got := strings.TrimSpace(out.String()); got != body {
		t.Errorf("output = %q, want the datum JSON %q", got, body)
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data/"+ref) {
		t.Errorf("requests = %v, want the datum request", fake.methods())
	}
}

func TestGetDataPrintsFileReferenceUnchanged(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"url":"https://example.com/input.txt","checksum":"sha256:abc"}`
	ref := fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGetData([]string{"fix-bug/basic/" + ref}, d); err != nil {
		t.Fatalf("get data = %v, want no error", err)
	}

	var parsed struct {
		URL      string `json:"url"`
		Checksum string `json:"checksum"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if parsed.URL != "https://example.com/input.txt" || parsed.Checksum != "sha256:abc" {
		t.Errorf("file reference = %+v, want the url and checksum readable", parsed)
	}
}

func TestGetDataFailsWhenDatumMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, out := getDeps(t, workDir, fake.url)

	err := runGetData([]string{"fix-bug/basic/missing"}, d)
	if err == nil {
		t.Fatal("get data = nil, want an error for a missing datum")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %q, want it to name the missing datum", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing datum", out.String())
	}
}

func TestGetDataReportsMissingDatumOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := getDeps(t, workDir, fake.url)

	code, out, errOut := exitCode(t, []string{"get", "data", "fix-bug/basic/missing"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing datum")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing datum", out)
	}
	if !strings.Contains(errOut, "missing") {
		t.Errorf("stderr = %q, want the missing datum named", errOut)
	}
}

func TestGetDataFailsWhenTestMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	d, out := getDeps(t, workDir, fake.url)

	err := runGetData([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get data = nil, want an error for a missing test")
	}
	if !strings.Contains(err.Error(), "basic") {
		t.Errorf("error = %q, want it to name the missing test", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing test", out.String())
	}
}

func TestGetDataFailsWhenTaskMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	err := runGetData([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get data = nil, want an error for a missing task")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the missing task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing task", out.String())
	}
}

func TestGetDataFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGetData([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get data = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetDataFailsOnInvalidPath(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := getDeps(t, workDir, fake.url)

	for _, path := range []string{"fix-bug", "fix-bug/basic/ref/extra", "fix-bug/"} {
		if err := runGetData([]string{path}, d); err == nil {
			t.Errorf("get data %q = nil, want an error", path)
		}
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid path", fake.count())
	}
}

func TestGetDataSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := getDeps(t, workDir, service)

	err := runGetData([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("get data = nil, want an error when the service rejects the read")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestGetDataSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGetData([]string{"fix-bug/basic"}, d); err == nil {
		t.Fatal("get data = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}
