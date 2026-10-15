package store

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	s := newTestStore(t)

	for _, model := range []any{&Repo{}, &Task{}, &Test{}, &Datum{}, &Result{}} {
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

// TestMoveTestRenamesTest checks MoveTest renames a test, keeps its fields and
// schema, leaves one test, and makes the old stub miss.
func TestMoveTestRenamesTest(t *testing.T) {
	s := newTestStore(t)
	stored := Test{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic",
		Name: "Basic", Description: "A basic test", Version: "1",
		Schema: []SchemaKey{{Stub: "correctness", Name: "Correctness"}},
	}
	if err := s.SaveTest(stored); err != nil {
		t.Fatal(err)
	}

	got, err := s.MoveTest("acme", "widgets", "fix-bug", "basic", "smoke")
	if err != nil {
		t.Fatalf("MoveTest: %v", err)
	}
	want := stored
	want.Stub = "smoke"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MoveTest = %#v, want %#v", got, want)
	}
	if _, err := s.GetTest("acme", "widgets", "fix-bug", "basic"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTest(old) error = %v, want ErrNotFound", err)
	}
	tests, err := s.ListTests("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 1 || !reflect.DeepEqual(tests[0], want) {
		t.Errorf("ListTests = %#v, want one renamed test", tests)
	}
}

// TestMoveTestMissing checks MoveTest reports a test that does not exist.
func TestMoveTestMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.MoveTest("acme", "widgets", "fix-bug", "basic", "smoke"); !errors.Is(err, ErrNotFound) {
		t.Errorf("MoveTest error = %v, want ErrNotFound", err)
	}
}

// TestMoveTestConflict checks MoveTest refuses to overwrite a test with the new
// stub and leaves both tests unchanged.
func TestMoveTestConflict(t *testing.T) {
	s := newTestStore(t)
	for _, test := range []Test{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Version: "1"},
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "smoke", Name: "Smoke", Version: "1"},
	} {
		if err := s.SaveTest(test); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.MoveTest("acme", "widgets", "fix-bug", "basic", "smoke"); !errors.Is(err, ErrConflict) {
		t.Errorf("MoveTest error = %v, want ErrConflict", err)
	}
	tests, err := s.ListTests("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 2 {
		t.Fatalf("ListTests returned %d tests, want the two unchanged tests", len(tests))
	}
	if tests[0].Stub != "basic" || tests[1].Stub != "smoke" {
		t.Errorf("ListTests stubs = %q, want [basic smoke]", []string{tests[0].Stub, tests[1].Stub})
	}
}

// TestMoveTestLeavesOtherTasks checks MoveTest renames only its own task's test
// and leaves the same stub in another task alone.
func TestMoveTestLeavesOtherTasks(t *testing.T) {
	s := newTestStore(t)
	for _, test := range []Test{
		{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Version: "1"},
		{Org: "acme", Repo: "widgets", Task: "other", Stub: "smoke", Name: "Smoke", Version: "1"},
	} {
		if err := s.SaveTest(test); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.MoveTest("acme", "widgets", "fix-bug", "basic", "smoke"); err != nil {
		t.Fatalf("MoveTest: %v", err)
	}

	other, err := s.GetTest("acme", "widgets", "other", "smoke")
	if err != nil {
		t.Fatalf("GetTest(other): %v", err)
	}
	if other.Name != "Smoke" {
		t.Errorf("other test = %#v, want it unchanged", other)
	}
}

// TestMoveTestRejectsInvalidStub checks a move to a stub that is empty or not
// URL-safe is refused and leaves the test unchanged.
func TestMoveTestRejectsInvalidStub(t *testing.T) {
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
			want := Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Version: "1"}
			if err := s.SaveTest(want); err != nil {
				t.Fatal(err)
			}

			if _, err := s.MoveTest("acme", "widgets", "fix-bug", "basic", tc.stub); !errors.Is(err, ErrInvalidStub) {
				t.Errorf("MoveTest error = %v, want ErrInvalidStub", err)
			}
			got, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
			if err != nil {
				t.Fatalf("GetTest: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("test = %#v, want the unchanged test %#v", got, want)
			}
		})
	}
}

// TestSaveAndGetDatum checks a datum's JSON round trips through the store
// unchanged under its ref.
func TestSaveAndGetDatum(t *testing.T) {
	s := newTestStore(t)
	body := []byte(`{"prompt":  "hi"}`)

	ref, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", body)
	if err != nil {
		t.Fatalf("SaveDatum: %v", err)
	}
	if ref == "" {
		t.Fatal("SaveDatum returned an empty ref")
	}

	got, err := s.GetDatum("acme", "widgets", "fix-bug", "basic", ref)
	if err != nil {
		t.Fatalf("GetDatum: %v", err)
	}
	want := Datum{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Ref: ref, JSON: string(body)}
	if got != want {
		t.Errorf("GetDatum = %#v, want %#v", got, want)
	}
}

