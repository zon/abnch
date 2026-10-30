package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/store"
)

// resultRefBody mirrors the JSON body the API returns for an added result.
type resultRefBody struct {
	Ref string `json:"ref"`
}

// resultStore returns a store holding the repo, task, test, and datum a result
// write needs.
func resultStore() *fakeDB {
	st := dataStore()
	body := []byte(`{"prompt":"hi"}`)
	st.data = []store.Datum{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Ref: store.DatumRef(body), JSON: string(body),
	}}
	return st
}

// TestAddResultStoresResult checks POST /repos/{org}/{repo}/results stores the
// result the body carries for its datum and returns 201 with its ref.
func TestAddResultStoresResult(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
		`"json":{"answer":"hi"},"logs":"worked","inputTokens":12,"outputTokens":3,"tokenCost":0.01,"duration":1.5,"local":"main"}`

	resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got resultRefBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(st.results) != 1 {
		t.Fatalf("store holds %d results, want 1", len(st.results))
	}
	stored := st.results[0]
	if stored.Org != "acme" || stored.Repo != "widgets" || stored.Task != "fix-bug" || stored.Test != "basic" || stored.Data != st.data[0].Ref {
		t.Errorf("stored result identity = %#v, want the route and body stubs", stored)
	}
	if stored.Ref != got.Ref {
		t.Errorf("stored ref = %q, want the returned ref %q", stored.Ref, got.Ref)
	}
	if stored.JSON != `{"answer":"hi"}` || stored.Logs != "worked" {
		t.Errorf("stored result = %#v, want the body's JSON and logs", stored)
	}
	if stored.InputTokens != 12 || stored.OutputTokens != 3 || stored.TokenCost != 0.01 || stored.Duration != 1.5 || stored.Local != "main" {
		t.Errorf("stored result = %#v, want the body's metrics and local branch", stored)
	}
}

// TestAddResultCreatesAndReusesModel checks a result write creates the model it
// carries, links it to the result, and reuses it for the same model values.
func TestAddResultCreatesAndReusesModel(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
		`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},"json":{"answer":"hi"}}`
	want := store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}

	for i, wantStatus := range []int{http.StatusCreated, http.StatusOK} {
		resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Fatalf("post %d status = %d, want %d", i, resp.StatusCode, wantStatus)
		}
	}

	if len(st.models) != 1 {
		t.Fatalf("store holds %d models, want 1", len(st.models))
	}
	if st.models[0] != want {
		t.Errorf("stored model = %#v, want %#v", st.models[0], want)
	}
	if len(st.results) != 1 {
		t.Fatalf("store holds %d results, want 1", len(st.results))
	}
	if st.results[0].Model != want {
		t.Errorf("result model = %#v, want %#v", st.results[0].Model, want)
	}
}

// TestAddResultDifferentModelVariantDifferentRecord checks a result carrying the
// same model name and provider with another variant links to a different model
// record.
func TestAddResultDifferentModelVariantDifferentRecord(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/results"

	for _, variant := range []string{"high", "low"} {
		body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
			`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"` + variant + `"},"json":{"answer":"hi"}}`
		resp := sendResultPost(t, base, path, body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("variant %s status = %d, want %d", variant, resp.StatusCode, http.StatusCreated)
		}
	}

	if len(st.models) != 2 {
		t.Errorf("store holds %d models, want 2", len(st.models))
	}
	if len(st.results) != 2 {
		t.Errorf("store holds %d results, want 2", len(st.results))
	}
}

// TestAddResultRejectsIncompleteModel checks a result carrying a model without a
// name or provider returns 422 and creates neither the result nor the model.
func TestAddResultRejectsIncompleteModel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model string
	}{
		{name: "missing name", model: `{"provider":"anthropic","variant":"high"}`},
		{name: "missing provider", model: `{"name":"claude-sonnet","variant":"high"}`},
		{name: "only variant", model: `{"variant":"high"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","model":` + tc.model + `,"json":{"answer":"hi"}}`

			resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.models) != 0 {
				t.Errorf("store holds %d models, want none", len(st.models))
			}
			if len(st.results) != 0 {
				t.Errorf("store holds %d results, want none", len(st.results))
			}
		})
	}
}

