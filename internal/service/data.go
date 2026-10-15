package service

import (
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// listData writes the refs of the data in the test the route identifies as a
// JSON array, or the service error response when the repo, task, or test does
// not exist.
func listData(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}
		task := r.PathValue("task")
		if _, ok := requireTask(w, st, org, repo, task); !ok {
			return
		}
		test := r.PathValue("test")
		if _, ok := requireTest(w, st, org, repo, task, test); !ok {
			return
		}

		data, err := st.ListData(org, repo, task, test)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list data")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dataRefs(data))
	}
}

// dataRefs returns the data refs as the API represents them. An empty list is an
// empty JSON array rather than null.
func dataRefs(data []store.Datum) []string {
	refs := make([]string, 0, len(data))
	for _, d := range data {
		refs = append(refs, d.Ref)
	}
	return refs
}
