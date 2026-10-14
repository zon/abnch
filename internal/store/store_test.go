package store

import (
	"errors"
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	s := newTestStore(t)

	for _, model := range []any{&Repo{}, &Task{}} {
		if !s.db.Migrator().HasTable(model) {
			t.Errorf("Migrate did not create the table for %T", model)
		}
	}
}

func TestSaveAndGetRepo(t *testing.T) {
	s := newTestStore(t)
	want := Repo{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}

	if err := s.SaveRepo(want); err != nil {
		t.Fatalf("SaveRepo: %v", err)
	}

	got, err := s.GetRepo("acme", "widgets")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if got != want {
		t.Errorf("GetRepo = %#v, want %#v", got, want)
	}
}

// TestSaveRepoUpdatesExistingRepo checks a second write to the same stubs
// replaces the repo instead of creating another one.
func TestSaveRepoUpdatesExistingRepo(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveRepo(Repo{Org: "acme", Stub: "widgets", URL: "https://old.example/acme/widgets"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRepo(Repo{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}); err != nil {
		t.Fatal(err)
	}

	repos, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("ListRepos returned %d repos, want 1", len(repos))
	}
	if repos[0].URL != "https://github.com/acme/widgets" {
		t.Errorf("URL = %q, want the updated URL", repos[0].URL)
	}
}

// TestSaveRepoRejectsInvalidStub checks a repo whose org stub or repo stub is
// empty or not URL-safe is not stored.
func TestSaveRepoRejectsInvalidStub(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo Repo
	}{
		{"empty org", Repo{Org: "", Stub: "widgets", URL: "https://example/acme/widgets"}},
		{"empty stub", Repo{Org: "acme", Stub: "", URL: "https://example/acme/widgets"}},
		{"separator in stub", Repo{Org: "acme", Stub: "a/b", URL: "https://example/acme/a/b"}},
		{"space in stub", Repo{Org: "acme", Stub: "a b", URL: "https://example/acme/a b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if err := s.SaveRepo(tc.repo); !errors.Is(err, ErrInvalidStub) {
				t.Errorf("SaveRepo error = %v, want ErrInvalidStub", err)
			}
			repos, err := s.ListRepos()
			if err != nil {
				t.Fatal(err)
			}
			if len(repos) != 0 {
				t.Errorf("ListRepos returned %d repos, want none", len(repos))
			}
		})
	}
}

// TestValidStub checks a stub is URL-safe only when it is non-empty and holds
// unreserved URL characters.
func TestValidStub(t *testing.T) {
	for _, tc := range []struct {
		stub string
		want bool
	}{
		{stub: "widgets", want: true},
		{stub: "acme-1._~", want: true},
		{stub: "", want: false},
		{stub: "a/b", want: false},
		{stub: "a b", want: false},
		{stub: "a%2Fb", want: false},
		{stub: "héllo", want: false},
	} {
		if got := ValidStub(tc.stub); got != tc.want {
			t.Errorf("ValidStub(%q) = %v, want %v", tc.stub, got, tc.want)
		}
	}
}

func TestListRepos(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveRepo(Repo{Org: "b", Stub: "two", URL: "https://example/two"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRepo(Repo{Org: "a", Stub: "one", URL: "https://example/one"}); err != nil {
		t.Fatal(err)
	}

	repos, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	want := []Repo{
		{Org: "a", Stub: "one", URL: "https://example/one"},
		{Org: "b", Stub: "two", URL: "https://example/two"},
	}
	if len(repos) != len(want) {
		t.Fatalf("ListRepos returned %d repos, want %d", len(repos), len(want))
	}
	for i := range want {
		if repos[i] != want[i] {
			t.Errorf("repo %d = %#v, want %#v", i, repos[i], want[i])
		}
	}
}

func TestGetRepoMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetRepo("acme", "widgets"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetRepo error = %v, want ErrNotFound", err)
	}
}

func TestSaveAndGetTask(t *testing.T) {
	s := newTestStore(t)
	want := Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"}

	if err := s.SaveTask(want); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	got, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got != want {
		t.Errorf("GetTask = %#v, want %#v", got, want)
	}
}

// TestSaveTaskUpdatesExistingTask checks a second write to the same stub
// replaces the task instead of creating another one.
func TestSaveTaskUpdatesExistingTask(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveTask(Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTask(Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "new"}); err != nil {
		t.Fatal(err)
	}

	tasks, err := s.ListTasks("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("ListTasks returned %d tasks, want 1", len(tasks))
	}
	if tasks[0].Description != "new" {
		t.Errorf("Description = %q, want the updated description", tasks[0].Description)
	}
}

// TestSaveTaskRejectsInvalidStub checks a task whose stub is empty or not
// URL-safe is not stored.
func TestSaveTaskRejectsInvalidStub(t *testing.T) {
	for _, tc := range []struct {
		name string
		stub string
	}{
		{"empty stub", ""},
		{"separator in stub", "a/b"},
		{"space in stub", "a b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			task := Task{Org: "acme", Repo: "widgets", Stub: tc.stub, Name: "Fix a bug"}
			if err := s.SaveTask(task); !errors.Is(err, ErrInvalidStub) {
				t.Errorf("SaveTask error = %v, want ErrInvalidStub", err)
			}
			tasks, err := s.ListTasks("acme", "widgets")
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != 0 {
				t.Errorf("ListTasks returned %d tasks, want none", len(tasks))
			}
		})
	}
}

func TestListTasks(t *testing.T) {
	s := newTestStore(t)
	for _, task := range []Task{
		{Org: "acme", Repo: "widgets", Stub: "two", Name: "Two"},
		{Org: "acme", Repo: "widgets", Stub: "one", Name: "One"},
		{Org: "acme", Repo: "gadgets", Stub: "other", Name: "Other"},
	} {
		if err := s.SaveTask(task); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := s.ListTasks("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("ListTasks returned %d tasks, want 2", len(tasks))
	}
	if tasks[0].Stub != "one" || tasks[1].Stub != "two" {
		t.Errorf("ListTasks stubs = %q, want [one two]", []string{tasks[0].Stub, tasks[1].Stub})
	}
}

func TestGetTaskMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetTask("acme", "widgets", "fix-bug"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTask error = %v, want ErrNotFound", err)
	}
}
