# Run CLI Specification

## Purpose

Run a task's agent against a test's data and read the results. A run executes locally. Workflow execution is out of scope. The agent config is defined in the [Agent CLI spec](agent.md). The model config is defined in the [Model CLI spec](model.md). The service contract is defined in the [Result Service API](../service/result.md).

## Requirements

### Requirement: Run a Test

`abnch run <task>/<test>` SHALL run the task's agent against each datum in the test and record a result for each.

The CLI SHALL read the agent from the config, resolve the model, build the agent once, then run it once per datum.

The command SHALL accept `--model`, `--local`, `--limit`, and `--shuffle`. `--model` overrides the task's default model. `--limit` caps the number of data run. `--shuffle` randomizes the data order. `--local` selects a local run, which is the only mode.

The command SHALL fail when the task has no agent, when no model is set, or when the test is missing.

#### Scenario: Run a test

- GIVEN task `acme/widgets/fix-bug` has an agent and a model
- AND test `fix-bug/basic` has two data
- WHEN the user runs `abnch run fix-bug/basic`
- THEN the agent runs against both data
- AND a result is recorded for each datum
- AND the command exits zero

#### Scenario: Limit the run

- GIVEN test `fix-bug/basic` has three data
- WHEN the user runs `abnch run fix-bug/basic --limit 2`
- THEN the agent runs against two data
- AND two results are recorded

#### Scenario: Shuffle the run

- GIVEN test `fix-bug/basic` has several data
- WHEN the user runs `abnch run fix-bug/basic --shuffle`
- THEN the agent runs against every datum in a randomized order

#### Scenario: Missing agent

- GIVEN task `acme/widgets/fix-bug` has no agent
- WHEN the user runs `abnch run fix-bug/basic`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no datum runs

#### Scenario: Missing model

- GIVEN the task has no default model and no `--model` is given
- WHEN the user runs `abnch run fix-bug/basic`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no datum runs

#### Scenario: Unknown model

- GIVEN the config holds no model `ghost`
- WHEN the user runs `abnch run fix-bug/basic --model ghost`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: Run One Datum

`abnch run <task>/<test>/<ref>` SHALL run the agent against one datum. The command SHALL fail when the datum is missing.

#### Scenario: Run one datum

- GIVEN datum `<ref>` exists in `acme/widgets/fix-bug/basic`
- WHEN the user runs `abnch run fix-bug/basic/<ref>`
- THEN the agent runs against that datum
- AND a result is recorded for it

#### Scenario: Missing datum

- GIVEN no datum with that ref exists
- WHEN the user runs `abnch run fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND the agent does not run

### Requirement: Run Input

The CLI SHALL pass a JSON object to the agent's run command on stdin.

The object SHALL carry the model's name, provider, and variant, and the datum's JSON.

#### Scenario: Agent reads the model and the datum

- GIVEN a model and a datum
- WHEN the agent's run command runs
- THEN stdin carries the model's name, provider, and variant, and the datum's JSON

### Requirement: Run Output

The agent's run command SHALL write its logs to stdout, then the abnch marker, then a run output as JSON. The marker and the run output are defined in the [Run Design](../../docs/design/run.md).

The CLI SHALL treat stdout before the marker as the result's logs and parse the JSON after the marker as the run output. The run output SHALL carry a `json` property holding the agent's result. It MAY carry `inputTokens`, `outputTokens`, and `tokenCost`.

The CLI SHALL measure each run's duration and record it with the result.

A run whose stdout carries no marker, or whose run output carries no `json` property, SHALL fail.

#### Scenario: Record the run output

- GIVEN an agent that writes logs, the marker, and a run output with a result and token counts
- WHEN the agent runs
- THEN the result carries the logs, the result JSON, the token counts, and the run duration

#### Scenario: Missing marker

- GIVEN an agent that writes no marker
- WHEN the user runs `abnch run fix-bug/basic`
- THEN the command exits non-zero
- AND an error is written to stderr

#### Scenario: Missing result

- GIVEN an agent whose run output carries no `json` property
- WHEN the user runs `abnch run fix-bug/basic`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: Local Branch

A local run SHALL record the working directory's Git branch as the result's local branch when the working directory is on a branch.

#### Scenario: Record the branch

- GIVEN the working directory is on branch `main`
- WHEN a local run records a result
- THEN the result's local branch is `main`

### Requirement: Read Results

`abnch get results` SHALL list the inferred repo's results as YAML. `abnch get result <ref>` SHALL output one result as JSON. The command SHALL fail when the result is missing.

#### Scenario: List results

- GIVEN the inferred repo has two results
- WHEN the user runs `abnch get results`
- THEN the output lists both results as YAML
- AND the command exits zero

#### Scenario: Get a result

- GIVEN a result `<ref>` exists in the inferred repo
- WHEN the user runs `abnch get result <ref>`
- THEN the result is written to stdout as JSON
- AND the command exits zero

#### Scenario: Get a missing result

- GIVEN no result with that ref exists
- WHEN the user runs `abnch get result <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.

When the service returns an error, the CLI SHALL surface the service error message and fail.

#### Scenario: Service error

- GIVEN the service returns `403` for a result write
- WHEN the user runs `abnch run fix-bug/basic`
- THEN the command exits non-zero
- AND the service error message is written to stderr
