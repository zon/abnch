package main

import (
	"os"
	"testing"
)

func TestRemovedSetCommandsFailWithoutChangingConfig(t *testing.T) {
	const body = "url: https://bench.example.com\n" +
		"models:\n  sonnet:\n    provider: anthropic\n    name: claude-sonnet\n" +
		"tasks:\n  fix-bug:\n    model: sonnet\n"
	commands := []struct {
		name string
		args []string
	}{
		{"model", []string{"set", "model", "anthropic/claude-sonnet"}},
		{"agent", []string{"set", "agent", "fix-bug", "--run-cmd", "./run.sh"}},
		{"grader", []string{"set", "grader", "fix-bug", "--run-cmd", "./grade.sh"}},
		{"task model", []string{"set", "task", "fix-bug", "--model", "sonnet"}},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			workDir := t.TempDir()
			writeCLIConfig(t, workDir, body)
			d := commandDeps(t, workDir)

			code, _, errOut := exitCode(t, command.args, d)
			if code == 0 {
				t.Error("exit = 0, want a non-zero exit for a removed command")
			}
			if errOut == "" {
				t.Error("stderr is empty, want the removed command reported")
			}
			after, err := os.ReadFile(configPath(workDir))
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != body {
				t.Errorf("config = %q, want it unchanged as %q", after, body)
			}
		})
	}
}