// TestAddResultCreatesAndReusesAgent checks a result write creates the agent it
// carries, associates it with the result's task, defaults uncommitted to false,
// and reuses it for the same agent values.
func TestAddResultCreatesAndReusesAgent(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/results"
	body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
		`"agent":{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main"},"json":{"answer":"hi"}}`
	want := store.Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}

	for i, wantStatus := range []int{http.StatusCreated, http.StatusOK} {
		resp := sendResultPost(t, base, path, body)
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Fatalf("post %d status = %d, want %d", i, resp.StatusCode, wantStatus)
		}
	}

	if len(st.agents) != 1 {
		t.Fatalf("store holds %d agents, want 1", len(st.agents))
	}
	if st.agents[0] != want {
		t.Errorf("stored agent = %#v, want %#v", st.agents[0], want)
	}
	if len(st.results) != 1 {
		t.Fatalf("store holds %d results, want 1", len(st.results))
	}
	if st.results[0].Agent != want {
		t.Errorf("result agent = %#v, want %#v", st.results[0].Agent, want)
	}

	other := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
		`"agent":{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main"},"json":{"answer":"bye"}}`
	resp := sendResultPost(t, base, path, other)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("second result status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if len(st.agents) != 1 {
		t.Errorf("store holds %d agents, want the reused agent", len(st.agents))
	}
	if len(st.results) != 2 {
		t.Errorf("store holds %d results, want 2", len(st.results))
	}
}

// TestAddResultDifferentAgentDifferentRecord checks a result carrying an agent
// that differs in an identity field links to a different agent record, and the
// same JSON from a different agent is a different result.
func TestAddResultDifferentAgentDifferentRecord(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
	}{
		{name: "different stub", agent: `{"name":"claude-code","stub":"other","commit":"abc123","branch":"main"}`},
		{name: "different commit", agent: `{"name":"claude-code","stub":"claude","commit":"def456","branch":"main"}`},
		{name: "different branch", agent: `{"name":"claude-code","stub":"claude","commit":"abc123","branch":"dev"}`},
		{name: "different version", agent: `{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main","version":"1.2.3"}`},
		{name: "uncommitted", agent: `{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main","uncommitted":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			path := "repos/acme/widgets/results"
			first := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `",` +
				`"agent":{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main"},"json":{"answer":"hi"}}`
			created := sendResultPost(t, base, path, first)
			defer created.Body.Close()
			if created.StatusCode != http.StatusCreated {
				t.Fatalf("first status = %d, want %d", created.StatusCode, http.StatusCreated)
			}

			body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","agent":` + tc.agent + `,"json":{"answer":"hi"}}`
			resp := sendResultPost(t, base, path, body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			if len(st.agents) != 2 {
				t.Errorf("store holds %d agents, want 2", len(st.agents))
			}
			if len(st.results) != 2 {
				t.Errorf("store holds %d results, want 2", len(st.results))
			}
		})
	}
}

// TestAddResultAgentNameNotIdentity checks an agent's name does not separate
// records, so the same identity with another name reuses the stored agent.
func TestAddResultAgentNameNotIdentity(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/results"

	first := sendResultPost(t, base, path, `{"task":"fix-bug","test":"basic","data":"`+st.data[0].Ref+`",`+
		`"agent":{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main"},"json":{"answer":"hi"}}`)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	second := sendResultPost(t, base, path, `{"task":"fix-bug","test":"basic","data":"`+st.data[0].Ref+`",`+
		`"agent":{"name":"renamed-agent","stub":"claude","commit":"abc123","branch":"main"},"json":{"answer":"hi"}}`)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	if len(st.agents) != 1 {
		t.Errorf("store holds %d agents, want 1", len(st.agents))
	}
	if len(st.results) != 1 {
		t.Errorf("store holds %d results, want 1", len(st.results))
	}
}

