package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// resultRef is a result's ref as the service API returns it.
type resultRef struct {
	Ref string `json:"ref"`
}

// result is a result as the service API returns it.
type result struct {
	Ref          string          `json:"ref"`
	Task         string          `json:"task"`
	Test         string          `json:"test"`
	Data         string          `json:"data"`
	JSON         json.RawMessage `json:"json"`
	Logs         string          `json:"logs"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	TokenCost    float64         `json:"tokenCost"`
	Duration     float64         `json:"duration"`
	Local        string          `json:"local"`
}

// resultRequest is the JSON body a result write carries. JSON is the result the
// agent produced; the rest are its logs and run metrics.
type resultRequest struct {
	Task         string          `json:"task"`
	Test         string          `json:"test"`
	Data         string          `json:"data"`
	JSON         json.RawMessage `json:"json"`
	Logs         string          `json:"logs"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	TokenCost    float64         `json:"tokenCost"`
	Duration     float64         `json:"duration"`
	Local        string          `json:"local"`
}

// addResult stores the result the request body carries for its datum and writes
// its ref. It answers 201 for a new ref and 200 when the ref already exists,
// without creating a second result. A result the store rejects is a client
// error, so a negative run metric is rejected with 422 without creating a
// result, and a missing datum writes the service error response.
func addResult(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		var body resultRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.JSON) == 0 {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid result body")
			return
		}

		if _, ok := requireTask(w, st, org, repo, body.Task); !ok {
			return
		}
		if _, ok := requireTest(w, st, org, repo, body.Task, body.Test); !ok {
			return
		}
		if _, ok := requireDatum(w, st, org, repo, body.Task, body.Test, body.Data); !ok {
			return
		}

		result := newResult(org, repo, body)
		if err := result.Validate(); err != nil {
			writeResultStoreError(w, err)
			return
		}

		ref := store.ResultRef(result)
		_, err := st.GetResult(org, repo, ref)
		switch {
		case err == nil:
			httpx.WriteJSON(w, http.StatusOK, resultRef{Ref: ref})
			return
		case !errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get result")
			return
		}

		ref, err = st.SaveResult(result)
		if err != nil {
			writeResultStoreError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, resultRef{Ref: ref})
	}
}

// listResults writes the refs of the results in the repo the route identifies
// as a JSON array, or the service error response when the repo does not exist.
func listResults(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		results, err := st.ListResults(org, repo)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list results")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, resultRefs(results))
	}
}

// getResult writes one result as JSON, or the service error response when the
// repo or result does not exist.
func getResult(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		res, err := st.GetResult(org, repo, r.PathValue("ref"))
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "result not found")
			return
		}
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get result")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, resultJSON(res))
	}
}

// resultJSON returns one result as the API represents it. An empty result JSON
// comes back as null rather than breaking the response.
func resultJSON(r store.Result) result {
	body := json.RawMessage(r.JSON)
	if len(body) == 0 {
		body = json.RawMessage("null")
	}
	return result{
		Ref: r.Ref, Task: r.Task, Test: r.Test, Data: r.Data,
		JSON: body, Logs: r.Logs,
		InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		TokenCost: r.TokenCost, Duration: r.Duration, Local: r.Local,
	}
}

// resultRefs returns the result refs as the API represents them. An empty list
// is an empty JSON array rather than null.
func resultRefs(results []store.Result) []string {
	refs := make([]string, 0, len(results))
	for _, r := range results {
		refs = append(refs, r.Ref)
	}
	return refs
}

// newResult builds the store result a body describes for the route's repo.
func newResult(org, repo string, body resultRequest) store.Result {
	return store.Result{
		Org: org, Repo: repo, Task: body.Task, Test: body.Test, Data: body.Data,
		JSON: string(body.JSON), Logs: body.Logs,
		InputTokens: body.InputTokens, OutputTokens: body.OutputTokens,
		TokenCost: body.TokenCost, Duration: body.Duration, Local: body.Local,
	}
}

// writeResultStoreError writes the service error response for a result store
// failure. A result the store rejects is a client error.
func writeResultStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrInvalidResult) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "result metrics must be zero or greater")
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, "failed to save result")
}
