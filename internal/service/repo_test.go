package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/store"
)

// repoBody mirrors the JSON body the API returns for a repo.
type repoBody struct {
	Org  string `json:"org"`
	Stub string `json:"stub"`
	URL  string `json:"url"`
}

// TestGetReposListsRepos checks GET /repos returns the stored repos as a JSON
// array.
func TestGetReposListsRepos(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{
		{Org: "acme", Stub: "gadgets", URL: "https://github.com/acme/gadgets"},
		{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"},
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos")
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

	var got []repoBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := []repoBody{
		{Org: "acme", Stub: "gadgets", URL: "https://github.com/acme/gadgets"},
		{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"},
	}
	if len(got) != len(want) {
		t.Fatalf("body listed %d repos, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("repo %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

// TestGetReposEmptyList checks an empty store returns an empty JSON array
// rather than a null body.
func TestGetReposEmptyList(t *testing.T) {
	base := startServiceWithStore(t, &fakeDB{}, auth.RoleReader)

	resp, err := http.Get(base + "repos")
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

// TestGetReposReportsStoreFailure checks a store failure returns the service
// error response instead of a partial list.
func TestGetReposReportsStoreFailure(t *testing.T) {
	base := startServiceWithStore(t, &fakeDB{listErr: errors.New("boom")}, auth.RoleReader)

	resp, err := http.Get(base + "repos")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetReposDeniedForUnpermittedRole checks a role without read access
// reaches no repo list.
func TestGetReposDeniedForUnpermittedRole(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetRepoReturnsRepo checks GET /repos/{org}/{stub} returns one repo with
// its org stub, repo stub, and URL.
func TestGetRepoReturnsRepo(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{
		{Org: "acme", Stub: "gadgets", URL: "https://github.com/acme/gadgets"},
		{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"},
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets")
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

	var got repoBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := repoBody{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}
	if got != want {
		t.Errorf("repo = %#v, want %#v", got, want)
	}
}

// TestGetRepoMissing checks a request for a repo that does not exist returns
// the service error response.
func TestGetRepoMissing(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{
		{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"},
	}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/gadgets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetRepoReportsStoreFailure checks a store failure returns the service
// error response instead of a repo.
func TestGetRepoReportsStoreFailure(t *testing.T) {
	base := startServiceWithStore(t, &fakeDB{getErr: errors.New("boom")}, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetRepoDeniedForUnpermittedRole checks a role without read access reaches
// no repo.
func TestGetRepoDeniedForUnpermittedRole(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPutRepoCreatesRepo checks PUT /repos/{org}/{stub} creates a missing repo
// from the body's URL, returns 201, and makes it readable.
func TestPutRepoCreatesRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendRepoPut(t, base, "repos/acme/widgets", `{"url":"https://github.com/acme/widgets"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var got repoBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := repoBody{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}
	if got != want {
		t.Errorf("repo = %#v, want %#v", got, want)
	}

	read, err := http.Get(base + "repos/acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	defer read.Body.Close()
	if read.StatusCode != http.StatusOK {
		t.Errorf("later GET status = %d, want %d", read.StatusCode, http.StatusOK)
	}
}

// TestPutRepoSameStubsIsOneRepo checks two writes to the same stubs, with
// different URLs, update one repo rather than create a second.
func TestPutRepoSameStubsIsOneRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	create := sendRepoPut(t, base, "repos/acme/widgets", `{"url":"https://old.example/acme/widgets"}`)
	create.Body.Close()
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", create.StatusCode, http.StatusCreated)
	}

	update := sendRepoPut(t, base, "repos/acme/widgets", `{"url":"https://github.com/acme/widgets"}`)
	defer update.Body.Close()
	if update.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want %d", update.StatusCode, http.StatusOK)
	}

	var got repoBody
	if err := json.NewDecoder(update.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := repoBody{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}
	if got != want {
		t.Errorf("repo = %#v, want the updated URL %#v", got, want)
	}

	list, err := http.Get(base + "repos")
	if err != nil {
		t.Fatal(err)
	}
	defer list.Body.Close()
	var repos []repoBody
	if err := json.NewDecoder(list.Body).Decode(&repos); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("ListRepos returned %d repos, want 1", len(repos))
	}
}

// TestPutRepoUpdateMergesBody checks an update whose body omits the URL keeps
// the stored URL and returns 200, so the body merges into the existing repo.
func TestPutRepoUpdateMergesBody(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{
		{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"},
	}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	update := sendRepoPut(t, base, "repos/acme/widgets", `{}`)
	defer update.Body.Close()
	if update.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", update.StatusCode, http.StatusOK)
	}

	var got repoBody
	if err := json.NewDecoder(update.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := repoBody{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}
	if got != want {
		t.Errorf("repo = %#v, want the stored URL %#v", got, want)
	}
	if len(st.repos) != 1 {
		t.Fatalf("store holds %d repos, want 1", len(st.repos))
	}
}

// TestPutRepoRejectsMissingURLCreate checks a create without a URL returns 422
// and writes no repo.
func TestPutRepoRejectsMissingURLCreate(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendRepoPut(t, base, "repos/acme/widgets", `{}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	if len(st.repos) != 0 {
		t.Errorf("store holds %d repos, want none", len(st.repos))
	}
}

// TestPutRepoRejectsUnsafeStub checks a stub that is not URL-safe returns 422
// and writes no repo.
func TestPutRepoRejectsUnsafeStub(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendRepoPut(t, base, "repos/acme/a%2Fb", `{"url":"https://github.com/acme/a"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	if len(st.repos) != 0 {
		t.Errorf("store holds %d repos, want none", len(st.repos))
	}
}

// TestPutRepoEmptyStubNotFound checks a write with an empty repo stub reaches
// no repo route.
func TestPutRepoEmptyStubNotFound(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendRepoPut(t, base, "repos/acme/", `{"url":"https://github.com/acme/widgets"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if len(st.repos) != 0 {
		t.Errorf("store holds %d repos, want none", len(st.repos))
	}
}

// TestPutRepoReportsStoreFailure checks a store failure returns the service
// error response.
func TestPutRepoReportsStoreFailure(t *testing.T) {
	st := &fakeDB{saveErr: errors.New("boom")}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendRepoPut(t, base, "repos/acme/widgets", `{"url":"https://github.com/acme/widgets"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestPutRepoDeniedForUnpermittedRole checks a role without write access reaches
// no repo write.
func TestPutRepoDeniedForUnpermittedRole(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp := sendRepoPut(t, base, "repos/acme/widgets", `{"url":"https://github.com/acme/widgets"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if len(st.repos) != 0 {
		t.Errorf("store holds %d repos, want none", len(st.repos))
	}
}

// sendRepoPut sends a repo write to the service and returns the response.
func sendRepoPut(t *testing.T, base, path, body string) *http.Response {
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

// startServiceWithStore runs the service over st and authenticates every
// request with the given role.
func startServiceWithStore(t *testing.T, st Store, role string) string {
	t.Helper()
	port := freePort(t)
	return startServiceWith(t, port, Options{
		ConfigPath:    writeConfig(t, validConfig(port)),
		Authenticator: roleAuthenticator(role),
		Opener:        func(context.Context, devconfig.Postgres) (Store, error) { return st, nil },
	})
}

// roleAuthenticator authenticates every request with one role.
func roleAuthenticator(role string) auth.Authenticator {
	return func(*http.Request) (auth.Caller, error) {
		return auth.Caller{Subject: "caller", Roles: []string{role}}, nil
	}
}
