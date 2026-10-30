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
// tells a number from any other JSON value; Grader is the grader that produced
// it; the rest are its logs and run metrics. A request that omits its grader
// creates no such record; a grader it carries must have a name, stub, commit,
// and branch.
type scoreRequest struct {
	Task         string         `json:"task"`
	Test         string         `json:"test"`
	Result       string         `json:"result"`
	Model        *store.Model   `json:"model"`
	Grader       *store.Grader  `json:"grader"`
	Dictionary   map[string]any `json:"dictionary"`
	Logs         string         `json:"logs"`
	InputTokens  int            `json:"inputTokens"`
	OutputTokens int            `json:"outputTokens"`
	TokenCost    float64        `json:"tokenCost"`
	Duration     float64        `json:"duration"`
}

// score is a score as the service API returns it.
type score struct {
	Ref          string             `json:"ref"`
	Task         string             `json:"task"`
	Test         string             `json:"test"`
	Result       string             `json:"result"`
	Model        store.Model        `json:"model"`
	Grader       store.Grader       `json:"grader"`
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
// so a negative run metric, a value that is not a number, a key outside the
// test's score schema, an incomplete model or grader, or a task or test that
// does not match the result's is rejected with 422 without creating a score,
// and a missing result writes the service error response.
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
		if body.Model != nil {
			if err := body.Model.Validate(); err != nil {
				writeScoreStoreError(w, err)
				return
			}
		}
		if body.Grader != nil {
			if err := body.Grader.Validate(); err != nil {
				writeScoreStoreError(w, err)
				return
			}
		}

		result, ok := requireResult(w, st, org, repo, body.Result)
		if !ok {
			return
		}
		if result.Task != body.Task || result.Test != body.Test {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "score task and test must match its result")
			return
		}

		if _, ok := requireTask(w, st, org, repo, body.Task); !ok {
			return
		}
		test, ok := requireTest(w, st, org, repo, body.Task, body.Test)
		if !ok {
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

// listScores writes the refs of the scores in the repo the route identifies as
// a JSON array, or the service error response when the repo does not exist.
func listScores(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		scores, err := st.ListScores(org, repo)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list scores")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, scoreRefs(scores))
	}
}

// scoreRefs returns the score refs as the API represents them. An empty list is
// an empty JSON array rather than null.
func scoreRefs(scores []store.Score) []string {
	refs := make([]string, 0, len(scores))
	for _, sc := range scores {
		refs = append(refs, sc.Ref)
	}
	return refs
}

// getScore writes one score as JSON, or the service error response when the
// repo or score does not exist.
func getScore(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}

		sc, ok := requireScore(w, st, org, repo, r.PathValue("ref"))
		if !ok {
			return
		}
		httpx.WriteJSON(w, http.StatusOK, scoreJSON(sc))
	}
}

// requireScore returns the score the route identifies under the repo, writing
// the service error response and reporting false when it does not exist.
func requireScore(w http.ResponseWriter, st Store, org, repo, ref string) (store.Score, bool) {
	sc, err := st.GetScore(org, repo, ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "score not found")
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get score")
		}
		return store.Score{}, false
	}
	return sc, true
}

// scoreJSON returns one score as the API represents it. An empty dictionary
// comes back as null.
func scoreJSON(sc store.Score) score {
	return score{
		Ref: sc.Ref, Task: sc.Task, Test: sc.Test, Result: sc.Result, Model: sc.Model,
		Grader: sc.Grader, Dictionary: sc.Dictionary, Logs: sc.Logs,
		InputTokens: sc.InputTokens, OutputTokens: sc.OutputTokens,
		TokenCost: sc.TokenCost, Duration: sc.Duration,
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

// newScore builds the store score a body describes for the route's repo,
// including the model and grader it carries when it names them.
func newScore(org, repo string, body scoreRequest, dictionary map[string]float64) store.Score {
	sc := store.Score{
		Org: org, Repo: repo, Task: body.Task, Test: body.Test, Result: body.Result,
		Dictionary: dictionary, Logs: body.Logs,
		InputTokens: body.InputTokens, OutputTokens: body.OutputTokens,
		TokenCost: body.TokenCost, Duration: body.Duration,
	}
	if body.Model != nil {
		sc.Model = *body.Model
	}
	if body.Grader != nil {
		sc.Grader = *body.Grader
	}
	return sc
}

// writeScoreStoreError writes the service error response for a score store
// failure. A score, model, or grader the store rejects is a client error.
func writeScoreStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidModel):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "model must carry a name and provider")
	case errors.Is(err, store.ErrInvalidGrader):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "grader must carry a name, stub, commit, and branch")
	case errors.Is(err, store.ErrInvalidScore):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "score metrics must be zero or greater")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "failed to save score")
	}
}
