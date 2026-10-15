package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// scoreRef is a score's ref as the service API returns it.
type scoreRef struct {
	Ref string `json:"ref"`
}

// scoreRequest is the JSON body a score write carries. Dictionary is the
// key-number dictionary the grader produced, carried untyped so the service
// tells a number from any other JSON value; the rest are its logs and run
// metrics.
type scoreRequest struct {
	Task         string         `json:"task"`
	Test         string         `json:"test"`
	Result       string         `json:"result"`
	Dictionary   map[string]any `json:"dictionary"`
	Logs         string         `json:"logs"`
	InputTokens  int            `json:"inputTokens"`
	OutputTokens int            `json:"outputTokens"`
	TokenCost    float64        `json:"tokenCost"`
	Duration     float64        `json:"duration"`
}

// addScore stores the score the request body carries for its result and writes
// its ref. It answers 201 for a new ref and 200 when the ref already exists,
// without creating a second score. A score the store rejects is a client error,
// so a negative run metric, a value that is not a number, or a key outside the
// test's score schema is rejected with 422 without creating a score, and a
// missing result writes the service error response.
func addScore(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		var body scoreRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid score body")
			return
		}
		dictionary, ok := numberDictionary(body.Dictionary)
		if !ok {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid score body")
			return
		}

		if _, ok := requireTask(w, st, org, repo, body.Task); !ok {
			return
		}
		test, ok := requireTest(w, st, org, repo, body.Task, body.Test)
		if !ok {
			return
		}
		if _, ok := requireResult(w, st, org, repo, body.Result); !ok {
			return
		}
		if !test.AllowsScore(dictionary) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "score keys must be in the test score schema")
			return
		}

		score := newScore(org, repo, body, dictionary)
		if err := score.Validate(); err != nil {
			writeScoreStoreError(w, err)
			return
		}

		ref := store.ScoreRef(score)
		_, err := st.GetScore(org, repo, ref)
		switch {
		case err == nil:
			httpx.WriteJSON(w, http.StatusOK, scoreRef{Ref: ref})
			return
		case !errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get score")
			return
		}

		ref, err = st.SaveScore(score)
		if err != nil {
			writeScoreStoreError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, scoreRef{Ref: ref})
	}
}

// numberDictionary returns the request's dictionary as numbers, reporting false
// when the body carries none or one of its values is not a JSON number.
func numberDictionary(raw map[string]any) (map[string]float64, bool) {
	if raw == nil {
		return nil, false
	}
	dictionary := make(map[string]float64, len(raw))
	for key, value := range raw {
		number, ok := value.(float64)
		if !ok {
			return nil, false
		}
		dictionary[key] = number
	}
	return dictionary, true
}

// newScore builds the store score a body describes for the route's repo.
func newScore(org, repo string, body scoreRequest, dictionary map[string]float64) store.Score {
	return store.Score{
		Org: org, Repo: repo, Task: body.Task, Test: body.Test, Result: body.Result,
		Dictionary: dictionary, Logs: body.Logs,
		InputTokens: body.InputTokens, OutputTokens: body.OutputTokens,
		TokenCost: body.TokenCost, Duration: body.Duration,
	}
}

// writeScoreStoreError writes the service error response for a score store
// failure. A score the store rejects is a client error.
func writeScoreStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrInvalidScore) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "score metrics must be zero or greater")
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, "failed to save score")
}
