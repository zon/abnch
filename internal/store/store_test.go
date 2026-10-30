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

	for _, model := range []any{&Repo{}, &Task{}, &Test{}, &Model{}, &Agent{}, &Grader{}, &Datum{}, &Result{}, &Score{}} {
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

// TestTestAllowsScore checks a test allows a score whose keys are all in its
// schema, including a partial score, and refuses a key outside it or any key
// when its schema is empty.
func TestTestAllowsScore(t *testing.T) {
	test := Test{Schema: []SchemaKey{{Stub: "correctness", Name: "Correctness"}, {Stub: "style", Name: "Style"}}}
	for _, tc := range []struct {
		name       string
		test       Test
		dictionary map[string]float64
		want       bool
	}{
		{"every key", test, map[string]float64{"correctness": 1, "style": 0.5}, true},
		{"partial score", test, map[string]float64{"correctness": 1}, true},
		{"empty dictionary", test, map[string]float64{}, true},
		{"unknown key", test, map[string]float64{"speed": 1}, false},
		{"unknown key beside a known key", test, map[string]float64{"correctness": 1, "speed": 1}, false},
		{"empty schema", Test{}, map[string]float64{"correctness": 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.test.AllowsScore(tc.dictionary); got != tc.want {
				t.Errorf("AllowsScore = %v, want %v", got, tc.want)
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

// TestSaveResultDefaultsMetricsToZero checks a result that omits its run metrics
// records zero for each of them.
func TestSaveResultDefaultsMetricsToZero(t *testing.T) {
	s := newTestStore(t)
	r := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
	}

	ref, err := s.SaveResult(r)
	if err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	got, err := s.GetResult("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if got.InputTokens != 0 || got.OutputTokens != 0 || got.TokenCost != 0 || got.Duration != 0 {
		t.Errorf("metrics = %d input, %d output, %v cost, %v duration, want zero", got.InputTokens, got.OutputTokens, got.TokenCost, got.Duration)
	}
	if got.Logs != "" || got.Local != "" {
		t.Errorf("logs = %q, local = %q, want them empty", got.Logs, got.Local)
	}
}

// TestSaveResultRejectsNegativeMetrics checks a result with a negative token
// count, cost, or duration is refused and stored nowhere.
func TestSaveResultRejectsNegativeMetrics(t *testing.T) {
	valid := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
	}
	for _, tc := range []struct {
		name   string
		change func(Result) Result
	}{
		{"negative input tokens", func(r Result) Result { r.InputTokens = -1; return r }},
		{"negative output tokens", func(r Result) Result { r.OutputTokens = -1; return r }},
		{"negative token cost", func(r Result) Result { r.TokenCost = -0.01; return r }},
		{"negative duration", func(r Result) Result { r.Duration = -0.5; return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if _, err := s.SaveResult(tc.change(valid)); !errors.Is(err, ErrInvalidResult) {
				t.Errorf("SaveResult error = %v, want ErrInvalidResult", err)
			}
			results, err := s.ListResults("acme", "widgets")
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 0 {
				t.Errorf("ListResults returned %d results, want none", len(results))
			}
		})
	}
}

// TestResultValidate checks a result may carry zero or positive run metrics and
// is refused when one is negative.
func TestResultValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result Result
		want   error
	}{
		{"zero metrics", Result{}, nil},
		{"positive metrics", Result{InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5}, nil},
		{"negative input tokens", Result{InputTokens: -1}, ErrInvalidResult},
		{"negative output tokens", Result{OutputTokens: -1}, ErrInvalidResult},
		{"negative token cost", Result{TokenCost: -0.01}, ErrInvalidResult},
		{"negative duration", Result{Duration: -0.5}, ErrInvalidResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.result.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate error = %v, want %v", err, tc.want)
			}
		})
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

// TestSaveResultIdentityIncludesModelAndAgent checks the same JSON for the same
// datum from a different model or agent is a distinct result under a different
// ref, while saving the same model and agent values again reuses one result.
func TestSaveResultIdentityIncludesModelAndAgent(t *testing.T) {
	s := newTestStore(t)
	base := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	}

	ref, err := s.SaveResult(base)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.SaveResult(base); err != nil {
		t.Fatal(err)
	} else if again != ref {
		t.Errorf("SaveResult again = %q, want the ref %q", again, ref)
	}

	for _, tc := range []struct {
		name   string
		change func(Result) Result
	}{
		{"different model name", func(r Result) Result { r.Model.Name = "claude-opus"; return r }},
		{"different model provider", func(r Result) Result { r.Model.Provider = "openai"; return r }},
		{"different model variant", func(r Result) Result { r.Model.Variant = "low"; return r }},
		{"different agent stub", func(r Result) Result { r.Agent.Stub = "other"; return r }},
		{"different agent commit", func(r Result) Result { r.Agent.Commit = "def456"; return r }},
		{"different agent branch", func(r Result) Result { r.Agent.Branch = "dev"; return r }},
		{"different agent version", func(r Result) Result { r.Agent.Version = "1.2.3"; return r }},
		{"uncommitted agent", func(r Result) Result { r.Agent.Uncommitted = true; return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.SaveResult(tc.change(base))
			if err != nil {
				t.Fatal(err)
			}
			if got == ref {
				t.Errorf("SaveResult = %q, want a different ref for %s", got, tc.name)
			}
		})
	}

	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if want := 1 + 8; len(results) != want {
		t.Errorf("ListResults returned %d results, want %d", len(results), want)
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
// result JSON in the same datum from the same model and agent, and changes when
// the JSON or any part of the datum's, model's, or agent's identity changes.
func TestResultRef(t *testing.T) {
	base := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
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
		{"different model name", func(r Result) Result { r.Model.Name = "claude-opus"; return r }},
		{"different model provider", func(r Result) Result { r.Model.Provider = "openai"; return r }},
		{"different model variant", func(r Result) Result { r.Model.Variant = "low"; return r }},
		{"different agent stub", func(r Result) Result { r.Agent.Stub = "other"; return r }},
		{"different agent commit", func(r Result) Result { r.Agent.Commit = "def456"; return r }},
		{"different agent branch", func(r Result) Result { r.Agent.Branch = "dev"; return r }},
		{"different agent version", func(r Result) Result { r.Agent.Version = "1.2.3"; return r }},
		{"uncommitted agent", func(r Result) Result { r.Agent.Uncommitted = true; return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResultRef(tc.change(base)); got == ref {
				t.Errorf("ResultRef = %q, want a different ref", got)
			}
		})
	}
}

// TestSaveAndGetScore checks a score round trips through the store with its
// dictionary, logs, token counts, cost, and duration under its ref.
func TestSaveAndGetScore(t *testing.T) {
	s := newTestStore(t)
	want := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result:      "56ac1b0e4f0d",
		Dictionary:  map[string]float64{"correctness": 1, "style": 0.5},
		Logs:        "grading the answer",
		InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5,
	}

	ref, err := s.SaveScore(want)
	if err != nil {
		t.Fatalf("SaveScore: %v", err)
	}
	if ref == "" {
		t.Fatal("SaveScore returned an empty ref")
	}

	got, err := s.GetScore("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}
	want.Ref = ref
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetScore = %#v, want %#v", got, want)
	}
}

// TestSaveScoreDefaultsMetricsToZero checks a score that omits its run metrics
// records zero for each of them and stores no logs.
func TestSaveScoreDefaultsMetricsToZero(t *testing.T) {
	s := newTestStore(t)
	sc := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
	}

	ref, err := s.SaveScore(sc)
	if err != nil {
		t.Fatalf("SaveScore: %v", err)
	}

	got, err := s.GetScore("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}
	if got.InputTokens != 0 || got.OutputTokens != 0 || got.TokenCost != 0 || got.Duration != 0 {
		t.Errorf("metrics = %d input, %d output, %v cost, %v duration, want zero", got.InputTokens, got.OutputTokens, got.TokenCost, got.Duration)
	}
	if got.Logs != "" {
		t.Errorf("logs = %q, want them empty", got.Logs)
	}
}

// TestSaveScoreRejectsNegativeMetrics checks a score with a negative token
// count, cost, or duration is refused and stored nowhere.
func TestSaveScoreRejectsNegativeMetrics(t *testing.T) {
	valid := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
	}
	for _, tc := range []struct {
		name   string
		change func(Score) Score
	}{
		{"negative input tokens", func(s Score) Score { s.InputTokens = -1; return s }},
		{"negative output tokens", func(s Score) Score { s.OutputTokens = -1; return s }},
		{"negative token cost", func(s Score) Score { s.TokenCost = -0.01; return s }},
		{"negative duration", func(s Score) Score { s.Duration = -0.5; return s }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if _, err := s.SaveScore(tc.change(valid)); !errors.Is(err, ErrInvalidScore) {
				t.Errorf("SaveScore error = %v, want ErrInvalidScore", err)
			}
			scores, err := s.ListScores("acme", "widgets")
			if err != nil {
				t.Fatal(err)
			}
			if len(scores) != 0 {
				t.Errorf("ListScores returned %d scores, want none", len(scores))
			}
		})
	}
}