// TestSaveAndGetDatumFileReference checks a datum whose JSON holds a file
// reference round trips unchanged. The store keeps the URL and checksum as the
// client gave them and fetches nothing.
func TestSaveAndGetDatumFileReference(t *testing.T) {
	s := newTestStore(t)
	body := []byte(`{"input": {"url": "https://example.com/input.txt", "checksum": "sha256:abc123"}}`)

	ref, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", body)
	if err != nil {
		t.Fatalf("SaveDatum: %v", err)
	}

	got, err := s.GetDatum("acme", "widgets", "fix-bug", "basic", ref)
	if err != nil {
		t.Fatalf("GetDatum: %v", err)
	}
	if got.JSON != string(body) {
		t.Fatalf("GetDatum JSON = %q, want the unchanged JSON %q", got.JSON, body)
	}

	var parsed struct {
		Input struct {
			URL      string `json:"url"`
			Checksum string `json:"checksum"`
		} `json:"input"`
	}
	if err := json.Unmarshal([]byte(got.JSON), &parsed); err != nil {
		t.Fatalf("unmarshal datum JSON: %v", err)
	}
	if parsed.Input.URL != "https://example.com/input.txt" || parsed.Input.Checksum != "sha256:abc123" {
		t.Errorf("file reference = %#v, want its URL and checksum unchanged", parsed.Input)
	}
}

// TestSaveDatumSameJSONSameRef checks saving the same JSON in a test again
// returns the same ref and leaves one datum.
func TestSaveDatumSameJSONSameRef(t *testing.T) {
	s := newTestStore(t)
	body := []byte(`{"prompt":"hi"}`)

	first, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", body)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", body)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("second ref = %q, want the first ref %q", second, first)
	}

	data, err := s.ListData("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 {
		t.Errorf("ListData returned %d data, want 1", len(data))
	}
}

// TestSaveDatumDifferentJSONDifferentRef checks different JSON in a test gets a
// different ref and adds a second datum.
func TestSaveDatumDifferentJSONDifferentRef(t *testing.T) {
	s := newTestStore(t)

	first, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", []byte(`{"prompt":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", []byte(`{"prompt":"bye"}`))
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Errorf("second ref = %q, want a different ref", second)
	}

	data, err := s.ListData("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 {
		t.Errorf("ListData returned %d data, want 2", len(data))
	}
}

// TestSaveDatumIdentityIncludesTest checks the same JSON in two tests is two
// data, not one, because a datum's identity includes its test.
func TestSaveDatumIdentityIncludesTest(t *testing.T) {
	s := newTestStore(t)
	body := []byte(`{"prompt":"hi"}`)
	for _, test := range []string{"basic", "other"} {
		if _, err := s.SaveDatum("acme", "widgets", "fix-bug", test, body); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []string{"basic", "other"} {
		data, err := s.ListData("acme", "widgets", "fix-bug", test)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 1 {
			t.Fatalf("ListData(%q) returned %d data, want 1", test, len(data))
		}
		if data[0].Test != test {
			t.Errorf("datum test = %q, want %q", data[0].Test, test)
		}
	}
}

// TestSaveDatumIdentityIncludesRepoAndTask checks the same JSON under different
// repos and tasks is a different datum under the same ref, because a datum's
// identity includes its repo and task stub, and that SaveDatum returns the ref
// derived from the JSON.
func TestSaveDatumIdentityIncludesRepoAndTask(t *testing.T) {
	s := newTestStore(t)
	body := []byte(`{"prompt":"hi"}`)
	coords := []struct{ repo, task string }{
		{"widgets", "fix-bug"},
		{"gadgets", "fix-bug"},
		{"widgets", "ship-it"},
	}

	for _, c := range coords {
		ref, err := s.SaveDatum("acme", c.repo, c.task, "basic", body)
		if err != nil {
			t.Fatal(err)
		}
		if ref != DatumRef(body) {
			t.Errorf("SaveDatum(%s/%s) ref = %q, want %q", c.repo, c.task, ref, DatumRef(body))
		}
	}

	for _, c := range coords {
		data, err := s.ListData("acme", c.repo, c.task, "basic")
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 1 {
			t.Fatalf("ListData(%s/%s) returned %d data, want 1", c.repo, c.task, len(data))
		}
	}
}

// TestListData checks ListData returns only a test's data, ordered by ref.
func TestListData(t *testing.T) {
	s := newTestStore(t)
	want := map[string]string{}
	for _, body := range []string{`{"n":1}`, `{"n":2}`} {
		ref, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		want[ref] = body
	}
	if _, err := s.SaveDatum("acme", "widgets", "fix-bug", "other", []byte(`{"n":3}`)); err != nil {
		t.Fatal(err)
	}

	data, err := s.ListData("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(want) {
		t.Fatalf("ListData returned %d data, want %d", len(data), len(want))
	}
	refs := make([]string, len(data))
	for i, d := range data {
		if want[d.Ref] != d.JSON {
			t.Errorf("datum %q JSON = %q, want %q", d.Ref, d.JSON, want[d.Ref])
		}
		refs[i] = d.Ref
	}
	if !sort.StringsAreSorted(refs) {
		t.Errorf("ListData refs = %q, want them ordered", refs)
	}
}

func TestGetDatumMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetDatum("acme", "widgets", "fix-bug", "basic", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetDatum error = %v, want ErrNotFound", err)
	}
}

