package service

import (
	"encoding/json"
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
