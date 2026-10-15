package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// test is a test as the service API returns it.
type test struct {
	Stub        string            `json:"stub"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Schema      []store.SchemaKey `json:"schema"`
}

// testRequest is the JSON body a test write carries. Schema is a pointer so the
// service tells a body that carries one from a body that omits it.
type testRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Version     string             `json:"version"`
	Schema      *[]store.SchemaKey `json:"schema"`
}

// listTests writes the tests of the task the route identifies as a JSON array,
// or the service error response when the repo or task does not exist.
func listTests(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}
		task := r.PathValue("task")
		if _, ok := requireTask(w, st, org, repo, task); !ok {
			return
		}

		tests, err := st.ListTests(org, repo, task)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list tests")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, testsJSON(tests))
	}
}

// putTest creates or updates the test the route identifies and writes it as
// JSON. It answers 201 when it creates a test and 200 when it updates one. A
// write the store rejects is a client error, so a create that misses a required
// field is rejected without writing a test.
func putTest(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		if _, ok := requireRepo(w, st, org, repo); !ok {
			return
		}
		task := r.PathValue("task")
		if _, ok := requireTask(w, st, org, repo, task); !ok {
			return
		}

		var body testRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid test body")
			return
		}

		stub := r.PathValue("stub")
		existing, err := st.GetTest(org, repo, task, stub)
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeSavedTest(w, st, newTest(org, repo, task, stub, body), http.StatusCreated)
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get test")
		default:
			writeSavedTest(w, st, mergedTest(existing, body), http.StatusOK)
		}
	}
}

// newTest builds the test a body describes for the route's stubs.
func newTest(org, repo, task, stub string, body testRequest) store.Test {
	t := store.Test{
		Org: org, Repo: repo, Task: task, Stub: stub,
		Name: body.Name, Description: body.Description, Version: body.Version,
	}
	if body.Schema != nil {
		t.Schema = *body.Schema
	}
	return t
}

// mergedTest returns the stored test with the fields the body carries replaced.
// A body that carries a schema replaces the stored schema.
func mergedTest(t store.Test, body testRequest) store.Test {
	if body.Name != "" {
		t.Name = body.Name
	}
	if body.Description != "" {
		t.Description = body.Description
	}
	if body.Version != "" {
		t.Version = body.Version
	}
	if body.Schema != nil {
		t.Schema = *body.Schema
	}
	return t
}

// writeSavedTest saves a test and writes it with the given status. A store
// failure writes the service error response instead. A test the store rejects
// is a client error.
func writeSavedTest(w http.ResponseWriter, st Store, t store.Test, status int) {
	if err := st.SaveTest(t); err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidTest):
			httpx.WriteError(w, http.StatusUnprocessableEntity, "test name and version are required")
		case errors.Is(err, store.ErrInvalidSchema):
			httpx.WriteError(w, http.StatusUnprocessableEntity, "score schema key stubs must be unique")
		case errors.Is(err, store.ErrInvalidStub):
			httpx.WriteError(w, http.StatusUnprocessableEntity, "test stub is not URL-safe")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to save test")
		}
		return
	}
	httpx.WriteJSON(w, status, testJSON(t))
}

// testsJSON returns the tests as the API represents them. An empty list is an
// empty JSON array rather than null.
func testsJSON(tests []store.Test) []test {
	out := make([]test, 0, len(tests))
	for _, t := range tests {
		out = append(out, testJSON(t))
	}
	return out
}

// testJSON returns one test as the API represents it. An empty schema is an
// empty JSON array rather than null.
func testJSON(t store.Test) test {
	schema := t.Schema
	if schema == nil {
		schema = []store.SchemaKey{}
	}
	return test{Stub: t.Stub, Name: t.Name, Description: t.Description, Version: t.Version, Schema: schema}
}