// TestScoreValidate checks a score may carry zero or positive run metrics and is
// refused when one is negative.
func TestScoreValidate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		score Score
		want  error
	}{
		{"zero metrics", Score{}, nil},
		{"positive metrics", Score{InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5}, nil},
		{"negative input tokens", Score{InputTokens: -1}, ErrInvalidScore},
		{"negative output tokens", Score{OutputTokens: -1}, ErrInvalidScore},
		{"negative token cost", Score{TokenCost: -0.01}, ErrInvalidScore},
		{"negative duration", Score{Duration: -0.5}, ErrInvalidScore},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.score.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestSaveScoreSameDictionarySameRef checks saving the same dictionary for the
// same result again returns the same ref and leaves one score.
func TestSaveScoreSameDictionarySameRef(t *testing.T) {
	s := newTestStore(t)
	sc := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
	}

	first, err := s.SaveScore(sc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveScore(sc)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("second ref = %q, want the first ref %q", second, first)
	}

	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 1 {
		t.Errorf("ListScores returned %d scores, want 1", len(scores))
	}
}

// TestSaveScoreIdentityIgnoresLogsAndMetrics checks the ref comes from the
// dictionary within its result alone, so the same dictionary for the same
// result stays one score under one ref whatever logs or run metrics accompany
// it.
func TestSaveScoreIdentityIgnoresLogsAndMetrics(t *testing.T) {
	s := newTestStore(t)
	sc := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result:      "56ac1b0e4f0d",
		Dictionary:  map[string]float64{"correctness": 1},
		Logs:        "first pass",
		InputTokens: 12, OutputTokens: 3, TokenCost: 0.01, Duration: 1.5,
	}

	ref, err := s.SaveScore(sc)
	if err != nil {
		t.Fatal(err)
	}

	changed := sc
	changed.Logs = "second pass"
	changed.InputTokens = 99
	changed.OutputTokens = 88
	changed.TokenCost = 0.5
	changed.Duration = 9.5
	if got := ScoreRef(changed); got != ref {
		t.Errorf("ScoreRef = %q, want the ref %q whatever the logs and metrics", got, ref)
	}

	again, err := s.SaveScore(changed)
	if err != nil {
		t.Fatal(err)
	}
	if again != ref {
		t.Errorf("SaveScore = %q, want the ref %q for the same dictionary and result", again, ref)
	}

	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 1 {
		t.Errorf("ListScores returned %d scores, want 1", len(scores))
	}
}