// TestAddResultRejectsIncompleteAgent checks a result carrying an agent without
// a name, stub, commit, or branch returns 422 and creates neither the result nor
// the agent.
func TestAddResultRejectsIncompleteAgent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
	}{
		{name: "missing name", agent: `{"stub":"claude","commit":"abc123","branch":"main"}`},
		{name: "missing stub", agent: `{"name":"claude-code","commit":"abc123","branch":"main"}`},
		{name: "missing commit", agent: `{"name":"claude-code","stub":"claude","branch":"main"}`},
		{name: "missing branch", agent: `{"name":"claude-code","stub":"claude","commit":"abc123"}`},
		{name: "empty agent", agent: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","agent":` + tc.agent + `,"json":{"answer":"hi"}}`

			resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.agents) != 0 {
				t.Errorf("store holds %d agents, want none", len(st.agents))
			}
			if len(st.results) != 0 {
				t.Errorf("store holds %d results, want none", len(st.results))
			}
		})
	}
}

// TestAddResultRejectsNegativeMetrics checks a result with a negative token
// count, cost, or duration returns 422 and creates no result.
func TestAddResultRejectsNegativeMetrics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		metric string
	}{
		{name: "negative input tokens", metric: `"inputTokens":-1`},
		{name: "negative output tokens", metric: `"outputTokens":-1`},
		{name: "negative token cost", metric: `"tokenCost":-0.01`},
		{name: "negative duration", metric: `"duration":-0.5`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","json":{"answer":"hi"},` + tc.metric + `}`

			resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.results) != 0 {
				t.Errorf("store holds %d results, want none", len(st.results))
			}
		})
	}
}

// TestAddResultFailedWriteCreatesNoModelOrAgent checks a result write that
// fails after carrying a valid model and agent creates neither record.
func TestAddResultFailedWriteCreatesNoModelOrAgent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		missing    bool
		metric     string
		wantStatus int
	}{
		{name: "negative metric", metric: `,"inputTokens":-1`, wantStatus: http.StatusUnprocessableEntity},
		{name: "missing datum", missing: true, wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			data := st.data[0].Ref
			if tc.missing {
				data = "missing"
			}
			body := `{"task":"fix-bug","test":"basic","data":"` + data + `",` +
				`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},` +
				`"agent":{"name":"claude-code","stub":"claude","commit":"abc123","branch":"main"},` +
				`"json":{"answer":"hi"}` + tc.metric + `}`

			resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, tc.wantStatus)
			if len(st.models) != 0 {
				t.Errorf("store holds %d models, want none", len(st.models))
			}
			if len(st.agents) != 0 {
				t.Errorf("store holds %d agents, want none", len(st.agents))
			}
			if len(st.results) != 0 {
				t.Errorf("store holds %d results, want none", len(st.results))
			}
		})
	}
}

// TestAddResultRejectsNegativeMetricsOverExistingResult checks a negative
// metric is rejected with 422 even when the datum already holds a result for the
// same JSON, leaving the stored result unchanged.
func TestAddResultRejectsNegativeMetricsOverExistingResult(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/results"
	valid := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","json":{"answer":"hi"},"inputTokens":12}`

	created := sendResultPost(t, base, path, valid)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.StatusCode, http.StatusCreated)
	}

	negative := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","json":{"answer":"hi"},"inputTokens":-1}`
	rejected := sendResultPost(t, base, path, negative)
	defer rejected.Body.Close()

	assertServiceError(t, rejected, http.StatusUnprocessableEntity)
	if len(st.results) != 1 {
		t.Fatalf("store holds %d results, want 1", len(st.results))
	}
	if st.results[0].InputTokens != 12 {
		t.Errorf("stored result = %#v, want the stored result unchanged", st.results[0])
	}
}

