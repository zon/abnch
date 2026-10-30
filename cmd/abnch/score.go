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
)

// scoreRef is a score's ref as the service API returns it.
type scoreRef struct {
	Ref string `json:"ref"`
}

// scoreRequest is the JSON body a score write carries. Model and Grader are the
// records the service creates or reuses; Body holds the key-number score the
// grader produced; the rest are its logs and run metrics.
type scoreRequest struct {
	Task         string             `json:"task"`
	Test         string             `json:"test"`
	Result       string             `json:"result"`
	Model        *configModel       `json:"model,omitempty"`
	Grader       *configGrader      `json:"grader,omitempty"`
	Body         map[string]float64 `json:"body"`
	Logs         string             `json:"logs"`
	InputTokens  int                `json:"inputTokens"`
	OutputTokens int                `json:"outputTokens"`
	TokenCost    float64            `json:"tokenCost"`
	Duration     float64            `json:"duration"`
}

// scoreInput is the YAML or JSON input an add score command reads. Model and
// Grader are the records the service creates or reuses; Body is the score the
// grader produced; the rest are its logs and run metrics.
type scoreInput struct {
	Model        *configModel       `json:"model" yaml:"model"`
	Grader       *configGrader      `json:"grader" yaml:"grader"`
	Body         map[string]float64 `json:"body" yaml:"body"`
	Logs         string             `json:"logs" yaml:"logs"`
	InputTokens  int                `json:"inputTokens" yaml:"inputTokens"`
	OutputTokens int                `json:"outputTokens" yaml:"outputTokens"`
	TokenCost    float64            `json:"tokenCost" yaml:"tokenCost"`
	Duration     float64            `json:"duration" yaml:"duration"`
}

// scoreResultView is the part of a result the add score command reads: the
// result's ref, the task and test it belongs to, the datum that produced it, and
// its body.
type scoreResultView struct {
	Ref  string          `json:"ref"`
	Task string          `json:"task"`
	Test string          `json:"test"`
	Data string          `json:"data"`
	Body json.RawMessage `json:"body"`
}

// addScore reads a score input for the result the ref names in the inferred
// repo, posts it, prints the score ref, and records the model and grader in the
// CLI config. It reads the input from `--file` when given and from stdin
// otherwise. It fails without a request when the input lacks a model, a grader,
// or a body, or when the result is missing.
func addScore(d deps, c addScoreCmd) error {
	input, err := readAddScoreInput(d, c.File)
	if err != nil {
		return err
	}
	request, err := scoreRequestFromInput(c.Ref, input)
	if err != nil {
		return err
	}

	target, err := resolveTarget(d)
	if err != nil {
		return err
	}
	result, err := readScoreResult(d.ctx, target, c.Ref)
	if err != nil {
		return err
	}
	request.Task = result.Task
	request.Test = result.Test

	ref, err := postScore(d.ctx, target.client, target.org, target.stub, request)
	if err != nil {
		return err
	}
	if err := recordScoreProvenance(d.workDir, result.Task, *input.Model, *input.Grader); err != nil {
		return err
	}
	fmt.Fprintln(d.out, ref)
	return nil
}

// readAddScoreInput reads and parses the score input from the file the command
// names, or from stdin when no file is given. The input may be YAML or JSON.
func readAddScoreInput(d deps, file string) (scoreInput, error) {
	return readInput[scoreInput](d, file, "score")
}

// scoreRequestFromInput builds the score write body from the input and the
// result ref. It fails when the input lacks a model, a grader, or a body, or
// when a record lacks a required field.
func scoreRequestFromInput(ref string, input scoreInput) (scoreRequest, error) {
	if input.Model == nil {
		return scoreRequest{}, errors.New("score input has no model")
	}
	if input.Model.Name == "" || input.Model.Provider == "" {
		return scoreRequest{}, errors.New("score model must carry a name and provider")
	}
	if input.Grader == nil {
		return scoreRequest{}, errors.New("score input has no grader")
	}
	if input.Grader.Name == "" || input.Grader.Stub == "" || input.Grader.Commit == "" || input.Grader.Branch == "" {
		return scoreRequest{}, errors.New("score grader must carry a name, stub, commit, and branch")
	}
	if input.Body == nil {
		return scoreRequest{}, errors.New("score input has no body")
	}
	return scoreRequest{
		Result: ref, Model: input.Model, Grader: input.Grader,
		Body: input.Body, Logs: input.Logs,
		InputTokens: input.InputTokens, OutputTokens: input.OutputTokens,
		TokenCost: input.TokenCost, Duration: input.Duration,
	}, nil
}

