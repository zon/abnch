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

// TestAddScoreDifferentModelDifferentScore checks a score carrying a model that
// differs in an identity field is a distinct score under a different ref and
// links to a different model record.
func TestAddScoreDifferentModelDifferentScore(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model string
	}{
		{name: "different name", model: `{"name":"claude-opus","provider":"anthropic","variant":"high"}`},
		{name: "different provider", model: `{"name":"claude-sonnet","provider":"openai","variant":"high"}`},
		{name: "different variant", model: `{"name":"claude-sonnet","provider":"anthropic","variant":"low"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			path := "repos/acme/widgets/scores"
			first := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
				`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},"dictionary":{"correctness":1}}`
			created := sendScorePost(t, base, path, first)
			defer created.Body.Close()
			if created.StatusCode != http.StatusCreated {
				t.Fatalf("first status = %d, want %d", created.StatusCode, http.StatusCreated)
			}
			var firstRef scoreRefBody
			if err := json.NewDecoder(created.Body).Decode(&firstRef); err != nil {
				t.Fatalf("decode first body: %v", err)
			}

			body := `{"task":"fix-bug","test":"basic","result":"aaa111","model":` + tc.model + `,"dictionary":{"correctness":1}}`
			resp := sendScorePost(t, base, path, body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			var secondRef scoreRefBody
			if err := json.NewDecoder(resp.Body).Decode(&secondRef); err != nil {
				t.Fatalf("decode second body: %v", err)
			}
			if secondRef.Ref == firstRef.Ref {
				t.Errorf("second ref = %q, want a different ref for %s", secondRef.Ref, tc.name)
			}
			if len(st.models) != 2 {
				t.Errorf("store holds %d models, want 2", len(st.models))
			}
			if len(st.scores) != 2 {
				t.Errorf("store holds %d scores, want 2", len(st.scores))
			}
		})
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

// TestAddScoreCreatesAndReusesGrader checks a score write creates the grader it
// carries, associates it with the result's test, defaults uncommitted to false,
// and reuses it for the same grader values.
func TestAddScoreCreatesAndReusesGrader(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"
	body := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
		`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},"dictionary":{"correctness":1}}`
	want := store.Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}

	for i, wantStatus := range []int{http.StatusCreated, http.StatusOK} {
		resp := sendScorePost(t, base, path, body)
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Fatalf("post %d status = %d, want %d", i, resp.StatusCode, wantStatus)
		}
	}

	if len(st.graders) != 1 {
		t.Fatalf("store holds %d graders, want 1", len(st.graders))
	}
	if st.graders[0] != want {
		t.Errorf("stored grader = %#v, want %#v", st.graders[0], want)
	}
	if len(st.scores) != 1 {
		t.Fatalf("store holds %d scores, want 1", len(st.scores))
	}
	if st.scores[0].Grader != want {
		t.Errorf("score grader = %#v, want %#v", st.scores[0].Grader, want)
	}

	other := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
		`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},"dictionary":{"style":1}}`
	resp := sendScorePost(t, base, path, other)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("second score status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if len(st.graders) != 1 {
		t.Errorf("store holds %d graders, want the reused grader", len(st.graders))
	}
	if len(st.scores) != 2 {
		t.Errorf("store holds %d scores, want 2", len(st.scores))
	}
}

// TestAddScoreReturnsModelAndGrader checks a score write carrying a model and a
// grader creates and links both records, and a later read returns them with the
// score.
func TestAddScoreReturnsModelAndGrader(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"
	body := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
		`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},` +
		`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},` +
		`"dictionary":{"correctness":1}}`
	wantModel := store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	wantGrader := store.Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}

	created := sendScorePost(t, base, path, body)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", created.StatusCode, http.StatusCreated)
	}
	var added scoreRefBody
	if err := json.NewDecoder(created.Body).Decode(&added); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if len(st.models) != 1 || st.models[0] != wantModel {
		t.Fatalf("store models = %#v, want the carried model %#v", st.models, wantModel)
	}
	if len(st.graders) != 1 || st.graders[0] != wantGrader {
		t.Fatalf("store graders = %#v, want the carried grader %#v", st.graders, wantGrader)
	}
	if len(st.scores) != 1 {
		t.Fatalf("store holds %d scores, want 1", len(st.scores))
	}
	if st.scores[0].Model != wantModel || st.scores[0].Grader != wantGrader {
		t.Errorf("score records = %#v and %#v, want the carried records", st.scores[0].Model, st.scores[0].Grader)
	}

	resp, err := http.Get(base + path + "/" + added.Ref)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got struct {
		Ref    string       `json:"ref"`
		Model  store.Model  `json:"model"`
		Grader store.Grader `json:"grader"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Ref != added.Ref {
		t.Errorf("score ref = %q, want the added ref %q", got.Ref, added.Ref)
	}
	if got.Model != wantModel {
		t.Errorf("score model = %#v, want %#v", got.Model, wantModel)
	}
	if got.Grader != wantGrader {
		t.Errorf("score grader = %#v, want %#v", got.Grader, wantGrader)
	}
}

