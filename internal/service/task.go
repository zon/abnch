package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// task is a task as the service API returns it.
type task struct {
	Stub        string `json:"stub"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
}

// taskRequest is the JSON body a task write carries.
type taskRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// moveTaskRequest is the JSON body a task move carries.
type moveTaskRequest struct {
	Stub string `json:"stub"`
}

// listTasks writes the tasks of the repo the route identifies as a JSON array,
// or the service error response when the repo does not exist.
func listTasks(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		rp, ok := requireRepo(w, st, org, repo)
		if !ok {
			return
		}

		tasks, err := st.ListTasks(org, repo)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list tasks")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, tasksJSON(tasks, rp.URL))
	}
}

// getTask writes one task as JSON, or the service error response when the repo
// or the task does not exist.
func getTask(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		rp, ok := requireRepo(w, st, org, repo)
		if !ok {
			return
		}

		t, err := st.GetTask(org, repo, r.PathValue("stub"))
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "task not found")
			return
		}
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get task")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, taskJSON(t, rp.URL))
	}
}

// putTask creates or updates the task the route identifies and writes it as
// JSON. It answers 201 when it creates a task and 200 when it updates one. A
// create that misses a required field is rejected without writing a task.
func putTask(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		rp, ok := requireRepo(w, st, org, repo)
		if !ok {
			return
		}

		var body taskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid task body")
			return
		}

		stub := r.PathValue("stub")
		existing, err := st.GetTask(org, repo, stub)
		switch {
		case errors.Is(err, store.ErrNotFound):
			if body.Name == "" || body.Description == "" {
				httpx.WriteError(w, http.StatusUnprocessableEntity, "task name and description are required")
				return
			}
			writeSavedTask(w, st, store.Task{Org: org, Repo: repo, Stub: stub, Name: body.Name, Description: body.Description}, rp.URL, http.StatusCreated)
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get task")
		default:
			if body.Name != "" {
				existing.Name = body.Name
			}
			if body.Description != "" {
				existing.Description = body.Description
			}
			writeSavedTask(w, st, existing, rp.URL, http.StatusOK)
		}
	}
}

// moveTask renames the task the route identifies to the body's stub and writes
// the moved task as JSON. A missing task is a 404, a stub already held by
// another task is a 409, and a stub the store rejects is a client error.
func moveTask(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")
		rp, ok := requireRepo(w, st, org, repo)
		if !ok {
			return
		}

		var body moveTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid task body")
			return
		}

		moved, err := st.MoveTask(org, repo, r.PathValue("stub"), body.Stub)
		switch {
		case errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "task not found")
		case errors.Is(err, store.ErrConflict):
			httpx.WriteError(w, http.StatusConflict, "task stub is taken")
		case errors.Is(err, store.ErrInvalidStub):
			httpx.WriteError(w, http.StatusUnprocessableEntity, "task stub is not URL-safe")
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to move task")
		default:
			httpx.WriteJSON(w, http.StatusOK, taskJSON(moved, rp.URL))
		}
	}
}

// requireRepo returns the repo the route identifies, writing the service error
// response and reporting false when it does not exist.
func requireRepo(w http.ResponseWriter, st Store, org, repo string) (store.Repo, bool) {
	rp, err := st.GetRepo(org, repo)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "repo not found")
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get repo")
		}
		return store.Repo{}, false
	}
	return rp, true
}

// writeSavedTask saves a task and writes it with the given status. A store
// failure writes the service error response instead. A stub the store rejects
// is a client error.
func writeSavedTask(w http.ResponseWriter, st Store, t store.Task, repoURL string, status int) {
	if err := st.SaveTask(t); err != nil {
		if errors.Is(err, store.ErrInvalidStub) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "task stub is not URL-safe")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "failed to save task")
		return
	}
	httpx.WriteJSON(w, status, taskJSON(t, repoURL))
}

// tasksJSON returns the tasks as the API represents them. Each task carries the
// URL of the repo that holds it.
func tasksJSON(tasks []store.Task, repoURL string) []task {
	out := make([]task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, taskJSON(t, repoURL))
	}
	return out
}

// taskJSON returns one task as the API represents it. The repo field carries the
// URL of the repo that holds the task.
func taskJSON(t store.Task, repoURL string) task {
	return task{Stub: t.Stub, Name: t.Name, Description: t.Description, Repo: repoURL}
}
