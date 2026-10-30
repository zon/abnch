package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const resultInputYAML = `model:
  name: claude-sonnet
  provider: anthropic
  variant: high
agent:
  name: claude-code
  stub: claude
  commit: abc123
  branch: main
  version: 1.2.3
  uncommitted: true
body:
  answer: hi
logs: worked
inputTokens: 12
outputTokens: 3
tokenCost: 0.01
duration: 0.5
`

const resultInputJSON = `{
  "model": {"name": "claude-sonnet", "provider": "anthropic", "variant": "high"},
  "agent": {"name": "claude-code", "stub": "claude", "commit": "abc123", "branch": "main", "version": "1.2.3", "uncommitted": true},
  "body": {"answer": "hi"},
  "logs": "worked",
  "inputTokens": 12,
  "outputTokens": 3,
  "tokenCost": 0.01,
  "duration": 0.5
}`

// seedResultDatum seeds the repo, task, test, and datum an add result command
// needs and returns the datum's ref.
func seedResultDatum(t *testing.T, fake *fakeService, body string) string {
	t.Helper()
	seedDataTest(fake)
	return fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
}

// readResultInputFromFile writes body to name under workDir and returns the
// stored result input path relative to workDir.
func writeResultInputFile(t *testing.T, workDir, name, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workDir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestAddResultFromStdinRecordsTheResult(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	d, out := addDataDeps(t, workDir, fake.url, resultInputYAML)

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	refs := fake.resultRefs("acme", "widgets")
	if len(refs) != 1 {
		t.Fatalf("recorded %d results, want 1", len(refs))
	}
	if printed := strings.TrimSpace(out.String()); printed != refs[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, refs[0])
	}
	got, ok := fake.result("acme", "widgets", refs[0])
	if !ok {
		t.Fatal("the result was not recorded")
	}
	if got.Task != "fix-bug" || got.Test != "basic" || got.Data != datum {
		t.Errorf("result identity = %s/%s/%s, want fix-bug/basic/%s", got.Task, got.Test, got.Data, datum)
	}
	if got.Body != `{"answer":"hi"}` {
		t.Errorf("result body = %q, want the input body", got.Body)
	}
	if got.Logs != "worked" || got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 || got.Duration != 0.5 {
		t.Errorf("result = %+v, want the input logs and metrics", got)
	}
	if got.Model.Name != "claude-sonnet" || got.Model.Provider != "anthropic" || got.Model.Variant != "high" {
		t.Errorf("model = %+v, want the input model", got.Model)
	}
	if got.Agent.Name != "claude-code" || got.Agent.Stub != "claude" || got.Agent.Commit != "abc123" || got.Agent.Branch != "main" || got.Agent.Version != "1.2.3" || !got.Agent.Uncommitted {
		t.Errorf("agent = %+v, want the input agent", got.Agent)
	}
	if !contains(fake.methods(), "POST /repos/acme/widgets/results") {
		t.Errorf("requests = %v, want the result write", fake.methods())
	}
}

func TestAddResultFromFileRecordsTheResult(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	file := writeResultInputFile(t, workDir, "result.json", resultInputJSON)
	d, out := addDataDeps(t, workDir, fake.url, "")

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum, "--file", file}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	refs := fake.resultRefs("acme", "widgets")
	if len(refs) != 1 {
		t.Fatalf("recorded %d results, want 1", len(refs))
	}
	if printed := strings.TrimSpace(out.String()); printed != refs[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, refs[0])
	}
	got, _ := fake.result("acme", "widgets", refs[0])
	if got.Body != `{"answer":"hi"}` || got.Model.Name != "claude-sonnet" || got.Agent.Commit != "abc123" {
		t.Errorf("result = %+v, want the file's body, model, and agent", got)
	}
}

