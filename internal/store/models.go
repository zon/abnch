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

// AllowsScore reports whether every key in the dictionary is a key in the
// test's score schema. A dictionary may omit a schema key, and an empty
// dictionary is allowed.
func (t Test) AllowsScore(dictionary map[string]float64) bool {
	keys := make(map[string]struct{}, len(t.Schema))
	for _, key := range t.Schema {
		keys[key.Stub] = struct{}{}
	}
	for stub := range dictionary {
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

// Result is the JSON an agent produced for a datum, with the logs and run
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
	JSON         string
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
	Local        string
}

// Validate reports whether the result's run metrics are zero or greater. A
// result that omits a metric records zero. Its JSON, logs, and local branch may
// be empty.
func (r Result) Validate() error {
	if r.InputTokens < 0 || r.OutputTokens < 0 || r.TokenCost < 0 || r.Duration < 0 {
		return ErrInvalidResult
	}
	return nil
}

// Score is the key-number dictionary a grader produced for a result, with the
// logs and run metrics that came with it. It is identified by its repo, task
// stub, test stub, result ref, and ref, so two scores with the same identity
// are the same score. Dictionary holds a number per key of the result's test
// score schema.
type Score struct {
	Org          string             `gorm:"primaryKey"`
	Repo         string             `gorm:"primaryKey"`
	Task         string             `gorm:"primaryKey"`
	Test         string             `gorm:"primaryKey"`
	Result       string             `gorm:"primaryKey"`
	Ref          string             `gorm:"primaryKey"`
	Dictionary   map[string]float64 `gorm:"serializer:json;type:jsonb"`
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
}

// Validate reports whether the score's run metrics are zero or greater. A score
// that omits a metric records zero. Its dictionary, logs, and result may be
// empty.
func (s Score) Validate() error {
	if s.InputTokens < 0 || s.OutputTokens < 0 || s.TokenCost < 0 || s.Duration < 0 {
		return ErrInvalidScore
	}
	return nil
}
