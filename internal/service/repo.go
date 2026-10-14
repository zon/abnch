package service

import (
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
