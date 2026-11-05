package store

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound reports that a requested repo or task does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidStub reports that a repo's org stub or repo stub is empty or not
// URL-safe.
var ErrInvalidStub = errors.New("invalid stub")

// ErrConflict reports that a write would collide with an existing record.
var ErrConflict = errors.New("conflict")

// ErrInvalidTest reports that a test is missing a required field.
var ErrInvalidTest = errors.New("invalid test")

// ErrInvalidSchema reports that a test's score schema breaks its contract, such
// as two keys sharing a stub.
var ErrInvalidSchema = errors.New("invalid schema")

// ErrInvalidModel reports that a model is missing its name or provider.
var ErrInvalidModel = errors.New("invalid model")

// ErrInvalidAgent reports that an agent is missing its name, stub, commit, or
// branch.
var ErrInvalidAgent = errors.New("invalid agent")

// ErrInvalidGrader reports that a grader is missing its name, stub, commit, or
// branch.
var ErrInvalidGrader = errors.New("invalid grader")

// ErrInvalidResult reports that a result carries a negative run metric.
var ErrInvalidResult = errors.New("invalid result")

// ErrInvalidScore reports that a score carries a negative run metric.
var ErrInvalidScore = errors.New("invalid score")

// Store reads and writes the service's domain data in Postgres.
type Store struct {
	db *gorm.DB
}

// New returns a store over an open database.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Migrate creates the tables the store needs and the links between their
// parents. It carries repos, tasks, tests, and data from the schema that
// identified them by stubs to the one that links them by id.
func (s *Store) Migrate() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		legacy, err := renameLegacyTables(tx)
		if err != nil {
			return err
		}
		if err := tx.AutoMigrate(&Repo{}, &Task{}, &Test{}, &Model{}, &Agent{}, &Grader{}, &Datum{}, &Result{}, &Score{}); err != nil {
			return err
		}
		for _, fk := range []struct {
			table, name, column, parent string
		}{
			{"tasks", "fk_tasks_repo", "repo_id", "repos"},
			{"tests", "fk_tests_task", "task_id", "tasks"},
			{"data", "fk_data_test", "test_id", "tests"},
		} {
			if err := addForeignKey(tx, fk.table, fk.name, fk.column, fk.parent); err != nil {
				return err
			}
		}
		return moveLegacyData(tx, legacy)
	})
}

// legacyTables records which tables the migration renamed from the stub-keyed
// schema.
type legacyTables struct {
	repos, tasks, tests, data bool
}

// renameLegacyTables renames the tables that still identify repos, tasks,
// tests, and data by stubs, so AutoMigrate creates the id-linked tables. It
// reports no tables on a new database.
func renameLegacyTables(tx *gorm.DB) (legacyTables, error) {
	var legacy legacyTables
	if !tx.Migrator().HasTable("repos") || tx.Migrator().HasColumn(&Repo{}, "id") {
		return legacy, nil
	}

	legacy = legacyTables{
		repos: tx.Migrator().HasTable("repos"),
		tasks: tx.Migrator().HasTable("tasks"),
		tests: tx.Migrator().HasTable("tests"),
		data:  tx.Migrator().HasTable("data"),
	}
	for _, table := range []string{"data", "tests", "tasks", "repos"} {
		if !hasLegacyTable(legacy, table) {
			continue
		}
		if err := tx.Exec(`ALTER TABLE "` + table + `" RENAME TO "` + table + `_legacy"`).Error; err != nil {
			return legacy, err
		}
	}
	return legacy, nil
}

// hasLegacyTable reports whether the table is one the migration renamed.
func hasLegacyTable(legacy legacyTables, table string) bool {
	switch table {
	case "repos":
		return legacy.repos
	case "tasks":
		return legacy.tasks
	case "tests":
		return legacy.tests
	case "data":
		return legacy.data
	}
	return false
}

