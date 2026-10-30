package main

import (
	"fmt"
)

// configGrader is a grader entry in the CLI config. It records the grader that
// scored a result. A value that is not set is omitted when the entry is printed.
type configGrader struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Stub        string `yaml:"stub,omitempty" json:"stub,omitempty"`
	Commit      string `yaml:"commit,omitempty" json:"commit,omitempty"`
	Branch      string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Uncommitted bool   `yaml:"uncommitted" json:"uncommitted"`
}

// getGrader writes the grader recorded for the task as YAML. It fails when the
// task has no grader.
func getGrader(d deps, stub string) error {
	tasks, err := loadConfigTasks(d.workDir)
	if err != nil {
		return err
	}
	task, ok := tasks[stub]
	if !ok || task.Grader == nil {
		return fmt.Errorf("task %s has no grader", stub)
	}
	return writeYAML(d.out, *task.Grader)
}
