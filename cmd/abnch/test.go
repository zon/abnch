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
	"strings"
)

// test is a test as the CLI reads it from the service.
type test struct {
	Stub        string      `json:"stub" yaml:"stub"`
	Name        string      `json:"name" yaml:"name"`
	Description string      `json:"description" yaml:"description"`
	Version     string      `json:"version" yaml:"version"`
	Schema      []schemaKey `json:"schema" yaml:"schema"`
}

// schemaKey is one key of a test's score schema.
type schemaKey struct {
	Stub string `json:"stub" yaml:"stub"`
	Name string `json:"name" yaml:"name"`
}

// testBody carries the test fields the CLI merges from options and stdin. A
// schema the body carries replaces the stored schema, so a nil schema leaves it
// alone.
type testBody struct {
	Name        string      `json:"name" yaml:"name"`
	Description string      `json:"description" yaml:"description"`
	Version     string      `json:"version" yaml:"version"`
	Schema      []schemaKey `json:"schema" yaml:"schema"`
}

// moveTestRequest is the JSON body a test move carries.
type moveTestRequest struct {
	Stub string `json:"stub"`
}

// testMissingMessage is the message the service reports for a test it does not
// know.
const testMissingMessage = "test not found"

// taskMissingMessage is the message the service reports for a task it does not
// know.
const taskMissingMessage = "task not found"

// setTest creates, updates, or renames the test the command names under the task
// in the inferred repo. It renames the test when `--move` is given and otherwise
// merges the options and the stdin body with the stored test, failing when the
// test would miss a required field or its task or repo is missing.
func setTest(d deps, c setTestCmd) error {
	task, stub, err := splitTestPath(c.Path)
	if err != nil {
		return err
	}

	target, err := resolveTarget(d)
	if err != nil {
		return err
	}

	if c.Move != "" {
		if err := moveTest(d.ctx, target.client, target.org, target.stub, task, stub, c.Move); err != nil {
			return err
		}
		fmt.Fprintf(d.out, "Moved test %s/%s/%s/%s to %s\n", target.org, target.stub, task, stub, c.Move)
		return nil
	}

	body, err := readTestBody(d.in)
	if err != nil {
		return err
	}

	existing, found, err := readTest(d.ctx, target.client, target.org, target.stub, task, stub)
	if err != nil {
		return err
	}

	merged := mergeTest(existing, body)
	merged = mergeTest(merged, testBody{Name: c.Name, Description: c.Description, Version: c.Version})
	if err := requireTestFields(merged); err != nil {
		return err
	}

	if err := writeTest(d.ctx, target.client, target.org, target.stub, task, stub, merged); err != nil {
		return err
	}

	action := "Created"
	if found {
		action = "Updated"
	}
	fmt.Fprintf(d.out, "%s test %s/%s/%s/%s\n", action, target.org, target.stub, task, stub)
	return nil
}

// splitTestPath splits the `<task>/<stub>` argument into its task and test
// stubs.
func splitTestPath(path string) (task, stub string, err error) {
	task, stub, ok := strings.Cut(path, "/")
	if !ok || task == "" || stub == "" {
		return "", "", fmt.Errorf("test path %q must be <task>/<stub>", path)
	}
	return task, stub, nil
}

// readTestBody parses the YAML or JSON test body from in. Empty input is an
// empty body, and a terminal carries no body.
func readTestBody(in io.Reader) (testBody, error) {
	return readBody[testBody](in, "test")
}

// readTest returns the test the repo, task, and stub identify and whether it
// exists. A missing repo or task is an error, because a test cannot be written
// to a task the service does not know.
func readTest(ctx context.Context, client *serviceClient, org, repo, task, stub string) (test, bool, error) {
	resp, err := client.do(ctx, http.MethodGet, testPath(org, repo, task, stub), nil)
	if err != nil {
		return test{}, false, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		var t test
		if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
			return test{}, false, fmt.Errorf("failed to read the test: %w", err)
		}
		return t, true, nil
	case http.StatusNotFound:
		message, ok := serviceMessage(resp)
		switch {
		case !ok:
			return test{}, false, fmt.Errorf("service returned %s", resp.Status)
		case message == testMissingMessage:
			return test{}, false, nil
		case message == repoMissingMessage:
			return test{}, false, repoMissing{org: org, repo: repo}
		case message == taskMissingMessage:
			return test{}, false, fmt.Errorf("task %s not found", task)
		default:
			return test{}, false, errors.New(message)
		}
	default:
		return test{}, false, serviceError(resp)
	}
}

// readTests returns the tests of the task the repo and stub identify. A missing
// repo or task is an error, because a task the service does not know cannot
// hold tests.
func readTests(ctx context.Context, client *serviceClient, org, repo, task string) ([]test, error) {
	resp, err := client.do(ctx, http.MethodGet, testsPath(org, repo, task), nil)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		var tests []test
		if err := json.NewDecoder(resp.Body).Decode(&tests); err != nil {
			return nil, fmt.Errorf("failed to read the tests: %w", err)
		}
		return tests, nil
	case http.StatusNotFound:
		message, ok := serviceMessage(resp)
		switch {
		case !ok:
			return nil, fmt.Errorf("service returned %s", resp.Status)
		case message == repoMissingMessage:
			return nil, repoMissing{org: org, repo: repo}
		case message == taskMissingMessage:
			return nil, fmt.Errorf("task %s not found", task)
		default:
			return nil, errors.New(message)
		}
	default:
		return nil, serviceError(resp)
	}
}

// mergeTest replaces the fields body carries on t and keeps the rest. A body
// that carries a schema replaces the stored schema.
func mergeTest(t test, body testBody) test {
	if body.Name != "" {
		t.Name = body.Name
	}
	if body.Description != "" {
		t.Description = body.Description
	}
	if body.Version != "" {
		t.Version = body.Version
	}
	if body.Schema != nil {
		t.Schema = body.Schema
	}
	return t
}

// requireTestFields reports whether the merged test carries the fields the
// service requires: a name and a version. A description is optional.
func requireTestFields(t test) error {
	if t.Name == "" {
		return errors.New("test name is required")
	}
	if t.Version == "" {
		return errors.New("test version is required")
	}
	return nil
}

// writeTest writes the test the repo, task, and stub identify.
func writeTest(ctx context.Context, client *serviceClient, org, repo, task, stub string, merged test) error {
	body := testBody{
		Name:        merged.Name,
		Description: merged.Description,
		Version:     merged.Version,
		Schema:      merged.Schema,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := client.do(ctx, http.MethodPut, testPath(org, repo, task, stub), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return serviceError(resp)
	}
	resp.Body.Close()
	return nil
}

// testPath returns the API path for a test.
func testPath(org, repo, task, stub string) string {
	return taskPath(org, repo, task) + "/tests/" + url.PathEscape(stub)
}

// testsPath returns the API path for a task's tests.
func testsPath(org, repo, task string) string {
	return taskPath(org, repo, task) + "/tests"
}

// moveTest renames the test the repo, task, and stub identify to newStub.
func moveTest(ctx context.Context, client *serviceClient, org, repo, task, stub, newStub string) error {
	payload, err := json.Marshal(moveTestRequest{Stub: newStub})
	if err != nil {
		return err
	}
	resp, err := client.do(ctx, http.MethodPost, testPath(org, repo, task, stub)+"/move", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return serviceError(resp)
	}
	resp.Body.Close()
	return nil
}
