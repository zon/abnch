package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.yaml.in/yaml/v3"
)

// getTasks writes the tasks of the inferred repo as YAML.
func getTasks(d deps) error {
	target, err := resolveTarget(d)
	if err != nil {
		return err
	}
	tasks, err := readTasks(d.ctx, target.client, target.org, target.stub)
	if err != nil {
		return err
	}
	return writeYAML(d.out, tasks)
}

// getTask writes the task the stub names as YAML. It fails when the task is
// missing.
func getTask(d deps, stub string) error {
	target, err := resolveTarget(d)
	if err != nil {
		return err
	}
	t, found, err := readTask(d.ctx, target.client, target.org, target.stub, stub)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("task %s not found", stub)
	}
	return writeYAML(d.out, t)
}

// resolveTarget resolves the service URL and the repo a command acts on.
func resolveTarget(d deps) (repoTarget, error) {
	serviceURL, err := resolveServiceURL("", d.workDir)
	if err != nil {
		return repoTarget{}, err
	}
	return resolveRepo(d, serviceURL)
}

// readTasks returns the tasks of the repo the client identifies.
func readTasks(ctx context.Context, client *serviceClient, org, repo string) ([]task, error) {
	resp, err := client.do(ctx, http.MethodGet, tasksPath(org, repo), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, serviceError(resp)
	}
	defer resp.Body.Close()

	var tasks []task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to read the tasks: %w", err)
	}
	return tasks, nil
}

// tasksPath returns the API path for a repo's tasks.
func tasksPath(org, repo string) string {
	return repoPath(org, repo) + "/tasks"
}

// writeYAML renders value to w as YAML with two-space indents, the CLI's output
// style.
func writeYAML(w io.Writer, value any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(value); err != nil {
		return err
	}
	return enc.Close()
}
