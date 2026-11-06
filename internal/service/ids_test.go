package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/store"
)

// apiIDStore returns a store whose records carry ids, including the records a
// result and score embed, so a response that leaks an id fails its test.
func apiIDStore() *fakeDB {
	st := scoreStore()
	st.repos[0].ID = 1
	st.tasks[0].ID = 2
	st.tasks[0].RepoID = 1
	st.tests[0].ID = 3
	st.tests[0].TaskID = 2
	st.data[0].ID = 4
	st.data[0].TestID = 3

	model := store.Model{ID: 5, Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	agent := store.Agent{
		ID: 6, TaskID: 2,
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}
	st.results = []store.Result{{
		ID: 7, DatumID: 4, ModelID: 5, AgentID: 6,
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: st.data[0].Ref,
		Ref: "aaa111", Model: model, Agent: agent, Body: `{"answer":"hi"}`,
	}}

	grader := store.Grader{
		ID: 8, TestID: 3,
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}
	st.scores = []store.Score{{
		ID: 9, ResultID: 7, ModelID: 5, GraderID: 8,
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Result: "aaa111",
		Ref: "bbb222", Model: model, Grader: grader, Body: map[string]float64{"correctness": 1},
	}}
	return st
}

// TestAPIResponsesOmitIDs checks every read response identifies its records by
// stubs and refs, never by the ids the store holds.
func TestAPIResponsesOmitIDs(t *testing.T) {
	st := apiIDStore()
	base := startServiceWithStore(t, st, auth.RoleReader)
	dataPath := "repos/acme/widgets/tasks/fix-bug/tests/basic/data/" + st.data[0].Ref

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "repos", path: "repos"},
		{name: "repo", path: "repos/acme/widgets"},
		{name: "tasks", path: "repos/acme/widgets/tasks"},
		{name: "task", path: "repos/acme/widgets/tasks/fix-bug"},
		{name: "tests", path: "repos/acme/widgets/tasks/fix-bug/tests"},
		{name: "test", path: "repos/acme/widgets/tasks/fix-bug/tests/basic"},
		{name: "data", path: "repos/acme/widgets/tasks/fix-bug/tests/basic/data"},
		{name: "datum", path: dataPath},
		{name: "results", path: "repos/acme/widgets/results"},
		{name: "result", path: "repos/acme/widgets/results/aaa111"},
		{name: "scores", path: "repos/acme/widgets/scores"},
		{name: "score", path: "repos/acme/widgets/scores/bbb222"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(base + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			var body any
			dec := json.NewDecoder(resp.Body)
			dec.UseNumber()
			if err := dec.Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			assertNoIDs(t, body)
		})
	}
}

// assertNoIDs reports an id key anywhere in a decoded JSON value.
func assertNoIDs(t *testing.T, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if isIDKey(key) {
				t.Errorf("response carries id key %q", key)
			}
			assertNoIDs(t, child)
		}
	case []any:
		for _, child := range v {
			assertNoIDs(t, child)
		}
	}
}

// isIDKey reports whether a JSON key names a record id.
func isIDKey(key string) bool {
	return strings.EqualFold(key, "id") ||
		strings.HasSuffix(key, "Id") ||
		strings.HasSuffix(key, "_id")
}

