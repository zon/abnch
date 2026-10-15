package service

import (
	"encoding/json"
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
