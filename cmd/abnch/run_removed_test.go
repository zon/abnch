package main

import (
	"os"
	"testing"
)

func TestRemovedRunAndScoreCommandsFailWithoutChangingConfig(t *testing.T) {
	const body = "url: https://bench.example.com\n" +
		"other:\n  key: value\n"
	commands := []struct {
		name string
		args []string
	}{
		{"run", []string{"run", "fix-bug/basic", "--limit", "1", "--shuffle"}},
		{"score", []string{"score", "result-ref", "--model", "sonnet"}},
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