// TestAddScoreReusesModelAndGraderAcrossReads checks two scores carrying the
// same model and grader share one record each, and each read returns them.
func TestAddScoreReusesModelAndGraderAcrossReads(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"
	records := `"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},` +
		`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"}`
	wantModel := store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	wantGrader := store.Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}

	var refs []string
	for _, dictionary := range []string{`{"correctness":1}`, `{"style":1}`} {
		body := `{"task":"fix-bug","test":"basic","result":"aaa111",` + records +
			`,"dictionary":` + dictionary + `}`
		created := sendScorePost(t, base, path, body)
		defer created.Body.Close()
		if created.StatusCode != http.StatusCreated {
			t.Fatalf("dictionary %s status = %d, want %d", dictionary, created.StatusCode, http.StatusCreated)
		}
		var added scoreRefBody
		if err := json.NewDecoder(created.Body).Decode(&added); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		refs = append(refs, added.Ref)
	}

	if len(st.models) != 1 || st.models[0] != wantModel {
		t.Fatalf("store models = %#v, want one carried model %#v", st.models, wantModel)
	}
	if len(st.graders) != 1 || st.graders[0] != wantGrader {
		t.Fatalf("store graders = %#v, want one carried grader %#v", st.graders, wantGrader)
	}
	if len(st.scores) != 2 {
		t.Fatalf("store holds %d scores, want 2", len(st.scores))
	}

	for _, ref := range refs {
		resp, err := http.Get(base + path + "/" + ref)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("get %s status = %d, want %d", ref, resp.StatusCode, http.StatusOK)
		}
		var got struct {
			Model  store.Model  `json:"model"`
			Grader store.Grader `json:"grader"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			resp.Body.Close()
			t.Fatalf("decode body: %v", err)
		}
		resp.Body.Close()
		if got.Model != wantModel {
			t.Errorf("score %s model = %#v, want %#v", ref, got.Model, wantModel)
		}
		if got.Grader != wantGrader {
			t.Errorf("score %s grader = %#v, want %#v", ref, got.Grader, wantGrader)
		}
	}
}

// TestAddScoreDifferentGraderDifferentRecord checks a score carrying a grader
// that differs in an identity field links to a different grader record, and the
// same dictionary from a different grader is a different score.
func TestAddScoreDifferentGraderDifferentRecord(t *testing.T) {
	for _, tc := range []struct {
		name   string
		grader string
	}{
		{name: "different stub", grader: `{"name":"llm-judge","stub":"other","commit":"def456","branch":"main"}`},
		{name: "different commit", grader: `{"name":"llm-judge","stub":"judge","commit":"abc123","branch":"main"}`},
		{name: "different branch", grader: `{"name":"llm-judge","stub":"judge","commit":"def456","branch":"dev"}`},
		{name: "different version", grader: `{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main","version":"2.0.0"}`},
		{name: "uncommitted", grader: `{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main","uncommitted":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			path := "repos/acme/widgets/scores"
			first := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
				`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},"dictionary":{"correctness":1}}`
			created := sendScorePost(t, base, path, first)
			defer created.Body.Close()
			if created.StatusCode != http.StatusCreated {
				t.Fatalf("first status = %d, want %d", created.StatusCode, http.StatusCreated)
			}
			var firstRef scoreRefBody
			if err := json.NewDecoder(created.Body).Decode(&firstRef); err != nil {
				t.Fatalf("decode first body: %v", err)
			}

			body := `{"task":"fix-bug","test":"basic","result":"aaa111","grader":` + tc.grader + `,"dictionary":{"correctness":1}}`
			resp := sendScorePost(t, base, path, body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
			}
			var secondRef scoreRefBody
			if err := json.NewDecoder(resp.Body).Decode(&secondRef); err != nil {
				t.Fatalf("decode second body: %v", err)
			}
			if secondRef.Ref == firstRef.Ref {
				t.Errorf("second ref = %q, want a different ref for %s", secondRef.Ref, tc.name)
			}
			if len(st.graders) != 2 {
				t.Errorf("store holds %d graders, want 2", len(st.graders))
			}
			if len(st.scores) != 2 {
				t.Errorf("store holds %d scores, want 2", len(st.scores))
			}
		})
	}
}

// TestAddScoreGraderNameNotIdentity checks a grader's name does not separate
// records, so the same identity with another name reuses the stored grader.
func TestAddScoreGraderNameNotIdentity(t *testing.T) {
	st := scoreStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/scores"

	first := sendScorePost(t, base, path, `{"task":"fix-bug","test":"basic","result":"aaa111",`+
		`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},"dictionary":{"correctness":1}}`)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	second := sendScorePost(t, base, path, `{"task":"fix-bug","test":"basic","result":"aaa111",`+
		`"grader":{"name":"renamed-judge","stub":"judge","commit":"def456","branch":"main"},"dictionary":{"correctness":1}}`)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	if len(st.graders) != 1 {
		t.Errorf("store holds %d graders, want 1", len(st.graders))
	}
	if len(st.scores) != 1 {
		t.Errorf("store holds %d scores, want 1", len(st.scores))
	}
}

// TestAddScoreRejectsIncompleteGrader checks a score carrying a grader without
// a name, stub, commit, or branch returns 422 and creates no score, grader, or
// model.
func TestAddScoreRejectsIncompleteGrader(t *testing.T) {
	for _, tc := range []struct {
		name   string
		grader string
	}{
		{name: "missing name", grader: `{"stub":"judge","commit":"def456","branch":"main"}`},
		{name: "missing stub", grader: `{"name":"llm-judge","commit":"def456","branch":"main"}`},
		{name: "missing commit", grader: `{"name":"llm-judge","stub":"judge","branch":"main"}`},
		{name: "missing branch", grader: `{"name":"llm-judge","stub":"judge","commit":"def456"}`},
		{name: "empty grader", grader: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"aaa111",` +
				`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},"grader":` + tc.grader + `,"dictionary":{"correctness":1}}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, http.StatusUnprocessableEntity)
			if len(st.graders) != 0 {
				t.Errorf("store holds %d graders, want none", len(st.graders))
			}
			if len(st.scores) != 0 {
				t.Errorf("store holds %d scores, want none", len(st.scores))
			}
			if len(st.models) != 0 {
				t.Errorf("store holds %d models, want none", len(st.models))
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

// TestAddScoreFailedWriteCreatesNoModelOrGrader checks a score write that fails
// after carrying a valid model and grader creates neither record.
func TestAddScoreFailedWriteCreatesNoModelOrGrader(t *testing.T) {
	for _, tc := range []struct {
		name       string
		result     string
		dictionary string
		metric     string
		wantStatus int
	}{
		{name: "negative metric", result: "aaa111", dictionary: `{"correctness":1}`, metric: `,"inputTokens":-1`, wantStatus: http.StatusUnprocessableEntity},
		{name: "unknown schema key", result: "aaa111", dictionary: `{"speed":1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "missing result", result: "missing", dictionary: `{"correctness":1}`, wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := scoreStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)
			body := `{"task":"fix-bug","test":"basic","result":"` + tc.result + `",` +
				`"model":{"name":"claude-sonnet","provider":"anthropic","variant":"high"},` +
				`"grader":{"name":"llm-judge","stub":"judge","commit":"def456","branch":"main"},` +
				`"dictionary":` + tc.dictionary + tc.metric + `}`

			resp := sendScorePost(t, base, "repos/acme/widgets/scores", body)
			defer resp.Body.Close()

			assertServiceError(t, resp, tc.wantStatus)
			if len(st.models) != 0 {
				t.Errorf("store holds %d models, want none", len(st.models))
			}
			if len(st.graders) != 0 {
				t.Errorf("store holds %d graders, want none", len(st.graders))
			}
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
// returns one score with its ref, task, test, result ref, model, grader,
// dictionary, logs, tokens, cost, and duration.
func TestGetScoreReturnsFields(t *testing.T) {
	st := scoreStore()
	st.scores = []store.Score{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "aaa111", Ref: "bbb222",
		Model: store.Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: store.Grader{
			Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
			Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
		},
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
		Grader       store.Grader       `json:"grader"`
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
	if got.Grader != st.scores[0].Grader {
		t.Errorf("score grader = %#v, want the stored grader %#v", got.Grader, st.scores[0].Grader)
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
