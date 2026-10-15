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

// dataStore returns a store holding the repo, task, and test a data read needs.
func dataStore() *fakeDB {
	st := testStore()
	st.tests = []store.Test{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Version: "1",
	}}
	return st
}

// TestGetDataListsRefs checks GET
// /repos/{org}/{repo}/tasks/{task}/tests/{test}/data returns the test's data
// refs as a JSON array, and only that test's data.
func TestGetDataListsRefs(t *testing.T) {
	st := dataStore()
	st.data = []store.Datum{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "aaa111", JSON: `{"n":1}`},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "bbb222", JSON: `{"n":2}`},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "other", Ref: "ccc333", JSON: `{"n":3}`},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data")
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

// TestGetDataEmptyList checks a test with no data returns an empty JSON array
// rather than a null body.
func TestGetDataEmptyList(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data")
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

// TestGetDataMissingTest checks a request for the data of a test that does not
// exist returns the service error response.
func TestGetDataMissingTest(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/missing/data")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetDataMissingTask checks a request for the data of a test in a task that
// does not exist returns the service error response.
func TestGetDataMissingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetDataReportsStoreFailure checks a store failure returns the service
// error response instead of a partial list.
func TestGetDataReportsStoreFailure(t *testing.T) {
	st := dataStore()
	st.listDataErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetDataDeniedForUnpermittedRole checks a role without read access reaches
// no data list.
func TestGetDataDeniedForUnpermittedRole(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetOneDataReturnsJSON checks GET
// /repos/{org}/{repo}/tasks/{task}/tests/{test}/data/{ref} returns one datum's
// JSON, unchanged and only that datum's.
func TestGetOneDataReturnsJSON(t *testing.T) {
	st := dataStore()
	st.data = []store.Datum{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "aaa111", JSON: `{"prompt":  "hi"}`},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "bbb222", JSON: `{"prompt":"bye"}`},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/aaa111")
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != `{"prompt":  "hi"}` {
		t.Errorf("body = %q, want the datum's JSON unchanged", got)
	}
}

// TestGetOneDataReturnsFileReference checks a datum whose JSON holds a file
// reference comes back unchanged, with its URL and checksum readable.
func TestGetOneDataReturnsFileReference(t *testing.T) {
	st := dataStore()
	st.data = []store.Datum{{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "aaa111",
		JSON: `{"input": {"url": "https://example.com/input.txt", "checksum": "sha256:abc123"}}`,
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Input struct {
			URL      string `json:"url"`
			Checksum string `json:"checksum"`
		} `json:"input"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Input.URL != "https://example.com/input.txt" || body.Input.Checksum != "sha256:abc123" {
		t.Errorf("file reference = %#v, want its URL and checksum unchanged", body.Input)
	}
}

// TestGetOneDataMissingDatum checks a request for a ref the test does not hold
// returns the service error response.
func TestGetOneDataMissingDatum(t *testing.T) {
	st := dataStore()
	st.data = []store.Datum{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "aaa111", JSON: `{"n":1}`},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetOneDataMissingTest checks a request for a datum of a test that does not
// exist returns the service error response.
func TestGetOneDataMissingTest(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/missing/data/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetOneDataMissingTask checks a request for a datum in a task that does not
// exist returns the service error response.
func TestGetOneDataMissingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetOneDataReportsStoreFailure checks a store failure returns the service
// error response instead of a partial body.
func TestGetOneDataReportsStoreFailure(t *testing.T) {
	st := dataStore()
	st.getDataErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetOneDataDeniedForUnpermittedRole checks a role without read access
// reaches no datum.
func TestGetOneDataDeniedForUnpermittedRole(t *testing.T) {
	st := dataStore()
	st.data = []store.Datum{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: "aaa111", JSON: `{"n":1}`},
	}
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/aaa111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
