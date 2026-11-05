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

// TestRenameTaskKeepsResultsAndScoresLinked checks renaming a task keeps its
// test, data, results, scores, and agent linked and returns the same records
// under the new stub.
func TestRenameTaskKeepsResultsAndScoresLinked(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	data := saveDatum(t, s, "acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	result := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: data,
		Body:  `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	}
	score := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Body:   map[string]float64{"correctness": 1},
		Model:  Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	}
	resultRef, err := s.SaveResult(result)
	if err != nil {
		t.Fatal(err)
	}
	score.Result = resultRef
	scoreRef, err := s.SaveScore(score)
	if err != nil {
		t.Fatal(err)
	}
	beforeResult, err := s.GetResult("acme", "widgets", resultRef)
	if err != nil {
		t.Fatal(err)
	}
	beforeScore, err := s.GetScore("acme", "widgets", scoreRef)
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask("acme", "widgets", "fix-bug")
	if err != nil {
		t.Fatal(err)
	}
	test, err := s.GetTest("acme", "widgets", "fix-bug", "basic")
	if err != nil {
		t.Fatal(err)
	}
	datum, err := s.GetDatum("acme", "widgets", "fix-bug", "basic", data)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := s.GetAgent(result.Agent)
	if err != nil {
		t.Fatal(err)
	}

	moved, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo")
	if err != nil {
		t.Fatal(err)
	}
	if moved.ID != task.ID {
		t.Errorf("moved task id = %d, want %d", moved.ID, task.ID)
	}
	movedTest, err := s.GetTest("acme", "widgets", "fix-typo", "basic")
	if err != nil {
		t.Fatalf("GetTest under the renamed task: %v", err)
	}
	if movedTest.ID != test.ID || movedTest.Name != test.Name || movedTest.Version != test.Version {
		t.Errorf("test after the task rename = %#v, want %#v", movedTest, test)
	}
	movedData, err := s.ListData("acme", "widgets", "fix-typo", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if len(movedData) != 1 || movedData[0].ID != datum.ID || movedData[0].Ref != datum.Ref {
		t.Errorf("data after the task rename = %#v, want the datum %#v", movedData, datum)
	}

	afterResult, err := s.GetResult("acme", "widgets", resultRef)
	if err != nil {
		t.Fatalf("GetResult after the task rename: %v", err)
	}
	if afterResult.ID != beforeResult.ID || afterResult.DatumID != beforeResult.DatumID ||
		afterResult.ModelID != beforeResult.ModelID || afterResult.AgentID != beforeResult.AgentID ||
		afterResult.Body != beforeResult.Body || afterResult.Data != beforeResult.Data ||
		afterResult.Task != "fix-typo" {
		t.Errorf("result after the task rename = %#v, want %#v under the new stub", afterResult, beforeResult)
	}
	afterScore, err := s.GetScore("acme", "widgets", scoreRef)
	if err != nil {
		t.Fatalf("GetScore after the task rename: %v", err)
	}
	if afterScore.ID != beforeScore.ID || afterScore.ResultID != beforeScore.ResultID ||
		afterScore.ModelID != beforeScore.ModelID || afterScore.GraderID != beforeScore.GraderID ||
		afterScore.Body["correctness"] != beforeScore.Body["correctness"] ||
		afterScore.Task != "fix-typo" {
		t.Errorf("score after the task rename = %#v, want %#v under the new stub", afterScore, beforeScore)
	}

	movedAgent := result.Agent
	movedAgent.Task = "fix-typo"
	gotAgent, err := s.GetAgent(movedAgent)
	if err != nil {
		t.Fatalf("GetAgent under the renamed task: %v", err)
	}
	if gotAgent.ID != agent.ID {
		t.Errorf("agent id after the task rename = %d, want %d", gotAgent.ID, agent.ID)
	}
	for _, tc := range []struct {
		name  string
		model any
	}{
		{"results", &resultRow{}},
		{"scores", &scoreRow{}},
		{"agents", &Agent{}},
	} {
		var count int64
		if err := s.db.Model(tc.model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s after the task rename = %d, want 1", tc.name, count)
		}
	}
}

// TestResultAfterTaskRenameReusesAgent checks a later result for a renamed task
// with the same agent values reuses the stored agent record.
func TestResultAfterTaskRenameReusesAgent(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	firstData := saveDatum(t, s, "acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	agent := Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"}
	model := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	if _, err := s.SaveResult(Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: firstData,
		Body: `{"answer":"hi"}`, Model: model, Agent: agent,
	}); err != nil {
		t.Fatal(err)
	}
	savedAgent, err := s.GetAgent(agent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.MoveTask("acme", "widgets", "fix-bug", "fix-typo"); err != nil {
		t.Fatal(err)
	}

	secondData := saveDatum(t, s, "acme", "widgets", "fix-typo", "basic", `{"prompt":"bye"}`)
	renamedAgent := agent
	renamedAgent.Task = "fix-typo"
	secondRef, err := s.SaveResult(Result{
		Org: "acme", Repo: "widgets", Task: "fix-typo", Test: "basic", Data: secondData,
		Body: `{"answer":"bye"}`, Model: model, Agent: renamedAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.GetResult("acme", "widgets", secondRef)
	if err != nil {
		t.Fatal(err)
	}
	if second.AgentID != savedAgent.ID || second.Agent.ID != savedAgent.ID {
		t.Errorf("second result agent = %d/%d, want the stored agent %d", second.AgentID, second.Agent.ID, savedAgent.ID)
	}
	var count int64
	if err := s.db.Model(&Agent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("agents after the second result = %d, want 1", count)
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
		`DROP TABLE "results", "scores", "agents", "graders", "models"`,
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

// TestResultsAndScoresLinkByIDs checks results and scores store only ids for
// their linked records and return those records and parent stubs when read.
func TestResultsAndScoresLinkByIDs(t *testing.T) {
	s := newTestStore(t)

	for _, tc := range []struct {
		model   any
		present []string
		absent  []string
	}{
		{
			&Result{},
			[]string{"id", "datum_id", "model_id", "agent_id"},
			[]string{"org", "repo", "task", "test", "data", "name", "provider", "variant", "model_name", "model_provider", "model_variant", "agent_name", "agent_stub", "agent_commit", "agent_branch"},
		},
		{
			&Score{},
			[]string{"id", "result_id", "model_id", "grader_id"},
			[]string{"org", "repo", "task", "test", "result", "name", "provider", "variant", "model_name", "model_provider", "model_variant", "grader_name", "grader_stub", "grader_commit", "grader_branch"},
		},
	} {
		for _, column := range tc.present {
			if !s.db.Migrator().HasColumn(tc.model, column) {
				t.Errorf("%T has no %q column", tc.model, column)
			}
		}
		for _, column := range tc.absent {
			if s.db.Migrator().HasColumn(tc.model, column) {
				t.Errorf("%T has a %q column, want it linked by id", tc.model, column)
			}
		}
	}

	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	dataRef := saveDatum(t, s, "acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	datum, err := s.GetDatum("acme", "widgets", "fix-bug", "basic", dataRef)
	if err != nil {
		t.Fatal(err)
	}

	resultRef, err := s.SaveResult(Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: dataRef,
		Body:  `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	model, err := s.GetModel("claude-sonnet", "anthropic", "high")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := s.GetAgent(Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Stub: "claude", Commit: "abc123", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.GetResult("acme", "widgets", resultRef)
	if err != nil {
		t.Fatal(err)
	}
	if result.DatumID != datum.ID || result.ModelID != model.ID || result.AgentID != agent.ID {
		t.Errorf("result links = datum %d, model %d, agent %d, want %d, %d, and %d",
			result.DatumID, result.ModelID, result.AgentID, datum.ID, model.ID, agent.ID)
	}
	if result.Data != dataRef || result.Model != model || result.Agent != agent {
		t.Errorf("result = %#v, want its datum ref and linked records", result)
	}

	scoreRef, err := s.SaveScore(Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Result: resultRef,
		Body:   map[string]float64{"correctness": 1},
		Model:  Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	grader, err := s.GetGrader(Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Stub: "judge", Commit: "def456", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	var resultRow resultRow
	if err := s.db.First(&resultRow, "ref = ?", resultRef).Error; err != nil {
		t.Fatal(err)
	}
	var score scoreRow
	if err := s.db.First(&score, "ref = ?", scoreRef).Error; err != nil {
		t.Fatal(err)
	}
	if score.ResultID != resultRow.ID || score.ModelID != model.ID || score.GraderID != grader.ID {
		t.Errorf("score links = result %d, model %d, grader %d, want %d, %d, and %d",
			score.ResultID, score.ModelID, score.GraderID, resultRow.ID, model.ID, grader.ID)
	}
}

// TestResultsAndScoresRequireLinkedRecords checks the store and database refuse
// a result or score whose linked records are missing.
func TestResultsAndScoresRequireLinkedRecords(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	dataRef := saveDatum(t, s, "acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	model := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	agent := Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"}
	grader := Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"}
	result := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: dataRef,
		Body: `{"answer":"hi"}`, Model: model, Agent: agent,
	}

	missingDatum := result
	missingDatum.Data = "missing"
	if _, err := s.SaveResult(missingDatum); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveResult with a missing datum error = %v, want ErrNotFound", err)
	}
	missingModel := result
	missingModel.Model = Model{}
	if _, err := s.SaveResult(missingModel); !errors.Is(err, ErrInvalidModel) {
		t.Errorf("SaveResult without a model error = %v, want ErrInvalidModel", err)
	}
	missingAgent := result
	missingAgent.Agent = Agent{}
	if _, err := s.SaveResult(missingAgent); !errors.Is(err, ErrInvalidAgent) {
		t.Errorf("SaveResult without an agent error = %v, want ErrInvalidAgent", err)
	}

	resultRef, err := s.SaveResult(result)
	if err != nil {
		t.Fatal(err)
	}
	savedModel, err := s.GetModel(model.Name, model.Provider, model.Variant)
	if err != nil {
		t.Fatal(err)
	}
	savedAgent, err := s.GetAgent(agent)
	if err != nil {
		t.Fatal(err)
	}
	var savedResult resultRow
	if err := s.db.First(&savedResult, "ref = ?", resultRef).Error; err != nil {
		t.Fatal(err)
	}

	for _, row := range []resultRow{
		{DatumID: 999999, ModelID: savedModel.ID, AgentID: savedAgent.ID, Ref: "orphan-datum"},
		{DatumID: savedResult.DatumID, ModelID: 999999, AgentID: savedAgent.ID, Ref: "orphan-model"},
		{DatumID: savedResult.DatumID, ModelID: savedModel.ID, AgentID: 999999, Ref: "orphan-agent"},
	} {
		if err := s.db.Create(&row).Error; err == nil {
			t.Errorf("database accepted a result row with a missing link: %#v", row)
		}
	}

	score := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Result: resultRef,
		Body: map[string]float64{"correctness": 1}, Model: model, Grader: grader,
	}
	missingResult := score
	missingResult.Result = "missing"
	if _, err := s.SaveScore(missingResult); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveScore with a missing result error = %v, want ErrNotFound", err)
	}
	missingScoreModel := score
	missingScoreModel.Model = Model{}
	if _, err := s.SaveScore(missingScoreModel); !errors.Is(err, ErrInvalidModel) {
		t.Errorf("SaveScore without a model error = %v, want ErrInvalidModel", err)
	}
	missingGrader := score
	missingGrader.Grader = Grader{}
	if _, err := s.SaveScore(missingGrader); !errors.Is(err, ErrInvalidGrader) {
		t.Errorf("SaveScore without a grader error = %v, want ErrInvalidGrader", err)
	}

	scoreRef, err := s.SaveScore(score)
	if err != nil {
		t.Fatal(err)
	}
	savedGrader, err := s.GetGrader(grader)
	if err != nil {
		t.Fatal(err)
	}
	var savedScore scoreRow
	if err := s.db.First(&savedScore, "ref = ?", scoreRef).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []scoreRow{
		{ResultID: 999999, ModelID: savedModel.ID, GraderID: savedGrader.ID, Ref: "orphan-result"},
		{ResultID: savedScore.ResultID, ModelID: 999999, GraderID: savedGrader.ID, Ref: "orphan-score-model"},
		{ResultID: savedScore.ResultID, ModelID: savedModel.ID, GraderID: 999999, Ref: "orphan-grader"},
	} {
		if err := s.db.Create(&row).Error; err == nil {
			t.Errorf("database accepted a score row with a missing link: %#v", row)
		}
	}
}

