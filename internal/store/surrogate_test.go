package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestIdsStayWithTheirRecords checks the service assigns each record an id and
// keeps it when the same record is written again.
func TestIdsStayWithTheirRecords(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveRepo(Repo{Org: "acme", Stub: "widgets", URL: "https://github.com/acme/widgets"}); err != nil {
		t.Fatal(err)
	}
	repo, err := s.GetRepo("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID == 0 {
		t.Fatal("repo id = 0, want a service-assigned id")
	}
	repo.URL = "https://example.com/acme/widgets"
	if err := s.SaveRepo(repo); err != nil {
		t.Fatal(err)
	}
	again, err := s.GetRepo("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != repo.ID {
		t.Errorf("repo id after a second write = %d, want %d", again.ID, repo.ID)
	}

	task := Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug"}
	if err := s.SaveTask(task); err != nil {
		t.Fatal(err)
	}
	savedTask, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if savedTask.ID == 0 || savedTask.RepoID != repo.ID {
		t.Errorf("task id = %d, repo id = %d, want an id and repo %d", savedTask.ID, savedTask.RepoID, repo.ID)
	}
	savedTask.Description = "updated"
	if err := s.SaveTask(savedTask); err != nil {
		t.Fatal(err)
	}
	againTask, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if againTask.ID != savedTask.ID {
		t.Errorf("task id after a second write = %d, want %d", againTask.ID, savedTask.ID)
	}
	if againTask.Description != "updated" {
		t.Errorf("task description = %q, want the updated description", againTask.Description)
	}

	test := Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Version: "1"}
	if err := s.SaveTest(test); err != nil {
		t.Fatal(err)
	}
	savedTest, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if savedTest.ID == 0 || savedTest.TaskID != savedTask.ID {
		t.Errorf("test id = %d, task id = %d, want an id and task %d", savedTest.ID, savedTest.TaskID, savedTask.ID)
	}
	savedTest.Version = "2"
	if err := s.SaveTest(savedTest); err != nil {
		t.Fatal(err)
	}
	againTest, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if againTest.ID != savedTest.ID {
		t.Errorf("test id after a second write = %d, want %d", againTest.ID, savedTest.ID)
	}
	if againTest.Version != "2" {
		t.Errorf("test version = %q, want the updated version", againTest.Version)
	}

	body := []byte(`{"prompt":"hi"}`)
	if _, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", body); err != nil {
		t.Fatal(err)
	}
	data, err := s.ListData("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 {
		t.Fatalf("ListData returned %d data, want 1", len(data))
	}
	if data[0].ID == 0 || data[0].TestID != savedTest.ID {
		t.Errorf("datum id = %d, test id = %d, want an id and test %d", data[0].ID, data[0].TestID, savedTest.ID)
	}
	if _, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", body); err != nil {
		t.Fatal(err)
	}
	againData, err := s.ListData("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(againData) != 1 || againData[0].ID != data[0].ID {
		t.Errorf("data after a second write = %#v, want one datum with id %d", againData, data[0].ID)
	}
}

// TestParentLinksUseIds checks a task, test, and datum store only the id of
// their parent, not the parent's stubs.
func TestParentLinksUseIds(t *testing.T) {
	s := newTestStore(t)

	for _, tc := range []struct {
		model   any
		present string
		absent  []string
	}{
		{&Task{}, "repo_id", []string{"org", "repo"}},
		{&Test{}, "task_id", []string{"org", "repo", "task"}},
		{&Datum{}, "test_id", []string{"org", "repo", "task", "test"}},
	} {
		if !s.db.Migrator().HasColumn(tc.model, tc.present) {
			t.Errorf("%T has no %q column", tc.model, tc.present)
		}
		for _, column := range tc.absent {
			if s.db.Migrator().HasColumn(tc.model, column) {
				t.Errorf("%T has a %q column, want no parent stub column", tc.model, column)
			}
		}
	}
}

