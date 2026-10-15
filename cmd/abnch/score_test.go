package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// serveAddScore stores the score the body carries for its result and returns its
// ref, mirroring the service: the same dictionary for the same result returns
// the existing ref as 200, and a new score returns 201. A malformed body, or a
// missing repo, task, test, or result, writes the service error response.
func (f *fakeService) serveAddScore(w http.ResponseWriter, org, repo string, body []byte) {
	var req scoreRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Dictionary == nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid score body")
		return
	}
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	if _, ok := f.tasks[org+"/"+repo+"/"+req.Task]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "task not found")
		return
	}
	if _, ok := f.tests[org+"/"+repo+"/"+req.Task+"/"+req.Test]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "test not found")
		return
	}
	if _, ok := f.results[org+"/"+repo+"/"+req.Result]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "result not found")
		return
	}

	score := store.Score{
		Org: org, Repo: repo, Task: req.Task, Test: req.Test, Result: req.Result,
		Dictionary: req.Dictionary, Logs: req.Logs,
		InputTokens: req.InputTokens, OutputTokens: req.OutputTokens,
		TokenCost: req.TokenCost, Duration: req.Duration,
	}
	ref := store.ScoreRef(score)
	score.Ref = ref
	key := org + "/" + repo + "/" + ref
	_, existed := f.scores[key]
	f.scores[key] = score
	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, scoreRef{Ref: ref})
}

// score returns a stored score and whether it exists.
func (f *fakeService) score(org, repo, ref string) (store.Score, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.scores[org+"/"+repo+"/"+ref]
	return s, ok
}

// scoreRefs returns the repo's score refs, ordered by ref.
func (f *fakeService) scoreRefs(org, repo string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := org + "/" + repo + "/"
	refs := make([]string, 0)
	for key := range f.scores {
		if strings.HasPrefix(key, prefix) {
			refs = append(refs, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(refs)
	return refs
}

// seedScoreResult seeds the repo, task, test, datum, and result a score command
// needs and returns the result ref.
func seedScoreResult(t *testing.T, fake *fakeService, body, resultJSON string) string {
	t.Helper()
	seedDataTest(fake)
	datum := fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	return fake.seedResult(store.Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: datum, JSON: resultJSON,
	})
}

// runScore runs `abnch score` with the given arguments.
func runScore(args []string, d deps) error {
	return runCLI(append([]string{"score"}, args...), d, func(int) {})
}

// scoreConfig is a CLI config that records a model `sonnet` and a task `fix-bug`
// whose grader builds and runs shell scripts in the working directory.
func scoreConfig(serviceURL string) string {
	return "url: " + serviceURL + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n    grader:\n      buildCmd: sh build.sh\n      runCmd: sh grade.sh\n"
}

// writeGraderScripts writes a build script that records each build and a grade
// script that records each run and its stdin, and writes a score output with a
// dictionary and token counts.
func writeGraderScripts(t *testing.T, workDir string) {
	t.Helper()
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "grade.sh",
		"input=$(cat)\n"+
			"echo \"$input\" >> inputs.log\n"+
			"echo grade >> grades.log\n"+
			"printf 'grading\\n__abnch__\\n{\"score\":{\"correctness\":1},\"inputTokens\":12,\"outputTokens\":3,\"tokenCost\":0.01}\\n'\n")
}

func TestScoreRunsGraderAndRecordsScore(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	writeGraderScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, scoreConfig(fake.url))

	if err := runScore([]string{ref}, d); err != nil {
		t.Fatalf("score = %v, want no error", err)
	}

	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 1 {
		t.Errorf("build ran %d times, want once", got)
	}
	if got := countLines(t, filepath.Join(workDir, "grades.log")); got != 1 {
		t.Errorf("grader ran %d times, want once", got)
	}

	recorded := fake.scoreRefs("acme", "widgets")
	if len(recorded) != 1 {
		t.Fatalf("recorded %d scores, want 1", len(recorded))
	}
	if printed := strings.TrimSpace(out.String()); printed != recorded[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, recorded[0])
	}
	got, ok := fake.score("acme", "widgets", recorded[0])
	if !ok {
		t.Fatal("the score was not recorded")
	}
	if got.Task != "fix-bug" || got.Test != "basic" || got.Result != ref {
		t.Errorf("score identity = %s/%s/%s, want fix-bug/basic/%s", got.Task, got.Test, got.Result, ref)
	}
	if got.Dictionary["correctness"] != 1 {
		t.Errorf("dictionary = %v, want correctness 1", got.Dictionary)
	}
	if got.Logs != "grading\n" {
		t.Errorf("logs = %q, want the text before the marker", got.Logs)
	}
	if got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 {
		t.Errorf("metrics = %d/%d/%v, want the score output's tokens and cost", got.InputTokens, got.OutputTokens, got.TokenCost)
	}
	if got.Duration <= 0 {
		t.Errorf("duration = %v, want a measured duration", got.Duration)
	}
	if !contains(fake.methods(), "POST /repos/acme/widgets/scores") {
		t.Errorf("requests = %v, want the score write", fake.methods())
	}
}

