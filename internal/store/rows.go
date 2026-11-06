package store

// resultRow is the stored form of a result. It links to its datum, model, and
// agent by id and holds no parent stubs or provenance values.
type resultRow struct {
	ID           uint   `gorm:"primaryKey"`
	DatumID      uint   `gorm:"not null;uniqueIndex:idx_results_identity"`
	ModelID      uint   `gorm:"not null;uniqueIndex:idx_results_identity"`
	AgentID      uint   `gorm:"not null;uniqueIndex:idx_results_identity"`
	Ref          string `gorm:"not null;uniqueIndex:idx_results_identity"`
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
		DatumID: r.DatumID, ModelID: r.ModelID, AgentID: r.AgentID, Ref: r.Ref,
		Body: r.Body, Logs: r.Logs,
		InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		TokenCost: r.TokenCost, Duration: r.Duration, Local: r.Local,
	}
}

// result returns the result a row stores without the records it links to.
func (row resultRow) result() Result {
	return Result{
		ID: row.ID, DatumID: row.DatumID, ModelID: row.ModelID, AgentID: row.AgentID,
		Ref: row.Ref, Body: row.Body, Logs: row.Logs,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens,
		TokenCost: row.TokenCost, Duration: row.Duration, Local: row.Local,
	}
}

// scoreRow is the stored form of a score. It links to its result, model, and
// grader by id and holds no parent stubs or provenance values.
type scoreRow struct {
	ID           uint               `gorm:"primaryKey"`
	ResultID     uint               `gorm:"not null;uniqueIndex:idx_scores_identity"`
	ModelID      uint               `gorm:"not null;uniqueIndex:idx_scores_identity"`
	GraderID     uint               `gorm:"not null;uniqueIndex:idx_scores_identity"`
	Ref          string             `gorm:"not null;uniqueIndex:idx_scores_identity"`
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
		ResultID: s.ResultID, ModelID: s.ModelID, GraderID: s.GraderID, Ref: s.Ref,
		Body: s.Body, Logs: s.Logs,
		InputTokens: s.InputTokens, OutputTokens: s.OutputTokens,
		TokenCost: s.TokenCost, Duration: s.Duration,
	}
}

// score returns the score a row stores without the records it links to.
func (row scoreRow) score() Score {
	return Score{
		ID: row.ID, ResultID: row.ResultID, ModelID: row.ModelID, GraderID: row.GraderID,
		Ref: row.Ref, Body: row.Body, Logs: row.Logs,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens,
		TokenCost: row.TokenCost, Duration: row.Duration,
	}
}
