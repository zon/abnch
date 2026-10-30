package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// serveAddResult stores the result the body carries for its datum and returns
// its ref, mirroring the service: the same body for the same datum returns the
// existing ref as 200, and a new result returns 201. A malformed body, or a
// missing repo, task, test, or datum, writes the service error response.
func (f *fakeService) serveAddResult(w http.ResponseWriter, org, repo string, body []byte) {
	var req resultRequest
	if err := json.Unmarshal(body, &req); err != nil || len(req.Body) == 0 {
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
		Body: string(req.Body), Logs: req.Logs,
		InputTokens: req.InputTokens, OutputTokens: req.OutputTokens,
		TokenCost: req.TokenCost, Duration: req.Duration,
	}
	if req.Model != nil {
		result.Model = store.Model{Name: req.Model.Name, Provider: req.Model.Provider, Variant: req.Model.Variant}
	}
	if req.Agent != nil {
		result.Agent = store.Agent{
			Org: org, Repo: repo, Task: req.Task,
			Name: req.Agent.Name, Stub: req.Agent.Stub, Commit: req.Agent.Commit,
			Branch: req.Agent.Branch, Version: req.Agent.Version, Uncommitted: req.Agent.Uncommitted,
		}
	}
	ref := store.ResultRef(result)
	result.Ref = ref
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

// serveAddScore stores the score the body carries for its result and returns its
// ref, mirroring the service: the same body for the same result returns the
// existing ref as 200, and a new score returns 201. A malformed body, or a
// missing repo, task, test, or result, writes the service error response.
func (f *fakeService) serveAddScore(w http.ResponseWriter, org, repo string, body []byte) {
	var req scoreRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Body == nil {
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
	result, ok := f.results[org+"/"+repo+"/"+req.Result]
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "result not found")
		return
	}
	if result.Task != req.Task || result.Test != req.Test {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "score task and test must match its result")
		return
	}

	score := store.Score{
		Org: org, Repo: repo, Task: req.Task, Test: req.Test, Result: req.Result,
		Body: req.Body, Logs: req.Logs,
		InputTokens: req.InputTokens, OutputTokens: req.OutputTokens,
		TokenCost: req.TokenCost, Duration: req.Duration,
	}
	if req.Model != nil {
		score.Model = store.Model{Name: req.Model.Name, Provider: req.Model.Provider, Variant: req.Model.Variant}
	}
	if req.Grader != nil {
		score.Grader = store.Grader{
			Org: org, Repo: repo, Task: req.Task, Test: req.Test,
			Name: req.Grader.Name, Stub: req.Grader.Stub, Commit: req.Grader.Commit,
			Branch: req.Grader.Branch, Version: req.Grader.Version, Uncommitted: req.Grader.Uncommitted,
		}
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
func seedScoreResult(t *testing.T, fake *fakeService, body, resultBody string) string {
	t.Helper()
	seedDataTest(fake)
	datum := fake.seedDatum("acme", "widgets", "fix-bug", "basic", body)
	return fake.seedResult(store.Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: datum, Body: resultBody,
	})
}