// TestSaveScoreDifferentDictionaryDifferentRef checks a different dictionary for
// the same result gets a different ref and adds a second score.
func TestSaveScoreDifferentDictionaryDifferentRef(t *testing.T) {
	s := newTestStore(t)
	base := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d",
	}

	first := base
	first.Dictionary = map[string]float64{"correctness": 1}
	second := base
	second.Dictionary = map[string]float64{"correctness": 0.5}

	firstRef, err := s.SaveScore(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRef, err := s.SaveScore(second)
	if err != nil {
		t.Fatal(err)
	}
	if secondRef == firstRef {
		t.Errorf("second ref = %q, want a different ref", secondRef)
	}

	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 2 {
		t.Errorf("ListScores returned %d scores, want 2", len(scores))
	}
}

// TestSaveScoreIdentityIncludesResult checks SaveScore returns the ref ScoreRef
// derives from the dictionary within its result, that the same dictionary for
// different results stays a distinct score under a different ref, and that each
// result holds exactly one score for the same dictionary.
func TestSaveScoreIdentityIncludesResult(t *testing.T) {
	s := newTestStore(t)
	dict := map[string]float64{"correctness": 1}
	coords := []struct{ repo, task, test, result string }{
		{"widgets", "fix-bug", "basic", "56ac1b0e4f0d"},
		{"widgets", "fix-bug", "basic", "ffffffffffff"},
		{"widgets", "fix-bug", "other", "56ac1b0e4f0d"},
		{"widgets", "ship-it", "basic", "56ac1b0e4f0d"},
		{"gadgets", "fix-bug", "basic", "56ac1b0e4f0d"},
	}

	refs := map[string]struct{}{}
	for _, c := range coords {
		sc := Score{Org: "acme", Repo: c.repo, Task: c.task, Test: c.test, Result: c.result, Dictionary: dict}
		ref, err := s.SaveScore(sc)
		if err != nil {
			t.Fatal(err)
		}
		if want := ScoreRef(sc); ref != want {
			t.Errorf("SaveScore(%s/%s/%s/%s) ref = %q, want %q", c.repo, c.task, c.test, c.result, ref, want)
		}
		if _, ok := refs[ref]; ok {
			t.Errorf("SaveScore(%s/%s/%s/%s) ref %q is not distinct from an earlier score", c.repo, c.task, c.test, c.result, ref)
		}
		refs[ref] = struct{}{}

		again, err := s.SaveScore(sc)
		if err != nil {
			t.Fatal(err)
		}
		if again != ref {
			t.Errorf("SaveScore(%s/%s/%s/%s) again = %q, want the ref %q", c.repo, c.task, c.test, c.result, again, ref)
		}
	}

	for _, repo := range []string{"widgets", "gadgets"} {
		want := 0
		for _, c := range coords {
			if c.repo == repo {
				want++
			}
		}
		scores, err := s.ListScores("acme", repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(scores) != want {
			t.Errorf("ListScores(%s) returned %d scores, want %d", repo, len(scores), want)
		}
	}
}

// TestListScores checks ListScores returns only a repo's scores, ordered by ref,
// without another repo's scores.
func TestListScores(t *testing.T) {
	s := newTestStore(t)
	want := map[string]Score{}
	for _, d := range []float64{1, 2, 3} {
		sc := Score{
			Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
			Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"n": d},
		}
		ref, err := s.SaveScore(sc)
		if err != nil {
			t.Fatal(err)
		}
		sc.Ref = ref
		want[ref] = sc
	}
	other := Score{
		Org: "acme", Repo: "gadgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"n": 1},
	}
	if _, err := s.SaveScore(other); err != nil {
		t.Fatal(err)
	}

	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != len(want) {
		t.Fatalf("ListScores returned %d scores, want %d", len(scores), len(want))
	}
	refs := make([]string, len(scores))
	for i, sc := range scores {
		if got := want[sc.Ref]; !reflect.DeepEqual(sc, got) {
			t.Errorf("score %q = %#v, want %#v", sc.Ref, sc, got)
		}
		refs[i] = sc.Ref
	}
	if !sort.StringsAreSorted(refs) {
		t.Errorf("ListScores refs = %q, want them ordered", refs)
	}
}