// TestAddResultSameJSONReturnsExistingRef checks adding the same JSON for the
// same datum twice returns the existing ref with 200 and leaves one result.
func TestAddResultSameJSONReturnsExistingRef(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/results"
	body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","json":{"answer":"hi"}}`

	first := sendResultPost(t, base, path, body)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	var firstRef resultRefBody
	if err := json.NewDecoder(first.Body).Decode(&firstRef); err != nil {
		t.Fatalf("decode first body: %v", err)
	}

	second := sendResultPost(t, base, path, body)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	var secondRef resultRefBody
	if err := json.NewDecoder(second.Body).Decode(&secondRef); err != nil {
		t.Fatalf("decode second body: %v", err)
	}
	if secondRef.Ref != firstRef.Ref {
		t.Errorf("second ref = %q, want the existing ref %q", secondRef.Ref, firstRef.Ref)
	}
	if len(st.results) != 1 {
		t.Errorf("store holds %d results, want 1", len(st.results))
	}
}

// TestAddResultRejectsMalformedBody checks a body that carries no JSON, or is
// not valid JSON, returns 422 and creates no result.
func TestAddResultRejectsMalformedBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		tail string
	}{
		{name: "no json", tail: `}`},
		{name: "invalid json", tail: `,"json":}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `"` + tc.tail

			resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.results) != 0 {
				t.Errorf("store holds %d results, want none", len(st.results))
			}
		})
	}
}

