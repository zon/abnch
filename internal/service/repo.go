package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// repo is a repo as the service API returns it.
type repo struct {
	Org  string `json:"org"`
	Stub string `json:"stub"`
	URL  string `json:"url"`
}

// repoRequest is the JSON body a repo write carries.
type repoRequest struct {
	URL string `json:"url"`
}

// listRepos writes the repos the caller can see as a JSON array.
func listRepos(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		repos, err := st.ListRepos()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list repos")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, reposJSON(repos))
	}
}

// getRepo writes one repo as JSON, or the service error response when it does
// not exist.
func getRepo(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rp, err := st.GetRepo(r.PathValue("org"), r.PathValue("stub"))
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "repo not found")
			return
		}
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get repo")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, repoJSON(rp))
	}
}

// putRepo creates or updates the repo the route's stubs identify and writes it
// as JSON. It answers 201 when it creates a repo and 200 when it updates one.
// A create without a URL, or a stub that is not URL-safe, is rejected without
// writing a repo.
func putRepo(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, stub := r.PathValue("org"), r.PathValue("stub")

		var body repoRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid repo body")
			return
		}

		existing, err := st.GetRepo(org, stub)
		switch {
		case errors.Is(err, store.ErrNotFound):
			if body.URL == "" {
				httpx.WriteError(w, http.StatusUnprocessableEntity, "repo url is required")
				return
			}
			writeSavedRepo(w, st, store.Repo{Org: org, Stub: stub, URL: body.URL}, http.StatusCreated)
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get repo")
		default:
			if body.URL != "" {
				existing.URL = body.URL
			}
			writeSavedRepo(w, st, existing, http.StatusOK)
		}
	}
}

// writeSavedRepo saves a repo and writes it with the given status. A store
// failure writes the service error response instead.
func writeSavedRepo(w http.ResponseWriter, st Store, rp store.Repo, status int) {
	if err := st.SaveRepo(rp); err != nil {
		writeRepoStoreError(w, err)
		return
	}
	httpx.WriteJSON(w, status, repoJSON(rp))
}

// writeRepoStoreError writes the service error response for a repo store
// failure. A stub the store rejects is a client error.
func writeRepoStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrInvalidStub) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "repo stub is not URL-safe")
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, "failed to save repo")
}

// reposJSON returns the repos as the API represents them.
func reposJSON(repos []store.Repo) []repo {
	out := make([]repo, 0, len(repos))
	for _, r := range repos {
		out = append(out, repoJSON(r))
	}
	return out
}

// repoJSON returns one repo as the API represents it.
func repoJSON(r store.Repo) repo {
	return repo{Org: r.Org, Stub: r.Stub, URL: r.URL}
}
