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

// scoreRefBody mirrors the JSON body the API returns for an added score.
type scoreRefBody struct {
	Ref string `json:"ref"`
}

// scoreStore returns a store holding the repo, task, test, datum, and result a
// score write needs, with the test carrying a score schema.
func scoreStore() *fakeDB {
	st := resultStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Version: "1",
		Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}, {Stub: "style", Name: "Style"}},
	}}
	st.results = []store.Result{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: st.data[0].Ref, Ref: "aaa111", JSON: `{"answer":"hi"}`,
	}}
	return st
}

// TestAddScoreStoresScore checks POST /repos/{org}/{repo}/scores stores the
// score the body carries for its result and returns 201 with its ref.
func TestAddScoreStoresScore(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1},` +
		`"logs":"graded","inputTokens":12,"outputTokens":3,"tokenCost":0.01,"duration":1.5}`

	resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got scoreRefBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(st.scores) != 1 {
		t.Fatalf("store holds %d scores, want 1", len(st.scores))
	}
	stored := st.scores[0]
	if stored.Org != "acme" || stored.Repo != "widgets" || stored.Task != "fix-bug" || stored.Test != "basic" || stored.Result != "aaa111" {
		t.Errorf("stored score identity = %#v, want the route and body identity", stored)
	}
	if stored.Ref != got.Ref {
		t.Errorf("stored ref = %q, want the returned ref %q", stored.Ref, got.Ref)
	}
	if stored.Dictionary["correctness"] != 1 || stored.Logs != "graded" {
		t.Errorf("stored score = %#v, want the body's dictionary and logs", stored)
	}
	if stored.InputTokens != 12 || stored.OutputTokens != 3 || stored.TokenCost != 0.01 || stored.Duration != 1.5 {
		t.Errorf("stored score = %#v, want the body's metrics", stored)
	}
}

// TestAddScoreCreatesAndReusesModel checks a score write creates the model it
// carries, links it to the score, and reuses it for the same model values.
func TestAddScoreCreatesAndReusesModel(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	body := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
		`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},"dictionary":{"correctness":1}}`
	want := store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}

	for i, wantStatus := range []int{http.StatusCreated, http.StatusOK} {
		resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
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
	if len(st.scores) != 1 {
		t.Fatalf("store holds %d scores, want 1", len(st.scores))
	}
	if st.scores[0].Model != want {
		t.Errorf("score model = %#v, want %#v", st.scores[0].Model, want)
	}
}

// TestAddScoreDifferentModelVariantDifferentRecord checks a score carrying the
// same model name and provider with another variant links to a different model
// record.
func TestAddScoreDifferentModelVariantDifferentRecord(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"

	for _, variant := range []string{"high", "low"} {
		body := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
			`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"` + variant + `"},"dictionary":{"correctness":1}}`
		resp := sendScorePost(t, base, path, body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("variant %s status = %d, want %d", variant, resp.StatusCode, http.StatusCreated)
		}
	}

	if len(st.models) != 2 {
		t.Errorf("store holds %d models, want 2", len(st.models))
	}
	if len(st.scores) != 2 {
		t.Errorf("store holds %d scores, want 2", len(st.scores))
	}
}

// TestAddScoreRejectsIncompleteModel checks a score carrying a model without a
// name or provider returns 422 and creates neither the score nor the model.
func TestAddScoreRejectsIncompleteModel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model string
	}{
		{name: "missing name", model: `{"provider":"anthropic","variant":"high"}`},
		{name: "missing provider", model: `{"name":"claude-sonnet","variant":"high"}`},
		{name: "only variant", model: `{"variant":"high"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111","model":` + tc.model + `,"dictionary":{"correctness":1}}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.models) != 0 {
				t.Errorf("store holds %d models, want none", len(st.models))
			}
			if len(st.scores) != 0 {
				t.Errorf("store holds %d scores, want none", len(st.scores))
			}
		})
	}
}

// TestAddScoreSameDictionaryReturnsExistingRef checks adding the same dictionary
// for the same result twice returns the existing ref with 200 and leaves one
// score.
func TestAddScoreSameDictionaryReturnsExistingRef(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"
	body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1}}`

	first := sendScorePost(t, base, path, body)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	var firstRef scoreRefBody
	if err := json.NewDecoder(first.Body).Decode(&firstRef); err != nil {
		t.Fatalf("decode first body: %v", err)
	}

	second := sendScorePost(t, base, path, body)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	var secondRef scoreRefBody
	if err := json.NewDecoder(second.Body).Decode(&secondRef); err != nil {
		t.Fatalf("decode second body: %v", err)
	}
	if secondRef.Ref != firstRef.Ref {
		t.Errorf("second ref = %q, want the existing ref %q", secondRef.Ref, firstRef.Ref)
	}
	if len(st.scores) != 1 {
		t.Errorf("store holds %d scores, want 1", len(st.scores))
	}
}

// TestAddScoreRejectsMalformedBody checks a body that carries no dictionary, or
// is not valid JSON, returns 422 and creates no score.
func TestAddScoreRejectsMalformedBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		tail string
	}{
		{name: "no dictionary", tail: `}`},
		{name: "invalid json", tail: `,"dictionary":}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111"` + tc.tail

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.scores) != 0 {
				t.Errorf("store holds %d scores, want none", len(st.scores))
			}
		})
	}
}