// TestAddResultMissingDatum checks adding a result for a datum that does not
// exist returns 404 and creates no result.
func TestAddResultMissingDatum(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendResultPost(t, base, "repos/acme/widgets/results",
		`{"task":"fix-bug","test":"basic","data":"missing","json":{"answer":"hi"}}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
	if len(st.results) != 0 {
		t.Errorf("store holds %d results, want none", len(st.results))
	}
}

// TestGetResultsListsRefs checks GET /repos/{org}/{repo}/results returns the
// repo's result refs as a JSON array, and only that repo's results.
func TestGetResultsListsRefs(t *testing.T) {
	st := resultStore()
	st.results = []store.Result{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: "aaa111", Ref: "aaa111", JSON: `{"n":1}`},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: "bbb222", Ref: "bbb222", JSON: `{"n":2}`},
		{Org: "acme", Repo: "other", Task: "fix-bug", Test: "basic", Data: "ccc333", Ref: "ccc333", JSON: `{"n":3}`},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got []string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := []string{"aaa111", "bbb222"}
	if len(got) != len(want) {
		t.Fatalf("body listed %d refs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ref %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestGetResultsEmptyList checks a repo with no results returns an empty JSON
// array rather than a null body.
func TestGetResultsEmptyList(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(body)); got != "[]" {
		t.Errorf("body = %q, want %q", got, "[]")
	}
}

// TestGetResultsMissingRepo checks a request for a repo that does not exist
// returns the service error response.
func TestGetResultsMissingRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetResultsReportsStoreFailure checks a store failure returns the service
// error response instead of a partial list.
func TestGetResultsReportsStoreFailure(t *testing.T) {
	st := resultStore()
	st.listResultsErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetResultsDeniedForUnpermittedRole checks a role without read access
// reaches no result list.
func TestGetResultsDeniedForUnpermittedRole(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/results")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetResultsReadAllowedForRoles checks the result list is allowed for the
// Reader, Service, and Admin roles.
func TestGetResultsReadAllowedForRoles(t *testing.T) {
	for _, role := range []string{auth.RoleReader, auth.RoleService, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, role)

			resp, err := http.Get(base + "repos/acme/widgets/results")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
}

// TestGetResultReturnsFields checks GET /repos/{org}/{repo}/results/{ref}
// returns one result with its ref, task, test, datum ref, JSON, logs, tokens,
// cost, duration, and local branch.
func TestGetResultReturnsFields(t *testing.T) {
	st := resultStore()
	st.results = []store.Result{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data:  st.data[0].Ref,
		Model: store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: store.Agent{
			Org: "acme", Repo: "widgets", Task: "fix-bug",
			Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
		},
		Ref: "aaa111", JSON: `{"answer":"hi"}`, Logs: "worked",
		InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5, Local: "main",
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got struct {
		Ref          string          `json:"ref"`
		Task         string          `json:"task"`
		Test         string          `json:"test"`
		Data         string          `json:"data"`
		Model        store.Model     `json:"model"`
		Agent        store.Agent     `json:"agent"`
		JSON         json.RawMessage `json:"json"`
		Logs         string          `json:"logs"`
		InputTokens  int             `json:"inputTokens"`
		OutputTokens int             `json:"outputTokens"`
		TokenCost    float64         `json:"tokenCost"`
		Duration     float64         `json:"duration"`
		Local        string          `json:"local"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Ref != "aaa111" || got.Task != "fix-bug" || got.Test != "basic" || got.Data != st.data[0].Ref {
		t.Errorf("result identity = %#v, want the stored ref, task, test, and datum ref", got)
	}
	if got.Model != st.results[0].Model {
		t.Errorf("result model = %#v, want the stored model %#v", got.Model, st.results[0].Model)
	}
	if got.Agent != st.results[0].Agent {
		t.Errorf("result agent = %#v, want the stored agent %#v", got.Agent, st.results[0].Agent)
	}
	if string(got.JSON) != `{"answer":"hi"}` {
		t.Errorf("result JSON = %s, want the stored JSON", got.JSON)
	}
	if got.Logs != "worked" || got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 || got.Duration != 1.5 || got.Local != "main" {
		t.Errorf("result = %#v, want the stored logs, metrics, and local branch", got)
	}
}

// TestGetResultMissing checks a request for a ref that does not exist returns
// the service error response.
func TestGetResultMissing(t *testing.T) {
	st := resultStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetResultMissingRepo checks a request for a result in a repo that does not
// exist returns the service error response.
func TestGetResultMissingRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetResultReportsStoreFailure checks a store failure returns the service
// error response instead of a partial result.
func TestGetResultReportsStoreFailure(t *testing.T) {
	st := resultStore()
	st.getResultErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/results/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetResultDeniedForUnpermittedRole checks a role without read access
// reaches no result.
func TestGetResultDeniedForUnpermittedRole(t *testing.T) {
	st := resultStore()
	st.results = []store.Result{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: st.data[0].Ref, Ref: "aaa111", JSON: `{"answer":"hi"}`,
	}}
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/results/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetResultReadAllowedForRoles checks reading one result is allowed for the
// Reader, Service, and Admin roles.
func TestGetResultReadAllowedForRoles(t *testing.T) {
	for _, role := range []string{auth.RoleReader, auth.RoleService, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			st := resultStore()
			st.results = []store.Result{{
				Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
				Data: st.data[0].Ref, Ref: "aaa111", JSON: `{"answer":"hi"}`,
			}}
			base := startServiceWithStore(t, st, role)

			resp, err := http.Get(base + "repos/acme/widgets/results/aaa111")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
}

// TestAddResultWriteAllowedForRoles checks adding a result is allowed for the
// Service and Admin roles.
func TestAddResultWriteAllowedForRoles(t *testing.T) {
	for _, role := range []string{auth.RoleService, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			st := resultStore()
			base := startServiceWithStore(t, st, role)
			body := `{"task":"fix-bug","test":"basic","data":"` + st.data[0].Ref + `","json":{"answer":"hi"}}`

			resp := sendResultPost(t, base, "repos/acme/widgets/results", body)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			if len(st.results) != 1 {
				t.Errorf("store holds %d results, want 1", len(st.results))
			}
		})
	}
}

// TestAddResultDeniedForUnpermittedRole checks a role without write access is
// refused with 403 and reaches no part of the operation, so no result is
// created.
func TestAddResultDeniedForUnpermittedRole(t *testing.T) {
	for _, role := range []string{auth.RoleReader, auth.RoleAgent, auth.RoleGrader} {
		t.Run(role, func(t *testing.T) {
			st := resultStore()
			st.getResultErr = errors.New("write denied")
			st.saveResultErr = errors.New("write denied")
			base := startServiceWithStore(t, st, role)

			resp := sendResultPost(t, base, "repos/acme/widgets/results",
				`{"task":"fix-bug","test":"basic","data":"`+st.data[0].Ref+`","json":{"answer":"hi"}}`)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
			if len(st.results) != 0 {
				t.Errorf("store holds %d results, want none", len(st.results))
			}
		})
	}
}

// sendResultPost sends a result write to the service and returns the response.
func sendResultPost(t *testing.T, base, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
