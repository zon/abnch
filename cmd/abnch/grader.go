package main

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// configGrader is a grader entry in the CLI config. A command that is not set
// is omitted when the entry is printed.
type configGrader struct {
	VersionCmd string `yaml:"versionCmd,omitempty"`
	BuildCmd   string `yaml:"buildCmd,omitempty"`
	RunCmd     string `yaml:"runCmd"`
}

// setGrader records the task's grader in the CLI config under the working
// directory. It merges the commands the flags carry with an existing grader and
// fails when the merged grader has no run command.
func setGrader(d deps, c setGraderCmd) error {
	grader := configGrader{VersionCmd: c.VersionCmd, BuildCmd: c.BuildCmd, RunCmd: c.RunCmd}
	if err := setTaskGrader(d.workDir, c.Stub, grader); err != nil {
		return err
	}
	fmt.Fprintf(d.out, "Set grader for task %s\n", c.Stub)
	return nil
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

// graderOf reads the grader commands from a YAML mapping node.
func graderOf(entry *yaml.Node) configGrader {
	return configGrader{
		VersionCmd: mappingString(entry, "versionCmd"),
		BuildCmd:   mappingString(entry, "buildCmd"),
		RunCmd:     mappingString(entry, "runCmd"),
	}
}

// mergeGrader replaces the commands update carries on existing and keeps the
// rest.
func mergeGrader(existing, update configGrader) configGrader {
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
