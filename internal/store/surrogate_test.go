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
		{&Agent{}, "task_id", []string{"org", "repo", "task"}},
		{&Grader{}, "test_id", []string{"org", "repo", "task", "test"}},
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
	if !s.db.Migrator().HasColumn(&Model{}, "id") {
		t.Error("Model has no id column")
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
	if err := s.SaveAgent(Agent{Org: "acme", Repo: "missing", Task: "missing", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveAgent error = %v, want ErrNotFound", err)
	}
	if err := s.SaveGrader(Grader{Org: "acme", Repo: "missing", Task: "missing", Test: "missing", Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveGrader error = %v, want ErrNotFound", err)
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
	if err := s.db.Create(&Agent{TaskID: 999999, Name: "Orphan", Stub: "orphan", Commit: "abc123", Branch: "main"}).Error; err == nil {
		t.Error("database accepted an agent whose task is missing")
	}
	if err := s.db.Create(&Grader{TestID: 999999, Name: "Orphan", Stub: "orphan", Commit: "def456", Branch: "main"}).Error; err == nil {
		t.Error("database accepted a grader whose test is missing")
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

// TestModelsAgentsAndGradersKeepIds checks the service assigns each provenance
// record an id and keeps it when the same identity is written again.
func TestModelsAgentsAndGradersKeepIds(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	task, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	test, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}

	model := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	if err := s.SaveModel(model); err != nil {
		t.Fatal(err)
	}
	savedModel, err := s.GetModel(model.Name, model.Provider, model.Variant)
	if err != nil {
		t.Fatal(err)
	}
	if savedModel.ID == 0 {
		t.Fatal("model id = 0, want a service-assigned id")
	}
	if err := s.SaveModel(savedModel); err != nil {
		t.Fatal(err)
	}
	againModel, err := s.GetModel(model.Name, model.Provider, model.Variant)
	if err != nil {
		t.Fatal(err)
	}
	if againModel.ID != savedModel.ID {
		t.Errorf("model id after a second write = %d, want %d", againModel.ID, savedModel.ID)
	}

	agent := Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}
	if err := s.SaveAgent(agent); err != nil {
		t.Fatal(err)
	}
	savedAgent, err := s.GetAgent(agent)
	if err != nil {
		t.Fatal(err)
	}
	if savedAgent.ID == 0 || savedAgent.TaskID != task.ID {
		t.Errorf("agent id = %d, task id = %d, want an id and task %d", savedAgent.ID, savedAgent.TaskID, task.ID)
	}
	if err := s.SaveAgent(savedAgent); err != nil {
		t.Fatal(err)
	}
	againAgent, err := s.GetAgent(agent)
	if err != nil {
		t.Fatal(err)
	}
	if againAgent.ID != savedAgent.ID {
		t.Errorf("agent id after a second write = %d, want %d", againAgent.ID, savedAgent.ID)
	}

	grader := Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}
	if err := s.SaveGrader(grader); err != nil {
		t.Fatal(err)
	}
	savedGrader, err := s.GetGrader(grader)
	if err != nil {
		t.Fatal(err)
	}
	if savedGrader.ID == 0 || savedGrader.TestID != test.ID {
		t.Errorf("grader id = %d, test id = %d, want an id and test %d", savedGrader.ID, savedGrader.TestID, test.ID)
	}
	if err := s.SaveGrader(savedGrader); err != nil {
		t.Fatal(err)
	}
	againGrader, err := s.GetGrader(grader)
	if err != nil {
		t.Fatal(err)
	}
	if againGrader.ID != savedGrader.ID {
		t.Errorf("grader id after a second write = %d, want %d", againGrader.ID, savedGrader.ID)
	}
}

// TestAgentAndGraderIdentitiesFollowTheirParents checks agents and graders are
// unique within their parent, ignore their name, and keep their link when the
// parent is renamed.
func TestAgentAndGraderIdentitiesFollowTheirParents(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTask(t, s, "acme", "widgets", "ship-it")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	saveTest(t, s, "acme", "widgets", "fix-bug", "other")

	agent := Agent{
		Org: "acme", Repo: "widgets", Task: "fix-bug",
		Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main",
	}
	if err := s.SaveAgent(agent); err != nil {
		t.Fatal(err)
	}
	fixBugAgent, err := s.GetAgent(agent)
	if err != nil {
		t.Fatal(err)
	}
	otherTaskAgent := agent
	otherTaskAgent.Task = "ship-it"
	if err := s.SaveAgent(otherTaskAgent); err != nil {
		t.Fatal(err)
	}
	renamedAgent := agent
	renamedAgent.Name = "renamed-agent"
	if err := s.SaveAgent(renamedAgent); err != nil {
		t.Fatal(err)
	}
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Fatalf("ListAgents returned %d agents, want 2", len(agents))
	}

	grader := Grader{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main",
	}
	if err := s.SaveGrader(grader); err != nil {
		t.Fatal(err)
	}
	basicGrader, err := s.GetGrader(grader)
	if err != nil {
		t.Fatal(err)
	}
	otherTestGrader := grader
	otherTestGrader.Test = "other"
	if err := s.SaveGrader(otherTestGrader); err != nil {
		t.Fatal(err)
	}
	renamedGrader := grader
	renamedGrader.Name = "renamed-judge"
	if err := s.SaveGrader(renamedGrader); err != nil {
		t.Fatal(err)
	}
	graders, err := s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 2 {
		t.Fatalf("ListGraders returned %d graders, want 2", len(graders))
	}

	if _, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo"); err != nil {
		t.Fatal(err)
	}
	movedAgent := agent
	movedAgent.Task = "fix-typo"
	gotAgent, err := s.GetAgent(movedAgent)
	if err != nil {
		t.Fatalf("GetAgent under the renamed task: %v", err)
	}
	if gotAgent.ID != fixBugAgent.ID {
		t.Errorf("agent id after the task rename = %d, want %d", gotAgent.ID, fixBugAgent.ID)
	}
	if gotAgent.Task != "fix-typo" {
		t.Errorf("agent task = %q, want the renamed task", gotAgent.Task)
	}
	if err := s.SaveAgent(movedAgent); err != nil {
		t.Fatal(err)
	}
	agents, err = s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Errorf("ListAgents after the task rename returned %d agents, want 2", len(agents))
	}

	if _, err := s.MoveTest("acme", "widgets", "fix-typo", "basic", "smoke"); err != nil {
		t.Fatal(err)
	}
	movedGrader := grader
	movedGrader.Task = "fix-typo"
	movedGrader.Test = "smoke"
	gotGrader, err := s.GetGrader(movedGrader)
	if err != nil {
		t.Fatalf("GetGrader under the renamed test: %v", err)
	}
	if gotGrader.ID != basicGrader.ID {
		t.Errorf("grader id after the test rename = %d, want %d", gotGrader.ID, basicGrader.ID)
	}
	if gotGrader.Task != "fix-typo" || gotGrader.Test != "smoke" {
		t.Errorf("grader task and test = %q/%q, want the renamed ones", gotGrader.Task, gotGrader.Test)
	}
	if err := s.SaveGrader(movedGrader); err != nil {
		t.Fatal(err)
	}
	graders, err = s.ListGraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(graders) != 2 {
		t.Errorf("ListGraders after the test rename returned %d graders, want 2", len(graders))
	}
}

