package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
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
dictionary:
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
  "dictionary": {"correctness": 1, "style": 0.5},
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
	if got.Dictionary["correctness"] != 1 || got.Dictionary["style"] != 0.5 {
		t.Errorf("dictionary = %v, want the input dictionary", got.Dictionary)
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
	if got.Dictionary["correctness"] != 1 || got.Model.Name != "claude-sonnet" || got.Grader.Commit != "def456" {
		t.Errorf("score = %+v, want the file's dictionary, model, and grader", got)
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
	if got.Dictionary["correctness"] != 1 || got.Model.Name != "claude-sonnet" || got.Grader.Commit != "def456" {
		t.Errorf("score = %+v, want the file's dictionary, model, and grader", got)
	}
}

func TestAddScoreRecordsModelAndGraderInConfig(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, scoreInputYAML)

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
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
	grader := cfg.Tasks["fix-bug"].Grader
	if grader == nil {
		t.Fatal("the task has no grader, want the score's grader")
	}
	if grader.Name != "unit-grader" || grader.Stub != "unit" || grader.Commit != "def456" || grader.Branch != "main" || grader.Version != "2.0.0" || !grader.Uncommitted {
		t.Errorf("grader = %+v, want the input grader", *grader)
	}
}

func TestAddScoreRecordsProvenanceThatGetCommandsRead(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, scoreInputYAML)

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	modelDeps, modelOut := getConfigDeps(t, workDir)
	if err := runGet([]string{"model", "claude-sonnet"}, modelDeps); err != nil {
		t.Fatalf("get model = %v, want the score's model", err)
	}
	var model modelView
	if err := yaml.Unmarshal(modelOut.Bytes(), &model); err != nil {
		t.Fatalf("model output is not YAML: %v\n%s", err, modelOut.String())
	}
	if model.Stub != "claude-sonnet" || model.Name != "claude-sonnet" || model.Provider != "anthropic" || model.Variant != "high" {
		t.Errorf("model = %+v, want the score's model under its derived stub", model)
	}

	modelsDeps, modelsOut := getConfigDeps(t, workDir)
	if err := runGet([]string{"models"}, modelsDeps); err != nil {
		t.Fatalf("get models = %v, want no error", err)
	}
	var models []modelView
	if err := yaml.Unmarshal(modelsOut.Bytes(), &models); err != nil {
		t.Fatalf("models output is not YAML: %v\n%s", err, modelsOut.String())
	}
	if len(models) != 1 || models[0].Stub != "claude-sonnet" {
		t.Errorf("models = %+v, want the score's model listed", models)
	}

	graderDeps, graderOut := getConfigDeps(t, workDir)
	if err := runGet([]string{"grader", "fix-bug"}, graderDeps); err != nil {
		t.Fatalf("get grader = %v, want the score's grader", err)
	}
	var grader configGrader
	if err := yaml.Unmarshal(graderOut.Bytes(), &grader); err != nil {
		t.Fatalf("grader output is not YAML: %v\n%s", err, graderOut.String())
	}
	if grader.Name != "unit-grader" || grader.Stub != "unit" || grader.Commit != "def456" || grader.Branch != "main" || grader.Version != "2.0.0" || !grader.Uncommitted {
		t.Errorf("grader = %+v, want the score's grader", grader)
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
	cfg, err := loadConfig(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks["fix-bug"].Grader.Uncommitted {
		t.Error("config grader is uncommitted, want it defaulted to false")
	}
}

func TestAddScoreKeepsOtherConfig(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, scoreInputYAML)
	writeCLIConfig(t, workDir, "url: "+fake.url+"\n"+
		"models:\n  haiku:\n    provider: anthropic\n    name: claude-haiku\n"+
		"tasks:\n  other:\n    grader:\n      name: other-grader\n      stub: other\n      commit: 123abc\n      branch: main\n")

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
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
	if cfg.Tasks["other"].Grader == nil || cfg.Tasks["other"].Grader.Commit != "123abc" {
		t.Errorf("tasks = %+v, want the existing task kept", cfg.Tasks)
	}
}

func TestAddScoreKeepsOtherGraderKeys(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	d, _ := addDataDeps(t, workDir, fake.url, scoreInputYAML)
	writeCLIConfig(t, workDir, "url: "+fake.url+"\n"+
		"tasks:\n  fix-bug:\n    grader:\n      label: judge-one\n      note: keep-me\n")

	if err := runAdd([]string{"score", result}, d); err != nil {
		t.Fatalf("add score = %v, want no error", err)
	}

	data, err := os.ReadFile(configPath(workDir))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Tasks map[string]struct {
			Grader map[string]any `yaml:"grader"`
		} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	grader := raw.Tasks["fix-bug"].Grader
	if grader["label"] != "judge-one" || grader["note"] != "keep-me" {
		t.Errorf("grader = %+v, want the existing keys kept", grader)
	}
	if grader["name"] != "unit-grader" || grader["commit"] != "def456" {
		t.Errorf("grader = %+v, want the record written", grader)
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
	if cfg, _ := loadConfig(workDir); len(cfg.Models) != 0 {
		t.Errorf("models = %+v, want none recorded", cfg.Models)
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

func TestAddScoreFailsWhenInputHasNoDictionary(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	result := seedScoreResult(t, fake, `{"prompt":"hi"}`, `{"answer":"hi"}`)
	input := strings.Replace(scoreInputYAML, "dictionary:\n  correctness: 1\n  style: 0.5\n", "", 1)
	d, out := addDataDeps(t, workDir, fake.url, input)

	err := runAdd([]string{"score", result}, d)
	if err == nil {
		t.Fatal("add score = nil, want an error without a dictionary")
	}
	if !strings.Contains(err.Error(), "dictionary") {
		t.Errorf("error = %q, want it to name the dictionary", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing without a dictionary", out.String())
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a dictionary", fake.count())
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
	cfg, _ := loadConfig(workDir)
	if len(cfg.Models) != 0 || cfg.Tasks["fix-bug"].Grader != nil {
		t.Errorf("config = %+v, want no records for a failed write", cfg)
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
	if cfg, _ := loadConfig(workDir); len(cfg.Models) != 0 {
		t.Errorf("models = %+v, want none recorded for a rejected write", cfg.Models)
	}
}
