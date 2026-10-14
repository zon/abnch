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

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Message == "" {
		t.Error("error response body has no message")
	}
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
