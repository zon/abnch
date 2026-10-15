package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// runMarker is the line an agent's run command writes between its logs and its
// run output.
const runMarker = "__abnch__"

// resultRef is a result's ref as the service API returns it.
type resultRef struct {
	Ref string `json:"ref"`
}

// resultRequest is the JSON body a result write carries. JSON is the result the
// agent produced; the rest are its logs and run metrics.
type resultRequest struct {
	Task         string          `json:"task"`
	Test         string          `json:"test"`
	Data         string          `json:"data"`
	JSON         json.RawMessage `json:"json"`
	Logs         string          `json:"logs"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	TokenCost    float64         `json:"tokenCost"`
	Duration     float64         `json:"duration"`
	Local        string          `json:"local"`
}

// runInput is the JSON object the CLI writes to an agent's stdin.
type runInput struct {
	Model configModel     `json:"model"`
	Data  json.RawMessage `json:"data"`
}

// runOutput is the JSON an agent's run command writes after the marker.
type runOutput struct {
	JSON         json.RawMessage `json:"json"`
	InputTokens  int             `json:"inputTokens"`
	OutputTokens int             `json:"outputTokens"`
	TokenCost    float64         `json:"tokenCost"`
}

// agentRun is one agent run: the logs it wrote, its run output, and how long it
// took.
type agentRun struct {
	logs     string
	output   runOutput
	duration float64
}

// runTask runs the task's agent against each datum in the test and records a
// result for each, printing the result refs. It resolves the task's agent and
// model before it reads the test or runs anything, so it fails without running a
// datum when the task has no agent, no model is set, or the model is missing.
func runTask(d deps, c runCmd) error {
	taskStub, testStub, err := splitTestPath(c.Path)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(d.workDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath(d.workDir), err)
	}
	task, ok := cfg.Tasks[taskStub]
	if !ok || task.Agent == nil || task.Agent.RunCmd == "" {
		return fmt.Errorf("task %s has no agent", taskStub)
	}
	if task.Model == "" {
		return fmt.Errorf("task %s has no model", taskStub)
	}
	model, ok := cfg.Models[task.Model]
	if !ok {
		return fmt.Errorf("model %s not found", task.Model)
	}

	target, err := resolveTarget(d)
	if err != nil {
		return err
	}

	refs, err := readDataRefs(d.ctx, target.client, target.org, target.stub, taskStub, testStub)
	if err != nil {
		return err
	}

	if task.Agent.BuildCmd != "" {
		if _, _, err := runShell(d.ctx, d.workDir, task.Agent.BuildCmd, nil); err != nil {
			return fmt.Errorf("failed to build the agent: %w", err)
		}
	}

	local := gitBranch(d.workDir)
	for _, ref := range refs {
		datum, err := readDatum(d.ctx, target.client, target.org, target.stub, taskStub, testStub, ref)
		if err != nil {
			return err
		}
		run, err := runAgent(d.ctx, d.workDir, task.Agent.RunCmd, model, datum)
		if err != nil {
			return err
		}
		result, err := writeResult(d.ctx, target.client, target.org, target.stub, taskStub, testStub, ref, run, local)
		if err != nil {
			return err
		}
		fmt.Fprintln(d.out, result)
	}
	return nil
}

// runAgent runs the agent's run command against one datum, writing the run input
// on stdin and measuring the run's duration. It fails when the command fails,
// when the output carries no marker, or when the run output carries no result
// JSON.
func runAgent(ctx context.Context, workDir, command string, model configModel, datum []byte) (agentRun, error) {
	input, err := json.Marshal(runInput{Model: model, Data: datum})
	if err != nil {
		return agentRun{}, err
	}

	start := time.Now()
	stdout, _, err := runShell(ctx, workDir, command, input)
	duration := time.Since(start).Seconds()
	if err != nil {
		return agentRun{}, fmt.Errorf("agent run command failed: %w", err)
	}

	logs, raw, ok := splitRunOutput(stdout)
	if !ok {
		return agentRun{}, errors.New("agent output carries no abnch marker")
	}
	var output runOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return agentRun{}, fmt.Errorf("failed to read the run output: %w", err)
	}
	if len(output.JSON) == 0 {
		return agentRun{}, errors.New("run output carries no result JSON")
	}
	return agentRun{logs: logs, output: output, duration: duration}, nil
}

// runShell runs command in workDir with the given stdin and returns its stdout
// and stderr. A command that exits non-zero fails, reporting its stderr.
func runShell(ctx context.Context, workDir, command string, stdin []byte) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(errOut.String())
		if message != "" {
			return out.Bytes(), errOut.Bytes(), fmt.Errorf("%w: %s", err, message)
		}
		return out.Bytes(), errOut.Bytes(), err
	}
	return out.Bytes(), errOut.Bytes(), nil
}

// splitRunOutput splits an agent's stdout into the logs before the marker and
// the run output after it. It reports whether the marker is present.
func splitRunOutput(stdout []byte) (logs string, output []byte, ok bool) {
	lines := strings.Split(string(stdout), "\n")
	for i, line := range lines {
		if strings.TrimRight(line, "\r") == runMarker {
			if i > 0 {
				logs = strings.Join(lines[:i], "\n") + "\n"
			}
			return logs, []byte(strings.Join(lines[i+1:], "\n")), true
		}
	}
	return "", nil, false
}

// writeResult records a run's result for its datum and returns the result's ref.
// A not-found response maps to the missing repo, task, test, or datum.
func writeResult(ctx context.Context, client *serviceClient, org, repo, task, test, data string, run agentRun, local string) (string, error) {
	body := resultRequest{
		Task: task, Test: test, Data: data,
		JSON: run.output.JSON, Logs: run.logs,
		InputTokens: run.output.InputTokens, OutputTokens: run.output.OutputTokens,
		TokenCost: run.output.TokenCost, Duration: run.duration, Local: local,
	}
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
		return "", dataNotFoundError(resp, org, repo, task, test, data)
	default:
		return "", serviceError(resp)
	}
}

// resultsPath returns the API path for a repo's results.
func resultsPath(org, repo string) string {
	return repoPath(org, repo) + "/results"
}

// gitBranch returns the branch the Git repo under workDir is on, or "" when it
// is not on a branch or the branch cannot be read.
func gitBranch(workDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return ""
	}
	return branch
}
