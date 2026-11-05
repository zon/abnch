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
// parents. It carries repos, tasks, tests, data, models, agents, graders,
// results, and scores from the schema that identified them by stubs or embedded
// provenance values to the one that links them by id.
func (s *Store) Migrate() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		legacy, err := renameLegacyTables(tx)
		if err != nil {
			return err
		}
		if err := tx.AutoMigrate(&Repo{}, &Task{}, &Test{}, &Model{}, &Agent{}, &Grader{}, &Datum{}, &resultRow{}, &scoreRow{}); err != nil {
			return err
		}
		for _, fk := range []struct {
			table, name, column, parent string
		}{
			{"tasks", "fk_tasks_repo", "repo_id", "repos"},
			{"tests", "fk_tests_task", "task_id", "tasks"},
			{"data", "fk_data_test", "test_id", "tests"},
			{"agents", "fk_agents_task", "task_id", "tasks"},
			{"graders", "fk_graders_test", "test_id", "tests"},
			{"results", "fk_results_datum", "datum_id", "data"},
			{"results", "fk_results_model", "model_id", "models"},
			{"results", "fk_results_agent", "agent_id", "agents"},
			{"scores", "fk_scores_result", "result_id", "results"},
			{"scores", "fk_scores_model", "model_id", "models"},
			{"scores", "fk_scores_grader", "grader_id", "graders"},
		} {
			if err := addForeignKey(tx, fk.table, fk.name, fk.column, fk.parent); err != nil {
				return err
			}
		}
		if err := moveLegacyData(tx, legacy); err != nil {
			return err
		}
		return addIdentityIndexes(tx)
	})
}

// legacyTables records which tables the migration renamed from the stub-keyed
// schema.
type legacyTables struct {
	repos, tasks, tests, data, models, agents, graders, results, scores bool
}

// renameLegacyTables renames the tables that still identify their records by
// stubs, so AutoMigrate creates the id-linked tables. It reports no tables on a
// new database.
func renameLegacyTables(tx *gorm.DB) (legacyTables, error) {
	legacy := legacyTables{
		repos:   tableWithoutID(tx, &Repo{}),
		tasks:   tableWithoutID(tx, &Task{}),
		tests:   tableWithoutID(tx, &Test{}),
		data:    tableWithoutID(tx, &Datum{}),
		models:  tableWithoutID(tx, &Model{}),
		agents:  tableWithoutID(tx, &Agent{}),
		graders: tableWithoutID(tx, &Grader{}),
		results: tableWithoutID(tx, &resultRow{}),
		scores:  tableWithoutID(tx, &scoreRow{}),
	}
	for _, table := range []string{"data", "tests", "tasks", "repos", "graders", "agents", "models", "scores", "results"} {
		if !hasLegacyTable(legacy, table) {
			continue
		}
		if err := tx.Exec(`ALTER TABLE "` + table + `" RENAME TO "` + table + `_legacy"`).Error; err != nil {
			return legacy, err
		}
	}
	return legacy, nil
}