// TestAPIRequestBodiesIgnoreIDs checks an id in a write body does not identify a
// record, so the service still builds its records from the route and the body's
// stub, ref, and value fields.
func TestAPIRequestBodiesIgnoreIDs(t *testing.T) {
	t.Run("repo", func(t *testing.T) {
		st := &fakeDB{}
		base := startServiceWithStore(t, st, auth.RoleAdmin)

		resp := sendRepoPut(t, base, "repos/acme/widgets",
			`{"id":100,"org":"other","stub":"gadgets","url":"https://github.com/acme/widgets"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		stored := st.repos[0]
		if stored.ID != 0 || stored.Org != "acme" || stored.Stub != "widgets" {
			t.Errorf("stored repo = %#v, want the route identity and no body id", stored)
		}
	})

	t.Run("task", func(t *testing.T) {
		st := &fakeDB{repos: []store.Repo{{ID: 1, Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
		base := startServiceWithStore(t, st, auth.RoleAdmin)

		resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug",
			`{"id":100,"repoId":100,"org":"other","repo":"gadgets","stub":"other","name":"Fix a bug","description":"A broken widget"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		stored := st.tasks[0]
		if stored.ID != 0 || stored.RepoID != 0 || stored.Org != "acme" || stored.Repo != "widgets" || stored.Stub != "fix-bug" {
			t.Errorf("stored task = %#v, want the route identity and no body id", stored)
		}
	})

	t.Run("test", func(t *testing.T) {
		st := testStore()
		base := startServiceWithStore(t, st, auth.RoleAdmin)

		resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
			`{"id":100,"taskId":100,"org":"other","repo":"gadgets","task":"other","stub":"other","name":"Basic","version":"1"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		stored := st.tests[0]
		if stored.ID != 0 || stored.TaskID != 0 || stored.Org != "acme" || stored.Repo != "widgets" || stored.Task != "fix-bug" || stored.Stub != "basic" {
			t.Errorf("stored test = %#v, want the route identity and no body id", stored)
		}
	})

	t.Run("result", func(t *testing.T) {
		st := resultStore()
		base := startServiceWithStore(t, st, auth.RoleAdmin)
		body := `{"id":100,"datumId":100,"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
			`"model":{"id":100,"modelId":100,"name":"claude-sonnet","provider":"anthropic","variant":"high"},` +
			`"agent":{"id":200,"taskId":200,"name":"claude-code","stub":"claude","commit":"abc123","branch":"main"},` +
			`"body":{"answer":"hi"}}`

		resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		stored := st.results[0]
		if stored.ID != 0 || stored.DatumID != 0 || stored.ModelID != 0 || stored.AgentID != 0 {
			t.Errorf("stored result = %#v, want no body id", stored)
		}
		if stored.Model.ID != 0 || stored.Agent.ID != 0 || stored.Agent.TaskID != 0 {
			t.Errorf("stored result links = model %#v, agent %#v, want no body id", stored.Model, stored.Agent)
		}
		if len(st.models) != 1 || st.models[0].ID != 0 {
			t.Errorf("stored models = %#v, want one model with no body id", st.models)
		}
		if len(st.agents) != 1 || st.agents[0].ID != 0 || st.agents[0].TaskID != 0 {
			t.Errorf("stored agents = %#v, want one agent with no body id", st.agents)
		}
	})

	t.Run("score", func(t *testing.T) {
		st := scoreStore()
		base := startServiceWithStore(t, st, auth.RoleAdmin)
		body := `{"id":100,"resultId":100,"task":"fix-bug","test":"basic","result":"aaa111",` +
			`"model":{"id":100,"modelId":100,"name":"claude-sonnet","provider":"anthropic","variant":"high"},` +
			`"grader":{"id":200,"testId":200,"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},` +
			`"body":{"correctness":1}}`

		resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		stored := st.scores[0]
		if stored.ID != 0 || stored.ResultID != 0 || stored.ModelID != 0 || stored.GraderID != 0 {
			t.Errorf("stored score = %#v, want no body id", stored)
		}
		if stored.Model.ID != 0 || stored.Grader.ID != 0 || stored.Grader.TestID != 0 {
			t.Errorf("stored score links = model %#v, grader %#v, want no body id", stored.Model, stored.Grader)
		}
		if len(st.models) != 1 || st.models[0].ID != 0 {
			t.Errorf("stored models = %#v, want one model with no body id", st.models)
		}
		if len(st.graders) != 1 || st.graders[0].ID != 0 || st.graders[0].TestID != 0 {
			t.Errorf("stored graders = %#v, want one grader with no body id", st.graders)
		}
	})
}