func TestGetScoreMissing(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetScore("acme", "widgets", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetScore error = %v, want ErrNotFound", err)
	}
}

// TestSaveAndGetModel checks a model round trips through the store with its
// name, provider, and variant.
func TestSaveAndGetModel(t *testing.T) {
	s := newTestStore(t)
	want := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}

	if err := s.SaveModel(want); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}

	got, err := s.GetModel("claude-sonnet", "anthropic", "high")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if got != want {
		t.Errorf("GetModel = %#v, want %#v", got, want)
	}
}

// TestSaveModelReusesSameIdentity checks saving the same name, provider, and
// variant again leaves one model.
func TestSaveModelReusesSameIdentity(t *testing.T) {
	s := newTestStore(t)
	model := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}

	for i := 0; i < 2; i++ {
		if err := s.SaveModel(model); err != nil {
			t.Fatal(err)
		}
	}

	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Errorf("ListModels returned %d models, want 1", len(models))
	}
}

// TestSaveModelSeparatesDifferentValues checks a model that differs in its
// name, provider, or variant is a different record.
func TestSaveModelSeparatesDifferentValues(t *testing.T) {
	s := newTestStore(t)
	base := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	for _, model := range []Model{
		base,
		{Name: "claude-opus", Provider: base.Provider, Variant: base.Variant},
		{Name: base.Name, Provider: "openai", Variant: base.Variant},
		{Name: base.Name, Provider: base.Provider, Variant: "low"},
	} {
		if err := s.SaveModel(model); err != nil {
			t.Fatal(err)
		}
	}

	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 4 {
		t.Errorf("ListModels returned %d models, want 4", len(models))
	}
}

// TestSaveModelAllowsEmptyVariant checks a model needs only its name and
// provider, so an empty variant is a model of its own.
func TestSaveModelAllowsEmptyVariant(t *testing.T) {
	s := newTestStore(t)
	want := Model{Name: "claude-sonnet", Provider: "anthropic"}

	if err := s.SaveModel(want); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}

	got, err := s.GetModel("claude-sonnet", "anthropic", "")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if got != want {
		t.Errorf("GetModel = %#v, want %#v", got, want)
	}
}

// TestSaveModelRejectsMissingNameOrProvider checks a model without a name or
// provider is refused and stored nowhere.
func TestSaveModelRejectsMissingNameOrProvider(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model Model
	}{
		{"missing name", Model{Provider: "anthropic", Variant: "high"}},
		{"missing provider", Model{Name: "claude-sonnet", Variant: "high"}},
		{"only variant", Model{Variant: "high"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if err := s.SaveModel(tc.model); !errors.Is(err, ErrInvalidModel) {
				t.Errorf("SaveModel error = %v, want ErrInvalidModel", err)
			}
			models, err := s.ListModels()
			if err != nil {
				t.Fatal(err)
			}
			if len(models) != 0 {
				t.Errorf("ListModels returned %d models, want none", len(models))
			}
		})
	}
}

// TestSaveAndGetAgent checks an agent round trips through the store with its
// name, stub, commit, branch, version, uncommitted flag, and task.
func TestSaveAndGetAgent(t *testing.T) {
	s := newTestStore(t)
	want := Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
		Version: "1.2.3", Uncommitted: true,
	}

	if err := s.SaveAgent(want); err != nil {
		t.Fatalf("SaveAgent: %v", err)
	}

	got, err := s.GetAgent(want)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got != want {
		t.Errorf("GetAgent = %#v, want %#v", got, want)
	}
}

