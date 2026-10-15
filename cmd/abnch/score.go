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
	"time"
)

// graderInput is the JSON object the CLI writes to a grader's run command on
// stdin. It carries the model, the datum that produced the result, and the
// result.
type graderInput struct {
	Model  configModel     `json:"model"`
	Data   json.RawMessage `json:"data"`
	Result json.RawMessage `json:"result"`
}

// graderOutput is the JSON a grader's run command writes after the marker. Score
// is the key-number dictionary the grader produced; the rest are optional run
// metrics.
type graderOutput struct {
	Score        map[string]float64 `json:"score"`
	InputTokens  int                `json:"inputTokens"`
	OutputTokens int                `json:"outputTokens"`
	TokenCost    float64            `json:"tokenCost"`
}

// graderRun is one grader run: the logs it wrote, its score output, and how long
// it took.
type graderRun struct {
	logs     string
	output   graderOutput
	duration float64
}

// scoreRef is a score's ref as the service API returns it.
type scoreRef struct {
	Ref string `json:"ref"`
}

// scoreRequest is the JSON body a score write carries. Dictionary is the
// key-number dictionary the grader produced; the rest are its logs and run
// metrics.
type scoreRequest struct {
	Task         string             `json:"task"`
	Test         string             `json:"test"`
	Result       string             `json:"result"`
	Dictionary   map[string]float64 `json:"dictionary"`
	Logs         string             `json:"logs"`
	InputTokens  int                `json:"inputTokens"`
	OutputTokens int                `json:"outputTokens"`
	TokenCost    float64            `json:"tokenCost"`
	Duration     float64            `json:"duration"`
}

// scoreResultView is the part of a result the score command reads: the result's
// ref, the task and test it belongs to, the datum that produced it, and its JSON.
type scoreResultView struct {
	Ref  string          `json:"ref"`
	Task string          `json:"task"`
	Test string          `json:"test"`
	Data string          `json:"data"`
	JSON json.RawMessage `json:"json"`
}

// scoreResult runs the grader of the task that owns the result against the
// result and records a score, printing the score ref. It reads the result, the
// task's grader, and the model before it builds or runs anything, so it fails
// without running the grader when the result is missing, the task has no grader,
// or no model is set. The model flag overrides the task's default model.
func scoreResult(d deps, c scoreCmd) error {
	target, err := resolveTarget(d)
	if err != nil {
		return err
	}

	result, err := readScoreResult(d.ctx, target, c.Ref)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(d.workDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath(d.workDir), err)
	}
	task, ok := cfg.Tasks[result.Task]
	if !ok || task.Grader == nil || task.Grader.RunCmd == "" {
		return fmt.Errorf("task %s has no grader", result.Task)
	}
	model, err := resolveRunModel(cfg, task, c.Model, result.Task)
	if err != nil {
		return err
	}

	datum, err := readDatum(d.ctx, target.client, target.org, target.stub, result.Task, result.Test, result.Data)
	if err != nil {
		return err
	}

	if task.Grader.BuildCmd != "" {
		if _, _, err := runShell(d.ctx, d.workDir, task.Grader.BuildCmd, nil); err != nil {
			return fmt.Errorf("failed to build the grader: %w", err)
		}
	}

	run, err := runGrader(d.ctx, d.workDir, task.Grader.RunCmd, model, datum, result.JSON)
	if err != nil {
		return err
	}
	ref, err := writeScore(d.ctx, target.client, target.org, target.stub, result, run)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.out, ref)
	return nil
}

// readScoreResult reads the result the ref names in the repo and parses the
// fields the score command needs. A missing repo or result is an error.
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

// runGrader runs the grader's run command against one result, writing the grader
// input on stdin and measuring the run's duration. It fails when the command
// fails, when the output carries no marker, or when the score output carries no
// score dictionary.
func runGrader(ctx context.Context, workDir, command string, model configModel, datum, result json.RawMessage) (graderRun, error) {
	input, err := json.Marshal(graderInput{Model: model, Data: datum, Result: result})
	if err != nil {
		return graderRun{}, err
	}

	start := time.Now()
	stdout, _, err := runShell(ctx, workDir, command, input)
	duration := time.Since(start).Seconds()
	if err != nil {
		return graderRun{}, fmt.Errorf("grader run command failed: %w", err)
	}

	logs, raw, ok := splitRunOutput(stdout)
	if !ok {
		return graderRun{}, errors.New("grader output carries no abnch marker")
	}
	var output graderOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return graderRun{}, fmt.Errorf("failed to read the score output: %w", err)
	}
	if output.Score == nil {
		return graderRun{}, errors.New("score output carries no score dictionary")
	}
	return graderRun{logs: logs, output: output, duration: duration}, nil
}

// writeScore records a grader run's score for its result and returns the score's
// ref. A not-found response maps to the missing result.
func writeScore(ctx context.Context, client *serviceClient, org, repo string, result scoreResultView, run graderRun) (string, error) {
	body := scoreRequest{
		Task: result.Task, Test: result.Test, Result: result.Ref,
		Dictionary: run.output.Score, Logs: run.logs,
		InputTokens: run.output.InputTokens, OutputTokens: run.output.OutputTokens,
		TokenCost: run.output.TokenCost, Duration: run.duration,
	}
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
		return "", resultNotFoundError(resp, org, repo, result.Ref)
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
