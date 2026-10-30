package main

import (
	"fmt"
)

// configAgent is an agent entry in the CLI config. It records the agent that
// produced a result and, for the older run commands, the commands that run it.
// A value that is not set is omitted when the entry is printed.
type configAgent struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Stub        string `yaml:"stub,omitempty" json:"stub,omitempty"`
	Commit      string `yaml:"commit,omitempty" json:"commit,omitempty"`
	Branch      string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Uncommitted bool   `yaml:"uncommitted" json:"uncommitted"`
	VersionCmd  string `yaml:"versionCmd,omitempty" json:"-"`
	BuildCmd    string `yaml:"buildCmd,omitempty" json:"-"`
	RunCmd      string `yaml:"runCmd,omitempty" json:"-"`
}

// getAgent writes the agent recorded for the task as YAML. It fails when the
// task has no agent.
func getAgent(d deps, stub string) error {
	tasks, err := loadConfigTasks(d.workDir)
	if err != nil {
		return err
	}
	task, ok := tasks[stub]
	if !ok || task.Agent == nil {
		return fmt.Errorf("task %s has no agent", stub)
	}
	return writeYAML(d.out, *task.Agent)
}