func TestScorePassesGraderInputOnStdin(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	body := `{"prompt":"hi"}`
	resultJSON := `{"answer":"hi"}`
	ref := seedScoreResult(t, fake, body, resultJSON)
	writeGraderScripts(t, workDir)
	d, _ := runDeps(t, workDir, fake.url, scoreConfig(fake.url))

	if err := runScore([]string{ref}, d); err != nil {
		t.Fatalf("score = %v, want no error", err)
	}

	raw, err := os.ReadFile(filepath.Join(workDir, "inputs.log"))
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &input); err != nil {
		t.Fatalf("stdin is not a JSON object: %v\n%s", err, raw)
	}
	if len(input) != 3 || input["model"] == nil || input["data"] == nil || input["result"] == nil {
		t.Fatalf("stdin = %s, want an object carrying only model, data, and result", raw)
	}
	var model struct {
		Provider string `json:"provider"`
		Name     string `json:"name"`
		Variant  string `json:"variant"`
	}
	if err := json.Unmarshal(input["model"], &model); err != nil {
		t.Fatalf("model is not a JSON object: %v\n%s", err, input["model"])
	}
	if model.Provider != "anthropic" || model.Name != "claude-sonnet" || model.Variant != "high" {
		t.Errorf("model = %+v, want the task's model", model)
	}
	if string(input["data"]) != body {
		t.Errorf("data = %s, want the datum %s", input["data"], body)
	}
	if string(input["result"]) != resultJSON {
		t.Errorf("result = %s, want %s", input["result"], resultJSON)
	}
}

func TestScoreFailsWhenTaskHasNoGrader(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	writeGraderScripts(t, workDir)
	config := "url: " + fake.url + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n"
	d, out := runDeps(t, workDir, fake.url, config)

	err := runScore([]string{ref}, d)
	if err == nil {
		t.Fatal("score = nil, want an error when the task has no grader")
	}
	if !strings.Contains(err.Error(), "fix-bug") {
		t.Errorf("error = %q, want it to name the task", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing grader", out.String())
	}
	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 0 {
		t.Errorf("build ran %d times, want none for a missing grader", got)
	}
	if got := countLines(t, filepath.Join(workDir, "grades.log")); got != 0 {
		t.Errorf("grader ran %d times, want none for a missing grader", got)
	}
	if got := fake.scoreRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v scores, want none for a missing grader", got)
	}
}

func TestScoreReportsMissingGraderOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	config := "url: " + fake.url + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n"
	d, _ := runDeps(t, workDir, fake.url, config)

	code, out, errOut := exitCode(t, []string{"score", ref}, d)
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

func TestScoreFailsWhenNoModelIsSet(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	writeGraderScripts(t, workDir)
	config := "url: " + fake.url + "\n" +
		"tasks:\n  fix-bug:\n    grader:\n      runCmd: sh grade.sh\n"
	d, out := runDeps(t, workDir, fake.url, config)

	err := runScore([]string{ref}, d)
	if err == nil {
		t.Fatal("score = nil, want an error when no model is set")
	}
	if !strings.Contains(err.Error(), "fix-bug") || !strings.Contains(err.Error(), "model") {
		t.Errorf("error = %q, want the task and the missing model reported", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing when no model is set", out.String())
	}
	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 0 {
		t.Errorf("build ran %d times, want none when no model is set", got)
	}
	if got := countLines(t, filepath.Join(workDir, "grades.log")); got != 0 {
		t.Errorf("grader ran %d times, want none when no model is set", got)
	}
	if got := fake.scoreRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v scores, want none when no model is set", got)
	}
}

func TestScoreReportsMissingModelOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	config := "url: " + fake.url + "\n" +
		"tasks:\n  fix-bug:\n    grader:\n      runCmd: sh grade.sh\n"
	d, _ := runDeps(t, workDir, fake.url, config)

	code, out, errOut := exitCode(t, []string{"score", ref}, d)
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

func TestScoreFailsWhenResultIsMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	writeGraderScripts(t, workDir)
	d, out := runDeps(t, workDir, fake.url, scoreConfig(fake.url))

	err := runScore([]string{"ghost"}, d)
	if err == nil {
		t.Fatal("score = nil, want an error for a missing result")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing result", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing result", out.String())
	}
	if got := countLines(t, filepath.Join(workDir, "build.log")); got != 0 {
		t.Errorf("build ran %d times, want none for a missing result", got)
	}
	if got := countLines(t, filepath.Join(workDir, "grades.log")); got != 0 {
		t.Errorf("grader ran %d times, want none for a missing result", got)
	}
	if got := fake.scoreRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v scores, want none for a missing result", got)
	}
}

func TestScoreReportsMissingResultOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	fake.seedDatum("acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	d, _ := runDeps(t, workDir, fake.url, scoreConfig(fake.url))

	code, out, errOut := exitCode(t, []string{"score", "ghost"}, d)
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

func TestScoreFailsWhenGraderWritesNoMarker(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "grade.sh", "echo no marker here\n")
	d, _ := runDeps(t, workDir, fake.url, scoreConfig(fake.url))

	err := runScore([]string{ref}, d)
	if err == nil {
		t.Fatal("score = nil, want an error when the output carries no marker")
	}
	if !strings.Contains(err.Error(), "marker") {
		t.Errorf("error = %q, want it to report the missing marker", err)
	}
	if got := fake.scoreRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v scores, want none without a marker", got)
	}
}

func TestScoreFailsWhenScoreOutputHasNoDictionary(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	writeScript(t, workDir, "build.sh", "echo build >> build.log\n")
	writeScript(t, workDir, "grade.sh", "printf '__abnch__\\n{\"inputTokens\":1}\\n'\n")
	d, _ := runDeps(t, workDir, fake.url, scoreConfig(fake.url))

	err := runScore([]string{ref}, d)
	if err == nil {
		t.Fatal("score = nil, want an error when the score output carries no dictionary")
	}
	if !strings.Contains(err.Error(), "score") {
		t.Errorf("error = %q, want it to report the missing score dictionary", err)
	}
	if got := fake.scoreRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v scores, want none without a dictionary", got)
	}
}

func TestScoreModelFlagOverridesTheTaskModel(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	writeGraderScripts(t, workDir)
	config := "url: " + fake.url + "\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n    variant: high\n" +
		"  opus:\n    provider: anthropic\n    name: claude-opus\n    variant: low\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n    grader:\n      runCmd: sh grade.sh\n"
	d, _ := runDeps(t, workDir, fake.url, config)

	if err := runScore([]string{ref, "--model", "opus"}, d); err != nil {
		t.Fatalf("score = %v, want no error", err)
	}

	raw, err := os.ReadFile(filepath.Join(workDir, "inputs.log"))
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Model struct {
			Provider string `json:"provider"`
			Name     string `json:"name"`
			Variant  string `json:"variant"`
		} `json:"model"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(raw), &input); err != nil {
		t.Fatalf("stdin is not JSON: %v\n%s", err, raw)
	}
	if input.Model.Provider != "anthropic" || input.Model.Name != "claude-opus" || input.Model.Variant != "low" {
		t.Errorf("model = %+v, want the flag's model", input.Model)
	}
}
