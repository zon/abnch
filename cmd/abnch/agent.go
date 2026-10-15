package main

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// configAgent is an agent entry in the CLI config.
type configAgent struct {
	VersionCmd string `yaml:"versionCmd"`
	BuildCmd   string `yaml:"buildCmd"`
	RunCmd     string `yaml:"runCmd"`
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