// TestSaveAgentDefaultsUncommittedAndAllowsEmptyVersion checks an agent needs no
// version, and an omitted uncommitted flag records false.
func TestSaveAgentDefaultsUncommittedAndAllowsEmptyVersion(t *testing.T) {
	s := newTestStore(t)
	want := Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}

	if err := s.SaveAgent(want); err != nil {
		t.Fatalf("SaveAgent: %v", err)
	}

	got, err := s.GetAgent(want)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Uncommitted || got.Version != "" {
		t.Errorf("GetAgent = %#v, want a false uncommitted flag and an empty version", got)
	}
}

// TestSaveAgentReusesSameIdentityIgnoringName checks saving the same identity
// again leaves one agent and keeps the stored name, because a name is not part
// of an agent's identity.
func TestSaveAgentReusesSameIdentityIgnoringName(t *testing.T) {
	s := newTestStore(t)
	agent := Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}

	if err := s.SaveAgent(agent); err != nil {
		t.Fatal(err)
	}
	renamed := agent
	renamed.Name = "renamed-agent"
	if err := s.SaveAgent(renamed); err != nil {
		t.Fatal(err)
	}

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Fatalf("ListAgents returned %d agents, want 1", len(agents))
	}
	if agents[0].Name != agent.Name {
		t.Errorf("agent name = %q, want the stored name %q", agents[0].Name, agent.Name)
	}
}

// TestSaveAgentSeparatesDifferentValues checks an agent that differs in its
// repo, task, stub, commit, branch, version, or uncommitted flag is a different
// record.
func TestSaveAgentSeparatesDifferentValues(t *testing.T) {
	s := newTestStore(t)
	base := Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}
	for _, agent := range []Agent{
		base,
		{Org: "other", Repo: base.Repo, Task: base.Task, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: "gadgets", Task: base.Task, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: "ship-it", Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Name: base.Name, Stub: "other", Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Name: base.Name, Stub: base.Stub, Commit: "def456", Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: "dev"},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch, Version: "1.2.3"},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch, Uncommitted: true},
	} {
		if err := s.SaveAgent(agent); err != nil {
			t.Fatal(err)
		}
	}

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 9 {
		t.Errorf("ListAgents returned %d agents, want 9", len(agents))
	}
}

// TestSaveAgentRejectsIncomplete checks an agent without a name, stub, commit,
// or branch is refused and stored nowhere.
func TestSaveAgentRejectsIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent Agent
	}{
		{"missing name", Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "claude", Commit: "abc123", Branch: "main"}},
		{"missing stub", Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Commit: "abc123", Branch: "main"}},
		{"missing commit", Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Branch: "main"}},
		{"missing branch", Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if err := s.SaveAgent(tc.agent); !errors.Is(err, ErrInvalidAgent) {
				t.Errorf("SaveAgent error = %v, want ErrInvalidAgent", err)
			}
			agents, err := s.ListAgents()
			if err != nil {
				t.Fatal(err)
			}
			if len(agents) != 0 {
				t.Errorf("ListAgents returned %d agents, want none", len(agents))
			}
		})
	}
}

// TestSaveAndGetGrader checks a grader round trips through the store with its
// name, stub, commit, branch, version, uncommitted flag, and test.
func TestSaveAndGetGrader(t *testing.T) {
	s := newTestStore(t)
	want := Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
		Version: "2.0.0", Uncommitted: true,
	}

	if err := s.SaveGrader(want); err != nil {
		t.Fatalf("SaveGrader: %v", err)
	}

	got, err := s.GetGrader(want)
	if err != nil {
		t.Fatalf("GetGrader: %v", err)
	}
	if got != want {
		t.Errorf("GetGrader = %#v, want %#v", got, want)
	}
}

// TestSaveGraderDefaultsUncommittedAndAllowsEmptyVersion checks a grader needs
// no version, and an omitted uncommitted flag records false.
func TestSaveGraderDefaultsUncommittedAndAllowsEmptyVersion(t *testing.T) {
	s := newTestStore(t)
	want := Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}

	if err := s.SaveGrader(want); err != nil {
		t.Fatalf("SaveGrader: %v", err)
	}

	got, err := s.GetGrader(want)
	if err != nil {
		t.Fatalf("GetGrader: %v", err)
	}
	if got.Uncommitted || got.Version != "" {
		t.Errorf("GetGrader = %#v, want a false uncommitted flag and an empty version", got)
	}
}

