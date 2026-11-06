package store

// Repo is a repository the service tracks. It is identified by its org stub and
// repo stub, so two repos with the same stubs are the same repo whatever their
// URL. The service assigns its id, which never changes.
type Repo struct {
	ID   uint   `gorm:"primaryKey" json:"-"`
	Org  string `gorm:"uniqueIndex:idx_repos_org_stub;not null"`
	Stub string `gorm:"uniqueIndex:idx_repos_org_stub;not null"`
	URL  string
}

// Task is a use case within a repo. It links to its repo by id and is unique by
// that id and its stub. The service assigns its id, which never changes. Org and
// Repo carry the parent stubs for callers; the store does not persist them.
type Task struct {
	ID          uint   `gorm:"primaryKey" json:"-"`
	RepoID      uint   `gorm:"not null;uniqueIndex:idx_tasks_repo_stub"`
	Org         string `gorm:"-"`
	Repo        string `gorm:"-"`
	Stub        string `gorm:"not null;uniqueIndex:idx_tasks_repo_stub"`
	Name        string
	Description string
}

// Test is a group of data and a score schema within a task. It links to its task
// by id and is unique by that id and its stub, so two tests with the same stubs
// are the same test. The service assigns its id, which never changes. Org, Repo,
// and Task carry the parent stubs for callers; the store does not persist them.
type Test struct {
	ID          uint   `gorm:"primaryKey" json:"-"`
	TaskID      uint   `gorm:"not null;uniqueIndex:idx_tests_task_stub"`
	Org         string `gorm:"-"`
	Repo        string `gorm:"-"`
	Task        string `gorm:"-"`
	Stub        string `gorm:"not null;uniqueIndex:idx_tests_task_stub"`
	Name        string
	Description string
	Version     string
	Schema      []SchemaKey `gorm:"serializer:json;type:jsonb"`
}

// Validate reports whether the test carries the fields it needs: a non-empty
// name and version, and a score schema whose key stubs are unique. An empty
// score schema is allowed.
func (t Test) Validate() error {
	if t.Name == "" || t.Version == "" {
		return ErrInvalidTest
	}
	if !uniqueSchemaStubs(t.Schema) {
		return ErrInvalidSchema
	}
	return nil
}

// uniqueSchemaStubs reports whether every schema key holds a distinct stub.
func uniqueSchemaStubs(schema []SchemaKey) bool {
	seen := make(map[string]struct{}, len(schema))
	for _, key := range schema {
		if _, ok := seen[key.Stub]; ok {
			return false
		}
		seen[key.Stub] = struct{}{}
	}
	return true
}

// AllowsScore reports whether every key in the score body is a key in the
// test's score schema. A body may omit a schema key, and an empty body is
// allowed.
func (t Test) AllowsScore(body map[string]float64) bool {
	keys := make(map[string]struct{}, len(t.Schema))
	for _, key := range t.Schema {
		keys[key.Stub] = struct{}{}
	}
	for stub := range body {
		if _, ok := keys[stub]; !ok {
			return false
		}
	}
	return true
}

// SchemaKey is one key of a test's score schema. Its stub is the key stored in
// scores and its name is what the UI displays.
type SchemaKey struct {
	Stub string `json:"stub"`
	Name string `json:"name"`
}

