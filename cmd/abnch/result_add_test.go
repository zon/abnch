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
json:
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
  "json": {"answer": "hi"},
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
	if got.JSON != `{"answer":"hi"}` {
		t.Errorf("result JSON = %q, want the input json", got.JSON)
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
	if got.JSON != `{"answer":"hi"}` || got.Model.Name != "claude-sonnet" || got.Agent.Commit != "abc123" {
		t.Errorf("result = %+v, want the file's JSON, model, and agent", got)
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
	if got.JSON != `{"answer":"hi"}` || got.Model.Name != "claude-sonnet" || got.Agent.Commit != "abc123" {
		t.Errorf("result = %+v, want the file's JSON, model, and agent", got)
	}
}

func TestAddResultRecordsModelAndAgentInConfig(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, resultInputYAML)

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != fake.url {
		t.Errorf("config url = %q, want it kept as %q", cfg.URL, fake.url)
	}
	model, ok := cfg.Models["claude-sonnet"]
	if !ok {
		t.Fatalf("models = %+v, want the model under its name", cfg.Models)
	}
	if model.Name != "claude-sonnet" || model.Provider != "anthropic" || model.Variant != "high" {
		t.Errorf("model = %+v, want the input model", model)
	}
	agent := cfg.Tasks["fix-bug"].Agent
	if agent == nil {
		t.Fatal("the task has no agent, want the result's agent")
	}
	if agent.Name != "claude-code" || agent.Stub != "claude" || agent.Commit != "abc123" || agent.Branch != "main" || agent.Version != "1.2.3" || !agent.Uncommitted {
		t.Errorf("agent = %+v, want the input agent", *agent)
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
	cfg, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks["fix-bug"].Agent.Uncommitted {
		t.Error("config agent is uncommitted, want it defaulted to false")
	}
}

func TestAddResultKeepsOtherConfig(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, resultInputYAML)
	writeCLIConfig(t, workDir, "url: "+fake.url+"\n"+
		"models:\n  haiku:\n    provider: anthropic\n    name: claude-haiku\n"+
		"tasks:\n  other:\n    model: haiku\n    agent:\n      runCmd: ./other.sh\n")

	if err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d); err != nil {
		t.Fatalf("add result = %v, want no error", err)
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != fake.url {
		t.Errorf("config url = %q, want it kept", cfg.URL)
	}
	if cfg.Models["haiku"].Name != "claude-haiku" {
		t.Errorf("models = %+v, want the existing model kept", cfg.Models)
	}
	if cfg.Tasks["other"].Model != "haiku" || cfg.Tasks["other"].Agent == nil || cfg.Tasks["other"].Agent.RunCmd != "./other.sh" {
		t.Errorf("tasks = %+v, want the existing task kept", cfg.Tasks)
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
	if cfg, _ := loadConfig(workDir); len(cfg.Models) != 0 {
		t.Errorf("models = %+v, want none recorded", cfg.Models)
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

func TestAddResultFailsWhenInputHasNoJSON(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	datum := seedResultDatum(t, fake, `{"prompt":"hi"}`)
	input := strings.Replace(resultInputYAML, "json:\n  answer: hi\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"result", "fix-bug/basic/" + datum}, d)
	if err == nil {
		t.Fatal("add result = nil, want an error without json")
	}
	if !strings.Contains(err.Error(), "json") {
		t.Errorf("error = %q, want it to name json", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing without json", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without json", fake.count())
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
	cfg, _ := loadConfig(workDir)
	if len(cfg.Models) != 0 || cfg.Tasks["fix-bug"].Agent != nil {
		t.Errorf("config = %+v, want no records for a failed write", cfg)
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
	if cfg, _ := loadConfig(workDir); len(cfg.Models) != 0 {
		t.Errorf("models = %+v, want none recorded for a rejected write", cfg.Models)
	}
}