// TestSaveGraderReusesSameIdentityIgnoringName checks saving the same identity
// again leaves one grader and keeps the stored name, because a name is not part
// of a grader's identity.
func TestSaveGraderReusesSameIdentityIgnoringName(t *testing.T) {
	s := newTestStore(t)
	grader := Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}

	if err := s.SaveGrader(grader); err != nil {
		t.Fatal(err)
	}
	renamed := grader
	renamed.Name = "renamed-judge"
	if err := s.SaveGrader(renamed); err != nil {
		t.Fatal(err)
	}

	graders, err := s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 1 {
		t.Fatalf("ListGraders returned %d graders, want 1", len(graders))
	}
	if graders[0].Name != grader.Name {
		t.Errorf("grader name = %q, want the stored name %q", graders[0].Name, grader.Name)
	}
}

// TestSaveGraderSeparatesDifferentValues checks a grader that differs in its
// repo, task, test, stub, commit, branch, version, or uncommitted flag is a
// different record.
func TestSaveGraderSeparatesDifferentValues(t *testing.T) {
	s := newTestStore(t)
	base := Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}
	for _, grader := range []Grader{
		base,
		{Org: "other", Repo: base.Repo, Task: base.Task, Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: "gadgets", Task: base.Task, Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: "ship-it", Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Test: "other", Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Test: base.Test, Name: base.Name, Stub: "other", Commit: base.Commit, Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: "abc123", Branch: base.Branch},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: "dev"},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch, Version: "2.0.0"},
		{Org: base.Org, Repo: base.Repo, Task: base.Task, Test: base.Test, Name: base.Name, Stub: base.Stub, Commit: base.Commit, Branch: base.Branch, Uncommitted: true},
	} {
		if err := s.SaveGrader(grader); err != nil {
			t.Fatal(err)
		}
	}

	graders, err := s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 10 {
		t.Errorf("ListGraders returned %d graders, want 10", len(graders))
	}
}

// TestSaveGraderRejectsIncomplete checks a grader without a name, stub, commit,
// or branch is refused and stored nowhere.
func TestSaveGraderRejectsIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name   string
		grader Grader
	}{
		{"missing name", Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Stub: "judge", Commit: "def456", Branch: "main"}},
		{"missing stub", Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Commit: "def456", Branch: "main"}},
		{"missing commit", Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Branch: "main"}},
		{"missing branch", Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Commit: "def456"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)

			if err := s.SaveGrader(tc.grader); !errors.Is(err, ErrInvalidGrader) {
				t.Errorf("SaveGrader error = %v, want ErrInvalidGrader", err)
			}
			graders, err := s.ListGraders()
			if err != nil {
				t.Fatal(err)
			}
			if len(graders) != 0 {
				t.Errorf("ListGraders returned %d graders, want none", len(graders))
			}
		})
	}
}

// TestSaveResultStoresAndReusesModel checks a result creates the model it
// carries, reuses it for the same values, and links a different model value to
// a different record.
func TestSaveResultStoresAndReusesModel(t *testing.T) {
	s := newTestStore(t)
	base := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
	}

	ref, err := s.SaveResult(base)
	if err != nil {
		t.Fatalf("SaveResult: %v", err)
	}
	got, err := s.GetResult("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if got.Model != base.Model {
		t.Errorf("result model = %#v, want %#v", got.Model, base.Model)
	}

	again := base
	again.Data = "ffffffffffff"
	if _, err := s.SaveResult(again); err != nil {
		t.Fatal(err)
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Errorf("ListModels returned %d models, want 1", len(models))
	}

	other := base
	other.Data = "eeeeeeeeeeee"
	other.Model.Variant = "low"
	if _, err := s.SaveResult(other); err != nil {
		t.Fatal(err)
	}
	models, err = s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Errorf("ListModels returned %d models, want 2", len(models))
	}
}

// TestSaveResultRejectsIncompleteModel checks a result carrying a model without
// a name or provider is refused and creates neither the result nor the model.
func TestSaveResultRejectsIncompleteModel(t *testing.T) {
	s := newTestStore(t)
	r := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
		Model: Model{Provider: "anthropic", Variant: "high"},
	}

	if _, err := s.SaveResult(r); !errors.Is(err, ErrInvalidModel) {
		t.Errorf("SaveResult error = %v, want ErrInvalidModel", err)
	}
	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("ListResults returned %d results, want none", len(results))
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Errorf("ListModels returned %d models, want none", len(models))
	}
}

// TestSaveResultStoresAndReusesAgent checks a result creates the agent it
// carries, associates it with the result's task, reuses it for the same values,
// and links a different agent value or task to a different record.
func TestSaveResultStoresAndReusesAgent(t *testing.T) {
	s := newTestStore(t)
	base := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
		Agent: Agent{Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	}
	want := base.Agent
	want.Org, want.Repo, want.Task = base.Org, base.Repo, base.Task

	ref, err := s.SaveResult(base)
	if err != nil {
		t.Fatalf("SaveResult: %v", err)
	}
	got, err := s.GetResult("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if got.Agent != want {
		t.Errorf("result agent = %#v, want %#v", got.Agent, want)
	}

	again := base
	again.Data = "ffffffffffff"
	if _, err := s.SaveResult(again); err != nil {
		t.Fatal(err)
	}
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Errorf("ListAgents returned %d agents, want 1", len(agents))
	}

	otherTask := base
	otherTask.Task = "ship-it"
	otherTask.Data = "eeeeeeeeeeee"
	if _, err := s.SaveResult(otherTask); err != nil {
		t.Fatal(err)
	}
	agents, err = s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Errorf("ListAgents returned %d agents, want 2", len(agents))
	}

	otherAgent := base
	otherAgent.Data = "dddddddddddd"
	otherAgent.Agent.Version = "1.2.3"
	if _, err := s.SaveResult(otherAgent); err != nil {
		t.Fatal(err)
	}
	agents, err = s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 3 {
		t.Errorf("ListAgents returned %d agents, want 3", len(agents))
	}
}

