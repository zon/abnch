package store

import (
	"errors"
	"reflect"
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	s := newTestStore(t)

	for _, model := range []any{&Repo{}, &Task{}, &Test{}} {
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

// TestMoveTaskRenamesTask checks MoveTask renames a task, leaves one task, and
// makes the old stub miss.
func TestMoveTaskRenamesTask(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveTask(Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"}); err != nil {
		t.Fatal(err)
	}

	got, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo")
	if err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	want := Task{Org: "acme", Repo: "widgets", Stub: "fix-typo", Name: "Fix a bug", Description: "A broken widget"}
	if got != want {
		t.Errorf("MoveTask = %#v, want %#v", got, want)
	}
	if _, err := s.GetTask("acme", "widgets", "fix-bug"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTask(old) error = %v, want ErrNotFound", err)
	}
	tasks, err := s.ListTasks("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0] != want {
		t.Errorf("ListTasks = %#v, want one renamed task", tasks)
	}
}

// TestMoveTaskMissing checks MoveTask reports a task that does not exist.
func TestMoveTaskMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("MoveTask error = %v, want ErrNotFound", err)
	}
}

// TestMoveTaskLeavesOtherTasks checks MoveTask renames only its own task and
// leaves the repo's other tasks alone.
func TestMoveTaskLeavesOtherTasks(t *testing.T) {
	s := newTestStore(t)
	for _, task := range []Task{
		{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug"},
		{Org: "acme", Repo: "widgets", Stub: "other", Name: "Other"},
	} {
		if err := s.SaveTask(task); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	other, err := s.GetTask("acme", "widgets", "other")
	if err != nil {
		t.Fatalf("GetTask(other): %v", err)
	}
	if other.Name != "Other" {
		t.Errorf("other task = %#v, want it unchanged", other)
	}
	tasks, err := s.ListTasks("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("ListTasks returned %d tasks, want 2", len(tasks))
	}
}

// TestMoveTaskConflict checks MoveTask refuses to overwrite a task with the new
// stub and leaves both tasks unchanged.
func TestMoveTaskConflict(t *testing.T) {
	s := newTestStore(t)
	for _, task := range []Task{
		{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"},
		{Org: "acme", Repo: "widgets", Stub: "fix-typo", Name: "Fix a typo", Description: "A typed word"},
	} {
		if err := s.SaveTask(task); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo"); !errors.Is(err, ErrConflict) {
		t.Errorf("MoveTask error = %v, want ErrConflict", err)
	}
	tasks, err := s.ListTasks("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("ListTasks returned %d tasks, want the two unchanged tasks", len(tasks))
	}
	if tasks[0].Stub != "fix-bug" || tasks[1].Stub != "fix-typo" {
		t.Errorf("ListTasks stubs = %q, want [fix-bug fix-typo]", []string{tasks[0].Stub, tasks[1].Stub})
	}
}

// TestMoveTaskRejectsInvalidStub checks a move to a stub that is empty or not
// URL-safe is refused and leaves the task unchanged.
func TestMoveTaskRejectsInvalidStub(t *testing.T) {
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
			want := Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug", Description: "A broken widget"}
			if err := s.SaveTask(want); err != nil {
				t.Fatal(err)
			}

			if _, err := s.MoveTask("acme", "widgets", "fix-bug", tc.stub); !errors.Is(err, ErrInvalidStub) {
				t.Errorf("MoveTask error = %v, want ErrInvalidStub", err)
			}
			got, err := s.GetTask("acme", "widgets", "fix-bug")
			if err != nil {
				t.Fatalf("GetTask: %v", err)
			}
			if got != want {
				t.Errorf("task = %#v, want the unchanged task %#v", got, want)
			}
		})
	}
}

// TestSaveAndGetTest checks a test round trips through the store with its name,
// description, version, and schema.
func TestSaveAndGetTest(t *testing.T) {
	s := newTestStore(t)
	want := Test{
		Org:         "acme",
		Repo:        "widgets",
		Task:        "fix-bug",
		Stub:        "basic",
		Name:        "Basic",
		Description: "A basic test",
		Version:     "1",
		Schema:      []SchemaKey{{Stub: "correctness", Name: "Correctness"}, {Stub: "style", Name: "Style"}},
	}

	if err := s.SaveTest(want); err != nil {
		t.Fatalf("SaveTest: %v", err)
	}

	got, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatalf("GetTest: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetTest = %#v, want %#v", got, want)
	}
}

// TestSaveTestUpdatesExistingTest checks a second write to the same repo, task,
// and stub replaces the test instead of creating another one.
func TestSaveTestUpdatesExistingTest(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveTest(Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Version: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTest(Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Description: "new", Version: "2"}); err != nil {
		t.Fatal(err)
	}

	tests, err := s.ListTests("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 1 {
		t.Fatalf("ListTests returned %d tests, want 1", len(tests))
	}
	if tests[0].Version != "2" {
		t.Errorf("Version = %q, want the updated version", tests[0].Version)
	}
}

