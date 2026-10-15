package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// dataRef is a datum's ref as the service API returns it.
type dataRef struct {
	Ref string `json:"ref"`
}

// addData stores the JSON the request body carries as a datum and writes its
// ref. It answers 201 for a new ref and 200 when the ref already exists, without
// creating a second datum. A malformed body is a client error, and a missing
// repo, task, or test writes the service error response.
func addData(st Store) http.HandlerFunc {
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

		body, err := io.ReadAll(r.Body)
		if err != nil || !json.Valid(body) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid datum body")
			return
		}

		ref := store.DatumRef(body)
		_, err = st.GetDatum(org, repo, task, test, ref)
		switch {
		case err == nil:
			httpx.WriteJSON(w, http.StatusOK, dataRef{Ref: ref})
			return
		case !errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get datum")
			return
		}

		ref, err = st.SaveDatum(org, repo, task, test, body)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to save datum")
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, dataRef{Ref: ref})
	}
}

// getData writes one datum's JSON unchanged, or the service error response when
// the repo, task, test, or datum does not exist.
func getData(st Store) http.HandlerFunc {
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

		d, ok := requireDatum(w, st, org, repo, task, test, r.PathValue("ref"))
		if !ok {
			return
		}
		httpx.WriteRawJSON(w, http.StatusOK, []byte(d.JSON))
	}
}

// requireDatum returns the datum the route identifies under the test, writing
// the service error response and reporting false when it does not exist.
func requireDatum(w http.ResponseWriter, st Store, org, repo, task, test, ref string) (store.Datum, bool) {
	d, err := st.GetDatum(org, repo, task, test, ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "datum not found")
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get datum")
		}
		return store.Datum{}, false
	}
	return d, true
}

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
