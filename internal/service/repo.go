package service

import (
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

// reposJSON returns the repos as the API represents them.
func reposJSON(repos []store.Repo) []repo {
	out := make([]repo, 0, len(repos))
	for _, r := range repos {
		out = append(out, repo{Org: r.Org, Stub: r.Stub, URL: r.URL})
	}
	return out
}