// moveLegacyData copies the renamed tables into the id-linked tables, assigning
// each record a new id and linking it to its parent. It drops the renamed tables
// once their records are copied.
func moveLegacyData(tx *gorm.DB, legacy legacyTables) error {
	steps := []struct {
		moved bool
		sql   string
	}{
		{legacy.repos, `INSERT INTO "repos" ("org", "stub", "url")
			SELECT "org", "stub", "url" FROM "repos_legacy"`},
		{legacy.tasks, `INSERT INTO "tasks" ("repo_id", "stub", "name", "description")
			SELECT r."id", t."stub", t."name", t."description"
			FROM "tasks_legacy" t
			JOIN "repos" r ON r."org" = t."org" AND r."stub" = t."repo"`},
		{legacy.tests, `INSERT INTO "tests" ("task_id", "stub", "name", "description", "version", "schema")
			SELECT t."id", s."stub", s."name", s."description", s."version", s."schema"
			FROM "tests_legacy" s
			JOIN "repos" r ON r."org" = s."org" AND r."stub" = s."repo"
			JOIN "tasks" t ON t."repo_id" = r."id" AND t."stub" = s."task"`},
		{legacy.data, `INSERT INTO "data" ("test_id", "ref", "json")
			SELECT s."id", d."ref", d."json"
			FROM "data_legacy" d
			JOIN "repos" r ON r."org" = d."org" AND r."stub" = d."repo"
			JOIN "tasks" t ON t."repo_id" = r."id" AND t."stub" = d."task"
			JOIN "tests" s ON s."task_id" = t."id" AND s."stub" = d."test"`},
		{legacy.data, `DROP TABLE "data_legacy"`},
		{legacy.tests, `DROP TABLE "tests_legacy"`},
		{legacy.tasks, `DROP TABLE "tasks_legacy"`},
		{legacy.repos, `DROP TABLE "repos_legacy"`},
	}
	for _, step := range steps {
		if !step.moved {
			continue
		}
		if err := tx.Exec(step.sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// addForeignKey adds a foreign key constraint unless the table already holds
// one with its name, so the database rejects rows whose parent is missing.
func addForeignKey(db *gorm.DB, table, name, column, parent string) error {
	if db.Migrator().HasConstraint(table, name) {
		return nil
	}
	return db.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + name +
		" FOREIGN KEY (" + column + ") REFERENCES " + parent + " (id)").Error
}

// Close closes the store's database connection.
func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// SaveRepo writes a repo, creating it or replacing the stored repo with the
// same org stub and repo stub. It returns ErrInvalidStub when the org stub or
// repo stub is empty or not URL-safe.
func (s *Store) SaveRepo(r Repo) error {
	if !ValidStub(r.Org) || !ValidStub(r.Stub) {
		return ErrInvalidStub
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org"}, {Name: "stub"}},
		DoUpdates: clause.AssignmentColumns([]string{"url"}),
	}).Create(&r).Error
}

// GetRepo returns the repo with the given stubs, or ErrNotFound.
func (s *Store) GetRepo(org, stub string) (Repo, error) {
	var r Repo
	if err := s.db.First(&r, "org = ? AND stub = ?", org, stub).Error; err != nil {
		return Repo{}, notFound(err)
	}
	return r, nil
}

// ListRepos returns every repo, ordered by org stub and repo stub.
func (s *Store) ListRepos() ([]Repo, error) {
	var repos []Repo
	if err := s.db.Order("org, stub").Find(&repos).Error; err != nil {
		return nil, err
	}
	return repos, nil
}

// SaveTask writes a task, creating it or replacing the stored task with the
// same repo and stub. It returns ErrInvalidStub when the task stub is empty or
// not URL-safe and ErrNotFound when the repo does not exist.
func (s *Store) SaveTask(t Task) error {
	if !ValidStub(t.Stub) {
		return ErrInvalidStub
	}
	if t.RepoID == 0 {
		id, err := s.parentID(&Repo{}, "org = ? AND stub = ?", t.Org, t.Repo)
		if err != nil {
			return err
		}
		t.RepoID = id
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "stub"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "description"}),
	}).Create(&t).Error
}

// GetTask returns the task with the given repo and stub, or ErrNotFound.
func (s *Store) GetTask(org, repo, stub string) (Task, error) {
	var t Task
	err := s.db.Where("repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", org, repo, stub).
		First(&t).Error
	if err != nil {
		return Task{}, notFound(err)
	}
	t.Org, t.Repo = org, repo
	return t, nil
}

// ListTasks returns a repo's tasks, ordered by stub.
func (s *Store) ListTasks(org, repo string) ([]Task, error) {
	repoID, err := s.parentID(&Repo{}, "org = ? AND stub = ?", org, repo)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks []Task
	if err := s.db.Where("repo_id = ?", repoID).Order("stub").Find(&tasks).Error; err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i].Org, tasks[i].Repo = org, repo
	}
	return tasks, nil
}