// TestSaveTestRejectsInvalidStub checks a test whose stub is empty or not
// URL-safe is not stored.
func TestSaveTestRejectsInvalidStub(t *testing.T) {
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

			test := Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: tc.stub, Name: "Basic", Version: "1"}
			if err := s.SaveTest(test); !errors.Is(err, ErrInvalidStub) {
				t.Errorf("SaveTest error = %v, want ErrInvalidStub", err)
			}
			tests, err := s.ListTests("acme", "widgets", "fix-bug")
			if err != nil {
				t.Fatal(err)
			}
			if len(tests) != 0 {
				t.Errorf("ListTests returned %d tests, want none", len(tests))
			}
		})
	}
}

// TestSaveTestRejectsMissingRequiredFields checks a test without a name or a
// version is refused and stored nowhere.
func TestSaveTestRejectsMissingRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		test Test
	}{
		{"empty name", Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Version: "1"}},
		{"empty version", Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if err := s.SaveTest(tc.test); !errors.Is(err, ErrInvalidTest) {
				t.Errorf("SaveTest error = %v, want ErrInvalidTest", err)
			}
			tests, err := s.ListTests("acme", "widgets", "fix-bug")
			if err != nil {
				t.Fatal(err)
			}
			if len(tests) != 0 {
				t.Errorf("ListTests returned %d tests, want none", len(tests))
			}
		})
	}
}

// TestSaveTestRejectsDuplicateSchemaStub checks a test whose score schema gives
// two keys the same stub is refused and stored nowhere.
func TestSaveTestRejectsDuplicateSchemaStub(t *testing.T) {
	s := newTestStore(t)

	test := Test{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Version: "1",
		Schema: []SchemaKey{{Stub: "style", Name: "Style"}, {Stub: "style", Name: "Other"}},
	}
	if err := s.SaveTest(test); !errors.Is(err, ErrInvalidSchema) {
		t.Errorf("SaveTest error = %v, want ErrInvalidSchema", err)
	}
	tests, err := s.ListTests("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 0 {
		t.Errorf("ListTests returned %d tests, want none", len(tests))
	}
}

// TestSaveTestAllowsEmptySchema checks a test with no schema keys is allowed and
// round trips with an empty schema.
func TestSaveTestAllowsEmptySchema(t *testing.T) {
	s := newTestStore(t)
	want := Test{
		Org:         "acme",
		Repo:        "widgets",
		Task:        "fix-bug",
		Stub:        "basic",
		Name:        "Basic",
		Description: "A basic test",
		Version:     "1",
	}

	if err := s.SaveTest(want); err != nil {
		t.Fatalf("SaveTest: %v", err)
	}

	got, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatalf("GetTest: %v", err)
	}
	if len(got.Schema) != 0 {
		t.Errorf("Schema = %#v, want an empty schema", got.Schema)
	}
	if got.Name != want.Name || got.Description != want.Description || got.Version != want.Version {
		t.Errorf("GetTest = %#v, want %#v", got, want)
	}
}

// TestTestValidate checks a test needs a non-empty name and version, may carry
// an empty schema, and rejects a schema whose key stubs are not unique.
func TestTestValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		test Test
		want error
	}{
		{"valid", Test{Name: "Basic", Version: "1"}, nil},
		{"valid with schema", Test{Name: "Basic", Version: "1", Schema: []SchemaKey{{Stub: "style", Name: "Style"}}}, nil},
		{"empty name", Test{Version: "1"}, ErrInvalidTest},
		{"empty version", Test{Name: "Basic"}, ErrInvalidTest},
		{"duplicate schema stub", Test{Name: "Basic", Version: "1", Schema: []SchemaKey{{Stub: "style", Name: "Style"}, {Stub: "style", Name: "Other"}}}, ErrInvalidSchema},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.test.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestSaveTestIdentityIncludesTask checks the same test stub in two tasks is two
// tests, not one, because a test's identity includes its task stub.
func TestSaveTestIdentityIncludesTask(t *testing.T) {
	s := newTestStore(t)
	for _, task := range []string{"fix-bug", "other"} {
		if err := s.SaveTest(Test{Org: "acme", Repo: "widgets", Task: task, Stub: "basic", Name: "Basic", Version: "1"}); err != nil {
			t.Fatal(err)
		}
	}

	for _, task := range []string{"fix-bug", "other"} {
		tests, err := s.ListTests("acme", "widgets", task)
		if err != nil {
			t.Fatal(err)
		}
		if len(tests) != 1 {
			t.Fatalf("ListTests(%q) returned %d tests, want 1", task, len(tests))
		}
		if tests[0].Task != task {
			t.Errorf("test task = %q, want %q", tests[0].Task, task)
		}
	}
}

func TestListTests(t *testing.T) {
	s := newTestStore(t)
	for _, test := range []Test{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "two", Name: "Two", Version: "1"},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "one", Name: "One", Version: "1"},
		{Org: "acme", Repo: "widgets", Task: "other", Stub: "other", Name: "Other", Version: "1"},
	} {
		if err := s.SaveTest(test); err != nil {
			t.Fatal(err)
		}
	}

	tests, err := s.ListTests("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 2 {
		t.Fatalf("ListTests returned %d tests, want 2", len(tests))
	}
	if tests[0].Stub != "one" || tests[1].Stub != "two" {
		t.Errorf("ListTests stubs = %q, want [one two]", []string{tests[0].Stub, tests[1].Stub})
	}
}

func TestGetTestMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetTest("acme", "widgets", "fix-bug", "basic"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTest error = %v, want ErrNotFound", err)
	}
}
