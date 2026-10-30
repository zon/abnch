package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// resultInput is the YAML or JSON input an add result command reads. Model and
// Agent are the records the service creates or reuses; JSON is the result the
// agent produced; the rest are its logs and run metrics.
type resultInput struct {
	Model        *configModel `json:"model" yaml:"model"`
	Agent        *configAgent `json:"agent" yaml:"agent"`
	JSON         jsonValue    `json:"json" yaml:"json"`
	Logs         string       `json:"logs" yaml:"logs"`
	InputTokens  int          `json:"inputTokens" yaml:"inputTokens"`
	OutputTokens int          `json:"outputTokens" yaml:"outputTokens"`
	TokenCost    float64      `json:"tokenCost" yaml:"tokenCost"`
	Duration     float64      `json:"duration" yaml:"duration"`
}

// jsonValue is a JSON value from a YAML or JSON input. It keeps the raw JSON so
// the CLI posts the value as a JSON property alongside the rest of the input.
type jsonValue struct {
	raw     json.RawMessage
	present bool
}

// UnmarshalJSON keeps a JSON input's value as it was written.
func (v *jsonValue) UnmarshalJSON(data []byte) error {
	v.raw = append(v.raw[:0], data...)
	v.present = true
	return nil
}

// UnmarshalYAML converts a YAML input's value to JSON.
func (v *jsonValue) UnmarshalYAML(node *yaml.Node) error {
	var value any
	if err := node.Decode(&value); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	v.raw = data
	v.present = true
	return nil
}

// addResult reads a result input for the datum the path names in the inferred
// repo, posts it, prints the result ref, and records the model and agent in the
// CLI config. It reads the input from `--file` when given and from stdin
// otherwise. It fails without a request when the input lacks a model, an agent,
// or json, or when the path does not name a datum.
func addResult(d deps, c addResultCmd) error {
	task, test, datum, err := splitResultPath(c.Path)
	if err != nil {
		return err
	}

	input, err := readResultInput(d, c.File)
	if err != nil {
		return err
	}
	request, err := resultRequestFromInput(task, test, datum, input)
	if err != nil {
		return err
	}

	target, err := resolveTarget(d)
	if err != nil {
		return err
	}

	ref, err := postResult(d.ctx, target.client, target.org, target.stub, request)
	if err != nil {
		return err
	}
	if err := recordResultProvenance(d.workDir, task, *input.Model, *input.Agent); err != nil {
		return err
	}
	fmt.Fprintln(d.out, ref)
	return nil
}

// splitResultPath splits `<task>/<test>/<datum ref>` into its parts. All three
// are required.
func splitResultPath(path string) (task, test, datum string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("result path %q must be <task>/<test>/<datum ref>", path)
	}
	return parts[0], parts[1], parts[2], nil
}

// readResultInput reads and parses the result input from the file the command
// names, or from stdin when no file is given. The input may be YAML or JSON.
func readResultInput(d deps, file string) (resultInput, error) {
	data, err := readResultInputBytes(d, file)
	if err != nil {
		return resultInput{}, err
	}
	var input resultInput
	if json.Valid(data) {
		err = json.Unmarshal(data, &input)
	} else {
		err = yaml.Unmarshal(data, &input)
	}
	if err != nil {
		return resultInput{}, fmt.Errorf("invalid result input: %w", err)
	}
	return input, nil
}

// readResultInputBytes reads the raw result input from the file the command
// names, or from stdin when no file is given. A relative file resolves under the
// working directory, and a terminal with no file carries no input.
func readResultInputBytes(d deps, file string) ([]byte, error) {
	if file != "" {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(d.workDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}
		return data, nil
	}
	if !pipedInput(d.in) {
		return nil, errors.New("no result input: provide a value on stdin or with --file")
	}
	data, err := io.ReadAll(d.in)
	if err != nil {
		return nil, fmt.Errorf("failed to read the result input: %w", err)
	}
	return data, nil
}

// resultRequestFromInput builds the result write body from the input and the
// datum the path names. It fails when the input lacks a model, an agent, or
// json, or when a record lacks a required field.
func resultRequestFromInput(task, test, datum string, input resultInput) (resultRequest, error) {
	if input.Model == nil {
		return resultRequest{}, errors.New("result input has no model")
	}
	if input.Model.Name == "" || input.Model.Provider == "" {
		return resultRequest{}, errors.New("result model must carry a name and provider")
	}
	if input.Agent == nil {
		return resultRequest{}, errors.New("result input has no agent")
	}
	if input.Agent.Name == "" || input.Agent.Stub == "" || input.Agent.Commit == "" || input.Agent.Branch == "" {
		return resultRequest{}, errors.New("result agent must carry a name, stub, commit, and branch")
	}
	if !input.JSON.present || len(input.JSON.raw) == 0 {
		return resultRequest{}, errors.New("result input has no json")
	}
	return resultRequest{
		Task: task, Test: test, Data: datum,
		Model: input.Model, Agent: input.Agent,
		JSON: input.JSON.raw, Logs: input.Logs,
		InputTokens: input.InputTokens, OutputTokens: input.OutputTokens,
		TokenCost: input.TokenCost, Duration: input.Duration,
	}, nil
}

// recordResultProvenance records the model and agent a result used in the CLI
// config under workDir. The model is stored under its name and the agent under
// the result's task.
func recordResultProvenance(workDir, task string, model configModel, agent configAgent) error {
	if err := setResultModel(workDir, model); err != nil {
		return err
	}
	return setTaskAgentRecord(workDir, task, agent)
}

// resultMissingMessage is the message the service reports for a result it does
// not know.
const resultMissingMessage = "result not found"

// resultPath returns the API path for one result in a repo's results.
func resultPath(org, repo, ref string) string {
	return resultsPath(org, repo) + "/" + url.PathEscape(ref)
}

// readResultRefs returns the refs of the results in the repo the client
// identifies. A missing repo is an error.
func readResultRefs(ctx context.Context, client *serviceClient, org, repo string) ([]string, error) {
	resp, err := client.do(ctx, http.MethodGet, resultsPath(org, repo), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		var refs []string
		if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
			return nil, fmt.Errorf("failed to read the results: %w", err)
		}
		return refs, nil
	case http.StatusNotFound:
		return nil, resultNotFoundError(resp, org, repo, "")
	default:
		return nil, serviceError(resp)
	}
}

// readResult returns the JSON of the result the ref names in the repo. A missing
// repo or result is an error.
func readResult(ctx context.Context, client *serviceClient, org, repo, ref string) ([]byte, error) {
	resp, err := client.do(ctx, http.MethodGet, resultPath(org, repo, ref), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read the result: %w", err)
		}
		return body, nil
	case http.StatusNotFound:
		return nil, resultNotFoundError(resp, org, repo, ref)
	default:
		return nil, serviceError(resp)
	}
}

// resultNotFoundError maps a result request's not-found response to the resource
// the service does not know. A ref names the result when the response reports
// one.
func resultNotFoundError(resp *http.Response, org, repo, ref string) error {
	message, ok := serviceMessage(resp)
	switch {
	case !ok:
		return fmt.Errorf("service returned %s", resp.Status)
	case message == repoMissingMessage:
		return repoMissing{org: org, repo: repo}
	case message == resultMissingMessage && ref != "":
		return fmt.Errorf("result %s not found", ref)
	default:
		return errors.New(message)
	}
}