// TestChildrenRequireParents checks the store refuses a task, test, or datum
// when its parent does not exist.
func TestChildrenRequireParents(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveTask(Task{Org: "acme", Repo: "missing", Stub: "fix-bug", Name: "Fix a bug"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveTask error = %v, want ErrNotFound", err)
	}
	if err := s.SaveTest(Test{Org: "acme", Repo: "missing", Task: "missing", Stub: "basic", Name: "Basic", Version: "1"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveTest error = %v, want ErrNotFound", err)
	}
	if _, err := s.SaveDatum("acme", "missing", "missing", "missing", []byte(`{}`)); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveDatum error = %v, want ErrNotFound", err)
	}

	if err := s.db.Create(&Task{RepoID: 999999, Stub: "orphan", Name: "Orphan"}).Error; err == nil {
		t.Error("database accepted a task whose repo is missing")
	}
	if err := s.db.Create(&Test{TaskID: 999999, Stub: "orphan", Name: "Orphan", Version: "1"}).Error; err == nil {
		t.Error("database accepted a test whose task is missing")
	}
	if err := s.db.Create(&Datum{TestID: 999999, Ref: "orphan", JSON: "{}"}).Error; err == nil {
		t.Error("database accepted a datum whose test is missing")
	}
}

// TestStubsStayUniqueWithinParent checks the same stub in two parents is two
// records while a repeated stub in one parent updates its record.
func TestStubsStayUniqueWithinParent(t *testing.T) {
	s := newTestStore(t)

	for _, repo := range []Repo{
		{Org: "acme", Stub: "widgets", URL: "https://example.com/acme/widgets"},
		{Org: "acme", Stub: "gadgets", URL: "https://example.com/acme/gadgets"},
	} {
		if err := s.SaveRepo(repo); err != nil {
			t.Fatal(err)
		}
	}
	for _, repo := range []string{"widgets", "gadgets"} {
		if err := s.SaveTask(Task{Org: "acme", Repo: repo, Stub: "fix-bug", Name: "Fix a bug"}); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := s.ListTasks("acme", "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("ListTasks(widgets) returned %d tasks, want 1", len(tasks))
	}
	taskID := tasks[0].ID

	if err := s.SaveTask(Task{Org: "acme", Repo: "widgets", Stub: "ship-it", Name: "Ship it"}); err != nil {
		t.Fatal(err)
	}
	for _, task := range []string{"fix-bug", "ship-it"} {
		if err := s.SaveTest(Test{Org: "acme", Repo: "widgets", Task: task, Stub: "basic", Name: "Basic", Version: "1"}); err != nil {
			t.Fatal(err)
		}
	}
	tests, err := s.ListTests("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 1 {
		t.Fatalf("ListTests returned %d tests, want 1", len(tests))
	}
	if tests[0].TaskID != taskID {
		t.Errorf("test task id = %d, want %d", tests[0].TaskID, taskID)
	}
}

// TestRenameKeepsIdsAndChildLinks checks renaming a task or test keeps its id
// and the links from its children.
func TestRenameKeepsIdsAndChildLinks(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveRepo(Repo{Org: "acme", Stub: "widgets", URL: "https://example.com/acme/widgets"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTask(Task{Org: "acme", Repo: "widgets", Stub: "fix-bug", Name: "Fix a bug"}); err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTest(Test{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "basic", Name: "Basic", Version: "1"}); err != nil {
		t.Fatal(err)
	}
	test, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveDatum("acme", "widgets", "fix-bug", "basic", []byte(`{"prompt":"hi"}`)); err != nil {
		t.Fatal(err)
	}

	movedTask, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo")
	if err != nil {
		t.Fatal(err)
	}
	if movedTask.ID != task.ID {
		t.Errorf("moved task id = %d, want %d", movedTask.ID, task.ID)
	}
	movedTest, err := s.GetTest("acme", "widgets", "fix-typo", "basic")
	if err != nil {
		t.Fatalf("GetTest under the renamed task: %v", err)
	}
	if movedTest.ID != test.ID {
		t.Errorf("test id after the task rename = %d, want %d", movedTest.ID, test.ID)
	}

	movedTest, err = s.MoveTest("acme", "widgets", "fix-typo", "basic", "smoke")
	if err != nil {
		t.Fatal(err)
	}
	if movedTest.ID != test.ID {
		t.Errorf("moved test id = %d, want %d", movedTest.ID, test.ID)
	}
	data, err := s.ListData("acme", "widgets", "fix-typo", "smoke")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 {
		t.Fatalf("ListData under the renamed test returned %d data, want 1", len(data))
	}
	if data[0].TestID != test.ID {
		t.Errorf("datum test id after the test rename = %d, want %d", data[0].TestID, test.ID)
	}
}

// TestMigrateMovesLegacyStubSchema checks migration carries the records of a
// database that identified repos, tasks, tests, and data by stubs into the
// id-linked tables.
func TestMigrateMovesLegacyStubSchema(t *testing.T) {
	pg := localPostgres(t)
	admin, err := open(context.Background(), pg)
	if err != nil {
		t.Skipf("Postgres at %s:%d is unavailable: %v", pg.Host, pg.Port, err)
	}
	schema := fmt.Sprintf("abnch_legacy_%d", time.Now().UnixNano())
	if err := admin.Exec("create schema " + schema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	db, err := gorm.Open(postgres.Open(DSN(pg)+" search_path="+schema), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	s := New(db)
	t.Cleanup(func() {
		s.Close()
		admin.Exec("drop schema if exists " + schema + " cascade")
		if conn, err := admin.DB(); err == nil {
			conn.Close()
		}
	})

	for _, stmt := range []string{
		`create table repos (org text not null, stub text not null, url text, primary key (org, stub))`,
		`create table tasks (org text not null, repo text not null, stub text not null, name text, description text, primary key (org, repo, stub))`,
		`create table tests (org text not null, repo text not null, task text not null, stub text not null, name text, description text, version text, schema jsonb, primary key (org, repo, task, stub))`,
		`create table data (org text not null, repo text not null, task text not null, test text not null, ref text not null, json text, primary key (org, repo, task, test, ref))`,
		`insert into repos values ('acme', 'widgets', 'https://example.com/acme/widgets')`,
		`insert into tasks values ('acme', 'widgets', 'fix-bug', 'Fix a bug', 'A broken widget')`,
		`insert into tests values ('acme', 'widgets', 'fix-bug', 'basic', 'Basic', 'A basic test', '1', '[]')`,
		`insert into data values ('acme', 'widgets', 'fix-bug', 'basic', 'abc123', '{"prompt":"hi"}')`,
	} {
		if err := s.db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	repo, err := s.GetRepo("acme", "widgets")
	if err != nil {
		t.Fatalf("GetRepo after migration: %v", err)
	}
	task, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatalf("GetTask after migration: %v", err)
	}
	test, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatalf("GetTest after migration: %v", err)
	}
	datum, err := s.GetDatum("acme", "widgets", "fix-bug", "basic", "abc123")
	if err != nil {
		t.Fatalf("GetDatum after migration: %v", err)
	}
	if repo.ID == 0 || task.RepoID != repo.ID || test.TaskID != task.ID || datum.TestID != test.ID {
		t.Errorf("migrated links = repo %d, task %d/%d, test %d/%d, datum %d/%d, want them linked",
			repo.ID, task.ID, task.RepoID, test.ID, test.TaskID, datum.ID, datum.TestID)
	}
	if datum.JSON != `{"prompt":"hi"}` {
		t.Errorf("migrated datum JSON = %q, want it unchanged", datum.JSON)
	}
	if !s.db.Migrator().HasColumn(&Task{}, "repo_id") || s.db.Migrator().HasColumn(&Task{}, "org") {
		t.Error("tasks after migration do not have the id-linked columns only")
	}
}
