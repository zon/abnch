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

// TestPutTaskCreatesTask checks PUT /repos/{org}/{repo}/tasks/{stub} creates a
// missing task from the body, returns 201, and makes it readable.
func TestPutTaskCreatesTask(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", `{"name":"Fix a bug","description":"A broken widget"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var got taskBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := taskBody{Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget", Repo: "widgets"}
	if got != want {
		t.Errorf("task = %#v, want %#v", got, want)
	}

	read, err := http.Get(base + "repos/acme/widgets/tasks/fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	defer read.Body.Close()
	if read.StatusCode != http.StatusOK {
		t.Errorf("later GET status = %d, want %d", read.StatusCode, http.StatusOK)
	}
}

// TestPutTaskUpdatesExistingTask checks a write to a stored task merges the body
// into it, returns 200, and replaces the fields the body carries.
func TestPutTaskUpdatesExistingTask(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "old"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", `{"description":"new"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got taskBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := taskBody{Stub: "fix-bug", Name: "Fix a bug", Description: "new", Repo: "widgets"}
	if got != want {
		t.Errorf("task = %#v, want the description replaced and the name kept %#v", got, want)
	}
	if len(st.tasks) != 1 {
		t.Errorf("store holds %d tasks, want 1", len(st.tasks))
	}
}

// TestPutTaskUpdateReplacesName checks an update whose body carries a name
// replaces the stored name while keeping the stored description.
func TestPutTaskUpdateReplacesName(t *testing.T) {
	st := &fakeDB{
		repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		tasks: []store.Task{
			{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		},
	}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", `{"name":"Fix the bug"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got taskBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := taskBody{Stub: "fix-bug", Name: "Fix the bug", Description: "A broken widget", Repo: "widgets"}
	if got != want {
		t.Errorf("task = %#v, want the name replaced and the description kept %#v", got, want)
	}
}

// TestPutTaskRejectsMissingFieldCreate checks a create that misses a required
// field returns 422 and writes no task.
func TestPutTaskRejectsMissingFieldCreate(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing name", body: `{"description":"A broken widget"}`},
		{name: "missing description", body: `{"name":"Fix a bug"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
			base := startServiceWithStore(t, st, auth.RoleAdmin)

			resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", tc.body)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
			}
			if len(st.tasks) != 0 {
				t.Errorf("store holds %d tasks, want none", len(st.tasks))
			}
		})
	}
}

// TestPutTaskMissingRepo checks a write to a repo that does not exist returns
// the service error response and creates neither a repo nor a task.
func TestPutTaskMissingRepo(t *testing.T) {
	st := &fakeDB{}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", `{"name":"Fix a bug","description":"A broken widget"}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusNotFound)
	if len(st.repos) != 0 || len(st.tasks) != 0 {
		t.Errorf("store holds %d repos and %d tasks, want none", len(st.repos), len(st.tasks))
	}
}

// TestPutTaskRejectsUnsafeStub checks a task stub that is not URL-safe returns
// 422 and writes no task.
func TestPutTaskRejectsUnsafeStub(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/a%2Fb", `{"name":"Fix a bug","description":"A broken widget"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	if len(st.tasks) != 0 {
		t.Errorf("store holds %d tasks, want none", len(st.tasks))
	}
}

// TestPutTaskReportsStoreFailure checks a store failure returns the service
// error response.
func TestPutTaskReportsStoreFailure(t *testing.T) {
	st := &fakeDB{
		repos:       []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}},
		saveTaskErr: errors.New("boom"),
	}
	base := startServiceWithStore(t, st, auth.RoleAdmin)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", `{"name":"Fix a bug","description":"A broken widget"}`)
	defer resp.Body.Close()

	assertServiceError(t, resp, http.StatusInternalServerError)
}

// TestPutTaskDeniedForUnpermittedRole checks a role without write access reaches
// no task write.
func TestPutTaskDeniedForUnpermittedRole(t *testing.T) {
	st := &fakeDB{repos: []store.Repo{{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}}}
	base := startServiceWithStore(t, st, auth.RoleReader)

	resp := sendTaskPut(t, base, "repos/acme/widgets/tasks/fix-bug", `{"name":"Fix a bug","description":"A broken widget"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if len(st.tasks) != 0 {
		t.Errorf("store holds %d tasks, want none", len(st.tasks))
	}
}

// sendTaskPut sends a task write to the service and returns the response.
func sendTaskPut(t *testing.T, base, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