// TestAddScoreMissingResult checks adding a score for a result that does not
// exist returns 404 and creates no score.
func TestAddScoreMissingResult(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendScorePost(t, base, "repos/acme/widgets/scores",
		`{"task":"fix-bug","test":"basic","result":"missing","dictionary":{"correctness":1}}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
	if len(st.scores) != 0 {
		t.Errorf("store holds %d scores, want none", len(st.scores))
	}
}

// TestAddScoreAcceptsSchemaKeys checks a score whose keys are all in the
// result's test score schema is created, including a score that omits a schema
// key.
func TestAddScoreAcceptsSchemaKeys(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dictionary string
	}{
		{name: "every key", dictionary: `{"correctness":1,"style":0.5}`},
		{name: "partial score", dictionary: `{"correctness":1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":` + tc.dictionary + `}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			if len(st.scores) != 1 {
				t.Fatalf("store holds %d scores, want 1", len(st.scores))
			}
		})
	}
}

// TestAddScoreRejectsUnknownSchemaKey checks a score with a key outside its
// test's score schema returns 422 and creates no score.
func TestAddScoreRejectsUnknownSchemaKey(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"speed":1}}`

	resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusUnprocessableEntity)
	if len(st.scores) != 0 {
		t.Errorf("store holds %d scores, want none", len(st.scores))
	}
}

// TestAddScoreRejectsNonNumberValue checks a score with a value that is not a
// number returns 422 and creates no score.
func TestAddScoreRejectsNonNumberValue(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dictionary string
	}{
		{name: "string", dictionary: `{"correctness":"high"}`},
		{name: "boolean", dictionary: `{"correctness":true}`},
		{name: "null", dictionary: `{"correctness":null}`},
		{name: "object", dictionary: `{"correctness":{"value":1}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":` + tc.dictionary + `}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.scores) != 0 {
				t.Errorf("store holds %d scores, want none", len(st.scores))
			}
		})
	}
}

// TestAddScoreRejectsNegativeMetrics checks a score with a negative token count,
// cost, or duration returns 422 and creates no score.
func TestAddScoreRejectsNegativeMetrics(t *testing.T) {
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
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1},` + tc.metric + `}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.scores) != 0 {
				t.Errorf("store holds %d scores, want none", len(st.scores))
			}
		})
	}
}

// TestAddScoreRejectsNegativeMetricsOverExistingScore checks a negative metric
// is rejected with 422 even when the result already holds a score for the same
// dictionary, leaving the stored score unchanged.
func TestAddScoreRejectsNegativeMetricsOverExistingScore(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"
	valid := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1},"inputTokens":12}`

	created := sendScorePost(t, base, path, valid)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.StatusCode, http.StatusCreated)
	}

	negative := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1},"inputTokens":-1}`
	rejected := sendScorePost(t, base, path, negative)
	defer rejected.Body.Close()

	assertServiceError(t, rejected, http.StatusUnprocessableEntity)
	if len(st.scores) != 1 {
		t.Fatalf("store holds %d scores, want 1", len(st.scores))
	}
	if st.scores[0].InputTokens != 12 {
		t.Errorf("stored score = %#v, want the stored score unchanged", st.scores[0])
	}
}

// TestAddScoreWriteAllowedForRoles checks adding a score is allowed for the
// Service and Admin roles.
func TestAddScoreWriteAllowedForRoles(t *testing.T) {
	for _, role := range []string{auth.RoleService, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, role)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1}}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			if len(st.scores) != 1 {
				t.Errorf("store holds %d scores, want 1", len(st.scores))
			}
		})
	}
}

// TestAddScoreDeniedForUnpermittedRole checks a role without write access is
// refused with 403 and reaches no part of the operation, so no score is
// created.
func TestAddScoreDeniedForUnpermittedRole(t *testing.T) {
	for _, role := range []string{auth.RoleReader, auth.RoleAgent, auth.RoleGrader} {
		t.Run(role, func(t *testing.T) {
			st := scoreStore()
			st.getScoreErr = errors.New("write denied")
			st.saveScoreErr = errors.New("write denied")
			base := startServiceWithStore(t, st, role)

			resp := sendScorePost(t, base, "repos/acme/widgets/scores",
				`{"task":"fix-bug","test":"basic","result":"aaa111","dictionary":{"correctness":1}}`)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
			if len(st.scores) != 0 {
				t.Errorf("store holds %d scores, want none", len(st.scores))
			}
		})
	}
}

