package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"go.yaml.in/yaml/v3"
)

// task is a task as the CLI reads it from the service.
type task struct {
	Stub        string `json:"stub"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
}

// taskBody carries the task fields the CLI merges from options and stdin.
type taskBody struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// moveTaskRequest is the JSON body a task move carries.
type moveTaskRequest struct {
	Stub string `json:"stub"`
}

// setTask creates, updates, or renames the task the command names in the
// inferred repo. A `--model` records the task's default model in the CLI config
// and leaves the service untouched. It renames the task when `--move` is given
// and otherwise merges the options and the stdin body with the stored task,
// creating the repo first when it is missing.
func setTask(d deps, c setTaskCmd) error {
	if c.Model != "" {
		if err := setTaskModel(d.workDir, c.Stub, c.Model); err != nil {
			return err
		}
		fmt.Fprintf(d.out, "Set model %s for task %s\n", c.Model, c.Stub)
		return nil
	}

	target, err := resolveTarget(d)
	if err != nil {
		return err
	}

	if c.Move != "" {
		if err := moveTask(d.ctx, target.client, target.org, target.stub, c.Stub, c.Move); err != nil {
			return err
		}
		fmt.Fprintf(d.out, "Moved task %s/%s/%s to %s\n", target.org, target.stub, c.Stub, c.Move)
		return nil
	}

	body, err := readTaskBody(d.in)
	if err != nil {
		return err
	}

	existing, found, err := readTask(d.ctx, target.client, target.org, target.stub, c.Stub)
	if err != nil {
		return err
	}

	merged := mergeTask(existing, body)
	merged = mergeTask(merged, taskBody{Name: c.Name, Description: c.Description})
	if merged.Name == "" || merged.Description == "" {
		return errors.New("task name and description are required")
	}

	if err := ensureRepo(d.ctx, target); err != nil {
		return err
	}
	if err := writeTask(d.ctx, target.client, target.org, target.stub, c.Stub, taskBody{Name: merged.Name, Description: merged.Description}); err != nil {
		return err
	}

	action := "Created"
	if found {
		action = "Updated"
	}
	fmt.Fprintf(d.out, "%s task %s/%s/%s\n", action, target.org, target.stub, c.Stub)
	return nil
}

// readTaskBody parses the YAML or JSON task body from in. Empty input is an
// empty body, and a terminal carries no body.
func readTaskBody(in io.Reader) (taskBody, error) {
	if !pipedInput(in) {
		return taskBody{}, nil
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return taskBody{}, fmt.Errorf("failed to read the task body: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return taskBody{}, nil
	}
	var body taskBody
	if err := yaml.Unmarshal(data, &body); err != nil {
		return taskBody{}, fmt.Errorf("invalid task body: %w", err)
	}
	return body, nil
}

// pipedInput reports whether in carries piped data rather than a terminal.
func pipedInput(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return in != nil
	}
	info, err := f.Stat()
	if err != nil {
		return true
	}
	return info.Mode()&os.ModeCharDevice == 0
}

// mergeTask replaces the fields body carries on t and keeps the rest.
func mergeTask(t task, body taskBody) task {
	if body.Name != "" {
		t.Name = body.Name
	}
	if body.Description != "" {
		t.Description = body.Description
	}
	return t
}

// readTask returns the task the repo and stub identify and whether it exists.
func readTask(ctx context.Context, client *serviceClient, org, repo, stub string) (task, bool, error) {
	resp, err := client.do(ctx, http.MethodGet, taskPath(org, repo, stub), nil)
	if err != nil {
		return task{}, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return task{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return task{}, false, serviceError(resp)
	}
	defer resp.Body.Close()

	var t task
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return task{}, false, fmt.Errorf("failed to read the task: %w", err)
	}
	return t, true, nil
}

// writeTask writes the task the repo and stub identify.
func writeTask(ctx context.Context, client *serviceClient, org, repo, stub string, body taskBody) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := client.do(ctx, http.MethodPut, taskPath(org, repo, stub), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return serviceError(resp)
	}
	resp.Body.Close()
	return nil
}

// taskPath returns the API path for a task.
func taskPath(org, repo, stub string) string {
	return repoPath(org, repo) + "/tasks/" + url.PathEscape(stub)
}

// moveTask renames the task the repo and stub identify to newStub.
func moveTask(ctx context.Context, client *serviceClient, org, repo, stub, newStub string) error {
	payload, err := json.Marshal(moveTaskRequest{Stub: newStub})
	if err != nil {
		return err
	}
	resp, err := client.do(ctx, http.MethodPost, taskPath(org, repo, stub)+"/move", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return serviceError(resp)
	}
	resp.Body.Close()
	return nil
}
