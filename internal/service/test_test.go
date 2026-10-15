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

// TestPutTestRejectsDuplicateSchemaStub checks a create whose score schema gives
// two keys the same stub returns 422 and writes no test.
func TestPutTestRejectsDuplicateSchemaStub(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"name":"Basic","version":"1","schema":[{"stub":"style","name":"Style"},{"stub":"style","name":"Other"}]}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusUnprocessableEntity)
	if len(st.tests) != 0 {
		t.Errorf("store holds %d tests, want none", len(st.tests))
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

// TestPutTestUpdateKeepsSchemaWhenBodyOmitsIt checks an update whose body
// carries no schema leaves the stored schema in place, because only a body that
// carries one replaces it.
func TestPutTestUpdateKeepsSchemaWhenBodyOmitsIt(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Description: "A basic test", Version: "1",
		Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}},
	}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic", `{"version":"2"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got testBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}}
	if len(got.Schema) != len(want) || got.Schema[0] != want[0] {
		t.Errorf("schema = %#v, want the stored schema kept as %#v", got.Schema, want)
	}
	if len(st.tests) != 1 || len(st.tests[0].Schema) != 1 || st.tests[0].Schema[0] != want[0] {
		t.Errorf("stored tests = %#v, want the schema kept as %#v", st.tests, want)
	}
}

// TestPutTestUpdateReplacesSchemaWithEmpty checks an update whose body carries an
// empty schema clears the stored schema, because an empty schema is still a
// schema the body carries.
func TestPutTestUpdateReplacesSchemaWithEmpty(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Version: "1",
		Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}},
	}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"name":"Basic","version":"1","schema":[]}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(st.tests) != 1 || len(st.tests[0].Schema) != 0 {
		t.Errorf("stored tests = %#v, want the schema cleared", st.tests)
	}
}

// TestPutTestUpdateMergesNameAndDescription checks an update replaces the name
// and description the body carries and keeps the fields it omits.
func TestPutTestUpdateMergesNameAndDescription(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Description: "A basic test", Version: "1",
	}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTestPut(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic",
		`{"name":"Renamed","description":"A renamed test"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got testBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Name != "Renamed" || got.Description != "A renamed test" || got.Version != "1" {
		t.Errorf("test = %#v, want the name and description replaced and the version kept", got)
	}
}

// TestPutTestCreateThenUpdate checks the same write creates a test with 201 and
// a later write updates it with 200, leaving one stored test.
func TestPutTestCreateThenUpdate(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/tasks/fix-bug/tests/basic"

	created := sendTestPut(t, base, path, `{"name":"Basic","version":"1"}`)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.StatusCode, http.StatusCreated)
	}

	updated := sendTestPut(t, base, path, `{"version":"2"}`)
	defer updated.Body.Close()
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want %d", updated.StatusCode, http.StatusOK)
	}

	if len(st.tests) != 1 || st.tests[0].Name != "Basic" || st.tests[0].Version != "2" {
		t.Errorf("store holds %#v, want one test updated to version 2", st.tests)
	}
}

// TestPutTestMissingTaskCreatesNoTaskOrTest checks a write to a task that does
// not exist returns 404 and creates neither the task nor the test.
func TestPutTestMissingTaskCreatesNoTaskOrTest(t *testing.T) {
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

// TestGetTestsListsTests checks GET /repos/{org}/{repo}/tasks/{task}/tests
// returns the task's tests as a JSON array.
func TestGetTestsListsTests(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{
		{
			Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
			Name: "Basic", Description: "A basic test", Version: "1",
			Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}},
		},
		{
			Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "smoke",
			Name: "Smoke", Description: "A smoke test", Version: "2",
			Schema: []store.SchemaKey{{Stub: "style", Name: "Style"}},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests")
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

	var got []testBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("body listed %d tests, want 2", len(got))
	}
	if got[0].Stub != "basic" || got[0].Name != "Basic" || got[0].Description != "A basic test" || got[0].Version != "1" {
		t.Errorf("test 0 = %#v, want the first test's fields", got[0])
	}
	if len(got[0].Schema) != 1 || got[0].Schema[0] != (store.SchemaKey{Stub: "correctness", Name: "Correctness"}) {
		t.Errorf("test 0 schema = %#v, want the correctness key", got[0].Schema)
	}
	if got[1].Stub != "smoke" || got[1].Name != "Smoke" || got[1].Version != "2" {
		t.Errorf("test 1 = %#v, want the second test's fields", got[1])
	}
}

// TestGetTestsEmptyList checks a task with no tests returns an empty JSON array
// rather than a null body.
func TestGetTestsEmptyList(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests")
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

// TestGetTestsMissingTask checks a request for the tests of a task that does not
// exist returns the service error response.
func TestGetTestsMissingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetTestsReportsStoreFailure checks a store failure returns the service
// error response instead of a partial list.
func TestGetTestsReportsStoreFailure(t *testing.T) {
	st := testStore()
	st.listTestsErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetTestsDeniedForUnpermittedRole checks a role without read access reaches
// no test list.
func TestGetTestsDeniedForUnpermittedRole(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetTestReturnsTest checks GET
// /repos/{org}/{repo}/tasks/{task}/tests/{stub} returns one test with its stub,
// name, description, version, and schema.
func TestGetTestReturnsTest(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{
		{
			Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
			Name: "Basic", Description: "A basic test", Version: "1",
			Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}},
		},
		{
			Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "smoke",
			Name: "Smoke", Description: "A smoke test", Version: "2",
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic")
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

	var got testBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := testBody{
		Stub: "basic", Name: "Basic", Description: "A basic test", Version: "1",
		Schema: []store.SchemaKey{{Stub: "correctness", Name: "Correctness"}},
	}
	if got.Stub != want.Stub || got.Name != want.Name || got.Description != want.Description || got.Version != want.Version {
		t.Errorf("test = %#v, want %#v", got, want)
	}
	if len(got.Schema) != len(want.Schema) || got.Schema[0] != want.Schema[0] {
		t.Errorf("schema = %#v, want %#v", got.Schema, want.Schema)
	}
}

// TestGetTestMissing checks a request for a test that does not exist returns the
// service error response.
func TestGetTestMissing(t *testing.T) {
	st := testStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Version: "1",
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetTestMissingTask checks a request for a test in a task that does not
// exist returns the service error response.
func TestGetTestMissingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetTestReportsStoreFailure checks a store failure returns the service
// error response instead of a test.
func TestGetTestReportsStoreFailure(t *testing.T) {
	st := testStore()
	st.getTestErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetTestDeniedForUnpermittedRole checks a role without read access reaches
// no test.
func TestGetTestDeniedForUnpermittedRole(t *testing.T) {
	st := testStore()
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
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