// recordScoreProvenance records the model and grader a score used in the CLI
// config under workDir. The model is stored under its name and the grader under
// the result's task.
func recordScoreProvenance(workDir, task string, model configModel, grader configGrader) error {
	if err := setResultModel(workDir, model); err != nil {
		return err
	}
	return setTaskGraderRecord(workDir, task, grader)
}

// readScoreResult reads the result the ref names in the repo and parses the
// fields the add score command needs. A missing repo or result is an error.
func readScoreResult(ctx context.Context, target repoTarget, ref string) (scoreResultView, error) {
	body, err := readResult(ctx, target.client, target.org, target.stub, ref)
	if err != nil {
		return scoreResultView{}, err
	}
	var result scoreResultView
	if err := json.Unmarshal(body, &result); err != nil {
		return scoreResultView{}, fmt.Errorf("failed to read the result: %w", err)
	}
	return result, nil
}

// postScore records a score and returns its ref. A not-found response maps to
// the missing result the body names.
func postScore(ctx context.Context, client *serviceClient, org, repo string, body scoreRequest) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	resp, err := client.do(ctx, http.MethodPost, scoresPath(org, repo), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		defer resp.Body.Close()
		var ref scoreRef
		if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
			return "", fmt.Errorf("failed to read the score ref: %w", err)
		}
		return ref.Ref, nil
	case http.StatusNotFound:
		return "", resultNotFoundError(resp, org, repo, body.Result)
	default:
		return "", serviceError(resp)
	}
}

// scoresPath returns the API path for a repo's scores.
func scoresPath(org, repo string) string {
	return repoPath(org, repo) + "/scores"
}

// scoreMissingMessage is the message the service reports for a score it does
// not know.
const scoreMissingMessage = "score not found"

// scorePath returns the API path for one score in a repo's scores.
func scorePath(org, repo, ref string) string {
	return scoresPath(org, repo) + "/" + url.PathEscape(ref)
}

// readScoreRefs returns the refs of the scores in the repo the client
// identifies. A missing repo is an error.
func readScoreRefs(ctx context.Context, client *serviceClient, org, repo string) ([]string, error) {
	resp, err := client.do(ctx, http.MethodGet, scoresPath(org, repo), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		var refs []string
		if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
			return nil, fmt.Errorf("failed to read the scores: %w", err)
		}
		return refs, nil
	case http.StatusNotFound:
		return nil, scoreNotFoundError(resp, org, repo, "")
	default:
		return nil, serviceError(resp)
	}
}

// readScore returns the JSON of the score the ref names in the repo. A missing
// repo or score is an error.
func readScore(ctx context.Context, client *serviceClient, org, repo, ref string) ([]byte, error) {
	resp, err := client.do(ctx, http.MethodGet, scorePath(org, repo, ref), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read the score: %w", err)
		}
		return body, nil
	case http.StatusNotFound:
		return nil, scoreNotFoundError(resp, org, repo, ref)
	default:
		return nil, serviceError(resp)
	}
}

// scoreNotFoundError maps a score request's not-found response to the resource
// the service does not know. A ref names the score when the response reports
// one.
func scoreNotFoundError(resp *http.Response, org, repo, ref string) error {
	message, ok := serviceMessage(resp)
	switch {
	case !ok:
		return fmt.Errorf("service returned %s", resp.Status)
	case message == repoMissingMessage:
		return repoMissing{org: org, repo: repo}
	case message == scoreMissingMessage && ref != "":
		return fmt.Errorf("score %s not found", ref)
	default:
		return errors.New(message)
	}
}