// tableWithoutID reports whether the model's table exists without the id column
// the id-linked schema adds.
func tableWithoutID(tx *gorm.DB, model any) bool {
	return tx.Migrator().HasTable(model) && !tx.Migrator().HasColumn(model, "id")
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
	case "models":
		return legacy.models
	case "agents":
		return legacy.agents
	case "graders":
		return legacy.graders
	case "results":
		return legacy.results
	case "scores":
		return legacy.scores
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
		{legacy.models, `INSERT INTO "models" ("name", "provider", "variant")
			SELECT "name", "provider", "variant" FROM "models_legacy"`},
		{legacy.agents, `INSERT INTO "agents" ("task_id", "name", "stub", "commit", "branch", "version", "uncommitted")
			SELECT t."id", a."name", a."stub", a."commit", a."branch", a."version", a."uncommitted"
			FROM "agents_legacy" a
			JOIN "repos" r ON r."org" = a."org" AND r."stub" = a."repo"
			JOIN "tasks" t ON t."repo_id" = r."id" AND t."stub" = a."task"`},
		{legacy.graders, `INSERT INTO "graders" ("test_id", "name", "stub", "commit", "branch", "version", "uncommitted")
			SELECT s."id", g."name", g."stub", g."commit", g."branch", g."version", g."uncommitted"
			FROM "graders_legacy" g
			JOIN "repos" r ON r."org" = g."org" AND r."stub" = g."repo"
			JOIN "tasks" t ON t."repo_id" = r."id" AND t."stub" = g."task"
			JOIN "tests" s ON s."task_id" = t."id" AND s."stub" = g."test"`},
		{legacy.results, `INSERT INTO "results" ("datum_id", "model_id", "agent_id", "ref", "body", "logs", "input_tokens", "output_tokens", "token_cost", "duration", "local")
			SELECT d."id", m."id", a."id", r."ref", r."body", r."logs", r."input_tokens", r."output_tokens", r."token_cost", r."duration", r."local"
			FROM "results_legacy" r
			JOIN "repos" p ON p."org" = r."org" AND p."stub" = r."repo"
			JOIN "tasks" t ON t."repo_id" = p."id" AND t."stub" = r."task"
			JOIN "tests" s ON s."task_id" = t."id" AND s."stub" = r."test"
			JOIN "data" d ON d."test_id" = s."id" AND d."ref" = r."data"
			JOIN "models" m ON m."name" = r."name" AND m."provider" = r."provider" AND m."variant" = r."variant"
			JOIN "agents" a ON a."task_id" = t."id" AND a."stub" = r."agent_stub" AND a."commit" = r."agent_commit"
				AND a."branch" = r."agent_branch" AND a."version" = r."agent_version" AND a."uncommitted" = r."agent_uncommitted"`},
		{legacy.scores, `INSERT INTO "scores" ("result_id", "model_id", "grader_id", "ref", "body", "logs", "input_tokens", "output_tokens", "token_cost", "duration")
			SELECT x."id", m."id", g."id", c."ref", c."body", c."logs", c."input_tokens", c."output_tokens", c."token_cost", c."duration"
			FROM "scores_legacy" c
			JOIN "results" x ON x."ref" = c."result"
			JOIN "data" d ON d."id" = x."datum_id"
			JOIN "tests" s ON s."id" = d."test_id" AND s."stub" = c."test"
			JOIN "tasks" t ON t."id" = s."task_id" AND t."stub" = c."task"
			JOIN "repos" p ON p."id" = t."repo_id" AND p."org" = c."org" AND p."stub" = c."repo"
			JOIN "models" m ON m."name" = c."name" AND m."provider" = c."provider" AND m."variant" = c."variant"
			JOIN "graders" g ON g."test_id" = s."id" AND g."stub" = c."grader_stub" AND g."commit" = c."grader_commit"
				AND g."branch" = c."grader_branch" AND g."version" = c."grader_version" AND g."uncommitted" = c."grader_uncommitted"`},
		{legacy.data, `DROP TABLE "data_legacy"`},
		{legacy.tests, `DROP TABLE "tests_legacy"`},
		{legacy.tasks, `DROP TABLE "tasks_legacy"`},
		{legacy.repos, `DROP TABLE "repos_legacy"`},
		{legacy.graders, `DROP TABLE "graders_legacy"`},
		{legacy.agents, `DROP TABLE "agents_legacy"`},
		{legacy.models, `DROP TABLE "models_legacy"`},
		{legacy.scores, `DROP TABLE "scores_legacy"`},
		{legacy.results, `DROP TABLE "results_legacy"`},
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

// addIdentityIndexes adds the unique identities of models, agents, graders,
// results, and scores and makes each agent's task and grader's test required.
func addIdentityIndexes(tx *gorm.DB) error {
	for _, stmt := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_models_identity ON "models" ("name", "provider", "variant")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_identity ON "agents" ("task_id", "stub", "commit", "branch", "version", "uncommitted")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_graders_identity ON "graders" ("test_id", "stub", "commit", "branch", "version", "uncommitted")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_results_identity ON "results" ("datum_id", "model_id", "agent_id", "ref")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_scores_identity ON "scores" ("result_id", "model_id", "grader_id", "ref")`,
		`ALTER TABLE "agents" ALTER COLUMN "task_id" SET NOT NULL`,
		`ALTER TABLE "graders" ALTER COLUMN "test_id" SET NOT NULL`,
	} {
		if err := tx.Exec(stmt).Error; err != nil {
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
		id, err := parentID(s.db, &Repo{}, "org = ? AND stub = ?", t.Org, t.Repo)
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
	repoID, err := parentID(s.db, &Repo{}, "org = ? AND stub = ?", org, repo)
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
func parentID(db *gorm.DB, parent any, query string, args ...any) (uint, error) {
	var got struct{ ID uint }
	err := db.Model(parent).Select("id").Where(query, args...).Take(&got).Error
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
		id, err := parentID(s.db, &Task{}, "repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", t.Org, t.Repo, t.Task)
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
	taskID, err := parentID(s.db, &Task{}, "repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", org, repo, task)
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
	_, err := saveModel(s.db, m)
	return err
}

// saveModel writes a model with db, reusing and returning the stored model with
// the same identity.
func saveModel(db *gorm.DB, m Model) (Model, error) {
	if err := m.Validate(); err != nil {
		return Model{}, err
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error; err != nil {
		return Model{}, err
	}
	if m.ID != 0 {
		return m, nil
	}
	var got Model
	if err := db.First(&got, "name = ? AND provider = ? AND variant = ?", m.Name, m.Provider, m.Variant).Error; err != nil {
		return Model{}, err
	}
	return got, nil
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
// same task, stub, commit, branch, version, and uncommitted flag. It returns
// ErrInvalidAgent when the agent misses its name, stub, commit, or branch and
// ErrNotFound when its task does not exist.
func (s *Store) SaveAgent(a Agent) error {
	_, err := saveAgent(s.db, a)
	return err
}

// saveAgent writes an agent with db, reusing and returning the stored agent with
// the same identity.
func saveAgent(db *gorm.DB, a Agent) (Agent, error) {
	if err := a.Validate(); err != nil {
		return Agent{}, err
	}
	if a.TaskID == 0 {
		id, err := parentID(db, &Task{}, "repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", a.Org, a.Repo, a.Task)
		if err != nil {
			return Agent{}, err
		}
		a.TaskID = id
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "task_id"}, {Name: "stub"}, {Name: "commit"},
			{Name: "branch"}, {Name: "version"}, {Name: "uncommitted"},
		},
		DoNothing: true,
	}).Create(&a).Error; err != nil {
		return Agent{}, err
	}
	if a.ID == 0 {
		var got Agent
		if err := db.First(&got, "task_id = ? AND stub = ? AND commit = ? AND branch = ? AND version = ? AND uncommitted = ?",
			a.TaskID, a.Stub, a.Commit, a.Branch, a.Version, a.Uncommitted).Error; err != nil {
			return Agent{}, err
		}
		a = got
	}
	if err := fillAgentParent(db, &a); err != nil {
		return Agent{}, err
	}
	return a, nil
}

// GetAgent returns the agent with the given identity, or ErrNotFound. Its name
// is not part of its identity.
func (s *Store) GetAgent(a Agent) (Agent, error) {
	taskID := a.TaskID
	if taskID == 0 {
		id, err := parentID(s.db, &Task{}, "repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?", a.Org, a.Repo, a.Task)
		if err != nil {
			return Agent{}, err
		}
		taskID = id
	}
	var got Agent
	err := s.db.First(&got, "task_id = ? AND stub = ? AND commit = ? AND branch = ? AND version = ? AND uncommitted = ?",
		taskID, a.Stub, a.Commit, a.Branch, a.Version, a.Uncommitted).Error
	if err != nil {
		return Agent{}, notFound(err)
	}
	if err := fillAgentParent(s.db, &got); err != nil {
		return Agent{}, err
	}
	return got, nil
}

// ListAgents returns every agent, ordered by org, repo, task, stub, commit,
// branch, version, and uncommitted flag.
func (s *Store) ListAgents() ([]Agent, error) {
	var agents []Agent
	err := s.db.Table(`"agents"`).
		Select(`"agents".*`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "agents"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Order(`"repos"."org", "repos"."stub", "tasks"."stub", "agents"."stub", "agents"."commit", "agents"."branch", "agents"."version", "agents"."uncommitted"`).
		Find(&agents).Error
	if err != nil {
		return nil, err
	}
	for i := range agents {
		if err := fillAgentParent(s.db, &agents[i]); err != nil {
			return nil, err
		}
	}
	return agents, nil
}

// fillAgentParent loads an agent's parent stubs from its task.
func fillAgentParent(db *gorm.DB, a *Agent) error {
	var parent struct{ Org, Repo, Task string }
	err := db.Table(`"tasks"`).
		Select(`"repos"."org" AS "org", "repos"."stub" AS "repo", "tasks"."stub" AS "task"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"tasks"."id" = ?`, a.TaskID).
		Take(&parent).Error
	if err != nil {
		return notFound(err)
	}
	a.Org, a.Repo, a.Task = parent.Org, parent.Repo, parent.Task
	return nil
}

// SaveGrader writes a grader, creating it or leaving the stored grader with the
// same test, stub, commit, branch, version, and uncommitted flag. It returns
// ErrInvalidGrader when the grader misses its name, stub, commit, or branch and
// ErrNotFound when its test does not exist.
func (s *Store) SaveGrader(g Grader) error {
	_, err := saveGrader(s.db, g)
	return err
}

// saveGrader writes a grader with db, reusing and returning the stored grader
// with the same identity.
func saveGrader(db *gorm.DB, g Grader) (Grader, error) {
	if err := g.Validate(); err != nil {
		return Grader{}, err
	}
	if g.TestID == 0 {
		id, err := parentID(db, &Test{}, "task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
			g.Org, g.Repo, g.Task, g.Test)
		if err != nil {
			return Grader{}, err
		}
		g.TestID = id
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "test_id"}, {Name: "stub"}, {Name: "commit"},
			{Name: "branch"}, {Name: "version"}, {Name: "uncommitted"},
		},
		DoNothing: true,
	}).Create(&g).Error; err != nil {
		return Grader{}, err
	}
	if g.ID == 0 {
		var got Grader
		if err := db.First(&got, "test_id = ? AND stub = ? AND commit = ? AND branch = ? AND version = ? AND uncommitted = ?",
			g.TestID, g.Stub, g.Commit, g.Branch, g.Version, g.Uncommitted).Error; err != nil {
			return Grader{}, err
		}
		g = got
	}
	if err := fillGraderParent(db, &g); err != nil {
		return Grader{}, err
	}
	return g, nil
}

// GetGrader returns the grader with the given identity, or ErrNotFound. Its name
// is not part of its identity.
func (s *Store) GetGrader(g Grader) (Grader, error) {
	testID := g.TestID
	if testID == 0 {
		id, err := parentID(s.db, &Test{}, "task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
			g.Org, g.Repo, g.Task, g.Test)
		if err != nil {
			return Grader{}, err
		}
		testID = id
	}
	var got Grader
	err := s.db.First(&got, "test_id = ? AND stub = ? AND commit = ? AND branch = ? AND version = ? AND uncommitted = ?",
		testID, g.Stub, g.Commit, g.Branch, g.Version, g.Uncommitted).Error
	if err != nil {
		return Grader{}, notFound(err)
	}
	if err := fillGraderParent(s.db, &got); err != nil {
		return Grader{}, err
	}
	return got, nil
}

// ListGraders returns every grader, ordered by org, repo, task, test, stub,
// commit, branch, version, and uncommitted flag.
func (s *Store) ListGraders() ([]Grader, error) {
	var graders []Grader
	err := s.db.Table(`"graders"`).
		Select(`"graders".*`).
		Joins(`JOIN "tests" ON "tests"."id" = "graders"."test_id"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Order(`"repos"."org", "repos"."stub", "tasks"."stub", "tests"."stub", "graders"."stub", "graders"."commit", "graders"."branch", "graders"."version", "graders"."uncommitted"`).
		Find(&graders).Error
	if err != nil {
		return nil, err
	}
	for i := range graders {
		if err := fillGraderParent(s.db, &graders[i]); err != nil {
			return nil, err
		}
	}
	return graders, nil
}

// fillGraderParent loads a grader's parent stubs from its test.
func fillGraderParent(db *gorm.DB, g *Grader) error {
	var parent struct{ Org, Repo, Task, Test string }
	err := db.Table(`"tests"`).
		Select(`"repos"."org" AS "org", "repos"."stub" AS "repo", "tasks"."stub" AS "task", "tests"."stub" AS "test"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"tests"."id" = ?`, g.TestID).
		Take(&parent).Error
	if err != nil {
		return notFound(err)
	}
	g.Org, g.Repo, g.Task, g.Test = parent.Org, parent.Repo, parent.Task, parent.Test
	return nil
}

// SaveDatum stores the JSON a test passes to an agent and returns the datum's
// ref. The ref is derived from the JSON, so saving the same JSON in the same
// test again returns the same ref and leaves one datum. It stores the JSON
// unchanged.
func (s *Store) SaveDatum(org, repo, task, test string, body []byte) (string, error) {
	testID, err := parentID(s.db, &Test{}, "task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
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
	testID, err := parentID(s.db, &Test{}, "task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?",
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
// negative, ErrInvalidModel when the result omits its model or the model misses
// its name or provider, ErrInvalidAgent when the result omits its agent or the
// agent misses its name, stub, commit, or branch, and ErrNotFound when its datum
// does not exist.
func (s *Store) SaveResult(r Result) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		datumID, err := parentID(tx, &Datum{}, "test_id = (SELECT id FROM tests WHERE task_id = (SELECT id FROM tasks WHERE repo_id = (SELECT id FROM repos WHERE org = ? AND stub = ?) AND stub = ?) AND stub = ?) AND ref = ?",
			r.Org, r.Repo, r.Task, r.Test, r.Data)
		if err != nil {
			return err
		}
		r.DatumID = datumID

		model, err := saveModel(tx, r.Model)
		if err != nil {
			return err
		}
		r.ModelID = model.ID

		r.Agent.Org, r.Agent.Repo, r.Agent.Task = r.Org, r.Repo, r.Task
		agent, err := saveAgent(tx, r.Agent)
		if err != nil {
			return err
		}
		r.AgentID = agent.ID

		r.Ref = ResultRef(r)
		row := r.row()
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "datum_id"}, {Name: "model_id"}, {Name: "agent_id"}, {Name: "ref"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"body", "logs", "input_tokens", "output_tokens", "token_cost", "duration", "local",
			}),
		}).Create(&row).Error
	})
	if err != nil {
		return "", err
	}
	return r.Ref, nil
}