// parentID returns the id of the parent record that matches the query.
func (s *Store) parentID(parent any, query string, args ...any) (uint, error) {
	var got struct{ ID uint }
	err := s.db.Model(parent).Select("id").Where(query, args...).Take(&got).Error
	if err != nil {
		return 0, notFound(err)
	}
	return got.ID, nil
}

// SaveTest writes a test, creating it or replacing the stored test with the
// same repo, task stub, and test stub. It returns ErrInvalidStub when the test
// stub is empty or not URL-safe, ErrInvalidTest when the test misses a required
// field, ErrInvalidSchema when the score schema breaks its contract, and
// ErrNotFound when the task does not exist.
func (s *Store) SaveTest(t Test) error {
	if !ValidStub(t.Stub) {
		return ErrInvalidStub
	}
	if err := t.Validate(); err != nil {
		return err
	}
	if t.TaskID == 0 {
		id, err := s.parentID(&Task{}, "repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", t.Org, t.Repo, t.Task)
		if err != nil {
			return err
		}
		t.TaskID = id
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "stub"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "description", "version", "schema"}),
	}).Create(&t).Error
}

// GetTest returns the test with the given repo, task stub, and test stub, or
// ErrNotFound.
func (s *Store) GetTest(org, repo, task, stub string) (Test, error) {
	var t Test
	err := s.db.Where("task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
		org, repo, task, stub).First(&t).Error
	if err != nil {
		return Test{}, notFound(err)
	}
	t.Org, t.Repo, t.Task = org, repo, task
	return t, nil
}

// ListTests returns a task's tests, ordered by stub.
func (s *Store) ListTests(org, repo, task string) ([]Test, error) {
	taskID, err := s.parentID(&Task{}, "repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", org, repo, task)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tests []Test
	if err := s.db.Where("task_id = ?", taskID).Order("stub").Find(&tests).Error; err != nil {
		return nil, err
	}
	for i := range tests {
		tests[i].Org, tests[i].Repo, tests[i].Task = org, repo, task
	}
	return tests, nil
}

// SaveModel writes a model, creating it or leaving the stored model with the
// same name, provider, and variant. It returns ErrInvalidModel when the model
// misses its name or provider.
func (s *Store) SaveModel(m Model) error {
	return saveModel(s.db, m)
}

// saveModel writes a model with db, reusing the stored model with the same
// identity.
func saveModel(db *gorm.DB, m Model) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
}

// GetModel returns the model with the given name, provider, and variant, or
// ErrNotFound.
func (s *Store) GetModel(name, provider, variant string) (Model, error) {
	var m Model
	if err := s.db.First(&m, "name = ? AND provider = ? AND variant = ?", name, provider, variant).Error; err != nil {
		return Model{}, notFound(err)
	}
	return m, nil
}