// TestMigrateMovesLegacyResultsAndScores checks migration carries results and
// scores from the schema that embedded their provenance values into the
// id-linked tables.
func TestMigrateMovesLegacyResultsAndScores(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	dataRef := saveDatum(t, s, "acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	model := Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"}
	agent := Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"}
	grader := Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"}
	if err := s.SaveModel(model); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAgent(agent); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGrader(grader); err != nil {
		t.Fatal(err)
	}

	for _, stmt := range []string{
		`DROP TABLE "scores", "results"`,
		`create table results (
			org text not null, repo text not null, task text not null, test text not null, data text not null, ref text not null,
			name text, provider text, variant text,
			agent_org text, agent_repo text, agent_task text, agent_name text, agent_stub text, agent_commit text,
			agent_branch text, agent_version text, agent_uncommitted boolean,
			body text, logs text, input_tokens integer, output_tokens integer, token_cost double precision, duration double precision, local text,
			primary key (org, repo, task, test, data, ref))`,
		`create table scores (
			org text not null, repo text not null, task text not null, test text not null, result text not null, ref text not null,
			name text, provider text, variant text,
			grader_org text, grader_repo text, grader_task text, grader_test text, grader_name text, grader_stub text,
			grader_commit text, grader_branch text, grader_version text, grader_uncommitted boolean,
			body jsonb, logs text, input_tokens integer, output_tokens integer, token_cost double precision, duration double precision,
			primary key (org, repo, task, test, result, ref))`,
		`insert into results values ('acme', 'widgets', 'fix-bug', 'basic', '` + dataRef + `', 'resultref',
			'claude-sonnet', 'anthropic', 'high',
			'acme', 'widgets', 'fix-bug', 'claude-code', 'claude', 'abc123', 'main', '', false,
			'{"answer":"hi"}', 'worked', 12, 3, 0.01, 1.5, 'main')`,
		`insert into scores values ('acme', 'widgets', 'fix-bug', 'basic', 'resultref', 'scoreref',
			'claude-sonnet', 'anthropic', 'high',
			'acme', 'widgets', 'fix-bug', 'basic', 'llm-judge', 'judge', 'def456', 'main', '', false,
			'{"correctness":1}', 'graded', 7, 2, 0.02, 0.5)`,
	} {
		if err := s.db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	result, err := s.GetResult("acme", "widgets", "resultref")
	if err != nil {
		t.Fatalf("GetResult after migration: %v", err)
	}
	if result.Data != dataRef || result.Body != `{"answer":"hi"}` || result.Model.Name != model.Name || result.Agent.Commit != agent.Commit {
		t.Errorf("migrated result = %#v, want its values and links preserved", result)
	}
	score, err := s.GetScore("acme", "widgets", "scoreref")
	if err != nil {
		t.Fatalf("GetScore after migration: %v", err)
	}
	if score.Result != "resultref" || score.Body["correctness"] != 1 || score.Model.Name != model.Name || score.Grader.Commit != grader.Commit {
		t.Errorf("migrated score = %#v, want its values and links preserved", score)
	}
	if !s.db.Migrator().HasColumn(&Result{}, "datum_id") || s.db.Migrator().HasColumn(&Result{}, "org") {
		t.Error("results after migration do not have the id-linked columns only")
	}
	if !s.db.Migrator().HasColumn(&Score{}, "result_id") || s.db.Migrator().HasColumn(&Score{}, "org") {
		t.Error("scores after migration do not have the id-linked columns only")
	}
}

// TestDuplicateResultsKeepOneLinkedRow checks saving the same result body for a
// datum again returns its ref, leaves one result, and keeps its linked records
// even when the agent carries a different name.
func TestDuplicateResultsKeepOneLinkedRow(t *testing.T) {
	s := newTestStore(t)
	saveRepo(t, s, "acme", "widgets")
	saveTask(t, s, "acme", "widgets", "fix-bug")
	saveTest(t, s, "acme", "widgets", "fix-bug", "basic")
	data := saveDatum(t, s, "acme", "widgets", "fix-bug", "basic", `{"prompt":"hi"}`)
	result := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Data: data,
		Body:  `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Org: "acme", Repo: "widgets", Task: "fix-bug", Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	}

	first, err := s.SaveResult(result)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetResult("acme", "widgets", first)
	if err != nil {
		t.Fatal(err)
	}

	renamed := result
	renamed.Agent.Name = "renamed-agent"
	second, err := s.SaveResult(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("second ref = %q, want the first ref %q", second, first)
	}

	again, err := s.GetResult("acme", "widgets", first)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != saved.ID || again.DatumID != saved.DatumID || again.ModelID != saved.ModelID || again.AgentID != saved.AgentID {
		t.Errorf("result after a second write = %#v, want the links and id of %#v", again, saved)
	}
	if again.Agent.ID != saved.Agent.ID {
		t.Errorf("agent id after a second write = %d, want %d", again.Agent.ID, saved.Agent.ID)
	}

	for _, tc := range []struct {
		name  string
		model any
	}{
		{"results", &resultRow{}},
		{"models", &Model{}},
		{"agents", &Agent{}},
	} {
		var count int64
		if err := s.db.Model(tc.model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s after a second write = %d, want 1", tc.name, count)
		}
	}
}

// TestDuplicateScoresKeepOneLinkedRow checks saving the same score body for a
// result again returns its ref, leaves one score, and keeps its linked records
// even when the grader carries a different name.
func TestDuplicateScoresKeepOneLinkedRow(t *testing.T) {
	s := newTestStore(t)
	resultRef := saveResultBase(t, s, "acme", "widgets", "fix-bug", "basic")
	score := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Result: resultRef,
		Body:   map[string]float64{"correctness": 1},
		Model:  Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: Grader{Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic", Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	}

	first, err := s.SaveScore(score)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetScore("acme", "widgets", first)
	if err != nil {
		t.Fatal(err)
	}

	renamed := score
	renamed.Grader.Name = "renamed-judge"
	second, err := s.SaveScore(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("second ref = %q, want the first ref %q", second, first)
	}

	again, err := s.GetScore("acme", "widgets", first)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != saved.ID || again.ResultID != saved.ResultID || again.ModelID != saved.ModelID || again.GraderID != saved.GraderID {
		t.Errorf("score after a second write = %#v, want the links and id of %#v", again, saved)
	}
	if again.Grader.ID != saved.Grader.ID {
		t.Errorf("grader id after a second write = %d, want %d", again.Grader.ID, saved.Grader.ID)
	}

	for _, tc := range []struct {
		name  string
		model any
	}{
		{"scores", &scoreRow{}},
		{"models", &Model{}},
		{"graders", &Grader{}},
	} {
		var count int64
		if err := s.db.Model(tc.model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s after a second write = %d, want 1", tc.name, count)
		}
	}
}

// TestRefsForExistingValuesStay checks the ref derivation keeps the values
// results and scores already carry, so moving their links to ids leaves old
// refs valid.
func TestRefsForExistingValuesStay(t *testing.T) {
	result := Result{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Data: "56ac1b0e4f0d", Body: `{"answer":"hi"}`,
		Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Agent: Agent{Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
	}
	if got, want := ResultRef(result), "572eab319157"; got != want {
		t.Errorf("ResultRef = %q, want the existing ref %q", got, want)
	}

	score := Score{
		Org: "acme", Repo: "widgets", Task: "fix-bug", Test: "basic",
		Result: "572eab319157", Body: map[string]float64{"correctness": 1},
		Model:  Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
		Grader: Grader{Name: "llm-judge", Stub: "judge", Commit: "def456", Branch: "main"},
	}
	if got, want := ScoreRef(score), "3a2893e00114"; got != want {
		t.Errorf("ScoreRef = %q, want the existing ref %q", got, want)
	}
}