// TestSaveResultRejectsIncompleteAgent checks a result carrying an agent without
// a name, stub, commit, or branch is refused and creates neither the result nor
// the agent.
func TestSaveResultRejectsIncompleteAgent(t *testing.T) {
	s := newTestStore(t)
	r := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`,
		Agent: Agent{Name: "claude-code", Stub: "claude", Commit: "abc123"},
	}

	if _, err := s.SaveResult(r); !errors.Is(err, ErrInvalidAgent) {
		t.Errorf("SaveResult error = %v, want ErrInvalidAgent", err)
	}
	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("ListResults returned %d results, want none", len(results))
	}
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Errorf("ListAgents returned %d agents, want none", len(agents))
	}
}

// TestSaveResultWriteFailureCreatesNoModelOrAgent checks a result write that
// fails after storing its model and agent leaves neither record behind.
func TestSaveResultWriteFailureCreatesNoModelOrAgent(t *testing.T) {
	s := newTestStore(t)
	r := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", JSON: `{"answer":"hi"}`, Logs: "bad\x00logs",
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	}

	if _, err := s.SaveResult(r); err == nil {
		t.Fatal("SaveResult = nil, want an error for a write Postgres rejects")
	}
	results, err := s.ListResults("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("ListResults returned %d results, want none", len(results))
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Errorf("ListModels returned %d models, want none", len(models))
	}
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Errorf("ListAgents returned %d agents, want none", len(agents))
	}
}

// TestSaveScoreStoresAndReusesModel checks a score creates the model it
// carries, reuses it for the same values, and links a different model value to
// a different record.
func TestSaveScoreStoresAndReusesModel(t *testing.T) {
	s := newTestStore(t)
	base := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
	}

	ref, err := s.SaveScore(base)
	if err != nil {
		t.Fatalf("SaveScore: %v", err)
	}
	got, err := s.GetScore("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}
	if got.Model != base.Model {
		t.Errorf("score model = %#v, want %#v", got.Model, base.Model)
	}

	again := base
	again.Result = "ffffffffffff"
	if _, err := s.SaveScore(again); err != nil {
		t.Fatal(err)
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Errorf("ListModels returned %d models, want 1", len(models))
	}

	other := base
	other.Result = "eeeeeeeeeeee"
	other.Model.Variant = "low"
	if _, err := s.SaveScore(other); err != nil {
		t.Fatal(err)
	}
	models, err = s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Errorf("ListModels returned %d models, want 2", len(models))
	}
}

// TestSaveScoreRejectsIncompleteModel checks a score carrying a model without a
// name or provider is refused and creates neither the score nor the model.
func TestSaveScoreRejectsIncompleteModel(t *testing.T) {
	s := newTestStore(t)
	sc := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
		Model: Model{Name: "claude-sonnet", Variant: "high"},
	}

	if _, err := s.SaveScore(sc); !errors.Is(err, ErrInvalidModel) {
		t.Errorf("SaveScore error = %v, want ErrInvalidModel", err)
	}
	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 0 {
		t.Errorf("ListScores returned %d scores, want none", len(scores))
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Errorf("ListModels returned %d models, want none", len(models))
	}
}

// TestSaveScoreStoresAndReusesGrader checks a score creates the grader it
// carries, associates it with the result's test, reuses it for the same values,
// and links a different grader value or test to a different record.
func TestSaveScoreStoresAndReusesGrader(t *testing.T) {
	s := newTestStore(t)
	base := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
		Grader: Grader{Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	}
	want := base.Grader
	want.Org, want.Repo, want.Task, want.Test = base.Org, base.Repo, base.Task, base.Test

	ref, err := s.SaveScore(base)
	if err != nil {
		t.Fatalf("SaveScore: %v", err)
	}
	got, err := s.GetScore("acme", "widgets", ref)
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}
	if got.Grader != want {
		t.Errorf("score grader = %#v, want %#v", got.Grader, want)
	}

	again := base
	again.Result = "ffffffffffff"
	if _, err := s.SaveScore(again); err != nil {
		t.Fatal(err)
	}
	graders, err := s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 1 {
		t.Errorf("ListGraders returned %d graders, want 1", len(graders))
	}

	otherTest := base
	otherTest.Test = "other"
	otherTest.Result = "eeeeeeeeeeee"
	if _, err := s.SaveScore(otherTest); err != nil {
		t.Fatal(err)
	}
	graders, err = s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 2 {
		t.Errorf("ListGraders returned %d graders, want 2", len(graders))
	}

	otherGrader := base
	otherGrader.Result = "dddddddddddd"
	otherGrader.Grader.Version = "2.0.0"
	if _, err := s.SaveScore(otherGrader); err != nil {
		t.Fatal(err)
	}
	graders, err = s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 3 {
		t.Errorf("ListGraders returned %d graders, want 3", len(graders))
	}
}

// TestSaveScoreRejectsIncompleteGrader checks a score carrying a grader without
// a name, stub, commit, or branch is refused and creates neither the score nor
// the grader.
func TestSaveScoreRejectsIncompleteGrader(t *testing.T) {
	s := newTestStore(t)
	sc := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
		Model:  Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: Grader{Name: "llm-judge", Stub: "judge", Commit: "def456"},
	}

	if _, err := s.SaveScore(sc); !errors.Is(err, ErrInvalidGrader) {
		t.Errorf("SaveScore error = %v, want ErrInvalidGrader", err)
	}
	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 0 {
		t.Errorf("ListScores returned %d scores, want none", len(scores))
	}
	graders, err := s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 0 {
		t.Errorf("ListGraders returned %d graders, want none", len(graders))
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Errorf("ListModels returned %d models, want none", len(models))
	}
}

// TestSaveScoreWriteFailureCreatesNoModelOrGrader checks a score write that
// fails after storing its model and grader leaves neither record behind.
func TestSaveScoreWriteFailureCreatesNoModelOrGrader(t *testing.T) {
	s := newTestStore(t)
	sc := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1},
		Logs:   "bad\x00logs",
		Model:  Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: Grader{Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	}

	if _, err := s.SaveScore(sc); err == nil {
		t.Fatal("SaveScore = nil, want an error for a write Postgres rejects")
	}
	scores, err := s.ListScores("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 0 {
		t.Errorf("ListScores returned %d scores, want none", len(scores))
	}
	models, err := s.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Errorf("ListModels returned %d models, want none", len(models))
	}
	graders, err := s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 0 {
		t.Errorf("ListGraders returned %d graders, want none", len(graders))
	}
}

// TestScoreRef checks a ref is short and URL-safe, is stable for the same
// dictionary within the same result, is independent of dictionary key order,
// and changes when the dictionary or any part of the result identity changes.
func TestScoreRef(t *testing.T) {
	base := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "56ac1b0e4f0d", Dictionary: map[string]float64{"correctness": 1, "style": 0.5},
		Grader: Grader{Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	}
	ref := ScoreRef(base)
	if ref != ScoreRef(base) {
		t.Errorf("ScoreRef returned different refs for the same score")
	}
	if ref == "" || !ValidStub(ref) {
		t.Errorf("ScoreRef = %q, want a non-empty URL-safe ref", ref)
	}

	reordered := base
	reordered.Dictionary = map[string]float64{"style": 0.5, "correctness": 1}
	if got := ScoreRef(reordered); got != ref {
		t.Errorf("ScoreRef = %q, want the same ref whatever the dictionary key order", got)
	}

	for _, tc := range []struct {
		name   string
		change func(Score) Score
	}{
		{"different dictionary", func(s Score) Score { s.Dictionary = map[string]float64{"correctness": 0.5, "style": 0.5}; return s }},
		{"different result", func(s Score) Score { s.Result = "ffffffffffff"; return s }},
		{"different test", func(s Score) Score { s.Test = "other"; return s }},
		{"different task", func(s Score) Score { s.Task = "other"; return s }},
		{"different repo", func(s Score) Score { s.Repo = "gadgets"; return s }},
		{"different model name", func(s Score) Score { s.Model.Name = "claude-opus"; return s }},
		{"different model provider", func(s Score) Score { s.Model.Provider = "openai"; return s }},
		{"different model variant", func(s Score) Score { s.Model.Variant = "low"; return s }},
		{"different grader stub", func(s Score) Score { s.Grader.Stub = "other"; return s }},
		{"different grader commit", func(s Score) Score { s.Grader.Commit = "abc123"; return s }},
		{"different grader branch", func(s Score) Score { s.Grader.Branch = "dev"; return s }},
		{"different grader version", func(s Score) Score { s.Grader.Version = "2.0.0"; return s }},
		{"uncommitted grader", func(s Score) Score { s.Grader.Uncommitted = true; return s }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScoreRef(tc.change(base)); got == ref {
				t.Errorf("ScoreRef = %q, want a different ref", got)
			}
		})
	}
}