// TestDatumRef checks a ref is short and URL-safe, and that it is the same for
// the same JSON and different for different JSON.
func TestDatumRef(t *testing.T) {
	first := DatumRef([]byte(`{"prompt":"hi"}`))
	if first != DatumRef([]byte(`{"prompt":"hi"}`)) {
		t.Errorf("DatumRef returned different refs for the same JSON")
	}
	if first == DatumRef([]byte(`{"prompt":"bye"}`)) {
		t.Errorf("DatumRef returned the same ref for different JSON")
	}
	if first == "" || !ValidStub(first) {
		t.Errorf("DatumRef(%q) = %q, want a non-empty URL-safe ref", `{"prompt":"hi"}`, first)
	}
}

// TestSaveAndGetResult checks a result round trips through the store with its
// JSON, logs, token counts, cost, duration, and local branch under its ref.
func TestSaveAndGetResult(t *testing.T) {
	s := newTestStore(t)
	want := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: "56ac1b0e4f0d",
		JSON: `{"answer":"hi"}`, Logs: "resolving the prompt",
		InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5,
		Local: "main",
	}

	ref, err := s.SaveResult(want)
	if err != nil {
		t.Fatalf("SaveResult: %v", err)
	}
	if ref == "" {
		t.Fatal("SaveResult returned an empty ref")
	}

	got, err := s.GetResult("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	want.Ref = ref
	if got != want {
		t.Errorf("GetResult = %#v, want %#v", got, want)
	}
}

// TestSaveResultSameIdentitySameRef checks saving the same JSON for the same
// datum again returns the same ref and leaves one result.
func TestSaveResultSameIdentitySameRef(t *testing.T) {
	s := newTestStore(t)
	r := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
	}

	first, err := s.SaveResult(r)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveResult(r)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("second ref = %q, want the first ref %q", second, first)
	}

	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("ListResults returned %d results, want 1", len(results))
	}
}

// TestSaveResultDifferentJSONDifferentRef checks different JSON for the same
// datum gets a different ref and adds a second result.
func TestSaveResultDifferentJSONDifferentRef(t *testing.T) {
	s := newTestStore(t)
	coords := Result{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: "56ac1b0e4f0d"}

	first := coords
	first.JSON = `{"answer":"hi"}`
	second := coords
	second.JSON = `{"answer":"bye"}`

	firstRef, err := s.SaveResult(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRef, err := s.SaveResult(second)
	if err != nil {
		t.Fatal(err)
	}
	if secondRef == firstRef {
		t.Errorf("second ref = %q, want a different ref", secondRef)
	}

	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("ListResults returned %d results, want 2", len(results))
	}
}