// Model is the model a result or score used. It is unique by its name,
// provider, and variant, so two models with the same values are the same
// record. The service assigns its id, which never changes. A model may have an
// empty variant.
type Model struct {
	ID       uint   `gorm:"primaryKey" json:"-"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Variant  string `json:"variant"`
}

// Validate reports whether the model carries the fields it needs: a non-empty
// name and provider. An empty variant is allowed.
func (m Model) Validate() error {
	if m.Name == "" || m.Provider == "" {
		return ErrInvalidModel
	}
	return nil
}

// Present reports whether the model carries any value. An absent model is the
// zero value, so a result or score that omits its model creates no record.
func (m Model) Present() bool {
	return m != Model{}
}

// Agent is the external code that produced a result. It links to its task by
// id and is unique by that id, its stub, commit, branch, version, and
// uncommitted flag, so two agents with the same values are the same record. The
// service assigns its id, which never changes. Its name may differ without
// making a new record. Org, Repo, and Task carry the parent stubs for callers;
// the store does not persist them. A version may be empty, and an omitted
// uncommitted flag records false.
type Agent struct {
	ID          uint   `gorm:"primaryKey" json:"-"`
	TaskID      uint   `gorm:"not null" json:"-"`
	Org         string `gorm:"-" json:"org"`
	Repo        string `gorm:"-" json:"repo"`
	Task        string `gorm:"-" json:"task"`
	Name        string `json:"name"`
	Stub        string `json:"stub"`
	Commit      string `json:"commit"`
	Branch      string `json:"branch"`
	Version     string `json:"version"`
	Uncommitted bool   `json:"uncommitted"`
}

// Validate reports whether the agent carries the fields it needs: a non-empty
// name, stub, commit, and branch. An empty version or a false uncommitted flag
// is allowed.
func (a Agent) Validate() error {
	if a.Name == "" || a.Stub == "" || a.Commit == "" || a.Branch == "" {
		return ErrInvalidAgent
	}
	return nil
}

// Present reports whether the agent carries any value. An absent agent is the
// zero value, so a result that omits its agent creates no record.
func (a Agent) Present() bool {
	return a != Agent{}
}

// Grader is the external code that scored a result. It links to its test by id
// and is unique by that id, its stub, commit, branch, version, and uncommitted
// flag, so two graders with the same values are the same record. The service
// assigns its id, which never changes. Its name may differ without making a new
// record. Org, Repo, Task, and Test carry the parent stubs for callers; the
// store does not persist them. A version may be empty, and an omitted
// uncommitted flag records false.
type Grader struct {
	ID          uint   `gorm:"primaryKey" json:"-"`
	TestID      uint   `gorm:"not null" json:"-"`
	Org         string `gorm:"-" json:"org"`
	Repo        string `gorm:"-" json:"repo"`
	Task        string `gorm:"-" json:"task"`
	Test        string `gorm:"-" json:"test"`
	Name        string `json:"name"`
	Stub        string `json:"stub"`
	Commit      string `json:"commit"`
	Branch      string `json:"branch"`
	Version     string `json:"version"`
	Uncommitted bool   `json:"uncommitted"`
}

// Validate reports whether the grader carries the fields it needs: a non-empty
// name, stub, commit, and branch. An empty version or a false uncommitted flag
// is allowed.
func (g Grader) Validate() error {
	if g.Name == "" || g.Stub == "" || g.Commit == "" || g.Branch == "" {
		return ErrInvalidGrader
	}
	return nil
}

// Present reports whether the grader carries any value. An absent grader is the
// zero value, so a score that omits its grader creates no record.
func (g Grader) Present() bool {
	return g != Grader{}
}

// Datum is the JSON a test passes to an agent, one per case. It links to its
// test by id and is unique by that id and its ref, so two data with the same
// identity are the same datum. The service assigns its id, which never changes.
// Org, Repo, Task, and Test carry the parent stubs for callers; the store does
// not persist them.
type Datum struct {
	ID     uint   `gorm:"primaryKey" json:"-"`
	TestID uint   `gorm:"not null;uniqueIndex:idx_data_test_ref"`
	Org    string `gorm:"-"`
	Repo   string `gorm:"-"`
	Task   string `gorm:"-"`
	Test   string `gorm:"-"`
	Ref    string `gorm:"not null;uniqueIndex:idx_data_test_ref"`
	JSON   string
}

// Result is the body an agent produced for a datum, with the logs and run
// metrics that came with it. It links to its datum, model, and agent by id and
// is unique by those links and its ref. The service assigns its id, which never
// changes. Org, Repo, Task, Test, and Data carry the linked records' stubs for
// callers; the store does not persist them. Local is the local branch that
// produced it, empty for a remote run.
type Result struct {
	ID           uint   `gorm:"primaryKey" json:"-"`
	DatumID      uint   `gorm:"not null;uniqueIndex:idx_results_identity" json:"-"`
	ModelID      uint   `gorm:"not null;uniqueIndex:idx_results_identity" json:"-"`
	AgentID      uint   `gorm:"not null;uniqueIndex:idx_results_identity" json:"-"`
	Org          string `gorm:"-"`
	Repo         string `gorm:"-"`
	Task         string `gorm:"-"`
	Test         string `gorm:"-"`
	Data         string `gorm:"-"`
	Ref          string `gorm:"not null;uniqueIndex:idx_results_identity"`
	Model        Model  `gorm:"-"`
	Agent        Agent  `gorm:"-"`
	Body         string
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
	Local        string
}

// Validate reports whether the result's run metrics are zero or greater and its
// model and agent are present and carry their required fields. A result that
// omits a metric records zero. Its body, logs, and local branch may be empty.
func (r Result) Validate() error {
	if r.InputTokens < 0 || r.OutputTokens < 0 || r.TokenCost < 0 || r.Duration < 0 {
		return ErrInvalidResult
	}
	if !r.Model.Present() {
		return ErrInvalidModel
	}
	if err := r.Model.Validate(); err != nil {
		return err
	}
	if !r.Agent.Present() {
		return ErrInvalidAgent
	}
	return r.Agent.Validate()
}

// Score is the body a grader produced for a result, with the logs and run
// metrics that came with it. It links to its result, model, and grader by id and
// is unique by those links and its ref. The service assigns its id, which never
// changes. Org, Repo, Task, Test, and Result carry the linked records' stubs for
// callers; the store does not persist them. Body holds a number per key of the
// result's test score schema.
type Score struct {
	ID           uint               `gorm:"primaryKey" json:"-"`
	ResultID     uint               `gorm:"not null;uniqueIndex:idx_scores_identity" json:"-"`
	ModelID      uint               `gorm:"not null;uniqueIndex:idx_scores_identity" json:"-"`
	GraderID     uint               `gorm:"not null;uniqueIndex:idx_scores_identity" json:"-"`
	Org          string             `gorm:"-"`
	Repo         string             `gorm:"-"`
	Task         string             `gorm:"-"`
	Test         string             `gorm:"-"`
	Result       string             `gorm:"-"`
	Ref          string             `gorm:"not null;uniqueIndex:idx_scores_identity"`
	Model        Model              `gorm:"-"`
	Grader       Grader             `gorm:"-"`
	Body         map[string]float64 `gorm:"serializer:json;type:jsonb"`
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
}

// Validate reports whether the score's run metrics are zero or greater and its
// model and grader are present and carry their required fields. A score that
// omits a metric records zero. Its body, logs, and result may be empty.
func (s Score) Validate() error {
	if s.InputTokens < 0 || s.OutputTokens < 0 || s.TokenCost < 0 || s.Duration < 0 {
		return ErrInvalidScore
	}
	if !s.Model.Present() {
		return ErrInvalidModel
	}
	if err := s.Model.Validate(); err != nil {
		return err
	}
	if !s.Grader.Present() {
		return ErrInvalidGrader
	}
	return s.Grader.Validate()
}
