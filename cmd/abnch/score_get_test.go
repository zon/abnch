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

// scoreAPI is one score as the service API returns it. The fake service serves
// it so a get score command reads the same shape the service writes.
type scoreAPI struct {
	Ref          string             `json:"ref"`
	Task         string             `json:"task"`
	Test         string             `json:"test"`
	Result       string             `json:"result"`
	Model        store.Model        `json:"model"`
	Grader       store.Grader       `json:"grader"`
	Body         map[string]float64 `json:"body"`
	Logs         string             `json:"logs"`
	InputTokens  int                `json:"inputTokens"`
	OutputTokens int                `json:"outputTokens"`
	TokenCost    float64            `json:"tokenCost"`
	Duration     float64            `json:"duration"`
}

// seedScore stores a score in the fake service and returns its ref.
func (f *fakeService) seedScore(s store.Score) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref := store.ScoreRef(s)
	s.Ref = ref
	f.scores[s.Org+"/"+s.Repo+"/"+ref] = s
	return ref
}

// serveListScores writes the repo's score refs as a JSON array, or the service
// error response when the repo does not exist.
func (f *fakeService) serveListScores(w http.ResponseWriter, org, repo string) {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	prefix := org + "/" + repo + "/"
	refs := make([]string, 0)
	for key := range f.scores {
		if strings.HasPrefix(key, prefix) {
			refs = append(refs, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(refs)
	httpx.WriteJSON(w, http.StatusOK, refs)
}

// serveGetScore writes one score as JSON, or the service error response when
// the repo or score does not exist.
func (f *fakeService) serveGetScore(w http.ResponseWriter, org, repo, ref string) {
	if _, ok := f.repos[org+"/"+repo]; !ok {
		httpx.WriteError(w, http.StatusNotFound, "repo not found")
		return
	}
	sc, ok := f.scores[org+"/"+repo+"/"+ref]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "score not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, scoreAPI{
		Ref: sc.Ref, Task: sc.Task, Test: sc.Test, Result: sc.Result,
		Model: sc.Model, Grader: sc.Grader,
		Body: sc.Body, Logs: sc.Logs,
		InputTokens: sc.InputTokens, OutputTokens: sc.OutputTokens,
		TokenCost: sc.TokenCost, Duration: sc.Duration,
	})
}

// seedWidgetScores seeds the repo `acme/widgets` with the given scores and
// returns their refs.
func seedWidgetScores(t *testing.T, fake *fakeService, scores ...store.Score) []string {
	t.Helper()
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	refs := make([]string, 0, len(scores))
	for _, s := range scores {
		s.Org, s.Repo = "acme", "widgets"
		refs = append(refs, fake.seedScore(s))
	}
	return refs
}

func TestGetScoresListsScoresAsYAML(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	refs := seedWidgetScores(t, fake,
		store.Score{Task: "fix-bug", Test: "basic", Result: "aaaa", Body: map[string]float64{"correctness": 1}},
		store.Score{Task: "fix-bug", Test: "basic", Result: "aaaa", Body: map[string]float64{"correctness": 0}},
	)
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"scores"}, d); err != nil {
		t.Fatalf("get scores = %v, want no error", err)
	}

	var listed []string
	if err := yaml.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(listed) != len(refs) || !contains(listed, refs[0]) || !contains(listed, refs[1]) {
		t.Errorf("listed %v, want the refs %v", listed, refs)
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/scores") {
		t.Errorf("requests = %v, want the score list request", fake.methods())
	}
}

func TestGetScoresListsOnlyTheRepoScores(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	want := seedWidgetScores(t, fake,
		store.Score{Task: "fix-bug", Test: "basic", Result: "aaaa", Body: map[string]float64{"correctness": 1}},
	)[0]
	fake.seedScore(store.Score{Org: "acme", Repo: "gadgets", Task: "fix-bug", Test: "basic", Result: "aaaa", Body: map[string]float64{"correctness": 1}})
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"scores"}, d); err != nil {
		t.Fatalf("get scores = %v, want no error", err)
	}

	var listed []string
	if err := yaml.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(listed) != 1 || listed[0] != want {
		t.Errorf("listed %v, want only %s", listed, want)
	}
}

func TestGetScoresListsEmptyRepoAsEmptyList(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"scores"}, d); err != nil {
		t.Fatalf("get scores = %v, want no error", err)
	}

	var listed []string
	if err := yaml.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, out.String())
	}
	if len(listed) != 0 {
		t.Errorf("listed %v, want none", listed)
	}
}

func TestGetScoresFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"scores"}, d)
	if err == nil {
		t.Fatal("get scores = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetScoresSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := getDeps(t, workDir, service)

	err := runGet([]string{"scores"}, d)
	if err == nil {
		t.Fatal("get scores = nil, want an error when the service rejects the read")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestGetScoresSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"scores"}, d); err == nil {
		t.Fatal("get scores = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}

func TestGetScorePrintsScoreAsJSON(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	ref := seedWidgetScores(t, fake, store.Score{
		Task: "fix-bug", Test: "basic", Result: "aaaa",
		Body: map[string]float64{"correctness": 1, "style": 0.5},
		Logs: "graded\n", InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 0.5,
	})[0]
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"score", ref}, d); err != nil {
		t.Fatalf("get score = %v, want no error", err)
	}

	var got struct {
		Ref          string             `json:"ref"`
		Task         string             `json:"task"`
		Test         string             `json:"test"`
		Result       string             `json:"result"`
		Body         map[string]float64 `json:"body"`
		Logs         string             `json:"logs"`
		InputTokens  int                `json:"inputTokens"`
		OutputTokens int                `json:"outputTokens"`
		TokenCost    float64            `json:"tokenCost"`
		Duration     float64            `json:"duration"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got.Ref != ref || got.Task != "fix-bug" || got.Test != "basic" || got.Result != "aaaa" {
		t.Errorf("score = %+v, want the stored score", got)
	}
	if got.Body["correctness"] != 1 || got.Body["style"] != 0.5 {
		t.Errorf("body = %v, want the stored body", got.Body)
	}
	if got.Logs != "graded\n" || got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 || got.Duration != 0.5 {
		t.Errorf("score = %+v, want the stored logs and metrics", got)
	}
	if !contains(fake.methods(), "GET /repos/acme/widgets/scores/"+ref) {
		t.Errorf("requests = %v, want the score request", fake.methods())
	}
}

func TestGetScorePrintsLinkedModelAndGrader(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	wantModel := store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	wantGrader := store.Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456",
		Branch: "main", Version: "2.0.0", Uncommitted: true,
	}
	ref := seedWidgetScores(t, fake, store.Score{
		Task: "fix-bug", Test: "basic", Result: "aaaa",
		Body:  map[string]float64{"correctness": 1},
		Model: wantModel, Grader: wantGrader,
	})[0]
	d, out := getDeps(t, workDir, fake.url)

	if err := runGet([]string{"score", ref}, d); err != nil {
		t.Fatalf("get score = %v, want no error", err)
	}

	var got struct {
		Model  store.Model  `json:"model"`
		Grader store.Grader `json:"grader"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got.Model != wantModel {
		t.Errorf("model = %+v, want the linked model %+v", got.Model, wantModel)
	}
	if got.Grader != wantGrader {
		t.Errorf("grader = %+v, want the linked grader %+v", got.Grader, wantGrader)
	}
}

func TestGetScoreFailsWhenScoreMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"score", "ghost"}, d)
	if err == nil {
		t.Fatal("get score = nil, want an error for a missing score")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the missing score", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing score", out.String())
	}
}

func TestGetScoreReportsMissingScoreOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	fake.seedRepo("acme", "widgets", "git@github.com:acme/widgets.git")
	d, _ := getDeps(t, workDir, fake.url)

	code, out, errOut := exitCode(t, []string{"get", "score", "ghost"}, d)
	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a missing score")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a missing score", out)
	}
	if !strings.Contains(errOut, "ghost") {
		t.Errorf("stderr = %q, want the missing score named", errOut)
	}
}

func TestGetScoreFailsWhenRepoMissing(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	fake := newFakeService(t)
	d, out := getDeps(t, workDir, fake.url)

	err := runGet([]string{"score", "ghost"}, d)
	if err == nil {
		t.Fatal("get score = nil, want an error for a missing repo")
	}
	if !strings.Contains(err.Error(), "acme/widgets") || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %q, want the repo named and reported as unregistered", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing for a missing repo", out.String())
	}
}

func TestGetScoreSurfacesServiceError(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d, _ := getDeps(t, workDir, service)

	err := runGet([]string{"score", "ghost"}, d)
	if err == nil {
		t.Fatal("get score = nil, want an error when the service rejects the read")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error = %q, want the service error message", err)
	}
}

func TestGetScoreSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fake := newFakeService(t)
	d, _ := getDeps(t, t.TempDir(), fake.url)

	if err := runGet([]string{"score", "ghost"}, d); err == nil {
		t.Fatal("get score = nil, want an error outside a Git repo")
	}
	if fake.count() != 0 {
		t.Errorf("requests = %d, want none without a repo", fake.count())
	}
}