// ListModels returns every model, ordered by provider, name, and variant.
func (s *Store) ListModels() ([]Model, error) {
	var models []Model
	if err := s.db.Order("provider, name, variant").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// SaveAgent writes an agent, creating it or leaving the stored agent with the
// same repo, task, stub, commit, branch, version, and uncommitted flag. It
// returns ErrInvalidAgent when the agent misses its name, stub, commit, or
// branch.
func (s *Store) SaveAgent(a Agent) error {
	return saveAgent(s.db, a)
}

// saveAgent writes an agent with db, reusing the stored agent with the same
// identity.
func saveAgent(db *gorm.DB, a Agent) error {
	if err := a.Validate(); err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error
}

// GetAgent returns the agent with the given identity, or ErrNotFound. Its name
// is not part of its identity.
func (s *Store) GetAgent(a Agent) (Agent, error) {
	var got Agent
	err := s.db.First(&got, "org = ? AND repo = ? AND task = ? AND stub = ? AND commit = ? AND branch = ? AND version = ? AND uncommitted = ?",
		a.Org, a.Repo, a.Task, a.Stub, a.Commit, a.Branch, a.Version, a.Uncommitted).Error
	if err != nil {
		return Agent{}, notFound(err)
	}
	return got, nil
}

// ListAgents returns every agent, ordered by org, repo, task, stub, commit,
// branch, version, and uncommitted flag.
func (s *Store) ListAgents() ([]Agent, error) {
	var agents []Agent
	if err := s.db.Order("org, repo, task, stub, commit, branch, version, uncommitted").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

// SaveGrader writes a grader, creating it or leaving the stored grader with the
// same repo, task, test, stub, commit, branch, version, and uncommitted flag. It
// returns ErrInvalidGrader when the grader misses its name, stub, commit, or
// branch.
func (s *Store) SaveGrader(g Grader) error {
	return saveGrader(s.db, g)
}

// saveGrader writes a grader with db, reusing the stored grader with the same
// identity.
func saveGrader(db *gorm.DB, g Grader) error {
	if err := g.Validate(); err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&g).Error
}

// GetGrader returns the grader with the given identity, or ErrNotFound. Its name
// is not part of its identity.
func (s *Store) GetGrader(g Grader) (Grader, error) {
	var got Grader
	err := s.db.First(&got, "org = ? AND repo = ? AND task = ? AND test = ? AND stub = ? AND commit = ? AND branch = ? AND version = ? AND uncommitted = ?",
		g.Org, g.Repo, g.Task, g.Test, g.Stub, g.Commit, g.Branch, g.Version, g.Uncommitted).Error
	if err != nil {
		return Grader{}, notFound(err)
	}
	return got, nil
}

// ListGraders returns every grader, ordered by org, repo, task, test, stub,
// commit, branch, version, and uncommitted flag.
func (s *Store) ListGraders() ([]Grader, error) {
	var graders []Grader
	if err := s.db.Order("org, repo, task, test, stub, commit, branch, version, uncommitted").Find(&graders).Error; err != nil {
		return nil, err
	}
	return graders, nil
}

// SaveDatum stores the JSON a test passes to an agent and returns the datum's
// ref. The ref is derived from the JSON, so saving the same JSON in the same
// test again returns the same ref and leaves one datum. It stores the JSON
// unchanged.
func (s *Store) SaveDatum(org, repo, task, test string, body []byte) (string, error) {
	testID, err := s.parentID(&Test{}, "task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
		org, repo, task, test)
	if err != nil {
		return "", err
	}
	d := Datum{
		TestID: testID,
		Org:    org, Repo: repo, Task: task, Test: test,
		Ref:  DatumRef(body),
		JSON: string(body),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "test_id"}, {Name: "ref"}},
		DoUpdates: clause.AssignmentColumns([]string{"json"}),
	}).Create(&d).Error; err != nil {
		return "", err
	}
	return d.Ref, nil
}

// GetDatum returns the datum with the given ref in a test, or ErrNotFound.
func (s *Store) GetDatum(org, repo, task, test, ref string) (Datum, error) {
	var d Datum
	err := s.db.Where("test_id = (SELECT id FROM tests WHERE task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?) AND ref = ?",
		org, repo, task, test, ref).First(&d).Error
	if err != nil {
		return Datum{}, notFound(err)
	}
	d.Org, d.Repo, d.Task, d.Test = org, repo, task, test
	return d, nil
}

// ListData returns a test's data, ordered by ref.
func (s *Store) ListData(org, repo, task, test string) ([]Datum, error) {
	testID, err := s.parentID(&Test{}, "task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
		org, repo, task, test)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data []Datum
	if err := s.db.Where("test_id = ?", testID).Order("ref").Find(&data).Error; err != nil {
		return nil, err
	}
	for i := range data {
		data[i].Org, data[i].Repo, data[i].Task, data[i].Test = org, repo, task, test
	}
	return data, nil
}

// SaveResult stores the body an agent produced for a datum and returns the
// result's ref. The ref is derived from the body within its datum and the model
// and agent it carries, so saving the same body from the same model and agent
// again returns the same ref and leaves one result. It creates or reuses the
// model and agent the result carries in the same write, and associates the
// agent with the result's task. It returns ErrInvalidResult when a run metric is
// negative, ErrInvalidModel when the carried model misses its name or provider,
// and ErrInvalidAgent when the carried agent misses its name, stub, commit, or
// branch.
func (s *Store) SaveResult(r Result) (string, error) {
	hasAgent := r.Agent.Present()
	if hasAgent {
		r.Agent.Org, r.Agent.Repo, r.Agent.Task = r.Org, r.Repo, r.Task
	}
	if err := r.Validate(); err != nil {
		return "", err
	}
	r.Ref = ResultRef(r)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if r.Model.Present() {
			if err := saveModel(tx, r.Model); err != nil {
				return err
			}
		}
		if hasAgent {
			if err := saveAgent(tx, r.Agent); err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&r).Error
	})
	if err != nil {
		return "", err
	}
	return r.Ref, nil
}

