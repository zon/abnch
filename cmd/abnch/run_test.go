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

// serveAddResult stores the result the body carries for its datum and returns
// its ref, mirroring the service: the same JSON for the same datum returns the
// existing ref as 200, and a new result returns 201. A malformed body, or a
// missing repo, task, test, or datum, writes the service error response.
func (f *fakeService) serveAddResult(w http.ResponseWriter, org, repo string, body []byte) {
	var req resultRequest
	if err := json.Unmarshal(body, &req); err != nil || len(req.JSON) == 0 {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid result body")
		return
	}
	if !f.requireDataTest(w, org, repo, req.Task, req.Test) {
		return
	}
	if _, ok := f.data[org+"/"+repo+"/"+req.Task+"/"+req.Test+"/"+req.Data]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "datum not found")
		return
	}

	result := store.Result{
		Org: org, Repo: repo, Task: req.Task, Test: req.Test, Data: req.Data,
		JSON: string(req.JSON), Logs: req.Logs,
		InputTokens: req.InputTokens, OutputTokens: req.OutputTokens,
		TokenCost: req.TokenCost, Duration: req.Duration, Local: req.Local,
	}
	ref := store.ResultRef(result)
	key := org + "/" + repo + "/" + ref
	_, existed := f.results[key]
	f.results[key] = result
	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, resultRef{Ref: ref})
}

// resultRefs returns the repo's result refs, ordered by ref.
func (f *fakeService) resultRefs(org, repo string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := org + "/" + repo + "/"
	refs := make([]string, 0)
	for key := range f.results {
		if strings.HasPrefix(key, prefix) {
			refs = append(refs, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(refs)
	return refs
}

// result returns a stored result and whether it exists.
func (f *fakeService) result(org, repo, ref string) (store.Result, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.results[org+"/"+repo+"/"+ref]
	return r, ok
}

// runDeps returns deps for a run command with a stored session, the given CLI
// config, and a buffer that captures the command output.
func runDeps(t *testing.T, workDir, serviceURL, config string) (deps, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	if err := devconfig.StoreSession(home, serviceURL, "session-token"); err != nil {
		t.Fatal(err)
	}
	writeCLIConfig(t, workDir, config)
	out := new(bytes.Buffer)
	return deps{
		homeDir: home,
		workDir: workDir,
		in:      strings.NewReader(""),
		out:     out,
		ctx:     context.Background(),
	}, out
}

// runRun runs `abnch run` with the given arguments.
func runRun(args []string, d deps) error {
	return runCLI(append([]string{"run"}, args...), d, func(int) {})
}

// runConfig is a CLI config that records a model `sonnet` and a task `fix-bug`
// whose agent builds and runs shell scripts in the working directory.
func runConfig(serviceURL string) string {
	return "url: " + serviceURL + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n    agent:\n      buildCmd: sh build.sh\n      runCmd: sh run.sh\n"
}

// writeRunScripts writes a build script that records each build and a run
// script that records each run, each stdin, and a run output with a result and
// token counts.
func writeRunScripts(t *testing.T, workDir string) {
	t.Helper()
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "run.sh",
		"input=$(cat)\n"+
			"echo \"$input\" >> inputs.log\n"+
			"echo run >> runs.log\n"+
			"printf 'worked\\n__abnch__\\n{\"json\":{\"answer\":\"hi\"},\"inputTokens\":12,\"outputTokens\":3,\"tokenCost\":0.01}\\n'\n")
}

// writeScript writes an executable shell script under workDir.
func writeScript(t *testing.T, workDir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workDir, name), []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

// countLines returns the number of non-empty lines in the file at path, or zero
// when it does not exist.
func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func TestRunRunsAgentAgainstEachDatum(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"bye"}`)
	writeRunScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 1 {
		t.Errorf("build ran %d times, want once", got)
	}
	if got := countLines(t, filepath.Join(workDir, "runs.log")); got != 2 {
		t.Errorf("agent ran %d times, want once per datum", got)
	}

	recorded := fake.resultRefs("acme", "widgets")
	if len(recorded) != 2 {
		t.Fatalf("recorded %d results, want 2", len(recorded))
	}
	printed := strings.Fields(out.String())
	if len(printed) != 2 {
		t.Fatalf("printed %q, want two refs", out.String())
	}
	for _, ref := range printed {
		if !contains(recorded, ref) {
			t.Errorf("printed ref %q is not one of the recorded refs %v", ref, recorded)
		}
	}
	if !contains(fake.methods(), "POST /repos/acme/widgets/results") {
		t.Errorf("requests = %v, want the result write", fake.methods())
	}
}

func TestRunRecordsTheRunOutput(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	recorded := fake.resultRefs("acme", "widgets")
	if len(recorded) != 1 {
		t.Fatalf("recorded %d results, want 1", len(recorded))
	}
	got, ok := fake.result("acme", "widgets", recorded[0])
	if !ok {
		t.Fatal("the result was not recorded")
	}
	if got.Logs != "worked\n" {
		t.Errorf("logs = %q, want the text before the marker", got.Logs)
	}
	if got.JSON != `{"answer":"hi"}` {
		t.Errorf("result JSON = %q, want the run output's json", got.JSON)
	}
	if got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 {
		t.Errorf("metrics = %d/%d/%v, want the run output's tokens and cost", got.InputTokens, got.OutputTokens, got.TokenCost)
	}
	if got.Duration <= 0 {
		t.Errorf("duration = %v, want a measured duration", got.Duration)
	}
}

func TestRunPassesModelAndDatumOnStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"hi"}`
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "inputs.log"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Model struct {
			Provider string `json:"provider"`
			Name     string `json:"name"`
			Variant  string `json:"variant"`
		} `json:"model"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatalf("stdin is not JSON: %v\n%s", err, data)
	}
	if got.Model.Provider != "anthropic" || got.Model.Name != "claude-sonnet" || got.Model.Variant != "high" {
		t.Errorf("model = %+v, want the task's model", got.Model)
	}
	if string(got.Data) != body {
		t.Errorf("data = %s, want the datum %s", got.Data, body)
	}
}

func TestRunPassesRunInputAsJSONOnStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"hi"}`
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	raw, err := os.ReadFile(filepath.Join(workDir, "inputs.log"))
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &input); err != nil {
		t.Fatalf("stdin is not a JSON object: %v\n%s", err, raw)
	}
	if len(input) != 2 || input["model"] == nil || input["data"] == nil {
		t.Fatalf("stdin = %s, want an object carrying only model and data", raw)
	}
	var model map[string]json.RawMessage
	if err := json.Unmarshal(input["model"], &model); err != nil {
		t.Fatalf("model is not a JSON object: %v\n%s", err, input["model"])
	}
	for _, key := range []string{"name", "provider", "variant"} {
		if _, ok := model[key]; !ok {
			t.Errorf("model = %s, want it to carry %q", input["model"], key)
		}
	}
	if string(input["data"]) != body {
		t.Errorf("data = %s, want the datum %s", input["data"], body)
	}
}

func TestRunPassesModelWithoutVariantOnStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	body := `{"prompt":"hi"}`
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	writeRunScripts(t, workDir)
	config := "url: " + fake.url + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n    agent:\n      runCmd: sh run.sh\n"
	d, _ := runDeps(t, workDir, fake.url, config)

	if err := runRun([]string{"fix-bug/basic"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	got := readRunInput(t, workDir)
	if got.Model.Provider != "anthropic" || got.Model.Name != "claude-sonnet" {
		t.Errorf("model = %+v, want the model's provider and name", got.Model)
	}
	if got.Model.Variant != "" {
		t.Errorf("variant = %q, want empty for a model without a variant", got.Model.Variant)
	}
	if string(got.Data) != body {
		t.Errorf("data = %s, want the datum %s", got.Data, body)
	}
}

func TestRunRunsAgentAgainstOneDatum(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	ref := fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"bye"}`)
	writeRunScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic/" + ref}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 1 {
		t.Errorf("build ran %d times, want once", got)
	}
	if got := countLines(t, filepath.Join(workDir, "runs.log")); got != 1 {
		t.Errorf("agent ran %d times, want once for one datum", got)
	}

	recorded := fake.resultRefs("acme", "widgets")
	if len(recorded) != 1 {
		t.Fatalf("recorded %d results, want 1", len(recorded))
	}
	if printed := strings.TrimSpace(out.String()); printed != recorded[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, recorded[0])
	}
	got, ok := fake.result("acme", "widgets", recorded[0])
	if !ok {
		t.Fatal("the result was not recorded")
	}
	if got.Data != ref {
		t.Errorf("result datum = %q, want the named datum %q", got.Data, ref)
	}
}

func TestRunFailsWhenDatumIsMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeRunScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, runConfig(fake.url))

	err := runRun([]string{"fix-bug/basic/ghost"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error for a missing datum")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing datum", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing datum", out.String())
	}
	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 0 {
		t.Errorf("build ran %d times, want none for a missing datum", got)
	}
	if got := countLines(t, filepath.Join(workDir, "runs.log")); got != 0 {
		t.Errorf("agent ran %d times, want none for a missing datum", got)
	}
	if got := fake.resultRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v results, want none for a missing datum", got)
	}
}

func TestRunReportsMissingDatumOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic/ghost"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing datum")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing datum", out)
	}
	if !strings.Contains(errOut, "ghost") {
		t.Errorf("stderr = %q, want the missing datum named", errOut)
	}
}

func TestRunFailsWhenTaskHasNoAgent(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n"+
			"tasks:\n  fix-bug:\n    model: sonnet\n")
	d, out := getConfigDeps(t, workDir)

	err := runRun([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error when the task has no agent")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing agent", out.String())
	}
}

func TestRunReportsMissingAgentOnStderr(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    model: sonnet\n")
	d, _ := getConfigDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic"}, d)
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

func TestRunFailsWhenNoModelIsSet(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    agent:\n      runCmd: sh run.sh\n")
	d, out := getConfigDeps(t, workDir)

	err := runRun([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error when no model is set")
	}
	if !strings.Contains(err.Error(), "fix-bug") || !strings.Contains(err.Error(), "model") {
		t.Errorf("error = %q, want the task and the missing model reported", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing when no model is set", out.String())
	}
}

func TestRunReportsMissingModelOnStderr(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "tasks:\n  fix-bug:\n    agent:\n      runCmd: sh run.sh\n")
	d, _ := getConfigDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit when no model is set")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing when no model is set", out)
	}
	if !strings.Contains(errOut, "model") {
		t.Errorf("stderr = %q, want the missing model reported", errOut)
	}
}

func TestRunFailsWhenModelIsMissing(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"tasks:\n  fix-bug:\n    model: ghost\n    agent:\n      runCmd: sh run.sh\n")
	d, _ := getConfigDeps(t, workDir)

	err := runRun([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error for a model the config does not hold")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing model", err)
	}
}

func TestRunFailsWhenTestIsMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	writeRunScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, runConfig(fake.url))

	err := runRun([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error for a missing test")
	}
	if !strings.Contains(err.Error(), "basic") {
		t.Errorf("error = %q, want it to name the missing test", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing test", out.String())
	}
	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 0 {
		t.Errorf("build ran %d times, want none for a missing test", got)
	}
	if got := countLines(t, filepath.Join(workDir, "runs.log")); got != 0 {
		t.Errorf("agent ran %d times, want none for a missing test", got)
	}
}

func TestRunReportsMissingTestOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	fake.seedTask("acme", task{Stub: "fix-bug", Repo: "widgets", Name: "Fix bug", Description: "Fix the bug"})
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic"}, d)
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

func TestRunFailsWhenAgentWritesNoMarker(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "run.sh", "echo no marker here\n")
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	err := runRun([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error when the output carries no marker")
	}
	if !strings.Contains(err.Error(), "marker") {
		t.Errorf("error = %q, want it to report the missing marker", err)
	}
	if got := fake.resultRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v results, want none without a marker", got)
	}
}

func TestRunFailsWhenRunOutputHasNoResult(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "run.sh", "printf '__abnch__\\n{\"inputTokens\":1}\\n'\n")
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	err := runRun([]string{"fix-bug/basic"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error when the run output carries no result JSON")
	}
	if !strings.Contains(err.Error(), "result") {
		t.Errorf("error = %q, want it to report the missing result JSON", err)
	}
	if got := fake.resultRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v results, want none without a result JSON", got)
	}
}

func TestRunReportsMissingMarkerOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "run.sh", "echo no marker here\n")
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit when the output carries no marker")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing when the output carries no marker", out)
	}
	if !strings.Contains(errOut, "marker") {
		t.Errorf("stderr = %q, want the missing marker reported", errOut)
	}
}

func TestRunReportsMissingResultOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "run.sh", "printf '__abnch__\\n{\"inputTokens\":1}\\n'\n")
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit when the run output carries no result JSON")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing when the run output carries no result JSON", out)
	}
	if !strings.Contains(errOut, "result") {
		t.Errorf("stderr = %q, want the missing result JSON reported", errOut)
	}
}

// commitWorkDir commits an empty change so the Git repo under dir is on a
// branch. It skips the test when git is not installed.
func commitWorkDir(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	cmd := exec.Command("git",
		"-c", "user.email=test@example.com",
		"-c", "user.name=Test",
		"commit", "--allow-empty", "-q", "-m", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

// runInputRecord is the run input a test reads back from the agent's recorded
// stdin.
type runInputRecord struct {
	Model struct {
		Provider string `json:"provider"`
		Name     string `json:"name"`
		Variant  string `json:"variant"`
	} `json:"model"`
	Data json.RawMessage `json:"data"`
}

// readRunInput reads the last run input the agent recorded on stdin.
func readRunInput(t *testing.T, workDir string) runInputRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workDir, "inputs.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var got runInputRecord
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &got); err != nil {
		t.Fatalf("stdin is not JSON: %v\n%s", err, data)
	}
	return got
}

func TestRunModelFlagOverridesTheTaskModel(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeRunScripts(t, workDir)
	config := "url: " + fake.url + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n" +
		"  opus:\n    provider: anthropic\n    name: claude-opus\n    variant: low\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n    agent:\n      runCmd: sh run.sh\n"
	d, _ := runDeps(t, workDir, fake.url, config)

	if err := runRun([]string{"fix-bug/basic", "--model", "opus"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	got := readRunInput(t, workDir)
	if got.Model.Provider != "anthropic" || got.Model.Name != "claude-opus" || got.Model.Variant != "low" {
		t.Errorf("model = %+v, want the flag's model", got.Model)
	}
}

func TestRunFailsWhenModelFlagNamesAMissingModel(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n"+
			"tasks:\n  fix-bug:\n    model: sonnet\n    agent:\n      runCmd: sh run.sh\n")
	d, out := getConfigDeps(t, workDir)

	err := runRun([]string{"fix-bug/basic", "--model", "ghost"}, d)
	if err == nil {
		t.Fatal("run = nil, want an error for a model the config does not hold")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing model", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing model", out.String())
	}
}

func TestRunReportsMissingModelFlagOnStderr(t *testing.T) {
	workDir := t.TempDir()
	writeCLIConfig(t, workDir,
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n"+
			"tasks:\n  fix-bug:\n    model: sonnet\n    agent:\n      runCmd: sh run.sh\n")
	d, _ := getConfigDeps(t, workDir)

	code, out, errOut := exitCode(t, []string{"run", "fix-bug/basic", "--model", "ghost"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a model the config does not hold")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing model", out)
	}
	if !strings.Contains(errOut, "ghost") {
		t.Errorf("stderr = %q, want the missing model named", errOut)
	}
}

func TestRunLimitCapsTheData(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	for _, body := range []string{`{"prompt":"one"}`, `{"prompt":"two"}`, `{"prompt":"three"}`} {
		fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	}
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic", "--limit", "2"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	if got := countLines(t, filepath.Join(workDir, "runs.log")); got != 2 {
		t.Errorf("agent ran %d times, want the limit of 2", got)
	}
	if got := fake.resultRefs("acme", "widgets"); len(got) != 2 {
		t.Errorf("recorded %d results, want the limit of 2", len(got))
	}
}

func TestRunShuffleRandomizesTheDataOrder(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	var refs []string
	for _, body := range []string{`{"prompt":"one"}`, `{"prompt":"two"}`, `{"prompt":"three"}`} {
		refs = append(refs, fake.seedDatum("acme", "widgets", "fix-bug", "basic", body))
	}
	sort.Strings(refs)
	writeRunScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, runConfig(fake.url))

	original := shuffleRunData
	shuffleRunData = func(data []runDatum) {
		for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
			data[i], data[j] = data[j], data[i]
		}
	}
	t.Cleanup(func() { shuffleRunData = original })

	if err := runRun([]string{"fix-bug/basic", "--shuffle"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	want := make([]string, len(refs))
	for i, datum := range refs {
		result := store.Result{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: datum, JSON: `{"answer":"hi"}`}
		want[len(refs)-1-i] = store.ResultRef(result)
	}
	if got := strings.Fields(out.String()); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("printed %v, want the shuffled order %v", got, want)
	}
}

func TestRunLocalRecordsTheLocalBranch(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	commitWorkDir(t, workDir)
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeRunScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, runConfig(fake.url))

	if err := runRun([]string{"fix-bug/basic", "--local"}, d); err != nil {
		t.Fatalf("run = %v, want no error", err)
	}

	recorded := fake.resultRefs("acme", "widgets")
	if len(recorded) != 1 {
		t.Fatalf("recorded %d results, want 1", len(recorded))
	}
	got, ok := fake.result("acme", "widgets", recorded[0])
	if !ok {
		t.Fatal("the result was not recorded")
	}
	if got.Local == "" {
		t.Error("local branch is empty, want the working directory's branch")
	}
}