func TestAddResultFromYAMLFileRecordsTheResult(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	file := writeResultInputFile(t, workDir, "result.yaml", resultInputYAML)
	d, out := addDataDeps(t, workDir, fake.url, "")

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum, "--file", file}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	refs := fake.resultRefs("acme", "widgets")
	if len(refs) != 1 {
		t.Fatalf("recorded %d results, want 1", len(refs))
	}
	if printed := strings.TrimSpace(out.String()); printed != refs[0] {
		t.Errorf("printed %q, want the recorded ref %q", printed, refs[0])
	}
	got, _ := fake.result("acme", "widgets", refs[0])
	if got.Body != `{"answer":"hi"}` || got.Model.Name != "claude-sonnet" || got.Agent.Commit != "abc123" {
		t.Errorf("result = %+v, want the file's body, model, and agent", got)
	}
}

func TestAddResultDefaultsUncommittedToFalse(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	input := strings.Replace(resultInputYAML, "  uncommitted: true\n", "", 1)
	d, _ := addDataDeps(t, workDir, fake.url, input)

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	refs := fake.resultRefs("acme", "widgets")
	got, _ := fake.result("acme", "widgets", refs[0])
	if got.Agent.Uncommitted {
		t.Errorf("agent = %+v, want uncommitted false when the input omits it", got.Agent)
	}
}

func TestAddResultLeavesConfigUnchanged(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, resultInputYAML)
	body := "url: " + fake.url + "\n" +
		"other:\n  key: value\n"
	writeCLIConfig(t, workDir, body)

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	after, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("config = %q, want it unchanged as %q", after, body)
	}
}

func TestAddResultFailsWhenInputHasNoModel(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	input := strings.Replace(resultInputYAML, "model:\n  name: claude-sonnet\n  provider: anthropic\n  variant: high\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error without a model")
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

func TestAddResultFailsWhenInputHasNoAgent(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	input := strings.Replace(resultInputYAML,
		"agent:\n  name: claude-code\n  stub: claude\n  commit: abc123\n  branch: main\n  version: 1.2.3\n  uncommitted: true\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error without an agent")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error = %q, want it to name the agent", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing without an agent", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without an agent", fake.count())
	}
}

func TestAddResultFailsWhenInputHasNoBody(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	input := strings.Replace(resultInputYAML, "body:\n  answer: hi\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error without a body")
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

func TestAddResultFailsWhenDatumIsMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, out := addDataDeps(t, workDir, fake.url, resultInputYAML)

	err := runAdd([]string{"result", "fix-bug/basic/ghost"}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error for a missing datum")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing datum", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing datum", out.String())
	}
	if got := fake.resultRefs("acme", "widgets"); len(got) != 0 {
		t.Errorf("recorded %v results, want none for a missing datum", got)
	}
}

func TestAddResultReportsMissingDatumOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := addDataDeps(t, workDir, fake.url, resultInputYAML)

	code, out, errOut := exitCode(t, []string{"add", "result", "fix-bug/basic/ghost"}, d)
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

func TestAddResultFailsOnInvalidInput(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	d, out := addDataDeps(t, workDir, fake.url, "model: [\n")

	err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error for invalid input")
	}
	if !strings.Contains(err.Error(), "invalid result input") {
		t.Errorf("error = %q, want it to report invalid input", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for invalid input", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for invalid input", fake.count())
	}
}

func TestAddResultFailsOnInvalidPath(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	seedDataTest(fake)
	d, _ := addDataDeps(t, workDir, fake.url, resultInputYAML)

	for _, path := range []string{"fix-bug", "fix-bug/basic", "fix-bug/basic/ref/extra", "fix-bug//ref"} {
		if err := runAdd([]string{"result", path}, d); err == nil {
			t.Errorf("add result %q = nil, want an error", path)
		}
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none for an invalid path", fake.count())
	}
}

func TestAddResultSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := addDataDeps(t, workDir, service, resultInputYAML)

	err := runAdd([]string{"result", "fix-bug/basic/ref"}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error when the service rejects the write")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}