// GetResult returns the result with the given ref in a repo, or ErrNotFound.
func (s *Store) GetResult(org, repo, ref string) (Result, error) {
	row, err := resultRowFor(s.db, org, repo, ref)
	if err != nil {
		return Result{}, err
	}
	return loadResult(s.db, row)
}

// ListResults returns a repo's results, ordered by ref.
func (s *Store) ListResults(org, repo string) ([]Result, error) {
	var rows []resultRow
	err := s.db.Table(`"results"`).
		Select(`"results".*`).
		Joins(`JOIN "data" ON "data"."id" = "results"."datum_id"`).
		Joins(`JOIN "tests" ON "tests"."id" = "data"."test_id"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"repos"."org" = ? AND "repos"."stub" = ?`, org, repo).
		Order(`"results"."ref"`).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	results := make([]Result, len(rows))
	for i, row := range rows {
		results[i], err = loadResult(s.db, row)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// resultRowFor returns the result row with the given ref in a repo.
func resultRowFor(db *gorm.DB, org, repo, ref string) (resultRow, error) {
	var row resultRow
	err := db.Table(`"results"`).
		Select(`"results".*`).
		Joins(`JOIN "data" ON "data"."id" = "results"."datum_id"`).
		Joins(`JOIN "tests" ON "tests"."id" = "data"."test_id"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"repos"."org" = ? AND "repos"."stub" = ? AND "results"."ref" = ?`, org, repo, ref).
		Take(&row).Error
	if err != nil {
		return resultRow{}, notFound(err)
	}
	return row, nil
}

// loadResult returns the result a row stores with its linked records and parent
// stubs.
func loadResult(db *gorm.DB, row resultRow) (Result, error) {
	r := row.result()
	loc, err := datumLocation(db, row.DatumID)
	if err != nil {
		return Result{}, err
	}
	r.Org, r.Repo, r.Task, r.Test, r.Data = loc.Org, loc.Repo, loc.Task, loc.Test, loc.Data
	if err := db.First(&r.Model, "id = ?", row.ModelID).Error; err != nil {
		return Result{}, notFound(err)
	}
	if err := db.First(&r.Agent, "id = ?", row.AgentID).Error; err != nil {
		return Result{}, notFound(err)
	}
	r.Agent.Org, r.Agent.Repo, r.Agent.Task = r.Org, r.Repo, r.Task
	return r, nil
}

// SaveScore stores the key-number body a grader produced for a result and
// returns the score's ref. The ref is derived from the body within its result
// and the model and grader it carries, so saving the same body from the same
// model and grader again returns the same ref and leaves one score. It creates
// or reuses the model and grader the score carries in the same write, and
// associates the grader with the result's test. It returns ErrInvalidScore when
// a run metric is negative, ErrInvalidModel when the score omits its model or
// the model misses its name or provider, ErrInvalidGrader when the score omits
// its grader or the grader misses its name, stub, commit, or branch, and
// ErrNotFound when its result does not exist.
func (s *Store) SaveScore(sc Score) (string, error) {
	if err := sc.Validate(); err != nil {
		return "", err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		result, err := resultRowFor(tx, sc.Org, sc.Repo, sc.Result)
		if err != nil {
			return err
		}
		loc, err := datumLocation(tx, result.DatumID)
		if err != nil {
			return err
		}
		sc.Org, sc.Repo, sc.Task, sc.Test = loc.Org, loc.Repo, loc.Task, loc.Test
		sc.Result, sc.ResultID = result.Ref, result.ID

		model, err := saveModel(tx, sc.Model)
		if err != nil {
			return err
		}
		sc.ModelID = model.ID

		sc.Grader.Org, sc.Grader.Repo, sc.Grader.Task, sc.Grader.Test = sc.Org, sc.Repo, sc.Task, sc.Test
		grader, err := saveGrader(tx, sc.Grader)
		if err != nil {
			return err
		}
		sc.GraderID = grader.ID

		sc.Ref = ScoreRef(sc)
		row := sc.row()
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "result_id"}, {Name: "model_id"}, {Name: "grader_id"}, {Name: "ref"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"body", "logs", "input_tokens", "output_tokens", "token_cost", "duration",
			}),
		}).Create(&row).Error
	})
	if err != nil {
		return "", err
	}
	return sc.Ref, nil
}

