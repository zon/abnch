package service

import (
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

// listTasks writes the tasks of the repo the route identifies as a JSON array,
// or the service error response when the repo does not exist.
func listTasks(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")

		_, err := st.GetRepo(org, repo)
		switch {
		case errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "repo not found")
			return
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get repo")
			return
		}

		tasks, err := st.ListTasks(org, repo)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "failed to list tasks")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, tasksJSON(tasks))
	}
}

// getTask writes one task as JSON, or the service error response when the repo
// or the task does not exist.
func getTask(st Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, repo := r.PathValue("org"), r.PathValue("repo")

		_, err := st.GetRepo(org, repo)
		switch {
		case errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "repo not found")
			return
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, "failed to get repo")
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
		httpx.WriteJSON(w, http.StatusOK, taskJSON(t))
	}
}

// tasksJSON returns the tasks as the API represents them.
func tasksJSON(tasks []store.Task) []task {
	out := make([]task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, taskJSON(t))
	}
	return out
}

// taskJSON returns one task as the API represents it.
func taskJSON(t store.Task) task {
	return task{Stub: t.Stub, Name: t.Name, Description: t.Description, Repo: t.Repo}
}