// TestSaveResultIdentityIncludesData checks the same JSON for two data in one
// test is two results under different refs, because a result's identity
// includes its datum.
func TestSaveResultIdentityIncludesData(t *testing.T) {
	s := newTestStore(t)
	coords := Result{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", JSON: `{"answer":"hi"}`}

	first := coords
	first.Data = "56ac1b0e4f0d"
	second := coords
	second.Data = "ffffffffffff"

	firstRef, err := s.SaveResult(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRef, err := s.SaveResult(second)
	if err != nil {
		t.Fatal(err)
	}
	if secondRef == firstRef {
		t.Errorf("second ref = %q, want a different ref for a different datum", secondRef)
	}

	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("ListResults returned %d results, want 2", len(results))
	}
}

// TestSaveResultIdentityIncludesDatum checks SaveResult returns the ref
// ResultRef derives from the JSON within the result's datum, that the same JSON
// for different data is a distinct result under a different ref, and that each
// datum holds exactly one result for the same JSON.
func TestSaveResultIdentityIncludesDatum(t *testing.T) {
	s := newTestStore(t)
	body := `{"answer":"hi"}`
	coords := []struct{ repo, task, test, data string }{
		{"widgets", "fix-bug", "basic", "56ac1b0e4f0d"},
		{"widgets", "fix-bug", "basic", "ffffffffffff"},
		{"widgets", "fix-bug", "other", "56ac1b0e4f0d"},
		{"widgets", "ship-it", "basic", "56ac1b0e4f0d"},
		{"gadgets", "fix-bug", "basic", "56ac1b0e4f0d"},
	}

	refs := map[string]struct{}{}
	for _, c := range coords {
		r := Result{Org: "acme", Repo: c.repo, Task: c.task, Test: c.test, Data: c.data, JSON: body}
		ref, err := s.SaveResult(r)
		if err != nil {
			t.Fatal(err)
		}
		if want := ResultRef(r); ref != want {
			t.Errorf("SaveResult(%s/%s/%s/%s) ref = %q, want %q", c.repo, c.task, c.test, c.data, ref, want)
		}
		if _, ok := refs[ref]; ok {
			t.Errorf("SaveResult(%s/%s/%s/%s) ref %q is not distinct from an earlier result", c.repo, c.task, c.test, c.data, ref)
		}
		refs[ref] = struct{}{}

		again, err := s.SaveResult(r)
		if err != nil {
			t.Fatal(err)
		}
		if again != ref {
			t.Errorf("SaveResult(%s/%s/%s/%s) again = %q, want the ref %q", c.repo, c.task, c.test, c.data, again, ref)
		}
	}

	for _, repo := range []string{"widgets", "gadgets"} {
		want := 0
		for _, c := range coords {
			if c.repo == repo {
				want++
			}
		}
		results, err := s.ListResults("acme", repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != want {
			t.Errorf("ListResults(%s) returned %d results, want %d", repo, len(results), want)
		}
	}
}

// TestListResults checks ListResults returns only a repo's results, ordered by
// ref, without another repo's results.
func TestListResults(t *testing.T) {
	s := newTestStore(t)
	want := map[string]Result{}
	for _, body := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
		r := Result{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: "56ac1b0e4f0d", JSON: body}
		ref, err := s.SaveResult(r)
		if err != nil {
			t.Fatal(err)
		}
		r.Ref = ref
		want[ref] = r
	}
	other := Result{Org: "acme", Repo: "gadgets", Task: "fix-bug", Test: "basic", Data: "56ac1b0e4f0d", JSON: `{"n":1}`}
	if _, err := s.SaveResult(other); err != nil {
		t.Fatal(err)
	}

	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(want) {
		t.Fatalf("ListResults returned %d results, want %d", len(results), len(want))
	}
	refs := make([]string, len(results))
	for i, r := range results {
		if got := want[r.Ref]; got != r {
			t.Errorf("result %q = %#v, want %#v", r.Ref, r, got)
		}
		refs[i] = r.Ref
	}
	if !sort.StringsAreSorted(refs) {
		t.Errorf("ListResults refs = %q, want them ordered", refs)
	}
}

func TestGetResultMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetResult("acme", "widgets", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetResult error = %v, want ErrNotFound", err)
	}
}

// TestResultRef checks a ref is short and URL-safe, is stable for the same
// result JSON in the same datum, and changes when the JSON or any part of the
// datum's identity changes.
func TestResultRef(t *testing.T) {
	base := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
	}
	ref := ResultRef(base)
	if ref != ResultRef(base) {
		t.Errorf("ResultRef returned different refs for the same result")
	}
	if ref == "" || !ValidStub(ref) {
		t.Errorf("ResultRef = %q, want a non-empty URL-safe ref", ref)
	}

	for _, tc := range []struct {
		name   string
		change func(Result) Result
	}{
		{"different JSON", func(r Result) Result { r.JSON = `{"answer":"bye"}`; return r }},
		{"different datum", func(r Result) Result { r.Data = "ffffffffffff"; return r }},
		{"different test", func(r Result) Result { r.Test = "other"; return r }},
		{"different task", func(r Result) Result { r.Task = "other"; return r }},
		{"different repo", func(r Result) Result { r.Repo = "gadgets"; return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResultRef(tc.change(base)); got == ref {
				t.Errorf("ResultRef = %q, want a different ref", got)
			}
		})
	}
}
