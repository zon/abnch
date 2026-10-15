package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/store"
)

// testBody mirrors the JSON body the API returns for a test.
type testBody struct {
	Stub        string            `json:"stub"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Schema      []store.SchemaKey `json:"schema"`
}

// testStore returns a store holding the repo and task a test write needs.
func testStore() *fakeDB {
	return &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		tasks: []store.Task{{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"}},
	}
}

// TestPutTestCreatesTest checks PUT
// /repos/{org}/{repo}/tasks/{task}/tests/{stub} creates a missing test from the
// body, returns 201, and stores its fields.
func TestPutTestCreatesTest(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"name":"Basic","description":"A basic test","version":"1","schema":[{"stub":"correctness","name":"Correctness"}]}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var got testBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Stub != "basic" || got.Name != "Basic" || got.Description != "A basic test" || got.Version != "1" {
		t.Errorf("test = %#v, want the stub and the body's fields", got)
	}
	want := []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}}
	if len(got.Schema) != len(want) || got.Schema[0] != want[0] {
		t.Errorf("schema = %#v, want %#v", got.Schema, want)
	}

	if len(st.tests) != 1 {
		t.Fatalf("store holds %d tests, want 1", len(st.tests))
	}
	stored := st.tests[0]
	if stored.Org != "acme" || stored.Repo != "widgets" || stored.Task != "fix-bug" || stored.Stub != "basic" {
		t.Errorf("stored test identity = %#v, want the route's stubs", stored)
	}
	if stored.Name != "Basic" || stored.Description != "A basic test" || stored.Version != "1" {
		t.Errorf("stored test = %#v, want the body's fields", stored)
	}
}

// TestPutTestAllowsEmptySchema checks a create without schema keys is accepted,
// because only the name and version are required.
func TestPutTestAllowsEmptySchema(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"name":"Basic","version":"1"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if len(st.tests) != 1 {
		t.Fatalf("store holds %d tests, want 1", len(st.tests))
	}
	if len(st.tests[0].Schema) != 0 {
		t.Errorf("stored schema = %#v, want an empty schema", st.tests[0].Schema)
	}
}

// TestPutTestRejectsMissingRequiredField checks a create that misses a name or a
// version returns 422 and writes no test.
func TestPutTestRejectsMissingRequiredField(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing name", body: `{"description":"A basic test","version":"1"}`},
		{name: "empty name", body: `{"name":"","version":"1"}`},
		{name: "missing version", body: `{"name":"Basic","description":"A basic test"}`},
		{name: "empty version", body: `{"name":"Basic","version":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := testStore()
			base := startServiceWithStore(t, st, auth.RoleAdmin)

			resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic", tc.body)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
			}
			if len(st.tests) != 0 {
				t.Errorf("store holds %d tests, want none", len(st.tests))
			}
		})
	}
}

// TestPutTestUpdatesExistingTest checks a write to a stored test merges the body
// into it, returns 200, and replaces the schema when the body carries one.
func TestPutTestUpdatesExistingTest(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Description: "A basic test", Version: "1",
		Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}},
	}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"version":"2","schema":[{"stub":"style","name":"Style"}]}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got testBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Basic" || got.Description != "A basic test" || got.Version != "2" {
		t.Errorf("test = %#v, want the version replaced and the other fields kept", got)
	}
	want := []store.SchemaKey{{Stub: "style", Name: "Style"}}
	if len(got.Schema) != len(want) || got.Schema[0] != want[0] {
		t.Errorf("schema = %#v, want it replaced with %#v", got.Schema, want)
	}
	if len(st.tests) != 1 {
		t.Errorf("store holds %d tests, want 1", len(st.tests))
	}
}

// TestPutTestMissingTask checks a write to a task that does not exist returns
// the service error response and creates no test.
func TestPutTestMissingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
	}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"name":"Basic","version":"1"}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
	if len(st.tasks) != 0 || len(st.tests) != 0 {
		t.Errorf("store holds %d tasks and %d tests, want none", len(st.tasks), len(st.tests))
	}
}

// sendTestPut sends a test write to the service and returns the response.
func sendTestPut(t *testing.T, base, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, base+path, strings.NewReader(body))
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
