package main

import (
	"fmt"

	"go.yaml.in/yaml/v3"
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

// setAgent records the task's agent in the CLI config under the working
// directory. It merges the commands the flags carry with an existing agent and
// fails when the merged agent has no run command.
func setAgent(d deps, c setAgentCmd) error {
	agent := configAgent{VersionCmd: c.VersionCmd, BuildCmd: c.BuildCmd, RunCmd: c.RunCmd}
	if err := setTaskAgent(d.workDir, c.Stub, agent); err != nil {
		return err
	}
	fmt.Fprintf(d.out, "Set agent for task %s\n", c.Stub)
	return nil
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

// agentOf reads the agent commands from a YAML mapping node.
func agentOf(entry *yaml.Node) configAgent {
	return configAgent{
		VersionCmd: mappingString(entry, "versionCmd"),
		BuildCmd:   mappingString(entry, "buildCmd"),
		RunCmd:     mappingString(entry, "runCmd"),
	}
}

// mergeAgent replaces the commands update carries on existing and keeps the
// rest.
func mergeAgent(existing, update configAgent) configAgent {
	if update.VersionCmd != "" {
		existing.VersionCmd = update.VersionCmd
	}
	if update.BuildCmd != "" {
		existing.BuildCmd = update.BuildCmd
	}
	if update.RunCmd != "" {
		existing.RunCmd = update.RunCmd
	}
	return existing
}
