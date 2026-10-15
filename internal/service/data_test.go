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

// dataRefBody mirrors the JSON body the API returns for an added datum.
type dataRefBody struct {
	Ref string `json:"ref"`
}

// TestAddDataCreatesDatum checks POST .../data stores the body's JSON as a datum
// and returns 201 with the datum's ref, which a later GET reads unchanged.
func TestAddDataCreatesDatum(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	body := `{"prompt":"hi"}`

	resp := sendDataPost(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic/data", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got dataRefBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if want := store.DatumRef([]byte(body)); got.Ref != want {
		t.Errorf("ref = %q, want the ref derived from the JSON %q", got.Ref, want)
	}
	if len(st.data) != 1 {
		t.Fatalf("store holds %d data, want 1", len(st.data))
	}
	if st.data[0].Org != "acme" || st.data[0].Repo != "widgets" || st.data[0].Task != "fix-bug" || st.data[0].Test != "basic" {
		t.Errorf("stored datum identity = %#v, want the route's stubs", st.data[0])
	}
	if st.data[0].Ref != got.Ref || st.data[0].JSON != body {
		t.Errorf("stored datum = %#v, want the JSON unchanged under the returned ref", st.data[0])
	}

	read, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug/tests/basic/data/" + got.Ref)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Body.Close()
	if read.StatusCode != http.StatusOK {
		t.Fatalf("GET datum status = %d, want %d", read.StatusCode, http.StatusOK)
	}
	readBody, err := io.ReadAll(read.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(readBody) != body {
		t.Errorf("GET body = %q, want the added JSON %q", readBody, body)
	}
}

// TestAddDataSameJSONReturnsExistingRef checks adding the same JSON twice
// returns the existing ref with 200 and leaves one datum.
func TestAddDataSameJSONReturnsExistingRef(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/tasks/fix-bug/tests/basic/data"
	body := `{"prompt":"hi"}`

	first := sendDataPost(t, base, path, body)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	var firstRef dataRefBody
	if err := json.NewDecoder(first.Body).Decode(&firstRef); err != nil {
		t.Fatalf("decode first body: %v", err)
	}

	second := sendDataPost(t, base, path, body)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	var secondRef dataRefBody
	if err := json.NewDecoder(second.Body).Decode(&secondRef); err != nil {
		t.Fatalf("decode second body: %v", err)
	}
	if secondRef.Ref != firstRef.Ref {
		t.Errorf("second ref = %q, want the existing ref %q", secondRef.Ref, firstRef.Ref)
	}
	if len(st.data) != 1 {
		t.Errorf("store holds %d data, want 1", len(st.data))
	}
}

// TestAddDataDifferentJSONNewRef checks different JSON gets a different ref and
// a second datum.
func TestAddDataDifferentJSONNewRef(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)
	path := "repos/acme/widgets/tasks/fix-bug/tests/basic/data"

	first := sendDataPost(t, base, path, `{"prompt":"hi"}`)
	defer first.Body.Close()
	second := sendDataPost(t, base, path, `{"prompt":"bye"}`)
	defer second.Body.Close()

	if first.StatusCode != http.StatusCreated || second.StatusCode != http.StatusCreated {
		t.Fatalf("statuses = %d and %d, want %d for each", first.StatusCode, second.StatusCode, http.StatusCreated)
	}
	var firstRef, secondRef dataRefBody
	if err := json.NewDecoder(first.Body).Decode(&firstRef); err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(second.Body).Decode(&secondRef); err != nil {
		t.Fatal(err)
	}
	if firstRef.Ref == secondRef.Ref {
		t.Errorf("refs = %q and %q, want different refs", firstRef.Ref, secondRef.Ref)
	}
	if len(st.data) != 2 {
		t.Errorf("store holds %d data, want 2", len(st.data))
	}
}

// TestAddDataRejectsMalformedBody checks a body that is not valid JSON returns
// 422 and creates no datum.
func TestAddDataRejectsMalformedBody(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendDataPost(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic/data", `{"prompt":`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusUnprocessableEntity)
	if len(st.data) != 0 {
		t.Errorf("store holds %d data, want none", len(st.data))
	}
}

// TestAddDataMissingTest checks adding a datum to a test that does not exist
// returns 404 and creates no datum.
func TestAddDataMissingTest(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendDataPost(t, base, "repos/acme/widgets/tasks/fix-bug/tests/missing/data", `{"prompt":"hi"}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
	if len(st.data) != 0 {
		t.Errorf("store holds %d data, want none", len(st.data))
	}
}

// TestAddDataMissingTask checks adding a datum to a test in a task that does not
// exist returns 404 and creates no datum.
func TestAddDataMissingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
	}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendDataPost(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic/data", `{"prompt":"hi"}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
	if len(st.data) != 0 {
		t.Errorf("store holds %d data, want none", len(st.data))
	}
}

// TestAddDataReportsStoreFailure checks a store failure returns the service
// error response instead of a ref.
func TestAddDataReportsStoreFailure(t *testing.T) {
	st := dataStore()
	st.saveDatumErr = errors.New("boom")
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendDataPost(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic/data", `{"prompt":"hi"}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestAddDataDeniedForUnpermittedRole checks a role without write access reaches
// no datum write.
func TestAddDataDeniedForUnpermittedRole(t *testing.T) {
	st := dataStore()
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp := sendDataPost(t, base, "repos/acme/widgets/tasks/fix-bug/tests/basic/data", `{"prompt":"hi"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if len(st.data) != 0 {
		t.Errorf("store holds %d data, want none", len(st.data))
	}
}

// sendDataPost sends a datum write to the service and returns the response.
func sendDataPost(t *testing.T, base, path, body string) *http.Response {
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
