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

	"go.yaml.in/yaml/v3"
)

// resultRef is a result's ref as the service API returns it.
type resultRef struct {
	Ref string `json:"ref"`
}

// resultRequest is the JSON body a result write carries. Model and Agent are the
// records that produced the result, and Body is the result itself; the rest are
// its logs and run metrics.
type resultRequest struct {
	Task         string          `json:"task"`
	Test         string          `json:"test"`
	Data         string          `json:"data"`
	Model        *configModel    `json:"model,omitempty"`
	Agent        *configAgent    `json:"agent,omitempty"`
	Body         json.RawMessage `json:"body"`
	Logs         string          `json:"logs"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	TokenCost    float64         `json:"tokenCost"`
	Duration     float64         `json:"duration"`
}

// resultInput is the YAML or JSON input an add result command reads. Model and
// Agent are the records the service creates or reuses; Body is the result the
// agent produced; the rest are its logs and run metrics.
type resultInput struct {
	Model        *configModel `json:"model" yaml:"model"`
	Agent        *configAgent `json:"agent" yaml:"agent"`
	Body         bodyValue    `json:"body" yaml:"body"`
	Logs         string       `json:"logs" yaml:"logs"`
	InputTokens  int          `json:"inputTokens" yaml:"inputTokens"`
	OutputTokens int          `json:"outputTokens" yaml:"outputTokens"`
	TokenCost    float64      `json:"tokenCost" yaml:"tokenCost"`
	Duration     float64      `json:"duration" yaml:"duration"`
}

// bodyValue is a JSON value from a YAML or JSON input. It keeps the raw JSON so
// the CLI posts the value as a JSON property alongside the rest of the input.
type bodyValue struct {
	raw     json.RawMessage
	present bool
}

// UnmarshalJSON keeps a JSON input's value as it was written.
func (v *bodyValue) UnmarshalJSON(data []byte) error {
	v.raw = append(v.raw[:0], data...)
	v.present = true
	return nil
}

// UnmarshalYAML converts a YAML input's value to JSON.
func (v *bodyValue) UnmarshalYAML(node *yaml.Node) error {
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
// or a body, or when the path does not name a datum.
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
	return readInput[resultInput](d, file, "result")
}

// resultRequestFromInput builds the result write body from the input and the
// datum the path names. It fails when the input lacks a model, an agent, or a
// body, or when a record lacks a required field.
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
	if !input.Body.present || len(input.Body.raw) == 0 {
		return resultRequest{}, errors.New("result input has no body")
	}
	return resultRequest{
		Task: task, Test: test, Data: datum,
		Model: input.Model, Agent: input.Agent,
		Body: input.Body.raw, Logs: input.Logs,
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

// postResult posts a result write and returns the result's ref. The service
// returns the existing ref when the same result is already stored, so both a
// create and an existing result succeed. A not-found response maps to the
// missing repo, task, test, or datum.
func postResult(ctx context.Context, client *serviceClient, org, repo string, body resultRequest) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	resp, err := client.do(ctx, http.MethodPost, resultsPath(org, repo), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		defer resp.Body.Close()
		var ref resultRef
		if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
			return "", fmt.Errorf("failed to read the result ref: %w", err)
		}
		return ref.Ref, nil
	case http.StatusNotFound:
		return "", dataNotFoundError(resp, org, repo, body.Task, body.Test, body.Data)
	default:
		return "", serviceError(resp)
	}
}

// resultsPath returns the API path for a repo's results.
func resultsPath(org, repo string) string {
	return repoPath(org, repo) + "/results"
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