// TestMigrateMovesLegacyModelsAgentsAndGraders checks migration carries models,
// agents, and graders from a database whose other records already link by id.
func TestMigrateMovesLegacyModelsAgentsAndGraders(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")

	for _, stmt := range []string{
		`DROP TABLE "agents", "graders", "models"`,
		`create table models (name text not null, provider text not null, variant text not null, primary key (name, provider, variant))`,
		`create table agents (org text not null, repo text not null, task text not null, name text, stub text not null, commit text not null, branch text not null, version text not null, uncommitted boolean not null, primary key (org, repo, task, stub, commit, branch, version, uncommitted))`,
		`create table graders (org text not null, repo text not null, task text not null, test text not null, name text, stub text not null, commit text not null, branch text not null, version text not null, uncommitted boolean not null, primary key (org, repo, task, test, stub, commit, branch, version, uncommitted))`,
		`insert into models values ('claude-sonnet', 'anthropic', 'high')`,
		`insert into agents values ('acme', 'widgets', 'fix-bug', 'claude-code', 'claude', 'abc123', 'main', '', false)`,
		`insert into graders values ('acme', 'widgets', 'fix-bug', 'basic', 'llm-judge', 'judge', 'def456', 'main', '', false)`,
	} {
		if err := s.db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	task, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	test, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	model, err := s.GetModel("claude-sonnet", "anthropic", "high")
	if err != nil {
		t.Fatalf("GetModel after migration: %v", err)
	}
	agent, err := s.GetAgent(Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "claude", Commit: "abc123", Branch: "main"})
	if err != nil {
		t.Fatalf("GetAgent after migration: %v", err)
	}
	grader, err := s.GetGrader(Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Stub: "judge", Commit: "def456", Branch: "main"})
	if err != nil {
		t.Fatalf("GetGrader after migration: %v", err)
	}
	if model.ID == 0 || agent.TaskID != task.ID || grader.TestID != test.ID {
		t.Errorf("migrated ids = model %d, agent %d/%d, grader %d/%d, want them linked",
			model.ID, agent.ID, agent.TaskID, grader.ID, grader.TestID)
	}
	if agent.Name != "claude-code" || grader.Name != "llm-judge" {
		t.Errorf("migrated names = %q and %q, want them preserved", agent.Name, grader.Name)
	}
}
