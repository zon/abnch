package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const scoreInputYAML = `model:
  name: claude-sonnet
  provider: anthropic
  variant: high
grader:
  name: unit-grader
  stub: unit
  commit: def456
  branch: main
  version: 2.0.0
  uncommitted: true
body:
  correctness: 1
  style: 0.5
logs: graded
inputTokens: 12
outputTokens: 3
tokenCost: 0.01
duration: 0.5
`

const scoreInputJSON = `{
  "model": {"name": "claude-sonnet", "provider": "anthropic", "variant": "high"},
  "grader": {"name": "unit-grader", "stub": "unit", "commit": "def456", "branch": "main", "version": "2.0.0", "uncommitted": true},
  "body": {"correctness": 1, "style": 0.5},
  "logs": "graded",
  "inputTokens": 12,
  "outputTokens": 3,
  "tokenCost": 0.01,
  "duration": 0.5
}`

// writeScoreInputFile writes body to name under workDir and returns the stored
// score input path relative to workDir.
func writeScoreInputFile(t *testing.T, workDir, name, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workDir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestAddScoreFromStdinRecordsTheScore(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, out := addDataDeps(t, workDir, fake.url, scoreInputYAML)

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	refs := fake.scoreRefs("acme", "widgets")
	if len(refs) != 1 {
		t.Fatalf("recorded %d scores, want 1", len(refs))
	}
	if printed := strings.TrimSpace(out.String()); printed != refs[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, refs[0])
	}
	got, ok := fake.score("acme", "widgets", refs[0])
	if !ok {
		t.Fatal("the score was not recorded")
	}
	if got.Task != "fix-bug" || got.Test != "basic" || got.Result != result {
		t.Errorf("score identity = %s/%s/%s, want fix-bug/basic/%s", got.Task, got.Test, got.Result, result)
	}
	if got.Body["correctness"] != 1 || got.Body["style"] != 0.5 {
		t.Errorf("body = %v, want the input body", got.Body)
	}
	if got.Logs != "graded" || got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 || got.Duration != 0.5 {
		t.Errorf("score = %+v, want the input logs and metrics", got)
	}
	if got.Model.Name != "claude-sonnet" || got.Model.Provider != "anthropic" || got.Model.Variant != "high" {
		t.Errorf("model = %+v, want the input model", got.Model)
	}
	if got.Grader.Name != "unit-grader" || got.Grader.Stub != "unit" || got.Grader.Commit != "def456" || got.Grader.Branch != "main" || got.Grader.Version != "2.0.0" || !got.Grader.Uncommitted {
		t.Errorf("grader = %+v, want the input grader", got.Grader)
	}
	if !contains(fake.methods(), "POST /repos/acme/widgets/scores") {
		t.Errorf("requests = %v, want the score write", fake.methods())
	}
}

func TestAddScoreFromFileRecordsTheScore(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	file := writeScoreInputFile(t, workDir, "score.json", scoreInputJSON)
	d, out := addDataDeps(t, workDir, fake.url, "")

	if err := runAdd([]string{"score", result, "--file", file}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	refs := fake.scoreRefs("acme", "widgets")
	if len(refs) != 1 {
		t.Fatalf("recorded %d scores, want 1", len(refs))
	}
	if printed := strings.TrimSpace(out.String()); printed != refs[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, refs[0])
	}
	got, _ := fake.score("acme", "widgets", refs[0])
	if got.Body["correctness"] != 1 || got.Model.Name != "claude-sonnet" || got.Grader.Commit != "def456" {
		t.Errorf("score = %+v, want the file's body, model, and grader", got)
	}
}

func TestAddScoreFromYAMLFileRecordsTheScore(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	file := writeScoreInputFile(t, workDir, "score.yaml", scoreInputYAML)
	d, out := addDataDeps(t, workDir, fake.url, "")

	if err := runAdd([]string{"score", result, "--file", file}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	refs := fake.scoreRefs("acme", "widgets")
	if len(refs) != 1 {
		t.Fatalf("recorded %d scores, want 1", len(refs))
	}
	if printed := strings.TrimSpace(out.String()); printed != refs[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, refs[0])
	}
	got, _ := fake.score("acme", "widgets", refs[0])
	if got.Body["correctness"] != 1 || got.Model.Name != "claude-sonnet" || got.Grader.Commit != "def456" {
		t.Errorf("score = %+v, want the file's body, model, and grader", got)
	}
}

func TestAddScoreDefaultsUncommittedToFalse(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML, "  uncommitted: true\n", "", 1)
	d, _ := addDataDeps(t, workDir, fake.url, input)

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	refs := fake.scoreRefs("acme", "widgets")
	got, _ := fake.score("acme", "widgets", refs[0])
	if got.Grader.Uncommitted {
		t.Errorf("grader = %+v, want uncommitted false when the input omits it", got.Grader)
	}
}

func TestAddScoreLeavesConfigUnchanged(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, scoreInputYAML)
	body := "url: " + fake.url + "\n" +
		"other:\n  key: value\n"
	writeCLIConfig(t, workDir, body)

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	after, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("config = %q, want it unchanged as %q", after, body)
	}
}

func TestAddScoreFailsWhenInputHasNoModel(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML, "model:\n  name: claude-sonnet\n  provider: anthropic\n  variant: high\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error without a model")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error = %q, want it to name the model", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing without a model", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a model", fake.count())
	}
}

func TestAddScoreFailsWhenModelMissesARequiredField(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML, "  provider: anthropic\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error for an incomplete model")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error = %q, want it to name the model", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for an incomplete model", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an incomplete model", fake.count())
	}
}

func TestAddScoreFailsWhenInputHasNoGrader(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML,
		"grader:\n  name: unit-grader\n  stub: unit\n  commit: def456\n  branch: main\n  version: 2.0.0\n  uncommitted: true\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error without a grader")
	}
	if !strings.Contains(err.Error(), "grader") {
		t.Errorf("error = %q, want it to name the grader", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing without a grader", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a grader", fake.count())
	}
}

func TestAddScoreFailsWhenGraderMissesARequiredField(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML, "  commit: def456\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error for an incomplete grader")
	}
	if !strings.Contains(err.Error(), "grader") {
		t.Errorf("error = %q, want it to name the grader", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for an incomplete grader", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an incomplete grader", fake.count())
	}
}

func TestAddScoreFailsWhenInputHasNoBody(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML, "body:\n  correctness: 1\n  style: 0.5\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error without a body")
	}
	if !strings.Contains(err.Error(), "body") {
		t.Errorf("error = %q, want it to name the body", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing without a body", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a body", fake.count())
	}
}

func TestAddScoreFailsWhenResultIsMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := addDataDeps(t, workDir, fake.url, scoreInputYAML)

	err := runAdd([]string{"score", "ghost"}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error for a missing result")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing result", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing result", out.String())
	}
	if got := fake.scoreRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v scores, want none for a missing result", got)
	}
}

func TestAddScoreReportsMissingResultOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, _ := addDataDeps(t, workDir, fake.url, scoreInputYAML)

	code, out, errOut := exitCode(t, []string{"add", "score", "ghost"}, d)
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

func TestAddScoreFailsOnInvalidInput(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, out := addDataDeps(t, workDir, fake.url, "model: [\n")

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error for invalid input")
	}
	if !strings.Contains(err.Error(), "invalid score input") {
		t.Errorf("error = %q, want it to report invalid input", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for invalid input", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for invalid input", fake.count())
	}
}

func TestAddScoreSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := addDataDeps(t, workDir, service, scoreInputYAML)

	err := runAdd([]string{"score", "ref"}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error when the service rejects the write")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}