// GetResult returns the result with the given ref in a repo, or ErrNotFound.
func (s *Store) GetResult(org, repo, ref string) (Result, error) {
	var r Result
	if err := s.db.First(&r, "org = ? AND repo = ? AND ref = ?", org, repo, ref).Error; err != nil {
		return Result{}, notFound(err)
	}
	return r, nil
}

// ListResults returns a repo's results, ordered by ref.
func (s *Store) ListResults(org, repo string) ([]Result, error) {
	var results []Result
	if err := s.db.Where("org = ? AND repo = ?", org, repo).Order("ref").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// SaveScore stores the key-number body a grader produced for a result and
// returns the score's ref. The ref is derived from the body within its result
// and the model and grader it carries, so saving the same body from the same
// model and grader again returns the same ref and leaves one score. It creates
// or reuses the model and grader the score carries in the same write, and
// associates the grader with the result's test. It returns ErrInvalidScore when
// a run metric is negative, ErrInvalidModel when the carried model misses its
// name or provider, and ErrInvalidGrader when the carried grader misses its
// name, stub, commit, or branch.
func (s *Store) SaveScore(sc Score) (string, error) {
	hasGrader := sc.Grader.Present()
	if hasGrader {
		sc.Grader.Org, sc.Grader.Repo, sc.Grader.Task, sc.Grader.Test = sc.Org, sc.Repo, sc.Task, sc.Test
	}
	if err := sc.Validate(); err != nil {
		return "", err
	}
	sc.Ref = ScoreRef(sc)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if sc.Model.Present() {
			if err := saveModel(tx, sc.Model); err != nil {
				return err
			}
		}
		if hasGrader {
			if err := saveGrader(tx, sc.Grader); err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&sc).Error
	})
	if err != nil {
		return "", err
	}
	return sc.Ref, nil
}

// GetScore returns the score with the given ref in a repo, or ErrNotFound.
func (s *Store) GetScore(org, repo, ref string) (Score, error) {
	var sc Score
	if err := s.db.First(&sc, "org = ? AND repo = ? AND ref = ?", org, repo, ref).Error; err != nil {
		return Score{}, notFound(err)
	}
	return sc, nil
}

// ListScores returns a repo's scores, ordered by ref.
func (s *Store) ListScores(org, repo string) ([]Score, error) {
	var scores []Score
	if err := s.db.Where("org = ? AND repo = ?", org, repo).Order("ref").Find(&scores).Error; err != nil {
		return nil, err
	}
	return scores, nil
}

// MoveTask renames the task with the given stub to newStub and returns the
// moved task. It returns ErrNotFound when the task is missing, ErrConflict when
// a task already holds newStub, and ErrInvalidStub when newStub is empty or not
// URL-safe.
func (s *Store) MoveTask(org, repo, stub, newStub string) (Task, error) {
	t, err := s.GetTask(org, repo, stub)
	if err != nil {
		return Task{}, err
	}
	if !ValidStub(newStub) {
		return Task{}, ErrInvalidStub
	}
	if _, err := s.GetTask(org, repo, newStub); err == nil {
		return Task{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return Task{}, err
	}

	err = s.db.Model(&Task{}).
		Where("id = ?", t.ID).
		Update("stub", newStub).Error
	if err != nil {
		return Task{}, err
	}
	t.Stub = newStub
	return t, nil
}

// MoveTest renames the test with the given stubs to newStub and returns the
// moved test. It returns ErrNotFound when the test is missing, ErrConflict when
// a test in the same task already holds newStub, and ErrInvalidStub when newStub
// is empty or not URL-safe.
func (s *Store) MoveTest(org, repo, task, stub, newStub string) (Test, error) {
	t, err := s.GetTest(org, repo, task, stub)
	if err != nil {
		return Test{}, err
	}
	if !ValidStub(newStub) {
		return Test{}, ErrInvalidStub
	}
	if _, err := s.GetTest(org, repo, task, newStub); err == nil {
		return Test{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return Test{}, err
	}

	err = s.db.Model(&Test{}).
		Where("id = ?", t.ID).
		Update("stub", newStub).Error
	if err != nil {
		return Test{}, err
	}
	t.Stub = newStub
	return t, nil
}

// notFound maps Gorm's record-not-found error to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
