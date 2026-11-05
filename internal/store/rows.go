package store

// resultModel is the model a result row carries.
type resultModel struct {
	Name     string
	Provider string
	Variant  string
}

// resultAgent is the agent a result row carries.
type resultAgent struct {
	Org         string
	Repo        string
	Task        string
	Name        string
	Stub        string
	Commit      string
	Branch      string
	Version     string
	Uncommitted bool
}

// resultRow is the stored form of a result. Results keep the provenance values
// they were written with until results link to their records by id.
type resultRow struct {
	Org          string      `gorm:"primaryKey"`
	Repo         string      `gorm:"primaryKey"`
	Task         string      `gorm:"primaryKey"`
	Test         string      `gorm:"primaryKey"`
	Data         string      `gorm:"primaryKey"`
	Ref          string      `gorm:"primaryKey"`
	Model        resultModel `gorm:"embedded"`
	Agent        resultAgent `gorm:"embedded;embeddedPrefix:agent_"`
	Body         string
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
	Local        string
}

// TableName returns the table that stores results.
func (resultRow) TableName() string {
	return "results"
}

// TableName returns the table that stores results.
func (Result) TableName() string {
	return "results"
}

// row returns the stored form of a result.
func (r Result) row() resultRow {
	return resultRow{
		Org: r.Org, Repo: r.Repo, Task: r.Task, Test: r.Test, Data: r.Data, Ref: r.Ref,
		Model: resultModel{Name: r.Model.Name, Provider: r.Model.Provider, Variant: r.Model.Variant},
		Agent: resultAgent{
			Org: r.Agent.Org, Repo: r.Agent.Repo, Task: r.Agent.Task,
			Name: r.Agent.Name, Stub: r.Agent.Stub, Commit: r.Agent.Commit,
			Branch: r.Agent.Branch, Version: r.Agent.Version, Uncommitted: r.Agent.Uncommitted,
		},
		Body: r.Body, Logs: r.Logs,
		InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		TokenCost: r.TokenCost, Duration: r.Duration, Local: r.Local,
	}
}

// result returns the result a row stores.
func (row resultRow) result() Result {
	return Result{
		Org: row.Org, Repo: row.Repo, Task: row.Task, Test: row.Test, Data: row.Data, Ref: row.Ref,
		Model: Model{Name: row.Model.Name, Provider: row.Model.Provider, Variant: row.Model.Variant},
		Agent: Agent{
			Org: row.Agent.Org, Repo: row.Agent.Repo, Task: row.Agent.Task,
			Name: row.Agent.Name, Stub: row.Agent.Stub, Commit: row.Agent.Commit,
			Branch: row.Agent.Branch, Version: row.Agent.Version, Uncommitted: row.Agent.Uncommitted,
		},
		Body: row.Body, Logs: row.Logs,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens,
		TokenCost: row.TokenCost, Duration: row.Duration, Local: row.Local,
	}
}

// scoreGrader is the grader a score row carries.
type scoreGrader struct {
	Org         string
	Repo        string
	Task        string
	Test        string
	Name        string
	Stub        string
	Commit      string
	Branch      string
	Version     string
	Uncommitted bool
}

// scoreRow is the stored form of a score. Scores keep the provenance values
// they were written with until scores link to their records by id.
type scoreRow struct {
	Org          string             `gorm:"primaryKey"`
	Repo         string             `gorm:"primaryKey"`
	Task         string             `gorm:"primaryKey"`
	Test         string             `gorm:"primaryKey"`
	Result       string             `gorm:"primaryKey"`
	Ref          string             `gorm:"primaryKey"`
	Model        resultModel        `gorm:"embedded"`
	Grader       scoreGrader        `gorm:"embedded;embeddedPrefix:grader_"`
	Body         map[string]float64 `gorm:"serializer:json;type:jsonb"`
	Logs         string
	InputTokens  int
	OutputTokens int
	TokenCost    float64
	Duration     float64
}

// TableName returns the table that stores scores.
func (scoreRow) TableName() string {
	return "scores"
}

// TableName returns the table that stores scores.
func (Score) TableName() string {
	return "scores"
}

// row returns the stored form of a score.
func (s Score) row() scoreRow {
	return scoreRow{
		Org: s.Org, Repo: s.Repo, Task: s.Task, Test: s.Test, Result: s.Result, Ref: s.Ref,
		Model: resultModel{Name: s.Model.Name, Provider: s.Model.Provider, Variant: s.Model.Variant},
		Grader: scoreGrader{
			Org: s.Grader.Org, Repo: s.Grader.Repo, Task: s.Grader.Task, Test: s.Grader.Test,
			Name: s.Grader.Name, Stub: s.Grader.Stub, Commit: s.Grader.Commit,
			Branch: s.Grader.Branch, Version: s.Grader.Version, Uncommitted: s.Grader.Uncommitted,
		},
		Body: s.Body, Logs: s.Logs,
		InputTokens: s.InputTokens, OutputTokens: s.OutputTokens,
		TokenCost: s.TokenCost, Duration: s.Duration,
	}
}

// score returns the score a row stores.
func (row scoreRow) score() Score {
	return Score{
		Org: row.Org, Repo: row.Repo, Task: row.Task, Test: row.Test, Result: row.Result, Ref: row.Ref,
		Model: Model{Name: row.Model.Name, Provider: row.Model.Provider, Variant: row.Model.Variant},
		Grader: Grader{
			Org: row.Grader.Org, Repo: row.Grader.Repo, Task: row.Grader.Task, Test: row.Grader.Test,
			Name: row.Grader.Name, Stub: row.Grader.Stub, Commit: row.Grader.Commit,
			Branch: row.Grader.Branch, Version: row.Grader.Version, Uncommitted: row.Grader.Uncommitted,
		},
		Body: row.Body, Logs: row.Logs,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens,
		TokenCost: row.TokenCost, Duration: row.Duration,
	}
}
