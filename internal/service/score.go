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
// key-number dictionary the grader produced; the rest are its logs and run
// metrics.
type scoreRequest struct {
	Task         string             `json:"task"`
	Test         string             `json:"test"`
	Result       string             `json:"result"`
	Dictionary   map[string]float64 `json:"dictionary"`
	Logs         string             `json:"logs"`
	InputTokens  int                `json:"inputTokens"`
	OutputTokens int                `json:"outputTokens"`
	TokenCost    float64            `json:"tokenCost"`
	Duration     float64            `json:"duration"`
}

// addScore stores the score the request body carries for its result and writes
// its ref. It answers 201 for a new ref and 200 when the ref already exists,
// without creating a second score. A score the store rejects is a client error,
// so a negative run metric is rejected with 422 without creating a score, and a
// missing result writes the service error response.
func addScore(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		var body scoreRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Dictionary == nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid score body")
			return
		}

		if _, ok := requireTask(w, st, org, repo, body.Task); !ok {
			return
		}
		if _, ok := requireTest(w, st, org, repo, body.Task, body.Test); !ok {
			return
		}
		if _, ok := requireResult(w, st, org, repo, body.Result); !ok {
			return
		}

		score := newScore(org, repo, body)
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

// newScore builds the store score a body describes for the route's repo.
func newScore(org, repo string, body scoreRequest) store.Score {
	return store.Score{
		Org: org, Repo: repo, Task: body.Task, Test: body.Test, Result: body.Result,
		Dictionary: body.Dictionary, Logs: body.Logs,
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
