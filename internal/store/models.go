package store

// Repo is a repository the service tracks. It is identified by its org stub and
// repo stub, so two repos with the same stubs are the same repo whatever their
// URL.
type Repo struct {
	Org  string `gorm:"primaryKey"`
	Stub string `gorm:"primaryKey"`
	URL  string
}

// Task is a use case within a repo. It is identified by its repo and its stub.
type Task struct {
	Org         string `gorm:"primaryKey"`
	Repo        string `gorm:"primaryKey"`
	Stub        string `gorm:"primaryKey"`
	Name        string
	Description string
}

// Test is a group of data and a score schema within a task. It is identified by
// its repo, task stub, and test stub, so two tests with the same stubs are the
// same test.
type Test struct {
	Org         string `gorm:"primaryKey"`
	Repo        string `gorm:"primaryKey"`
	Task        string `gorm:"primaryKey"`
	Stub        string `gorm:"primaryKey"`
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

// Model is the model a result or score used. It is identified by its name,
// provider, and variant, so two models with the same values are the same
// record. A model may have an empty variant.
type Model struct {
	Name     string `gorm:"primaryKey" json:"name"`
	Provider string `gorm:"primaryKey" json:"provider"`
	Variant  string `gorm:"primaryKey" json:"variant"`
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

// Agent is the external code that produced a result. It is identified by its
// repo, task stub, stub, commit, branch, version, and uncommitted flag, so two
// agents with the same values are the same record. Its name may differ without
// making a new record. A version may be empty, and an omitted uncommitted flag
// records false.
type Agent struct {
	Org         string `gorm:"primaryKey" json:"org"`
	Repo        string `gorm:"primaryKey" json:"repo"`
	Task        string `gorm:"primaryKey" json:"task"`
	Name        string `json:"name"`
	Stub        string `gorm:"primaryKey" json:"stub"`
	Commit      string `gorm:"primaryKey" json:"commit"`
	Branch      string `gorm:"primaryKey" json:"branch"`
	Version     string `gorm:"primaryKey" json:"version"`
	Uncommitted bool   `gorm:"primaryKey" json:"uncommitted"`
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

// Grader is the external code that scored a result. It is identified by its
// repo, task stub, test stub, stub, commit, branch, version, and uncommitted
// flag, so two graders with the same values are the same record. Its name may
// differ without making a new record. A version may be empty, and an omitted
// uncommitted flag records false.
type Grader struct {
	Org         string `gorm:"primaryKey" json:"org"`
	Repo        string `gorm:"primaryKey" json:"repo"`
	Task        string `gorm:"primaryKey" json:"task"`
	Test        string `gorm:"primaryKey" json:"test"`
	Name        string `json:"name"`
	Stub        string `gorm:"primaryKey" json:"stub"`
	Commit      string `gorm:"primaryKey" json:"commit"`
	Branch      string `gorm:"primaryKey" json:"branch"`
	Version     string `gorm:"primaryKey" json:"version"`
	Uncommitted bool   `gorm:"primaryKey" json:"uncommitted"`
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

// Datum is the JSON a test passes to an agent, one per case. It is identified by
// its repo, task stub, test stub, and ref, so two data with the same identity
// are the same datum.
type Datum struct {
	Org  string `gorm:"primaryKey"`
	Repo string `gorm:"primaryKey"`
	Task string `gorm:"primaryKey"`
	Test string `gorm:"primaryKey"`
	Ref  string `gorm:"primaryKey"`
	JSON string
}

// Result is the body an agent produced for a datum, with the logs and run
// metrics that came with it. It is identified by its repo, task stub, test
// stub, datum ref, and ref, so two results with the same identity are the same
// result. Local is the local branch that produced it, empty for a remote run.
type Result struct {
	Org          string `gorm:"primaryKey"`
	Repo         string `gorm:"primaryKey"`
	Task         string `gorm:"primaryKey"`
	Test         string `gorm:"primaryKey"`
	Data         string `gorm:"primaryKey"`
	Ref          string `gorm:"primaryKey"`
	Model        Model  `gorm:"embedded"`
	Agent        Agent  `gorm:"embedded;embeddedPrefix:agent_"`
	Body         string
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
	Local        string
}

// Validate reports whether the result's run metrics are zero or greater and its
// model and agent, when carried, carry their required fields. A result that
// omits a metric records zero. Its body, logs, and local branch may be empty.
func (r Result) Validate() error {
	if r.InputTokens < 0 || r.OutputTokens < 0 || r.TokenCost < 0 || r.Duration < 0 {
		return ErrInvalidResult
	}
	if r.Model.Present() {
		if err := r.Model.Validate(); err != nil {
			return err
		}
	}
	if r.Agent.Present() {
		return r.Agent.Validate()
	}
	return nil
}

// Score is the body a grader produced for a result, with the logs and run
// metrics that came with it. It is identified by its repo, task stub, test
// stub, result ref, and ref, so two scores with the same identity are the same
// score. Body holds a number per key of the result's test score schema.
type Score struct {
	Org          string             `gorm:"primaryKey"`
	Repo         string             `gorm:"primaryKey"`
	Task         string             `gorm:"primaryKey"`
	Test         string             `gorm:"primaryKey"`
	Result       string             `gorm:"primaryKey"`
	Ref          string             `gorm:"primaryKey"`
	Model        Model              `gorm:"embedded"`
	Grader       Grader             `gorm:"embedded;embeddedPrefix:grader_"`
	Body         map[string]float64 `gorm:"serializer:json;type:jsonb"`
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
}

// Validate reports whether the score's run metrics are zero or greater and its
// model and grader, when carried, carry their required fields. A score that
// omits a metric records zero. Its body, logs, and result may be empty.
func (s Score) Validate() error {
	if s.InputTokens < 0 || s.OutputTokens < 0 || s.TokenCost < 0 || s.Duration < 0 {
		return ErrInvalidScore
	}
	if s.Model.Present() {
		if err := s.Model.Validate(); err != nil {
			return err
		}
	}
	if s.Grader.Present() {
		return s.Grader.Validate()
	}
	return nil
}