// GetScore returns the score with the given ref in a repo, or ErrNotFound.
func (s *Store) GetScore(org, repo, ref string) (Score, error) {
	row, err := scoreRowFor(s.db, org, repo, ref)
	if err != nil {
		return Score{}, err
	}
	return loadScore(s.db, row)
}

// ListScores returns a repo's scores, ordered by ref.
func (s *Store) ListScores(org, repo string) ([]Score, error) {
	var rows []scoreRow
	err := s.db.Table(`"scores"`).
		Select(`"scores".*`).
		Joins(`JOIN "results" ON "results"."id" = "scores"."result_id"`).
		Joins(`JOIN "data" ON "data"."id" = "results"."datum_id"`).
		Joins(`JOIN "tests" ON "tests"."id" = "data"."test_id"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"repos"."org" = ? AND "repos"."stub" = ?`, org, repo).
		Order(`"scores"."ref"`).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	scores := make([]Score, len(rows))
	for i, row := range rows {
		scores[i], err = loadScore(s.db, row)
		if err != nil {
			return nil, err
		}
	}
	return scores, nil
}

// scoreRowFor returns the score row with the given ref in a repo.
func scoreRowFor(db *gorm.DB, org, repo, ref string) (scoreRow, error) {
	var row scoreRow
	err := db.Table(`"scores"`).
		Select(`"scores".*`).
		Joins(`JOIN "results" ON "results"."id" = "scores"."result_id"`).
		Joins(`JOIN "data" ON "data"."id" = "results"."datum_id"`).
		Joins(`JOIN "tests" ON "tests"."id" = "data"."test_id"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"repos"."org" = ? AND "repos"."stub" = ? AND "scores"."ref" = ?`, org, repo, ref).
		Take(&row).Error
	if err != nil {
		return scoreRow{}, notFound(err)
	}
	return row, nil
}