// TestGetScoresListsRefs checks GET /repos/{org}/{repo}/scores returns the
// repo's score refs as a JSON array, and only that repo's scores.
func TestGetScoresListsRefs(t *testing.T) {
	st := scoreStore()
	st.scores = []store.Score{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Result: "aaa111", Ref: "aaa111", Dictionary: map[string]float64{"correctness": 1}},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Result: "aaa111", Ref: "bbb222", Dictionary: map[string]float64{"style": 1}},
		{Org: "acme", Repo: "other", Task: "fix-bug", Test: "basic", Result: "aaa111", Ref: "ccc333", Dictionary: map[string]float64{"correctness": 1}},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores")
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

// TestGetScoresEmptyList checks a repo with no scores returns an empty JSON
// array rather than a null body.
func TestGetScoresEmptyList(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores")
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

// TestGetScoresMissingRepo checks a request for a repo that does not exist
// returns the service error response.
func TestGetScoresMissingRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetScoresReportsStoreFailure checks a store failure returns the service
// error response instead of a partial list.
func TestGetScoresReportsStoreFailure(t *testing.T) {
	st := scoreStore()
	st.listScoresErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetScoresReadAllowedForRoles checks the score list is allowed for the
// Reader, Service, and Admin roles.
func TestGetScoresReadAllowedForRoles(t *testing.T) {
	for _, role := range []string{auth.RoleReader, auth.RoleService, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, role)

			resp, err := http.Get(base + "repos/acme/widgets/scores")
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

// TestGetScoresDeniedForUnpermittedRole checks a role without read access
// reaches no score list.
func TestGetScoresDeniedForUnpermittedRole(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/scores")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetScoreReturnsFields checks GET /repos/{org}/{repo}/scores/{ref}
// returns one score with its ref, task, test, result ref, dictionary, logs,
// tokens, cost, and duration.
func TestGetScoreReturnsFields(t *testing.T) {
	st := scoreStore()
	st.scores = []store.Score{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "aaa111", Ref: "bbb222",
		Model:       store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Dictionary:  map[string]float64{"correctness": 1},
		Logs:        "graded",
		InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5,
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores/bbb222")
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
		Ref          string             `json:"ref"`
		Task         string             `json:"task"`
		Test         string             `json:"test"`
		Result       string             `json:"result"`
		Model        store.Model        `json:"model"`
		Dictionary   map[string]float64 `json:"dictionary"`
		Logs         string             `json:"logs"`
		InputTokens  int                `json:"inputTokens"`
		OutputTokens int                `json:"outputTokens"`
		TokenCost    float64            `json:"tokenCost"`
		Duration     float64            `json:"duration"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Ref != "bbb222" || got.Task != "fix-bug" || got.Test != "basic" || got.Result != "aaa111" {
		t.Errorf("score identity = %#v, want the stored ref, task, test, and result ref", got)
	}
	if got.Model != st.scores[0].Model {
		t.Errorf("score model = %#v, want the stored model %#v", got.Model, st.scores[0].Model)
	}
	if got.Dictionary["correctness"] != 1 {
		t.Errorf("score dictionary = %#v, want the stored dictionary", got.Dictionary)
	}
	if got.Logs != "graded" || got.InputTokens != 12 || got.OutputTokens != 3 || got.TokenCost != 0.01 || got.Duration != 1.5 {
		t.Errorf("score = %#v, want the stored logs and metrics", got)
	}
}

// TestGetScoreMissing checks a request for a ref that does not exist returns
// the service error response.
func TestGetScoreMissing(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetScoreMissingRepo checks a request for a score in a repo that does not
// exist returns the service error response.
func TestGetScoreMissingRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores/bbb222")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetScoreReportsStoreFailure checks a store failure returns the service
// error response instead of a partial score.
func TestGetScoreReportsStoreFailure(t *testing.T) {
	st := scoreStore()
	st.getScoreErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/scores/bbb222")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetScoreDeniedForUnpermittedRole checks a role without read access
// reaches no score.
func TestGetScoreDeniedForUnpermittedRole(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/scores/bbb222")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetScoreReadAllowedForRoles checks reading one score is allowed for the
// Reader, Service, and Admin roles.
func TestGetScoreReadAllowedForRoles(t *testing.T) {
	for _, role := range []string{auth.RoleReader, auth.RoleService, auth.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			st := scoreStore()
			st.scores = []store.Score{{
				Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
				Result: "aaa111", Ref: "bbb222",
				Dictionary: map[string]float64{"correctness": 1},
			}}
			base := startServiceWithStore(t, st, role)

			resp, err := http.Get(base + "repos/acme/widgets/scores/bbb222")
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

// sendScorePost sends a score write to the service and returns the response.
func sendScorePost(t *testing.T, base, path, body string) *http.Response {
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
