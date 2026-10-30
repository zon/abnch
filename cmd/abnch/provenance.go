package main

// modelRecord is the model a result or score used. Result and score submission
// carries it, so the service creates or reuses the record.
type modelRecord struct {
	Provider string `yaml:"provider" json:"provider"`
	Name     string `yaml:"name" json:"name"`
	Variant  string `yaml:"variant" json:"variant"`
}

// agentRecord is the agent that produced a result. Result submission carries it,
// so the service creates or reuses the record.
type agentRecord struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Stub        string `yaml:"stub,omitempty" json:"stub,omitempty"`
	Commit      string `yaml:"commit,omitempty" json:"commit,omitempty"`
	Branch      string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Uncommitted bool   `yaml:"uncommitted" json:"uncommitted"`
}

// graderRecord is the grader that scored a result. Score submission carries it,
// so the service creates or reuses the record.
type graderRecord struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Stub        string `yaml:"stub,omitempty" json:"stub,omitempty"`
	Commit      string `yaml:"commit,omitempty" json:"commit,omitempty"`
	Branch      string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Uncommitted bool   `yaml:"uncommitted" json:"uncommitted"`
}