// loadScore returns the score a row stores with its linked records and parent
// stubs.
func loadScore(db *gorm.DB, row scoreRow) (Score, error) {
	sc := row.score()
	var result resultRow
	if err := db.First(&result, "id = ?", row.ResultID).Error; err != nil {
		return Score{}, notFound(err)
	}
	loc, err := datumLocation(db, result.DatumID)
	if err != nil {
		return Score{}, err
	}
	sc.Org, sc.Repo, sc.Task, sc.Test, sc.Result = loc.Org, loc.Repo, loc.Task, loc.Test, result.Ref
	if err := db.First(&sc.Model, "id = ?", row.ModelID).Error; err != nil {
		return Score{}, notFound(err)
	}
	if err := db.First(&sc.Grader, "id = ?", row.GraderID).Error; err != nil {
		return Score{}, notFound(err)
	}
	sc.Grader.Org, sc.Grader.Repo, sc.Grader.Task, sc.Grader.Test = sc.Org, sc.Repo, sc.Task, sc.Test
	return sc, nil
}

// datumParents is the parent chain a datum belongs to.
type datumParents struct {
	Org, Repo, Task, Test, Data string
}

// datumLocation returns the parent stubs of the datum with the given id.
func datumLocation(db *gorm.DB, id uint) (datumParents, error) {
	var loc datumParents
	err := db.Table(`"data"`).
		Select(`"repos"."org" AS "org", "repos"."stub" AS "repo", "tasks"."stub" AS "task", "tests"."stub" AS "test", "data"."ref" AS "data"`).
		Joins(`JOIN "tests" ON "tests"."id" = "data"."test_id"`).
		Joins(`JOIN "tasks" ON "tasks"."id" = "tests"."task_id"`).
		Joins(`JOIN "repos" ON "repos"."id" = "tasks"."repo_id"`).
		Where(`"data"."id" = ?`, id).
		Take(&loc).Error
	if err != nil {
		return datumParents{}, notFound(err)
	}
	return loc, nil
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
