package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/store"
)

// taskBody mirrors the JSON body the API returns for a task.
type taskBody struct {
	Stub        string `json:"stub"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
}

// TestGetTasksListsTasks checks GET /repos/{org}/{repo}/tasks returns the repo's
// tasks as a JSON array.
func TestGetTasksListsTasks(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "add-feature", Name: "Add a feature", Description: "A new widget"},
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got []taskBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := []taskBody{
		{Stub: "add-feature", Name: "Add a feature", Description: "A new widget", Repo: "widgets"},
		{Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget", Repo: "widgets"},
	}
	if len(got) != len(want) {
		t.Fatalf("body listed %d tasks, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("task %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

// TestGetTasksEmptyList checks a repo with no tasks returns an empty JSON array
// rather than a null body.
func TestGetTasksEmptyList(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(body)); got != "[]" {
		t.Errorf("body = %q, want %q", got, "[]")
	}
}

// TestGetTasksMissingRepo checks a request for the tasks of a repo that does not
// exist returns the service error response.
func TestGetTasksMissingRepo(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "gadgets", URL: "https://github.com/acme/gadgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetTasksReportsStoreFailure checks a store failure returns the service
// error response instead of a partial list.
func TestGetTasksReportsStoreFailure(t *testing.T) {
	st := &fakeDB{
		repos:        []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		listTasksErr: errors.New("boom"),
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetTasksDeniedForUnpermittedRole checks a role without read access reaches
// no task list.
func TestGetTasksDeniedForUnpermittedRole(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetTaskReturnsTask checks GET /repos/{org}/{repo}/tasks/{stub} returns one
// task with its stub, name, description, and repo.
func TestGetTaskReturnsTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "add-feature", Name: "Add a feature", Description: "A new widget"},
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got taskBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := taskBody{Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget", Repo: "widgets"}
	if got != want {
		t.Errorf("task = %#v, want %#v", got, want)
	}
}

// TestGetTaskMissing checks a request for a task that does not exist returns the
// service error response.
func TestGetTaskMissing(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/add-feature")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetTaskMissingRepo checks a request for a task in a repo that does not
// exist returns the service error response.
func TestGetTaskMissingRepo(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "gadgets", URL: "https://github.com/acme/gadgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
}

// TestGetTaskReportsStoreFailure checks a store failure returns the service
// error response instead of a task.
func TestGetTaskReportsStoreFailure(t *testing.T) {
	st := &fakeDB{
		repos:      []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		getTaskErr: errors.New("boom"),
	}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestGetTaskDeniedForUnpermittedRole checks a role without read access reaches
// no task.
func TestGetTaskDeniedForUnpermittedRole(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleAgent)

	resp, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
